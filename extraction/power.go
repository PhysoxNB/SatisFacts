package extraction

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var generatorTypeRegex = regexp.MustCompile(`Build_Generator(\w+)`)

// generatorBasePower maps generator type names to their base power output at 100% clock speed.
var generatorBasePower = map[string]float64{
	"coal":      75.0,
	"fuel":      250.0,
	"nuclear":   2500.0,
	"biomass":   30.0,
	"geothermal": 0, // variable, calculated from actual production
}

// getGeneratorBasePower returns the base power output for a generator type at 100% clock speed.
func getGeneratorBasePower(typeLower string) float64 {
	for key, mw := range generatorBasePower {
		if strings.Contains(typeLower, key) {
			return mw
		}
	}
	return 0
}

// ExtractGeneratorPower extracts power output and standby status for all generators.
func ExtractGeneratorPower(buildingTypes map[string]string, buildingFacts map[string]*BuildingFacts, resourceNodeFacts map[string]*ResourceNodeFacts, inventoryStacks map[string][]InventoryStack) GeneratorPowerData {
	data := GeneratorPowerData{}

	for instanceName, typePath := range buildingTypes {
		typeLower := strings.ToLower(typePath)

		// Check if this is a generator
		isGenerator := strings.Contains(typeLower, "generator") || strings.Contains(typeLower, "biomass")
		if !isGenerator {
			continue
		}
		// Exclude integrated (non-biomass) generators
		if strings.Contains(typeLower, "integrated") && !strings.Contains(typeLower, "biomass") {
			continue
		}

		bf, ok := buildingFacts[instanceName]
		if !ok || bf == nil {
			continue
		}

		data.TotalGenerators++

		// Determine if generator is enabled
		hasIsProducing := bf.HasIsProducing
		hasProductivityMonitor := bf.HasProductivityMonitor
		hasProductionDuration := bf.HasProductionDuration
		isProductionPaused := bf.IsProductionPaused
		isEnabled := (hasIsProducing || hasProductivityMonitor || hasProductionDuration) && !isProductionPaused

		// Check power connection
		hasPowerConnection := bf.WireCount > 0

		// Get power info (now inlined on BuildingFacts)
		var actualMW, maxMW float64
		if bf.HasDynamicProductionCapacity {
			actualMW = float64(bf.DynamicProductionCapacity)
		}
		if bf.HasBaseProduction {
			maxMW = float64(bf.BaseProduction)
		} else if actualMW > 0 {
			maxMW = actualMW
		}
		// Geothermal: no mDynamicProductionCapacity
		if !bf.HasDynamicProductionCapacity {
			if isEnabled && maxMW > 0 {
				actualMW = maxMW
			} else if !isEnabled {
				actualMW = 0
			}
		}

		// Theoretical max = base power × clock speed.
		var theoreticalMaxMW float64
		basePower := getGeneratorBasePower(typeLower)
		if basePower > 0 {
			clockSpeed := 1.0 // default 100%
			if bf.HasCurrentPotential && bf.CurrentPotential > 0 {
				clockSpeed = float64(bf.CurrentPotential)
			}
			theoreticalMaxMW = basePower * clockSpeed
		} else if maxMW > 0 {
			// Geothermal or unknown: use actual maxMW as theoretical
			theoreticalMaxMW = maxMW
		}
		data.TheoreticalMaxMW += theoreticalMaxMW

		// Determine status
		var status, reason string
		if !hasPowerConnection {
			status = "Standby"
			reason = "Not connected to power network"
			actualMW = 0
			data.StandbyGenerators++
			data.StandbyCapacityMW += maxMW
		} else if !isEnabled {
			status = "Standby"
			reason = "Disabled/Paused"
			actualMW = 0
			data.StandbyGenerators++
			data.StandbyCapacityMW += maxMW
		} else if actualMW == 0 {
			status = "Standby"
			reason = "No Fuel / Not Producing"
			data.StandbyGenerators++
			data.StandbyCapacityMW += maxMW
		} else if actualMW < maxMW*0.95 {
			status = "Active"
			reason = "Fuel-Starved"
			data.ActiveGenerators++
			data.ActiveCapacityMW += actualMW
		} else {
			status = "Active"
			reason = "Producing"
			data.ActiveGenerators++
			data.ActiveCapacityMW += actualMW
		}
		data.TotalCapacityMW += maxMW

		// Generator type
		generatorType := "Unknown"
		if m := generatorTypeRegex.FindStringSubmatch(typePath); m != nil {
			generatorType = m[1]
		}

		// Power shard count
		powerShardCount := 0
		if bf.HasCurrentPotential {
			potential := bf.CurrentPotential
			if potential >= 2.0 {
				powerShardCount = 3
			} else if potential >= 1.5 {
				powerShardCount = 2
			} else if potential >= 1.0 {
				powerShardCount = 1
			}
		}

		// Fuel type
		fuelType := extractFuelType(bf, resourceNodeFacts, inventoryStacks)

		outputPercent := 0
		if maxMW > 0 {
			outputPercent = int((actualMW / maxMW) * 100)
		}

		// Extract location from transform
		var genLoc *ConsumerLocation
		if bf.HasTransform {
			pos := bf.Translation
			genLoc = &ConsumerLocation{
				X: formatLocationNum(float64(pos[0]) / 100.0),
				Y: formatLocationNum(float64(pos[1]) / 100.0),
				Z: formatLocationNum(float64(pos[2]) / 100.0),
			}
		}

		data.Generators = append(data.Generators, GeneratorEntry{
			InstanceName:     instanceName,
			Type:             generatorType,
			TypePath:         typePath,
			Status:           status,
			ActualMW:         actualMW,
			MaxMW:            maxMW,
			TheoreticalMaxMW: theoreticalMaxMW,
			Reason:           reason,
			OutputPercent:    outputPercent,
			FuelType:         fuelType,
			PowerShardCount:  powerShardCount,
			Location:         genLoc,
		})
	}

	return data
}

