package extraction

import (
	"sort"
	"strings"
)

// resolveStorageInventories tallies items in storage containers from resolved inventory stacks.
//
// Both the container list and each container's item list get sorted before we
// return. Everything here comes out of maps, so without that the report would
// list the same containers in a different order every run and you couldn't
// diff two reports against each other.
func resolveStorageInventories(inventoryStacks map[string][]InventoryStack, buildingFacts map[string]*BuildingFacts, inventoryContents map[string]int, containerDetails *[]StorageContainerDetail) {
	for _, instanceName := range sortedKeys(inventoryStacks) {
		stacks := inventoryStacks[instanceName]
		// Find the parent building.
		if !strings.Contains(instanceName, ".") {
			continue
		}
		parts := strings.Split(instanceName, ".")
		parentInstance := strings.Join(parts[:len(parts)-1], ".")
		parentBf, ok := buildingFacts[parentInstance]
		if !ok {
			continue
		}
		parentTypeLower := strings.ToLower(parentBf.TypePath)

		// Only count storage containers, not production buildings.
		isStorage := strings.Contains(parentTypeLower, "storagecontainer") ||
			strings.Contains(parentTypeLower, "industrialstorage") ||
			strings.Contains(parentTypeLower, "centralstorage") ||
			strings.Contains(parentTypeLower, "storageintegrated") ||
			strings.Contains(parentTypeLower, "crate") ||
			strings.Contains(parentTypeLower, "containerwall") ||
			strings.Contains(parentTypeLower, "playerinventory") ||
			strings.Contains(parentTypeLower, "storage")

		isProduction := strings.Contains(parentTypeLower, "conveyor") ||
			strings.Contains(parentTypeLower, "belt") ||
			strings.Contains(parentTypeLower, "splitter") ||
			strings.Contains(parentTypeLower, "merger") ||
			strings.Contains(parentTypeLower, "lift") ||
			strings.Contains(parentTypeLower, "pipe") ||
			strings.Contains(parentTypeLower, "pump") ||
			strings.Contains(parentTypeLower, "valve") ||
			strings.Contains(parentTypeLower, "manufacturer") ||
			strings.Contains(parentTypeLower, "constructor") ||
			strings.Contains(parentTypeLower, "smelter") ||
			strings.Contains(parentTypeLower, "refinery") ||
			strings.Contains(parentTypeLower, "generator") ||
			strings.Contains(parentTypeLower, "drone") ||
			strings.Contains(parentTypeLower, "train") ||
			strings.Contains(parentTypeLower, "vehicle")

		if !isStorage || isProduction {
			continue
		}

		// Tally into inventory contents (use raw class path as key so
		// frontend can translate via displayName() based on selected language)
		for _, st := range stacks {
			if st.rawClass == "" {
				continue
			}
			if st.numItems > 0 {
				inventoryContents[st.rawClass] += st.numItems
			}
		}

		// Collect per-container details for relevant storage types only
		isRelevantStorage := strings.Contains(parentTypeLower, "storagecontainer") ||
			strings.Contains(parentTypeLower, "industrialstorage") ||
			strings.Contains(parentTypeLower, "personalstorage") ||
			strings.Contains(parentTypeLower, "medicalstorage") ||
			strings.Contains(parentTypeLower, "hazardstorage") ||
			strings.Contains(parentTypeLower, "storageintegrated") ||
			strings.Contains(parentTypeLower, "storageblueprint")

		if isRelevantStorage {
			// Consolidate items by name (multiple stack slots may have same item)
			itemMap := map[string]int{}
			for _, st := range stacks {
				if st.rawClass == "" || st.numItems <= 0 {
					continue
				}
				itemMap[st.rawClass] += st.numItems
			}
			var items []StorageItemEntry
			for _, name := range sortedKeys(itemMap) {
				items = append(items, StorageItemEntry{
					Item:  name,
					Count: itemMap[name],
				})
			}
			// Biggest stacks first, name as the tie-break.
			sort.SliceStable(items, func(i, j int) bool {
				return items[i].Count > items[j].Count
			})
			var loc *[3]float32
			if parentBf.HasTransform {
				loc = &[3]float32{
					parentBf.Translation[0] / 100,
					parentBf.Translation[1] / 100,
					parentBf.Translation[2] / 100,
				}
			}
			*containerDetails = append(*containerDetails, StorageContainerDetail{
				Type:     parentBf.TypePath,
				Location: loc,
				Items:    items,
			})
		}
	}
}

// getItemPathFromStack extracts the item descriptor path from an InventoryStack element.
func getItemPathFromStack(props map[string]interface{}) string {
	// Item -> StructProperty(InventoryItem) -> itemClass.pathName
	if itemRaw, ok := props["Item"]; ok {
		if itemWrapper, ok := itemRaw.(map[string]interface{}); ok {
			// StructProperty is wrapped as {"type": ..., "value": ...}.
			if itemValue, ok := itemWrapper["value"]; ok {
				if itemStruct, ok := itemValue.(map[string]interface{}); ok {
					if itemClass, ok := itemStruct["itemClass"]; ok {
						if ref, ok := itemClass.(map[string]string); ok {
							return ref["pathName"]
						}
					}
				}
			}
		}
	}
	// Fallback: ItemClass -> ObjectProperty -> pathName.
	if itemClassRaw, ok := props["ItemClass"]; ok {
		if ref, ok := itemClassRaw.(map[string]string); ok {
			return ref["pathName"]
		}
		// ...or wrapped in a StructProperty.
		if wrapper, ok := itemClassRaw.(map[string]interface{}); ok {
			if val, ok := wrapper["value"]; ok {
				if ref, ok := val.(map[string]string); ok {
					return ref["pathName"]
				}
			}
		}
	}
	return ""
}

