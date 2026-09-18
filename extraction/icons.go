package extraction

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"satisfacts/assets"
)

//go:embed report/data/item_display_names.json
var displayNamesFS []byte

//go:embed report/data/item_display_names_de.json
var displayNamesDeFS []byte

//go:embed report/data/ui_strings.json
var uiStringsFS []byte

//go:embed report/data/ui_strings_de.json
var uiStringsDeFS []byte

// iconData maps class name -> base64 data URI ("data:image/webp;base64,...").
// Populated at init time from embedded WebP files.
var iconData map[string]string

// iconReverseLookup maps bare resource names (e.g. "OreIron") to class names
// (e.g. "Desc_OreIron_C") for matching resource references in report data.
var iconReverseLookup map[string]string

// iconDisplayNameLookup maps display names (e.g. "Iron Ore") to class names
// (e.g. "Desc_OreIron_C") for matching human-readable names in report data
// (storage items, collectibles, production by resource).
var iconDisplayNameLookup map[string]string

// iconDisplayNameLookupDe maps German display names to class names, so that
// SelectIconsFromJSON can find items when the report is generated in German.
var iconDisplayNameLookupDe map[string]string

// iconAllLookups combines reverse, EN display, and DE display lookups into a
// single map for fast membership checking during SelectIconsFromJSON.
var iconAllLookups map[string]string

func init() {
	entries, err := assets.IconFS.ReadDir("icons")
	if err != nil {
		iconData = make(map[string]string)
		iconReverseLookup = make(map[string]string)
		iconDisplayNameLookup = make(map[string]string)
		iconDisplayNameLookupDe = make(map[string]string)
		return
	}

	iconData = make(map[string]string, len(entries))
	iconReverseLookup = make(map[string]string, len(entries))
	iconDisplayNameLookup = make(map[string]string)
	iconDisplayNameLookupDe = make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".webp") {
			continue
		}
		className := strings.TrimSuffix(entry.Name(), ".webp")
		raw, err := assets.IconFS.ReadFile("icons/" + entry.Name())
		if err != nil {
			continue
		}
		b64 := base64.StdEncoding.EncodeToString(raw)
		iconData[className] = "data:image/webp;base64," + b64

		// Build reverse lookup: strip Desc_ prefix and _C suffix
		bare := className
		bare = strings.TrimPrefix(bare, "Desc_")
		bare = strings.TrimSuffix(bare, "_C")
		if bare != className {
			iconReverseLookup[bare] = className
		}
	}

	// Build display name lookup from item_display_names.json (English)
	var displayNames map[string]string
	if err := json.Unmarshal(displayNamesFS, &displayNames); err == nil {
		for cls, dn := range displayNames {
			if _, ok := iconData[cls]; ok {
				iconDisplayNameLookup[dn] = cls
			}
		}
	}

	// Build display name lookup from item_display_names_de.json (German)
	var displayNamesDe map[string]string
	if err := json.Unmarshal(displayNamesDeFS, &displayNamesDe); err == nil {
		for cls, dn := range displayNamesDe {
			if _, ok := iconData[cls]; ok {
				iconDisplayNameLookupDe[dn] = cls
			}
		}
	}

	// Extra mappings for collectibles and items not in item_display_names.json
	extraDisplayNames := map[string]string{
		"Desc_HardDrive_C":            "Hard Drive",
		"Desc_Crystal_C":              "Blue",
		"Desc_Crystal_mk2_C":          "Yellow",
		"Desc_Crystal_mk3_C":          "Purple",
		"Desc_WAT1_C":                 "Somersloop",
		"Desc_WAT2_C":                 "Mercer Sphere",
		"Build_PipelinePump_C":        "Pipeline Pump Mk1",
		"Build_PipelinePumpMk2_C":     "Pipeline Pump Mk2",
		"Build_StorageContainerMk1_C": "Storage Container Mk.1",
		"Build_StorageContainerMk2_C": "Storage Container Mk.2",
		"Build_StorageIntegrated_C":   "Integrated Storage",
		"Build_StoragePlayer_C":       "Personal Storage",
		"Build_CentralStorage_C":      "Dimensional Depot",
	}
	// Additional aliases: display names that appear in data but don't match
	// the canonical display name above
	extraAliases := map[string]string{
		"Storage Container":   "Build_StorageContainerMk2_C",
		"Cyberwagon":          "Desc_CyberWagon_C",
		"Factory Cart":        "Desc_GolfCart_C",
		"Truck Station":       "Build_TruckStation_C",
		"Fluid Truck Station": "Build_FluidTruckStation_C",
		"Train Station":       "Build_TrainStation_C",
		"Drone Station":       "Build_DroneStation_C",
		"Electric Locomotive": "Desc_Locomotive_C",
		"Freight Car":         "Desc_FreightWagon_C",
		"Boombox":             "Desc_BoomBox_C",
	}
	for cls, dn := range extraDisplayNames {
		if _, ok := iconData[cls]; ok {
			if _, exists := iconDisplayNameLookup[dn]; !exists {
				iconDisplayNameLookup[dn] = cls
			}
		}
	}
	for dn, cls := range extraAliases {
		if _, ok := iconData[cls]; ok {
			if _, exists := iconDisplayNameLookup[dn]; !exists {
				iconDisplayNameLookup[dn] = cls
			}
		}
	}

	// Build combined display-name lookup for SelectIconsFromJSON.
	// Only display names are here - reverse lookup needs substring matching
	// so it's handled separately via bytes.Contains.
	iconAllLookups = make(map[string]string,
		len(iconDisplayNameLookup)+len(iconDisplayNameLookupDe))
	for k, v := range iconDisplayNameLookup {
		iconAllLookups[k] = v
	}
	for k, v := range iconDisplayNameLookupDe {
		iconAllLookups[k] = v
	}
}

