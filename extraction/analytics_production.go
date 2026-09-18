package extraction

import (
	"math"
	"sort"
	"strings"
)

func calculateProductionAnalytics(input AnalyticsInput) map[string]interface{} {
	if minersDataLoaded == nil {
		return nil
	}

	// Pre-compute production clock speeds by type
	prodClockSpeeds := map[string][]float64{}
	for _, inst := range sortedKeys(input.BuildingTypes) {
		typePath := input.BuildingTypes[inst]
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
	productionByResource := map[string]interface{}{}

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
		} else {
			// Modded miner (e.g. MinerMk5) - use the raw class name as type.
			// We don't know its base rate, so we skip production calculations.
			minerType = classNameFromPath(btype)
		}

		mData, ok := minersDataLoaded[minerType]
		knownRate := ok
		if !ok {
			// No data for this miner tier (modded). Still show it with
			// count/clock/purity/resource/location, but no production numbers.
			mData = MinerData{BaseRate: 0, PurityMultiplier: map[string]float64{}}
		}

		baseRate := mData.BaseRate
		clockSpeeds := prodClockSpeeds[btype]

		// Build clock speed distribution with purity
		clockSpeedDist := map[string]interface{}{}
		purityCounts := map[string]int{}
		totalCapacityFromDist := 0.0

		for _, inst := range sortedKeys(input.BuildingTypes) {
			instType := input.BuildingTypes[inst]
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

			var production float64
			if knownRate {
				production = float64(baseRate) * purityMult * (speed / 100)
			}

			keyStr := intToStr(roundedSpeed)
			if dist, ok := clockSpeedDist[keyStr].(map[string]interface{}); ok {
				dist["count"] = dist["count"].(int) + 1
				if knownRate {
					dist["production"] = dist["production"].(float64) + production
				}
				if purities, ok := dist["purities"].(map[string]int); ok {
					purities[purity]++
				}
			} else {
				purities := map[string]int{}
				purities[purity] = 1
				entry := map[string]interface{}{
					"count":    1,
					"purities": purities,
				}
				if knownRate {
					entry["production"] = production
				}
				clockSpeedDist[keyStr] = entry
			}

			purityCounts[purity]++
			if knownRate {
				totalCapacityFromDist += production
			}

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
				"type":        minerType,
				"clock_speed": speed,
				"purity":      purity,
				"resource":    resource,
				"input_belts": func() []string {
					if connections != nil {
						return connections.Inputs
					}
					return []string{}
				}(),
				"output_belts": func() []string {
					if connections != nil {
						return connections.Outputs
					}
					return []string{}
				}(),
			}
			if knownRate {
				minerDetail["capacity"] = production
			}
			// Belt bottleneck analysis
			var beltOut []string
			if connections != nil {
				beltOut = connections.Outputs
			}
			beltTput, beltLabel := outputThroughput(beltOut, false)
			minerDetail["belt_throughput"] = beltTput
			minerDetail["belt_label"] = beltLabel
			var effective float64
			if knownRate {
				effective = production
				if beltTput > 0 && production > beltTput {
					effective = beltTput
					minerDetail["bottleneck"] = true
				} else {
					minerDetail["bottleneck"] = false
				}
				minerDetail["effective_output"] = effective
			} else {
				minerDetail["bottleneck"] = false
			}
			if loc != nil {
				minerDetail["location"] = loc
			}
			minerDetails[inst] = minerDetail

			// Aggregate by resource
			resKey := resource
			if resKey == "Unknown" {
				resKey = "Unknown Ore"
			}
			if entry, ok := productionByResource[resKey].(map[string]interface{}); ok {
				entry["count"] = entry["count"].(int) + 1
				if knownRate {
					if prev, ok2 := entry["total_output"].(float64); ok2 {
						entry["total_output"] = prev + production
					} else {
						entry["total_output"] = production
					}
					if prev, ok2 := entry["effective_output"].(float64); ok2 {
						entry["effective_output"] = prev + effective
					} else {
						entry["effective_output"] = effective
					}
				}
				if minerDetail["bottleneck"].(bool) {
					entry["bottlenecked"] = entry["bottlenecked"].(int) + 1
				}
			} else {
				bn := 0
				if minerDetail["bottleneck"].(bool) {
					bn = 1
				}
				entry := map[string]interface{}{
					"count":        1,
					"bottlenecked": bn,
					"is_fluid":     false,
				}
				if knownRate {
					entry["total_output"] = production
					entry["effective_output"] = effective
				}
				productionByResource[resKey] = entry
			}
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

		minerSummary := map[string]interface{}{
			"count":                    count,
			"avg_clock_speed":          avgClock,
			"clock_speed_distribution": clockSpeedDist,
			"purity_counts":            purityCounts,
		}
		if knownRate {
			minerSummary["base_rate"] = baseRate
			minerSummary["total_capacity"] = finalCapacity
		}
		// Accumulate if multiple mod paths map to the same minerType
		if existingIface, ok := miners[minerType]; ok {
			existing := existingIface.(map[string]interface{})
			existing["count"] = existing["count"].(int) + count
			// Merge purity counts
			existingPurities := existing["purity_counts"].(map[string]int)
			for p, c := range purityCounts {
				existingPurities[p] += c
			}
			// Merge clock speed distribution
			existingDist := existing["clock_speed_distribution"].(map[string]interface{})
			for k, vIface := range clockSpeedDist {
				v := vIface.(map[string]interface{})
				if d, ok2 := existingDist[k].(map[string]interface{}); ok2 {
					d["count"] = d["count"].(int) + v["count"].(int)
					if knownRate {
						if prev, ok3 := d["production"].(float64); ok3 {
							if add, ok4 := v["production"].(float64); ok4 {
								d["production"] = prev + add
							}
						} else if add, ok4 := v["production"].(float64); ok4 {
							d["production"] = add
						}
					}
				} else {
					existingDist[k] = v
				}
			}
			// Recalculate avg clock speed
			totalClock := avgClock * float64(len(clockSpeeds))
			totalCount := len(clockSpeeds)
			existingAvg := existing["avg_clock_speed"].(float64)
			existingCount := existing["count"].(int) - count
			if existingCount > 0 {
				totalClock += existingAvg * float64(existingCount)
				totalCount += existingCount
			}
			if totalCount > 0 {
				existing["avg_clock_speed"] = totalClock / float64(totalCount)
			}
			if knownRate {
				if prev, ok2 := existing["total_capacity"].(float64); ok2 {
					existing["total_capacity"] = prev + finalCapacity
				} else {
					existing["total_capacity"] = finalCapacity
				}
			}
		} else {
			miners[minerType] = minerSummary
		}
	}

	// Fluid extractors
	fluidExtractors := map[string]interface{}{}
	fluidExtractorDetails := map[string]interface{}{}

	for btype, count := range input.Buildings {
		typeLower := strings.ToLower(btype)
		if !(strings.Contains(typeLower, "frackingextractor") || strings.Contains(typeLower, "waterpump") ||
			strings.Contains(typeLower, "oilpump") || strings.Contains(typeLower, "resourcewell")) {
			continue
		}

		var extractorType string
		var baseRate int
		knownRate := true
		if strings.Contains(typeLower, "frackingextractor") {
			extractorType = "Fracking Unit"
			baseRate = 60
		} else if strings.Contains(typeLower, "waterpump") {
			extractorType = "Water Pump"
			baseRate = 120
			// Modded pumps (e.g. WaterPumpMk4) have unknown base rates
			if pumpMkRe.MatchString(btype) {
				knownRate = false
				extractorType = classNameFromPath(btype)
			}
		} else if strings.Contains(typeLower, "oilpump") {
			extractorType = "Oil Pump"
			baseRate = 120
			// Modded pumps (e.g. OilPumpMk3) have unknown base rates
			if pumpMkRe.MatchString(btype) {
				knownRate = false
				extractorType = classNameFromPath(btype)
			}
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

		isFracking := strings.Contains(typeLower, "frackingextractor")

		fluidTypes := map[string]int{}                           // fracking: resource -> count
		fluidTypesBySpeed := map[int]map[string]map[string]int{} // fracking: speed -> resource -> {count, purity...}
		clockSpeedDist := map[string]interface{}{}               // speed -> {count, production, fluid_types}
		purityCounts := map[string]int{}                         // purity -> count

		// Walk every instance of this extractor type once. We compute the real
		// output (base rate * purity multiplier * clock speed), feed the summary
		// stats, and build the per-instance detail row in the same pass.
		for _, inst := range sortedKeys(input.BuildingTypes) {
			instType := input.BuildingTypes[inst]
			instLower := strings.ToLower(instType)
			if !strings.Contains(instLower, typeLower) {
				continue
			}
			speed := 100.0
			if s, ok := input.ClockSpeeds[inst]; ok {
				speed = float64(s)
			}
			roundedSpeed := int(math.Round(speed))

			// Fracking and non-fracking extractors keep their purities in separate maps.
			purity := "normal"
			if isFracking {
				if p, ok := input.FrackingPurities[inst]; ok {
					purity = p
				}
			} else {
				if p, ok := input.MinerPurities[inst]; ok {
					purity = p
				}
			}

			purityMult := 1.0
			switch purity {
			case "pure":
				purityMult = 2.0
			case "impure":
				purityMult = 0.5
			}

			var production float64
			if knownRate {
				production = float64(baseRate) * purityMult * (speed / 100)
				actualCapacity += production
			}
			purityCounts[purity]++

			// Resource
			resource := "Unknown"
			if isFracking {
				if ft, ok := input.FrackingFluidTypes[inst]; ok {
					resource = ft
					fluidTypes[ft]++
				}
			} else {
				if r, ok := input.MinerResources[inst]; ok {
					resource = r
				} else if strings.Contains(typeLower, "water") {
					resource = "Desc_Water_C"
				} else if strings.Contains(typeLower, "oil") {
					resource = "Desc_LiquidOil_C"
				}
			}

			// Track which fluids appear at which clock speed (fracking only).
			if isFracking && resource != "Unknown" {
				if fluidTypesBySpeed[roundedSpeed] == nil {
					fluidTypesBySpeed[roundedSpeed] = map[string]map[string]int{}
				}
				if fluidTypesBySpeed[roundedSpeed][resource] == nil {
					fluidTypesBySpeed[roundedSpeed][resource] = map[string]int{}
				}
				fluidTypesBySpeed[roundedSpeed][resource]["count"]++
				fluidTypesBySpeed[roundedSpeed][resource][purity]++
			}

			// Clock speed distribution
			keyStr := intToStr(roundedSpeed)
			if dist, ok := clockSpeedDist[keyStr].(map[string]interface{}); ok {
				dist["count"] = dist["count"].(int) + 1
				if knownRate {
					dist["production"] = dist["production"].(float64) + production
				}
				if fts, ok := fluidTypesBySpeed[roundedSpeed]; ok {
					dist["fluid_types"] = fts
				}
			} else {
				entry := map[string]interface{}{
					"count":       1,
					"fluid_types": map[string]map[string]int{},
				}
				if knownRate {
					entry["production"] = production
				}
				if fts, ok := fluidTypesBySpeed[roundedSpeed]; ok {
					entry["fluid_types"] = fts
				}
				clockSpeedDist[keyStr] = entry
			}

			// Per-instance detail
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

			// Fracking extractors don't appear in power consumers - match to nearest smasher by position
			if loc == nil && isFracking {
				extractorBF, ok := input.BuildingFacts[inst]
				if ok && extractorBF.HasTransform {
					bestDist := -1.0
					var bestSmasher string
					for smasherInst, smasherBF := range input.BuildingFacts {
						if smasherBF == nil || !smasherBF.HasTransform {
							continue
						}
						if !strings.Contains(strings.ToLower(smasherBF.TypePath), "frackingsmasher") {
							continue
						}
						dx := float64(extractorBF.Translation[0] - smasherBF.Translation[0])
						dy := float64(extractorBF.Translation[1] - smasherBF.Translation[1])
						dz := float64(extractorBF.Translation[2] - smasherBF.Translation[2])
						dist := dx*dx + dy*dy + dz*dz
						if bestDist < 0 || dist < bestDist {
							bestDist = dist
							bestSmasher = smasherInst
						}
					}
					if bestSmasher != "" {
						for i := range input.PowerConsumption.Consumers {
							if input.PowerConsumption.Consumers[i].InstanceName == bestSmasher {
								loc = input.PowerConsumption.Consumers[i].Location
								break
							}
						}
					}
				}
			}

			detail := map[string]interface{}{
				"type":        extractorType,
				"clock_speed": speed,
				"purity":      purity,
				"resource":    resource,
				"output_pipes": func() []string {
					if connections != nil {
						return connections.Outputs
					}
					return []string{}
				}(),
			}
			if knownRate {
				detail["capacity"] = production
			}
			// Pipe bottleneck analysis
			var pipeOut []string
			if connections != nil {
				pipeOut = connections.Outputs
			}
			pipeTput, pipeLabel := outputThroughput(pipeOut, true)
			detail["belt_throughput"] = pipeTput
			detail["belt_label"] = pipeLabel
			var effective float64
			if knownRate {
				effective = production
				if pipeTput > 0 && production > pipeTput {
					effective = pipeTput
					detail["bottleneck"] = true
				} else {
					detail["bottleneck"] = false
				}
				detail["effective_output"] = effective
			} else {
				detail["bottleneck"] = false
			}
			if loc != nil {
				detail["location"] = loc
			}
			fluidExtractorDetails[inst] = detail

			// Aggregate by resource
			resKey := resource
			if resKey == "Unknown" {
				resKey = "Unknown Fluid"
			}
			if entry, ok := productionByResource[resKey].(map[string]interface{}); ok {
				entry["count"] = entry["count"].(int) + 1
				if knownRate {
					if prev, ok2 := entry["total_output"].(float64); ok2 {
						entry["total_output"] = prev + production
					} else {
						entry["total_output"] = production
					}
					if prev, ok2 := entry["effective_output"].(float64); ok2 {
						entry["effective_output"] = prev + effective
					} else {
						entry["effective_output"] = effective
					}
				}
				if detail["bottleneck"].(bool) {
					entry["bottlenecked"] = entry["bottlenecked"].(int) + 1
				}
			} else {
				bn := 0
				if detail["bottleneck"].(bool) {
					bn = 1
				}
				entry := map[string]interface{}{
					"count":        1,
					"bottlenecked": bn,
					"is_fluid":     true,
				}
				if knownRate {
					entry["total_output"] = production
					entry["effective_output"] = effective
				}
				productionByResource[resKey] = entry
			}
		}

		if len(clockSpeeds) > 0 {
			sum := 0.0
			for _, s := range clockSpeeds {
				sum += s
			}
			avgClock = sum / float64(len(clockSpeeds))
		}

		fluidSummary := map[string]interface{}{
			"count":                    count,
			"avg_clock_speed":          avgClock,
			"clock_speed_distribution": clockSpeedDist,
			"fluid_types":              fluidTypes,
			"purity_counts":            purityCounts,
		}
		if knownRate {
			fluidSummary["base_rate"] = baseRate
			fluidSummary["total_capacity"] = actualCapacity
		}
		// Accumulate if multiple mod paths map to the same extractorType
		if existingIface, ok := fluidExtractors[extractorType]; ok {
			existing := existingIface.(map[string]interface{})
			existing["count"] = existing["count"].(int) + count
			existingPurities := existing["purity_counts"].(map[string]int)
			for p, c := range purityCounts {
				existingPurities[p] += c
			}
			existingDist := existing["clock_speed_distribution"].(map[string]interface{})
			for k, vIface := range clockSpeedDist {
				v := vIface.(map[string]interface{})
				if d, ok2 := existingDist[k].(map[string]interface{}); ok2 {
					d["count"] = d["count"].(int) + v["count"].(int)
					if knownRate {
						if prev, ok3 := d["production"].(float64); ok3 {
							if add, ok4 := v["production"].(float64); ok4 {
								d["production"] = prev + add
							}
						} else if add, ok4 := v["production"].(float64); ok4 {
							d["production"] = add
						}
					}
				} else {
					existingDist[k] = v
				}
			}
			existingFluids := existing["fluid_types"].(map[string]int)
			for ft, c := range fluidTypes {
				existingFluids[ft] += c
			}
			totalClock := avgClock * float64(len(clockSpeeds))
			totalCount := len(clockSpeeds)
			existingAvg := existing["avg_clock_speed"].(float64)
			existingCount := existing["count"].(int) - count
			if existingCount > 0 {
				totalClock += existingAvg * float64(existingCount)
				totalCount += existingCount
			}
			if totalCount > 0 {
				existing["avg_clock_speed"] = totalClock / float64(totalCount)
			}
			if knownRate {
				if prev, ok2 := existing["total_capacity"].(float64); ok2 {
					existing["total_capacity"] = prev + actualCapacity
				} else {
					existing["total_capacity"] = actualCapacity
				}
			}
		} else {
			fluidExtractors[extractorType] = fluidSummary
		}
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
	for _, inst := range sortedKeys(input.BuildingTypes) {
		typePath := input.BuildingTypes[inst]
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
				resource = "Desc_Water_C"
			}
		} else if strings.Contains(typeLower, "water") {
			resource = "Desc_Water_C"
		} else if strings.Contains(typeLower, "oil") {
			resource = "Desc_LiquidOil_C"
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
		"miners":                  miners,
		"miner_details":           minerDetails,
		"fluid_extractors":        fluidExtractors,
		"fluid_extractor_details": fluidExtractorDetails,
		"production_by_resource":  productionByResource,
		"production_buildings":    productionBuildings,
		"extractor_bottlenecks":   extractorBottlenecks,
	}
}
