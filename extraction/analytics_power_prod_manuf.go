package extraction

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

func calculatePowerAnalytics(input AnalyticsInput) map[string]interface{} {
	if powerDataLoaded == nil {
		return nil
	}

	// Aggregate generators by type
	type genTypeInfo struct {
		count        int
		activeCount  int
		standbyCount int
		totalMW      float64
		activeMW     float64
		standbyMW    float64
		clockSpeeds  []float64
		purities     map[string]int
	}
	genTypes := map[string]*genTypeInfo{}

	for _, gen := range input.GeneratorPower.Generators {
		typeLower := strings.ToLower(gen.TypePath)
		key := gen.TypePath
		info, ok := genTypes[key]
		if !ok {
			info = &genTypeInfo{purities: map[string]int{}}
			genTypes[key] = info
		}
		info.count++
		info.totalMW += gen.ActualMW
		info.clockSpeeds = append(info.clockSpeeds, float64(gen.OutputPercent))

		if gen.Status == "Active" || gen.Status == "Running" {
			info.activeCount++
			info.activeMW += gen.ActualMW
		} else {
			info.standbyCount++
			info.standbyMW += gen.ActualMW
		}

		// Track geothermal purities
		if strings.Contains(typeLower, "geothermal") {
			if purity, ok := input.GeothermalPurities[gen.InstanceName]; ok {
				info.purities[purity]++
			}
		}
	}

	// Build generator types summary
	genSummary := map[string]interface{}{}
	for typePath, info := range genTypes {
		avgClock := 100.0
		if len(info.clockSpeeds) > 0 {
			sum := 0.0
			for _, s := range info.clockSpeeds {
				sum += s
			}
			avgClock = sum / float64(len(info.clockSpeeds))
		}

		typeLower := strings.ToLower(typePath)
		isGeothermal := strings.Contains(typeLower, "geothermal")
		baseMW := 0.0
		var fuelTypes []string
		if !isGeothermal {
			if pd, ok := powerDataLoaded.Generators[typeLower]; ok {
				baseMW = toFloat64(pd.BaseMW)
				fuelTypes = pd.FuelTypes
			}
		}

		entry := map[string]interface{}{
			"count":          info.count,
			"active":         info.activeCount,
			"standby":        info.standbyCount,
			"total_mw":       info.totalMW,
			"active_mw":      info.activeMW,
			"standby_mw":     info.standbyMW,
			"avg_clock_speed": math.Round(avgClock*10) / 10,
		}
		if isGeothermal {
			// Geothermal: use actual extracted MW, no hardcoded base
			avgMW := 0.0
			if info.count > 0 {
				avgMW = info.totalMW / float64(info.count)
			}
			entry["avg_mw"] = math.Round(avgMW*10) / 10
		} else {
			entry["base_mw"] = baseMW
		}
		if len(fuelTypes) > 0 {
			entry["fuel_types"] = fuelTypes
		}
		if len(info.purities) > 0 {
			entry["purities"] = info.purities
		}
		genSummary[typePath] = entry
	}

	// Power storage — calculate from actual building count and stored energy
	powerStorageCount := 0
	totalStoredMWh := 0.0
	for typePath, count := range input.Buildings {
		if strings.Contains(strings.ToLower(typePath), "powerstorage") {
			powerStorageCount += count
		}
	}
	// Sum actual stored energy from BuildingFacts
	for _, bf := range input.BuildingFacts {
		if bf == nil {
			continue
		}
		typeLower := strings.ToLower(bf.TypePath)
		if strings.Contains(typeLower, "powerstorage") {
			if bf.HasPowerStore {
				totalStoredMWh += float64(bf.PowerStore)
			}
		}
	}
	perUnitCapacity := 100.0 // MWh per PowerStorage Mk1
	perUnitChargeRate := 100.0 // MW per PowerStorage Mk1
	powerStorage := map[string]interface{}{
		"count":              powerStorageCount,
		"capacity_mwh":       float64(powerStorageCount) * perUnitCapacity,
		"stored_mwh":         math.Round(totalStoredMWh*10) / 10,
		"max_charge_rate_mw": float64(powerStorageCount) * perUnitChargeRate,
	}

	// APA (Alien Power Augmenter)
	apaCount := input.APABuildingCount
	apaFueled := input.APAFueledCount
	apaUnfueled := apaCount - apaFueled
	apaBasePower := float64(apaCount) * 500.0 // 500 MW per APA
	apaMultiplier := 1.0 + (float64(apaUnfueled)*0.1 + float64(apaFueled)*0.3)
	totalBaseCapacity := input.GeneratorPower.ActiveCapacityMW + apaBasePower
	augmentedCapacity := totalBaseCapacity * apaMultiplier

	// Max theoretical: all generators at full capacity (base power × clock speed)
	maxBaseCapacity := input.GeneratorPower.TheoreticalMaxMW + apaBasePower
	maxAugmentedCapacity := maxBaseCapacity * apaMultiplier

	apa := map[string]interface{}{
		"building_count":             apaCount,
		"fueled_count":               apaFueled,
		"unfueled_count":             apaUnfueled,
		"base_power_mw":              apaBasePower,
		"multiplier":                 apaMultiplier,
		"augmented_capacity_mw":      augmentedCapacity,
		"max_augmented_capacity_mw":  maxAugmentedCapacity,
	}

	// Consumer summary
	consumerSummary := map[string]interface{}{
		"total_theoretical_mw": input.PowerConsumption.TotalTheoreticalMW,
		"total_actual_mw":      input.PowerConsumption.TotalActualMW,
		"active_buildings":     input.PowerConsumption.ActiveBuildings,
		"paused_buildings":     input.PowerConsumption.PausedBuildings,
	}

	// Power balance (includes APA contribution)
	capacityWithAPA := augmentedCapacity
	surplus := capacityWithAPA - input.PowerConsumption.TotalActualMW

	return map[string]interface{}{
		"generators": map[string]interface{}{
			"total_capacity_mw":    input.GeneratorPower.TotalCapacityMW,
			"active_capacity_mw":   input.GeneratorPower.ActiveCapacityMW,
			"standby_capacity_mw":  input.GeneratorPower.StandbyCapacityMW,
			"theoretical_max_mw":   input.GeneratorPower.TheoreticalMaxMW,
			"total_generators":     input.GeneratorPower.TotalGenerators,
			"active_generators":    input.GeneratorPower.ActiveGenerators,
			"standby_generators":   input.GeneratorPower.StandbyGenerators,
			"by_type":              genSummary,
		},
		"consumers":    consumerSummary,
		"power_storage": powerStorage,
		"apa":          apa,
		"power_balance": map[string]interface{}{
			"capacity_mw":              capacityWithAPA,
			"max_capacity_mw":          maxAugmentedCapacity,
			"generator_capacity_mw":    input.GeneratorPower.TotalCapacityMW,
			"active_capacity_mw":       input.GeneratorPower.ActiveCapacityMW,
			"standby_capacity_mw":      input.GeneratorPower.StandbyCapacityMW,
			"theoretical_max_mw":       input.GeneratorPower.TheoreticalMaxMW,
			"apa_base_power_mw":        apaBasePower,
			"apa_multiplier":           apaMultiplier,
			"consumption_mw":           input.PowerConsumption.TotalActualMW,
			"surplus_mw":               surplus,
		},
	}
}

