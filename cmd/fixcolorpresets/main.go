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
		if pos+4+byteCount > len(data) {
			return "", 4 + byteCount
		}
		raw := data[pos+4 : pos+4+byteCount]
		end := byteCount
		for i := 0; i < charCount; i++ {
			if raw[i*2] == 0 && raw[i*2+1] == 0 {
				end = i * 2
				break
			}
		}
		return string(raw[:end]), 4 + byteCount
	}
	if pos+4+int(length) > len(data) {
		return "", 4 + int(length)
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

// skipTagNode reads a tag node (name + childCount + children) and returns the name and total bytes consumed
func skipTagNode(data []byte, pos int) (string, int) {
	origPos := pos
	name, sz := readFString(data, pos)
	pos += sz
	if pos+4 > len(data) {
		return name, pos - origPos
	}
	childCount := int32(binary.LittleEndian.Uint32(data[pos:]))
	pos += 4
	for i := int32(0); i < childCount; i++ {
		_, csz := skipTagNode(data, pos)
		pos += csz
	}
	return name, pos - origPos
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: fixcolorpresets <savefile> [outputfile]")
		os.Exit(1)
	}
	inputPath := os.Args[1]
	outputPath := ""
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

	// Step 1: Parse header, decompress entire body
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

	// Read chunk header for recompression
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
			PackageFileTag:       tag,
			ChunkHeaderVersion:   version,
			MaxChunkSize:         maxChunkSize,
			CompressionAlgorithm: compAlgo,
			HeaderSize:           49,
		}
		if !isV2 {
			firstChunkHeader.HeaderSize = 48
		}
	}

	// Step 2: Parse body to find GameState
	fmt.Println("\n=== Step 2: Finding GameState ===")

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

	var dataBlobLenVPos int64
	var gameStateDataVPos int64
	var gameStateBinarySize int32
	var gameStateBinarySizeVPos int64
	var gameStateHeaderType string
	var gameStateUseCompleteTagType bool

	_, err = bp.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dbl int64, level *parser.Level) error {
		if level.IsPersistent {
			dataBlobLenVPos = sr.Position() - 8

			dbp := parser.NewDataBlobParser(ctx, level.ObjectHeaders, "")
			_, err := dbp.StreamObjects(sr, dbl, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, idx int) {
				if obj.Header == nil {
					return
				}
				cn := strings.ToLower(obj.Header.ClassName)
				if strings.Contains(cn, "gamestate") {
					gameStateDataVPos = obj.DataStartVPos
					gameStateBinarySize = obj.BinarySize
					gameStateBinarySizeVPos = obj.DataStartVPos - 4
					gameStateHeaderType = obj.Header.Type
					gameStateUseCompleteTagType = obj.UseCompleteTagType
					fmt.Printf("  Found GameState: index=%d, binarySize=%d, dataStartVPos=%d, headerType=%s, completeTagType=%v\n",
						idx, obj.BinarySize, obj.DataStartVPos, obj.Header.Type, obj.UseCompleteTagType)
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
		fmt.Printf("Error streaming levels: %v\n", err)
	}

	if gameStateDataVPos == 0 {
		fmt.Println("Error: GameState not found")
		os.Exit(1)
	}

	// Step 3: Parse GameState properties to find mPlayerGlobalColorPresets
	fmt.Println("\n=== Step 3: Finding mPlayerGlobalColorPresets ===")

	objData := bodyData[gameStateDataVPos : gameStateDataVPos+int64(gameStateBinarySize)]
	pos := 0

	// Actor header: parentObject + components
	if gameStateHeaderType == "Actor" {
		_, sz := readFString(objData, pos)
		pos += sz
		_, sz = readFString(objData, pos)
		pos += sz
		if pos+4 > len(objData) {
			fmt.Println("Error: object data too short")
			os.Exit(1)
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

	// For complete tag type, skip initial uint8
	if gameStateUseCompleteTagType {
		pos += 1
	}

	propStartPos := pos // position where properties start in objData
	_ = propStartPos

	var propToRemoveStart int // start of property name FString in objData
	var propToRemoveEnd int   // end of property value in objData
	found := false

	for pos < len(objData)-4 {
		propNameStart := pos
		name, sz := readFString(objData, pos)
		pos += sz
		if name == "None" {
			break
		}

		var propType string
		var propBinSize int32

		if gameStateUseCompleteTagType {
			// readFPropertyTagNode (recursive)
			tnName, totalSz := skipTagNode(objData, pos)
			pos += totalSz
			propType = tnName

			if pos+4 > len(objData) {
				break
			}
			propBinSize = int32(binary.LittleEndian.Uint32(objData[pos:]))
			pos += 4

			if pos >= len(objData) {
				break
			}
			propFlags := objData[pos]
			pos += 1
			if propFlags&0x1 != 0 {
				pos += 4
			}
			if propFlags&0x2 != 0 {
				pos += 16
			}
		} else {
			// Old format
			propType, sz = readFString(objData, pos)
			pos += sz

			if pos+4 > len(objData) {
				break
			}
			propBinSize = int32(binary.LittleEndian.Uint32(objData[pos:]))
			pos += 4
			pos += 4 // index

			switch propType {
			case "ArrayProperty":
				_, sz = readFString(objData, pos)
				pos += sz
			case "StructProperty":
				_, sz = readFString(objData, pos)
				pos += sz
				pos += 16
			case "SetProperty":
				_, sz = readFString(objData, pos)
				pos += sz
			case "BoolProperty":
				pos += 1
			case "ByteProperty", "EnumProperty":
				_, sz = readFString(objData, pos)
				pos += sz
			case "MapProperty":
				_, sz = readFString(objData, pos)
				pos += sz
				_, sz = readFString(objData, pos)
				pos += sz
			}

			if pos >= len(objData) {
				break
			}
			hasGuid := objData[pos]
			pos += 1
			if hasGuid != 0 {
				pos += 16
			}
		}

		// Now at value data
		valueStart := pos
		valueEnd := valueStart + int(propBinSize)

		if name == "mPlayerGlobalColorPresets" {
			// Read count
			if valueStart+4 <= len(objData) {
				count := int32(binary.LittleEndian.Uint32(objData[valueStart:]))
				fmt.Printf("  Found mPlayerGlobalColorPresets: count=%d, propBinSize=%d\n", count, propBinSize)
				fmt.Printf("  Property byte range in objData: [%d, %d) = %d bytes\n",
					propNameStart, valueEnd, valueEnd-propNameStart)
				fmt.Printf("  Absolute vPos range: [%d, %d)\n",
					gameStateDataVPos+int64(propNameStart), gameStateDataVPos+int64(valueEnd))
			}
			propToRemoveStart = propNameStart
			propToRemoveEnd = valueEnd
			found = true
		}

		pos = valueEnd
	}

	if !found {
		fmt.Println("  mPlayerGlobalColorPresets not found in GameState!")
		fmt.Println("  Nothing to fix. Exiting.")
		os.Exit(0)
	}

	// Step 4: Remove the property bytes from bodyData
	fmt.Println("\n=== Step 4: Removing property ===")

	removeStart := gameStateDataVPos + int64(propToRemoveStart)
	removeEnd := gameStateDataVPos + int64(propToRemoveEnd)
	removeSize := removeEnd - removeStart
	fmt.Printf("  Removing %d bytes from bodyData at vPos [%d, %d)\n", removeSize, removeStart, removeEnd)

	// Calculate new binarySize for GameState
	newBinarySize := gameStateBinarySize - int32(removeSize)
	fmt.Printf("  GameState binarySize: %d -> %d\n", gameStateBinarySize, newBinarySize)

	// Update binarySize field (4 bytes before dataStart)
	binary.LittleEndian.PutUint32(bodyData[gameStateBinarySizeVPos:], uint32(newBinarySize))

	// Update dataBlobLength
	oldDbl := int64(binary.LittleEndian.Uint64(bodyData[dataBlobLenVPos:]))
	newDbl := oldDbl - removeSize
	binary.LittleEndian.PutUint64(bodyData[dataBlobLenVPos:], uint64(newDbl))
	fmt.Printf("  dataBlobLength: %d -> %d\n", oldDbl, newDbl)

	// Update TotalSize
	oldTs := int32(binary.LittleEndian.Uint32(bodyData[0:]))
	newTs := oldTs - int32(removeSize)
	binary.LittleEndian.PutUint32(bodyData[0:], uint32(newTs))
	fmt.Printf("  TotalSize: %d -> %d\n", oldTs, newTs)

	// Now remove the bytes
	bodyData = append(bodyData[:removeStart], bodyData[removeEnd:]...)

	fmt.Printf("  New bodyData size: %d (was %d)\n", len(bodyData), len(bodyData)+int(removeSize))

	// Step 5: Recompress and write output
	fmt.Println("\n=== Step 5: Recompressing and writing output ===")

	maxChunkSize := int(firstChunkHeader.MaxChunkSize)
	if maxChunkSize <= 0 {
		maxChunkSize = 131072
	}
	isV2 := firstChunkHeader.ChunkHeaderVersion == 0x22222222

	var output bytes.Buffer
	output.Write(fileData[:headerEnd])

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

		var compressed bytes.Buffer
		zw := zlib.NewWriter(&compressed)
		zw.Write(chunkData)
		zw.Close()
		compData := compressed.Bytes()
		compSize := len(compData)

		binary.Write(&output, binary.LittleEndian, firstChunkHeader.PackageFileTag)
		binary.Write(&output, binary.LittleEndian, firstChunkHeader.ChunkHeaderVersion)
		binary.Write(&output, binary.LittleEndian, firstChunkHeader.MaxChunkSize)
		output.Write(make([]byte, 4))
		if isV2 {
			output.WriteByte(firstChunkHeader.CompressionAlgorithm)
		}
		binary.Write(&output, binary.LittleEndian, int32(compSize))
		output.Write(make([]byte, 4))
		binary.Write(&output, binary.LittleEndian, int32(uncompSize))
		output.Write(make([]byte, 4))
		binary.Write(&output, binary.LittleEndian, int32(compSize))
		output.Write(make([]byte, 4))
		binary.Write(&output, binary.LittleEndian, int32(uncompSize))
		output.Write(make([]byte, 4))
		output.Write(compData)
	}

	fmt.Printf("  Output size: %.1f MB\n", float64(output.Len())/1024/1024)

	err = os.WriteFile(outputPath, output.Bytes(), 0644)
	if err != nil {
		fmt.Printf("Error writing output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nDone! Fixed save written to: %s\n", outputPath)
}
