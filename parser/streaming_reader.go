package parser

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// StreamingReader reads decompressed data chunk by chunk so we never keep the
// whole body in memory. Forward-only, no seeking.
type StreamingReader struct {
	chunkCh    <-chan ChunkResult
	current    []byte
	offset     int
	virtPos    int64
	exhausted  bool
}

func NewStreamingReader(chunkCh <-chan ChunkResult) *StreamingReader {
	return &StreamingReader{chunkCh: chunkCh}
}

// advance grabs the next chunk once we've drained the current one.
func (r *StreamingReader) advance() error {
	for r.current == nil || r.offset >= len(r.current) {
		if r.exhausted {
			return fmt.Errorf("unexpected end of stream at virtual position %d", r.virtPos)
		}
		chunk, ok := <-r.chunkCh
		if !ok || chunk.Data == nil {
			r.exhausted = true
			r.current = nil
			r.offset = 0
			return fmt.Errorf("unexpected end of stream at virtual position %d", r.virtPos)
		}
		r.current = chunk.Data
		r.offset = 0
	}
	return nil
}

// readInto fills target, crossing chunk boundaries when it has to.
func (r *StreamingReader) readInto(target []byte) error {
	remaining := len(target)
	pos := 0
	for remaining > 0 {
		if err := r.advance(); err != nil {
			return err
		}
		available := len(r.current) - r.offset
		toCopy := remaining
		if toCopy > available {
			toCopy = available
		}
		copy(target[pos:pos+toCopy], r.current[r.offset:r.offset+toCopy])
		r.offset += toCopy
		pos += toCopy
		remaining -= toCopy
	}
	r.virtPos += int64(len(target))
	return nil
}

func (r *StreamingReader) Position() int64 { return r.virtPos }

func (r *StreamingReader) Skip(n int) error {
	remaining := n
	for remaining > 0 {
		if err := r.advance(); err != nil {
			return err
		}
		available := len(r.current) - r.offset
		toSkip := remaining
		if toSkip > available {
			toSkip = available
		}
		r.offset += toSkip
		remaining -= toSkip
	}
	r.virtPos += int64(n)
	return nil
}

func (r *StreamingReader) ReadInt8() (int8, error) {
	var buf [1]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return int8(buf[0]), nil
}

func (r *StreamingReader) ReadUInt8() (uint8, error) {
	var buf [1]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

func (r *StreamingReader) ReadInt16() (int16, error) {
	var buf [2]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf[:])), nil
}

func (r *StreamingReader) ReadUInt16() (uint16, error) {
	var buf [2]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf[:]), nil
}

func (r *StreamingReader) ReadInt32() (int32, error) {
	var buf [4]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf[:])), nil
}

func (r *StreamingReader) ReadUInt32() (uint32, error) {
	var buf [4]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func (r *StreamingReader) ReadInt64() (int64, error) {
	var buf [8]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf[:])), nil
}

func (r *StreamingReader) ReadUInt64() (uint64, error) {
	var buf [8]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

func (r *StreamingReader) ReadFloat32() (float32, error) {
	var buf [4]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(buf[:])), nil
}

func (r *StreamingReader) ReadFloat64() (float64, error) {
	var buf [8]byte
	if err := r.readInto(buf[:]); err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(buf[:])), nil
}

// ReadBytes reads n bytes into a freshly allocated slice.
func (r *StreamingReader) ReadBytes(n int) ([]byte, error) {
	if n <= 0 {
		return []byte{}, nil
	}
	result := make([]byte, n)
	if err := r.readInto(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ReadString reads an Unreal Engine FString (see BinaryReader.ReadString).
func (r *StreamingReader) ReadString() (string, error) {
	length, err := r.ReadInt32()
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}

	if length < 0 {
		// UTF-16
		charCount := int(-length)
		byteCount := charCount * 2
		bytes, err := r.ReadBytes(byteCount)
		if err != nil {
			return "", err
		}
		codeUnits := make([]uint16, 0, charCount)
		for i := 0; i < charCount; i++ {
			c := binary.LittleEndian.Uint16(bytes[i*2:])
			if c == 0 {
				break
			}
			codeUnits = append(codeUnits, c)
		}
		return string(utf16.Decode(codeUnits)), nil
	}

	// ASCII
	bytes, err := r.ReadBytes(int(length))
	if err != nil {
		return "", err
	}
	end := len(bytes)
	for i, b := range bytes {
		if b == 0 {
			end = i
			break
		}
	}
	return string(bytes[:end]), nil
}
