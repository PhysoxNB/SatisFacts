package parser

import (
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

// Magic number every chunk header starts with.
const packageFileTag = 0x9E2A83C1

// The two chunk-header versions we've seen.
const (
	headerV1 = 0x00000000
	headerV2 = 0x22222222
)

// ChunkHeader is the metadata in front of each compressed chunk.
type ChunkHeader struct {
	PackageFileTag      uint32
	ChunkHeaderVersion  uint32
	MaxChunkSize        int32
	CompressionAlgorithm uint8
	CompressedSize      int32
	UncompressedSize    int32
	HeaderSize          int
}

// ChunkResult is a single decompressed chunk plus its sizes.
type ChunkResult struct {
	Index             int
	CompressedSize   int
	UncompressedSize int
	Data             []byte
}

// Decompressor pulls compressed chunks off a raw stream and inflates them one
// at a time, handing the results back over a channel.
type Decompressor struct {
	reader io.ReadSeeker // raw compressed data, starting after the save header
	pos    int64
}

func NewDecompressor(reader io.ReadSeeker) *Decompressor {
	return &Decompressor{reader: reader}
}

// readChunkHeader reads and validates one chunk header off the raw stream.
func (d *Decompressor) readChunkHeader() (*ChunkHeader, error) {
	// First 8 bytes: packageFileTag + chunkHeaderVersion, both uint32.
	var buf [8]byte
	if _, err := io.ReadFull(d.reader, buf[:]); err != nil {
		return nil, err // EOF here just means we're done
	}
	d.pos += 8

	tag := binary.LittleEndian.Uint32(buf[:4])
	version := binary.LittleEndian.Uint32(buf[4:])

	if tag != packageFileTag {
		return nil, fmt.Errorf("invalid package file tag: 0x%X (expected 0x%X)", tag, packageFileTag)
	}

	isV2 := version == headerV2
	headerSize := 49 // V2
	if !isV2 {
		headerSize = 48 // V1
	}

	// maxChunkSize (int32), then 4 padding bytes.
	var maxChunkSize int32
	if err := binary.Read(d.reader, binary.LittleEndian, &maxChunkSize); err != nil {
		return nil, err
	}
	d.pos += 4

	// Eat the padding.
	if _, err := io.CopyN(io.Discard, d.reader, 4); err != nil {
		return nil, err
	}
	d.pos += 4

	var compressionAlgorithm uint8 = 3 // ZLIB
	if isV2 {
		if err := binary.Read(d.reader, binary.LittleEndian, &compressionAlgorithm); err != nil {
			return nil, err
		}
		d.pos += 1
	}

	// Four sizes back to back (compressed, uncompressed, then the same pair
	// again), each followed by 4 padding bytes.
	var compressedSize, uncompressedSize, compressedSize2, uncompressedSize2 int32
	for _, ptr := range []*int32{&compressedSize, &uncompressedSize, &compressedSize2, &uncompressedSize2} {
		if err := binary.Read(d.reader, binary.LittleEndian, ptr); err != nil {
			return nil, err
		}
		d.pos += 4
		// padding
		if _, err := io.CopyN(io.Discard, d.reader, 4); err != nil {
			return nil, err
		}
		d.pos += 4
	}

	return &ChunkHeader{
		PackageFileTag:       tag,
		ChunkHeaderVersion:   version,
		MaxChunkSize:         maxChunkSize,
		CompressionAlgorithm: compressionAlgorithm,
		CompressedSize:       compressedSize,
		UncompressedSize:     uncompressedSize,
		HeaderSize:           headerSize,
	}, nil
}

// DecompressAll inflates every chunk and glues the results together. On big
// saves use StreamChunks() instead so you don't hold it all in memory at once.
func (d *Decompressor) DecompressAll() ([]byte, error) {
	var result []byte
	chunks, err := d.StreamChunks()
	if err != nil {
		return nil, err
	}
	for chunk := range chunks {
		if chunk.Data == nil {
			break // either an error or the end
		}
		result = append(result, chunk.Data...)
	}
	return result, nil
}

// StreamChunks hands back a channel that yields decompressed chunks one by one.
// Each one is inflated on demand and can be thrown away once you're done with
// it. The channel closes when we run out of chunks or hit an error.
func (d *Decompressor) StreamChunks() (<-chan ChunkResult, error) {
	ch := make(chan ChunkResult, 1)

	// Read the first header up front so we can fail fast on a bad stream.
	header, err := d.readChunkHeader()
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(ch)
		idx := 0

		for {
			// Pull in the compressed bytes for this chunk.
			compressedSize := int(header.CompressedSize)
			compressedData := make([]byte, compressedSize)
			if _, err := io.ReadFull(d.reader, compressedData); err != nil {
				ch <- ChunkResult{Index: idx, Data: nil}
				return
			}
			d.pos += int64(compressedSize)

			// Inflate it.
			zr, err := zlib.NewReader(newBytesReader(compressedData))
			if err != nil {
				ch <- ChunkResult{Index: idx, Data: nil}
				return
			}

			uncompressedSize := int(header.UncompressedSize)
			decompressed := make([]byte, uncompressedSize)
			if _, err := io.ReadFull(zr, decompressed); err != nil {
				zr.Close()
				ch <- ChunkResult{Index: idx, Data: nil}
				return
			}
			zr.Close()

			ch <- ChunkResult{
				Index:             idx,
				CompressedSize:   compressedSize,
				UncompressedSize: uncompressedSize,
				Data:             decompressed,
			}
			idx++

			// On to the next header.
			header, err = d.readChunkHeader()
			if err != nil {
				// EOF just means we're finished.
				return
			}
		}
	}()

	return ch, nil
}

// bytesReader adapts a []byte to io.Reader.
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) io.Reader {
	return &bytesReader{data: data}
}

func (b *bytesReader) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}
