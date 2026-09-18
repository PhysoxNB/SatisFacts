package extraction

import (
	"math"
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

	var geoGens []struct{ mw, offset float64 }
	geoTypeKey := ""

	for _, gen := range input.GeneratorPower.Generators {
		typeLower := strings.ToLower(gen.TypePath)
		key := gen.TypePath
		info, ok := genTypes[key]
		if !ok {
			info = &genTypeInfo{purities: map[string]int{}}
			genTypes[key] = info
		}
		info.count++
		info.totalMW += gen.TheoreticalMaxMW
		info.clockSpeeds = append(info.clockSpeeds, float64(gen.OutputPercent))

		if gen.Status == "Running" {
			info.activeCount++
			info.activeMW += gen.ActualMW
		} else {
			info.standbyCount++
			info.standbyMW += gen.ActualMW
		}

		// Collect geothermal generators for phase-shift computation
		if strings.Contains(typeLower, "geothermal") {
			geoGens = append(geoGens, struct{ mw, offset float64 }{gen.ActualMW, float64(gen.VariablePowerCycleOffset)})
			geoTypeKey = key
		}
	}

	// Compute phase shift and classify geothermal purities
	if len(geoGens) > 0 && geoTypeKey != "" {
		info := genTypes[geoTypeKey]
		info.purities = map[string]int{}
		// Infer purity from power output using triangle wave phase-shift fitting.
		// NodePuritySetting (AllPure etc.) only affects miner resource nodes,
		// not geothermal geyser nodes, so we always use fitting.
		phaseShift := computeGeothermalPhaseShift(geoGens)
		for _, g := range geoGens {
			offset := float32(g.offset)
			purity := classifyGeothermalPurity(g.mw, offset, phaseShift)
			info.purities[purity]++
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
			"count":           info.count,
			"active":          info.activeCount,
			"standby":         info.standbyCount,
			"total_mw":        info.totalMW,
			"active_mw":       info.activeMW,
			"standby_mw":      info.standbyMW,
			"avg_clock_speed": math.Round(avgClock*10) / 10,
		}
		if isGeothermal {
			// Geothermal: use actual extracted MW, no hardcoded base
			avgMW := 0.0
			if info.count > 0 {
				avgMW = info.totalMW / float64(info.count)
			}
			entry["avg_mw"] = math.Round(avgMW*10) / 10

			// Calculate min/max range from purities.
			// Wiki: output fluctuates between 0.5x and 1.5x average.
			// Impure: avg 100 MW (50-150), Normal: avg 200 MW (100-300), Pure: avg 400 MW (200-600).
			geoAvgMW := map[string]float64{"impure": 100, "normal": 200, "pure": 400}
			var sumMinMW, sumMaxMW, sumAvgMW float64
			for purity, cnt := range info.purities {
				avg := geoAvgMW[purity]
				sumAvgMW += avg * float64(cnt)
				sumMinMW += avg * 0.5 * float64(cnt)
				sumMaxMW += avg * 1.5 * float64(cnt)
			}
			entry["geo_min_mw"] = math.Round(sumMinMW*10) / 10
			entry["geo_max_mw"] = math.Round(sumMaxMW*10) / 10
			entry["geo_avg_mw"] = math.Round(sumAvgMW*10) / 10
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

	// Power storage - calculate from actual building count and stored energy
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
	perUnitCapacity := 100.0   // MWh per PowerStorage Mk1
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
		"building_count":            apaCount,
		"fueled_count":              apaFueled,
		"unfueled_count":            apaUnfueled,
		"base_power_mw":             apaBasePower,
		"multiplier":                apaMultiplier,
		"augmented_capacity_mw":     augmentedCapacity,
		"max_augmented_capacity_mw": maxAugmentedCapacity,
	}

	// Locomotives pull off the rail network but have no power info component
	// on them at all, so the save simply never records what they're drawing.
	// We count the 25 MW they always need, even parked, and surface the rest
	// as a known unknown in the UI. Real draw climbs to 110 MW accelerating.
	locomotiveCount := input.Trains.Locomotives
	locomotiveBaseMW := getConsumerBasePower("locomotive")
	locomotiveMW := float64(locomotiveCount) * locomotiveBaseMW
	locomotiveMaxMW := float64(locomotiveCount) * locomotiveMaxDrawMW

	buildingConsumption := input.PowerConsumption.TotalActualMW
	totalConsumption := buildingConsumption + locomotiveMW

	// Consumer summary
	consumerSummary := map[string]interface{}{
		"total_theoretical_mw": input.PowerConsumption.TotalTheoreticalMW + locomotiveMaxMW,
		"total_actual_mw":      totalConsumption,
		"building_actual_mw":   buildingConsumption,
		"active_buildings":     input.PowerConsumption.ActiveBuildings,
		"paused_buildings":     input.PowerConsumption.PausedBuildings,
	}

	locomotives := map[string]interface{}{
		"count":        locomotiveCount,
		"base_mw":      locomotiveBaseMW,
		"max_draw_mw":  locomotiveMaxDrawMW,
		"counted_mw":   locomotiveMW,
		"upper_est_mw": locomotiveMaxMW,
	}

	// Power balance (includes APA contribution)
	capacityWithAPA := augmentedCapacity
	surplus := capacityWithAPA - totalConsumption

	return map[string]interface{}{
		"generators": map[string]interface{}{
			"total_capacity_mw":   input.GeneratorPower.TotalCapacityMW,
			"active_capacity_mw":  input.GeneratorPower.ActiveCapacityMW,
			"standby_capacity_mw": input.GeneratorPower.StandbyCapacityMW,
			"theoretical_max_mw":  input.GeneratorPower.TheoreticalMaxMW,
			"total_generators":    input.GeneratorPower.TotalGenerators,
			"active_generators":   input.GeneratorPower.ActiveGenerators,
			"standby_generators":  input.GeneratorPower.StandbyGenerators,
			"by_type":             genSummary,
		},
		"consumers":     consumerSummary,
		"power_storage": powerStorage,
		"apa":           apa,
		"locomotives":   locomotives,
		"power_balance": map[string]interface{}{
			"capacity_mw":               capacityWithAPA,
			"max_capacity_mw":           maxAugmentedCapacity,
			"generator_capacity_mw":     input.GeneratorPower.TotalCapacityMW,
			"active_capacity_mw":        input.GeneratorPower.ActiveCapacityMW,
			"standby_capacity_mw":       input.GeneratorPower.StandbyCapacityMW,
			"theoretical_max_mw":        input.GeneratorPower.TheoreticalMaxMW,
			"apa_base_power_mw":         apaBasePower,
			"apa_multiplier":            apaMultiplier,
			"consumption_mw":            totalConsumption,
			"building_consumption_mw":   buildingConsumption,
			"locomotive_consumption_mw": locomotiveMW,
			"surplus_mw":                surplus,
		},
	}
}
