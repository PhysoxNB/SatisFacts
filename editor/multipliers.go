package editor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"satisfacts/parser"
)

// FindFloatPropOffset scans an object's property list to find a named FloatProperty.
// Returns the byte offset of the float value within the object data, and the current value.
func FindFloatPropOffset(data []byte, propName string, isActor bool, useCompleteTag bool) (int, float32) {
	var foundOffset int = -1
	var foundVal float32

	WalkObjectProperties(data, isActor, useCompleteTag, func(it PropIteration) bool {
		if it.Name == propName && it.Header.PropType == "FloatProperty" {
			r := parser.NewBinaryReader(data[it.ValuePos:])
			foundVal = r.ReadFloat32()
			foundOffset = it.ValuePos
			return false
		}
		return true
	})

	return foundOffset, foundVal
}

// GameStateInfo holds the location and metadata of the BP_GameState object.
type GameStateInfo struct {
	VPos           int64
	Size           int32
	IsActor        bool
	UseCompleteTag bool
	ClassName      string
}

// FindGameState streams the persistent level to find the GameState object.
// It uses the parser's streaming API to locate the object.
func FindGameState(sd *SaveData) (*GameStateInfo, error) {
	chunkCh := make(chan parser.ChunkResult, 1)
	chunkCh <- parser.ChunkResult{Index: 0, Data: sd.Decompressed, UncompressedSize: len(sd.Decompressed)}
	close(chunkCh)

	sr := parser.NewStreamingReader(chunkCh)
	bp := parser.NewBodyParser(sr, sd.VersionCtx)
	body, err := bp.ParseBody()
	if err != nil {
		return nil, err
	}

	var gsi *GameStateInfo

	_, err = bp.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dbl int64, level *parser.Level) error {
		if !level.IsPersistent || dbl <= 0 {
			return sr.Skip(int(dbl))
		}
		dbp := parser.NewDataBlobParser(sd.VersionCtx, level.ObjectHeaders, "")
		_, err := dbp.StreamObjects(sr, dbl, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, idx int) {
			if obj.Header == nil {
				return
			}
			cl := strings.ToLower(obj.Header.ClassName)
			if strings.Contains(cl, "gamestate") {
				gsi = &GameStateInfo{
					VPos:           obj.DataStartVPos,
					Size:           obj.BinarySize,
					IsActor:        obj.Header.Type == "Actor",
					UseCompleteTag: obj.UseCompleteTagType,
					ClassName:      obj.Header.ClassName,
				}
			}
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	if gsi == nil {
		return nil, fmt.Errorf("GameState object not found")
	}
	return gsi, nil
}

// GameStateFullInfo holds the location and byte positions of the BP_GameState object
// within the decompressed body, for use with insertion operations that need to update
// size fields.
type GameStateFullInfo struct {
	IsActor         bool
	UseCompleteTag  bool
	ClassName       string
	BinaryDataStart int // absolute position of binary data in decompressed body
	BinaryDataSize  int // size of binary data
	BinarySizePos   int // absolute position of the binarySize field in decompressed body
	DataLenPos      int // absolute position of the data blob length field in decompressed body
}

// FindGameStateFull locates the GameState using ParseBodyLevels, returning exact byte
// positions needed for property insertion. Similar to FindBlueprintSubsystem.
func FindGameStateFull(sd *SaveData) (*GameStateFullInfo, error) {
	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		return nil, fmt.Errorf("parsing body: %w", err)
	}

	for _, l := range levels {
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
			if !strings.Contains(cl, "gamestate") {
				continue
			}

			objStart := l.DataObjOffsets[i]
			objData := sd.Decompressed[l.DataBlobStart+objStart : l.DataBlobStart+objStart+l.DataObjSizes[i]]

			offset := 0
			if l.LevelVersion >= 38 {
				offset += 8 // saveCustomVersion + shouldMigrate
			}
			binarySize := int32(binary.LittleEndian.Uint32(objData[offset:]))
			offset += 4
			binaryDataStart := l.DataBlobStart + objStart + offset

			saveCustomVersion := l.SaveCustomVersions[i]
			useCompleteTag := IsCompletePropertyTagType(sd.VersionCtx, saveCustomVersion)

			return &GameStateFullInfo{
				IsActor:         h.Type == "Actor",
				UseCompleteTag:  useCompleteTag,
				ClassName:       h.ClassName,
				BinaryDataStart: binaryDataStart,
				BinaryDataSize:  int(binarySize),
				BinarySizePos:   l.DataBlobStart + objStart + offset - 4,
				DataLenPos:      l.DataLenPos,
			}, nil
		}
	}
	return nil, fmt.Errorf("GameState object not found")
}

