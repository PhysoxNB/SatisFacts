package parser

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
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
	PackageFileTag       uint32
	ChunkHeaderVersion   uint32
	MaxChunkSize         int32
	CompressionAlgorithm uint8
	CompressedSize       int32
	UncompressedSize     int32
	HeaderSize           int
}

// ChunkResult is a single decompressed chunk plus its sizes.
type ChunkResult struct {
	Index            int
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
//
// Workers inflate each chunk straight into its place in the result. Allocating a
// buffer per chunk and copying it over left the garbage collector so much to
// clean up that memory use reached about twice the size of the body.
func (d *Decompressor) DecompressAll() ([]byte, error) {
	total, err := d.TotalUncompressedSize()
	if err != nil {
		return nil, fmt.Errorf("the file may be corrupt or truncated: %w", err)
	}
	result := make([]byte, total)

	type job struct {
		compressed []byte
		dst        []byte
	}
	jobs := make(chan job, decompressWorkers)
	var buffers sync.Pool
	var failed atomic.Bool

	var wg sync.WaitGroup
	for i := 0; i < decompressWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var in chunkInflater
			defer in.close()
			for j := range jobs {
				if !failed.Load() && in.inflate(j.compressed, j.dst) != nil {
					failed.Store(true)
				}
				buffers.Put(j.compressed[:0])
			}
		}()
	}

	var offset int64
	var readErr error
	for idx := 0; !failed.Load(); idx++ {
		header, err := d.readChunkHeader()
		if err == io.EOF && idx > 0 {
			break
		}
		if err != nil {
			readErr = err
			break
		}
		compressedSize, size := int(header.CompressedSize), int64(header.UncompressedSize)
		if compressedSize < 0 || size < 0 || offset+size > total {
			readErr = fmt.Errorf("chunk %d has invalid sizes", idx)
			break
		}

		buf, _ := buffers.Get().([]byte)
		if cap(buf) < compressedSize {
			buf = make([]byte, compressedSize)
		}
		buf = buf[:compressedSize]
		if _, err := io.ReadFull(d.reader, buf); err != nil {
			readErr = err
			break
		}
		d.pos += int64(compressedSize)

		jobs <- job{compressed: buf, dst: result[offset : offset+size]}
		offset += size
	}
	close(jobs)
	wg.Wait()

	if readErr != nil {
		return nil, fmt.Errorf("the save data could not be read - the file may be corrupt or truncated: %w", readErr)
	}
	if failed.Load() {
		return nil, fmt.Errorf("the save data could not be decompressed - the file may be corrupt or truncated")
	}
	return result[:offset], nil
}

// chunkInflater inflates chunks one at a time, keeping its zlib reader and input
// reader between chunks instead of allocating a fresh inflate window (~32 KB) and
// buffer each time.
type chunkInflater struct {
	src bytes.Reader
	zr  io.ReadCloser
}

func (in *chunkInflater) inflate(compressed, dst []byte) error {
	in.src.Reset(compressed)
	var err error
	if in.zr == nil {
		in.zr, err = zlib.NewReader(&in.src)
	} else {
		err = in.zr.(zlib.Resetter).Reset(&in.src, nil)
	}
	if err != nil {
		return err
	}
	if _, err := io.ReadFull(in.zr, dst); err != nil {
		return err
	}
	// zlib only verifies its checksum at end of stream, which ReadFull stops just
	// short of. Read on to the end so damage gets noticed.
	var extra [1]byte
	if _, err := in.zr.Read(extra[:]); err != io.EOF {
		return fmt.Errorf("chunk is longer than its header says, or its checksum is wrong: %v", err)
	}
	return nil
}

func (in *chunkInflater) close() {
	if in.zr != nil {
		in.zr.Close()
	}
}

// TotalUncompressedSize walks the chunk headers - skipping over each chunk's
// compressed data via Seek, without decompressing anything - to sum the
// total uncompressed size, then rewinds back to where the reader started.
// This lets DecompressAll allocate its result once, and is cheap enough to
// estimate memory needs before loading a save.
func (d *Decompressor) TotalUncompressedSize() (int64, error) {
	startPos := d.pos
	var total int64
	for {
		header, err := d.readChunkHeader()
		if err != nil {
			break // EOF here just means we're done counting
		}
		total += int64(header.UncompressedSize)
		if _, err := d.reader.Seek(int64(header.CompressedSize), io.SeekCurrent); err != nil {
			return 0, err
		}
		d.pos += int64(header.CompressedSize)
	}
	if _, err := d.reader.Seek(startPos, io.SeekStart); err != nil {
		return 0, err
	}
	d.pos = startPos
	return total, nil
}

