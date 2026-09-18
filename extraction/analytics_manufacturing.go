package extraction

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

func calculateManufacturingAnalytics(input AnalyticsInput) map[string]interface{} {
	if input.BuildingTypes == nil || input.ClockSpeeds == nil {
		return nil
	}

	manufacturingTypes := []string{"smelter", "constructor", "assembler", "manufacturer", "foundry",
		"refinery", "blender", "packager", "hadron", "quantum", "converter"}

	activeProduction := map[string]interface{}{}
	var waitingBuildings, blockedBuildings, pausedBuildings, idleBuildings []interface{}
	totalManufacturingBuildings := 0

	// Create consumer lookup map
	consumerMap := map[string]*PowerConsumerEntry{}
	for i := range input.PowerConsumption.Consumers {
		consumerMap[input.PowerConsumption.Consumers[i].InstanceName] = &input.PowerConsumption.Consumers[i]
	}

	for _, inst := range sortedKeys(input.BuildingTypes) {
		typePath := input.BuildingTypes[inst]
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

		// addToRecipe adds a building to the activeProduction map grouped by recipe.
		// Called for ALL buildings with recipes, regardless of production status.
		addToRecipe := func() {
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
					"sloop_mults":  []float64{},
					"count":        0,
				}
			}
			entry := activeProduction[recipePath].(map[string]interface{})
			sloopMult := 1.0
			if sloopCount, ok := input.ProductionBoosts[inst]; ok && sloopCount > 0 {
				maxSlots := 1
				for key, slots := range somersloopMaxSlots {
					if strings.Contains(typeLower, key) {
						maxSlots = slots
						break
					}
				}
				sloopMult = 1.0 + float64(sloopCount)/float64(maxSlots)
			}
			bldg := map[string]interface{}{
				"instance":     inst,
				"type":         typePath,
				"clock_speed":  speed,
				"input_belts":  getConnections("inputs"),
				"output_belts": getConnections("outputs"),
			}
			if sloopCount, ok := input.ProductionBoosts[inst]; ok && sloopCount > 0 {
				bldg["slooped"] = int(sloopCount)
			}
			if loc != nil {
				bldg["location"] = loc
			}
			entry["buildings"] = append(entry["buildings"].([]interface{}), bldg)
			entry["clock_speeds"] = append(entry["clock_speeds"].([]float64), speed)
			entry["sloop_mults"] = append(entry["sloop_mults"].([]float64), sloopMult)
			entry["count"] = entry["count"].(int) + 1
		}

		if status == "Paused" && !isActivelyProducing {
			entry := map[string]interface{}{
				"instance":     inst,
				"type":         typePath,
				"recipe":       recipeName,
				"clock_speed":  speed,
				"power_mw":     actualMW,
				"reason":       "Manually paused",
				"input_belts":  getConnections("inputs"),
				"output_belts": getConnections("outputs"),
			}
			if loc != nil {
				entry["location"] = loc
			}
			waitingBuildings = append(waitingBuildings, entry)
			addToRecipe()
		} else if status == "Blocked" && !isActivelyProducing {
			entry := map[string]interface{}{
				"instance":     inst,
				"type":         typePath,
				"recipe":       recipeName,
				"clock_speed":  speed,
				"power_mw":     actualMW,
				"reason":       "Output full (waiting for space)",
				"input_belts":  getConnections("inputs"),
				"output_belts": getConnections("outputs"),
			}
			if loc != nil {
				entry["location"] = loc
			}
			blockedBuildings = append(blockedBuildings, entry)
			addToRecipe()
		} else if status == "Starving" && !isActivelyProducing {
			entry := map[string]interface{}{
				"instance":     inst,
				"type":         typePath,
				"recipe":       recipeName,
				"clock_speed":  speed,
				"power_mw":     actualMW,
				"reason":       "No input items (waiting for material)",
				"input_belts":  getConnections("inputs"),
				"output_belts": getConnections("outputs"),
			}
			if loc != nil {
				entry["location"] = loc
			}
			waitingBuildings = append(waitingBuildings, entry)
			addToRecipe()
		} else if status == "Running" || isActivelyProducing {
			addToRecipe()
		} else {
			// Unknown status (e.g. modded buildings without power consumer data)
			if actualMW > 0.05 {
				addToRecipe()
			} else {
				// Buildings with recipe but no power consumer data (e.g. modded).
				reason := "Idle (no power data)"
				if actualMW > 0 {
					reason = "Unknown status"
				}
				entry := map[string]interface{}{
					"instance":     inst,
					"type":         typePath,
					"recipe":       recipeName,
					"clock_speed":  speed,
					"power_mw":     actualMW,
					"reason":       reason,
					"input_belts":  getConnections("inputs"),
					"output_belts": getConnections("outputs"),
				}
				if loc != nil {
					entry["location"] = loc
				}
				idleBuildings = append(idleBuildings, entry)
				addToRecipe()
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
		sloopMults := entry["sloop_mults"].([]float64)
		if recipeData, ok := entry["recipe_data"].(*Recipe); ok && recipeData != nil {
			totalOutput := map[string]interface{}{}
			actualTotal := 0.0
			theoreticalTotal := 0.0
			for _, product := range recipeData.Products {
				baseRate := (product.Amount / recipeData.Duration) * 60 // items per minute
				totalProduction := 0.0
				for i, s := range speeds {
					mult := 1.0
					if i < len(sloopMults) {
						mult = sloopMults[i]
					}
					totalProduction += baseRate * (s / 100) * mult
				}
				theoreticalMax := baseRate * float64(count) // at 100% clock, no sloops
				avgRate := 0.0
				if count > 0 {
					avgRate = totalProduction / float64(count)
				}
				totalOutput[product.Item] = map[string]interface{}{
					"total_per_minute": math.Round(totalProduction*10) / 10,
					"avg_per_building": math.Round(avgRate*10) / 10,
					"base_rate":        math.Round(baseRate*10) / 10,
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
	sort.Slice(idleBuildings, func(i, j int) bool {
		return getBuildingName(idleBuildings[i]) < getBuildingName(idleBuildings[j])
	})

	return map[string]interface{}{
		"total_manufacturing_buildings": totalManufacturingBuildings,
		"active_production":             sortedActive,
		"active_count":                  activeCount,
		"total_recipes_in_use":          len(sorted),
		"bottlenecks": map[string]interface{}{
			"waiting":       waitingBuildings,
			"blocked":       blockedBuildings,
			"paused":        pausedBuildings,
			"idle":          idleBuildings,
			"waiting_count": len(waitingBuildings),
			"blocked_count": len(blockedBuildings),
			"paused_count":  len(pausedBuildings),
			"idle_count":    len(idleBuildings),
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

	// Phases: 0 = pre-SE, 1-5 = SE phases, 6+ = post-game
	isCompleted := currentPhaseNum >= 5

	// Cap at 5 for post-game saves
	spaceElevatorPhase := currentPhaseNum
	if spaceElevatorPhase > 5 {
		spaceElevatorPhase = 5
	}

	// Post-game: target wraps back to 1
	targetPhase := targetPhaseNum
	if isCompleted && (targetPhase == 0 || targetPhase == 1) {
		targetPhase = 5
	}
	if targetPhase > 5 {
		targetPhase = 5
	}
	if targetPhase == 0 && !isCompleted {
		targetPhase = 1
	}

	// HUB tier from purchased schematics. The schematic number prefix (X in X-Y)
	// does NOT always match the in-game tier (mTechTier). Use a static map.
	schematicTier := map[string]int{
		"1-1": 1, "1-2": 1, "1-3": 1,
		"2-1": 2, "2-2": 2, "2-3": 2, "2-5": 2, "3-2": 2,
		"3-1": 3, "3-3": 3, "3-4": 3, "4-2": 3,
		"4-1": 4, "4-3": 4, "4-4": 4, "4-5": 4, "5-3": 4,
		"5-1": 5, "5-4": 5, "5-5": 5, "6-1": 5, "6-2": 5,
		"5-2": 6, "6-3": 6, "6-5": 6, "6-6": 6, "6-7": 6,
		"7-1": 7, "7-2": 7, "7-3": 7, "7-5": 7, "8-3": 7,
		"7-4": 8, "8-1": 8, "8-2": 8, "8-4": 8, "8-5": 8,
		"9-1": 9, "9-2": 9, "9-3": 9, "9-4": 9, "9-5": 9,
	}
	maxHubTier := 0
	for _, s := range gp.PurchasedSchematics {
		if tier, ok := schematicTier[s]; ok && tier > maxHubTier {
			maxHubTier = tier
		}
	}

	// MAM research trees
	mamTrees := []string{}
	seen := map[string]bool{}
	treeMap := map[string]string{
		"Caterium":        "Caterium",
		"Quartz":          "Quartz",
		"Sulfur":          "Sulfur",
		"Nutrients":       "Nutrients",
		"Mycelia":         "Mycelia",
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
		} else {
			name = strings.TrimSuffix(name, "_ResearchTree")
		}
		if !seen[name] {
			seen[name] = true
			mamTrees = append(mamTrees, name)
		}
	}

	// MAM progress
	mamProgress := map[string]interface{}{}
	treeTotals := map[string]int{
		"Caterium":         16,
		"Quartz":           12,
		"Sulfur":           16,
		"Nutrients":        5,
		"Mycelia":          10,
		"Power Slugs":      6,
		"Alien Technology": 19,
		"Flower Petals":    11,
		"Alien Organisms":  11,
		"XMas":             13,
	}
	if isV12OrLater {
		treeTotals["Caterium"] = 17
		treeTotals["Alien Technology"] = 20
	}

	// treeSchematicPrefixes maps each MAM tree to the schematic name prefixes
	// that belong to it. Schematics follow the pattern Research_<Prefix>_*.
	treeSchematicPrefixes := map[string][]string{
		"Caterium":         {"Research_Caterium_"},
		"Quartz":           {"Research_Quartz_"},
		"Sulfur":           {"Research_Sulfur_"},
		"Nutrients":        {"Research_Nutrients_"},
		"Mycelia":          {"Research_Mycelia_"},
		"Power Slugs":      {"Research_PowerSlugs_"},
		"Flower Petals":    {"Research_FlowerPetals_", "Research_FlowerPetal_"},
		"Alien Organisms":  {"Research_AO_", "Research_ACarapace_", "Research_AOrganisms_", "Research_AOrgans_"},
		"Alien Technology": {"Research_Alien_"},
		"XMas":             {"Research_XMas_"},
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
		isUnknownTree := total == 0 && prefixes == nil
		for _, s := range gp.PurchasedSchematics {
			if strings.HasSuffix(s, "_Hidden") {
				continue
			}
			if isUnknownTree {
				// For modded trees with no hardcoded prefixes, match schematics
				// that start with the tree name (e.g. InfiniteZoop_ResearchNode1).
				if strings.HasPrefix(s, tree+"_") {
					completed++
				}
				continue
			}
			if !strings.HasPrefix(s, "Research_") {
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
		// Cap completed at total - FICSMAS event adds bonus research nodes
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
		// Unknown trees (modded) use total=-1 to signal "unknown" to the viewer.
		if isUnknownTree {
			total = -1
		}
		mamProgress[tree] = map[string]interface{}{
			"completed":  completed,
			"total":      total,
			"percentage": percentage,
		}
	}

	return map[string]interface{}{
		"space_elevator": map[string]interface{}{
			"current_phase":        gp.CurrentPhase,
			"current_phase_number": spaceElevatorPhase,
			"target_phase":         gp.TargetPhase,
			"target_phase_number":  targetPhase,
			"is_completed":         isCompleted,
		},
		"hub": map[string]interface{}{
			"current_tier": maxHubTier,
			"max_tier":     9,
		},
		"mam": map[string]interface{}{
			"unlocked_trees": mamTrees,
			"unlocked_count": len(mamTrees),
			"is_activated":   gp.ResearchActivated,
			"progress":       mamProgress,
		},
		"schematics": map[string]interface{}{
			"purchased_count":      len(gp.PurchasedSchematics),
			"last_active":          gp.LastActiveSchematic,
			"purchased_schematics": gp.PurchasedSchematics,
		},
		"research": map[string]interface{}{
			"activated":               gp.ResearchActivated,
			"unlocked_trees_count":    len(gp.UnlockedResearchTrees),
			"last_used_hard_drive_id": gp.LastUsedHardDriveID,
			"unlocked_trees":          gp.UnlockedResearchTrees,
		},
		"awesome_sink": map[string]interface{}{
			"coupons":    gp.SinkCoupons,
			"points":     gp.SinkPoints,
			"dna_points": gp.SinkDNAPoints,
		},
	}
}
