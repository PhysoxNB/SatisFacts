package editor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"satisfacts/parser"
)

// Preset magic header: "SFBCAT01" - SatisFacts Blueprint CATegories, version 01
var presetMagic = []byte("SFBCAT01")

// BlueprintPreset is the display struct stored inside a .sfbcat file (JSON payload).
type BlueprintPreset struct {
	Categories      []BlueprintCategoryRecord `json:"categories"`
	UseCompleteTag  bool                      `json:"useCompleteTag"`
	SaveVersion     int32                     `json:"saveVersion"`
	PropBinarySize  int32                     `json:"propBinarySize"`
	RawPropertyData []byte                    `json:"-"` // not JSON-serialised
}

// ExportBlueprintPreset writes a .sfbcat preset file from a source save.
// The file contains a binary header (magic + version + useCompleteTag flag +
// propBinarySize) followed by the raw mBlueprintCategoryRecords property value bytes.
// A JSON metadata footer is appended for human-readable category names.
func ExportBlueprintPreset(savePath, outputPath string) (categories []BlueprintCategoryRecord, err error) {
	sd, err := LoadSave(savePath)
	if err != nil {
		return nil, fmt.Errorf("loading save: %w", err)
	}

	info, err := FindBlueprintSubsystem(sd)
	if err != nil {
		return nil, fmt.Errorf("finding blueprint subsystem: %w", err)
	}

	if err := LocateCategoryRecordsProperty(info, sd); err != nil {
		return nil, fmt.Errorf("locating category records: %w", err)
	}

	categories, err = ExtractCategoryRecords(info, sd)
	if err != nil {
		return nil, fmt.Errorf("extracting categories: %w", err)
	}

	// Extract raw property value bytes
	propValue := make([]byte, info.PropValueEnd-info.PropValueStart)
	copy(propValue, sd.Decompressed[info.BinaryDataStart+info.PropValueStart:info.BinaryDataStart+info.PropValueEnd])

	// Build the preset file:
	// [8 bytes magic] [1 byte useCompleteTag] [4 bytes propBinarySize] [4 bytes raw data length]
	// [raw property bytes] [JSON metadata]
	var file []byte
	file = append(file, presetMagic...)
	file = append(file, boolByte(info.UseCompleteTag))
	file = append(file, u32bytes(uint32(info.PropBinarySize))...)
	file = append(file, u32bytes(uint32(len(propValue)))...)
	file = append(file, propValue...)

	meta := struct {
		Categories     []BlueprintCategoryRecord `json:"categories"`
		UseCompleteTag bool                      `json:"useCompleteTag"`
	}{
		Categories:     categories,
		UseCompleteTag: info.UseCompleteTag,
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling metadata: %w", err)
	}
	file = append(file, metaBytes...)

	if err := os.WriteFile(outputPath, file, 0644); err != nil {
		return nil, fmt.Errorf("writing preset file: %w", err)
	}

	return categories, nil
}

