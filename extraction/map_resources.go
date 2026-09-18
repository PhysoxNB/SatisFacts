package extraction

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// resourceNodeEntry is one entry from resource_node_positions.json.
// Format: instanceName -> [resourceType, purity, [x,y,z], frackingCoreRef]
type resourceNodeEntry struct {
	InstanceName string
	ResourceType string
	Purity       string
	Position     [3]float64
	FrackingCore string
}

// loadResourceNodeEntries loads static resource node entries from embedded JSON.
func loadResourceNodeEntries() []resourceNodeEntry {
	data, err := dataFilesFS.ReadFile("report/data/resource_node_positions.json")
	if err != nil {
		return nil
	}
	var raw map[string]resourceNodeEntryJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var result []resourceNodeEntry
	for k, v := range raw {
		result = append(result, resourceNodeEntry{
			InstanceName: k,
			ResourceType: v.ResourceType,
			Purity:       v.Purity,
			Position:     v.Position,
			FrackingCore: v.FrackingCore,
		})
	}
	return result
}

// resourceNodeEntryJSON is decoded from a positional array, not an object:
//
//	instanceName -> [resourceType, purity, [x,y,z], frackingCoreRef]
//
// so the fields are filled by UnmarshalJSON below and carry no struct tags.
type resourceNodeEntryJSON struct {
	ResourceType string
	Purity       string
	Position     [3]float64
	FrackingCore string
}

// Need custom unmarshaling since the format is an array, not an object.
func (r *resourceNodeEntryJSON) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	if len(arr) < 3 {
		return nil
	}
	var rt string
	if err := json.Unmarshal(arr[0], &rt); err == nil {
		r.ResourceType = rt
	}
	var pur string
	if err := json.Unmarshal(arr[1], &pur); err == nil {
		r.Purity = pur
	}
	var pos [3]float64
	if err := json.Unmarshal(arr[2], &pos); err == nil {
		r.Position = pos
	}
	if len(arr) >= 4 {
		var fc string
		if err := json.Unmarshal(arr[3], &fc); err == nil {
			r.FrackingCore = fc
		}
	}
	return nil
}

// mapDisplayNameForType returns a display name for a type path.
func mapDisplayNameForType(typePath string) string {
	shortName := typePath
	if idx := strings.LastIndex(shortName, "/"); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	// Strip "Build_X." -> "Build_X_C"
	if idx := strings.LastIndex(shortName, "."); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	// Remove _C suffix
	shortName = strings.TrimSuffix(shortName, "_C")
	// Remove Build_ prefix
	shortName = strings.TrimPrefix(shortName, "Build_")
	// Try building_dimensions.json display name
	if entry := buildingDimsLoaded[typePath]; entry != nil && entry.DisplayName != "" {
		return entry.DisplayName
	}
	if entry := buildingDimsLoaded[shortName]; entry != nil && entry.DisplayName != "" {
		return entry.DisplayName
	}
	// Fallback: clean up the name
	name := FormatDisplayName(typePath)
	name = strings.ReplaceAll(name, "Attachment ", "")
	return name
}

// LoadMapImageBase64 returns the embedded Map.webp as a base64 data URI.
// This is injected directly into the HTML template, not through the JSON pipeline.
func LoadMapImageBase64(mapWebP []byte) string {
	return "data:image/webp;base64," + base64.StdEncoding.EncodeToString(mapWebP)
}
