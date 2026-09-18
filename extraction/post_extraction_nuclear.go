package extraction

import (
	"strings"

	"satisfacts/parser"
)

func extractNuclearPlant(bf *BuildingFacts, instanceName string, waste *NuclearWasteData, inventoryPotentialMap map[string]float32) {
	wasteLeft := bf.WasteLeftFromCurrentFuel
	fuel := bf.CurrentFuelAmount
	supplemental := bf.CurrentSupplementalAmount
	clockSpeed := bf.CurrentPotential
	fuelClassRef := bf.CurrentFuelClass

	fuelType := "Unknown"
	if fuelClassRef != "" {
		if strings.Contains(fuelClassRef, "NuclearFuelRod") {
			fuelType = "Uranium"
		} else if strings.Contains(fuelClassRef, "PlutoniumFuelRod") {
			fuelType = "Plutonium"
		} else if strings.Contains(strings.ToLower(fuelClassRef), "ficsonium") {
			fuelType = "Ficsonium"
		}
	}

	// Modded nuclear plants (e.g. MkPlus) may not store mCurrentFuelClass.
	// Still add them with "Unknown" fuel type so they appear in the nuclear tab.

	// Ficsonium plants don't set mCurrentPotential; use PendingPotential or InventoryPotential instead.
	if clockSpeed == 0 {
		if bf.HasPendingPotential && bf.PendingPotential > 0 {
			clockSpeed = bf.PendingPotential
		} else if bf.InventoryPotential != "" {
			if v, exists := inventoryPotentialMap[bf.InventoryPotential]; exists && v > 0 {
				clockSpeed = v
			}
		}
	}
	// Still nothing? Default to 100%.
	if clockSpeed == 0 {
		clockSpeed = 1.0
	}

	plantWaste := int(wasteLeft)
	if fuelType == "Ficsonium" {
		plantWaste = 0
	}

	var plantLoc *[3]float32
	if bf.HasTransform {
		plantLoc = &bf.Translation
	}

	waste.Plants = append(waste.Plants, NuclearPlant{
		Instance:     instanceName,
		Waste:        plantWaste,
		Fuel:         int(fuel),
		Supplemental: int(supplemental),
		FuelType:     fuelType,
		ClockSpeed:   clockSpeed * 100,
		Location:     plantLoc,
	})
	waste.TotalWaste += plantWaste
}

func accumulateNuclearWasteFromStacks(stacks []InventoryStack, waste *NuclearWasteData) {
	for _, st := range stacks {
		if st.rawClass == "" {
			continue
		}
		if strings.Contains(strings.ToLower(st.rawClass), "nuclearwaste") {
			if st.numItems > 0 {
				waste.TotalStoredWaste += st.numItems
				waste.WasteContainers++
			}
		}
	}
}

func extractCentralStorageItems(obj *parser.SaveObject, inventoryContents map[string]int, waste *NuclearWasteData) {
	storedItems, ok := GetPropArray(obj, "mStoredItems")
	if !ok {
		return
	}
	for _, item := range storedItems {
		structProps, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for nuclear waste
		itemPath := getItemClassPathFromEntry(structProps)
		if strings.Contains(strings.ToLower(itemPath), "nuclearwaste") {
			amount := getAmountFromEntry(structProps)
			if amount > 0 {
				waste.TotalStoredWaste += amount
				waste.WasteContainers++
			}
		}

		// Add to inventory contents (use raw class path as key so frontend
		// can translate via displayName() based on selected language)
		if itemPath == "" {
			continue
		}
		amount := getAmountFromEntry(structProps)
		if amount > 0 {
			inventoryContents[itemPath] += amount
		}
	}
}
