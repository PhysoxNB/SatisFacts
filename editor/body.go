package editor

import (
	"encoding/binary"
	"fmt"

	"satisfacts/parser"
)

// TotalBodyRestSizeFor returns the value that belongs in the first 4 bytes of the
// decompressed body. The field excludes the size field itself, and on UE5-era
// saves also the 4-byte zero that follows it - hence -8 there but -4 before.
// Writing -8 unconditionally leaves pre-UE5 saves 4 bytes short, which makes the
// game truncate the tail of the last data blob.
func TotalBodyRestSizeFor(body []byte, ctx *parser.VersionContext) int32 {
	if ctx != nil && ctx.SaveIsAtLeast(parser.SaveUnrealEngine5) {
		return int32(len(body) - 8)
	}
	return int32(len(body) - 4)
}

// WriteTotalBodyRestSize patches the totalBodyRestSize field of body in place.
func WriteTotalBodyRestSize(body []byte, ctx *parser.VersionContext) {
	if len(body) < 4 {
		return
	}
	binary.LittleEndian.PutUint32(body[0:4], uint32(TotalBodyRestSizeFor(body, ctx)))
}

// SkipFSaveObjectVersionData reads past the version data block and returns
// the number of bytes consumed.
func SkipFSaveObjectVersionData(r *parser.BinaryReader) int {
	start := r.Position()
	r.ReadUInt32()         // saveObjectVersionDataVersion
	r.ReadInt32()          // ue4Version
	r.ReadInt32()          // ue5Version
	r.ReadInt32()          // licenceVersion
	r.ReadUInt16()         // major
	r.ReadUInt16()         // minor
	r.ReadUInt16()         // patch
	r.ReadUInt32()         // changelist
	r.ReadString()         // branch
	count := r.ReadInt32() // customVersionContainer count
	for i := int32(0); i < count; i++ {
		r.ReadUInt32() // GUID part 1
		r.ReadUInt32() // GUID part 2
		r.ReadUInt32() // GUID part 3
		r.ReadUInt32() // GUID part 4
		r.ReadInt32()  // version
	}
	return r.Position() - start
}

// ExtractPackageFileVerUE5 reads the version data block and returns the UE5 version.
func ExtractPackageFileVerUE5(r *parser.BinaryReader) int32 {
	r.ReadUInt32() // saveObjectVersionDataVersion
	r.ReadInt32()  // ue4Version
	ue5Ver := r.ReadInt32()
	// Skip the rest
	r.ReadInt32()  // licenceVersion
	r.ReadUInt16() // major
	r.ReadUInt16() // minor
	r.ReadUInt16() // patch
	r.ReadUInt32() // changelist
	r.ReadString() // branch
	count := r.ReadInt32()
	for i := int32(0); i < count; i++ {
		r.ReadUInt32()
		r.ReadUInt32()
		r.ReadUInt32()
		r.ReadUInt32()
		r.ReadInt32()
	}
	return ue5Ver
}

// SkipSaveBodyValidation reads past the validation block.
func SkipSaveBodyValidation(r *parser.BinaryReader) int {
	start := r.Position()
	count := r.ReadInt32()
	for i := int32(0); i < count; i++ {
		r.ReadString()
		r.ReadInt32()
		r.ReadUInt32()
		childrenCount := r.ReadUInt32()
		for j := uint32(0); j < childrenCount; j++ {
			r.ReadString()
			r.ReadUInt32()
		}
	}
	return r.Position() - start
}

// IsCompletePropertyTagType returns true if the save uses the complete tag type format.
func IsCompletePropertyTagType(ctx *parser.VersionContext, saveCustomVersion int32) bool {
	return saveCustomVersion >= 53 && ctx.PackageFileVerUE5 >= 1012
}

// TagNode represents an FPropertyTagNode.
type TagNode struct {
	Name     string
	Children []TagNode
}

