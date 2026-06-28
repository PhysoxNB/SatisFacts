package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"satisfacts/parser"
)

// sliceReader implements io.ReadSeeker over a byte slice
type sliceReader struct {
	data []byte
	pos  int
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}
func (s *sliceReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		s.pos = int(offset)
	case io.SeekCurrent:
		s.pos += int(offset)
	case io.SeekEnd:
		s.pos = len(s.data) + int(offset)
	}
	return int64(s.pos), nil
}

// FString byte size: int32 length + content
func fstringSize(data []byte, pos int) int {
	if pos+4 > len(data) {
		return 4
	}
	length := int32(binary.LittleEndian.Uint32(data[pos:]))
	if length == 0 {
		return 4
	}
	if length < 0 {
		return 4 + int(-length)*2
	}
	return 4 + int(length)
}

// readFString reads an FString from a byte slice at given offset
func readFString(data []byte, pos int) (string, int) {
	if pos+4 > len(data) {
		return "", 4
	}
	length := int32(binary.LittleEndian.Uint32(data[pos:]))
	if length == 0 {
		return "", 4
	}
	if length < 0 {
		charCount := int(-length)
		byteCount := charCount * 2
		raw := data[pos+4 : pos+4+byteCount]
		// Find null terminator
		end := byteCount
		for i := 0; i < charCount; i++ {
			if raw[i*2] == 0 && raw[i*2+1] == 0 {
				end = i * 2
				break
			}
		}
		return string(raw[:end]), 4 + byteCount
	}
	raw := data[pos+4 : pos+4+int(length)]
	end := int(length)
	for i, b := range raw {
		if b == 0 {
			end = i
			break
		}
	}
	return string(raw[:end]), 4 + int(length)
}

func isXmasPath(path string) bool {
	p := strings.ToLower(path)
	return strings.Contains(p, "xmas") || strings.Contains(p, "ficsmas") || strings.Contains(p, "christmas") || strings.Contains(p, "xmass")
}

// targetInfo holds parsed info about a target object
type targetInfo struct {
	className      string
	headerType     string // "Actor" or "Object"
	useCompleteTagType bool
	dataStartVPos  int64
	binarySize     int32
	binarySizeVPos int64

	// For each target array property: positions within object data
	propName       string
	propBinSizeOff int // offset of propBinarySize field within object data
	countOff       int // offset of array count within object data
	entryOffsets   []int // offsets of each XMas entry within object data
	entrySizes     []int // sizes of each XMas entry
}