func extractFuelType(bf *BuildingFacts, resourceNodeFacts map[string]*ResourceNodeFacts, inventoryStacks map[string][]InventoryStack) string {
	// Check mExtractableResource (geothermal)
	if bf.ExtractableResource != "" {
		if rf, ok2 := resourceNodeFacts[bf.ExtractableResource]; ok2 && rf != nil {
			if fuel := extractDescName(rf.ResourceClassOverride); fuel != "" {
				return fuel
			}
		}
	}

	// Check mCurrentFuelClass
	if bf.CurrentFuelClass != "" {
		if fuel := extractDescName(bf.CurrentFuelClass); fuel != "" {
			return fuel
		}
	}

	// Check fuel inventory
	if bf.FuelInventory != "" {
		if fuel := extractFuelFromInventoryRef(bf.FuelInventory, inventoryStacks); fuel != "" {
			return fuel
		}
	}

	// Check input inventory
	if bf.InputInventory != "" {
		if fuel := extractFuelFromInventoryRef(bf.InputInventory, inventoryStacks); fuel != "" {
			return fuel
		}
	}

	return ""
}

var buildingTypeRegex = regexp.MustCompile(`Build_(\w+)`)

// getConsumerBasePower looks up the base power consumption for a building type from power_data.json
func getConsumerBasePower(typeLower string) float64 {
	if powerDataLoaded == nil {
		return 0
	}
	for key, cd := range powerDataLoaded.Consumers {
		if strings.Contains(typeLower, strings.ToLower(key)) {
			return cd.BaseMW
		}
	}
	return 0
}

// expectedConsumerPower calculates expected power consumption at a given clock speed.
// Formula from power_data.json: power = base * (clock_speed/100)^1.321928
func expectedConsumerPower(baseMW float64, clockSpeedPct float32) float64 {
	if baseMW <= 0 {
		return 0
	}
	multiplier := math.Pow(float64(clockSpeedPct)/100.0, 1.321928)
	return baseMW * multiplier
}

// isNonPowerConsumer returns true for buildings that have mPowerInfo but never consume power.
func isNonPowerConsumer(typeLower string) bool {
	nonPower := []string{
		"storage", "container", "tank", "dimdepot", "centralstorage",
		"pipelinejunction", "pipeconnection", "fgpipe",
		"conveyormerger", "conveyorsplitter", "programmable", "smart",
		"passthrough", "wallhole", "floorhole",
		"powerpole", "powertower", "powerline", "powerswitch",
		"powerstorage",
		"tradingpost", "tradestation",
		"hub", "elevator", "spacelevator",
		"portal", "sink", "awesomesink",
		"blueprintdesigner",
		"sign", "lamp", "light", "beacon",
		"door", "wall", "foundation", "ramp", "walkway", "roof",
		"beam", "pillar", "barrier", "gate",
		"conveyorbelt", "conveyorlift",
		"pipeline", "pipepump", "pipesupport",
		"hypertube", "hyper",
		"railroad", "rail", "switch", "signal",
		"truckstation", "freightplatform",
		"vehiclepathnode",
		"jumppad", "launchpad",
		"ladder", "stairs",
		"zipline", "parachute",
		"waterfall", "fog",
	}
	for _, e := range nonPower {
		if strings.Contains(typeLower, e) {
			return true
		}
	}
	return false
}