// ReadFPropertyTagNode reads an FPropertyTagNode recursively.
func ReadFPropertyTagNode(r *parser.BinaryReader) TagNode {
	name := r.ReadString()
	count := r.ReadInt32()
	node := TagNode{Name: name}
	for i := int32(0); i < count; i++ {
		node.Children = append(node.Children, ReadFPropertyTagNode(r))
	}
	return node
}

// ReadTagNodeName reads an FPropertyTagNode recursively and returns just the type name.
func ReadTagNodeName(r *parser.BinaryReader) string {
	name := r.ReadString()
	childCount := r.ReadInt32()
	for i := int32(0); i < childCount; i++ {
		ReadTagNodeName(r)
	}
	return name
}

// NestedPropHeader holds the parsed header of a nested property.
type NestedPropHeader struct {
	PropType            string
	PropBinarySize      int32
	PropFlags           uint8
	StructSubtype       string
	ArraySubtype        string
	BoolValueFromHeader bool
}

// PropIteration is passed to the callback of WalkObjectProperties for each
// property in the list. NamePos is the byte offset (relative to data) of the
// property name FString. ValuePos is the offset where the property value
// begins. PropBinarySizePos is the offset of the propBinarySize field.
type PropIteration struct {
	Name              string
	Header            NestedPropHeader
	NamePos           int
	ValuePos          int
	PropBinarySizePos int
}

// WalkObjectProperties reads past the actor header (if isActor) and the
// complete-tag byte (if useCompleteTag), then iterates every property in the
// list, calling fn for each one until it returns false or the "None" terminator
// is reached. This eliminates the ~40 lines of duplicated tag-parsing
// boilerplate that was copy-pasted across multipliers.go and
// blueprint_categories.go.
func WalkObjectProperties(data []byte, isActor bool, useCompleteTag bool, fn func(it PropIteration) bool) {
	r := parser.NewBinaryReader(data)

	if isActor {
		r.ReadString() // parent level name
		r.ReadString() // parent path name
		compCount := r.ReadInt32()
		for i := int32(0); i < compCount; i++ {
			r.ReadString()
			r.ReadString()
		}
	}

	if useCompleteTag {
		r.ReadUInt8()
	}

	posPropsStart := r.Position()
	remainingSize := len(data) - posPropsStart

	for r.Position()-posPropsStart < remainingSize-4 {
		propNamePos := r.Position()
		name := r.ReadString()
		if name == "None" {
			break
		}

		var propType string
		var propBinarySize int32
		var propBinarySizePos int
		var propFlags uint8

		if useCompleteTag {
			tn := ReadFPropertyTagNode(r)
			propType = tn.Name
			propBinarySize = r.ReadInt32()
			propBinarySizePos = r.Position() - 4
			propFlags = r.ReadUInt8()
			if propFlags&0x1 != 0 {
				r.ReadInt32()
			}
			if propFlags&0x2 != 0 {
				r.Skip(16)
			}
		} else {
			propType = r.ReadString()
			propBinarySize = r.ReadInt32()
			propBinarySizePos = r.Position() - 4
			r.ReadInt32() // index
			switch propType {
			case "ArrayProperty", "SetProperty":
				r.ReadString()
			case "StructProperty":
				r.ReadString()
				r.Skip(16)
			case "ByteProperty", "EnumProperty":
				r.ReadString()
			case "MapProperty":
				r.ReadString()
				r.ReadString()
			case "BoolProperty":
				r.ReadInt8()
			}
			hasGuid := r.ReadUInt8()
			if hasGuid != 0 {
				r.Skip(16)
			}
		}

		valuePos := r.Position()

		it := PropIteration{
			Name: name,
			Header: NestedPropHeader{
				PropType:       propType,
				PropBinarySize: propBinarySize,
				PropFlags:      propFlags,
			},
			NamePos:           propNamePos,
			ValuePos:          valuePos,
			PropBinarySizePos: propBinarySizePos,
		}

		if propType == "BoolProperty" && useCompleteTag {
			// BoolProperty in complete-tag format stores its value in the header
			// flags, not in the value area. There are no value bytes to skip.
			if !fn(it) {
				return
			}
			continue
		}

		if !fn(it) {
			return
		}

		r.SetPosition(valuePos + int(propBinarySize))
	}
}

