package extraction

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

var generatorTypeRegex = regexp.MustCompile(`Build_Generator(\w+)`)

// generatorBasePower maps generator type names to their base power output at 100% clock speed.
var generatorBasePower = map[string]float64{
	"coal":       75.0,
	"fuel":       250.0,
	"nuclear":    2500.0,
	"biomass":    30.0,
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

	// Sorted so the generator list and the running totals come out the same on
	// every run - float addition isn't associative, so even the MW sums drift
	// by a ULP if the order changes.
	for _, instanceName := range sortedKeys(buildingTypes) {
		typePath := buildingTypes[instanceName]
		typeLower := strings.ToLower(typePath)

		// Check if this is a generator: either by class name, or by having
		// mBaseProduction > 0 in the power info (catches modded generators
		// like RefinedPower WaterTurbine/WaterDam/MicroReactor whose class
		// names don't contain "generator").
		isGenerator := strings.Contains(typeLower, "generator") || strings.Contains(typeLower, "biomass")
		if !isGenerator {
			if bf, ok := buildingFacts[instanceName]; ok && bf != nil && bf.HasBaseProduction && bf.BaseProduction > 0 {
				isGenerator = true
			}
		}
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
		// Modded generators (e.g. MkPlus nuclear) may not store mBaseProduction.
		// Fall back to the known base power for this generator type.
		if maxMW == 0 {
			basePower := getGeneratorBasePower(typeLower)
			if basePower > 0 {
				maxMW = basePower
			}
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
			status = "Running"
			reason = "Fuel-Starved"
			data.ActiveGenerators++
			data.ActiveCapacityMW += actualMW
		} else {
			status = "Running"
			clockPct := 100.0
			if bf.HasCurrentPotential && bf.CurrentPotential > 0 {
				clockPct = float64(bf.CurrentPotential) * 100.0
			}
			reason = "Producing at " + fmt.Sprintf("%.0f%%", clockPct) + " clock"
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
				X: int(float64(pos[0]) / 100.0),
				Y: int(float64(pos[1]) / 100.0),
				Z: int(float64(pos[2]) / 100.0),
			}
		}

		data.Generators = append(data.Generators, GeneratorEntry{
			InstanceName:             instanceName,
			Type:                     generatorType,
			TypePath:                 typePath,
			Status:                   status,
			ActualMW:                 actualMW,
			MaxMW:                    maxMW,
			TheoreticalMaxMW:         theoreticalMaxMW,
			Reason:                   reason,
			OutputPercent:            outputPercent,
			FuelType:                 fuelType,
			PowerShardCount:          powerShardCount,
			Location:                 genLoc,
			VariablePowerCycleOffset: bf.VariablePowerCycleOffset,
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

// getConsumerBasePower looks up the base power consumption for a building type from power_data.json.
// Longer keys are checked first so "PipelinePumpMk2" matches before "PipelinePump".
func getConsumerBasePower(typeLower string) float64 {
	if powerDataLoaded == nil {
		return 0
	}
	var bestKey string
	var bestMW float64
	for key, cd := range powerDataLoaded.Consumers {
		lk := strings.ToLower(key)
		if strings.Contains(typeLower, lk) {
			if len(lk) > len(bestKey) {
				bestKey = lk
				bestMW = cd.BaseMW
			}
		}
	}
	return bestMW
}

// What a locomotive pulls when it's flat out accelerating. We only use this to
// show the top of the range in the report, never as a counted value. The 25 MW
// minimum lives in power_data.json with the rest of the building numbers.
const locomotiveMaxDrawMW = 110.0

// railBuildingKind picks out the rail buildings the normal producer logic
// can't cope with. Returns "" for everything else.
func railBuildingKind(typeLower string) string {
	switch {
	case strings.Contains(typeLower, "build_trainstation"):
		return "station"
	case strings.Contains(typeLower, "traindockingstation"):
		return "platform"
	}
	return ""
}

// railPowerStatus works out consumption and status for train stations and
// freight platforms.
//
// They need their own path for two reasons. There's no recipe and no
// mIsProducing on them, so the producer logic below sees the 0.1 MW idle draw,
// decides they're "Paused" and throws the number away. And they're fed off the
// third rail, so WireCount is no help in deciding if they're powered.
//
// The MW values are straight from the save: a platform reports 50 MW while a
// transfer is actually running and 0.1 MW the rest of the time, a station just
// sits at 0.1 MW. The 50 MW a station shows on the in-game power graph is its
// rating, not what it's really pulling.
func railPowerStatus(kind string, currentMW, ratedMW float64, bf *BuildingFacts) (actualMW, maxMW float64, status, reason string) {
	maxMW = ratedMW
	if maxMW <= 0 {
		maxMW = -1
	}

	if bf.HasCircuit && !bf.CircuitHasGenerator {
		return 0, maxMW, "Unpowered", "On a rail circuit with no power source"
	}
	if !bf.HasCircuit && bf.WireCount == 0 {
		return 0, maxMW, "Unpowered", "Not connected to a power circuit or rail network"
	}

	actualMW = currentMW
	busy := ratedMW > 0 && currentMW >= ratedMW*0.5

	if kind == "platform" {
		if busy {
			return actualMW, maxMW, "Running", "Transferring cargo"
		}
		return actualMW, maxMW, "Idle", "Powered, no cargo transfer"
	}
	if busy {
		return actualMW, maxMW, "Running", "Train docked"
	}
	return actualMW, maxMW, "Idle", "Powered, waiting for a train"
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
	// Pipe pumps consume power – check before "pipeline" catches them.
	if strings.Contains(typeLower, "pipelinepump") || strings.Contains(typeLower, "pipepump") {
		return false
	}
	// ResourceSinkShop is where you buy coupons, not a power consumer.
	if strings.Contains(typeLower, "resourcesinkshop") {
		return true
	}
	nonPower := []string{
		"storage", "container", "tank", "dimdepot", "centralstorage",
		"pipeline", "pipelinejunction", "pipeconnection", "fgpipe", "valve",
		"conveyormerger", "conveyorsplitter", "programmable", "smart",
		"passthrough", "wallhole", "floorhole",
		"powerpole", "powertower", "powerline", "powerswitch",
		"powerstorage",
		"tradingpost", "tradestation",
		"hub", "spacelevator",
		"blueprintdesigner",
		"sign", "lamp", "light", "beacon",
		"door", "wall", "foundation", "ramp", "walkway", "roof",
		"beam", "pillar", "barrier", "gate",
		"conveyorbelt", "conveyorlift",
		"pipesupport",
		"hypertube", "hyper",
		"railroad", "rail", "switch", "signal",
		"truckstation",
		// Empty platforms have power info but always report 0 MW.
		// Freight platforms and train stations are not in this list anymore –
		// they report real numbers. See railPowerStatus.
		"trainplatformempty",
		"frackingextractor", // powered via FrackingSmasher, not on its own
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
	buildingConnections map[string]BuildingConnection,
) PowerConsumptionData {
	data := PowerConsumptionData{}

	// Sorted for the same reason as the generator loop above.
	for _, instanceName := range sortedKeys(buildingFacts) {
		bf := buildingFacts[instanceName]
		if bf == nil {
			continue
		}

		typePath := bf.TypePath
		typeLower := strings.ToLower(typePath)

		// Skip generators - we only want consumers
		if strings.Contains(typeLower, "generator") {
			continue
		}

		// Must have mPowerInfo to be a power consumer
		if bf.PowerInfoRef == "" {
			continue
		}

		// Skip buildings that have mPowerInfo but never actually consume power
		if isNonPowerConsumer(typeLower) {
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

		// Check if the building actually has a recipe set. Buildings without
		// mCurrentRecipe (sinks, elevators, unconfigured machines) have no entry
		// in buildingRecipes, so the map lookup returns "".
		hasNoRecipe := buildingRecipes[instanceName] == ""

		// Check inventory to determine actual state (starving, blocked, or running)
		buildingInventory, hasInventoryData := resolvedInventories[instanceName]
		isStarving := false
		isBlocked := false

		if hasInventoryData {
			// Extractors have no inputs, so they can never be starving
			if !isExtractor {
				hasInput := len(buildingInventory.InputStacks) > 0
				totalInputCount := float64(0)
				filledInputSlots := 0
				for _, stack := range buildingInventory.InputStacks {
					totalInputCount += stack.Count
					if stack.Count > 0 {
						filledInputSlots++
					}
				}
				isStarving = !hasInput || totalInputCount == 0

				// If we know the recipe, also check that enough distinct input
				// slots are filled. A recipe with 2 ingredients needs 2 filled
				// slots - having only one (e.g. Crude Oil but no Empty Canister)
				// means the building is starving.
				recipePath := buildingRecipes[instanceName]
				ingredients := GetRecipeIngredients(recipePath)
				if ingredients != nil && filledInputSlots < len(ingredients) {
					isStarving = true
				}
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

		// Check if building has any output connections (belts or pipes)
		conn, hasConn := buildingConnections[instanceName]
		noOutputConnection := hasConn && len(conn.Outputs) == 0
		blockedReason := "Output full (waiting for space)"
		if noOutputConnection {
			blockedReason = "No output belt/pipe connected"
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

		// Check power connection - buildings with no wires are not connected to the grid
		hasPowerConnection := bf.WireCount > 0

		if railKind := railBuildingKind(typeLower); railKind != "" {
			actualMW, maxMW, status, reason = railPowerStatus(railKind, currentMW, expectedMW, bf)
		} else if !hasPowerConnection && currentMW == 0 {
			// Not connected to power network
			status = "Standby"
			reason = "Not connected to power network"
			if expectedMW > 0 {
				maxMW = expectedMW
			} else {
				maxMW = -1 // sentinel: unknown building, but still show it
			}
		} else if currentMW > 0 {
			if !isActive && currentMW < 1.0 {
				// Paused building with standby power. Report the real draw
				// (matches the "else" branch below, which does the same for
				// Blocked/Starving buildings above this threshold) so the
				// "Actual MW" column and TotalActualMW aren't silently
				// undercounted just because the value happens to be small.
				if expectedMW > 0 {
					maxMW = expectedMW
				} else {
					maxMW = -1
				}
				actualMW = currentMW
				if hasInventoryData && isBlocked {
					status = "Blocked"
					reason = blockedReason
				} else if hasInventoryData && isStarving && !isExtractor && !hasNoRecipe {
					status = "Starving"
					reason = "No input items (waiting for material)"
				} else {
					status = "Paused"
					if hasNoRecipe {
						reason = "Manually paused or no recipe"
					} else {
						reason = "Manually paused"
					}
				}
			} else if isActive && isRunningAtLowClock {
				// Building is running at low clock speed - consumption matches expected formula.
				// Some buildings (Converter, QuantumEncoder, HadronCollider) have recipe-dependent
				// power that can exceed the base power in power_data.json. Use the higher of
				// expected vs actual so theoretical max is never below current consumption.
				maxMW = math.Max(expectedMW, currentMW)
				actualMW = currentMW

				if hasInventoryData {
					if isBlocked {
						status = "Blocked"
						reason = blockedReason
					} else if isStarving && !isExtractor {
						if hasNoRecipe {
							status = "Running"
							reason = "Active"
						} else {
							status = "Starving"
							reason = "No input items (waiting for material)"
						}
					} else {
						status = "Running"
						reason = "Producing at " + fmt.Sprintf("%.0f%%", clockSpeedPct) + " clock"
					}
				} else {
					if isExtractor {
						extractProgress, hasEP := extractorProgress[instanceName]
						if hasEP && extractProgress < 0.1 {
							status = "Blocked"
							reason = blockedReason
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
				// Active but consuming standby power (0.1 MW) - not actively
				// producing. Report the real draw (see comment on the
				// Paused branch above).
				if expectedMW > 0 {
					maxMW = expectedMW
				} else {
					maxMW = -1
				}
				actualMW = currentMW

				if hasInventoryData {
					if isBlocked {
						status = "Blocked"
						reason = blockedReason
					} else if isStarving && !isExtractor {
						if hasNoRecipe {
							status = "Running"
							reason = "Active"
						} else {
							status = "Starving"
							reason = "No input items (waiting for material)"
						}
					} else if isExtractor {
						extractProgress, hasEP := extractorProgress[instanceName]
						if hasEP && extractProgress < 0.1 {
							status = "Blocked"
							reason = blockedReason
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
							reason = blockedReason
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
							reason = blockedReason
						} else if isStarving && !isExtractor {
							if hasNoRecipe {
								status = "Running"
								reason = "Active"
							} else {
								status = "Starving"
								reason = "No input items (waiting for material)"
							}
						} else if isExtractor {
							extractProgress, hasEP := extractorProgress[instanceName]
							if hasEP && extractProgress < 0.1 {
								status = "Blocked"
								reason = blockedReason
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
								reason = blockedReason
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
					if hasInventoryData && isBlocked {
						status = "Blocked"
						reason = blockedReason
					} else if hasInventoryData && isStarving && !isExtractor && !hasNoRecipe {
						status = "Starving"
						reason = "No input items (waiting for material)"
					} else {
						status = "Paused"
						if hasNoRecipe {
							reason = "Manually paused or no recipe"
						} else {
							reason = "Manually paused"
						}
					}
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
				X: int(float64(pos[0]) / 100.0),
				Y: int(float64(pos[1]) / 100.0),
				Z: int(float64(pos[2]) / 100.0),
			}
		}

		// Extract produced item from mCurrentRecipe or mExtractableResource
		producedItem := extractProducedItem(bf, resourceNodeFacts)

		// Skip non-consuming buildings (e.g. storage with mPowerInfo but no power use)
		if actualMW == 0 && maxMW == 0 && status == "Unknown" {
			continue
		}

		// Compute in-game productivity percentage from save file data
		var productivity float64
		if bf.HasLastProductivity && bf.LastProductivityDuration > 0 {
			productivity = float64(bf.LastProductivityProduceDuration) / float64(bf.LastProductivityDuration) * 100
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
			Productivity: productivity,
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
