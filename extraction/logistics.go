package extraction

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"satisfacts/parser"
)

// ResolveLogistics takes the raw train, vehicle, and drone objects collected
// during streaming and resolves them into the final LogisticsData with
// human-readable names, routes, and status warnings.
func (e *ExtractedData) ResolveLogistics() {
	// Build the track graph up front so we can compute train route distances
	trackGraph := e.buildTrackGraph()

	e.resolveTrains(trackGraph)
	e.resolveTrainStations()
	e.resolveVehicles()
	e.resolveVehicleStations()
	e.resolveDrones()
	e.resolveDroneStations()

	// Free raw maps
	e.rawTrains = nil
	e.rawTimetables = nil
	e.rawTrainStationIDs = nil
	e.rawTrainStations = nil
	e.rawLocomotives = nil
	e.rawWagons = nil
	e.rawWheeledVehicles = nil
	e.rawTrainPlatforms = nil
	e.rawWheeledVehicleIDs = nil
	e.rawWheeledVehicleInfos = nil
	e.rawVehiclePaths = nil
	e.rawTruckStationIDs = nil
	e.rawTruckStations = nil
	e.rawDrones = nil
	e.rawDroneStationInfos = nil
	e.rawDroneStations = nil
	e.rawTrackSegments = nil
	e.rawTrackConns = nil
	e.rawSignals = nil

	// Sort all logistics lists by name for consistent output.
	// SliceStable matters here: plenty of these share a name (239 stations, a
	// lot of them still called "Train Station"), and an unstable sort would
	// shuffle the equal ones around differently on every run.
	sort.SliceStable(e.Logistics.TrainStations, func(i, j int) bool {
		return e.Logistics.TrainStations[i].Name < e.Logistics.TrainStations[j].Name
	})
	sort.SliceStable(e.Logistics.VehicleStations, func(i, j int) bool {
		return e.Logistics.VehicleStations[i].Name < e.Logistics.VehicleStations[j].Name
	})
	sort.SliceStable(e.Logistics.DroneStations, func(i, j int) bool {
		return e.Logistics.DroneStations[i].Name < e.Logistics.DroneStations[j].Name
	})
	sort.SliceStable(e.Logistics.Trains, func(i, j int) bool {
		return e.Logistics.Trains[i].Name < e.Logistics.Trains[j].Name
	})
	sort.SliceStable(e.Logistics.Vehicles, func(i, j int) bool {
		return e.Logistics.Vehicles[i].Name < e.Logistics.Vehicles[j].Name
	})
}

// textPropValue reads a TextProperty and returns the string, or "" if the
// object has no text value (e.g. a station that was never renamed).
func textPropValue(obj *parser.SaveObject, name string) string {
	v, ok := GetPropText(obj, name)
	if !ok {
		return ""
	}
	return v
}

// refPathName returns the pathName from an ObjectProperty, or "" if missing.
func refPathName(obj *parser.SaveObject, name string) string {
	pn, ok := GetPropObjectRefPathName(obj, name)
	if !ok {
		return ""
	}
	return pn
}

