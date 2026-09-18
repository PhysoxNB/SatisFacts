package extraction

import (
	"strings"

	"satisfacts/parser"
)

func (e *ExtractedData) processLightweightBuildables(sp map[string]interface{}) {
	typeCounts, ok := sp["typeCounts"].(map[string]int)
	if !ok {
		return
	}
	typeTransforms, _ := sp["typeTransforms"].(map[string][]float64)
	typeBeamLengths, _ := sp["typeBeamLengths"].(map[string][]float64)

	for typePath, count := range typeCounts {
		if isInternalSystem(typePath) {
			continue
		}
		category := classifyStructure(typePath)
		if category == "" {
			continue
		}

		// Store lightweight counts separately - applied in post-streaming fixup
		if e.Structures.lightweightCounts[category] == nil {
			e.Structures.lightweightCounts[category] = make(map[string]int)
		}
		e.Structures.lightweightCounts[category][typePath] = count

		// Add map entries with transforms
		transforms, hasTransforms := typeTransforms[typePath]
		if !hasTransforms || len(transforms) == 0 {
			continue
		}
		beamLengths := typeBeamLengths[typePath]

		// Determine map category: foundations vs other structures
		mapCat := MapCatStructure
		if category == "foundations" {
			mapCat = MapCatFoundation
		}

		// Each transform is 7 float64: qx,qy,qz,qw, tx,ty,tz
		nInstances := len(transforms) / 7
		for i := 0; i < nInstances; i++ {
			base := i * 7
			qx, qy, qz, qw := float32(transforms[base]), float32(transforms[base+1]), float32(transforms[base+2]), float32(transforms[base+3])
			tx, ty, tz := float32(transforms[base+4]), float32(transforms[base+5]), float32(transforms[base+6])

			entry := MapEntry{
				X:        tx,
				Y:        ty,
				Z:        tz,
				Cat:      mapCat,
				TypePath: typePath,
			}
			if qw != 1 || qx != 0 || qy != 0 || qz != 0 {
				entry.Qx = qx
				entry.Qy = qy
				entry.Qz = qz
				entry.Qw = qw
				entry.HasRot = true
			}
			if i < len(beamLengths) && beamLengths[i] > 0 {
				entry.BeamLength = float32(beamLengths[i])
			}
			e.MapEntries = append(e.MapEntries, entry)
		}
	}
}

func (e *ExtractedData) ApplyLightweightFixup() {
	if e.Structures == nil {
		return
	}
	// For types in the lightweight subsystem, use lightweight count + sublevel individual count
	for category, typeCounts := range e.Structures.lightweightCounts {
		if e.Structures.Details[category] == nil {
			e.Structures.Details[category] = make(map[string]int)
		}
		for typePath, lwCount := range typeCounts {
			individualCount := e.Structures.Details[category][typePath]
			sublevelCount := 0
			if e.Structures.sublevelCounts[category] != nil {
				sublevelCount = e.Structures.sublevelCounts[category][typePath]
			}
			e.Structures.Details[category][typePath] = lwCount + individualCount + sublevelCount
		}
	}

	// Recalculate totals from details
	for cat := range e.Structures.Totals {
		e.Structures.Totals[cat] = 0
	}
	for category, types := range e.Structures.Details {
		for typePath, count := range types {
			typeLower := strings.ToLower(typePath)
			if category == "attachments" && IsBeltPipeSupportItem(typeLower) {
				continue
			}
			e.Structures.Totals[category] += count
		}
	}
}

func (e *ExtractedData) processStructure(obj *parser.SaveObject, typePath, typeLower, instanceName string) {
	category := classifyStructure(typePath)
	if category == "" {
		return
	}

	if isInternalSystem(typePath) {
		return
	}

	// Sublevel objects aren't in the lightweight subsystem, so track them separately.
	levelName := ""
	if obj.Header != nil {
		levelName = obj.Header.Reference.LevelName
	}
	isPersistent := strings.EqualFold(levelName, "Persistent_Level")
	if isPersistent {
		e.Structures.persistentLevelTypes[typePath] = true
	}

	if category == "hypertubes" {
		length := ExtractLength(obj)
		if length > 0 {
			e.Hypertubes = append(e.Hypertubes, length)
		}
		e.addStructure(category, typePath, instanceName, obj)
		if !isPersistent {
			if e.Structures.sublevelCounts[category] == nil {
				e.Structures.sublevelCounts[category] = make(map[string]int)
			}
			e.Structures.sublevelCounts[category][typePath]++
		}
	} else {
		// For decorations, use the descriptor path so each statue type shows individually
		if category == "statues" {
			if descPath, ok := GetPropObjectRefPathName(obj, "mDecorationDescriptor"); ok && descPath != "" {
				typePath = descPath
			}
		}
		e.addStructure(category, typePath, instanceName, obj)
		if !isPersistent {
			if e.Structures.sublevelCounts[category] == nil {
				e.Structures.sublevelCounts[category] = make(map[string]int)
			}
			e.Structures.sublevelCounts[category][typePath]++
		}
	}
	// Only skip totals for attachments that are belt/pipe support items
	if category == "attachments" && classInfoFor(typePath).isBeltPipeSupport {
		// Skip - shown in Production dropdowns
	} else {
		e.Structures.Totals[category]++
	}
}

