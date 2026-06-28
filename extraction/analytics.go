package extraction

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"satisfacts/parser"
)

// AnalyticsData holds all calculated analytics metrics.
type AnalyticsData struct {
	Power               map[string]interface{}            `json:"power"`
	Production          map[string]interface{}            `json:"production"`
	Manufacturing       map[string]interface{}            `json:"manufacturing"`
	Efficiency          map[string]interface{}            `json:"efficiency"`
	Storage             map[string]interface{}            `json:"storage"`
	Transport           map[string]interface{}            `json:"transport"`
	Map                 map[string]interface{}            `json:"map"`
	Collectibles        map[string]interface{}            `json:"collectibles"`
	ClockSpeedDistribution map[string]map[string]int      `json:"clockSpeedDistribution"`
	ProductionBoostDistribution map[string]map[string]int  `json:"productionBoostDistribution"`
	ClockSpeedSloopCounts map[string]map[string]int       `json:"clockSpeedSloopCounts"`
	Inventory           map[string]interface{}            `json:"inventory"`
	Trains              map[string]interface{}            `json:"trains"`
	Drones              map[string]interface{}            `json:"drones"`
	Vehicles            map[string]interface{}            `json:"vehicles"`
	GameProgression     map[string]interface{}            `json:"gameProgression"`
	NuclearWaste        map[string]interface{}            `json:"nuclearWaste"`
}

// AnalyticsInput holds all data needed for analytics calculation.
type AnalyticsInput struct {
	Buildings          map[string]int
	ClockSpeeds        map[string]float32
	ProductionBoosts   map[string]float32
	BuildingTypes      map[string]string
	BuildingRecipes    map[string]string
	BuildingInventories map[string]BuildingInventoryRefs
	BuildingConnections map[string]BuildingConnection
	InventoryContents  map[string]int
	PowerConsumption   PowerConsumptionData
	GeneratorPower     GeneratorPowerData
	NuclearWaste       NuclearWasteData
	APABuildingCount   int
	APAFueledCount     int
	MinerPurities      map[string]string
	MinerResources     map[string]string
	GeothermalPurities map[string]string
	FrackingFluidTypes map[string]string
	FrackingPurities   map[string]string
	Collectibles       CollectiblesData
	CollectiblesFound  CollectiblesFoundData
	CollectedCollectibles CollectedCollectiblesData
	Pickups            []PickupItem
	GameProgression    GameProgressionData
	SaveVersion        int
	// Transport data
	Belts              map[string]*BeltTypeStats
	BeltHeads          map[string]int
	Lifts              map[string]*BeltTypeStats
	Pipes              map[string]*PipeTypeStats
	PowerLines         []float64
	Hypertubes         []float64
	Elevators          []float64
	VehiclePaths       []float64
	Rails              RailStats
	Trains             TrainStats
	Drones             DroneStats
	Vehicles           VehicleStats
	// Structures
	Structures         *StructureData
	// Resolved inventories
	ResolvedInventories map[string]ResolvedInventory
	// Retained objects for position-based calculations
	RetainedObjects     []*parser.SaveObject
	// BuildingFacts for position-based calculations
	BuildingFacts       map[string]*BuildingFacts
}

// CalculateAnalytics computes all analytics from the extracted data.
func CalculateAnalytics(input AnalyticsInput) *AnalyticsData {
	manufacturing := calculateManufacturingAnalytics(input)
	production := calculateProductionAnalytics(input)

	return &AnalyticsData{
		Power:               calculatePowerAnalytics(input),
		Production:          production,
		Manufacturing:       manufacturing,
		Efficiency:          calculateEfficiencyAnalytics(input, manufacturing, production),
		Storage:             calculateStorageAnalytics(input),
		Transport:           calculateTransportAnalytics(input),
		Map:                 calculateMapAnalytics(input),
		Collectibles:        calculateCollectiblesAnalytics(input),
		ClockSpeedDistribution: calculateClockSpeedDistribution(input),
	ProductionBoostDistribution: calculateProductionBoostDistribution(input),
	ClockSpeedSloopCounts: calculateClockSpeedSloopCounts(input),
		Inventory:           calculateInventoryAnalytics(input),
		Trains:              calculateTrainAnalytics(input),
		Drones:              calculateDroneAnalytics(input),
		Vehicles:            calculateVehicleAnalytics(input),
		GameProgression:     calculateGameProgressionAnalytics(input),
		NuclearWaste:        calculateNuclearWasteAnalytics(input),
	}
}

