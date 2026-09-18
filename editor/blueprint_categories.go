package editor

import (
	"encoding/binary"
	"fmt"
	"strings"

	"satisfacts/parser"
)

// BlueprintCategoryRecord is a display struct for a blueprint category.
type BlueprintCategoryRecord struct {
	CategoryName  string                 `json:"categoryName"`
	IconID        int32                  `json:"iconId"`
	MenuPriority  float32                `json:"menuPriority"`
	IsUndefined   bool                   `json:"isUndefined"`
	SubCategories []BlueprintSubCategory `json:"subCategories"`
}

// BlueprintSubCategory is a display struct for a blueprint sub-category.
type BlueprintSubCategory struct {
	SubCategoryName string   `json:"subCategoryName"`
	MenuPriority    float32  `json:"menuPriority"`
	IsUndefined     bool     `json:"isUndefined"`
	BlueprintNames  []string `json:"blueprintNames"`
}

// BlueprintSubsystemInfo holds the location of the BlueprintSubsystem object.
type BlueprintSubsystemInfo struct {
	LevelIndex        int // index into levels slice
	ObjectIndex       int // index into level's object list
	Header            *parser.ObjectHeader
	SaveCustomVersion int32
	IsActor           bool
	UseCompleteTag    bool
	// Byte range of the object's binaryData within the decompressed body
	BinaryDataStart int
	BinaryDataSize  int
	// Byte range of the mBlueprintCategoryRecords property value (excluding header)
	// within the object's binaryData
	PropHeaderStart   int   // start of the property name FString
	PropValueStart    int   // start of the property value (after header)
	PropValueEnd      int   // end of the property value
	PropBinarySize    int32 // the propBinarySize field value
	PropBinarySizePos int   // position of propBinarySize field in decompressed body (for updating)
	// Positions for updating after modification (all before the property data, so they don't shift)
	DataLenPos    int // position of the data blob length field in decompressed body
	BinarySizePos int // position of the binarySize field in decompressed body
	LevelVersion  int32
}