// decompressWorkers is how many chunks we inflate concurrently. Each chunk is
// an independent zlib stream, so this is embarrassingly parallel. It's capped
// because inflating faster than the consumer parses only burns memory.
var decompressWorkers = func() int {
	n := runtime.NumCPU()
	if n > 8 {
		n = 8
	}
	if n < 1 {
		n = 1
	}
	return n
}()

// inflateJob is one chunk handed to a worker.
type inflateJob struct {
	index            int
	compressed       []byte
	uncompressedSize int
	out              chan ChunkResult
}

// StreamChunks hands back a channel that yields decompressed chunks in order.
// Reading the compressed bytes off the file stays sequential (it has to be,
// each chunk header tells us where the next one starts), but the actual
// inflating is spread over a pool of workers, since every chunk is a separate
// zlib stream. Results are re-ordered before they reach the caller, so the
// consumer still sees a plain sequential byte stream.
//
// The channel closes when we run out of chunks or hit an error; an error is
// signalled by a ChunkResult with a nil Data, same as before.
func (d *Decompressor) StreamChunks() (<-chan ChunkResult, error) {
	// Read the first header up front so we can fail fast on a bad stream.
	header, err := d.readChunkHeader()
	if err != nil {
		return nil, err
	}

	ch := make(chan ChunkResult, decompressWorkers)
	jobs := make(chan inflateJob, decompressWorkers)
	// ordered carries one result channel per chunk, in stream order, so the
	// emitter can hand chunks to the caller in the order they appear in the
	// file no matter which worker finishes first. Its capacity bounds how far
	// the reader may run ahead of the consumer.
	ordered := make(chan chan ChunkResult, decompressWorkers*4)

	// Reader: sequentially pull each chunk's compressed bytes off the stream
	// and queue it for inflating.
	go func() {
		defer close(jobs)
		defer close(ordered)
		idx := 0
		for {
			compressedSize := int(header.CompressedSize)
			compressedData := make([]byte, compressedSize)
			if _, err := io.ReadFull(d.reader, compressedData); err != nil {
				failed := make(chan ChunkResult, 1)
				failed <- ChunkResult{Index: idx, Data: nil}
				ordered <- failed
				return
			}
			d.pos += int64(compressedSize)

			out := make(chan ChunkResult, 1)
			ordered <- out
			jobs <- inflateJob{
				index:            idx,
				compressed:       compressedData,
				uncompressedSize: int(header.UncompressedSize),
				out:              out,
			}
			idx++

			// On to the next header.
			header, err = d.readChunkHeader()
			if err != nil {
				// A clean EOF means we're finished; anything else (a bad tag, a
				// header cut short) is a damaged file, not the end of the data.
				if err != io.EOF {
					failed := make(chan ChunkResult, 1)
					failed <- ChunkResult{Index: idx, Data: nil}
					ordered <- failed
				}
				return
			}
		}
	}()

	// Workers: inflate chunks in parallel. Each keeps one zlib reader alive and
	// resets it per chunk instead of allocating a fresh inflate window (~32 KB)
	// every time.
	var wg sync.WaitGroup
	for i := 0; i < decompressWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var in chunkInflater
			defer in.close()
			for job := range jobs {
				decompressed := make([]byte, job.uncompressedSize)
				if err := in.inflate(job.compressed, decompressed); err != nil {
					job.out <- ChunkResult{Index: job.index, Data: nil}
					continue
				}

				job.out <- ChunkResult{
					Index:            job.index,
					CompressedSize:   len(job.compressed),
					UncompressedSize: job.uncompressedSize,
					Data:             decompressed,
				}
			}
		}()
	}

	// Emitter: drain the ordered queue so the caller sees chunks in stream order.
	go func() {
		defer close(ch)
		for out := range ordered {
			result := <-out
			ch <- result
			if result.Data == nil {
				// Failure: drain the rest so the reader and workers can exit.
				go func() {
					for range ordered {
					}
				}()
				wg.Wait()
				return
			}
		}
		wg.Wait()
	}()

	return ch, nil
}