// --- Simple Analytics ---

func calculateClockSpeedDistribution(input AnalyticsInput) map[string]map[string]int {
	if input.ClockSpeeds == nil || input.BuildingTypes == nil {
		return nil
	}

	dist := make(map[string]map[string]int)
	for inst, speed := range input.ClockSpeeds {
		typePath, ok := input.BuildingTypes[inst]
		if !ok {
			continue
		}
		speedPercent := int(math.Round(float64(speed)))
		if speedPercent > 250 {
			speedPercent = 250
		}
		if dist[typePath] == nil {
			dist[typePath] = make(map[string]int)
		}
		dist[typePath][intToStr(speedPercent)]++
	}
	return dist
}

// calculateClockSpeedSloopCounts tracks buildings at each clock speed that
// also have a somersloop inserted. Returns typePath -> speed% -> sloopedCount.
func calculateClockSpeedSloopCounts(input AnalyticsInput) map[string]map[string]int {
	if input.ClockSpeeds == nil || input.BuildingTypes == nil || input.ProductionBoosts == nil {
		return nil
	}

	dist := make(map[string]map[string]int)
	for inst, speed := range input.ClockSpeeds {
		typePath, ok := input.BuildingTypes[inst]
		if !ok {
			continue
		}
		// Only count if this building has a production boost (sloop)
		if _, hasBoost := input.ProductionBoosts[inst]; !hasBoost {
			continue
		}
		speedPercent := int(math.Round(float64(speed)))
		if speedPercent > 250 {
			speedPercent = 250
		}
		if dist[typePath] == nil {
			dist[typePath] = make(map[string]int)
		}
		dist[typePath][intToStr(speedPercent)]++
	}
	return dist
}

// calculateProductionBoostDistribution groups buildings by their somersloop boost.
// Extractors use the multiplier (e.g. 2.0 = doubled output); production buildings
// use the somersloop count.
func calculateProductionBoostDistribution(input AnalyticsInput) map[string]map[string]int {
	if input.ProductionBoosts == nil || input.BuildingTypes == nil {
		return nil
	}

	dist := make(map[string]map[string]int)
	for inst, boost := range input.ProductionBoosts {
		typePath, ok := input.BuildingTypes[inst]
		if !ok {
			continue
		}
		// Extractors: boost is a multiplier; production: sloop count.
		var key string
		typeLower := strings.ToLower(typePath)
		isExtractor := strings.Contains(typeLower, "miner") || strings.Contains(typeLower, "waterpump") ||
			strings.Contains(typeLower, "oilpump") || strings.Contains(typeLower, "fracking") ||
			strings.Contains(typeLower, "resourcewell")
		if isExtractor {
			key = fmt.Sprintf("%.1fx", boost)
		} else {
			key = intToStr(int(boost)) + " sloop"
			if int(boost) != 1 {
				key += "s"
			}
		}
		if dist[typePath] == nil {
			dist[typePath] = make(map[string]int)
		}
		dist[typePath][key]++
	}
	return dist
}

func calculateCollectiblesAnalytics(input AnalyticsInput) map[string]interface{} {
	// Group pickups by item type
	pickupByType := map[string]interface{}{}
	totalPickupItems := 0
	availablePickups := 0
	for _, p := range input.Pickups {
		if p.Collected {
			continue
		}
		availablePickups++
		totalPickupItems += p.NumItems
		pos := map[string]string{
			"X": formatLocationNum(float64(p.Position[0]) / 100.0),
			"Y": formatLocationNum(float64(p.Position[1]) / 100.0),
			"Z": formatLocationNum(float64(p.Position[2]) / 100.0),
		}
		if existing, ok := pickupByType[p.ItemName]; ok {
			entry := existing.(map[string]interface{})
			entry["count"] = entry["count"].(int) + 1
			entry["total_items"] = entry["total_items"].(int) + p.NumItems
			entry["pickups"] = append(entry["pickups"].([]map[string]interface{}), map[string]interface{}{
				"num_items": p.NumItems,
				"position":  pos,
			})
		} else {
			pickupByType[p.ItemName] = map[string]interface{}{
				"item_class":  p.ItemClass,
				"count":       1,
				"total_items": p.NumItems,
				"pickups": []map[string]interface{}{{
					"num_items": p.NumItems,
					"position":  pos,
				}},
			}
		}
	}

	return map[string]interface{}{
		"tapes": map[string]interface{}{
			"collected":       input.Collectibles.Tapes,
			"total_collected": len(input.Collectibles.Tapes),
		},
		"drop_pods": map[string]interface{}{
			"collected": input.Collectibles.DropPods,
		},
		"total_pickups": input.Collectibles.TotalPickups,
		"item_pickups": map[string]interface{}{
			"available_count":    availablePickups,
			"total_items":        totalPickupItems,
			"by_type":            pickupByType,
		},
		"world_collectibles": calculateWorldCollectibles(input.CollectiblesFound, input.CollectedCollectibles),
	}
}

