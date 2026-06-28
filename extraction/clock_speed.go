package extraction

import (
	"strings"
)

// How many somersloop slots each building type has.
var somersloopMaxSlots = map[string]int{
	"smelter":             1,
	"constructor":         1,
	"assembler":           2,
	"foundry":             2,
	"refinery":            2,
	"converter":           2,
	"manufacturer":        4,
	"blender":             4,
	"hadron":              4,
	"quantum":             4,
}

// ClockSpeedResult is the clock speed and production boost for one building.
type ClockSpeedResult struct {
	ClockSpeed      float32 // percentage (100 = 100%)
	ProductionBoost float32 // somersloop multiplier or power shard multiplier
	IsExtractor     bool
}

// ExtractClockSpeeds returns clock speed (%), production boost, and typePath per instance.
func ExtractClockSpeeds(facts map[string]*BuildingFacts) (clockSpeeds map[string]float32, productionBoosts map[string]float32, buildingTypes map[string]string) {
	clockSpeeds = make(map[string]float32)
	productionBoosts = make(map[string]float32)
	buildingTypes = make(map[string]string)

	for instanceName, bf := range facts {
		typePath := bf.TypePath
		typeLower := strings.ToLower(typePath)

		isExcluded := isExcludedBuilding(typeLower)

		// Production buildings: clock speed from mCurrentPotential.
		if IsBuilding(typePath) && !isExcluded {
			if bf.HasCurrentPotential {
				clockSpeeds[instanceName] = bf.CurrentPotential * 100
				buildingTypes[instanceName] = typePath
			} else {
				clockSpeeds[instanceName] = 100
				buildingTypes[instanceName] = typePath
			}

			// Production boost from somersloops.
			if bf.HasCurrentProductionBoost && bf.CurrentProductionBoost > 1.0 {
				isExtractor := strings.Contains(typeLower, "miner") || strings.Contains(typeLower, "waterpump") ||
					strings.Contains(typeLower, "waterextractor") || strings.Contains(typeLower, "oilextractor") ||
					strings.Contains(typeLower, "oilpump") || strings.Contains(typeLower, "fracking") ||
					strings.Contains(typeLower, "resourcewell")

				if isExtractor {
					productionBoosts[instanceName] = bf.CurrentProductionBoost
				} else {
					maxSlots := 1
					for key, slots := range somersloopMaxSlots {
						if strings.Contains(typeLower, key) {
							maxSlots = slots
							break
						}
					}
					somersloopCount := int(bf.CurrentProductionBoost-float32(1)) * maxSlots
					if somersloopCount > 0 {
						productionBoosts[instanceName] = float32(somersloopCount)
					}
				}
			}
		}

		// Generators report via either mCurrentProductionBoost or mCurrentPotential.
		isGenerator := strings.Contains(typeLower, "generator") || strings.Contains(typeLower, "biomass") ||
			strings.Contains(typeLower, "fuel") || strings.Contains(typeLower, "nuclear") ||
			strings.Contains(typeLower, "geothermal")

		if isGenerator {
			if bf.HasCurrentProductionBoost {
				clockSpeeds[instanceName] = bf.CurrentProductionBoost
				buildingTypes[instanceName] = typePath
			}
			if bf.HasCurrentPotential {
				clockSpeeds[instanceName] = bf.CurrentPotential * 100
				buildingTypes[instanceName] = typePath
			} else if !bf.HasCurrentProductionBoost {
				clockSpeeds[instanceName] = 100
				buildingTypes[instanceName] = typePath
			}
		}
	}

	return clockSpeeds, productionBoosts, buildingTypes
}

func isExcludedBuilding(typeLower string) bool {
	excluded := []string{"storage", "conveyor", "belt", "pipe", "splitter", "merger", "lift",
		"wall", "door", "foundation", "ramp", "walkway", "dock", "table", "shelf",
		"lamp", "sign", "beacon", "train", "vehicle", "drone", "hub", "elevator",
		"tank", "powerstorage", "blueprintdesigner", "depot", "centralstorage",
		"portal", "sink", "switch", "pole", "powerline", "equip_", "truckstation"}
	for _, e := range excluded {
		if strings.Contains(typeLower, e) {
			return true
		}
	}
	return false
}