// shortName returns the last segment of a UE path (everything after the last dot).
func shortName(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// resolveTrains walks each BP_Train, follows its TimeTable reference, and
// resolves the list of stops into station names. It also calculates the total
// route distance by running Dijkstra on the track graph between each pair of
// consecutive stations.
// Vehicle counts come from the coupling-based BFS in train_assignment.go.
func (e *ExtractedData) resolveTrains(graph *trackGraph) {
	// Run the vehicle assignment BFS once for all trains.
	// Returns a map of firstVehicleRef -> [locomotives, wagons].
	vehicleCounts := e.resolveTrainVehicles()

	// Map station identifier pathNames to their world positions so we can
	// snap them to nearby track nodes later
	stationPosByID := make(map[string][3]float64)
	for _, sid := range e.rawTrainStationIDs {
		stationRef := refPathName(sid, "mStation")
		if station, ok := e.rawTrainStations[stationRef]; ok {
			if pos, ok := extractPos(station); ok {
				stationPosByID[sid.Header.Reference.PathName] = pos
			}
		}
	}

	for _, trainKey := range sortedKeys(e.rawTrains) {
		train := e.rawTrains[trainKey]
		info := TrainInfo{
			Name: textPropValue(train, "mTrainName"),
		}
		if info.Name == "" {
			info.Name = "" // frontend uses t('label.train') as default
		}

		// Look up the vehicle counts from the assignment algorithm
		firstRef := refPathName(train, "FirstVehicle")
		if vr, ok := vehicleCounts[firstRef]; ok {
			info.Locomotives = vr.counts[0]
			info.Wagons = vr.counts[1]

			// Compute wagon fill percentage
			if len(vr.wagonRefs) > 0 {
				totalFill := 0
				counted := 0
				for _, wagonRef := range vr.wagonRefs {
					wagon := e.rawWagons[wagonRef]
					if wagon == nil {
						continue
					}
					invRef := refPathName(wagon, "mStorageInventory")
					if invRef == "" {
						continue
					}
					if stacks, ok := e.InventoryStacks[invRef]; ok {
						totalSlots := e.InventoryTotalSlots[invRef]
						totalFill += computeInventoryFill(stacks, totalSlots)
						counted++
					}
				}
				if counted > 0 {
					info.WagonFillPct = totalFill / counted
				}
			}
		} else {
			// Fall back to counting just first/last if the BFS didn't find this train
			locoCount, wagonCount := e.countTrainVehicles(firstRef, refPathName(train, "LastVehicle"))
			info.Locomotives = locoCount
			info.Wagons = wagonCount
		}

		// Pull stops and current position from the timetable
		var stopPositions [][3]float64
		ttRef := refPathName(train, "TimeTable")
		if tt, ok := e.rawTimetables[ttRef]; ok {
			if stop, ok := GetPropInt32(tt, "mCurrentStop"); ok {
				info.CurrentStop = int(stop)
			}
			// Each stop points to a FGTrainStationIdentifier which has the station name
			if stops, ok := GetPropArray(tt, "mStops"); ok {
				for _, stopItem := range stops {
					stopMap, ok := stopItem.(map[string]interface{})
					if !ok {
						continue
					}
					stationRef, ok := stopMap["Station"].(map[string]string)
					if !ok {
						continue
					}
					stationIDPath := stationRef["pathName"]
					if sid, ok := e.rawTrainStationIDs[stationIDPath]; ok {
						name := textPropValue(sid, "mStationName")
						if name == "" {
							name = "Unnamed Station"
						}
						info.Route = append(info.Route, name)
					} else {
						info.Route = append(info.Route, "?")
					}
					// Grab the station position while we're here - we'll need it for distance calc
					if pos, ok := stationPosByID[stationIDPath]; ok {
						stopPositions = append(stopPositions, pos)
					} else {
						stopPositions = append(stopPositions, [3]float64{})
					}
				}
			}
		}

		// Run Dijkstra between consecutive stations to get the total route distance.
		// Build a per-train directed graph: signals already directed some segments,
		// the locomotive heading directs any remaining bidirectional (unsignaled) ones.
		if graph != nil && len(stopPositions) >= 2 {
			trainGraph := graph

			// Find the first locomotive to determine travel direction.
			locoObj := e.rawLocomotives[firstRef]
			if locoObj == nil {
				lastRef := refPathName(train, "LastVehicle")
				locoObj = e.rawLocomotives[lastRef]
			}
			if locoObj != nil && locoObj.Header != nil && locoObj.Header.NeedTransform {
				locoPos := [3]float64{
					float64(locoObj.Header.Transform.Translation[0]) / 100,
					float64(locoObj.Header.Transform.Translation[1]) / 100,
					float64(locoObj.Header.Transform.Translation[2]) / 100,
				}
				locoRot := locoObj.Header.Transform.Rotation
				isReversed := false
				if v, ok := GetPropBool(locoObj, "mIsOrientationReversed"); ok && v {
					isReversed = true
				}
				trainGraph = graph.buildDirectedForTrain(locoPos, locoRot, isReversed)
			}

			var totalDist float64
			for j := 0; j < len(stopPositions)-1; j++ {
				if stopPositions[j] == [3]float64{} || stopPositions[j+1] == [3]float64{} {
					continue
				}
				srcNode := trainGraph.findNearestNode(stopPositions[j])
				dstNode := trainGraph.findNearestNode(stopPositions[j+1])
				if srcNode >= 0 && dstNode >= 0 {
					legDist := trainGraph.dijkstra(srcNode, dstNode)
					if legDist == 0 && trainGraph != graph {
						// Fallback to signal-directed graph
						srcNode2 := graph.findNearestNode(stopPositions[j])
						dstNode2 := graph.findNearestNode(stopPositions[j+1])
						legDist = graph.dijkstra(srcNode2, dstNode2)
					}
					totalDist += legDist
				}
			}
			// If the route loops back to the start, count that leg too
			if len(stopPositions) >= 2 &&
				stopPositions[0] != [3]float64{} &&
				stopPositions[len(stopPositions)-1] != [3]float64{} {
				srcNode := trainGraph.findNearestNode(stopPositions[len(stopPositions)-1])
				dstNode := trainGraph.findNearestNode(stopPositions[0])
				if srcNode >= 0 && dstNode >= 0 {
					legDist := trainGraph.dijkstra(srcNode, dstNode)
					if legDist == 0 && trainGraph != graph {
						srcNode2 := graph.findNearestNode(stopPositions[len(stopPositions)-1])
						dstNode2 := graph.findNearestNode(stopPositions[0])
						legDist = graph.dijkstra(srcNode2, dstNode2)
					}
					if legDist > 0 {
						totalDist += legDist
					}
				}
			}
			if totalDist > 0 {
				info.TotalDistance = totalDist / 1000 // convert m to km
			}
		}

		if len(info.Route) == 0 {
			info.Warnings = append(info.Warnings, "No timetable")
		}
		if info.Locomotives == 0 {
			info.Warnings = append(info.Warnings, "No locomotive")
		}

		e.Logistics.Trains = append(e.Logistics.Trains, info)
	}
}

// countTrainVehicles checks whether the FirstVehicle and LastVehicle refs
// point to locomotives or wagons. The save doesn't store a full vehicle list
// for a train, so this is a best-effort count. Most trains are 1 loco + N wagons.
func (e *ExtractedData) countTrainVehicles(firstRef, lastRef string) (locomotives, wagons int) {
	if firstRef == "" {
		return 0, 0
	}
	// Check what type the first vehicle is
	if _, ok := e.rawLocomotives[firstRef]; ok {
		locomotives++
	} else if _, ok := e.rawWagons[firstRef]; ok {
		wagons++
	}
	if lastRef != "" && lastRef != firstRef {
		if _, ok := e.rawLocomotives[lastRef]; ok {
			locomotives++
		} else if _, ok := e.rawWagons[lastRef]; ok {
			wagons++
		}
	}
	return
}

// trainStationName returns the custom name of a Build_TrainStation, or "" if
// it has none. stationIDKeys is sortedKeys(e.rawTrainStationIDs), passed in so
// callers looping over many stations sort it only once.
func (e *ExtractedData) trainStationName(instanceName string, stationIDKeys []string) string {
	// Look up the name from the matching FGTrainStationIdentifier.
	// The identifier's mStation ref may point at a component suffix of the
	// station (e.g. "Build_X.RailroadTrack"), so we also try matching just
	// the last path segment.
	for _, sidKey := range stationIDKeys {
		sid := e.rawTrainStationIDs[sidKey]
		stationRef := refPathName(sid, "mStation")
		if stationRef == instanceName || strings.HasSuffix(stationRef, "."+shortName(instanceName)) {
			return textPropValue(sid, "mStationName")
		}
	}
	return ""
}

// resolveTrainStations builds the station list by looking up each
// Build_TrainStation, finding its name via the FGTrainStationIdentifier,
// and checking which train is currently docked.
func (e *ExtractedData) resolveTrainStations() {
	// Pre-compute station positions so we can assign platforms in a single pass.
	type stEntry struct {
		pos  [3]float64
		info *TrainStationInfo
		// fillSum/fillCount track the running average of PlatformFillPct
		// across only the platforms that actually had inventory data -
		// info.Platforms counts ALL assigned platforms (line below),
		// including ones with no inventory, so it can't be reused as the
		// average's denominator.
		fillSum   int
		fillCount int
	}
	var stList []stEntry

	// Sorted once and reused across every outer iteration below - this map
	// doesn't change during the loop, so re-sorting it per station (as
	// sortedKeys would do inline) is wasted work repeated stations-many
	// times over.
	stationIDKeys := sortedKeys(e.rawTrainStationIDs)

	for _, instanceName := range sortedKeys(e.rawTrainStations) {
		station := e.rawTrainStations[instanceName]
		info := &TrainStationInfo{
			Name: "Unnamed Station",
		}

		if name := e.trainStationName(instanceName, stationIDKeys); name != "" {
			info.Name = name
		}

		// Check for docked locomotive
		dockedLoco := refPathName(station, "mDockingLocomotive")
		if dockedLoco != "" {
			info.DockedTrain = e.findTrainByVehicle(dockedLoco)
		}

		var pos [3]float64
		if p, ok := extractPos(station); ok {
			pos = p
			info.Location = p
		}
		stList = append(stList, stEntry{pos: pos, info: info})
	}

	// Assign each platform to its nearest station by 3D distance.
	// Platforms sit on the track adjacent to their station, so nearest = correct.
	for _, platformKey := range sortedKeys(e.rawTrainPlatforms) {
		platform := e.rawTrainPlatforms[platformKey]
		pPos, ok := extractPos(platform)
		if !ok || len(stList) == 0 {
			continue
		}
		bestIdx := 0
		bestDist := stationDist3D(stList[0].pos, pPos)
		for i := 1; i < len(stList); i++ {
			d := stationDist3D(stList[i].pos, pPos)
			if d < bestDist {
				bestDist = d
				bestIdx = i
			}
		}
		stList[bestIdx].info.Platforms++

		// Compute fill for this platform's inventory
		invRef := refPathName(platform, "mInventory")
		if invRef != "" {
			if stacks, ok := e.InventoryStacks[invRef]; ok {
				totalSlots := e.InventoryTotalSlots[invRef]
				fill := computeInventoryFill(stacks, totalSlots)
				// Accumulate fill across only the platforms with inventory
				// data for this station, and recompute the average. Using a
				// dedicated sum/count (rather than reconstructing it from
				// PlatformFillPct itself) avoids two bugs the old formula
				// had: a legitimate 0% fill being mistaken for "not set yet"
				// and overwriting instead of averaging in, and dividing by
				// the total platform count when some platforms have no
				// inventory data at all.
				stList[bestIdx].fillSum += fill
				stList[bestIdx].fillCount++
				stList[bestIdx].info.PlatformFillPct = stList[bestIdx].fillSum / stList[bestIdx].fillCount
			}
		}
	}

	for _, st := range stList {
		e.Logistics.TrainStations = append(e.Logistics.TrainStations, *st.info)
	}
}

// extractPos returns the [x, y, z] position of an object in meters (in-game
// map coordinates), converting from the save file's centimeter units.
func extractPos(obj *parser.SaveObject) ([3]float64, bool) {
	if obj.Header == nil || !obj.Header.NeedTransform {
		return [3]float64{}, false
	}
	return [3]float64{
		float64(obj.Header.Transform.Translation[0]) / 100,
		float64(obj.Header.Transform.Translation[1]) / 100,
		float64(obj.Header.Transform.Translation[2]) / 100,
	}, true
}

// stationDist3D returns the distance between two positions (including Z).
func stationDist3D(a, b [3]float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	dz := a[2] - b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// findTrainByVehicle returns the name of the train whose FirstVehicle or
// LastVehicle matches the given locomotive reference.
func (e *ExtractedData) findTrainByVehicle(locoRef string) string {
	for _, trainKey := range sortedKeys(e.rawTrains) {
		train := e.rawTrains[trainKey]
		first := refPathName(train, "FirstVehicle")
		last := refPathName(train, "LastVehicle")
		if first == locoRef || last == locoRef {
			return textPropValue(train, "mTrainName")
		}
	}
	return ""
}

// resolveVehicles builds the vehicle list from FGWheeledVehicleIdentifier
// objects (1.2 saves) or directly from BP_* vehicle objects (1.0 saves),
// cross-referencing FGWheeledVehicleInfo for fuel status.
func (e *ExtractedData) resolveVehicles() {
	// Map vehicle instance -> fuel class from FGWheeledVehicleInfo
	vehicleFuelMap := make(map[string]string)
	for _, viKey := range sortedKeys(e.rawWheeledVehicleInfos) {
		vi := e.rawWheeledVehicleInfos[viKey]
		vehicleRef := refPathName(vi, "mVehicle")
		fuelClass := refPathName(vi, "mCurrentFuelClass")
		vehicleFuelMap[vehicleRef] = fuelClass
	}

	if len(e.rawWheeledVehicleIDs) > 0 {
		// 1.2 saves: vehicles have FGWheeledVehicleIdentifier objects that
		// hold the name and link back to the raw vehicle for position/status.
		for _, idKey := range sortedKeys(e.rawWheeledVehicleIDs) {
			id := e.rawWheeledVehicleIDs[idKey]
			vehicleRef := refPathName(id, "mOwnerVehicle")
			vehicleClass := refPathName(id, "mOwnerVehicleClass")

			info := VehicleInfo{
				Name: textPropValue(id, "mVehicleName"),
				Type: vehicleClass,
				Fuel: vehicleFuelMap[vehicleRef],
			}
			if info.Fuel == "" && !strings.Contains(strings.ToLower(vehicleClass), "golfcart") {
				info.Warnings = append(info.Warnings, "No fuel")
			}

			// Location and on-route status from the raw vehicle object
			if vehicle, ok := e.rawWheeledVehicles[vehicleRef]; ok {
				if pos, ok := extractPos(vehicle); ok {
					info.Location = pos
				}
				if refPathName(vehicle, "mCurrentVehiclePathSegment") != "" {
					info.OnRoute = true
				}
			}

			e.Logistics.Vehicles = append(e.Logistics.Vehicles, info)
		}
	} else {
		// 1.0 saves: no FGWheeledVehicleIdentifier, so we build directly
		// from the raw BP_* vehicle objects. Names default to the vehicle type.
		for _, vehicleRef := range sortedKeys(e.rawWheeledVehicles) {
			vehicle := e.rawWheeledVehicles[vehicleRef]
			vehicleClass := vehicle.Header.ClassName

			info := VehicleInfo{
				Type: vehicleClass,
				Fuel: vehicleFuelMap[vehicleRef],
			}
			if info.Fuel == "" && !strings.Contains(strings.ToLower(vehicleClass), "golfcart") {
				info.Warnings = append(info.Warnings, "No fuel")
			}

			if pos, ok := extractPos(vehicle); ok {
				info.Location = pos
			}
			// 1.0 saves use mIsAutopilotEnabled instead of mCurrentVehiclePathSegment
			// to indicate the vehicle is self-driving on a recorded path.
			if ap, ok := GetPropBool(vehicle, "mIsAutopilotEnabled"); ok && ap {
				info.OnRoute = true
			}

			e.Logistics.Vehicles = append(e.Logistics.Vehicles, info)
		}
	}
}

// resolveVehicleStations builds the truck station list from
// FGDockingStationIdentifier/Info objects, pulling names, power status, and
// docking statistics from the linked Build_TruckStation/FluidTruckStation.
func (e *ExtractedData) resolveVehicleStations() {
	stationCounter := 0
	for _, sidKey := range sortedKeys(e.rawTruckStationIDs) {
		sid := e.rawTruckStationIDs[sidKey]
		stationRef := refPathName(sid, "mStation")
		name := textPropValue(sid, "mStationName")

		stationType := "Truck Station"
		if station, ok := e.rawTruckStations[stationRef]; ok {
			if strings.Contains(strings.ToLower(station.Header.ClassName), "fluidtruckstation") {
				stationType = "Fluid Truck Station"
			}
		}

		if name == "" {
			stationCounter++
			name = fmt.Sprintf("%s %d", stationType, stationCounter)
		}

		info := VehicleStationInfo{
			Name: name,
			Type: stationType,
		}

		// Status
		if status, ok := GetPropEnum(sid, "mDockingStationStatus"); ok {
			info.Status = cleanEnum(status)
			if strings.Contains(status, "NoPower") {
				info.Warnings = append(info.Warnings, "No power")
			}
		}

		// Get docking stats and location from the Build_TruckStation
		if station, ok := e.rawTruckStations[stationRef]; ok {
			if pos, ok := extractPos(station); ok {
				info.Location = pos
			}
			if dockingStats, ok := GetPropMap(station, "mDockingVehicleStatistics"); ok {
				if entries, ok := dockingStats["entries"].([]interface{}); ok && len(entries) > 0 {
					// Use the first (most recent) docking stats entry
					if entry, ok := entries[0].(map[string]interface{}); ok {
						if val, ok := entry["value"].(map[string]interface{}); ok {
							if v, ok := val["mTimeSinceLastDocking"]; ok {
								info.LastDockTime = toFloat64(v)
							}
							if v, ok := val["mWeightedAverageItemsTransferred"]; ok {
								info.AvgItemsTransferred = toFloat64(v)
							}
							if v, ok := val["mWeightedAverageTimeBetweenDocks"]; ok {
								info.AvgTimeBetweenDocks = toFloat64(v)
							}
						}
					}
				}
			}
		}

		e.Logistics.VehicleStations = append(e.Logistics.VehicleStations, info)
	}
}

// resolveDrones builds the drone list from BP_DroneTransport objects,
// extracting fuel type, docking/flying state, and trip info.
func (e *ExtractedData) resolveDrones() {
	for _, droneKey := range sortedKeys(e.rawDrones) {
		drone := e.rawDrones[droneKey]
		// mActiveFuelName is a NameProperty (e.g. "Desc_Battery_C"), not an
		// ObjectProperty, so we read it as a string rather than an object ref.
		fuelName := ""
		if v, ok := GetPropString(drone, "mActiveFuelName"); ok {
			fuelName = v
		}
		info := DroneInfo{
			Fuel: fuelName,
		}

		// Check if drone is on a trip (in transit)
		hasBegunTrip := false
		if v, ok := GetPropBool(drone, "mHasBegunTrip"); ok && v {
			hasBegunTrip = true
		}

		if hasBegunTrip {
			// Drone is flying - show destination and trip info
			info.DockingState = "In Transit"
			destRef := refPathName(drone, "mCurrentDestinationStation")
			if destRef != "" {
				info.Destination = e.findDroneStationName(destRef)
			}
			// Trip details
			if trip, ok := GetPropStruct(drone, "mCurrentTripInformation"); ok {
				if tripMap, ok := trip.(map[string]interface{}); ok {
					info.TripItemCount = int(toFloat64(tripMap["IncomingItemCount"]))
					info.TripDuration = toFloat64(tripMap["TripDuration"])
				}
			}
		} else {
			// Drone is docked - show docking state
			if state, ok := GetPropStruct(drone, "mCurrentDockingState"); ok {
				if stateMap, ok := state.(map[string]interface{}); ok {
					if stateVal, ok := stateMap["State"].(string); ok {
						info.DockingState = cleanEnum(stateVal)
					}
				}
			}
		}

		// Home station name
		homeStationRef := refPathName(drone, "mHomeStation")
		if homeStationRef != "" {
			info.HomeStation = e.findDroneStationName(homeStationRef)
		}

		// Fuel warning: check if fuel data shows zero energy
		if fuelData, ok := GetPropArray(drone, "mFuelData"); ok && len(fuelData) > 0 {
			maxEnergy := 0.0
			for _, item := range fuelData {
				if itemMap, ok := item.(map[string]interface{}); ok {
					energy := toFloat64(itemMap["EnergyLevel"])
					if energy > maxEnergy {
						maxEnergy = energy
					}
				}
			}
			if maxEnergy <= 0 {
				info.Warnings = append(info.Warnings, "No fuel")
			}
		} else {
			info.Warnings = append(info.Warnings, "No fuel")
		}

		e.Logistics.Drones = append(e.Logistics.Drones, info)
	}
}

// resolveDroneStations builds the drone station list from FGDroneStationInfo
// objects, resolving paired station connections and custom names (mBuildingTag).
func (e *ExtractedData) resolveDroneStations() {
	// stationRef -> display name (custom mBuildingTag, or sequential
	// fallback) for every drone station - already the same lookup
	// findDroneStationName uses, computed once here (or on first prior use).
	stationNames := e.droneStationDisplayNames()

	for _, stationInfoKey := range sortedKeys(e.rawDroneStationInfos) {
		stationInfo := e.rawDroneStationInfos[stationInfoKey]
		stationRef := refPathName(stationInfo, "mStation")
		pairedRef := refPathName(stationInfo, "mPairedStation")

		info := DroneStationInfo{
			Name: stationNames[stationRef],
		}

		// Check if producing and get transfer stage
		if station, ok := e.rawDroneStations[stationRef]; ok {
			if pos, ok := extractPos(station); ok {
				info.Location = pos
			}
			if producing, ok := GetPropBool(station, "mIsProducing"); ok && !producing {
				info.Warnings = append(info.Warnings, "Not producing")
			}
			// Transfer stage (e.g. ITS_LOAD, ITS_UNLOAD, ITS_IDLE)
			if stage, ok := GetPropEnum(station, "mItemTransferringStage"); ok {
				info.TransferStage = cleanEnum(stage)
			}
		}

		// Paired station: mPairedStation points to another FGDroneStationInfo,
		// not to a Build_DroneStation. We have to hop through mStation to get
		// the actual station name.
		if pairedRef != "" {
			if paired, ok := e.rawDroneStationInfos[pairedRef]; ok {
				pairedStationRef := refPathName(paired, "mStation")
				info.PairedStation = stationNames[pairedStationRef]
			}
		}

		e.Logistics.DroneStations = append(e.Logistics.DroneStations, info)
	}
}

// findDroneStationName returns the custom name (mBuildingTag) for a drone
// station by its reference, falling back to a sequential name (A, B, C...).
// Used by resolveDrones to label home/destination stations.
func (e *ExtractedData) findDroneStationName(stationRef string) string {
	return e.droneStationDisplayNames()[stationRef]
}

// droneStationDisplayNames returns, for every drone station's mStation ref,
// its display name: the custom mBuildingTag if set, otherwise a sequential
// name ("Drone Station A", "B", ...) based on position in sorted key order.
// Built once and cached - every caller used to redo this (including a full
// sortedKeys scan) on every single lookup.
func (e *ExtractedData) droneStationDisplayNames() map[string]string {
	if e.droneStationNames != nil {
		return e.droneStationNames
	}
	names := make(map[string]string, len(e.rawDroneStationInfos))
	counter := 0
	for _, siKey := range sortedKeys(e.rawDroneStationInfos) {
		si := e.rawDroneStationInfos[siKey]
		counter++
		name := "Drone Station " + string(rune('A'+counter-1))
		if tag, ok := GetPropString(si, "mBuildingTag"); ok && tag != "" {
			name = tag
		}
		names[refPathName(si, "mStation")] = name
	}
	e.droneStationNames = names
	return names
}

// cleanEnum strips the UE enum prefix, e.g. "EDroneDockingState::DS_DOCKED" -> "DS_DOCKED".
func cleanEnum(s string) string {
	idx := strings.Index(s, "::")
	if idx >= 0 {
		return s[idx+2:]
	}
	return s
}
