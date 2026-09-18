package editor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"satisfacts/parser"
)

// Pipe class name patterns in Satisfactory:
//   Mk1: /Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline.Build_Pipeline_C
//        /Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline_NoIndicator.Build_Pipeline_NoIndicator_C
//   Mk2: /Game/FactoryGame/Buildable/Factory/PipelineMk2/Build_PipelineMK2.Build_PipelineMK2_C
//        /Game/FactoryGame/Buildable/Factory/PipelineMk2/Build_PipelineMK2_NoIndicator.Build_PipelineMK2_NoIndicator_C
//
// Other Pipeline buildables that must NOT be changed:
//   Build_PipelineSupport, Build_PipelineFlowIndicator, Build_PipelineConnectionComponent, etc.
//
// The key difference: "Build_Pipeline" followed by ".", "_C_", or "_NoIndicator" is a pipe.
// "Build_Pipeline" followed by "Support", "Flow", "Connection" etc. is NOT a pipe.

// upgradePipeString replaces Mk1 pipe class names with Mk2 in a string.
// ONLY the full class name with directory path is replaced (e.g.
// /Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline.Build_Pipeline_C).
// Path names like Build_Pipeline_C_<id> are NOT changed - other objects
// reference pipes by path name, so changing them breaks connections.
func upgradePipeString(s string) string {
	if !strings.Contains(s, "Build_Pipeline") {
		return s
	}
	if strings.Contains(s, "Build_PipelineMK2") {
		return s
	}

	result := s

	// Only replace full class name with directory path
	result = strings.ReplaceAll(result,
		"/Pipeline/Build_Pipeline.Build_Pipeline_C",
		"/PipelineMk2/Build_PipelineMK2.Build_PipelineMK2_C")
	result = strings.ReplaceAll(result,
		"/Pipeline/Build_Pipeline_NoIndicator.Build_Pipeline_NoIndicator_C",
		"/PipelineMk2/Build_PipelineMK2_NoIndicator.Build_PipelineMK2_NoIndicator_C")

	return result
}

// pipeStringNeedsUpgrade returns true if the string contains a Mk1 pipe
// class name with directory path. Only class names are upgraded, not path
// names (which are object identifiers referenced by other objects).
func pipeStringNeedsUpgrade(s string) bool {
	return strings.Contains(s, "Build_Pipeline") &&
		!strings.Contains(s, "Build_PipelineMK2") &&
		(strings.Contains(s, "/Pipeline/Build_Pipeline.") ||
			strings.Contains(s, "/Pipeline/Build_Pipeline_NoIndicator."))
}

