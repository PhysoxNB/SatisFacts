package editor

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"satisfacts/parser"
)

// SliceReader implements io.ReadSeeker over a byte slice.
type SliceReader struct {
	Data []byte
	Pos  int
}

func (s *SliceReader) Read(p []byte) (int, error) {
	if s.Pos >= len(s.Data) {
		return 0, io.EOF
	}
	n := copy(p, s.Data[s.Pos:])
	s.Pos += n
	return n, nil
}

func (s *SliceReader) Seek(offset int64, whence int) (int64, error) {
	var newPos int
	switch whence {
	case io.SeekStart:
		newPos = int(offset)
	case io.SeekCurrent:
		newPos = s.Pos + int(offset)
	case io.SeekEnd:
		newPos = len(s.Data) + int(offset)
	}
	if newPos < 0 || newPos > len(s.Data) {
		return 0, fmt.Errorf("seek out of bounds: %d", newPos)
	}
	s.Pos = newPos
	return int64(newPos), nil
}

// SaveData holds the raw and decompressed state of a save file.
type SaveData struct {
	FileData     []byte
	HeaderBytes  []byte // header bytes saved by FreeFileData, used by WriteSave
	HeaderEnd    int
	IsV2         bool
	MaxChunkSize int32
	Decompressed []byte
	VersionCtx   *parser.VersionContext
}

// FreeFileData copies the header bytes and releases the full compressed file
// data so the GC can reclaim that memory. Use this after decompression when
// you only need the header for writing output (e.g. optimize).
func (sd *SaveData) FreeFileData() {
	if sd.FileData == nil {
		return
	}
	if sd.HeaderBytes == nil {
		header := make([]byte, sd.HeaderEnd)
		copy(header, sd.FileData[:sd.HeaderEnd])
		sd.HeaderBytes = header
	}
	sd.FileData = nil
	runtime.GC()
}

// SaveHeader holds just the header fields without decompressing the body.
type SaveHeader struct {
	Header       *parser.SaveHeader
	HeaderEnd    int
	IsV2         bool
	MaxChunkSize int32
	VersionCtx   *parser.VersionContext
}

// LoadSaveHeader reads a save file's header and chunk header without decompressing the body.
// This is much faster than LoadSave for cases that only need header fields (e.g. session name).
func LoadSaveHeader(savePath string) (*SaveHeader, error) {
	fileData, err := os.ReadFile(savePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	headerReader := parser.NewBinaryReader(fileData)
	header, ctx, err := parser.ParseHeader(headerReader)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}
	headerEnd := headerReader.Position()

	bodyStart := headerEnd
	if bodyStart+12 > len(fileData) {
		return nil, fmt.Errorf("file too small for chunk header")
	}
	chunkTag := binary.LittleEndian.Uint32(fileData[bodyStart:])
	if chunkTag != 0x9E2A83C1 {
		return nil, fmt.Errorf("invalid chunk tag: 0x%X", chunkTag)
	}
	chunkVersion := binary.LittleEndian.Uint32(fileData[bodyStart+4:])
	isV2 := chunkVersion == 0x22222222
	maxChunkSize := int32(binary.LittleEndian.Uint32(fileData[bodyStart+8:]))

	return &SaveHeader{
		Header:       header,
		HeaderEnd:    headerEnd,
		IsV2:         isV2,
		MaxChunkSize: maxChunkSize,
		VersionCtx:   ctx,
	}, nil
}

// LoadSave reads a save file, parses the header, and decompresses the body.
func LoadSave(savePath string) (*SaveData, error) {
	fileData, err := os.ReadFile(savePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	headerReader := parser.NewBinaryReader(fileData)
	_, ctx, err := parser.ParseHeader(headerReader)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}
	headerEnd := headerReader.Position()

	bodyStart := headerEnd
	if bodyStart+12 > len(fileData) {
		return nil, fmt.Errorf("file too small for chunk header")
	}
	chunkTag := binary.LittleEndian.Uint32(fileData[bodyStart:])
	if chunkTag != 0x9E2A83C1 {
		return nil, fmt.Errorf("invalid chunk tag: 0x%X", chunkTag)
	}
	chunkVersion := binary.LittleEndian.Uint32(fileData[bodyStart+4:])
	isV2 := chunkVersion == 0x22222222
	maxChunkSize := int32(binary.LittleEndian.Uint32(fileData[bodyStart+8:]))

	bodyReader := &SliceReader{Data: fileData[headerEnd:], Pos: 0}
	decompressor := parser.NewDecompressor(bodyReader)
	decompressed, err := decompressor.DecompressAll()
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}

	return &SaveData{
		FileData:     fileData,
		HeaderEnd:    headerEnd,
		IsV2:         isV2,
		MaxChunkSize: maxChunkSize,
		Decompressed: decompressed,
		VersionCtx:   ctx,
	}, nil
}

// RecompressBody recompresses a decompressed body into chunk format.
func RecompressBody(decompressed []byte, isV2 bool, maxChunkSize int32) []byte {
	var compressed bytes.Buffer
	// Writing to a bytes.Buffer can't fail.
	_ = RecompressBodyToWriter(decompressed, isV2, maxChunkSize, &compressed)
	return compressed.Bytes()
}

// RecompressBodyToWriter recompresses a decompressed body and streams each
// compressed chunk directly to w, avoiding holding the full compressed output
// in memory.
func RecompressBodyToWriter(decompressed []byte, isV2 bool, maxChunkSize int32, w io.Writer) error {
	cw := newChunkWriter(w, isV2, maxChunkSize)
	if _, err := cw.Write(decompressed); err != nil {
		return err
	}
	return cw.Close()
}