// calculateWorldCollectibles computes collected vs remaining for slugs, somersloops,
// mercer spheres, and crash sites using the collectables1 list from the TOC blob
// for accurate collected counts, and the world data reference for totals.
func calculateWorldCollectibles(found CollectiblesFoundData, collected CollectedCollectiblesData) map[string]interface{} {
	if collectiblesWorldLoaded == nil {
		return nil
	}

	// Power slugs: collected from collectables1, remaining = total - collected
	blueTotal := len(collectiblesWorldLoaded.PowerSlugs.Blue)
	blueCollected := len(collected.PowerSlugsBlue)
	yellowTotal := len(collectiblesWorldLoaded.PowerSlugs.Yellow)
	yellowCollected := len(collected.PowerSlugsYellow)
	purpleTotal := len(collectiblesWorldLoaded.PowerSlugs.Purple)
	purpleCollected := len(collected.PowerSlugsPurple)
	slugTotal := blueTotal + yellowTotal + purpleTotal
	slugCollected := blueCollected + yellowCollected + purpleCollected

	// Somersloops
	sloopTotal := len(collectiblesWorldLoaded.Somersloops)
	sloopCollected := len(collected.Somersloops)

	// Mercer spheres
	sphereTotal := len(collectiblesWorldLoaded.MercerSpheres)
	sphereCollected := len(collected.MercerSpheres)

	// Crash sites: collectables1 tracks dismantled crash sites.
	// Crash sites still in the save can be opened or unopened.
	crashTotal := len(collectiblesWorldLoaded.CrashSites)
	crashDismantled := len(collected.CrashSites)
	crashOpened := len(found.CrashSiteOpened)
	crashUnopened := len(found.CrashSiteUnopened)
	// "collected" = dismantled + opened (hard drive grabbed in both cases)
	crashCollected := crashDismantled + crashOpened

	return map[string]interface{}{
		"power_slugs": map[string]interface{}{
			"total":     slugTotal,
			"collected": slugCollected,
			"remaining": slugTotal - slugCollected,
			"blue": map[string]interface{}{
				"total":     blueTotal,
				"collected": blueCollected,
				"remaining": blueTotal - blueCollected,
			},
			"yellow": map[string]interface{}{
				"total":     yellowTotal,
				"collected": yellowCollected,
				"remaining": yellowTotal - yellowCollected,
			},
			"purple": map[string]interface{}{
				"total":     purpleTotal,
				"collected": purpleCollected,
				"remaining": purpleTotal - purpleCollected,
			},
		},
		"somersloops": map[string]interface{}{
			"total":     sloopTotal,
			"collected": sloopCollected,
			"remaining": sloopTotal - sloopCollected,
		},
		"mercer_spheres": map[string]interface{}{
			"total":     sphereTotal,
			"collected": sphereCollected,
			"remaining": sphereTotal - sphereCollected,
		},
		"crash_sites": map[string]interface{}{
			"total":      crashTotal,
			"opened":     crashOpened,
			"unopened":   crashUnopened,
			"dismantled": crashDismantled,
			"collected":  crashCollected,
			"remaining":  crashTotal - crashCollected,
		},
	}
}

