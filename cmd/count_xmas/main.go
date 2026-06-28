package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"satisfacts/parser"
)

type sliceReader struct {
	data []byte
	pos  int
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

func (s *sliceReader) ReadByte() (byte, error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	b := s.data[s.pos]
	s.pos++
	return b, nil
}

func (s *sliceReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		s.pos = int(offset)
	case io.SeekCurrent:
		s.pos += int(offset)
	case io.SeekEnd:
		s.pos = len(s.data) + int(offset)
	}
	return int64(s.pos), nil
}

func isXmasClass(name string) bool {
	p := strings.ToLower(name)
	return strings.Contains(p, "xmas") || strings.Contains(p, "ficsmas") || strings.Contains(p, "christmas") || strings.Contains(p, "xmass")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: count_xmas <savefile>")
		os.Exit(1)
	}

	fileData, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	headerReader := parser.NewBinaryReader(fileData)
	_, ctx, err := parser.ParseHeader(headerReader)
	if err != nil {
		fmt.Printf("Error parsing header: %v\n", err)
		os.Exit(1)
	}
	headerEnd := headerReader.Position()

	// Decompress entire body
	bodyReader := &sliceReader{data: fileData[headerEnd:]}
	decompressor := parser.NewDecompressor(bodyReader)
	bodyData, err := decompressor.DecompressAll()
	if err != nil {
		fmt.Printf("Error decompressing: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Decompressed body: %.1f MB\n", float64(len(bodyData))/1024/1024)

	// Create fake chunk channel for StreamingReader
	fakeCh := make(chan parser.ChunkResult, 1)
	fakeCh <- parser.ChunkResult{Index: 0, Data: bodyData, UncompressedSize: len(bodyData)}
	close(fakeCh)
	sr := parser.NewStreamingReader(fakeCh)
	bp := parser.NewBodyParser(sr, ctx)

	body, err := bp.ParseBody()
	if err != nil {
		fmt.Printf("Error parsing body: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("TotalSize: %d, SublevelCount: %d\n", body.TotalSize, body.SublevelCount)

	totalXmas := 0
	xmasByClass := map[string]int{}
	xmasByLevel := map[string]int{}

	_, err = bp.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dbl int64, level *parser.Level) error {
		xmasCount := 0
		for _, hdr := range level.ObjectHeaders {
			if isXmasClass(hdr.ClassName) {
				xmasCount++
				xmasByClass[hdr.ClassName]++
			}
		}
		if xmasCount > 0 {
			fmt.Printf("Level %s: %d objects, %d XMas\n", level.Name, len(level.ObjectHeaders), xmasCount)
			xmasByLevel[level.Name] = xmasCount
		}
		totalXmas += xmasCount

		// Skip data blob
		if dbl > 0 {
			return sr.Skip(int(dbl))
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error streaming levels: %v\n", err)
	}

	fmt.Printf("\nTotal XMas objects: %d\n", totalXmas)
	fmt.Println("\nXMas objects by class:")
	for c, n := range xmasByClass {
		fmt.Printf("  %s: %d\n", c, n)
	}
	fmt.Println("\nXMas objects by level:")
	for l, n := range xmasByLevel {
		fmt.Printf("  %s: %d\n", l, n)
	}

	_ = binary.LittleEndian
}