// ParseNestedPropertyHeader reads a nested property header.
func ParseNestedPropertyHeader(r *parser.BinaryReader, useCompleteTagType bool) NestedPropHeader {
	h := NestedPropHeader{}
	if useCompleteTagType {
		tn := ReadFPropertyTagNode(r)
		h.PropType = tn.Name
		h.PropBinarySize = r.ReadInt32()
		h.PropFlags = r.ReadUInt8()
		if h.PropFlags&0x1 != 0 {
			r.ReadInt32()
		}
		if h.PropFlags&0x2 != 0 {
			r.Skip(16)
		}
		if len(tn.Children) > 0 {
			switch h.PropType {
			case "StructProperty":
				h.StructSubtype = tn.Children[0].Name
			case "ArrayProperty":
				h.ArraySubtype = tn.Children[0].Name
			}
		}
	} else {
		h.PropType = r.ReadString()
		h.PropBinarySize = r.ReadInt32()
		r.ReadInt32()
		switch h.PropType {
		case "ArrayProperty":
			h.ArraySubtype = r.ReadString()
		case "StructProperty":
			h.StructSubtype = r.ReadString()
			r.Skip(16)
		case "BoolProperty":
			h.BoolValueFromHeader = r.ReadInt8() > 0
		case "ByteProperty":
			r.ReadString() // enum type name
		case "EnumProperty":
			r.ReadString() // enum type name
		}
		hasGuid := r.ReadUInt8()
		if hasGuid != 0 {
			r.Skip(16)
		}
	}
	return h
}

// LevelInfo holds byte positions for a level within the decompressed body.
type LevelInfo struct {
	IsPersistent bool
	Name         string

	// Byte ranges within the decompressed body
	NameStart     int
	NameEnd       int
	TocLenPos     int
	TocDataStart  int
	TocDataEnd    int
	DataLenPos    int
	DataBlobStart int
	DataBlobEnd   int
	FooterStart   int
	FooterEnd     int

	// Parsed from TOC
	ObjectCount      int32
	TocHeaderOffsets []int
	TocHeaderSizes   []int

	// Parsed from data blob
	CountEntities      int32
	DataObjOffsets     []int
	DataObjSizes       []int
	SaveCustomVersions []int32 // per-object saveCustomVersion
	LevelVersion       int32   // level version used for parsing this level's data blob
}