func calculateInventoryAnalytics(input AnalyticsInput) map[string]interface{} {
	if input.InventoryContents == nil || len(input.InventoryContents) == 0 {
		return nil
	}

	totalItems := 0
	for _, count := range input.InventoryContents {
		totalItems += count
	}

	// Sort items by count (descending)
	type kv struct {
		key   string
		value int
	}
	var sorted []kv
	for k, v := range input.InventoryContents {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].value > sorted[j].value })

	itemsByCount := make(map[string]int)
	for _, item := range sorted {
		itemsByCount[item.key] = item.value
	}

	// Categorize items
	categories := map[string]map[string]int{
		"ores":       {},
		"ingots":     {},
		"components": {},
		"equipment":  {},
		"other":      {},
	}
	for name, count := range input.InventoryContents {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "ore") || strings.Contains(lower, "raw") {
			categories["ores"][name] = count
		} else if strings.Contains(lower, "ingot") || strings.Contains(lower, "plate") {
			categories["ingots"][name] = count
		} else if strings.Contains(lower, "screw") || strings.Contains(lower, "rod") ||
			strings.Contains(lower, "beam") || strings.Contains(lower, "encased") {
			categories["components"][name] = count
		} else if strings.Contains(lower, "gun") || strings.Contains(lower, "xeno") ||
			strings.Contains(lower, "rebar") {
			categories["equipment"][name] = count
		} else {
			categories["other"][name] = count
		}
	}

	return map[string]interface{}{
		"total_items":       totalItems,
		"items_by_count":    itemsByCount,
		"categories":        categories,
		"unique_item_types": len(input.InventoryContents),
	}
}

func calculateMapAnalytics(input AnalyticsInput) map[string]interface{} {
	if mapDataLoaded == nil {
		return nil
	}

	foundationCount := 0
	if input.Structures != nil && input.Structures.Details != nil {
		if foundations, ok := input.Structures.Details["foundations"]; ok {
			for _, count := range foundations {
				foundationCount += count
			}
		}
	}

	totalBuildings := 0
	for _, count := range input.Buildings {
		totalBuildings += count
	}

	// Calculate bounding box from all BuildingFacts with positions
	minX, maxX := math.MaxFloat32, -math.MaxFloat32
	minY, maxY := math.MaxFloat32, -math.MaxFloat32
	positionCount := 0
	for _, bf := range input.BuildingFacts {
		if bf == nil || !bf.HasTransform {
			continue
		}
		x, y := float64(bf.Translation[0]), float64(bf.Translation[1])
		if x < minX { minX = x }
		if x > maxX { maxX = x }
		if y < minY { minY = y }
		if y > maxY { maxY = y }
		positionCount++
	}

	// Convert cm to m, calculate area from bounding box
	totalAreaM2 := 0.0
	totalAreaKm2 := 0.0
	buildingDensity := 0.0
	if positionCount > 0 && maxX > minX && maxY > minY {
		widthM := (maxX - minX) / 100.0
		lengthM := (maxY - minY) / 100.0
		totalAreaM2 = widthM * lengthM
		totalAreaKm2 = totalAreaM2 / 1000000.0
		if totalAreaKm2 > 0 {
			buildingDensity = float64(totalBuildings) / totalAreaKm2
		}
	}

	return map[string]interface{}{
		"foundation_count": foundationCount,
		"total_area_m2":    totalAreaM2,
		"total_area_km2":   totalAreaKm2,
		"total_buildings":  totalBuildings,
		"building_density": buildingDensity,
		"foundation_size":  mapDataLoaded.Foundations,
		"bounding_box": map[string]interface{}{
			"width_m":  (maxX - minX) / 100.0,
			"length_m": (maxY - minY) / 100.0,
		},
	}
}

func calculateStorageAnalytics(input AnalyticsInput) map[string]interface{} {
	if storageDataLoaded == nil {
		return nil
	}

	var storageMk1, industrialContainers, dimensionalDepots, storageIntegrated, totalSlots int

	for btype, count := range input.Buildings {
		lower := strings.ToLower(btype)
		if strings.Contains(lower, "storagecontainer") && !strings.Contains(lower, "industrial") {
			if strings.Contains(lower, "mk1") || strings.Contains(lower, "mk_1") {
				storageMk1 = count
				totalSlots += count * storageDataLoaded.Containers["StorageContainer"].Slots
			} else if strings.Contains(lower, "mk2") || strings.Contains(lower, "mk_2") {
				industrialContainers = count
				totalSlots += count * storageDataLoaded.Containers["StorageContainer_Mk2"].Slots
			} else {
				industrialContainers = count
				totalSlots += count * storageDataLoaded.Containers["StorageContainer_Mk2"].Slots
			}
		} else if strings.Contains(lower, "industrialstorage") {
			industrialContainers = count
			totalSlots += count * storageDataLoaded.Containers["IndustrialStorageContainer"].Slots
		} else if strings.Contains(lower, "centralstorage") {
			dimensionalDepots = count
			totalSlots += count * storageDataLoaded.Containers["CentralStorage"].Slots
		} else if strings.Contains(lower, "storageintegrated") {
			storageIntegrated = count
			totalSlots += count * storageDataLoaded.Containers["StorageIntegrated"].Slots
		}
	}

	return map[string]interface{}{
		"containers": map[string]interface{}{
			"standard":          map[string]interface{}{"count": storageMk1, "slots": storageDataLoaded.Containers["StorageContainer"].Slots},
			"industrial":        map[string]interface{}{"count": industrialContainers, "slots": storageDataLoaded.Containers["StorageContainer_Mk2"].Slots},
			"dimensional_depot": map[string]interface{}{"count": dimensionalDepots, "slots": storageDataLoaded.Containers["CentralStorage"].Slots},
			"integrated":        map[string]interface{}{"count": storageIntegrated, "slots": storageDataLoaded.Containers["StorageIntegrated"].Slots},
		},
		"total_slots": totalSlots,
	}
}

