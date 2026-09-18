package editor

import (
	"encoding/binary"
	"math"
)

// bpWriter is a byte buffer for serializing blueprint category data.
type bpWriter struct {
	buf []byte
}

func (w *bpWriter) bytes() []byte { return w.buf }

func (w *bpWriter) writeU8(v uint8) { w.buf = append(w.buf, v) }
func (w *bpWriter) writeU16(v uint16) {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	w.buf = append(w.buf, b...)
}
func (w *bpWriter) writeI8(v int8)   { w.buf = append(w.buf, byte(v)) }
func (w *bpWriter) writeI32(v int32) { w.writeU32(uint32(v)) }

func (w *bpWriter) writeU32(v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	w.buf = append(w.buf, b...)
}

func (w *bpWriter) writeF32(v float32) { w.writeU32(math.Float32bits(v)) }

func (w *bpWriter) writeFString(s string) {
	if len(s) == 0 {
		w.writeI32(0)
		return
	}
	// Check if string is pure ASCII
	isASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			isASCII = false
			break
		}
	}
	if isASCII {
		w.writeI32(int32(len(s) + 1)) // length includes null terminator
		w.buf = append(w.buf, []byte(s)...)
		w.buf = append(w.buf, 0) // null terminator
	} else {
		// Encode as UTF-16LE with negative length
		runes := []rune(s)
		charCount := len(runes) + 1 // includes null terminator
		w.writeI32(int32(-charCount))
		for _, r := range runes {
			w.writeU16(uint16(r))
		}
		w.writeU16(0) // null terminator
	}
}

func (w *bpWriter) writeFName(s string) { w.writeFString(s) }

func (w *bpWriter) writeZeroGuid() { w.buf = append(w.buf, make([]byte, 16)...) }

// patchI32 overwrites 4 bytes at pos with the given int32 value.
func (w *bpWriter) patchI32(pos int, v int32) {
	binary.LittleEndian.PutUint32(w.buf[pos:], uint32(v))
}

// --- Property header writers ---
// Each writes the full property header (name + type info + size placeholder + flags)
// and returns the position of the Size field for later patching.

func (w *bpWriter) writePropHeaderSimple(name, propType string, useCompleteTag bool) int {
	w.writeFName(name)
	if useCompleteTag {
		w.writeFName(propType)
		w.writeI32(0) // InnerCount
		sizePos := len(w.buf)
		w.writeI32(0) // Size placeholder
		w.writeU8(0)  // Flags
		return sizePos
	}
	w.writeFName(propType)
	sizePos := len(w.buf)
	w.writeI32(0) // Size placeholder
	w.writeI32(0) // ArrayIndex
	w.writeU8(0)  // HasPropertyGuid
	return sizePos
}

func (w *bpWriter) writePropHeaderBool(name string, value bool, useCompleteTag bool) int {
	w.writeFName(name)
	if useCompleteTag {
		w.writeFName("BoolProperty")
		w.writeI32(0) // InnerCount
		sizePos := len(w.buf)
		w.writeI32(0) // Size = 0
		flags := uint8(0)
		if value {
			flags |= 0x10
		}
		w.writeU8(flags)
		return sizePos
	}
	w.writeFName("BoolProperty")
	sizePos := len(w.buf)
	w.writeI32(0)              // Size = 0
	w.writeI32(0)              // ArrayIndex
	w.writeI8(boolToI8(value)) // BoolVal
	w.writeU8(0)               // HasPropertyGuid
	return sizePos
}

func (w *bpWriter) writePropHeaderByte(name string, useCompleteTag bool) int {
	w.writeFName(name)
	if useCompleteTag {
		w.writeFName("ByteProperty")
		w.writeI32(0) // InnerCount = 0 (no children in complete-tag format)
		sizePos := len(w.buf)
		w.writeI32(0) // Size placeholder
		w.writeU8(0)  // Flags
		return sizePos
	}
	w.writeFName("ByteProperty")
	sizePos := len(w.buf)
	w.writeI32(0)        // Size placeholder
	w.writeI32(0)        // ArrayIndex
	w.writeFName("None") // EnumName
	w.writeU8(0)         // HasPropertyGuid
	return sizePos
}