// ParseBodyLevels walks the decompressed body and returns level info with
// exact byte positions for rebuilding.
func ParseBodyLevels(decompressed []byte, ctx *parser.VersionContext) (bodyHeaderEnd int, levels []*LevelInfo, trailingStart int, trailingEnd int, err error) {
	r := parser.NewBinaryReader(decompressed)

	r.ReadInt32() // totalBodyRestSize
	if ctx.SaveIsAtLeast(parser.SaveUnrealEngine5) {
		r.ReadInt32() // UE5 zero
	}
	if ctx.SaveIsAtLeast(parser.SaveSerializeDataPackageAndCustomVer) {
		ctx.PackageFileVerUE5 = ExtractPackageFileVerUE5(r)
	}
	if ctx.SaveIsAtLeast(parser.SaveIntroducedWorldPartition) {
		SkipSaveBodyValidation(r)
	}
	sublevelCount := r.ReadInt32()
	bodyHeaderEnd = r.Position()

	totalLevels := int(sublevelCount) + 1
	for li := 0; li < totalLevels; li++ {
		isPersistent := li == totalLevels-1
		info := &LevelInfo{IsPersistent: isPersistent}

		if !isPersistent {
			info.NameStart = r.Position()
			info.Name = r.ReadString()
			info.NameEnd = r.Position()
		} else {
			info.Name = "Persistent_Level"
		}

		// TOC blob
		info.TocLenPos = r.Position()
		tocLen := r.ReadInt64()
		info.TocDataStart = r.Position()
		tocData := r.ReadBytes(int(tocLen))
		info.TocDataEnd = r.Position()

		// Parse TOC
		tocReader := parser.NewBinaryReader(tocData)
		info.ObjectCount = tocReader.ReadInt32()
		if info.ObjectCount > 0 {
			offsets, sizes, oErr := parser.ParseObjectHeaderOffsets(tocReader, ctx, int(info.ObjectCount))
			if oErr != nil {
				return 0, nil, 0, 0, fmt.Errorf("parsing TOC headers in level %d: %w", li, oErr)
			}
			info.TocHeaderOffsets = offsets
			info.TocHeaderSizes = sizes
		}

		// Data blob
		info.DataLenPos = r.Position()
		dataLen := r.ReadInt64()
		info.DataBlobStart = r.Position()
		dataBlobData := r.ReadBytes(int(dataLen))
		info.DataBlobEnd = r.Position()

		// Parse data blob to find object entry positions
		if info.ObjectCount > 0 && len(dataBlobData) > 0 {
			dbReader := parser.NewBinaryReader(dataBlobData)
			info.CountEntities = dbReader.ReadInt32()

			levelVersion := ctx.LevelVersion
			info.LevelVersion = levelVersion
			for i := int32(0); i < info.ObjectCount; i++ {
				objStart := dbReader.Position()

				var saveCustomVersion int32
				if levelVersion >= 38 {
					saveCustomVersion = dbReader.ReadInt32()
					dbReader.ReadInt32() // shouldMigrate
				}

				binarySize := dbReader.ReadInt32()
				dbReader.Skip(int(binarySize))

				if saveCustomVersion >= 53 {
					shouldSerialize := dbReader.ReadInt32()
					if shouldSerialize == 1 {
						SkipFSaveObjectVersionData(dbReader)
					}
				}

				objEnd := dbReader.Position()
				info.DataObjOffsets = append(info.DataObjOffsets, objStart)
				info.DataObjSizes = append(info.DataObjSizes, objEnd-objStart)
				info.SaveCustomVersions = append(info.SaveCustomVersions, saveCustomVersion)
			}
		}

		// Footer
		info.FooterStart = r.Position()

		if !isPersistent && ctx.SaveIsAtLeast(parser.SaveSerializePerStreamableLevelTOC) {
			sv := r.ReadInt32()
			ctx.LevelVersion = sv
		}

		if isPersistent {
			// Some third-party tools (e.g. SCIM) truncate the persistent level
			// footer, leaving only a partial or missing destroyed-actors map.
			// The game tolerates this, so we do too - bail out gracefully on
			// truncated data instead of panicking.
			if r.Remaining() < 4 {
				info.FooterEnd = r.Position()
				levels = append(levels, info)
				continue
			}
			mapCount := r.ReadInt32()
			if mapCount > 0 && mapCount <= 1000000 {
				for i := int32(0); i < mapCount; i++ {
					if r.Remaining() < 4 {
						break
					}
					r.ReadString()
					if r.Remaining() < 4 {
						break
					}
					arrayCount := r.ReadInt32()
					if arrayCount >= 0 && arrayCount <= 100000 {
						for j := int32(0); j < arrayCount; j++ {
							if r.Remaining() < 4 {
								break
							}
							r.ReadString()
							if r.Remaining() < 4 {
								break
							}
							r.ReadString()
						}
					}
				}
			}
		} else {
			collectablesCount := r.ReadInt32()
			if collectablesCount >= 0 && collectablesCount <= 1000000 {
				for i := int32(0); i < collectablesCount; i++ {
					r.ReadString()
					r.ReadString()
				}
			}
			if ctx.SaveIsAtLeast(parser.SaveSerializeDataPackageAndCustomVer) {
				shouldSerialize := r.ReadInt32()
				if shouldSerialize >= 1 {
					SkipFSaveObjectVersionData(r)
				}
			}
		}

		info.FooterEnd = r.Position()
		levels = append(levels, info)
	}

	trailingStart = r.Position()
	trailingEnd = len(decompressed)

	return bodyHeaderEnd, levels, trailingStart, trailingEnd, nil
}