// classifyStructure returns the structure category for a type path, or "".
func classifyStructure(typePath string) string {
	return classInfoFor(typePath).classifiedStructure
}

// computeClassifyStructure is the uncached implementation, reusing the flags
// already computed on ci. Cached via classInfo.
func computeClassifyStructure(ci *classInfo) string {
	typeLower := ci.lower
	if strings.Contains(typeLower, "_subsystem") {
		return ""
	}
	if strings.Contains(typeLower, "foundationpassthrough") || strings.Contains(typeLower, "bp_gaspillar") {
		return ""
	}
	if strings.Contains(typeLower, "blueprintdesigner") || strings.Contains(typeLower, "blueprint_designer") {
		return ""
	}
	if strings.Contains(typeLower, "fglightweightbuildable") {
		return ""
	}

	// Priority cases first; these don't go through IsStructure.
	if strings.Contains(typeLower, "beam") || strings.Contains(typeLower, "truss") {
		return "beams"
	}
	// Lights, including control panels, but not anything power-line/pole/switch.
	if (strings.Contains(typeLower, "light") || strings.Contains(typeLower, "lamp") ||
		strings.Contains(typeLower, "floodlight") || strings.Contains(typeLower, "streetlight") ||
		strings.Contains(typeLower, "controlpanel")) &&
		!strings.Contains(typeLower, "powerline") && !strings.Contains(typeLower, "powerpole") &&
		!strings.Contains(typeLower, "powerswitch") && !strings.Contains(typeLower, "prioritypowerswitch") {
		return "lights"
	}
	// Power switches
	if strings.Contains(typeLower, "powerswitch") || strings.Contains(typeLower, "prioritypowerswitch") {
		return "powerSwitches"
	}
	// Signs (not signal, not pole, not blueprintdesigner)
	if (strings.Contains(typeLower, "sign") || strings.Contains(typeLower, "billboard") ||
		strings.Contains(typeLower, "widgetsign")) && !strings.Contains(typeLower, "signal") &&
		!strings.Contains(typeLower, "pole") && !strings.Contains(typeLower, "blueprintdesigner") {
		return "signs"
	}

	// Everything else goes through IsStructure + StructureCategory.
	if !ci.isStructure {
		return ""
	}
	if ci.structureCategory == "other" {
		return ""
	}
	return ci.structureCategory
}

func (e *ExtractedData) addStructure(category, typePath, instanceName string, obj *parser.SaveObject) {
	if e.Structures.Details[category] == nil {
		e.Structures.Details[category] = make(map[string]int)
	}
	e.Structures.Details[category][typePath]++
}

func isInternalSystem(typePath string) bool {
	return classInfoFor(typePath).isInternalSystem
}

// computeIsInternalSystem is the uncached implementation. Cached via classInfo.
func computeIsInternalSystem(typeLower string) bool {
	if strings.Contains(typeLower, "decoration") {
		return false
	}
	return strings.Contains(typeLower, "fgtrain") || strings.Contains(typeLower, "fgcentral") ||
		strings.Contains(typeLower, "fgdocking") || strings.Contains(typeLower, "fgdrone") ||
		strings.Contains(typeLower, "fglightweightbuildable") ||
		strings.Contains(typeLower, "/bp_") || strings.Contains(typeLower, "bp_")
}

// computeClockSpeedExcluded is the uncached implementation. Cached via classInfo.
func computeClockSpeedExcluded(typeLower string) bool {
	excluded := []string{"storage", "conveyor", "belt", "pipe", "splitter", "merger", "lift",
		"wall", "door", "foundation", "ramp", "walkway", "dock", "table", "shelf",
		"lamp", "sign", "beacon", "train", "vehicle", "drone", "hub", "elevator",
		"powerpole", "powertower", "powerswitch", "powerline",
		"workbench", "workshop", "mam",
		"beam", "lookout", "streetlight", "snow", "candycane", "snowman", "xmasstree",
		"tank", "powerstorage", "blueprintdesigner", "depot", "centralstorage",
		"portal", "sink", "switch", "pole", "equip_", "truckstation",
		// Generators handled separately during clock speed extraction.
		"generator", "biomass", "nuclear", "geothermal"}
	for _, e := range excluded {
		if strings.Contains(typeLower, e) {
			return true
		}
	}
	return false
}
