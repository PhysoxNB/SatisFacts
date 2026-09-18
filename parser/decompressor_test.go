package parser

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
)

func testChunk(t *testing.T, data []byte) []byte {
	t.Helper()
	var comp bytes.Buffer
	zw := zlib.NewWriter(&comp)
	zw.Write(data)
	zw.Close()

	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint32(packageFileTag))
	binary.Write(&b, binary.LittleEndian, uint32(headerV2))
	binary.Write(&b, binary.LittleEndian, int32(131072))
	b.Write(make([]byte, 4))
	b.WriteByte(3)
	for _, size := range []int{comp.Len(), len(data), comp.Len(), len(data)} {
		binary.Write(&b, binary.LittleEndian, int32(size))
		b.Write(make([]byte, 4))
	}
	b.Write(comp.Bytes())
	return b.Bytes()
}

func TestDecompressAllReadsEveryChunk(t *testing.T) {
	stream := append(testChunk(t, []byte("first ")), testChunk(t, []byte("second"))...)
	got, err := NewDecompressor(bytes.NewReader(stream)).DecompressAll()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first second" {
		t.Errorf("got %q", got)
	}
}

func TestDecompressAllReportsDamage(t *testing.T) {
	first := testChunk(t, []byte("first "))
	second := testChunk(t, []byte("second"))

	badTag := append(append([]byte{}, first...), second...)
	badTag[len(first)] ^= 0xFF

	badData := append(append([]byte{}, first...), second...)
	badData[len(badData)-3] ^= 0xFF

	cutHeader := append(append([]byte{}, first...), second[:20]...)

	for name, stream := range map[string][]byte{"bad tag": badTag, "bad zlib data": badData, "header cut short": cutHeader} {
		if _, err := NewDecompressor(bytes.NewReader(stream)).DecompressAll(); err == nil {
			t.Errorf("%s: expected an error, got partial data with none", name)
		}
	}
}
