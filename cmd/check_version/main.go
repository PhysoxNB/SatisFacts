package main

import (
	"fmt"
	"os"

	"satisfacts/parser"
)

type sliceReader struct {
	data []byte
	pos  int
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

func (s *sliceReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		s.pos = int(offset)
	case 1:
		s.pos += int(offset)
	case 2:
		s.pos = len(s.data) + int(offset)
	}
	return int64(s.pos), nil
}

func main() {
	fileData, _ := os.ReadFile(os.Args[1])
	headerReader := parser.NewBinaryReader(fileData)
	header, ctx, _ := parser.ParseHeader(headerReader)
	fmt.Printf("SaveVersion: %d\n", header.SaveVersion)
	fmt.Printf("LevelVersion: %d\n", ctx.LevelVersion)
	fmt.Printf("ObjectVersion: %d\n", ctx.ObjectVersion)
	fmt.Printf("PackageFileVerUE5: %d\n", ctx.PackageFileVerUE5)
	fmt.Printf("useCompleteTagType would be: %v\n", ctx.ObjectVersion >= 53 && ctx.PackageFileVerUE5 >= 1012)
}
