package parser

import (
	"encoding/binary"
	"fmt"
	"testing"
)

// headerBuilder writes a synthetic save header byte-for-byte the way the real
// game does, so ParseHeader's version-branching can be regression-tested
// without needing an actual save file for every header version.
type headerBuilder struct {
	buf []byte
}

func (b *headerBuilder) i32(v int32) *headerBuilder {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], uint32(v))
	b.buf = append(b.buf, tmp[:]...)
	return b
}

func (b *headerBuilder) i64(v int64) *headerBuilder {
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], uint64(v))
	b.buf = append(b.buf, tmp[:]...)
	return b
}

func (b *headerBuilder) u8(v uint8) *headerBuilder {
	b.buf = append(b.buf, v)
	return b
}

func (b *headerBuilder) bytes(n int) *headerBuilder {
	b.buf = append(b.buf, make([]byte, n)...)
	return b
}

// asciiString writes an FString: positive length prefix (including the null
// terminator) followed by the ASCII bytes and a trailing 0x00.
func (b *headerBuilder) asciiString(s string) *headerBuilder {
	if s == "" {
		return b.i32(0)
	}
	b.i32(int32(len(s) + 1))
	b.buf = append(b.buf, s...)
	b.buf = append(b.buf, 0)
	return b
}

// buildHeader constructs a byte stream matching ParseHeader's field order for
// a given SaveHeaderType, following the exact same version gates the parser
// checks (HeaderIsAtLeast, plus the v13/v14 quirks).
func buildHeader(headerType int32) []byte {
	b := &headerBuilder{}
	b.i32(headerType) // SaveHeaderType
	b.i32(42)         // SaveVersion
	b.i32(12345)      // BuildVersion

	if headerType >= HeaderAddedSaveName {
		b.asciiString("MySave")
	}
	b.asciiString("Persistent_Level")
	b.asciiString("")
	b.asciiString("TestSession")
	b.i32(100) // PlayDurationSeconds
	b.i64(0)   // SaveDateTime ticks
	b.u8(1)    // SessionVisibility

	if headerType >= HeaderUE425EngineUpdate {
		b.i32(7) // FEditorObjectVersion
	}
	if headerType >= HeaderAddedModdingParams {
		b.asciiString("")
		b.i32(0) // IsModdedSave
	}
	if headerType >= HeaderAddedSaveIdentifier {
		b.asciiString("save-id-123")
	}
	if headerType >= HeaderAddedWorldPartition {
		b.i32(1) // PartitionEnabledFlag
	}

	// Quirk: v13 puts creative-mode before the checksum.
	if headerType == 13 {
		b.i32(1) // CreativeModeEnabled
	}

	b.bytes(16) // checksum, always 16 bytes

	if headerType == 13 {
		b.i32(99) // ExtraField, after the checksum
	}
	if headerType >= HeaderAddedIsCreativeMode && headerType != 13 {
		b.i32(1) // CreativeModeEnabled
	}
	if headerType == 14 {
		b.i32(99) // ExtraField
	}

	return b.buf
}

func TestParseHeaderConsumesExactBytes(t *testing.T) {
	// v13 and v14 exercise the documented quirks; v20 stands in for every
	// "normal" version above 14 (no quirks, no FMD5Hash bIsValid prefix).
	for _, headerType := range []int32{13, 14, 15, 20} {
		t.Run(fmt.Sprintf("v%d", headerType), func(t *testing.T) {
			buf := buildHeader(headerType)
			r := NewBinaryReader(buf)

			header, _, err := ParseHeader(r)
			if err != nil {
				t.Fatalf("ParseHeader(v%d) returned error: %v", headerType, err)
			}
			if header.SaveHeaderType != headerType {
				t.Errorf("SaveHeaderType = %d, want %d", header.SaveHeaderType, headerType)
			}
			if r.Position() != len(buf) {
				t.Errorf("ParseHeader(v%d) consumed %d bytes, want exactly %d (all of the header) - "+
					"a mismatch here desyncs everything parsed after the header",
					headerType, r.Position(), len(buf))
			}
			if headerType != 13 && header.CreativeModeEnabled != 1 {
				t.Errorf("CreativeModeEnabled = %d, want 1 (field got mapped to the wrong offset)", header.CreativeModeEnabled)
			}
		})
	}
}