// chunkWriter turns a stream of decompressed body bytes into compressed save
// chunks. A save has tens of thousands of chunks, so it reuses one compressor and
// its buffers: allocating a new zlib writer (over 1 MB) per chunk made the garbage
// collector let the heap grow to several times the size of the save.
type chunkWriter struct {
	w            io.Writer
	isV2         bool
	maxChunkSize int32
	chunk        []byte
	compressed   bytes.Buffer
	out          bytes.Buffer
	zw           *zlib.Writer
}

func newChunkWriter(w io.Writer, isV2 bool, maxChunkSize int32) *chunkWriter {
	chunkSize := int(maxChunkSize)
	if chunkSize <= 0 {
		chunkSize = 64 * 1024
	}
	cw := &chunkWriter{w: w, isV2: isV2, maxChunkSize: maxChunkSize, chunk: make([]byte, 0, chunkSize)}
	cw.zw = zlib.NewWriter(&cw.compressed)
	return cw
}

func (cw *chunkWriter) Write(p []byte) (int, error) {
	written := len(p)
	for len(p) > 0 {
		n := min(cap(cw.chunk)-len(cw.chunk), len(p))
		cw.chunk = append(cw.chunk, p[:n]...)
		p = p[n:]
		if len(cw.chunk) == cap(cw.chunk) {
			if err := cw.flush(); err != nil {
				return 0, err
			}
		}
	}
	return written, nil
}

// Close writes out the final, partly filled chunk.
func (cw *chunkWriter) Close() error {
	if len(cw.chunk) == 0 {
		return nil
	}
	return cw.flush()
}

func (cw *chunkWriter) flush() error {
	cw.compressed.Reset()
	cw.zw.Reset(&cw.compressed)
	if _, err := cw.zw.Write(cw.chunk); err != nil {
		return err
	}
	if err := cw.zw.Close(); err != nil {
		return err
	}
	cw.out.Reset()
	writeChunk(&cw.out, cw.compressed.Bytes(), cw.chunk, cw.isV2, cw.maxChunkSize)
	cw.chunk = cw.chunk[:0]
	_, err := cw.w.Write(cw.out.Bytes())
	return err
}

// writeChunk writes a single chunk header + compressed data to a bytes.Buffer.
func writeChunk(buf *bytes.Buffer, compData, chunk []byte, isV2 bool, maxChunkSize int32) {
	binary.Write(buf, binary.LittleEndian, uint32(0x9E2A83C1))
	if isV2 {
		binary.Write(buf, binary.LittleEndian, uint32(0x22222222))
	} else {
		binary.Write(buf, binary.LittleEndian, uint32(0x00000000))
	}
	binary.Write(buf, binary.LittleEndian, maxChunkSize)
	buf.Write([]byte{0, 0, 0, 0})
	if isV2 {
		buf.WriteByte(3)
	}
	binary.Write(buf, binary.LittleEndian, int32(len(compData)))
	buf.Write([]byte{0, 0, 0, 0})
	binary.Write(buf, binary.LittleEndian, int32(len(chunk)))
	buf.Write([]byte{0, 0, 0, 0})
	binary.Write(buf, binary.LittleEndian, int32(len(compData)))
	buf.Write([]byte{0, 0, 0, 0})
	binary.Write(buf, binary.LittleEndian, int32(len(chunk)))
	buf.Write([]byte{0, 0, 0, 0})
	buf.Write(compData)
}

// WriteSave writes a save file from the original header + recompressed body.
// It streams compressed chunks directly to the output file to avoid holding
// the full compressed output in memory.
func WriteSave(sd *SaveData, outputPath string) error {
	return writeSaveFile(sd, outputPath, func(w io.Writer) error {
		return RecompressBodyToWriter(sd.Decompressed, sd.IsV2, sd.MaxChunkSize, w)
	})
}

// writeSaveFile writes the save header, then lets writeBody stream the compressed
// body. The save goes to a temporary file next to outputPath and only replaces it
// once fully written and flushed to disk. A full disk or crash therefore can't
// damage an existing save (the session fix can overwrite the original) or leave a
// half-written one behind.
func writeSaveFile(sd *SaveData, outputPath string, writeBody func(io.Writer) error) (err error) {
	// Write header: prefer HeaderBytes (set by FreeFileData), fall back to FileData
	header := sd.HeaderBytes
	if header == nil && sd.FileData != nil {
		header = sd.FileData[:sd.HeaderEnd]
	}
	if header == nil {
		return fmt.Errorf("no header bytes available (FreeFileData was called but HeaderBytes is nil)")
	}

	mode := os.FileMode(0644)
	if info, statErr := os.Stat(outputPath); statErr == nil {
		mode = info.Mode().Perm()
	}

	// The .tmp suffix also keeps the auto-detect watcher from picking it up.
	outFile, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
	}
	tmpPath := outFile.Name()
	defer func() {
		if err != nil {
			outFile.Close()
			os.Remove(tmpPath)
		}
	}()

	if _, err := outFile.Write(header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if err := writeBody(outFile); err != nil {
		return fmt.Errorf("writing body: %w", err)
	}
	if err := outFile.Sync(); err != nil {
		return fmt.Errorf("flushing output: %w", err)
	}
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("closing output: %w", err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("setting output permissions: %w", err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("replacing output: %w", err)
	}
	return nil
}
