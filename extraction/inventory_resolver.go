package extraction

import (
	"strings"
)

// BuildResolvedInventories turns each building's input/output inventory refs
// into the actual stacks they point at.
func BuildResolvedInventories(
	buildingInventories map[string]BuildingInventoryRefs,
	inventoryObjects map[string][]InventoryStack,
) map[string]ResolvedInventory {
	result := make(map[string]ResolvedInventory)

	for buildingInstance, refs := range buildingInventories {
		var inputStacks, outputStacks []InventoryStack

		if refs.InputRef != "" {
			if stacks, ok := inventoryObjects[refs.InputRef]; ok {
				inputStacks = stacks
			}
		}
		if refs.OutputRef != "" {
			if stacks, ok := inventoryObjects[refs.OutputRef]; ok {
				outputStacks = stacks
			}
		}

		result[buildingInstance] = ResolvedInventory{
			InputStacks:  inputStacks,
			OutputStacks: outputStacks,
		}
	}

	return result
}

// BuildExtractorProgress pulls mCurrentExtractProgress off every BuildingFacts entry.
func BuildExtractorProgress(facts map[string]*BuildingFacts) map[string]float32 {
	result := make(map[string]float32)
	for instanceName, bf := range facts {
		if bf.HasExtractProgress {
			result[instanceName] = bf.CurrentExtractProgress
		}
	}
	return result
}

// parseInventoryStacks turns raw mInventoryStacks elements into InventoryStacks.
func parseInventoryStacks(stacks []interface{}) []InventoryStack {
	var result []InventoryStack
	for _, item := range stacks {
		structProps, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		itemPath := getItemPathFromStack(structProps)
		if itemPath == "" {
			continue
		}
		itemName := CleanItemName(itemPath)
		if itemName == "" {
			continue
		}
		count := getNumItemsFromStack(structProps)

		// Fluids are stored in liters; show them in m³.
		isFluid := isFluidItem(itemName)
		var countFloat float64
		if isFluid {
			countFloat = float64(count) / 1000.0
		} else {
			countFloat = float64(count)
		}

		result = append(result, InventoryStack{
			Item:      itemName,
			Count:     countFloat,
			StackSize: GetStackSize(itemName),
			rawClass:  itemPath,
			numItems:  count,
		})
	}
	return result
}

// isFluidItem reports whether an item name is a known fluid or gas.
func isFluidItem(itemName string) bool {
	itemLower := strings.ToLower(itemName)
	for _, f := range fluidNames {
		if strings.Contains(itemLower, strings.ToLower(f)) {
			return true
		}
	}
	return false
}

// computeInventoryFill returns the fill percentage (0-100) of an inventory.
// stacks are the non-empty slots, totalSlots includes empty slots.
// Each slot's fill ratio is numItems/StackSize; empty slots contribute 0.
func computeInventoryFill(stacks []InventoryStack, totalSlots int) int {
	if totalSlots <= 0 {
		return 0
	}
	if len(stacks) == 0 {
		return 0
	}
	totalFill := 0.0
	for _, s := range stacks {
		if s.StackSize > 0 {
			totalFill += float64(s.numItems) / float64(s.StackSize)
		}
	}
	pct := totalFill / float64(totalSlots) * 100
	if pct > 100 {
		pct = 100
	}
	return int(pct)
}