// ExtractPowerConsumption collects power consumption and bottleneck status for all consumers.
func ExtractPowerConsumption(
	buildingFacts map[string]*BuildingFacts,
	resourceNodeFacts map[string]*ResourceNodeFacts,
	resolvedInventories map[string]ResolvedInventory,
	buildingRecipes map[string]string,
	clockSpeeds map[string]float32,
	productionBoosts map[string]float32,
	extractorProgress map[string]float32,
) PowerConsumptionData {
	data := PowerConsumptionData{}

	for instanceName, bf := range buildingFacts {
		if bf == nil {
			continue
		}

		typePath := bf.TypePath
		typeLower := strings.ToLower(typePath)

		// Skip generators — we only want consumers
		if strings.Contains(typeLower, "generator") {
			continue
		}

		// Must have mPowerInfo to be a power consumer
		if bf.PowerInfoRef == "" {
			continue
		}

		// Get power consumption from inlined power info fields
		var currentMW float64
		if bf.HasTargetConsumption {
			currentMW = float64(bf.TargetConsumption)
		}

		// Check if building is producing/active
		hasIsProducing := bf.HasIsProducing
		hasProductivityMonitor := bf.HasProductivityMonitor
		hasProductionDuration := bf.HasProductionDuration
		isProductionPaused := bf.IsProductionPaused
		hasPowerConsumption := currentMW > 0.5
		isActive := (hasIsProducing || hasProductivityMonitor || hasProductionDuration || hasPowerConsumption) && !isProductionPaused

		// Check if this is an extractor (miners, pumps, oil extractors)
		isExtractor := strings.Contains(typeLower, "miner") || strings.Contains(typeLower, "waterpump") ||
			strings.Contains(typeLower, "waterextractor") || strings.Contains(typeLower, "oilextractor") ||
			strings.Contains(typeLower, "oilpump") || strings.Contains(typeLower, "fracking") ||
			strings.Contains(typeLower, "resourcewell")

		// Check inventory to determine actual state (starving, blocked, or running)
		buildingInventory, hasInventoryData := resolvedInventories[instanceName]
		isStarving := false
		isBlocked := false

		if hasInventoryData {
			// Extractors have no inputs, so they can never be starving
			if !isExtractor {
				hasInput := len(buildingInventory.InputStacks) > 0
				totalInputCount := float64(0)
				for _, stack := range buildingInventory.InputStacks {
					totalInputCount += stack.Count
				}
				isStarving = !hasInput || totalInputCount == 0
			}

			// Check if output is full (blocked)
			hasOutput := len(buildingInventory.OutputStacks) > 0
			hasFullOutput := false

			if hasOutput {
				recipePath := buildingRecipes[instanceName]
				perCycleOutputs := GetPerCycleOutputs(recipePath)

				clockSpeed := float32(100)
				if cs, ok := clockSpeeds[instanceName]; ok {
					clockSpeed = cs
				}
				productionBoost := float32(1.0)
				if pb, ok := productionBoosts[instanceName]; ok {
					productionBoost = pb
				}
				totalMultiplier := float64(clockSpeed) / 100.0 * float64(productionBoost)

				hasRecipeData := len(perCycleOutputs) > 0

				for _, stack := range buildingInventory.OutputStacks {
					if stack.Item == "" {
						continue
					}
					stackSize := GetBuildingOutputStackSize(stack.Item)

					if hasRecipeData {
						// Use per-cycle output calculation
						outputPerCycle := float64(0)
						for _, p := range perCycleOutputs {
							if p.Item == stack.Item {
								outputPerCycle = p.Amount
								break
							}
						}
						nextCycleOutput := outputPerCycle * totalMultiplier
						if nextCycleOutput > 0 && (stack.Count+nextCycleOutput) > float64(stackSize) {
							hasFullOutput = true
							break
						}
					} else {
						// Fallback: use 80% threshold
						if stack.Count >= float64(stackSize)*0.80 {
							hasFullOutput = true
							break
						}
					}
				}
			}
			isBlocked = hasFullOutput
		}

		// Process power consumption
		var actualMW, maxMW float64
		status := "Unknown"
		reason := ""

		// Calculate expected power at current clock speed for low-power detection
		basePowerMW := getConsumerBasePower(typeLower)
		clockSpeedPct := float32(100)
		if cs, ok := clockSpeeds[instanceName]; ok && cs > 0 {
			clockSpeedPct = cs
		}
		expectedMW := expectedConsumerPower(basePowerMW, clockSpeedPct)
		// Apply somersloop power penalty: power = expected × (1 + sloopCount/maxSlots)^2
		if sloopCount, ok := productionBoosts[instanceName]; ok && sloopCount > 0 {
			maxSlots := 1
			for key, slots := range somersloopMaxSlots {
				if strings.Contains(typeLower, key) {
					maxSlots = slots
					break
				}
			}
			boostFraction := float64(sloopCount) / float64(maxSlots)
			sloopPowerMult := math.Pow(1.0+boostFraction, 2)
			expectedMW *= sloopPowerMult
		}
		// Consumption matching expected power (within 50%) means the building is running.
		isRunningAtLowClock := expectedMW > 0 && currentMW >= expectedMW*0.5

		if currentMW > 0 {
			if !isActive && currentMW < 1.0 {
				// Paused building with standby power
				maxMW = -1
				actualMW = 0
				status = "Paused"
				reason = "Manually paused or no recipe"
			} else if isActive && isRunningAtLowClock {
				// Building is running at low clock speed — consumption matches expected formula
				maxMW = expectedMW
				actualMW = currentMW

				if hasInventoryData {
					if isBlocked {
						status = "Blocked"
						reason = "Output full (waiting for space)"
					} else if isStarving && !isExtractor {
						status = "Starving"
						reason = "No input items (waiting for material)"
					} else {
						status = "Running"
						reason = "Producing at " + fmt.Sprintf("%.0f%%", clockSpeedPct) + " clock"
					}
				} else {
					if isExtractor {
						extractProgress, hasEP := extractorProgress[instanceName]
						if hasEP && extractProgress < 0.1 {
							status = "Blocked"
							reason = "Output full (waiting for space)"
							isBlocked = true
						} else {
							status = "Running"
							reason = "Extracting at " + fmt.Sprintf("%.0f%%", clockSpeedPct) + " clock"
						}
					} else {
						status = "Running"
						reason = "Producing at " + fmt.Sprintf("%.0f%%", clockSpeedPct) + " clock"
					}
				}
			} else if isActive && currentMW < 0.5 {
				// Active but consuming standby power (0.1 MW) — not actively producing
				maxMW = -1
				actualMW = currentMW

				if hasInventoryData {
					if isBlocked {
						status = "Blocked"
						reason = "Output full (waiting for space)"
					} else if isStarving && !isExtractor {
						status = "Starving"
						reason = "No input items (waiting for material)"
					} else if isExtractor {
						extractProgress, hasEP := extractorProgress[instanceName]
						if hasEP && extractProgress < 0.1 {
							status = "Blocked"
							reason = "Output full (waiting for space)"
							isBlocked = true
						} else {
							status = "Idle"
							reason = "Standby (not actively extracting)"
						}
					} else {
						status = "Idle"
						reason = "Standby (not actively producing)"
					}
				} else {
					// No inventory data
					if isExtractor {
						extractProgress, hasEP := extractorProgress[instanceName]
						if hasEP && extractProgress < 0.1 {
							status = "Blocked"
							reason = "Output full (waiting for space)"
							isBlocked = true
						} else {
							status = "Idle"
							reason = "Standby (not actively extracting)"
						}
					} else {
						status = "Starving"
						reason = "Low power consumption (waiting for input)"
					}
				}
			} else {
				// Active building: use current consumption
				maxMW = currentMW
				if isActive {
					actualMW = maxMW
				} else {
					actualMW = 0
				}

				if isActive {
					if hasInventoryData {
						if isBlocked {
							status = "Blocked"
							reason = "Output full (waiting for space)"
						} else if isStarving && !isExtractor {
							status = "Starving"
							reason = "No input items (waiting for material)"
						} else if isExtractor {
							extractProgress, hasEP := extractorProgress[instanceName]
							if hasEP && extractProgress < 0.1 {
								status = "Blocked"
								reason = "Output full (waiting for space)"
								isBlocked = true
							} else {
								status = "Running"
								reason = "Actively producing"
							}
						} else {
							status = "Running"
							reason = "Actively producing"
						}
					} else {
						if isExtractor {
							extractProgress, hasEP := extractorProgress[instanceName]
							if hasEP && extractProgress < 0.1 {
								status = "Blocked"
								reason = "Output full (waiting for space)"
								isBlocked = true
							} else {
								status = "Running"
								reason = "Actively producing"
							}
						} else {
							status = "Running"
							reason = "Actively producing"
						}
					}
				} else {
					status = "Paused"
					reason = "Manually paused or no recipe"
				}
			}
		}

		// Track all buildings with power info
		if maxMW > 0 || maxMW == -1 || actualMW > 0 {
			if status == "Unknown" {
				if isActive {
					status = "Running"
				} else {
					status = "Paused"
				}
			}

			if actualMW > 0 {
				data.ActiveBuildings++
				data.TotalActualMW += actualMW
				if maxMW > 0 {
					data.TotalTheoreticalMW += maxMW
				}
			} else {
				data.PausedBuildings++
				if maxMW > 0 {
					data.TotalTheoreticalMW += maxMW
				}
			}
		}

		// Get friendly building type
		buildingType := "Unknown"
		if m := buildingTypeRegex.FindStringSubmatch(typePath); m != nil {
			buildingType = m[1]
		}

		// Location: pos[0]=X(E/W), pos[1]=Y(N/S), pos[2]=Z(altitude), in cm
		var loc *ConsumerLocation
		if bf.HasTransform {
			pos := bf.Translation
			loc = &ConsumerLocation{
				X: formatLocationNum(float64(pos[0]) / 100.0),
				Y: formatLocationNum(float64(pos[1]) / 100.0),
				Z: formatLocationNum(float64(pos[2]) / 100.0),
			}
		}

		// Extract produced item from mCurrentRecipe or mExtractableResource
		producedItem := extractProducedItem(bf, resourceNodeFacts)

		// Skip non-consuming buildings (e.g. storage with mPowerInfo but no power use)
		if actualMW == 0 && maxMW == 0 && status == "Unknown" {
			continue
		}

		data.Consumers = append(data.Consumers, PowerConsumerEntry{
			InstanceName: instanceName,
			Type:         buildingType,
			TypePath:     typePath,
			Status:       status,
			Reason:       reason,
			ActualMW:     actualMW,
			MaxMW:        maxMW,
			IsBlocked:    isBlocked,
			Location:     loc,
			ProducedItem: producedItem,
		})
	}

	return data
}