func (w *bpWriter) writePropHeaderArray(name, innerType, structName string, useCompleteTag bool, flags uint8) int {
	w.writeFName(name)
	if useCompleteTag {
		w.writeFName("ArrayProperty")
		if structName != "" {
			// Array<StructProperty<StructName</Script/FactoryGame>>
			w.writeI32(1) // InnerCount = 1
			w.writeFName("StructProperty")
			w.writeI32(1) // child InnerCount = 1
			w.writeFName(structName)
			w.writeI32(1) // grandchild InnerCount = 1
			w.writeFName("/Script/FactoryGame")
			w.writeI32(0) // great-grandchild InnerCount = 0
		} else {
			w.writeI32(1) // InnerCount = 1
			w.writeFName(innerType)
			w.writeI32(0) // child InnerCount = 0
		}
		sizePos := len(w.buf)
		w.writeI32(0) // Size placeholder
		w.writeU8(flags)
		return sizePos
	}
	w.writeFName("ArrayProperty")
	sizePos := len(w.buf)
	w.writeI32(0)           // Size placeholder
	w.writeI32(0)           // ArrayIndex
	w.writeFName(innerType) // InnerType
	w.writeU8(0)            // HasPropertyGuid
	return sizePos
}

func (w *bpWriter) writePropHeaderStruct(name, structName string, useCompleteTag bool, flags uint8) int {
	w.writeFName(name)
	if useCompleteTag {
		w.writeFName("StructProperty")
		w.writeI32(1) // InnerCount = 1
		w.writeFName(structName)
		w.writeI32(1) // child InnerCount = 1
		w.writeFName("/Script/FactoryGame")
		w.writeI32(0) // grandchild InnerCount = 0
		sizePos := len(w.buf)
		w.writeI32(0) // Size placeholder
		w.writeU8(flags)
		return sizePos
	}
	w.writeFName("StructProperty")
	sizePos := len(w.buf)
	w.writeI32(0)            // Size placeholder
	w.writeI32(0)            // ArrayIndex
	w.writeFName(structName) // StructName
	w.writeZeroGuid()        // StructGuid
	w.writeU8(0)             // HasPropertyGuid
	return sizePos
}

// --- Main serialization entry point ---

// SerializeCategoryRecords serializes blueprint category records into the binary
// property value format. The output matches what ExtractCategoryRecords reads:
// [int32 count] [old-format inner tag if !useCompleteTag] [struct elements]
func SerializeCategoryRecords(categories []BlueprintCategoryRecord, useCompleteTag bool) []byte {
	w := &bpWriter{}

	w.writeI32(int32(len(categories)))

	if !useCompleteTag {
		// Old format: inner FPropertyTag (StructProperty) for the array
		w.writeFName("mBlueprintCategoryRecords")
		w.writeFName("StructProperty")
		innerSizePos := len(w.buf)
		w.writeI32(0) // Size placeholder (will patch)
		w.writeI32(0) // ArrayIndex
		w.writeFName("BlueprintCategoryRecord")
		w.writeZeroGuid()
		w.writeU8(0) // HasPropertyGuid

		structDataStart := len(w.buf)
		for _, cat := range categories {
			serializeCategoryRecord(w, &cat, useCompleteTag)
		}
		w.patchI32(innerSizePos, int32(len(w.buf)-structDataStart))
	} else {
		// Complete tag format: no inner tag, just struct elements
		for _, cat := range categories {
			serializeCategoryRecord(w, &cat, useCompleteTag)
		}
	}

	return w.bytes()
}