// ImportBlueprintPreset reads a .sfbcat preset file and imports the categories
// into a target save. mode is "replace" or "merge". Returns the result and the
// modified SaveData - the caller is responsible for writing it.
func ImportBlueprintPreset(presetPath, targetPath, mode string) (result *CopyBlueprintCategoriesResult, targetSD *SaveData, err error) {
	fileData, err := os.ReadFile(presetPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading preset file: %w", err)
	}

	if len(fileData) < 17 { // 8 magic + 1 flag + 4 propBinSize + 4 dataLen
		return nil, nil, fmt.Errorf("preset file is too small or corrupt")
	}

	if !bytes.Equal(fileData[:8], presetMagic) {
		return nil, nil, fmt.Errorf("not a valid SatisFacts blueprint preset file (magic mismatch)")
	}

	useCompleteTag := fileData[8] != 0
	// fileData[9:13] holds the source propBinarySize; it is recomputed from the
	// payload on import, so it is not read back here.
	rawDataLen := int32(binary.LittleEndian.Uint32(fileData[13:17]))

	// The negative check has to come first: a negative length would sail through
	// the length comparison below and then panic on the slice.
	if rawDataLen < 0 {
		return nil, nil, fmt.Errorf("preset file declares a negative payload length (%d)", rawDataLen)
	}
	if len(fileData) < 17+int(rawDataLen) {
		return nil, nil, fmt.Errorf("preset file is truncated or corrupt")
	}
	rawPropValue := fileData[17 : 17+rawDataLen]

	// Load target save
	targetSD, err = LoadSave(targetPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading target save: %w", err)
	}

	tgtInfo, err := FindBlueprintSubsystem(targetSD)
	if err != nil {
		return nil, nil, fmt.Errorf("target: %w", err)
	}
	if err := LocateCategoryRecordsProperty(tgtInfo, targetSD); err != nil {
		return nil, nil, fmt.Errorf("target: %w", err)
	}

	// Check if cross-version re-serialization is needed
	crossVersion := tgtInfo.UseCompleteTag != useCompleteTag

	// Extract target categories for counting
	tgtCategories, _ := ExtractCategoryRecords(tgtInfo, targetSD)

	// Parse preset categories from the JSON metadata
	presetCategories, err := parsePresetCategories(fileData, 17+int(rawDataLen))
	if err != nil {
		return nil, nil, fmt.Errorf("reading preset metadata: %w", err)
	}

	result = &CopyBlueprintCategoriesResult{
		SourceCategories: len(presetCategories),
		TargetCategories: len(tgtCategories),
		Mode:             mode,
	}

	if crossVersion {
		// Re-serialize preset categories in target's format
		if mode == "merge" {
			mergedCategories := mergeCategories(presetCategories, tgtCategories)
			mergedValue := SerializeCategoryRecords(mergedCategories, tgtInfo.UseCompleteTag)
			result.FinalCategories = len(mergedCategories)
			_, err = replacePropertyInTarget(tgtInfo, targetSD, mergedValue, result)
			if err != nil {
				return nil, nil, err
			}
			return result, targetSD, nil
		}
		// Replace mode
		newValue := SerializeCategoryRecords(presetCategories, tgtInfo.UseCompleteTag)
		result.FinalCategories = len(presetCategories)
		_, err = replacePropertyInTarget(tgtInfo, targetSD, newValue, result)
		if err != nil {
			return nil, nil, err
		}
		return result, targetSD, nil
	}

	// Same format: use raw byte copy (preserves all original data)
	if mode == "merge" {
		mergedValue, finalCount, err := mergePresetIntoTarget(rawPropValue, useCompleteTag, presetCategories, tgtInfo, targetSD)
		if err != nil {
			return nil, nil, fmt.Errorf("merge: %w", err)
		}
		result.FinalCategories = finalCount
		_, err = replacePropertyInTarget(tgtInfo, targetSD, mergedValue, result)
		if err != nil {
			return nil, nil, err
		}
		return result, targetSD, nil
	}

	// Replace mode: swap the property value bytes directly
	result.FinalCategories = len(presetCategories)
	_, err = replacePropertyInTarget(tgtInfo, targetSD, rawPropValue, result)
	if err != nil {
		return nil, nil, err
	}
	return result, targetSD, nil
}

// mergePresetIntoTarget merges preset category struct elements into the target.
func mergePresetIntoTarget(presetPropValue []byte, useCompleteTag bool, presetCats []BlueprintCategoryRecord, tgtInfo *BlueprintSubsystemInfo, targetSD *SaveData) ([]byte, int, error) {
	// Parse target array to get struct element byte ranges
	tgtElements, err := parseArrayStructElements(tgtInfo, targetSD)
	if err != nil {
		return nil, 0, fmt.Errorf("target array: %w", err)
	}

	// Get target category names for dedup
	tgtCategories, _ := ExtractCategoryRecords(tgtInfo, targetSD)
	tgtNames := make(map[string]bool)
	for _, c := range tgtCategories {
		tgtNames[strings.ToLower(c.CategoryName)] = true
	}

	// Parse preset array to get struct element byte ranges
	presetElements, err := parsePresetArrayStructElements(presetPropValue, useCompleteTag)
	if err != nil {
		return nil, 0, fmt.Errorf("preset array: %w", err)
	}

	// The element ranges come from the binary payload while the names come from
	// the JSON footer. If a hand-edited preset desynchronises the two, indexing
	// presetCats by element position would panic.
	if len(presetElements) != len(presetCats) {
		return nil, 0, fmt.Errorf("preset is inconsistent: %d category records in the payload but %d in the metadata",
			len(presetElements), len(presetCats))
	}

	// Get target property value for rebuilding
	tgtPropValue := targetSD.Decompressed[tgtInfo.BinaryDataStart+tgtInfo.PropValueStart : tgtInfo.BinaryDataStart+tgtInfo.PropValueEnd]
	tgtR := parser.NewBinaryReader(tgtPropValue)
	tgtR.ReadInt32() // count
	tgtArrayHeaderEnd := tgtR.Position()

	// Build merged array: target array header (with updated count) + target elements + preset elements (filtered)
	var merged []byte
	merged = append(merged, tgtPropValue[:tgtArrayHeaderEnd]...)

	addCount := int32(0)
	for _, pc := range presetCats {
		if !tgtNames[strings.ToLower(pc.CategoryName)] {
			addCount++
		}
	}

	newCount := int32(len(tgtElements)) + addCount
	binary.LittleEndian.PutUint32(merged[0:4], uint32(newCount))

	for _, elem := range tgtElements {
		merged = append(merged, tgtPropValue[elem.StartByte:elem.EndByte]...)
	}

	for i, elem := range presetElements {
		if !tgtNames[strings.ToLower(presetCats[i].CategoryName)] {
			merged = append(merged, presetPropValue[elem.StartByte:elem.EndByte]...)
		}
	}

	return merged, int(newCount), nil
}