// currentDisplayLang controls which language's item display names are used.
// "en" is the default. Set via SetItemDisplayLanguage.
var currentDisplayLang = "en"

// SetItemDisplayLanguage switches the item display name table used by
// CleanItemName and GetItemDisplayNames. Call this at startup and whenever
// the user changes language. Falls back to "en" for unknown languages.
func SetItemDisplayLanguage(lang string) {
	if lang == "" || lang == "en" || lang == "de" {
		currentDisplayLang = lang
		if currentDisplayLang == "" {
			currentDisplayLang = "en"
		}
	}
}

// GetCurrentDisplayLang returns the currently active item display language.
func GetCurrentDisplayLang() string {
	return currentDisplayLang
}

// GetItemDisplayNames returns the class name -> display name map from
// item_display_names_{lang}.json plus extra mappings for buildings/collectibles.
// Used to populate the frontend's displayName lookup.
func GetItemDisplayNames() map[string]string {
	result := make(map[string]string)
	var displayNames map[string]string

	var src []byte
	switch currentDisplayLang {
	case "de":
		src = displayNamesDeFS
	default:
		src = displayNamesFS
	}

	if err := json.Unmarshal(src, &displayNames); err == nil {
		for k, v := range displayNames {
			result[k] = v
		}
	}

	return result
}

// GetItemDisplayNamesDe returns the German class name -> display name map.
// Always returns the DE table regardless of currentDisplayLang, so the frontend
// can switch languages without a backend round-trip.
func GetItemDisplayNamesDe() map[string]string {
	result := make(map[string]string)
	var displayNames map[string]string
	if err := json.Unmarshal(displayNamesDeFS, &displayNames); err == nil {
		for k, v := range displayNames {
			result[k] = v
		}
	}
	return result
}

// GetItemDisplayNamesEn returns the English class name -> display name map.
// Always returns the EN table regardless of currentDisplayLang, so the frontend
// can switch languages without a backend round-trip.
func GetItemDisplayNamesEn() map[string]string {
	result := make(map[string]string)
	var displayNames map[string]string
	if err := json.Unmarshal(displayNamesFS, &displayNames); err == nil {
		for k, v := range displayNames {
			result[k] = v
		}
	}
	return result
}