// replacePipeStrings scans a byte slice for length-prefixed strings containing
// Mk1 pipe patterns and replaces them with Mk2 equivalents. Returns a new byte
// slice (may be longer than the input due to string length changes).
//
// headerOffsets/headerSizes, when non-nil, are the structurally-parsed
// per-object header regions within data (e.g. a level's TOC object headers,
// already computed by ParseBodyLevels) in ascending, non-overlapping order.
// When a "Build_Pipeline" match falls inside one of these regions, the
// backward scan for its enclosing length-prefixed string is clamped to that
// region instead of an arbitrary 500-byte window - pinning the match to the
// real FString boundary instead of a coincidentally-plausible one elsewhere
// in the blob. Pass nil for data with no such structural offsets available
// (e.g. the level footer), which keeps the old window-only behavior there.
func replacePipeStrings(data []byte, headerOffsets, headerSizes []int) []byte {
	var replacements []replacement
	needle := []byte("Build_Pipeline")
	headerIdx := 0

	for i := 0; i <= len(data)-len(needle); i++ {
		if data[i] != needle[0] || !bytes.Equal(data[i:i+len(needle)], needle) {
			continue
		}
		// Skip if already MK2
		if i+len(needle)+3 <= len(data) && data[i+len(needle)] == 'M' && data[i+len(needle)+1] == 'K' && data[i+len(needle)+2] == '2' {
			continue
		}
		// Only match class names: Build_Pipeline. or Build_Pipeline_NoIndicator.
		// Skip path names (Build_Pipeline_C_<id>), Support, Flow, Connection, etc.
		if i+len(needle) >= len(data) {
			continue
		}
		nextChar := data[i+len(needle)]
		if nextChar != '.' && nextChar != '_' {
			continue
		}
		if nextChar == '_' {
			rest := data[i+len(needle):]
			if len(rest) < 3 {
				continue
			}
			sub := string(rest[:3])
			// Only _NoIndicator is valid (class name); skip _C_, _Su, _Fl, _Co, _Ja
			if sub != "_No" {
				continue
			}
		}

		// Advance to the structural header region (if any) that could
		// contain this match. headerOffsets is ascending, so a single
		// forward-only cursor suffices across the whole outer loop.
		for headerIdx+1 < len(headerOffsets) && headerOffsets[headerIdx+1] <= i {
			headerIdx++
		}

		// Find the enclosing length-prefixed string by scanning backwards.
		// We validate two extra constraints to eliminate false positives:
		//  1. The string must start with /Game/ (all UE class names do)
		//  2. The needle must be within the first 200 chars of the string
		//     (class names are near the start of the path)
		scanFloor := i - 500
		if scanFloor < 4 {
			scanFloor = 4
		}
		if headerIdx < len(headerOffsets) {
			regionStart, regionEnd := headerOffsets[headerIdx], headerOffsets[headerIdx]+headerSizes[headerIdx]
			if i >= regionStart && i < regionEnd && regionStart+4 > scanFloor {
				scanFloor = regionStart + 4
			}
		}

		found := false
		for s := i; s >= scanFloor; s-- {
			if s < 4 {
				break
			}
			l := int(int32(binary.LittleEndian.Uint32(data[s-4:])))
			if l <= 0 || l > 1000 {
				continue
			}
			if s+l > len(data) {
				continue
			}
			if data[s+l-1] != 0 {
				continue
			}
			// Verify the needle is within this string
			if i+len(needle) > s+l-1 {
				continue
			}
			// Verify the string starts with /Game/ — all UE object paths do
			if l < 6 || string(data[s:s+6]) != "/Game/" {
				continue
			}
			// Verify the needle is near the start of the string (within the path)
			if i-s > 200 {
				continue
			}
			// Extract the string and check if it needs upgrade
			oldStr := string(data[s : s+l-1]) // exclude null terminator
			if !pipeStringNeedsUpgrade(oldStr) {
				break
			}
			newStr := upgradePipeString(oldStr)
			if newStr == oldStr {
				break
			}
			newData := append([]byte(newStr), 0) // add null terminator
			replacements = append(replacements, replacement{
				lengthPos: s - 4,
				oldLen:    l,
				newData:   newData,
			})
			found = true
			break
		}
		_ = found
	}

	if len(replacements) == 0 {
		return data
	}

	// Sort by position and remove overlaps
	sortReplacements(replacements)

	// Build new data
	var result []byte
	pos := 0
	for _, r := range replacements {
		if r.lengthPos < pos {
			continue // overlapping
		}
		result = append(result, data[pos:r.lengthPos]...)
		var lenBuf [4]byte
		binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(r.newData)))
		result = append(result, lenBuf[:]...)
		result = append(result, r.newData...)
		pos = r.lengthPos + 4 + r.oldLen
	}
	result = append(result, data[pos:]...)

	return result
}

// sortReplacements sorts replacements by lengthPos and removes overlapping entries.
func sortReplacements(reps []replacement) {
	// Simple insertion sort (reps is typically small)
	for i := 1; i < len(reps); i++ {
		for j := i; j > 0 && reps[j].lengthPos < reps[j-1].lengthPos; j-- {
			reps[j], reps[j-1] = reps[j-1], reps[j]
		}
	}
}

// replacement is a planned string replacement in the body.
type replacement struct {
	lengthPos int
	oldLen    int
	newData   []byte
}

