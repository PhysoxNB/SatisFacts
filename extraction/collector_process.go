package extraction

import (
	"fmt"
	"strings"

	"satisfacts/parser"
)

func (e *ExtractedData) ProcessObject(obj *parser.SaveObject, extractStructures bool) {
	e.TotalProcessed++

	if obj.Header == nil {
		return
	}

	typePath := obj.Header.ClassName
	// Everything derived purely from the class name is computed once per
	// distinct class name and cached; see classcache.go.
	ci := classInfoFor(typePath)
	typeLower := ci.lower
	instanceName := obj.Header.Reference.PathName

	// Store position for all objects with transforms (for power line resolution)
	if instanceName != "" && obj.Header.NeedTransform {
		e.ObjectPositions[instanceName] = [3]float32{
			obj.Header.Transform.Translation[0],
			obj.Header.Transform.Translation[1],
			obj.Header.Transform.Translation[2],
		}
	}

	// If this is a known world collectible that's still in the save, it hasn't
	// been picked up yet.
	isCollectible := false
	if collectiblePositionsLoaded != nil && instanceName != "" {
		ref := instanceName
		if _, ok := collectiblePositionsLoaded.PowerSlugs.Blue[ref]; ok {
			e.CollectiblesFound.PowerSlugsBlue[ref] = true
			isCollectible = true
		} else if _, ok := collectiblePositionsLoaded.PowerSlugs.Yellow[ref]; ok {
			e.CollectiblesFound.PowerSlugsYellow[ref] = true
			isCollectible = true
		} else if _, ok := collectiblePositionsLoaded.PowerSlugs.Purple[ref]; ok {
			e.CollectiblesFound.PowerSlugsPurple[ref] = true
			isCollectible = true
		} else if _, ok := collectiblePositionsLoaded.Somersloops[ref]; ok {
			e.CollectiblesFound.Somersloops[ref] = true
			isCollectible = true
		} else if _, ok := collectiblePositionsLoaded.MercerSpheres[ref]; ok {
			e.CollectiblesFound.MercerSpheres[ref] = true
			isCollectible = true
		} else if _, ok := collectiblePositionsLoaded.CrashSites[ref]; ok {
			e.CollectiblesFound.CrashSites[ref] = true
			isCollectible = true
			// Opened/looted means the hard drive's already been grabbed.
			opened := false
			if v, ok := GetPropBool(obj, "mHasBeenOpened"); ok && v {
				opened = true
			}
			if v, ok := GetPropBool(obj, "mHasBeenLooted"); ok && v {
				opened = true
			}
			if opened {
				e.CollectiblesFound.CrashSiteOpened[ref] = true
			} else {
				e.CollectiblesFound.CrashSiteUnopened[ref] = true
			}
		}
	}

	// Retain collectible positions for the viewer.
	// Transform is always parsed even when NeedTransform is false.
	if isCollectible && instanceName != "" {
		e.CollectibleLocations[instanceName] = obj.Header.Transform.Translation
	}

	// Resolve inventory stacks to compact form now, discard raw object later.
	if ci.containsLower["inventorycomponent"] {
		if raw, ok := GetPropArray(obj, "mInventoryStacks"); ok {
			e.InventoryStacks[instanceName] = parseInventoryStacks(raw)
			e.InventoryTotalSlots[instanceName] = len(raw)
		}
	}

	// BuildingFacts for buildings and power-consuming logistics
	isBuildingType := (ci.isBuilding && !ci.isInternalSystem) ||
		ci.containsLower["pipepump"] ||
		ci.containsLower["trainstation"] ||
		ci.containsLower["truckstation"] ||
		ci.containsLower["freightplatform"] ||
		ci.containsLower["dockingstation"]

	// Modded buildings (e.g. RefinedPower) may not match computeIsBuilding
	// keywords but still have mPowerInfo. Detect them by property presence
	// so power info gets inlined and they appear in the power tab.
	// Exclude hypertube entrances — they have mPowerInfo but are infrastructure,
	// not buildings. Rendering them as building squares misplaces them on the map.
	if !isBuildingType && instanceName != "" && obj.Properties != nil {
		if _, hasPowerInfo := obj.Properties["mPowerInfo"]; hasPowerInfo &&
			!ci.containsLower["pipehyperstart"] {
			isBuildingType = true
		}
	}

	if isBuildingType && instanceName != "" {
		bf := populateBuildingFacts(obj)
		e.BuildingFacts[instanceName] = bf
		// Map PowerInfo ref to building for later inlining
		if bf.PowerInfoRef != "" {
			e.powerInfoToBuilding[bf.PowerInfoRef] = instanceName
		}
		// Buildings without mCurrentPotential default to 100% clock speed.
		if !bf.HasCurrentPotential && !ci.clockSpeedExcluded {
			e.DefaultClockSpeeds[instanceName] = typePath
		}
	}

	// Inline power info fields onto the parent building's BuildingFacts
	if ci.containsLower["powerinfo"] && !ci.containsLower["powerconnection"] && instanceName != "" {
		if buildingInst, ok := e.powerInfoToBuilding[instanceName]; ok {
			if bf, ok2 := e.BuildingFacts[buildingInst]; ok2 && bf != nil {
				if v, ok2 := GetPropFloat32(obj, "mTargetConsumption"); ok2 {
					bf.TargetConsumption = v
					bf.HasTargetConsumption = true
				}
				if v, ok2 := GetPropFloat32(obj, "mDynamicProductionCapacity"); ok2 {
					bf.DynamicProductionCapacity = v
					bf.HasDynamicProductionCapacity = true
				}
				if v, ok2 := GetPropFloat32(obj, "mBaseProduction"); ok2 {
					bf.BaseProduction = v
					bf.HasBaseProduction = true
				}
				if v, ok2 := GetPropFloat32(obj, "mCurrentPotential"); ok2 {
					bf.PowerInfoPotential = v
					bf.HasPowerInfoPotential = true
				}
			}
		}
	}

	// Power connection: strip ".PowerConnection" suffix to find parent building
	if ci.containsLower["powerconnection"] && instanceName != "" {
		if bf, ok := e.BuildingFacts[stripLastComponent(instanceName)]; ok && bf != nil {
			if wires, ok2 := GetPropArray(obj, "mWires"); ok2 {
				bf.WireCount = len(wires)
			}
		}
	}

	// Populate ResourceNodeFacts for resource nodes / sources / fracking.
	if (ci.containsLower["resourcenode"] ||
		ci.containsLower["resourcesource"] ||
		ci.containsLower["frackingsatellite"] ||
		ci.containsLower["resourcewell"]) && instanceName != "" {
		e.ResourceNodeFacts[instanceName] = populateResourceNodeFacts(obj)
	}

	// Capture InventoryPotential mCurrentPotential for nuclear plant fallback.
	if ci.containsLower["inventorypotential"] && instanceName != "" {
		if v, ok := GetPropFloat32(obj, "mCurrentPotential"); ok {
			e.InventoryPotentialMap[instanceName] = v
		}
	}
	// Retain only what the second pass needs
	retained := false
	if shouldRetainForPostExtraction(typePath, len(obj.Properties) > 0) {
		e.RetainedObjects = append(e.RetainedObjects, obj)
		if instanceName != "" {
			e.RetainedObjectMap[instanceName] = obj
		}
		retained = true
	}

	// === Hypertubes ===
	if ci.containsPath["PipeHyper"] && !ci.containsPath["TJunction"] {
		// A spline segment is drawn as a polyline. Its actor position is merely
		// the spline origin, so also emitting a point entry there would stamp a
		// second, footprint-less marker part way along the tube. Only the
		// point-like members of this family (entrances, supports) get an entry.
		drawnAsPolyline := false
		if splineProp, ok := obj.Properties["mSplineData"]; ok {
			length := CalculateSplineLength(splineProp)
			if length > 0 {
				e.Hypertubes = append(e.Hypertubes, length)
			}
			// Extract control points + tangents for map before freeing
			if cps, zAvg := SplineControlPointsWorld(splineProp, obj.Header.Transform, obj.Header.NeedTransform); len(cps) >= 2 {
				pts := make([][2]float64, len(cps))
				tans := make([][4]float64, len(cps))
				for i, cp := range cps {
					pts[i] = cp.Pos
					tans[i] = [4]float64{cp.LeaveTan[0], cp.LeaveTan[1], cp.ArriveTan[0], cp.ArriveTan[1]}
				}
				e.PolylineEntries = append(e.PolylineEntries, MapPolylineEntry{
					Cat: MapCatHypertube, TypePath: typePath, Points: pts, Tangents: tans, Z: zAvg,
				})
				drawnAsPolyline = true
			}
			delete(obj.Properties, "mSplineData") // free memory
		}
		if !drawnAsPolyline {
			e.AddMapEntry(obj, MapCatHypertube)
		}
	}

	// === Hypertube Branch (TJunction) ===
	if ci.containsPath["TJunction"] {
		e.AddMapEntry(obj, MapCatHypertube)
	}

	// === Buffer Stop ===
	if ci.containsLower["railroadendstop"] {
		e.AddMapEntry(obj, MapCatRail)
	}

	// === Rails ===
	if ci.containsLower["railroadtrack"] {
		if splineProp, ok := obj.Properties["mSplineData"]; ok {
			length := CalculateSplineLength(splineProp)
			if length > 0 {
				e.Rails.TotalLength += length
				e.Rails.Count++
			}
			// Keep this track piece for the distance graph (skip stations/platforms/bridges)
			if instanceName != "" && !ci.containsLower["station"] && !ci.containsLower["platform"] && !ci.containsLower["bridge"] {
				seg := &TrackSegment{
					Length: length,
					Conn0:  instanceName + ".TrackConnection0",
					Conn1:  instanceName + ".TrackConnection1",
				}
				if obj.Header.NeedTransform {
					seg.Pos = [3]float64{
						float64(obj.Header.Transform.Translation[0]) / 100,
						float64(obj.Header.Transform.Translation[1]) / 100,
						float64(obj.Header.Transform.Translation[2]) / 100,
					}
				}
				// Grab the real endpoints while the spline is still around - it
				// gets dropped just below to save memory.
				if first, last, ok := SplineEndpointsWorld(splineProp, obj.Header.Transform, obj.Header.NeedTransform); ok {
					seg.Conn0Pos = first
					seg.Conn1Pos = last
					seg.HasEndpoints = true
				}
				e.rawTrackSegments[instanceName] = seg
			}
			// Extract control points + tangents for map before freeing
			// Skip integrated station tracks — they're part of stations, not buildable
			if !strings.Contains(typeLower, "integrated") {
				if cps, zAvg := SplineControlPointsWorld(splineProp, obj.Header.Transform, obj.Header.NeedTransform); len(cps) >= 2 {
					pts := make([][2]float64, len(cps))
					tans := make([][4]float64, len(cps))
					for i, cp := range cps {
						pts[i] = cp.Pos
						tans[i] = [4]float64{cp.LeaveTan[0], cp.LeaveTan[1], cp.ArriveTan[0], cp.ArriveTan[1]}
					}
					e.PolylineEntries = append(e.PolylineEntries, MapPolylineEntry{
						Cat: MapCatRail, TypePath: typePath, Points: pts, Tangents: tans, Z: zAvg,
					})
				}
			}
			delete(obj.Properties, "mSplineData") // free memory
		}
	}

	// === Track connection components (junctions between track pieces) ===
	if ci.containsLower["railroadtrackconnection"] {
		if instanceName != "" {
			conn := &TrackConn{}
			if connProp, ok := obj.Properties["mConnectedComponents"]; ok {
				var items []interface{}
				if arr, ok := connProp.Value.([]interface{}); ok {
					items = arr
				} else if m, ok := connProp.Value.(map[string]interface{}); ok {
					if arr, ok := m["items"].([]interface{}); ok {
						items = arr
					}
				}
				for _, item := range items {
					// The parser can give us either map[string]string or map[string]interface{}
					// depending on the property type, so we handle both
					if ref, ok := item.(map[string]string); ok {
						if pathName := ref["pathName"]; pathName != "" {
							conn.ConnectedTo = append(conn.ConnectedTo, pathName)
						}
					} else if ref, ok := item.(map[string]interface{}); ok {
						if pathName, _ := ref["pathName"].(string); pathName != "" {
							conn.ConnectedTo = append(conn.ConnectedTo, pathName)
						}
					}
				}
			}
			e.rawTrackConns[instanceName] = conn
		}
	}

	// === Railroad signals (block + path) ===
	if ci.containsLower["railroadblocksignal"] || ci.containsLower["railroadpathsignal"] {
		sig := &SignalInfo{}
		if ci.containsLower["pathsignal"] {
			sig.Type = "path"
			e.Rails.PathSignals++
		} else {
			sig.Type = "block"
			e.Rails.BlockSignals++
		}
		// mGuardedConnections - the connection this signal protects
		if prop, ok := obj.Properties["mGuardedConnections"]; ok {
			sig.Guarded = extractConnRefs(prop)
		}
		// mObservedConnections - where trains approach from
		if prop, ok := obj.Properties["mObservedConnections"]; ok {
			sig.Observed = extractConnRefs(prop)
		}
		if instanceName != "" {
			e.rawSignals[instanceName] = sig
		}
	}

	// === Trains ===
	if ci.containsLower["train"] {
		if ci.containsLower["locomotive"] {
			e.Trains.Locomotives++
		} else if ci.containsLower["wagon"] {
			e.Trains.Wagons++
		}
		if ci.containsLower["dockingstationliquid"] {
			e.Trains.FluidFreightPlatforms++
		} else if ci.containsLower["dockingstation"] {
			e.Trains.FreightPlatforms++
		} else if ci.containsLower["build_trainstation"] {
			e.Trains.Stations++
		}
	}

	// === Vehicle Paths (1.2+) ===
	if ci.containsLower["vehiclepath"] && !ci.containsLower["node"] && !ci.containsLower["network"] {
		if splineProp, ok := obj.Properties["mSplinePoints"]; ok {
			length := CalculateSplineLength(splineProp)
			if length > 0 {
				e.VehiclePaths = append(e.VehiclePaths, length)
			}
			delete(obj.Properties, "mSplinePoints") // free memory
		}
	}

	// === Drones ===
	if ci.containsLower["build_dronestation"] {
		e.Drones.Stations++
	}
	if ci.containsLower["build_freightplatform"] {
		e.Drones.FreightPlatforms++
	}
	if ci.containsLower["bp_dronetransport"] {
		e.Drones.Drones++
	}

	// === Belt heads ===
	if ci.containsLower["build_conveyorbelt"] && !ci.containsLower["lift"] {
		if m := beltMkRegex.FindStringSubmatch(typePath); m != nil {
			beltType := fmt.Sprintf("/Game/FactoryGame/Buildable/Factory/ConveyorBelt%s/Build_ConveyorBelt%s.Build_ConveyorBelt%s_C", m[1], m[1], m[1])
			e.BeltHeads[beltType]++
		}
	}

	// === Pipe pumps ===
	if ci.containsLower["build_pipepump"] {
		if m := pipePumpRegex.FindStringSubmatch(typePath); m != nil {
			var pumpType string
			if m[1] != "" {
				pumpType = "PipelineMk" + m[1]
			} else {
				pumpType = "Pipeline"
			}
			e.PipePumps[pumpType]++
		}
	}

	// === Vehicles ===
	if ci.containsLower["bp_fluidtruck"] {
		e.Vehicles.FluidTrucks++
	} else if ci.containsLower["bp_truck"] && !ci.containsLower["station"] {
		e.Vehicles.Trucks++
	}
	if ci.containsLower["bp_explorer"] && !ci.containsLower["station"] {
		e.Vehicles.Explorers++
	}
	if ci.containsLower["bp_tractor"] && !ci.containsLower["station"] {
		e.Vehicles.Tractors++
	}
	if ci.containsLower["bp_golfcart"] {
		e.Vehicles.Golfcarts++
	}
	if ci.containsLower["testa_bp_wb"] || ci.containsLower["cyberwagon"] {
		e.Vehicles.Cyberwagons++
	}
	if ci.containsLower["build_fluidtruckstation"] {
		e.Vehicles.FluidTruckStations++
	} else if ci.containsLower["build_truckstation"] {
		e.Vehicles.TruckStations++
	}

	// Collect wheeled vehicle objects for logistics (location, on-route status)
	if instanceName != "" {
		isWheeledVehicle := (ci.containsLower["bp_fluidtruck"] ||
			(ci.containsLower["bp_truck"] && !ci.containsLower["station"]) ||
			(ci.containsLower["bp_explorer"] && !ci.containsLower["station"]) ||
			(ci.containsLower["bp_tractor"] && !ci.containsLower["station"]) ||
			ci.containsLower["bp_golfcart"] ||
			ci.containsLower["testa_bp_wb"] ||
			ci.containsLower["cyberwagon"])
		if isWheeledVehicle {
			e.rawWheeledVehicles[instanceName] = obj
		}
	}

	// === Logistics: collect raw objects for later linking ===
	if instanceName != "" {
		switch {
		case ci.containsLower["bp_train.bp_train"]:
			e.rawTrains[instanceName] = obj
		case ci.containsLower["fgrailroadtimetable"]:
			e.rawTimetables[instanceName] = obj
		case ci.containsLower["fgtrainstationidentifier"]:
			e.rawTrainStationIDs[instanceName] = obj
		case ci.containsLower["build_trainstation.build_trainstation"]:
			e.rawTrainStations[instanceName] = obj
		case ci.containsLower["locomotive.bp_locomotive"]:
			e.rawLocomotives[instanceName] = obj
		case ci.containsLower["freightwagon.bp_freightwagon"]:
			e.rawWagons[instanceName] = obj
		case ci.containsLower["build_traindockingstation"]:
			e.rawTrainPlatforms[instanceName] = obj
		case ci.containsLower["fgwheeledvehicleidentifier"]:
			e.rawWheeledVehicleIDs[instanceName] = obj
		case ci.containsLower["fgwheeledvehicleinfo"]:
			e.rawWheeledVehicleInfos[instanceName] = obj
		case ci.containsLower["fgsavedwheeledvehiclepath"]:
			e.rawVehiclePaths[instanceName] = obj
		case ci.containsLower["fgdockingstationidentifier"] || ci.containsLower["fgdockingstationinfo"]:
			e.rawTruckStationIDs[instanceName] = obj
		case ci.containsLower["build_truckstation.build_truckstation"] || ci.containsLower["build_fluidtruckstation"]:
			e.rawTruckStations[instanceName] = obj
		case ci.containsLower["bp_dronetransport"]:
			e.rawDrones[instanceName] = obj
		case ci.containsLower["fgdronestationinfo"]:
			e.rawDroneStationInfos[instanceName] = obj
		case ci.containsLower["build_dronestation.build_dronestation"]:
			e.rawDroneStations[instanceName] = obj
		}
	}

	// Structures (must be before instanceName check - many structures have empty instanceName)
	if extractStructures && e.Structures != nil {
		e.processStructure(obj, typePath, typeLower, instanceName)
		// Collect map entries for individual structure objects (not lightweight).
		// Lightweight buildable transforms are extracted by the parser and
		// processed in processLightweightBuildables.
		if obj.Header != nil && obj.Header.NeedTransform {
			cat := classifyStructure(typePath)
			if cat != "" && !isInternalSystem(typePath) {
				mapCat := MapCatStructure
				if cat == "foundations" {
					mapCat = MapCatFoundation
				}
				e.AddMapEntry(obj, mapCat)
			}
		}
	}

	// Lightweight buildable subsystem - packed arrays of structures
	if extractStructures && e.Structures != nil {
		if special, ok := obj.Properties["__special__"]; ok {
			if special.Type == "Special" {
				if sp, ok := special.Value.(map[string]interface{}); ok {
					if spType, ok := sp["type"].(string); ok && spType == "BuildableSubsystemSpecialProperties" {
						e.processLightweightBuildables(sp)
					}
				}
			}
		}
	}

	// ConveyorChainActor - extract belt/lift lengths from chain data
	if ci.containsLower["conveyorchain"] {
		if special, ok := obj.Properties["__special__"]; ok {
			if special.Type == "Special" {
				if sp, ok := special.Value.(map[string]interface{}); ok {
					if spType, ok := sp["type"].(string); ok && spType == "ConveyorChainActorSpecialProperties" {
						e.processConveyorChain(sp)
					}
				}
			}
		}
	}

	// === Connection components (extracted during streaming, not retained) ===
	if instanceName != "" {
		if ci.containsPath["FGFactoryConnectionComponent"] {
			extractBeltConnection(obj, instanceName, e.BuildingConnections)
		} else if ci.containsPath["FGPipeConnectionFactory"] {
			extractPipeConnection(obj, instanceName, e.BuildingConnections)
		}
	}

	// === Objects with instanceName ===
	if instanceName != "" {
		// Buildings
		if ci.isBuilding {
			if ci.isInternalSystem {
				// Skip internal game systems
			} else if ci.containsLower["vehiclepathnode"] {
				// Skip
			} else if ci.containsLower["truck"] || ci.containsLower["drone"] ||
				(ci.containsLower["dock"] && !ci.containsLower["train"]) {
				e.Others[typePath]++
			} else {
				if ci.containsLower["train"] || ci.containsLower["rail"] ||
					ci.containsLower["switch"] || ci.containsLower["signal"] {
					if !ci.containsLower["truck"] && !ci.containsLower["drone"] {
						e.RailComponents[typePath]++
					}
				}
				e.Buildings[typePath]++
				e.BuildingInstances[instanceName] = typePath
			}
		}

		// Blueprints and subsystems
		if ci.isBlueprintOrSubsystem {
			e.Blueprints[typePath]++
		}

		// Pets - count Lizard Doggos (Char_SpaceRabbit) and track tamed ones
		if ci.containsLower["char_spacerabbit"] {
			e.Pets.TotalDoggos++
			if v, ok := GetPropBool(obj, "mTamed"); ok && v {
				e.Pets.TamedDoggos++
			}
		}

		// Elevators
		if ci.containsLower["elevator"] {
			if ci.containsPath["Build_Elevator"] && !ci.containsLower["floorstop"] && !ci.containsLower["cabin"] {
				length := ExtractLength(obj)
				e.Elevators = append(e.Elevators, length)
			} else if ci.containsLower["floorstop"] || ci.containsLower["cabin"] {
				e.ElevatorComponents[typePath] = append(e.ElevatorComponents[typePath], instanceName)
			}
		}

		// Mergers and splitters
		// Lift-variant splitters/mergers (e.g. Build_ConveyorAttachmentSplitterLift_C)
		// are normalized to their base type so they don't appear as separate
		// entries in the map type list.
		normalizedType := typePath
		if ci.containsLower["lift"] {
			normalizedType = strings.ReplaceAll(normalizedType, "SplitterSmartLift", "SplitterSmart")
			normalizedType = strings.ReplaceAll(normalizedType, "SplitterLift", "Splitter")
			normalizedType = strings.ReplaceAll(normalizedType, "MergerLift", "Merger")
		}
		if ci.containsLower["merger"] {
			e.Mergers++
			e.AddMapEntryWithType(obj, MapCatBelt, normalizedType)
		} else if ci.containsLower["programmable"] {
			e.ProgrammableSplitters++
			e.AddMapEntryWithType(obj, MapCatBelt, normalizedType)
		} else if ci.containsLower["smart"] {
			e.SmartSplitters++
			e.AddMapEntryWithType(obj, MapCatBelt, normalizedType)
		} else if ci.containsLower["splitter"] {
			e.Splitters++
			e.AddMapEntryWithType(obj, MapCatBelt, normalizedType)
		}

		// Power poles
		if ci.containsLower["powerpole"] || ci.containsLower["powerpolewall"] ||
			ci.containsLower["wall outlet"] || ci.containsLower["powertower"] {
			e.PowerPoles++
			e.PowerPoleTypes[typePath]++
			e.AddMapEntry(obj, MapCatPowerPole)
		}

		// Hypertube junctions/entrances
		if ci.containsLower["hypertube"] && (ci.containsLower["junction"] ||
			ci.containsLower["entrance"] || ci.containsLower["support"]) {
			e.HypertubeJunctions++
		}

		// Passthroughs
		if ci.containsLower["passthrough"] {
			if ci.containsLower["lift"] {
				e.LiftFloorHoles++
			} else if ci.containsLower["belt"] {
				e.BeltWallHoles++
			} else if ci.containsLower["pipe"] || ci.containsLower["pipeline"] {
				e.PipeFloorHoles++
			}
		}

		// Production (pipes, tanks)
		if ci.isProduction {
			length := ExtractLength(obj)
			// Extract control points + tangents for map before freeing
			drawnAsPolyline := false
			if splineProp, ok := obj.Properties["mSplineData"]; ok {
				if cps, zAvg := SplineControlPointsWorld(splineProp, obj.Header.Transform, obj.Header.NeedTransform); len(cps) >= 2 {
					// Conveyor lifts are vertical — all spline points collapse to
					// the same X/Y in 2D. Skip the zero-length polyline so the
					// lift gets a point entry instead.
					zeroLen := true
					for i := 1; i < len(cps); i++ {
						if cps[i].Pos[0] != cps[0].Pos[0] || cps[i].Pos[1] != cps[0].Pos[1] {
							zeroLen = false
							break
						}
					}
					if !zeroLen {
						mapCat := MapCatBelt
						if ci.containsLower["pipeline"] {
							mapCat = MapCatPipe
						}
						pts := make([][2]float64, len(cps))
						tans := make([][4]float64, len(cps))
						for i, cp := range cps {
							pts[i] = cp.Pos
							tans[i] = [4]float64{cp.LeaveTan[0], cp.LeaveTan[1], cp.ArriveTan[0], cp.ArriveTan[1]}
						}
						e.PolylineEntries = append(e.PolylineEntries, MapPolylineEntry{
							Cat: mapCat, TypePath: typePath, Points: pts, Tangents: tans, Z: zAvg,
						})
						drawnAsPolyline = true
					}
				}
			}
			// Free spline data after length extraction
			delete(obj.Properties, "mSplineData")
			// As with hypertubes and rails, a run already drawn as a polyline
			// must not also get a point entry at its spline origin.
			if ci.containsLower["conveyor"] && (ci.containsLower["belt"] || ci.containsLower["lift"]) &&
				!ci.containsLower["splitter"] && !ci.containsLower["merger"] {
				// skip - belts/lifts handled via ConveyorChainActor
				if !drawnAsPolyline {
					e.AddMapEntry(obj, MapCatBelt)
				}
			} else if ci.containsLower["pipeline"] && !ci.containsLower["pump"] &&
				!ci.containsLower["support"] && !ci.containsLower["junction"] &&
				!ci.containsLower["connection"] && !ci.containsLower["fgpipe"] &&
				!ci.containsLower["flowindicator"] {
				if !drawnAsPolyline {
					e.AddMapEntry(obj, MapCatPipe)
				}
				if _, ok := e.Pipes[typePath]; !ok {
					e.Pipes[typePath] = &PipeTypeStats{}
				}
				e.Pipes[typePath].Count++
				e.Pipes[typePath].TotalLength += length
			} else if ci.containsLower["pipeline"] && ci.containsLower["junction"] {
				if !drawnAsPolyline {
					e.AddMapEntry(obj, MapCatPipe)
				}
			} else if ci.containsLower["industrialtank"] || ci.containsLower["pipestoragetank"] {
				// Fluid tanks - counted as buildings, not pipe segments
				e.Others[typePath]++
			}
		} else if ci.containsLower["powerline"] {
			// Power lines
			if v, ok := GetPropFloat32(obj, "mCachedLength"); ok && v > 0 {
				e.PowerLines = append(e.PowerLines, float64(v)/100)
			}
			e.PowerGrid.LineCount++
			// Extract power line endpoints from special properties
			if special, ok := obj.Properties["__special__"]; ok {
				if sp, ok := special.Value.(map[string]interface{}); ok {
					if spType, _ := sp["type"].(string); spType == "PowerLineSpecialProperties" {
						var srcTf, tgtTf map[string]float32
						if v, ok := sp["sourceTranslation"].(map[string]float32); ok {
							srcTf = v
						}
						if v, ok := sp["targetTranslation"].(map[string]float32); ok {
							tgtTf = v
						}
						if srcTf != nil && tgtTf != nil {
							pts := [][2]float64{
								{float64(srcTf["x"]) / 100, float64(srcTf["y"]) / 100},
								{float64(tgtTf["x"]) / 100, float64(tgtTf["y"]) / 100},
							}
							polyZ := (float64(srcTf["z"]) + float64(tgtTf["z"])) / 200
							e.PolylineEntries = append(e.PolylineEntries, MapPolylineEntry{
								Cat: MapCatPowerPole, TypePath: typePath, Points: pts, Z: polyZ,
							})
						} else {
							// Translations not available, store refs for deferred resolution
							srcRef, _ := sp["source"].(map[string]string)
							tgtRef, _ := sp["target"].(map[string]string)
							if srcRef != nil && tgtRef != nil {
								e.PowerLineRefs = append(e.PowerLineRefs, PowerLineRef{
									TypePath: typePath,
									SrcPath:  srcRef["pathName"],
									TgtPath:  tgtRef["pathName"],
								})
							}
						}
					}
				}
			}
		} else if ci.containsLower["fgpowercircuit"] {
			// Power circuits
			if v, ok := GetPropInt32(obj, "mCircuitID"); ok {
				componentCount := 0
				var components []string
				if arr, ok2 := GetPropArray(obj, "mComponents"); ok2 {
					componentCount = len(arr)
					components = make([]string, 0, len(arr))
					for _, it := range arr {
						switch m := it.(type) {
						case map[string]string:
							if pn := m["pathName"]; pn != "" {
								components = append(components, pn)
							}
						case map[string]interface{}:
							if pn, _ := m["pathName"].(string); pn != "" {
								components = append(components, pn)
							}
						}
					}
				}
				e.PowerGrid.Circuits = append(e.PowerGrid.Circuits, CircuitInfo{
					ID:             v,
					ComponentCount: componentCount,
					Components:     components,
				})
			}
		}
	} // end if instanceName != ""

	// Creatures killed + Consumables consumed - read from FGStatisticsSubsystem
	if ci.containsLower["statisticssubsystem"] {
		if ckProp, ok := obj.Properties["mCreaturesKilledCount"]; ok {
			if mp, ok := ckProp.Value.(map[string]interface{}); ok {
				if entries, ok := mp["entries"].([]interface{}); ok {
					for _, entry := range entries {
						if em, ok := entry.(map[string]interface{}); ok {
							var pathName string
							// ObjectProperty keys are map[string]string
							if keyMap, ok := em["key"].(map[string]string); ok {
								pathName = keyMap["pathName"]
							} else if keyMap, ok := em["key"].(map[string]interface{}); ok {
								pathName, _ = keyMap["pathName"].(string)
							}
							var count int
							switch v := em["value"].(type) {
							case int32:
								count = int(v)
							case int:
								count = v
							case int64:
								count = int(v)
							}
							if pathName != "" && count > 0 {
								e.CreatureKills.ByType[pathName] = count
								e.CreatureKills.Total += count
							}
						}
					}
				}
			}
		}
		if ccProp, ok := obj.Properties["mConsumablesConsumedCount"]; ok {
			if mp, ok := ccProp.Value.(map[string]interface{}); ok {
				if entries, ok := mp["entries"].([]interface{}); ok {
					for _, entry := range entries {
						if em, ok := entry.(map[string]interface{}); ok {
							var pathName string
							if keyMap, ok := em["key"].(map[string]string); ok {
								pathName = keyMap["pathName"]
							} else if keyMap, ok := em["key"].(map[string]interface{}); ok {
								pathName, _ = keyMap["pathName"].(string)
							}
							var count int
							switch v := em["value"].(type) {
							case int32:
								count = int(v)
							case int:
								count = v
							case int64:
								count = int(v)
							}
							if pathName != "" && count > 0 {
								e.ConsumablesConsumed.ByType[pathName] = count
								e.ConsumablesConsumed.Total += count
							}
						}
					}
				}
			}
		}
	}

	// Extract item pickups (scattered items on the map)
	if ci.containsLower["itempickup"] && obj.Header.NeedTransform {
		if pickup, ok := extractPickupItem(obj); ok {
			e.Pickups = append(e.Pickups, pickup)
		}
	}

	// Strip non-essential props from retained objects
	if retained && obj.Properties != nil {
		for key := range obj.Properties {
			if !essentialPostExtractionProps[key] {
				delete(obj.Properties, key)
			}
		}
	}
}