// FindBlueprintSubsystem locates the BP_BlueprintSusbystem object in the save.
func FindBlueprintSubsystem(sd *SaveData) (*BlueprintSubsystemInfo, error) {
	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		return nil, fmt.Errorf("parsing body: %w", err)
	}

	for li, l := range levels {
		if !l.IsPersistent || l.ObjectCount == 0 {
			continue
		}
		tocData := sd.Decompressed[l.TocDataStart:l.TocDataEnd]
		headers, err := parser.ParseObjectHeaders(parser.NewBinaryReader(tocData[4:]), sd.VersionCtx, int(l.ObjectCount))
		if err != nil {
			continue
		}
		for i, h := range headers {
			cl := strings.ToLower(h.ClassName)
			if strings.Contains(cl, "blueprintsubsystem") || strings.Contains(cl, "blueprintsusbystem") {
				objStart := l.DataObjOffsets[i]
				objEnd := objStart + l.DataObjSizes[i]
				objData := sd.Decompressed[l.DataBlobStart+objStart : l.DataBlobStart+objEnd]

				// Parse object header to find binaryData location
				offset := 0
				if l.LevelVersion >= 38 {
					offset += 8 // saveCustomVersion + shouldMigrate
				}
				binarySize := int32(binary.LittleEndian.Uint32(objData[offset:]))
				offset += 4
				binaryDataStart := l.DataBlobStart + objStart + offset
				binaryDataSize := int(binarySize)

				saveCustomVersion := l.SaveCustomVersions[i]
				useCompleteTag := IsCompletePropertyTagType(sd.VersionCtx, saveCustomVersion)

				return &BlueprintSubsystemInfo{
					LevelIndex:        li,
					ObjectIndex:       i,
					Header:            &headers[i],
					SaveCustomVersion: saveCustomVersion,
					IsActor:           h.Type == "Actor",
					UseCompleteTag:    useCompleteTag,
					BinaryDataStart:   binaryDataStart,
					BinaryDataSize:    binaryDataSize,
					DataLenPos:        l.DataLenPos,
					BinarySizePos:     l.DataBlobStart + objStart + offset - 4, // offset points past binarySize
					LevelVersion:      l.LevelVersion,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("BlueprintSubsystem not found in save")
}

// LocateCategoryRecordsProperty finds the mBlueprintCategoryRecords property
// within the BlueprintSubsystem's binaryData and returns byte offsets.
func LocateCategoryRecordsProperty(info *BlueprintSubsystemInfo, sd *SaveData) error {
	objData := sd.Decompressed[info.BinaryDataStart : info.BinaryDataStart+info.BinaryDataSize]

	found := false
	WalkObjectProperties(objData, info.IsActor, info.UseCompleteTag, func(it PropIteration) bool {
		if it.Name == "mBlueprintCategoryRecords" {
			info.PropHeaderStart = it.NamePos
			info.PropValueStart = it.ValuePos
			info.PropValueEnd = it.ValuePos + int(it.Header.PropBinarySize)
			info.PropBinarySize = it.Header.PropBinarySize
			info.PropBinarySizePos = info.BinaryDataStart + it.PropBinarySizePos
			found = true
			return false
		}
		return true
	})

	if !found {
		return fmt.Errorf("mBlueprintCategoryRecords property not found")
	}
	return nil
}

// ExtractCategoryRecords parses the mBlueprintCategoryRecords array property
// and returns a display-friendly struct.
func ExtractCategoryRecords(info *BlueprintSubsystemInfo, sd *SaveData) ([]BlueprintCategoryRecord, error) {
	if err := LocateCategoryRecordsProperty(info, sd); err != nil {
		return nil, err
	}

	propData := sd.Decompressed[info.BinaryDataStart+info.PropValueStart : info.BinaryDataStart+info.PropValueEnd]
	r := parser.NewBinaryReader(propData)

	structCount := r.ReadInt32()

	// Skip array header (old format)
	if !info.UseCompleteTag {
		r.ReadString() // hName
		r.ReadString() // hType
		r.ReadInt32()  // hBinSize
		r.ReadInt32()  // hIdx
		r.ReadString() // hSubtype
		r.Skip(16)
		hasGuid := r.ReadUInt8()
		if hasGuid != 0 {
			r.Skip(16)
		}
	}

	categories := make([]BlueprintCategoryRecord, 0, structCount)
	for i := int32(0); i < structCount; i++ {
		cat, err := parseCategoryRecord(r, info.UseCompleteTag)
		if err != nil {
			return nil, fmt.Errorf("category %d: %w", i, err)
		}
		categories = append(categories, cat)
	}

	return categories, nil
}

func parseCategoryRecord(r *parser.BinaryReader, useCompleteTag bool) (BlueprintCategoryRecord, error) {
	cat := BlueprintCategoryRecord{}
	for {
		pn := r.ReadString()
		if pn == "None" {
			break
		}
		nested := ParseNestedPropertyHeader(r, useCompleteTag)

		if nested.PropType == "BoolProperty" && !useCompleteTag {
			cat.IsUndefined = nested.BoolValueFromHeader
			continue
		}
		if nested.PropType == "BoolProperty" && useCompleteTag {
			cat.IsUndefined = (nested.PropFlags & 0x10) != 0
			continue
		}

		valStart := r.Position()

		switch pn {
		case "CategoryName":
			cat.CategoryName = r.ReadString()
		case "IconID":
			cat.IconID = r.ReadInt32()
		case "MenuPriority":
			cat.MenuPriority = r.ReadFloat32()
		case "IsUndefined":
			if nested.PropType == "ByteProperty" {
				cat.IsUndefined = r.ReadUInt8() != 0
			}
		case "SubCategoryRecords":
			subs, err := parseSubCategoryArray(r, useCompleteTag)
			if err != nil {
				return cat, err
			}
			cat.SubCategories = subs
		default:
			r.Skip(int(nested.PropBinarySize))
		}

		// Ensure we're at the expected position
		expectedPos := valStart + int(nested.PropBinarySize)
		if r.Position() != expectedPos {
			r.SetPosition(expectedPos)
		}
	}
	return cat, nil
}

func parseSubCategoryArray(r *parser.BinaryReader, useCompleteTag bool) ([]BlueprintSubCategory, error) {
	subCount := r.ReadInt32()

	// Skip array header (old format)
	if !useCompleteTag {
		r.ReadString() // hName
		r.ReadString() // hType
		r.ReadInt32()  // hBinSize
		r.ReadInt32()  // hIdx
		r.ReadString() // hSubtype
		r.Skip(16)
		hasGuid := r.ReadUInt8()
		if hasGuid != 0 {
			r.Skip(16)
		}
	}

	subs := make([]BlueprintSubCategory, 0, subCount)
	for i := int32(0); i < subCount; i++ {
		sub, err := parseSubCategoryRecord(r, useCompleteTag)
		if err != nil {
			return nil, fmt.Errorf("sub-category %d: %w", i, err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func parseSubCategoryRecord(r *parser.BinaryReader, useCompleteTag bool) (BlueprintSubCategory, error) {
	sub := BlueprintSubCategory{}
	for {
		pn := r.ReadString()
		if pn == "None" {
			break
		}
		nested := ParseNestedPropertyHeader(r, useCompleteTag)

		if nested.PropType == "BoolProperty" && !useCompleteTag {
			sub.IsUndefined = nested.BoolValueFromHeader
			continue
		}
		if nested.PropType == "BoolProperty" && useCompleteTag {
			sub.IsUndefined = (nested.PropFlags & 0x10) != 0
			continue
		}

		valStart := r.Position()

		switch pn {
		case "SubCategoryName":
			sub.SubCategoryName = r.ReadString()
		case "MenuPriority":
			sub.MenuPriority = r.ReadFloat32()
		case "IsUndefined":
			if nested.PropType == "ByteProperty" {
				sub.IsUndefined = r.ReadUInt8() != 0
			}
		case "BlueprintNames":
			bpCount := r.ReadInt32()
			for j := int32(0); j < bpCount; j++ {
				sub.BlueprintNames = append(sub.BlueprintNames, r.ReadString())
			}
		default:
			r.Skip(int(nested.PropBinarySize))
		}

		expectedPos := valStart + int(nested.PropBinarySize)
		if r.Position() != expectedPos {
			r.SetPosition(expectedPos)
		}
	}
	return sub, nil
}

// CopyBlueprintCategoriesResult holds the result of a copy operation.
type CopyBlueprintCategoriesResult struct {
	SourceCategories int    `json:"sourceCategories"`
	TargetCategories int    `json:"targetCategories"`
	FinalCategories  int    `json:"finalCategories"`
	Mode             string `json:"mode"`
}

// CopyBlueprintCategories copies blueprint category records from source to target save.
// mode is "replace" or "merge".
// The target save is modified in place (sd.Decompressed) and should be written by the caller.
//
// Cross-version support: if source and target use different property tag formats
// (e.g. source is v52 old-format, target is v60 complete-tag), categories are
// re-serialized in the target's format instead of copying raw bytes.
func CopyBlueprintCategories(sourceSD, targetSD *SaveData, mode string) (*CopyBlueprintCategoriesResult, error) {
	// Find BlueprintSubsystem in source
	srcInfo, err := FindBlueprintSubsystem(sourceSD)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	if err := LocateCategoryRecordsProperty(srcInfo, sourceSD); err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	// Find BlueprintSubsystem in target
	tgtInfo, err := FindBlueprintSubsystem(targetSD)
	if err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}
	if err := LocateCategoryRecordsProperty(tgtInfo, targetSD); err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}

	// Extract source categories
	srcCategories, err := ExtractCategoryRecords(srcInfo, sourceSD)
	if err != nil {
		return nil, fmt.Errorf("source extract: %w", err)
	}
	tgtCategories, _ := ExtractCategoryRecords(tgtInfo, targetSD)

	result := &CopyBlueprintCategoriesResult{
		SourceCategories: len(srcCategories),
		TargetCategories: len(tgtCategories),
		Mode:             mode,
	}

	// Check if cross-version re-serialization is needed
	crossVersion := srcInfo.UseCompleteTag != tgtInfo.UseCompleteTag

	if mode == "merge" {
		if crossVersion {
			// Re-serialize: merge by filtering source categories not in target,
			// then serialize all (target + new source) in target's format
			mergedCategories := mergeCategories(srcCategories, tgtCategories)
			mergedValue := SerializeCategoryRecords(mergedCategories, tgtInfo.UseCompleteTag)
			result.FinalCategories = len(mergedCategories)
			return replacePropertyInTarget(tgtInfo, targetSD, mergedValue, result)
		}
		// Same format: use raw byte merge (safe, preserves all original data)
		mergedValue, finalCount, err := mergeCategoryRecordsBinary(srcInfo, tgtInfo, sourceSD, targetSD)
		if err != nil {
			return nil, fmt.Errorf("merge: %w", err)
		}
		result.FinalCategories = finalCount
		return replacePropertyInTarget(tgtInfo, targetSD, mergedValue, result)
	}

	// Replace mode
	if crossVersion {
		// Re-serialize source categories in target's format
		newValue := SerializeCategoryRecords(srcCategories, tgtInfo.UseCompleteTag)
		result.FinalCategories = len(srcCategories)
		return replacePropertyInTarget(tgtInfo, targetSD, newValue, result)
	}

	// Same format: copy raw bytes
	srcPropValue := sourceSD.Decompressed[srcInfo.BinaryDataStart+srcInfo.PropValueStart : srcInfo.BinaryDataStart+srcInfo.PropValueEnd]
	result.FinalCategories = len(srcCategories)
	return replacePropertyInTarget(tgtInfo, targetSD, srcPropValue, result)
}

// mergeCategories combines target categories with source categories that don't
// already exist in target (dedup by category name, case-insensitive).
func mergeCategories(srcCats, tgtCats []BlueprintCategoryRecord) []BlueprintCategoryRecord {
	tgtNames := make(map[string]bool)
	for _, c := range tgtCats {
		tgtNames[strings.ToLower(c.CategoryName)] = true
	}
	merged := make([]BlueprintCategoryRecord, len(tgtCats), len(tgtCats)+len(srcCats))
	copy(merged, tgtCats)
	for _, c := range srcCats {
		if !tgtNames[strings.ToLower(c.CategoryName)] {
			merged = append(merged, c)
		}
	}
	return merged
}

// mergeCategoryRecordsBinary merges source categories into target by concatenating
// struct elements. It parses the array to find struct element boundaries, then
// copies target elements + source elements whose CategoryName doesn't exist in target.
func mergeCategoryRecordsBinary(srcInfo, tgtInfo *BlueprintSubsystemInfo, sourceSD, targetSD *SaveData) ([]byte, int, error) {
	// Parse source array to get struct element byte ranges
	srcElements, err := parseArrayStructElements(srcInfo, sourceSD)
	if err != nil {
		return nil, 0, fmt.Errorf("source array: %w", err)
	}

	// Parse target array to get struct element byte ranges
	tgtElements, err := parseArrayStructElements(tgtInfo, targetSD)
	if err != nil {
		return nil, 0, fmt.Errorf("target array: %w", err)
	}

	// Get target category names for dedup
	tgtCategories, err := ExtractCategoryRecords(tgtInfo, targetSD)
	if err != nil {
		return nil, 0, fmt.Errorf("target categories: %w", err)
	}
	tgtNames := make(map[string]bool)
	for _, c := range tgtCategories {
		tgtNames[strings.ToLower(c.CategoryName)] = true
	}

	// Get source category names to know which to skip
	srcCategories, err := ExtractCategoryRecords(srcInfo, sourceSD)
	if err != nil {
		return nil, 0, fmt.Errorf("source categories: %w", err)
	}

	// srcCategories is indexed by position against srcElements below (and
	// srcCount below that governs how many struct elements the array
	// actually holds); if the two ever desynchronize on malformed source
	// data, that indexing would panic instead of failing cleanly - mirrors
	// the same check mergePresetIntoTarget makes for presetElements/presetCats.
	if len(srcElements) != len(srcCategories) {
		return nil, 0, fmt.Errorf("source is inconsistent: %d struct elements but %d category records",
			len(srcElements), len(srcCategories))
	}

	// Build merged array: all target elements + source elements not in target
	srcPropValue := sourceSD.Decompressed[srcInfo.BinaryDataStart+srcInfo.PropValueStart : srcInfo.BinaryDataStart+srcInfo.PropValueEnd]
	tgtPropValue := targetSD.Decompressed[tgtInfo.BinaryDataStart+tgtInfo.PropValueStart : tgtInfo.BinaryDataStart+tgtInfo.PropValueEnd]

	// Parse the array header from target to know how to rebuild
	tgtR := parser.NewBinaryReader(tgtPropValue)
	tgtCount := tgtR.ReadInt32()

	// Read array header (old format) - we need to preserve it
	tgtArrayHeaderStart := 4
	tgtArrayHeaderEnd := tgtR.Position()

	// Read struct elements from target
	tgtStructStart := tgtR.Position()
	_ = tgtCount
	_ = tgtStructStart

	// Parse source array header
	srcR := parser.NewBinaryReader(srcPropValue)
	srcCount := srcR.ReadInt32()
	srcArrayHeaderEnd := srcR.Position()

	// Now rebuild: target array header (with updated count) + target elements + source elements (filtered)
	var merged []byte

	// Copy target array header but update the count
	merged = append(merged, tgtPropValue[:tgtArrayHeaderEnd]...)

	// Count how many source elements we'll add
	addCount := int32(0)
	for i, sc := range srcCategories {
		if !tgtNames[strings.ToLower(sc.CategoryName)] {
			addCount++
			_ = i
		}
	}

	// Update the count in the merged header
	newCount := int32(len(tgtElements)) + addCount
	binary.LittleEndian.PutUint32(merged[0:4], uint32(newCount))

	// Copy target struct elements
	for _, elem := range tgtElements {
		merged = append(merged, tgtPropValue[elem.StartByte:elem.EndByte]...)
	}

	// Copy source struct elements that aren't in target
	for i, elem := range srcElements {
		if !tgtNames[strings.ToLower(srcCategories[i].CategoryName)] {
			merged = append(merged, srcPropValue[elem.StartByte:elem.EndByte]...)
		}
	}

	finalCount := int(newCount)
	_ = srcCount
	_ = srcArrayHeaderEnd
	_ = tgtArrayHeaderStart

	return merged, finalCount, nil
}

// arrayStructElement represents a single struct element's byte range within the array property value.
type arrayStructElement struct {
	StartByte int
	EndByte   int
}

// parseArrayStructElements parses an ArrayProperty<StructProperty> value to find
// the byte ranges of each struct element.
func parseArrayStructElements(info *BlueprintSubsystemInfo, sd *SaveData) ([]arrayStructElement, error) {
	propValue := sd.Decompressed[info.BinaryDataStart+info.PropValueStart : info.BinaryDataStart+info.PropValueEnd]
	r := parser.NewBinaryReader(propValue)

	count := r.ReadInt32()

	// Skip array header (old format)
	if !info.UseCompleteTag {
		r.ReadString() // hName
		r.ReadString() // hType
		r.ReadInt32()  // hBinSize
		r.ReadInt32()  // hIdx
		r.ReadString() // hSubtype
		r.Skip(16)
		hasGuid := r.ReadUInt8()
		if hasGuid != 0 {
			r.Skip(16)
		}
	}

	elements := make([]arrayStructElement, 0, count)
	for i := int32(0); i < count; i++ {
		elemStart := r.Position()
		// Walk the struct's property list until "None"
		for {
			pn := r.ReadString()
			if pn == "None" {
				break
			}
			nested := ParseNestedPropertyHeader(r, info.UseCompleteTag)
			if nested.PropType == "BoolProperty" && !info.UseCompleteTag {
				continue
			}
			if nested.PropType == "BoolProperty" && info.UseCompleteTag {
				continue
			}
			valStart := r.Position()
			// Skip the value
			r.SetPosition(valStart + int(nested.PropBinarySize))
		}
		elemEnd := r.Position()
		elements = append(elements, arrayStructElement{StartByte: elemStart, EndByte: elemEnd})
	}

	return elements, nil
}

// replacePropertyInTarget swaps the mBlueprintCategoryRecords property value in
// the target's decompressed body for newPropValue.
//
// Four size fields describe the region and all four have to be corrected, or the
// game reads garbage: propBinarySize, the object's binarySize, the level's data
// blob length, and totalBodyRestSize. Miss the last one and objects at the end of
// the blob get truncated - that was the pipes-with-no-spline-points crash.
//
// All four live before the property data, so the stored positions stay valid even
// though the body grows or shrinks.
func replacePropertyInTarget(tgtInfo *BlueprintSubsystemInfo, targetSD *SaveData, newPropValue []byte, result *CopyBlueprintCategoriesResult) (*CopyBlueprintCategoriesResult, error) {
	propValueAbsStart := tgtInfo.BinaryDataStart + tgtInfo.PropValueStart
	propValueAbsEnd := tgtInfo.BinaryDataStart + tgtInfo.PropValueEnd
	oldPropLen := propValueAbsEnd - propValueAbsStart
	newPropLen := len(newPropValue)
	sizeDiff := newPropLen - oldPropLen

	// Build new decompressed body with the property value replaced
	newDecompressed := make([]byte, 0, len(targetSD.Decompressed)+sizeDiff)
	newDecompressed = append(newDecompressed, targetSD.Decompressed[:propValueAbsStart]...)
	newDecompressed = append(newDecompressed, newPropValue...)
	newDecompressed = append(newDecompressed, targetSD.Decompressed[propValueAbsEnd:]...)
	targetSD.Decompressed = newDecompressed

	newPropBinarySize := int32(newPropLen)
	binary.LittleEndian.PutUint32(targetSD.Decompressed[tgtInfo.PropBinarySizePos:], uint32(newPropBinarySize))

	oldBinarySize := int32(binary.LittleEndian.Uint32(targetSD.Decompressed[tgtInfo.BinarySizePos:]))
	newBinarySize := oldBinarySize + int32(sizeDiff)
	binary.LittleEndian.PutUint32(targetSD.Decompressed[tgtInfo.BinarySizePos:], uint32(newBinarySize))

	oldDataBlobLen := int64(binary.LittleEndian.Uint64(targetSD.Decompressed[tgtInfo.DataLenPos:]))
	newDataBlobLen := oldDataBlobLen + int64(sizeDiff)
	binary.LittleEndian.PutUint64(targetSD.Decompressed[tgtInfo.DataLenPos:], uint64(newDataBlobLen))

	WriteTotalBodyRestSize(targetSD.Decompressed, targetSD.VersionCtx)

	return result, nil
}