func calculateProductionAnalytics(input AnalyticsInput) map[string]interface{} {
	if minersDataLoaded == nil {
		return nil
	}

	// Pre-compute production clock speeds by type
	prodClockSpeeds := map[string][]float64{}
	for inst, typePath := range input.BuildingTypes {
		typeLower := strings.ToLower(typePath)
		if isProductionBuilding(typeLower) {
			if speed, ok := input.ClockSpeeds[inst]; ok {
				prodClockSpeeds[typePath] = append(prodClockSpeeds[typePath], float64(speed))
			} else {
				prodClockSpeeds[typePath] = append(prodClockSpeeds[typePath], 100)
			}
		}
	}

	// Miners
	miners := map[string]interface{}{}
	minerDetails := map[string]interface{}{}
	totalMinerCapacity := 0.0

	for btype, count := range input.Buildings {
		typeLower := strings.ToLower(btype)
		if !strings.Contains(typeLower, "miner") || strings.Contains(typeLower, "portable") {
			continue
		}

		var minerType string
		if strings.Contains(typeLower, "mk.1") || strings.Contains(typeLower, "mk1") {
			minerType = "MinerMk1"
		} else if strings.Contains(typeLower, "mk.2") || strings.Contains(typeLower, "mk2") {
			minerType = "MinerMk2"
		} else if strings.Contains(typeLower, "mk.3") || strings.Contains(typeLower, "mk3") {
			minerType = "MinerMk3"
		}

		if minerType == "" {
			continue
		}
		mData, ok := minersDataLoaded[minerType]
		if !ok {
			continue
		}

		baseRate := mData.BaseRate
		clockSpeeds := prodClockSpeeds[btype]

		// Build clock speed distribution with purity
		clockSpeedDist := map[string]interface{}{}
		purityCounts := map[string]int{}
		totalCapacityFromDist := 0.0

		for inst, instType := range input.BuildingTypes {
			instLower := strings.ToLower(instType)
			if !strings.Contains(instLower, strings.ToLower(minerType)) {
				continue
			}
			speed := 100.0
			if s, ok := input.ClockSpeeds[inst]; ok {
				speed = float64(s)
			}
			roundedSpeed := int(math.Round(speed))

			purity := "normal"
			if p, ok := input.MinerPurities[inst]; ok {
				purity = p
			}
			purityMult := 1.0
			if pm, ok := mData.PurityMultiplier[purity]; ok {
				purityMult = pm
			}

			production := float64(baseRate) * purityMult * (speed / 100)

			keyStr := intToStr(roundedSpeed)
			if dist, ok := clockSpeedDist[keyStr].(map[string]interface{}); ok {
				dist["count"] = dist["count"].(int) + 1
				dist["production"] = dist["production"].(float64) + production
				if purities, ok := dist["purities"].(map[string]int); ok {
					purities[purity]++
				}
			} else {
				purities := map[string]int{}
				purities[purity] = 1
				clockSpeedDist[keyStr] = map[string]interface{}{
					"count":      1,
					"production": production,
					"purities":   purities,
				}
			}

			purityCounts[purity]++
			totalCapacityFromDist += production

			// Store miner details
			resource := "Unknown"
			if r, ok := input.MinerResources[inst]; ok {
				resource = r
			}
			var connections *BuildingConnection
			if c, ok := input.BuildingConnections[inst]; ok {
				connections = &c
			}
			var loc *ConsumerLocation
			for i := range input.PowerConsumption.Consumers {
				if input.PowerConsumption.Consumers[i].InstanceName == inst {
					loc = input.PowerConsumption.Consumers[i].Location
					break
				}
			}
			minerDetail := map[string]interface{}{
				"type":         minerType,
				"clock_speed":  speed,
				"capacity":     production,
				"purity":       purity,
				"resource":     resource,
				"input_belts":  func() []string { if connections != nil { return connections.Inputs }; return []string{} }(),
				"output_belts": func() []string { if connections != nil { return connections.Outputs }; return []string{} }(),
			}
			if loc != nil {
				minerDetail["location"] = loc
			}
			minerDetails[inst] = minerDetail
		}

		avgClock := 100.0
		if len(clockSpeeds) > 0 {
			sum := 0.0
			for _, s := range clockSpeeds {
				sum += s
			}
			avgClock = sum / float64(len(clockSpeeds))
		}

		finalCapacity := totalCapacityFromDist
		if finalCapacity == 0 {
			for _, s := range clockSpeeds {
				finalCapacity += float64(baseRate) * (s / 100)
			}
		}

		miners[minerType] = map[string]interface{}{
			"count":                   count,
			"base_rate":               baseRate,
			"avg_clock_speed":         avgClock,
			"total_capacity":          finalCapacity,
			"clock_speed_distribution": clockSpeedDist,
			"purity_counts":           purityCounts,
		}
		totalMinerCapacity += finalCapacity
	}

	// Fluid extractors
	fluidExtractors := map[string]interface{}{}
	totalFluidCapacity := 0.0

	for btype, count := range input.Buildings {
		typeLower := strings.ToLower(btype)
		if !(strings.Contains(typeLower, "frackingextractor") || strings.Contains(typeLower, "waterpump") ||
			strings.Contains(typeLower, "oilpump") || strings.Contains(typeLower, "resourcewell")) {
			continue
		}

		var extractorType string
		var baseRate int
		if strings.Contains(typeLower, "frackingextractor") {
			extractorType = "Fracking Unit"
			baseRate = 60
		} else if strings.Contains(typeLower, "waterpump") {
			extractorType = "Water Pump"
			baseRate = 120
		} else if strings.Contains(typeLower, "oilpump") {
			extractorType = "Oil Pump"
			baseRate = 120
		} else if strings.Contains(typeLower, "resourcewell") {
			extractorType = "Resource Well"
			baseRate = 60
		}

		if extractorType == "" {
			continue
		}

		clockSpeeds := prodClockSpeeds[btype]
		actualCapacity := 0.0
		avgClock := 100.0

		// Fluid types for fracking
		fluidTypes := map[string]int{}
		fluidTypesBySpeed := map[int]map[string]map[string]int{}
		if strings.Contains(typeLower, "frackingextractor") {
			for inst, instType := range input.BuildingTypes {
				if !strings.Contains(strings.ToLower(instType), "frackingextractor") {
					continue
				}
				if ft, ok := input.FrackingFluidTypes[inst]; ok {
					speed := 100.0
					if s, ok := input.ClockSpeeds[inst]; ok {
						speed = float64(s)
					}
					roundedSpeed := int(math.Round(speed))
					purity := "normal"
					if p, ok := input.FrackingPurities[inst]; ok {
						purity = p
					}

					fluidTypes[ft]++

					if fluidTypesBySpeed[roundedSpeed] == nil {
						fluidTypesBySpeed[roundedSpeed] = map[string]map[string]int{}
					}
					if fluidTypesBySpeed[roundedSpeed][ft] == nil {
						fluidTypesBySpeed[roundedSpeed][ft] = map[string]int{}
					}
					fluidTypesBySpeed[roundedSpeed][ft]["count"]++
					fluidTypesBySpeed[roundedSpeed][ft][purity]++
				}
			}
		}

		if len(clockSpeeds) > 0 {
			for _, s := range clockSpeeds {
				actualCapacity += float64(baseRate) * (s / 100)
			}
			sum := 0.0
			for _, s := range clockSpeeds {
				sum += s
			}
			avgClock = sum / float64(len(clockSpeeds))
		} else {
			actualCapacity = float64(baseRate) * float64(count)
		}

		// Clock speed distribution
		clockSpeedDist := map[string]interface{}{}
		for _, speed := range clockSpeeds {
			roundedSpeed := int(math.Round(speed))
			keyStr := intToStr(roundedSpeed)
			if dist, ok := clockSpeedDist[keyStr].(map[string]interface{}); ok {
				dist["count"] = dist["count"].(int) + 1
				dist["production"] = dist["production"].(float64) + float64(baseRate)*(speed/100)
				if fts, ok := fluidTypesBySpeed[roundedSpeed]; ok {
					dist["fluid_types"] = fts
				}
			} else {
				entry := map[string]interface{}{
					"count":      1,
					"production": float64(baseRate) * (speed / 100),
					"fluid_types": map[string]map[string]int{},
				}
				if fts, ok := fluidTypesBySpeed[roundedSpeed]; ok {
					entry["fluid_types"] = fts
				}
				clockSpeedDist[keyStr] = entry
			}
		}

		fluidExtractors[extractorType] = map[string]interface{}{
			"count":                   count,
			"base_rate":               baseRate,
			"avg_clock_speed":         avgClock,
			"total_capacity":          actualCapacity,
			"clock_speed_distribution": clockSpeedDist,
			"fluid_types":             fluidTypes,
		}
		totalFluidCapacity += actualCapacity
	}

	// Production buildings count
	productionBuildings := map[string]int{}
	for btype, count := range input.Buildings {
		typeLower := strings.ToLower(btype)
		if strings.Contains(typeLower, "assembler") || strings.Contains(typeLower, "smelter") ||
			strings.Contains(typeLower, "foundry") || strings.Contains(typeLower, "constructor") ||
			strings.Contains(typeLower, "manufacturer") || strings.Contains(typeLower, "refinery") ||
			strings.Contains(typeLower, "blender") || strings.Contains(typeLower, "packager") ||
			strings.Contains(typeLower, "hadron") || strings.Contains(typeLower, "quantum") ||
			strings.Contains(typeLower, "converter") || strings.Contains(typeLower, "particleaccelerator") ||
			strings.Contains(typeLower, "cauldron") || strings.Contains(typeLower, "collider") {
			productionBuildings[btype] = count
		}
	}

	// Extractor bottlenecks
	extractorBottlenecks := map[string]interface{}{
		"blocked":        []interface{}{},
		"paused":         []interface{}{},
		"blocked_count":  0,
		"starving_count": 0,
		"no_power_count": 0,
		"paused_count":   0,
	}

	var blockedList, pausedList []interface{}
	for inst, typePath := range input.BuildingTypes {
		typeLower := strings.ToLower(typePath)
		isExtractor := strings.Contains(typeLower, "miner") || strings.Contains(typeLower, "waterpump") ||
			strings.Contains(typeLower, "waterextractor") || strings.Contains(typeLower, "oilextractor") ||
			strings.Contains(typeLower, "oilpump") || strings.Contains(typeLower, "fracking") ||
			strings.Contains(typeLower, "resourcewell")
		if !isExtractor {
			continue
		}

		// Find power consumer
		var powerConsumer *PowerConsumerEntry
		for i := range input.PowerConsumption.Consumers {
			if input.PowerConsumption.Consumers[i].InstanceName == inst {
				powerConsumer = &input.PowerConsumption.Consumers[i]
				break
			}
		}

		speed := 100.0
		if s, ok := input.ClockSpeeds[inst]; ok {
			speed = float64(s)
		}

		resource := "Unknown"
		if strings.Contains(typeLower, "miner") {
			if r, ok := input.MinerResources[inst]; ok {
				resource = r
			}
		} else if strings.Contains(typeLower, "fracking") {
			if r, ok := input.FrackingFluidTypes[inst]; ok {
				resource = r
			} else {
				resource = "Water"
			}
		} else if strings.Contains(typeLower, "water") {
			resource = "Water"
		} else if strings.Contains(typeLower, "oil") {
			resource = "CrudeOil"
		}

		var connections *BuildingConnection
		if c, ok := input.BuildingConnections[inst]; ok {
			connections = &c
		}

		actualMW := 0.0
		status := ""
		isBlocked := false
		if powerConsumer != nil {
			actualMW = powerConsumer.ActualMW
			status = powerConsumer.Status
			isBlocked = powerConsumer.IsBlocked
		}

		buildingInfo := map[string]interface{}{
			"instance":    inst,
			"type":        typePath,
			"clock_speed": speed,
			"power_mw":    actualMW,
			"resource":    resource,
			"output_belts": func() []string {
				if connections != nil {
					return connections.Outputs
				}
				return []string{}
			}(),
		}
		if powerConsumer != nil && powerConsumer.Location != nil {
			buildingInfo["location"] = powerConsumer.Location
		}

		if status == "Paused" {
			pausedList = append(pausedList, buildingInfo)
		} else if isBlocked {
			blockedList = append(blockedList, buildingInfo)
		}
	}

	// Sort by building name
	sort.Slice(blockedList, func(i, j int) bool {
		return getBuildingName(blockedList[i]) < getBuildingName(blockedList[j])
	})
	sort.Slice(pausedList, func(i, j int) bool {
		return getBuildingName(pausedList[i]) < getBuildingName(pausedList[j])
	})

	extractorBottlenecks["blocked"] = blockedList
	extractorBottlenecks["paused"] = pausedList
	extractorBottlenecks["blocked_count"] = len(blockedList)
	extractorBottlenecks["paused_count"] = len(pausedList)

	return map[string]interface{}{
		"miners":                   miners,
		"miner_details":            minerDetails,
		"total_miner_capacity":     totalMinerCapacity,
		"fluid_extractors":         fluidExtractors,
		"fluid_extractor_details":  map[string]interface{}{},
		"total_fluid_capacity":     totalFluidCapacity,
		"production_buildings":     productionBuildings,
		"extractor_bottlenecks":    extractorBottlenecks,
	}
}