// UpgradePipes upgrades all Mk1 pipelines to Mk2 in the decompressed body.
// Unlike belts/lifts which use in-place digit patching, pipes require string
// length changes (Build_Pipeline -> Build_PipelineMK2 adds 3 bytes), so the
// body must be rebuilt with updated length fields for TOC, data blob, and
// per-object binarySize fields.
func UpgradePipes(sd *SaveData) (pipeCount int, err error) {
	body := sd.Decompressed
	ctx := sd.VersionCtx

	bodyHeaderEnd, levels, trailingStart, trailingEnd, err := ParseBodyLevels(body, ctx)
	if err != nil {
		return 0, fmt.Errorf("parsing body: %w", err)
	}

	// Count how many pipe objects will be upgraded (from original body)
	for _, level := range levels {
		if level.ObjectCount == 0 {
			continue
		}
		tocData := body[level.TocDataStart:level.TocDataEnd]
		tocReader := parser.NewBinaryReader(tocData[4:])
		headers, hErr := parser.ParseObjectHeaders(tocReader, ctx, int(level.ObjectCount))
		if hErr != nil {
			continue
		}
		for _, h := range headers {
			cn := h.ClassName
			if strings.Contains(cn, "Build_Pipeline.") || strings.Contains(cn, "Build_Pipeline_NoIndicator.") {
				if !strings.Contains(cn, "MK2") {
					pipeCount++
				}
			}
		}
	}

	// Build new body
	var newBody []byte

	// Copy body header (up to first level)
	newBody = append(newBody, body[:bodyHeaderEnd]...)

	for _, level := range levels {
		// Copy level name (if sublevel)
		if !level.IsPersistent {
			newBody = append(newBody, body[level.NameStart:level.NameEnd]...)
		}

		// Process TOC blob - strings in TOC headers can change length
		oldTocBlob := body[level.TocDataStart:level.TocDataEnd]
		newTocBlob := replacePipeStrings(oldTocBlob, level.TocHeaderOffsets, level.TocHeaderSizes)

		// Write TOC length + TOC blob
		var lenBuf [8]byte
		binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(newTocBlob)))
		newBody = append(newBody, lenBuf[:]...)
		newBody = append(newBody, newTocBlob...)

		// Process data blob - must handle per-object binarySize fields
		newDataBlob := upgradeDataBlobPipes(body, level)

		// Write data blob length + data blob
		binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(newDataBlob)))
		newBody = append(newBody, lenBuf[:]...)
		newBody = append(newBody, newDataBlob...)

		// Process footer
		oldFooter := body[level.FooterStart:level.FooterEnd]
		newFooter := replacePipeStrings(oldFooter, nil, nil)
		newBody = append(newBody, newFooter...)
	}

	// Copy trailing data
	if trailingEnd > trailingStart {
		newBody = append(newBody, body[trailingStart:trailingEnd]...)
	}

	WriteTotalBodyRestSize(newBody, sd.VersionCtx)

	sd.Decompressed = newBody
	return pipeCount, nil
}

// upgradeDataBlobPipes processes the data blob for a level, replacing pipe
// strings in each object's binary data and updating binarySize fields.
func upgradeDataBlobPipes(body []byte, level *LevelInfo) []byte {
	if level.ObjectCount == 0 {
		return body[level.DataBlobStart:level.DataBlobEnd]
	}

	dataBlob := body[level.DataBlobStart:level.DataBlobEnd]

	// The data blob starts with countEntities (int32), then per-object entries.
	// We use the pre-parsed DataObjOffsets and DataObjSizes to identify each
	// object's region. Each region contains:
	//   [saveCustomVersion + shouldMigrate] (if levelVersion >= 38)
	//   binarySize (int32)
	//   binaryData (binarySize bytes)  ← strings live here
	//   [shouldSerialize + FSaveObjectVersionData] (if saveCustomVersion >= 53)

	var result []byte

	// Copy countEntities (first 4 bytes)
	result = append(result, dataBlob[:4]...)

	for i := int32(0); i < level.ObjectCount; i++ {
		objStart := level.DataObjOffsets[i]
		objEnd := objStart + level.DataObjSizes[i]
		objData := dataBlob[objStart:objEnd]

		// Parse the object entry to find its region
		// No string replacement in data blob - see comment below
		_ = level.LevelVersion

		// Do NOT replace pipe strings in binaryData/restData. The property
		// structure uses propBinarySize fields that would become stale when
		// string lengths change, corrupting subsequent property reads. The
		// TOC class name change is sufficient for the game to load the entity
		// as Mk2; recipe references in mCustomizationData are just metadata.
		result = append(result, objData...)
	}

	return result
}