func serializeCategoryRecord(w *bpWriter, cat *BlueprintCategoryRecord, useCompleteTag bool) {
	// CategoryName: StrProperty
	sizePos := w.writePropHeaderSimple("CategoryName", "StrProperty", useCompleteTag)
	valStart := len(w.buf)
	w.writeFString(cat.CategoryName)
	w.patchI32(sizePos, int32(len(w.buf)-valStart))

	// IconID: IntProperty
	sizePos = w.writePropHeaderSimple("IconID", "IntProperty", useCompleteTag)
	w.writeI32(cat.IconID)
	w.patchI32(sizePos, 4)

	// MenuPriority: FloatProperty
	sizePos = w.writePropHeaderSimple("MenuPriority", "FloatProperty", useCompleteTag)
	w.writeF32(cat.MenuPriority)
	w.patchI32(sizePos, 4)

	// IsUndefined: BoolProperty (value in header, no value bytes)
	w.writePropHeaderBool("IsUndefined", cat.IsUndefined, useCompleteTag)

	// SubCategoryRecords: ArrayProperty<StructProperty<BlueprintSubCategoryRecord>>
	sizePos = w.writePropHeaderArray("SubCategoryRecords", "StructProperty", "BlueprintSubCategoryRecord", useCompleteTag, 0)
	valStart = len(w.buf)
	w.writeI32(int32(len(cat.SubCategories)))
	if !useCompleteTag {
		// Old format: inner FPropertyTag for the sub-array
		w.writeFName("SubCategoryRecords")
		w.writeFName("StructProperty")
		subInnerSizePos := len(w.buf)
		w.writeI32(0) // Size placeholder
		w.writeI32(0) // ArrayIndex
		w.writeFName("BlueprintSubCategoryRecord")
		w.writeZeroGuid()
		w.writeU8(0) // HasPropertyGuid
		subDataStart := len(w.buf)
		for _, sub := range cat.SubCategories {
			serializeSubCategoryRecord(w, &sub, useCompleteTag)
		}
		w.patchI32(subInnerSizePos, int32(len(w.buf)-subDataStart))
	} else {
		for _, sub := range cat.SubCategories {
			serializeSubCategoryRecord(w, &sub, useCompleteTag)
		}
	}
	w.patchI32(sizePos, int32(len(w.buf)-valStart))

	// lastEditedBy: StructProperty<PlayerInfoHandle> in v60, ArrayProperty<StructProperty> in v52
	if useCompleteTag {
		sizePos = w.writePropHeaderStruct("lastEditedBy", "PlayerInfoHandle", useCompleteTag, 0x08)
		// PlayerInfoHandle empty value: 00 ff ff ff ff (5 bytes)
		w.writeU8(0)
		w.writeU32(0xFFFFFFFF)
		w.patchI32(sizePos, 5)
	} else {
		sizePos = w.writePropHeaderArray("lastEditedBy", "StructProperty", "LocalUserNetIdBundle", useCompleteTag, 0x08)
		valStart = len(w.buf)
		w.writeI32(0) // empty array count
		w.writeFName("lastEditedBy")
		w.writeFName("StructProperty")
		w.writeI32(0) // Size = 0
		w.writeI32(0) // ArrayIndex
		w.writeFName("LocalUserNetIdBundle")
		w.writeZeroGuid()
		w.writeU8(0) // HasPropertyGuid
		w.patchI32(sizePos, int32(len(w.buf)-valStart))
	}

	// None: end of property list
	w.writeFName("None")
}

func serializeSubCategoryRecord(w *bpWriter, sub *BlueprintSubCategory, useCompleteTag bool) {
	// SubCategoryName: StrProperty
	sizePos := w.writePropHeaderSimple("SubCategoryName", "StrProperty", useCompleteTag)
	valStart := len(w.buf)
	w.writeFString(sub.SubCategoryName)
	w.patchI32(sizePos, int32(len(w.buf)-valStart))

	// MenuPriority: FloatProperty
	sizePos = w.writePropHeaderSimple("MenuPriority", "FloatProperty", useCompleteTag)
	w.writeF32(sub.MenuPriority)
	w.patchI32(sizePos, 4)

	// IsUndefined: ByteProperty (enum name "None"), value is 1 byte
	sizePos = w.writePropHeaderByte("IsUndefined", useCompleteTag)
	w.writeU8(boolToByte(sub.IsUndefined))
	w.patchI32(sizePos, 1)

	// BlueprintNames: ArrayProperty<StrProperty>
	sizePos = w.writePropHeaderArray("BlueprintNames", "StrProperty", "", useCompleteTag, 0)
	valStart = len(w.buf)
	w.writeI32(int32(len(sub.BlueprintNames)))
	for _, name := range sub.BlueprintNames {
		w.writeFString(name)
	}
	w.patchI32(sizePos, int32(len(w.buf)-valStart))

	// lastEditedBy: StructProperty<PlayerInfoHandle> in v60, StructProperty in v52
	if useCompleteTag {
		sizePos = w.writePropHeaderStruct("lastEditedBy", "PlayerInfoHandle", useCompleteTag, 0x08)
		// PlayerInfoHandle empty value: 06 00 00 00 00 (5 bytes)
		w.writeU8(6)
		w.writeU32(0)
		w.patchI32(sizePos, 5)
	} else {
		sizePos = w.writePropHeaderStruct("lastEditedBy", "LocalUserNetIdBundle", useCompleteTag, 0x08)
		valStart = len(w.buf)
		w.writeFName("None")
		w.patchI32(sizePos, int32(len(w.buf)-valStart))
	}

	// None: end of property list
	w.writeFName("None")
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func boolToI8(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