func calculateTransportAnalytics(input AnalyticsInput) map[string]interface{} {

	// Infrastructure lengths
	beltLength := 0.0
	for _, belt := range input.Belts {
		beltLength += belt.TotalLength
	}
	beltLengthKm := beltLength / 1000

	liftLength := 0.0
	for _, lift := range input.Lifts {
		liftLength += lift.TotalLength
	}
	liftLengthKm := liftLength / 1000

	pipeLength := 0.0
	for _, pipe := range input.Pipes {
		pipeLength += pipe.TotalLength
	}
	pipeLengthKm := pipeLength / 1000

	powerLineLength := 0.0
	for _, l := range input.PowerLines {
		powerLineLength += l
	}
	powerLineLengthKm := powerLineLength / 1000

	hypertubeLength := 0.0
	for _, l := range input.Hypertubes {
		hypertubeLength += l
	}
	hypertubeLengthKm := hypertubeLength / 1000

	vehiclePathLength := 0.0
	for _, l := range input.VehiclePaths {
		vehiclePathLength += l
	}
	vehiclePathLengthKm := vehiclePathLength / 1000

	trainLengthKm := input.Rails.TotalLength / 1000

	totalInfraKm := beltLengthKm + liftLengthKm + pipeLengthKm + powerLineLengthKm + hypertubeLengthKm + trainLengthKm + vehiclePathLengthKm

	return map[string]interface{}{
		"total_infrastructure_km": totalInfraKm,
		"infrastructure_breakdown": map[string]interface{}{
			"belts_km":       beltLengthKm,
			"lifts_km":       liftLengthKm,
			"pipes_km":       pipeLengthKm,
			"power_lines_km": powerLineLengthKm,
			"hypertubes_km":    hypertubeLengthKm,
			"trains_km":        trainLengthKm,
			"vehicle_paths_km": vehiclePathLengthKm,
		},
	}
}

func calculateTrainAnalytics(input AnalyticsInput) map[string]interface{} {
	loc := input.Trains.Locomotives
	wagons := input.Trains.Wagons
	stations := input.Trains.Stations
	railCount := input.Rails.Count

	if loc == 0 && wagons == 0 && stations == 0 && railCount == 0 {
		return nil
	}

	return map[string]interface{}{
		"locomotives":     loc,
		"wagons":          wagons,
		"stations":        stations,
		"total_vehicles":  loc + wagons,
		"rail_count":      railCount,
		"rail_length_km":  input.Rails.TotalLength / 1000,
	}
}

func calculateDroneAnalytics(input AnalyticsInput) map[string]interface{} {
	stations := input.Drones.Stations
	freight := input.Drones.FreightPlatforms
	if stations == 0 && freight == 0 {
		return nil
	}
	return map[string]interface{}{
		"stations":             stations,
		"freight_platforms":    freight,
		"total_infrastructure": stations + freight,
	}
}

func calculateVehicleAnalytics(input AnalyticsInput) map[string]interface{} {
	trucks := input.Vehicles.Trucks
	explorers := input.Vehicles.Explorers
	tractors := input.Vehicles.Tractors
	cykles := input.Vehicles.Cykles
	total := trucks + explorers + tractors + cykles
	if total == 0 {
		return nil
	}
	return map[string]interface{}{
		"trucks":    trucks,
		"explorers": explorers,
		"tractors":  tractors,
		"cykles":    cykles,
		"total":     total,
	}
}