// parsePresetArrayStructElements parses the preset's array property value to find
// struct element byte ranges, using the same logic as parseArrayStructElements but
// operating on a standalone byte slice instead of the full decompressed body.
// useCompleteTag must be the flag stored in the preset, not the target's.
func parsePresetArrayStructElements(propValue []byte, useCompleteTag bool) ([]arrayStructElement, error) {
	r := parser.NewBinaryReader(propValue)
	count := r.ReadInt32()

	if !useCompleteTag {
		r.ReadString()
		r.ReadString()
		r.ReadInt32()
		r.ReadInt32()
		r.ReadString()
		r.Skip(16)
		hasGuid := r.ReadUInt8()
		if hasGuid != 0 {
			r.Skip(16)
		}
	}

	elements := make([]arrayStructElement, 0, count)
	for i := int32(0); i < count; i++ {
		elemStart := r.Position()
		for {
			pn := r.ReadString()
			if pn == "None" {
				break
			}
			nested := ParseNestedPropertyHeader(r, useCompleteTag)
			if nested.PropType == "BoolProperty" {
				continue
			}
			valStart := r.Position()
			r.SetPosition(valStart + int(nested.PropBinarySize))
		}
		elemEnd := r.Position()
		elements = append(elements, arrayStructElement{StartByte: elemStart, EndByte: elemEnd})
	}

	return elements, nil
}

// ReadPresetCategories reads a .sfbcat preset file and returns the category records
// from its JSON metadata. Used by the frontend to preview preset contents.
func ReadPresetCategories(presetPath string) ([]BlueprintCategoryRecord, error) {
	fileData, err := os.ReadFile(presetPath)
	if err != nil {
		return nil, fmt.Errorf("reading preset file: %w", err)
	}

	if len(fileData) < 17 {
		return nil, fmt.Errorf("preset file is too small or corrupt")
	}

	if !bytes.Equal(fileData[:8], presetMagic) {
		return nil, fmt.Errorf("not a valid SatisFacts blueprint preset file")
	}

	rawDataLen := int32(binary.LittleEndian.Uint32(fileData[13:17]))
	if rawDataLen < 0 {
		return nil, fmt.Errorf("preset file declares a negative payload length (%d)", rawDataLen)
	}
	jsonStart := 17 + int(rawDataLen)
	if jsonStart >= len(fileData) {
		return nil, fmt.Errorf("preset file is truncated or corrupt")
	}

	return parsePresetCategories(fileData, jsonStart)
}

// parsePresetCategories extracts the category records from the JSON metadata at the end of the file.
// A malformed footer is reported rather than swallowed: returning nil silently made
// "merge" a no-op and made "replace" report 0 categories while still rewriting the save.
func parsePresetCategories(fileData []byte, jsonStart int) ([]BlueprintCategoryRecord, error) {
	if jsonStart >= len(fileData) {
		return nil, fmt.Errorf("preset file has no metadata footer")
	}
	var meta struct {
		Categories []BlueprintCategoryRecord `json:"categories"`
	}
	if err := json.Unmarshal(fileData[jsonStart:], &meta); err != nil {
		return nil, fmt.Errorf("preset metadata is not valid JSON: %w", err)
	}
	return meta.Categories, nil
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func u32bytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}