// GetUIStrings returns the English UI string table from ui_strings.json.
func GetUIStrings() map[string]string {
	result := make(map[string]string)
	var m map[string]string
	if err := json.Unmarshal(uiStringsFS, &m); err == nil {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// GetUIStringsDe returns the German UI string table from ui_strings_de.json.
func GetUIStringsDe() map[string]string {
	result := make(map[string]string)
	var m map[string]string
	if err := json.Unmarshal(uiStringsDeFS, &m); err == nil {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// classNameRe matches class names like Build_xxx_C, Desc_xxx_C, BP_xxx_C
// in JSON strings (type paths or bare class names).
var classNameRe = regexp.MustCompile(`(?:Build_|Desc_|BP_)[A-Za-z0-9_]+_C`)

// SelectIconsFromJSON scans marshaled report JSON for class name patterns
// and returns a map of class name -> base64 data URI for only the icons
// that are referenced in the data. This keeps the HTML small by not
// embedding all 700+ icons when a save only uses a fraction of them.
func SelectIconsFromJSON(jsonData []byte) map[string]string {
	needed := make(map[string]bool)

	// Find all class name patterns (Build_xxx_C, Desc_xxx_C, BP_xxx_C)
	for _, m := range classNameRe.FindAll(jsonData, -1) {
		needed[string(m)] = true
	}

	// Display name lookups: extract all unique quoted strings from the
	// JSON in a single pass, then check each against the combined display
	// name map. This replaces the previous O(m*n) bytes.Contains approach
	// for the large display name maps (~3000 entries × 2 languages).
	extractJSONStrings(jsonData, func(s string) {
		if className, ok := iconAllLookups[s]; ok {
			needed[className] = true
		}
	})

	// Reverse lookup: bare resource names (OreIron, LiquidOil, etc.) can
	// appear as substrings inside longer path strings in the JSON. This
	// map is small (~300 entries) so bytes.Contains is affordable here.
	for bare, className := range iconReverseLookup {
		if bytes.Contains(jsonData, []byte(bare)) {
			needed[className] = true
		}
	}

	// Build the result map with only needed icons
	result := make(map[string]string, len(needed))
	for class := range needed {
		if icon, ok := iconData[class]; ok {
			result[class] = icon
		}
	}

	// Always include the mod fallback icon so the viewer can show it
	// for modded items that don't have a matching icon.
	if modIcon, ok := iconData["Mod_Icon_C"]; ok {
		result["Mod_Icon_C"] = modIcon
	}

	return result
}

// extractJSONStrings scans valid JSON and calls cb for each unique string value
// found between double quotes. It handles standard escape sequences (\n, \t,
// \\, \", \uXXXX). String keys and values are both yielded.
func extractJSONStrings(data []byte, cb func(string)) {
	seen := make(map[string]bool)
	i := 0
	n := len(data)
	for i < n {
		if data[i] != '"' {
			i++
			continue
		}
		// Start of a JSON string at position i.
		i++ // skip opening quote
		var sb strings.Builder
		for i < n {
			c := data[i]
			if c == '\\' {
				i++
				if i >= n {
					break
				}
				switch data[i] {
				case 'n':
					sb.WriteByte('\n')
				case 't':
					sb.WriteByte('\t')
				case 'r':
					sb.WriteByte('\r')
				case '\\':
					sb.WriteByte('\\')
				case '"':
					sb.WriteByte('"')
				case '/':
					sb.WriteByte('/')
				case 'b':
					sb.WriteByte('\b')
				case 'f':
					sb.WriteByte('\f')
				case 'u':
					if i+4 < n {
						sb.Write(data[i-1 : i+5]) // keep raw \uXXXX
						i += 4
					}
				default:
					sb.WriteByte(data[i])
				}
				i++
			} else if c == '"' {
				// End of string
				s := sb.String()
				if !seen[s] {
					seen[s] = true
					cb(s)
				}
				i++
				break
			} else {
				sb.WriteByte(c)
				i++
			}
		}
	}
}