func calculateEfficiencyAnalytics(input AnalyticsInput, manufacturing, production map[string]interface{}) map[string]interface{} {
	manuf := manufacturing
	if manuf == nil {
		manuf = map[string]interface{}{}
	}

	recipeEfficiency := map[string]interface{}{}
	if activeProd, ok := manuf["active_production"].(map[string]interface{}); ok {
		for _, rd := range activeProd {
			recipeData, ok := rd.(map[string]interface{})
			if !ok {
				continue
			}
			totalActualOutput := 0.0
			totalTheoreticalMax := 0.0
			if totalOutput, ok := recipeData["total_output"].(map[string]interface{}); ok {
				for _, out := range totalOutput {
					if o, ok := out.(map[string]interface{}); ok {
						if tpm, ok := o["total_per_minute"].(float64); ok {
							totalActualOutput += tpm
						}
						if br, ok := o["base_rate"].(float64); ok {
							if cnt, ok := recipeData["count"].(int); ok {
								totalTheoreticalMax += br * float64(cnt)
							}
						}
					}
				}
			}

			efficiency := 0.0
			if totalTheoreticalMax > 0 {
				efficiency = (totalActualOutput / totalTheoreticalMax) * 100
			}

			recipeName, _ := recipeData["recipe_name"].(string)
			recipeEfficiency[recipeName] = map[string]interface{}{
				"efficiency":      math.Round(efficiency*10) / 10,
				"actual_output":   math.Round(totalActualOutput*10) / 10,
				"theoretical_max": math.Round(totalTheoreticalMax*10) / 10,
				"building_count":  recipeData["count"],
				"avg_clock_speed": recipeData["avg_clock_speed"],
			}
		}
	}

	return map[string]interface{}{
		"recipe_efficiency": recipeEfficiency,
	}
}

func calculateNuclearWasteAnalytics(input AnalyticsInput) map[string]interface{} {
	waste := input.NuclearWaste
	if len(waste.Plants) == 0 && waste.TotalStoredWaste == 0 {
		return nil
	}

	wasteRates := map[string]float64{
		"Uranium":   10,
		"Plutonium": 1,
		"Ficsonium": 0,
	}
	baseMW := 2500.0

	plantCount := len(waste.Plants)
	clockSpeeds := make([]float64, plantCount)
	for i, p := range waste.Plants {
		clockSpeeds[i] = float64(p.ClockSpeed)
		if clockSpeeds[i] == 0 {
			clockSpeeds[i] = 100
		}
	}

	avgClockSpeed := 100.0
	if plantCount > 0 {
		sum := 0.0
		for _, s := range clockSpeeds {
			sum += s
		}
		avgClockSpeed = sum / float64(plantCount)
	}

	totalPowerOutput := 0.0
	totalWasteProduction := 0.0
	enrichedPlants := make([]map[string]interface{}, plantCount)

	for i, plant := range waste.Plants {
		clockSpeed := float64(plant.ClockSpeed)
		if clockSpeed == 0 {
			clockSpeed = 100
		}
		clockMult := clockSpeed / 100

		powerOutput := baseMW * clockMult
		totalPowerOutput += powerOutput

		fuelType := plant.FuelType
		if fuelType == "" {
			fuelType = "Unknown"
		}
		wasteRate := 0.0
		if strings.Contains(fuelType, "Uranium") {
			wasteRate = wasteRates["Uranium"]
		} else if strings.Contains(fuelType, "Plutonium") {
			wasteRate = wasteRates["Plutonium"]
		} else if strings.Contains(fuelType, "Ficsonium") {
			wasteRate = wasteRates["Ficsonium"]
		}

		wasteProdRate := wasteRate * clockMult
		totalWasteProduction += wasteProdRate

		enrichedPlants[i] = map[string]interface{}{
			"instance":             plant.Instance,
			"clockSpeed":           plant.ClockSpeed,
			"fuelType":             fuelType,
			"powerOutput":          powerOutput,
			"wasteProductionRate":  wasteProdRate,
		}
	}

	fuelTypeCounts := map[string]int{}
	for _, plant := range waste.Plants {
		ft := plant.FuelType
		if ft == "" {
			ft = "Unknown"
		}
		fuelTypeCounts[ft]++
	}

	return map[string]interface{}{
		"plant_count":           plantCount,
		"total_power_output":    totalPowerOutput,
		"total_waste_production": totalWasteProduction,
		"average_clock_speed":   avgClockSpeed,
		"fuel_type_counts":      fuelTypeCounts,
		"total_stored_waste":    waste.TotalStoredWaste,
		"waste_containers":      waste.WasteContainers,
		"plants":                enrichedPlants,
	}
}