func getItemClassPathFromEntry(props map[string]interface{}) string {
	if itemClassRaw, ok := props["ItemClass"]; ok {
		if ref, ok := itemClassRaw.(map[string]string); ok {
			return ref["pathName"]
		}
		if wrapper, ok := itemClassRaw.(map[string]interface{}); ok {
			if val, ok := wrapper["value"]; ok {
				if ref, ok := val.(map[string]string); ok {
					return ref["pathName"]
				}
			}
		}
	}
	return ""
}

func getNumItemsFromStack(props map[string]interface{}) int {
	if numRaw, ok := props["NumItems"]; ok {
		switch v := numRaw.(type) {
		case int32:
			return int(v)
		case float32:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func getAmountFromEntry(props map[string]interface{}) int {
	for _, key := range []string{"Amount", "amount"} {
		if amountRaw, ok := props[key]; ok {
			switch v := amountRaw.(type) {
			case int32:
				return int(v)
			case float32:
				return int(v)
			case float64:
				return int(v)
			}
		}
	}
	return 0
}

// CleanItemName turns a descriptor path into a readable item name.
// Prefers item_display_names.json, falls back to trimming prefixes/suffixes.
func CleanItemName(itemPath string) string {
	if itemPath == "" {
		return ""
	}
	// Pull the class name off the end of the path.
	className := itemPath
	if idx := strings.LastIndex(className, "/"); idx >= 0 {
		className = className[idx+1:]
	}
	// e.g. "Desc_IronPlate.Desc_IronPlate_C" -> "Desc_IronPlate_C"
	if idx := strings.Index(className, "."); idx >= 0 {
		className = className[idx+1:]
	}

	// Try the display names table first (language-specific).
	var lookupTable ItemDisplayNamesDataFile
	if currentDisplayLang == "de" && itemDisplayNamesDeLoaded != nil {
		lookupTable = itemDisplayNamesDeLoaded
	} else if itemDisplayNamesLoaded != nil {
		lookupTable = itemDisplayNamesLoaded
	}
	if lookupTable != nil {
		if displayName, ok := lookupTable[className]; ok {
			return displayName
		}
	}

	// No mapping, so trim the usual prefixes/suffixes.
	name := strings.TrimSuffix(className, "_C")
	name = strings.TrimPrefix(name, "Desc_")
	// Handle a stray "Desc" prefix followed by a capital letter.
	if strings.HasPrefix(name, "Desc") && len(name) > 4 {
		if name[4] >= 'A' && name[4] <= 'Z' {
			name = name[4:]
		} else if name[4] >= 'a' && name[4] <= 'z' {
			name = "C" + name[4:]
		}
	}
	// Remove BP_ItemDescriptor prefix
	name = strings.TrimPrefix(name, "BP_ItemDescriptor")
	// Remove BP_EquipmentDescriptor prefix
	name = strings.TrimPrefix(name, "BP_EquipmentDescriptor")
	return name
}

// inferResourceFromOutputInventory extracts the resource class (e.g.
// "Desc_Stone_C") from the first item in the extractor's output inventory.
// Used as a last-resort fallback when the resource node has no override and
// is missing from resource_wells.json.
//
// Uses bf.OutputInventory directly — NOT BuildingInventories, which may alias
// InventoryPotential as OutputRef for buildings without mOutputInventory
// (e.g. water extractors), causing Power Shards to be read instead.
func inferResourceFromOutputInventory(bf *BuildingFacts, inventoryStacks map[string][]InventoryStack) string {
	if bf == nil || bf.OutputInventory == "" {
		return ""
	}
	stacks, ok := inventoryStacks[bf.OutputInventory]
	if !ok || len(stacks) == 0 {
		return ""
	}
	rawClass := stacks[0].rawClass
	if rawClass == "" {
		return ""
	}
	// Extract the class name from the path, e.g.
	// "/Game/.../Desc_Stone.Desc_Stone_C" -> "Desc_Stone_C"
	className := rawClass
	if idx := strings.LastIndex(className, "/"); idx >= 0 {
		className = className[idx+1:]
	}
	if idx := strings.Index(className, "."); idx >= 0 {
		className = className[idx+1:]
	}
	return className
}

// lookupResourceWell looks up a fracking satellite or resource node by its
// full instance name in resource_wells.json. Returns type, purity, and found flag.
func lookupResourceWell(refPath string) (string, string, bool) {
	if len(resourceWellsLoaded) == 0 {
		return "", "", false
	}
	if well, ok := resourceWellsLoaded[refPath]; ok {
		return well.Type, well.Purity, true
	}
	return "", "", false
}