// writeFString writes an Unreal Engine ASCII FString (length-prefixed + null terminator) to a buffer.
func writeFString(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.LittleEndian, int32(len(s)+1))
	buf.WriteString(s)
	buf.WriteByte(0)
}

// buildFloatProperty constructs the binary representation of a FloatProperty.
// The format differs depending on whether the save uses the complete property tag format.
func buildFloatProperty(name string, value float32, useCompleteTag bool) []byte {
	var buf bytes.Buffer

	writeFString(&buf, name)

	if useCompleteTag {
		// FPropertyTagNode: type name + child count (0 for FloatProperty)
		writeFString(&buf, "FloatProperty")
		binary.Write(&buf, binary.LittleEndian, int32(0))
		// propBinarySize (just the float value = 4 bytes)
		binary.Write(&buf, binary.LittleEndian, int32(4))
		// propFlags (0 = no extra fields)
		buf.WriteByte(0)
	} else {
		// Property type as FString
		writeFString(&buf, "FloatProperty")
		// propBinarySize
		binary.Write(&buf, binary.LittleEndian, int32(4))
		// index
		binary.Write(&buf, binary.LittleEndian, int32(0))
		// hasGuid (0 = no GUID)
		buf.WriteByte(0)
	}

	// The float32 value
	binary.Write(&buf, binary.LittleEndian, math.Float32bits(value))

	return buf.Bytes()
}

// findNoneTerminatorOffset scans the property list within the object's binary data
// and returns the byte offset (relative to object data start) where the "None" FString begins.
func findNoneTerminatorOffset(data []byte, isActor bool, useCompleteTag bool) (int, error) {
	var lastEnd int = -1

	WalkObjectProperties(data, isActor, useCompleteTag, func(it PropIteration) bool {
		// After each property, the reader position advances past the value.
		// The "None" terminator starts right after the last property's value.
		lastEnd = it.ValuePos + int(it.Header.PropBinarySize)
		return true
	})

	if lastEnd < 0 {
		// No properties at all — the "None" is the first thing after the preamble.
		// Recompute the position by skipping just the preamble.
		r := parser.NewBinaryReader(data)
		if isActor {
			r.ReadString()
			r.ReadString()
			compCount := r.ReadInt32()
			for i := int32(0); i < compCount; i++ {
				r.ReadString()
				r.ReadString()
			}
		}
		if useCompleteTag {
			r.ReadUInt8()
		}
		return r.Position(), nil
	}

	return lastEnd, nil
}

// InsertFloatProperty inserts a new FloatProperty into the GameState object's property list,
// before the "None" terminator. Updates all four size fields: propBinarySize (not needed since
// this is a new property), the object's binarySize, the level's data blob length, and totalBodyRestSize.
func InsertFloatProperty(sd *SaveData, gsi *GameStateFullInfo, propName string, value float32) error {
	objData := sd.Decompressed[gsi.BinaryDataStart : gsi.BinaryDataStart+gsi.BinaryDataSize]

	noneOffset, err := findNoneTerminatorOffset(objData, gsi.IsActor, gsi.UseCompleteTag)
	if err != nil {
		return fmt.Errorf("finding None terminator: %w", err)
	}

	propBytes := buildFloatProperty(propName, value, gsi.UseCompleteTag)
	noneAbsPos := gsi.BinaryDataStart + noneOffset

	// Insert the new property before the "None" terminator
	newDecompressed := make([]byte, 0, len(sd.Decompressed)+len(propBytes))
	newDecompressed = append(newDecompressed, sd.Decompressed[:noneAbsPos]...)
	newDecompressed = append(newDecompressed, propBytes...)
	newDecompressed = append(newDecompressed, sd.Decompressed[noneAbsPos:]...)
	sd.Decompressed = newDecompressed

	sizeDiff := int32(len(propBytes))

	// Update binarySize (object's binary data size)
	oldBinarySize := int32(binary.LittleEndian.Uint32(sd.Decompressed[gsi.BinarySizePos:]))
	newBinarySize := oldBinarySize + sizeDiff
	binary.LittleEndian.PutUint32(sd.Decompressed[gsi.BinarySizePos:], uint32(newBinarySize))

	// Update dataBlobLen (level's data blob length)
	oldDataBlobLen := int64(binary.LittleEndian.Uint64(sd.Decompressed[gsi.DataLenPos:]))
	newDataBlobLen := oldDataBlobLen + int64(sizeDiff)
	binary.LittleEndian.PutUint64(sd.Decompressed[gsi.DataLenPos:], uint64(newDataBlobLen))

	WriteTotalBodyRestSize(sd.Decompressed, sd.VersionCtx)

	return nil
}