func isProductionBuilding(typeLower string) bool {
	return strings.Contains(typeLower, "miner") || strings.Contains(typeLower, "smelter") ||
		strings.Contains(typeLower, "constructor") || strings.Contains(typeLower, "assembler") ||
		strings.Contains(typeLower, "manufacturer") || strings.Contains(typeLower, "refinery") ||
		strings.Contains(typeLower, "blender") || strings.Contains(typeLower, "foundry") ||
		strings.Contains(typeLower, "packager") || strings.Contains(typeLower, "hadron") ||
		strings.Contains(typeLower, "quantum") || strings.Contains(typeLower, "converter") ||
		strings.Contains(typeLower, "waterpump") || strings.Contains(typeLower, "waterextractor") ||
		strings.Contains(typeLower, "oilextractor") || strings.Contains(typeLower, "oilpump") ||
		strings.Contains(typeLower, "fracking") || strings.Contains(typeLower, "resourcewell") ||
		strings.Contains(typeLower, "particleaccelerator")
}

func getBuildingName(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	typePath, _ := m["type"].(string)
	parts := strings.Split(typePath, "/")
	last := parts[len(parts)-1]
	className := strings.Split(last, ".")[0]
	className = strings.Replace(className, "Build_", "", 1)
	className = strings.TrimSuffix(className, "_C")
	return className
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func calculateManufacturingAnalytics(input AnalyticsInput) map[string]interface{} {
	if input.BuildingTypes == nil || input.ClockSpeeds == nil {
		return nil
	}

	manufacturingTypes := []string{"smelter", "constructor", "assembler", "manufacturer", "foundry",
		"refinery", "blender", "packager", "hadron", "quantum", "converter"}

	activeProduction := map[string]interface{}{}
	var waitingBuildings, blockedBuildings, pausedBuildings []interface{}
	totalManufacturingBuildings := 0

	// Create consumer lookup map
	consumerMap := map[string]*PowerConsumerEntry{}
	for i := range input.PowerConsumption.Consumers {
		consumerMap[input.PowerConsumption.Consumers[i].InstanceName] = &input.PowerConsumption.Consumers[i]
	}

	for inst, typePath := range input.BuildingTypes {
		typeLower := strings.ToLower(typePath)
		isManufacturing := false
		for _, mtype := range manufacturingTypes {
			if strings.Contains(typeLower, mtype) {
				isManufacturing = true
				break
			}
		}
		if !isManufacturing {
			continue
		}

		totalManufacturingBuildings++

		recipePath := input.BuildingRecipes[inst]
		speed := 100.0
		if s, ok := input.ClockSpeeds[inst]; ok {
			speed = float64(s)
		}

		var powerConsumer *PowerConsumerEntry
		if c, ok := consumerMap[inst]; ok {
			powerConsumer = c
		}

		status := ""
		actualMW := 0.0
		var loc *ConsumerLocation
		if powerConsumer != nil {
			status = powerConsumer.Status
			actualMW = powerConsumer.ActualMW
			loc = powerConsumer.Location
		}

		if recipePath == "" {
			pausedEntry := map[string]interface{}{
				"instance":    inst,
				"type":        typePath,
				"clock_speed": speed,
				"reason":      "No recipe set",
			}
			if loc != nil {
				pausedEntry["location"] = loc
			}
			pausedBuildings = append(pausedBuildings, pausedEntry)
			continue
		}

		isActivelyProducing := actualMW > 0

		var connections *BuildingConnection
		if c, ok := input.BuildingConnections[inst]; ok {
			connections = &c
		}

		getConnections := func(field string) []string {
			if connections != nil {
				if field == "inputs" {
					return connections.Inputs
				}
				return connections.Outputs
			}
			return []string{}
		}

		recipeName := "Unknown"
		if recipe, ok := recipeMap[recipePath]; ok {
			recipeName = recipe.Name
		}

		if status == "Paused" && !isActivelyProducing {
			entry := map[string]interface{}{
				"instance":      inst,
				"type":          typePath,
				"recipe":        recipeName,
				"clock_speed":   speed,
				"power_mw":      actualMW,
				"reason":        "Manually paused",
				"input_belts":   getConnections("inputs"),
				"output_belts":  getConnections("outputs"),
			}
			if loc != nil {
				entry["location"] = loc
			}
			waitingBuildings = append(waitingBuildings, entry)
		} else if status == "Blocked" && !isActivelyProducing {
			entry := map[string]interface{}{
				"instance":      inst,
				"type":          typePath,
				"recipe":        recipeName,
				"clock_speed":   speed,
				"power_mw":      actualMW,
				"reason":        "Output full (waiting for space)",
				"input_belts":   getConnections("inputs"),
				"output_belts":  getConnections("outputs"),
			}
			if loc != nil {
				entry["location"] = loc
			}
			blockedBuildings = append(blockedBuildings, entry)
		} else if status == "Starving" && !isActivelyProducing {
			entry := map[string]interface{}{
				"instance":      inst,
				"type":          typePath,
				"recipe":        recipeName,
				"clock_speed":   speed,
				"power_mw":      actualMW,
				"reason":        "No input items (waiting for material)",
				"input_belts":   getConnections("inputs"),
				"output_belts":  getConnections("outputs"),
			}
			if loc != nil {
				entry["location"] = loc
			}
			waitingBuildings = append(waitingBuildings, entry)
		} else if status == "Running" || isActivelyProducing {
			if _, ok := activeProduction[recipePath]; !ok {
				var recipeData *Recipe
				if r, ok := recipeMap[recipePath]; ok {
					recipeData = r
				}
				activeProduction[recipePath] = map[string]interface{}{
					"recipe_name":  recipeName,
					"recipe_data":  recipeData,
					"buildings":    []interface{}{},
					"clock_speeds": []float64{},
					"count":        0,
				}
			}

			entry := activeProduction[recipePath].(map[string]interface{})
			bldg := map[string]interface{}{
				"instance":      inst,
				"type":          typePath,
				"clock_speed":   speed,
				"input_belts":   getConnections("inputs"),
				"output_belts":  getConnections("outputs"),
			}
			if loc != nil {
				bldg["location"] = loc
			}
			entry["buildings"] = append(entry["buildings"].([]interface{}), bldg)
			entry["clock_speeds"] = append(entry["clock_speeds"].([]float64), speed)
			entry["count"] = entry["count"].(int) + 1
		} else {
			// Unknown status
			if actualMW > 0.05 {
				if _, ok := activeProduction[recipePath]; !ok {
					var recipeData *Recipe
					if r, ok := recipeMap[recipePath]; ok {
						recipeData = r
					}
					activeProduction[recipePath] = map[string]interface{}{
						"recipe_name":  recipeName,
						"recipe_data":  recipeData,
						"buildings":    []interface{}{},
						"clock_speeds": []float64{},
						"count":        0,
					}
				}
				entry := activeProduction[recipePath].(map[string]interface{})
				bldg := map[string]interface{}{
					"instance":      inst,
					"type":          typePath,
					"clock_speed":   speed,
					"input_belts":   getConnections("inputs"),
					"output_belts":  getConnections("outputs"),
				}
				if loc != nil {
					bldg["location"] = loc
				}
				entry["buildings"] = append(entry["buildings"].([]interface{}), bldg)
				entry["clock_speeds"] = append(entry["clock_speeds"].([]float64), speed)
				entry["count"] = entry["count"].(int) + 1
			} else {
				reason := "Unknown status"
				if actualMW == 0 {
					reason = "No power connection"
				}
				entry := map[string]interface{}{
					"instance":      inst,
					"type":          typePath,
					"recipe":        recipeName,
					"clock_speed":   speed,
					"power_mw":      actualMW,
					"reason":        reason,
					"input_belts":   getConnections("inputs"),
					"output_belts":  getConnections("outputs"),
				}
				if loc != nil {
					entry["location"] = loc
				}
				waitingBuildings = append(waitingBuildings, entry)
			}
		}
	}

	// Calculate production rates and clock speed distributions
	activeCount := 0
	for _, v := range activeProduction {
		entry := v.(map[string]interface{})
		speeds := entry["clock_speeds"].([]float64)

		// Clock speed distribution
		speedDist := map[int]int{}
		for _, s := range speeds {
			speedDist[int(math.Round(s))]++
		}
		distMap := map[string]int{}
		for k, v := range speedDist {
			distMap[intToStr(k)] = v
		}
		entry["clock_speed_distribution"] = distMap

		// Average clock speed
		count := entry["count"].(int)
		if count > 0 {
			sum := 0.0
			for _, s := range speeds {
				sum += s
			}
			entry["avg_clock_speed"] = int(math.Round(sum / float64(count)))
		}
		entry["building_count"] = count

		// Total output and efficiency
		if recipeData, ok := entry["recipe_data"].(*Recipe); ok && recipeData != nil {
			totalOutput := map[string]interface{}{}
			actualTotal := 0.0
			theoreticalTotal := 0.0
			for _, product := range recipeData.Products {
				baseRate := (product.Amount / recipeData.Duration) * 60 // items per minute
				totalProduction := 0.0
				for _, s := range speeds {
					totalProduction += baseRate * (s / 100)
				}
				theoreticalMax := baseRate * float64(count) // at 100% clock
				avgRate := 0.0
				if count > 0 {
					avgRate = totalProduction / float64(count)
				}
				totalOutput[product.Item] = map[string]interface{}{
					"total_per_minute":  math.Round(totalProduction*10) / 10,
					"avg_per_building":  math.Round(avgRate*10) / 10,
					"base_rate":         math.Round(baseRate*10) / 10,
				}
				actualTotal += totalProduction
				theoreticalTotal += theoreticalMax
			}
			entry["total_output"] = totalOutput
			entry["actual_output"] = math.Round(actualTotal*10) / 10
			entry["theoretical_max"] = math.Round(theoreticalTotal*10) / 10
			if theoreticalTotal > 0 {
				entry["efficiency"] = math.Round((actualTotal/theoreticalTotal)*1000) / 10
			} else {
				entry["efficiency"] = 0.0
			}
		} else {
			entry["actual_output"] = 0.0
			entry["theoretical_max"] = 0.0
			entry["efficiency"] = 0.0
		}

		activeCount += count
	}

	// Sort by count (most used first)
	type recipeEntry struct {
		key string
		val map[string]interface{}
	}
	var sorted []recipeEntry
	for k, v := range activeProduction {
		sorted = append(sorted, recipeEntry{k, v.(map[string]interface{})})
	}
	sort.Slice(sorted, func(i, j int) bool {
		ci, _ := sorted[i].val["count"].(int)
		cj, _ := sorted[j].val["count"].(int)
		return ci > cj
	})

	sortedActive := map[string]interface{}{}
	for _, e := range sorted {
		sortedActive[e.key] = e.val
	}

	// Sort bottlenecks
	sort.Slice(waitingBuildings, func(i, j int) bool {
		return getBuildingName(waitingBuildings[i]) < getBuildingName(waitingBuildings[j])
	})
	sort.Slice(blockedBuildings, func(i, j int) bool {
		return getBuildingName(blockedBuildings[i]) < getBuildingName(blockedBuildings[j])
	})
	sort.Slice(pausedBuildings, func(i, j int) bool {
		return getBuildingName(pausedBuildings[i]) < getBuildingName(pausedBuildings[j])
	})

	return map[string]interface{}{
		"total_manufacturing_buildings": totalManufacturingBuildings,
		"active_production":             sortedActive,
		"active_count":                  activeCount,
		"total_recipes_in_use":          len(sorted),
		"bottlenecks": map[string]interface{}{
			"waiting":        waitingBuildings,
			"blocked":        blockedBuildings,
			"paused":         pausedBuildings,
			"waiting_count":  len(waitingBuildings),
			"blocked_count":  len(blockedBuildings),
			"paused_count":   len(pausedBuildings),
		},
	}
}

func calculateGameProgressionAnalytics(input AnalyticsInput) map[string]interface{} {
	gp := input.GameProgression
	if gp.CurrentPhase == "" && gp.TargetPhase == "" && len(gp.PurchasedSchematics) == 0 {
		return nil
	}

	isV12OrLater := input.SaveVersion >= 53

	// Extract phase number
	extractPhaseNum := func(phaseName string) int {
		if phaseName == "" {
			return 0
		}
		re := regexp.MustCompile(`Phase_(\d+)`)
		m := re.FindStringSubmatch(phaseName)
		if m != nil {
			n := 0
			for _, c := range m[1] {
				n = n*10 + int(c-'0')
			}
			return n
		}
		return 0
	}

	currentPhaseNum := extractPhaseNum(gp.CurrentPhase)
	targetPhaseNum := extractPhaseNum(gp.TargetPhase)

	spaceElevatorPhase := currentPhaseNum
	if spaceElevatorPhase > 4 {
		spaceElevatorPhase = 4
	}
	isCompleted := currentPhaseNum >= 4
	targetPhase := targetPhaseNum
	if targetPhase > 4 {
		targetPhase = 4
	}
	if targetPhase == 0 {
		if isCompleted {
			targetPhase = 4
		} else {
			targetPhase = 1
		}
	}

	// HUB tier from purchased schematics (X-Y format)
	maxHubTier := 0
	hubRe := regexp.MustCompile(`^\d+-\d+$`)
	for _, s := range gp.PurchasedSchematics {
		if hubRe.MatchString(s) {
			parts := strings.Split(s, "-")
			tier := 0
			for _, c := range parts[0] {
				tier = tier*10 + int(c-'0')
			}
			if tier > maxHubTier {
				maxHubTier = tier
			}
		}
	}

	// MAM research trees
	mamTrees := []string{}
	seen := map[string]bool{}
	treeMap := map[string]string{
		"Caterium":        "Caterium",
		"Quartz":          "Quartz",
		"Sulfur":          "Sulfur",
		"Nutrients":       "Nutrients (Mycelia)",
		"Mycelia":         "Nutrients (Mycelia)",
		"PowerSlugs":      "Power Slugs",
		"FlowerPetals":    "Flower Petals",
		"AlienOrganisms":  "Alien Organisms",
		"AlienTech":       "Alien Technology",
		"AlienTechnology": "Alien Technology",
	}
	for _, t := range gp.UnlockedResearchTrees {
		if t == "HardDrive" {
			continue
		}
		name := t
		if mapped, ok := treeMap[t]; ok {
			name = mapped
		}
		if !seen[name] {
			seen[name] = true
			mamTrees = append(mamTrees, name)
		}
	}

	// MAM progress
	mamProgress := map[string]interface{}{}
	treeTotals := map[string]int{
		"Caterium":            16,
		"Quartz":              12,
		"Sulfur":              16,
		"Nutrients (Mycelia)": 15,
		"Power Slugs":         6,
		"Alien Technology":    19,
		"Flower Petals":       11,
		"Alien Organisms":     11,
		"XMas":                13,
	}
	if isV12OrLater {
		treeTotals["Caterium"] = 17
		treeTotals["Alien Technology"] = 20
	}

	// treeSchematicPrefixes maps each MAM tree to the schematic name prefixes
	// that belong to it. Schematics follow the pattern Research_<Prefix>_*.
	treeSchematicPrefixes := map[string][]string{
		"Caterium":            {"Research_Caterium_"},
		"Quartz":              {"Research_Quartz_"},
		"Sulfur":              {"Research_Sulfur_"},
		"Nutrients (Mycelia)": {"Research_Nutrients_", "Research_Mycelia_"},
		"Power Slugs":         {"Research_PowerSlugs_"},
		"Flower Petals":       {"Research_FlowerPetals_", "Research_FlowerPetal_"},
		"Alien Organisms":     {"Research_AO_", "Research_ACarapace_", "Research_AOrganisms_", "Research_AOrgans_"},
		"Alien Technology":    {"Research_Alien_"},
		"XMas":                {"Research_XMas_"},
	}

	// crossListed maps schematics whose prefix suggests one tree but actually
	// belong to another (e.g. Blade Runners is Research_Caterium_4_3 but is
	// in the Quartz tree).
	crossListed := map[string]string{
		"Research_Caterium_3_1": "Quartz",
		"Research_Caterium_4_3": "Quartz",
	}

	for _, tree := range mamTrees {
		total := treeTotals[tree]
		prefixes := treeSchematicPrefixes[tree]
		completed := 0
		for _, s := range gp.PurchasedSchematics {
			if !strings.HasPrefix(s, "Research_") {
				continue
			}
			if strings.HasSuffix(s, "_Hidden") {
				continue
			}
			if crossTree, isCross := crossListed[s]; isCross {
				if crossTree == tree {
					completed++
				}
				continue
			}
			for _, pfx := range prefixes {
				if strings.HasPrefix(s, pfx) {
					completed++
					break
				}
			}
		}
		// Cap completed at total — FICSMAS event adds bonus research nodes
		// to existing trees that don't count toward the tree's total in-game.
		if total > 0 && completed > total {
			completed = total
		}
		percentage := 0
		if total > 0 {
			percentage = (completed * 100) / total
			if percentage > 100 {
				percentage = 100
			}
		}
		mamProgress[tree] = map[string]interface{}{
			"completed":  completed,
			"total":      total,
			"percentage": percentage,
		}
	}

	return map[string]interface{}{
		"space_elevator": map[string]interface{}{
			"current_phase":         gp.CurrentPhase,
			"current_phase_number":  spaceElevatorPhase,
			"target_phase":          gp.TargetPhase,
			"target_phase_number":   targetPhase,
			"is_completed":          isCompleted,
		},
		"hub": map[string]interface{}{
			"current_tier": maxHubTier,
			"max_tier":      9,
		},
		"mam": map[string]interface{}{
			"unlocked_trees":  mamTrees,
			"unlocked_count":  len(mamTrees),
			"is_activated":    gp.ResearchActivated,
			"progress":        mamProgress,
		},
		"schematics": map[string]interface{}{
			"purchased_count":      len(gp.PurchasedSchematics),
			"last_active":          gp.LastActiveSchematic,
			"purchased_schematics": gp.PurchasedSchematics,
		},
		"research": map[string]interface{}{
			"activated":            gp.ResearchActivated,
			"unlocked_trees_count": len(gp.UnlockedResearchTrees),
			"last_used_hard_drive_id": gp.LastUsedHardDriveID,
			"unlocked_trees":       gp.UnlockedResearchTrees,
		},
		"awesome_sink": map[string]interface{}{
			"coupons":      gp.SinkCoupons,
			"total_points": gp.SinkTotalPoints,
		},
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case map[string]interface{}:
		// Geothermal map like {impure: 100, normal: 200, pure: 400}; return normal.
		if n, ok := val["normal"].(float64); ok {
			return n
		}
		if n, ok := val["normal"].(int); ok {
			return float64(n)
		}
	}
	return 0
}
