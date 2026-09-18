package extraction

import (
	"strings"

	"satisfacts/parser"
)

func (e *ExtractedData) ProcessCollectables1(refs []parser.ObjectRef) {
	if collectiblePositionsLoaded == nil || len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		pathName := ref.PathName
		if pathName == "" {
			continue
		}
		if _, ok := collectiblePositionsLoaded.PowerSlugs.Blue[pathName]; ok {
			e.CollectedCollectibles.PowerSlugsBlue[pathName] = true
		} else if _, ok := collectiblePositionsLoaded.PowerSlugs.Yellow[pathName]; ok {
			e.CollectedCollectibles.PowerSlugsYellow[pathName] = true
		} else if _, ok := collectiblePositionsLoaded.PowerSlugs.Purple[pathName]; ok {
			e.CollectedCollectibles.PowerSlugsPurple[pathName] = true
		} else if _, ok := collectiblePositionsLoaded.Somersloops[pathName]; ok {
			e.CollectedCollectibles.Somersloops[pathName] = true
		} else if _, ok := collectiblePositionsLoaded.MercerSpheres[pathName]; ok {
			e.CollectedCollectibles.MercerSpheres[pathName] = true
		} else if _, ok := collectiblePositionsLoaded.CrashSites[pathName]; ok {
			e.CollectedCollectibles.CrashSites[pathName] = true
		}
	}
}

func (e *ExtractedData) FinalizePowerGrid() {
	total := 0
	for _, c := range e.PowerGrid.Circuits {
		total += c.ComponentCount
	}
	e.PowerGrid.ComponentCount = total
}

func extractPickupItem(obj *parser.SaveObject) (PickupItem, bool) {
	prop, ok := obj.Properties["mPickupItems"]
	if !ok {
		return PickupItem{}, false
	}

	// Unwrap StructProperty -> InventoryStack -> value.
	outerMap, ok := prop.Value.(map[string]interface{})
	if !ok {
		return PickupItem{}, false
	}
	innerValue, ok := outerMap["value"].(map[string]interface{})
	if !ok {
		return PickupItem{}, false
	}

	// Extract item class path using existing helper
	itemClass := getItemPathFromStack(innerValue)
	if itemClass == "" {
		return PickupItem{}, false
	}

	// Extract NumItems
	numItems := getNumItemsFromStack(innerValue)
	if numItems <= 0 {
		return PickupItem{}, false
	}

	// Extract short item name from path
	itemName := shortenItemName(itemClass)

	// Check if collected (mHasBeenLooted or mPickupState)
	collected := false
	if looted, ok := GetPropBool(obj, "mHasBeenLooted"); ok && looted {
		collected = true
	}

	return PickupItem{
		ItemClass: itemClass,
		ItemName:  itemName,
		NumItems:  numItems,
		Position:  obj.Header.Transform.Translation,
		Collected: collected,
	}, true
}

// shortenItemName extracts a readable name from an item class path.
func shortenItemName(itemClass string) string {
	parts := strings.Split(itemClass, "/")
	last := parts[len(parts)-1]
	// Last segment looks like "Desc_Rotor.Desc_Rotor_C"; keep the part after the dot.
	if idx := strings.Index(last, "."); idx > 0 {
		last = last[idx+1:]
	}

	// Look up in item_display_names.json first
	if itemDisplayNamesLoaded != nil {
		if displayName, ok := itemDisplayNamesLoaded[last]; ok {
			return displayName
		}
	}

	// No mapping, so fall back to trimming the usual prefixes/suffixes.
	last = strings.TrimPrefix(last, "Desc_")
	last = strings.TrimPrefix(last, "BP_EquipmentDescriptor")
	last = strings.TrimSuffix(last, "_C")
	return last
}

// extractConnRefs pulls pathName strings from an ArrayProperty that holds
// ObjectProperty items (used by signal mGuardedConnections / mObservedConnections).
func extractConnRefs(prop parser.Property) []string {
	var items []interface{}
	if arr, ok := prop.Value.([]interface{}); ok {
		items = arr
	} else if m, ok := prop.Value.(map[string]interface{}); ok {
		if arr, ok := m["items"].([]interface{}); ok {
			items = arr
		}
	}
	var refs []string
	for _, item := range items {
		if ref, ok := item.(map[string]string); ok {
			if pathName := ref["pathName"]; pathName != "" {
				refs = append(refs, pathName)
			}
		} else if ref, ok := item.(map[string]interface{}); ok {
			if pathName, _ := ref["pathName"].(string); pathName != "" {
				refs = append(refs, pathName)
			}
		}
	}
	return refs
}
