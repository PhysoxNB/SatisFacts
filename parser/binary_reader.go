package parser

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// BinaryReader is a random-access reader for little-endian data. We use it for
// TOC blobs and other buffers we already have fully in memory.
type BinaryReader struct {
	data []byte
	pos  int
}

func NewBinaryReader(data []byte) *BinaryReader {
	return &BinaryReader{data: data, pos: 0}
}

func (r *BinaryReader) Position() int    { return r.pos }
func (r *BinaryReader) Length() int       { return len(r.data) }
func (r *BinaryReader) Remaining() int    { return len(r.data) - r.pos }
func (r *BinaryReader) CanRead(n int) bool { return r.pos+n <= len(r.data) }

func (r *BinaryReader) SetPosition(pos int) {
	if pos < 0 || pos > len(r.data) {
		panic(fmt.Sprintf("position %d out of bounds (0-%d)", pos, len(r.data)))
	}
	r.pos = pos
}

func (r *BinaryReader) Skip(n int) {
	r.SetPosition(r.pos + n)
}

func (r *BinaryReader) ReadInt8() int8 {
	v := int8(r.data[r.pos])
	r.pos++
	return v
}

func (r *BinaryReader) ReadUInt8() uint8 {
	v := r.data[r.pos]
	r.pos++
	return v
}

func (r *BinaryReader) ReadInt16() int16 {
	v := int16(binary.LittleEndian.Uint16(r.data[r.pos:]))
	r.pos += 2
	return v
}

func (r *BinaryReader) ReadUInt16() uint16 {
	v := binary.LittleEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v
}

func (r *BinaryReader) ReadInt32() int32 {
	v := int32(binary.LittleEndian.Uint32(r.data[r.pos:]))
	r.pos += 4
	return v
}

func (r *BinaryReader) ReadUInt32() uint32 {
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v
}

func (r *BinaryReader) ReadInt64() int64 {
	v := int64(binary.LittleEndian.Uint64(r.data[r.pos:]))
	r.pos += 8
	return v
}

func (r *BinaryReader) ReadUInt64() uint64 {
	v := binary.LittleEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return v
}

func (r *BinaryReader) ReadFloat32() float32 {
	v := math.Float32frombits(binary.LittleEndian.Uint32(r.data[r.pos:]))
	r.pos += 4
	return v
}

func (r *BinaryReader) ReadFloat64() float64 {
	v := math.Float64frombits(binary.LittleEndian.Uint64(r.data[r.pos:]))
	r.pos += 8
	return v
}

// ReadBytes hands back a slice straight into the underlying data (no copy).
// Don't hold onto it past the buffer's lifetime, copy first if you need to.
func (r *BinaryReader) ReadBytes(n int) []byte {
	if !r.CanRead(n) {
		panic(fmt.Sprintf("cannot read %d bytes at position %d (len=%d)", n, r.pos, len(r.data)))
	}
	s := r.data[r.pos : r.pos+n]
	r.pos += n
	return s
}

// ReadBytesCopy returns a fresh copy that's safe to hang onto.
func (r *BinaryReader) ReadBytesCopy(n int) []byte {
	s := r.ReadBytes(n)
	out := make([]byte, n)
	copy(out, s)
	return out
}

// ReadString reads an Unreal Engine FString. It's length-prefixed: a positive
// int32 means that many ASCII bytes, negative means abs(length) UTF-16 chars,
// and 0 means empty.
func (r *BinaryReader) ReadString() string {
	length := r.ReadInt32()
	if length == 0 {
		return ""
	}

	if length < 0 {
		// UTF-16
		charCount := int(-length)
		byteCount := charCount * 2
		bytes := r.ReadBytes(byteCount)

		// Decode UTF-16LE, bail at the null terminator.
		codeUnits := make([]uint16, 0, charCount)
		for i := 0; i < charCount; i++ {
			c := binary.LittleEndian.Uint16(bytes[i*2:])
			if c == 0 {
				break
			}
			codeUnits = append(codeUnits, c)
		}
		return string(utf16.Decode(codeUnits))
	}

	// ASCII
	bytes := r.ReadBytes(int(length))
	// Cut off at the null terminator.
	end := len(bytes)
	for i, b := range bytes {
		if b == 0 {
			end = i
			break
		}
	}
	return string(bytes[:end])
}