// extractProducedItem gets the recipe name or resource name a building produces.
func extractProducedItem(bf *BuildingFacts, resourceNodeFacts map[string]*ResourceNodeFacts) string {
	// Check for recipe (production buildings)
	if bf.CurrentRecipe != "" {
		if m := recipeNameRegex.FindStringSubmatch(bf.CurrentRecipe); m != nil {
			return m[1]
		}
	}

	// Check for extractable resource (miners, etc.)
	if bf.ExtractableResource != "" {
		if rf, ok2 := resourceNodeFacts[bf.ExtractableResource]; ok2 && rf != nil && rf.ResourceClassOverride != "" {
			if m := descNameRegex.FindStringSubmatch(rf.ResourceClassOverride); m != nil {
				return m[1]
			}
		}
	}

	return ""
}

var recipeNameRegex = regexp.MustCompile(`Recipe_(\w+)`)
var descNameRegex = regexp.MustCompile(`Desc_(\w+)`)

// formatLocationNum formats a float as a string with locale-style grouping.
func formatLocationNum(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64)
}

func extractFuelFromInventoryRef(invPath string, inventoryStacks map[string][]InventoryStack) string {
	stacks, ok := inventoryStacks[invPath]
	if !ok || len(stacks) == 0 {
		return ""
	}
	return extractDescName(stacks[0].rawClass)
}

func extractDescName(path string) string {
	if path == "" {
		return ""
	}
	// Extract Desc_XXX from path
	lower := strings.ToLower(path)
	idx := strings.Index(lower, "desc_")
	if idx >= 0 {
		rest := path[idx+5:]
		// Take until non-alphanumeric
		var name string
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				name += string(r)
			} else {
				break
			}
		}
		return name
	}
	return ""
}