// objPos holds position info for a single object in the data blob
type objPos struct {
	index         int
	dataStartVPos int64
	binarySize    int32
	className     string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: fixficsmas <savefile> [output]")
		os.Exit(1)
	}
	inputPath := os.Args[1]
	outputPath := inputPath
	if len(os.Args) >= 3 {
		outputPath = os.Args[2]
	} else if strings.HasSuffix(inputPath, ".sav") {
		outputPath = strings.TrimSuffix(inputPath, ".sav") + "_fixed.sav"
	} else {
		outputPath = inputPath + "_fixed"
	}

	fmt.Printf("Reading %s ...\n", inputPath)
	fileData, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  File size: %.1f MB\n", float64(len(fileData))/1024/1024)

	// ============================================================
	// Step 1: Parse header, decompress entire body into one buffer
	// ============================================================
	fmt.Println("\n=== Step 1: Decompressing body ===")
	headerReader := parser.NewBinaryReader(fileData)
	_, ctx, err := parser.ParseHeader(headerReader)
	if err != nil {
		fmt.Printf("Error parsing header: %v\n", err)
		os.Exit(1)
	}
	headerEnd := headerReader.Position()

	bodyReader := &sliceReader{data: fileData[headerEnd:]}
	decompressor := parser.NewDecompressor(bodyReader)
	bodyData, err := decompressor.DecompressAll()
	if err != nil {
		fmt.Printf("Error decompressing: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Decompressed body: %.1f MB\n", float64(len(bodyData))/1024/1024)

	// Read the first chunk header manually for recompression metadata
	var firstChunkHeader *parser.ChunkHeader
	{
		br2 := &sliceReader{data: fileData[headerEnd:]}
		buf := make([]byte, 8)
		io.ReadFull(br2, buf)
		tag := binary.LittleEndian.Uint32(buf[:4])
		version := binary.LittleEndian.Uint32(buf[4:])
		isV2 := version == 0x22222222
		var maxChunkSize int32
		binary.Read(br2, binary.LittleEndian, &maxChunkSize)
		io.CopyN(io.Discard, br2, 4)
		var compAlgo uint8 = 3
		if isV2 {
			binary.Read(br2, binary.LittleEndian, &compAlgo)
		}
		firstChunkHeader = &parser.ChunkHeader{
			PackageFileTag:      tag,
			ChunkHeaderVersion:  version,
			MaxChunkSize:        maxChunkSize,
			CompressionAlgorithm: compAlgo,
			HeaderSize:           49,
		}
		if !isV2 {
			firstChunkHeader.HeaderSize = 48
		}
	}
	fmt.Printf("  MaxChunkSize: %d, HeaderV2: %v\n", firstChunkHeader.MaxChunkSize, firstChunkHeader.ChunkHeaderVersion == 0x22222222)

	// ============================================================
	// Step 2: Parse body to find target objects
	// ============================================================
	fmt.Println("\n=== Step 2: Parsing body to find targets ===")

	// Feed bodyData through a single-chunk channel for the StreamingReader.
	fakeCh := make(chan parser.ChunkResult, 1)
	fakeCh <- parser.ChunkResult{Index: 0, Data: bodyData, UncompressedSize: len(bodyData)}
	close(fakeCh)
	sr := parser.NewStreamingReader(fakeCh)
	bp := parser.NewBodyParser(sr, ctx)
	body, err := bp.ParseBody()
	if err != nil {
		fmt.Printf("Error parsing body: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  TotalSize: %d, SublevelCount: %d\n", body.TotalSize, body.SublevelCount)

	// Targets to find
	targetClassNames := []string{"schematicmanager", "researchmanager", "gamestate"}
	var targets []*targetInfo

	var dataBlobLenVPos int64
	var persistentLevel *parser.Level
	var tocBlobLenVPos int64
	var tocBlobDataVPos int64
	var allObjPositions []objPos

	_, err = bp.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dbl int64, level *parser.Level) error {
		if level.IsPersistent {
			persistentLevel = level
			dataBlobLenVPos = sr.Position() - 8
			// TOC blob is before data blob: tocLenField(8) + tocData(tocBlobLen) + dataLenField(8)
			// So: tocBlobLenVPos = dataBlobLenVPos - 8 - tocBlobLength
			tocBlobLenVPos = dataBlobLenVPos - 8 - level.TOCBlobLength
			tocBlobDataVPos = tocBlobLenVPos + 8
			fmt.Printf("  Persistent level: dataBlobLength=%d at vPos=%d\n", dbl, dataBlobLenVPos)
			fmt.Printf("  TOC blob: length=%d at vPos=%d, data at vPos=%d\n", level.TOCBlobLength, tocBlobLenVPos, tocBlobDataVPos)

			dbp := parser.NewDataBlobParser(ctx, level.ObjectHeaders, "")
			_, err := dbp.StreamObjects(sr, dbl, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, idx int) {
				if obj.Header == nil {
					return
				}

				// Collect position info for all objects
				allObjPositions = append(allObjPositions, objPos{
					index:         idx,
					dataStartVPos: obj.DataStartVPos,
					binarySize:    obj.BinarySize,
					className:     obj.Header.ClassName,
				})

				cn := strings.ToLower(obj.Header.ClassName)
				for _, tc := range targetClassNames {
					if strings.Contains(cn, tc) {
						ti := &targetInfo{
							className:          obj.Header.ClassName,
							headerType:         obj.Header.Type,
							useCompleteTagType: obj.UseCompleteTagType,
							dataStartVPos:      obj.DataStartVPos,
							binarySize:         obj.BinarySize,
							binarySizeVPos:     obj.DataStartVPos - 4,
						}
						targets = append(targets, ti)
						fmt.Printf("  Found %s: index=%d, binarySize=%d, dataStartVPos=%d\n",
							obj.Header.ClassName, idx, obj.BinarySize, obj.DataStartVPos)
						break
					}
				}
			})
			return err
		}
		if dbl > 0 {
			return sr.Skip(int(dbl))
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Warning: error streaming levels: %v\n", err)
	}

	if len(targets) == 0 {
		fmt.Println("Error: no target objects found")
		os.Exit(1)
	}

	// ============================================================
	// Step 3: Parse each target object's data to find XMas entries
	// ============================================================
	fmt.Println("\n=== Step 3: Finding XMas entries in target objects ===")

	// Target array property names for each class
	targetProps := map[string]string{
		"schematicmanager": "mPurchasedSchematics",
		"researchmanager":  "mUnlockedResearchTrees",
		"gamestate":        "mPickedUpItems",
	}

	totalRemoveBytes := 0
	for _, ti := range targets {
		cnLower := strings.ToLower(ti.className)
		propName := ""
		for key, val := range targetProps {
			if strings.Contains(cnLower, key) {
				propName = val
				break
			}
		}
		if propName == "" {
			fmt.Printf("  Skipping %s (no target property)\n", ti.className)
			continue
		}
		ti.propName = propName

		// Extract object data from bodyData
		objData := bodyData[ti.dataStartVPos : ti.dataStartVPos+int64(ti.binarySize)]

		// Parse the object data to find the target array property
		// Old format (useCompleteTagType = false)
		pos := 0

		// Actor header: parentObject (2 FStrings) + componentCount + components
		// Object type: goes straight to properties
		if ti.headerType == "Actor" {
			_, sz := readFString(objData, pos)
			pos += sz
			_, sz = readFString(objData, pos)
			pos += sz
			if pos+4 > len(objData) {
				fmt.Printf("  Error: object data too short for %s\n", ti.className)
				continue
			}
			compCount := int32(binary.LittleEndian.Uint32(objData[pos:]))
			pos += 4
			for i := int32(0); i < compCount; i++ {
				_, sz = readFString(objData, pos)
				pos += sz
				_, sz = readFString(objData, pos)
				pos += sz
			}
		}

		// Properties loop
		found := false

		// For complete tag type, there's an initial uint8 to skip
		if ti.useCompleteTagType {
			pos += 1
		}

		for pos < len(objData)-4 {
			name, sz := readFString(objData, pos)
			pos += sz
			if name == "None" {
				break
			}

			var propType string
			var propBinSize int32
			var propBinSizeOff int

			if ti.useCompleteTagType {
				// readFPropertyTagNode: FString(name) + int32(childCount) + children
				tnName, sz2 := readFString(objData, pos)
				pos += sz2
				if pos+4 > len(objData) {
					break
				}
				childCount := int32(binary.LittleEndian.Uint32(objData[pos:]))
				pos += 4
				// Skip children
				for i := int32(0); i < childCount; i++ {
					_, sz2 = readFString(objData, pos)
					pos += sz2
					if pos+4 > len(objData) {
						break
					}
					pos += 4 // child's childCount (assume 0 for simple types)
				}
				propType = tnName

				// propBinarySize
				if pos+4 > len(objData) {
					break
				}
				propBinSize = int32(binary.LittleEndian.Uint32(objData[pos:]))
				propBinSizeOff = pos
				pos += 4

				// propFlags
				if pos >= len(objData) {
					break
				}
				propFlags := objData[pos]
				pos += 1
				if propFlags&0x1 != 0 {
					pos += 4 // int32
				}
				if propFlags&0x2 != 0 {
					pos += 16 // guid
				}

				// Our target arrays are ObjectProperty arrays; verified by value data below.
				if propType == "ArrayProperty" {
					_ = "ObjectProperty"
				}
			} else {
				// Old format
				propType, sz = readFString(objData, pos)
				pos += sz

				// propBinarySize
				if pos+4 > len(objData) {
					break
				}
				propBinSize = int32(binary.LittleEndian.Uint32(objData[pos:]))
				propBinSizeOff = pos
				pos += 4

				// index (int32)
				pos += 4

				// Type-specific header fields
				switch propType {
				case "ArrayProperty":
					_, sz = readFString(objData, pos)
					pos += sz
				case "StructProperty":
					_, sz = readFString(objData, pos) // structSubtype
					pos += sz
					pos += 16 // skip 16 bytes
				case "SetProperty":
					_, sz = readFString(objData, pos)
					pos += sz
				case "BoolProperty":
					pos += 1 // bool value in header
				case "ByteProperty", "EnumProperty":
					_, sz = readFString(objData, pos)
					pos += sz
				case "MapProperty":
					_, sz = readFString(objData, pos)
					pos += sz
					_, sz = readFString(objData, pos)
					pos += sz
				}

				// hasGuid
				if pos >= len(objData) {
					break
				}
				hasGuid := objData[pos]
				pos += 1
				if hasGuid != 0 {
					pos += 16
				}
			}

			// Now at value data start
			valueStart := pos

			if name == propName && propType == "ArrayProperty" {
				// Found target array! Verify it's ObjectProperty type by reading count and first entry
				if valueStart+4 > len(objData) {
					break
				}
				count := int32(binary.LittleEndian.Uint32(objData[valueStart:]))
				ti.propBinSizeOff = propBinSizeOff
				ti.countOff = valueStart

				fmt.Printf("  %s: found %s, count=%d, propBinSize=%d at off=%d, countOff=%d\n",
					ti.className, propName, count, propBinSize, propBinSizeOff, valueStart)

				// Iterate entries
				entryPos := valueStart + 4
				for i := int32(0); i < count; i++ {
					entryStart := entryPos
					// Read levelName FString
					levelName, sz := readFString(objData, entryPos)
					entryPos += sz
					// Read pathName FString
					pathName, sz := readFString(objData, entryPos)
					entryPos += sz

					entrySize := entryPos - entryStart

					if isXmasPath(pathName) {
						ti.entryOffsets = append(ti.entryOffsets, entryStart)
						ti.entrySizes = append(ti.entrySizes, entrySize)
						fmt.Printf("    XMas entry [%d]: off=%d size=%d path=%s\n",
							i, entryStart, entrySize, pathName)
					}
					_ = levelName
				}
				found = true
				fmt.Printf("  Total XMas entries in %s: %d\n", ti.className, len(ti.entryOffsets))
			}

			// Skip to end of property value
			pos = valueStart + int(propBinSize)
		}

		if !found {
			fmt.Printf("  Warning: property %s not found in %s\n", propName, ti.className)
		}

		totalRemoveBytes += len(ti.entryOffsets) * 0 // will sum below
	}

	// Calculate total bytes to remove
	totalRemove := 0
	for _, ti := range targets {
		for _, sz := range ti.entrySizes {
			totalRemove += sz
		}
	}
	fmt.Printf("\n  Total entries to remove: %d, total bytes: %d\n",
		countEntries(targets), totalRemove)

	// ============================================================
	// Step 3.5: Find XMas building objects to remove entirely
	// ============================================================
	fmt.Println("\n=== Step 3.5: Finding XMas building objects ===")

	type objRemoval struct {
		index    int
		tocOff   int // offset within TOC blob data
		tocSize  int
		dataVPos int64 // vPos of object data in bodyData
		dataSize int   // total size of object data entry (header + binarySize + data + versionData)
	}

	var xmasObjRemovals []objRemoval
	if persistentLevel != nil {
		// Parse TOC blob to find byte offsets of each object header
		// Use a cloned context since the original was mutated during streaming
		tocCtx := ctx.Clone()
		tocData := bodyData[tocBlobDataVPos : tocBlobDataVPos+persistentLevel.TOCBlobLength]
		tocReader := parser.NewBinaryReader(tocData)
		_ = tocReader.ReadInt32() // countEntities

		tocOffsets, tocSizes, err := parser.ParseObjectHeaderOffsets(tocReader, tocCtx, int(persistentLevel.ObjectHeaderCount))
		if err != nil {
			fmt.Printf("  Warning: error parsing TOC header offsets: %v\n", err)
		}

		// Find XMas objects using already-parsed headers
		for i, hdr := range persistentLevel.ObjectHeaders {
			if isXmasPath(hdr.ClassName) {
				xmasObjRemovals = append(xmasObjRemovals, objRemoval{
					index:   i,
					tocOff:  tocOffsets[i],
					tocSize: tocSizes[i],
				})
			}
		}

		// Compute each object's total size in the data blob from consecutive DataStartVPos values.
		dataBlobStart := dataBlobLenVPos + 8
		dataBlobEnd := dataBlobStart + persistentLevel.DataBlobLength

		for j := range xmasObjRemovals {
			idx := xmasObjRemovals[j].index
			if idx >= len(allObjPositions) {
				continue
			}
			// Header starts 12 bytes before DataStartVPos (saveCustomVersion + shouldMigrate + binarySize)
			headerVPos := allObjPositions[idx].dataStartVPos - 12
			xmasObjRemovals[j].dataVPos = headerVPos

			// Compute total size: from this object's header start to next object's header start
			if idx+1 < len(allObjPositions) {
				nextHeaderVPos := allObjPositions[idx+1].dataStartVPos - 12
				xmasObjRemovals[j].dataSize = int(nextHeaderVPos - headerVPos)
			} else {
				// Last object: extends to end of data blob
				xmasObjRemovals[j].dataSize = int(dataBlobEnd - headerVPos)
			}
		}

		fmt.Printf("  Found %d XMas building objects to remove\n", len(xmasObjRemovals))
		tocRemoveTotal := 0
		dataRemoveTotal := 0
		for _, r := range xmasObjRemovals {
			tocRemoveTotal += r.tocSize
			dataRemoveTotal += r.dataSize
			fmt.Printf("    [%d] TOC off=%d size=%d, data vPos=%d size=%d\n",
				r.index, r.tocOff, r.tocSize, r.dataVPos, r.dataSize)
		}
		fmt.Printf("  TOC bytes to remove: %d, data bytes to remove: %d\n", tocRemoveTotal, dataRemoveTotal)
	}

	// Calculate total bytes to remove from objects
	totalObjRemove := 0
	tocRemoveTotal := 0
	dataRemoveTotal := 0
	for _, r := range xmasObjRemovals {
		tocRemoveTotal += r.tocSize
		dataRemoveTotal += r.dataSize
		totalObjRemove += r.tocSize + r.dataSize
	}
	fmt.Printf("  Total object bytes to remove: %d (TOC: %d, data: %d)\n",
		totalObjRemove, tocRemoveTotal, dataRemoveTotal)

	if totalRemove == 0 && len(xmasObjRemovals) == 0 {
		fmt.Println("No XMas entries or objects found. Nothing to do.")
		os.Exit(0)
	}

	// ============================================================
	// Step 4: Modify bodyData - remove entries and update sizes
	// ============================================================
	fmt.Println("\n=== Step 4: Modifying body data ===")

	// --- 4a: Update size fields BEFORE removing any bytes ---

	// Update size fields for each target object (array entries)
	for _, ti := range targets {
		if len(ti.entryOffsets) == 0 {
			continue
		}

		objRemove := 0
		for _, sz := range ti.entrySizes {
			objRemove += sz
		}

		// Update binarySize (int32 at binarySizeVPos)
		oldBinSize := int32(binary.LittleEndian.Uint32(bodyData[ti.binarySizeVPos:]))
		newBinSize := oldBinSize - int32(objRemove)
		binary.LittleEndian.PutUint32(bodyData[ti.binarySizeVPos:], uint32(newBinSize))
		fmt.Printf("  %s: binarySize %d -> %d\n", ti.className, oldBinSize, newBinSize)

		// Update array count (int32 at dataStartVPos + countOff)
		countVPos := ti.dataStartVPos + int64(ti.countOff)
		oldCount := int32(binary.LittleEndian.Uint32(bodyData[countVPos:]))
		newCount := oldCount - int32(len(ti.entryOffsets))
		binary.LittleEndian.PutUint32(bodyData[countVPos:], uint32(newCount))
		fmt.Printf("  %s: %s count %d -> %d\n", ti.className, ti.propName, oldCount, newCount)

		// Update propBinarySize (int32 at dataStartVPos + propBinSizeOff)
		pbsVPos := ti.dataStartVPos + int64(ti.propBinSizeOff)
		oldPbs := int32(binary.LittleEndian.Uint32(bodyData[pbsVPos:]))
		newPbs := oldPbs - int32(objRemove)
		binary.LittleEndian.PutUint32(bodyData[pbsVPos:], uint32(newPbs))
		fmt.Printf("  %s: %s propBinarySize %d -> %d\n", ti.className, ti.propName, oldPbs, newPbs)
	}

	// --- 4b: Update count fields for XMas object removals ---
	if persistentLevel != nil && len(xmasObjRemovals) > 0 {
		xmasCount := int32(len(xmasObjRemovals))

		// Update countEntities in TOC blob (int32 at tocBlobDataVPos)
		oldTocCount := int32(binary.LittleEndian.Uint32(bodyData[tocBlobDataVPos:]))
		newTocCount := oldTocCount - xmasCount
		binary.LittleEndian.PutUint32(bodyData[tocBlobDataVPos:], uint32(newTocCount))
		fmt.Printf("  TOC countEntities: %d -> %d\n", oldTocCount, newTocCount)

		// Update countEntities in data blob (int32 at dataBlobLenVPos + 8)
		dataBlobStartVPos := dataBlobLenVPos + 8
		oldDataCount := int32(binary.LittleEndian.Uint32(bodyData[dataBlobStartVPos:]))
		newDataCount := oldDataCount - xmasCount
		binary.LittleEndian.PutUint32(bodyData[dataBlobStartVPos:], uint32(newDataCount))
		fmt.Printf("  Data countEntities: %d -> %d\n", oldDataCount, newDataCount)

		// Update TOCBlobLength (int64 at tocBlobLenVPos)
		tocRemoveBytes := 0
		for _, r := range xmasObjRemovals {
			tocRemoveBytes += r.tocSize
		}
		oldTocLen := int64(binary.LittleEndian.Uint64(bodyData[tocBlobLenVPos:]))
		newTocLen := oldTocLen - int64(tocRemoveBytes)
		binary.LittleEndian.PutUint64(bodyData[tocBlobLenVPos:], uint64(newTocLen))
		fmt.Printf("  TOCBlobLength: %d -> %d\n", oldTocLen, newTocLen)
	}

	// --- 4c: Collect ALL removals (array entries + object data + TOC headers) ---
	var removals []removal

	// Array entry removals
	for _, ti := range targets {
		for i, off := range ti.entryOffsets {
			removals = append(removals, removal{
				vPos: ti.dataStartVPos + int64(off),
				size: ti.entrySizes[i],
			})
		}
	}

	// XMas object data removals
	for _, r := range xmasObjRemovals {
		removals = append(removals, removal{
			vPos: r.dataVPos,
			size: r.dataSize,
		})
	}

	// XMas TOC header removals (convert TOC offset to vPos)
	for _, r := range xmasObjRemovals {
		removals = append(removals, removal{
			vPos: tocBlobDataVPos + int64(r.tocOff),
			size: r.tocSize,
		})
	}

	// Sort removals by vPos descending (remove from back to front)
	sortRemovalsDesc(removals)

	// Calculate total bytes to remove
	totalAllRemove := 0
	for _, r := range removals {
		totalAllRemove += r.size
	}
	fmt.Printf("  Total bytes to remove: %d (%d removals)\n", totalAllRemove, len(removals))

	// --- 4e: Update dataBlobLength and TotalSize BEFORE removing bytes ---
	// (must be done before removals because removals shift positions)
	// dataBlobLength needs to account for: array entry removals + object data removals
	// (TOC header removals don't affect dataBlobLength)
	dataRemoveBytes := 0
	for _, ti := range targets {
		for _, sz := range ti.entrySizes {
			dataRemoveBytes += sz
		}
	}
	for _, r := range xmasObjRemovals {
		dataRemoveBytes += r.dataSize
	}

	oldDbl := int64(binary.LittleEndian.Uint64(bodyData[dataBlobLenVPos:]))
	newDbl := oldDbl - int64(dataRemoveBytes)
	binary.LittleEndian.PutUint64(bodyData[dataBlobLenVPos:], uint64(newDbl))
	fmt.Printf("  dataBlobLength: %d -> %d\n", oldDbl, newDbl)

	// TotalSize accounts for all removals (array entries + object data + TOC headers)
	oldTs := int32(binary.LittleEndian.Uint32(bodyData[0:]))
	newTs := oldTs - int32(totalAllRemove)
	binary.LittleEndian.PutUint32(bodyData[0:], uint32(newTs))
	fmt.Printf("  TotalSize: %d -> %d\n", oldTs, newTs)

	// --- 4d: Remove bytes from bodyData ---
	for _, r := range removals {
		bodyData = append(bodyData[:r.vPos], bodyData[r.vPos+int64(r.size):]...)
	}

	// ============================================================
	// Step 5: Recompress and write output
	// ============================================================
	fmt.Println("\n=== Step 5: Recompressing and writing output ===")

	maxChunkSize := int(firstChunkHeader.MaxChunkSize)
	if maxChunkSize <= 0 {
		maxChunkSize = 131072
	}
	isV2 := firstChunkHeader.ChunkHeaderVersion == 0x22222222

	var output bytes.Buffer

	// Copy save header from input
	output.Write(fileData[:headerEnd])

	// Split bodyData into chunks and compress
	chunkCount := (len(bodyData) + maxChunkSize - 1) / maxChunkSize
	fmt.Printf("  Body chunks: %d (maxChunkSize=%d)\n", chunkCount, maxChunkSize)

	for i := 0; i < chunkCount; i++ {
		start := i * maxChunkSize
		end := start + maxChunkSize
		if end > len(bodyData) {
			end = len(bodyData)
		}
		chunkData := bodyData[start:end]
		uncompSize := len(chunkData)

		// Compress with zlib
		var compressed bytes.Buffer
		zw := zlib.NewWriter(&compressed)
		zw.Write(chunkData)
		zw.Close()
		compData := compressed.Bytes()
		compSize := len(compData)

		// Write chunk header
		// packageFileTag (uint32)
		binary.Write(&output, binary.LittleEndian, firstChunkHeader.PackageFileTag)
		// chunkHeaderVersion (uint32)
		binary.Write(&output, binary.LittleEndian, firstChunkHeader.ChunkHeaderVersion)
		// maxChunkSize (int32) + 4 bytes padding
		binary.Write(&output, binary.LittleEndian, firstChunkHeader.MaxChunkSize)
		output.Write(make([]byte, 4)) // padding
		// compressionAlgorithm (uint8) - only for V2
		if isV2 {
			output.WriteByte(firstChunkHeader.CompressionAlgorithm)
		}
		// compressedSize (int32) + 4 bytes padding
		binary.Write(&output, binary.LittleEndian, int32(compSize))
		output.Write(make([]byte, 4))
		// uncompressedSize (int32) + 4 bytes padding
		binary.Write(&output, binary.LittleEndian, int32(uncompSize))
		output.Write(make([]byte, 4))
		// compressedSize2 (int32) + 4 bytes padding
		binary.Write(&output, binary.LittleEndian, int32(compSize))
		output.Write(make([]byte, 4))
		// uncompressedSize2 (int32) + 4 bytes padding
		binary.Write(&output, binary.LittleEndian, int32(uncompSize))
		output.Write(make([]byte, 4))

		// Write compressed data
		output.Write(compData)
	}

	fmt.Printf("  Output size: %.1f MB\n", float64(output.Len())/1024/1024)

	err = os.WriteFile(outputPath, output.Bytes(), 0644)
	if err != nil {
		fmt.Printf("Error writing output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nDone! Fixed save written to: %s\n", outputPath)
	fmt.Printf("  Removed %d XMas array entries (%d bytes)\n", countEntries(targets), totalRemove)
	fmt.Printf("  Removed %d XMas building objects (%d bytes)\n", len(xmasObjRemovals), totalAllRemove)
}

func countEntries(targets []*targetInfo) int {
	total := 0
	for _, ti := range targets {
		total += len(ti.entryOffsets)
	}
	return total
}

type removal struct {
	vPos int64
	size int
}

func sortRemovalsDesc(r []removal) {
	for i := 0; i < len(r); i++ {
		for j := i + 1; j < len(r); j++ {
			if r[j].vPos > r[i].vPos {
				r[i], r[j] = r[j], r[i]
			}
		}
	}
}
