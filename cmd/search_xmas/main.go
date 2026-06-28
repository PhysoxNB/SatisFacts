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

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: search_xmas <savefile>")
		os.Exit(1)
	}
	inputPath := os.Args[1]

	fileData, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	headerReader := parser.NewBinaryReader(fileData)
	_, _, err = parser.ParseHeader(headerReader)
	if err != nil {
		fmt.Printf("Error parsing header: %v\n", err)
		os.Exit(1)
	}
	headerEnd := headerReader.Position()

	bodyReader := &sliceReader{data: fileData[headerEnd:]}
	decompressor := parser.NewDecompressor(bodyReader)
	bodyData, err := decompressor.DecompressAll()
	if err != nil {
		fmt.Printf("Error decompressing: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Decompressed body: %d bytes (%.1f MB)\n", len(bodyData), float64(len(bodyData))/1024/1024)

	// Search for all xmas/ficsmas/christmas strings
	patterns := []string{"xmas", "Xmas", "XMAS", "ficsmas", "Ficsmas", "FICSMAS", "christmas", "Christmas", "CHRISTMAS", "xmass", "Xmass"}

	for _, p := range patterns {
		pb := []byte(p)
		start := 0
		count := 0
		for {
			idx := indexOf(bodyData, pb, start)
			if idx == -1 {
				break
			}
			count++
			if count <= 30 {
				// Get context: try to read as FString (length-prefixed)
				context := getContext(bodyData, idx)
				fmt.Printf("[%s] at offset %d (0x%x): %s\n", p, idx, idx, context)
			}
			start = idx + 1
		}
		if count > 30 {
			fmt.Printf("[%s] ... and %d more\n", p, count-30)
		}
		if count > 0 {
			fmt.Printf("[%s] total: %d occurrences\n\n", p, count)
		}
	}

	// Also search for the specific crash-related pattern: look for int32 value 17 near arrays
	// The crash is "index 17 into array of size 14"
	// Let's search for the byte pattern 0x11 0x00 0x00 0x00 (int32 17) in the GameState area
	fmt.Println("\n=== Searching for GameState object ===")
	// Find BP_GameState in bodyData
	gsIdx := indexOf(bodyData, []byte("BP_GameState"), 0)
	if gsIdx != -1 {
		fmt.Printf("BP_GameState found at offset %d (0x%x)\n", gsIdx, gsIdx)
	}
}

func indexOf(data, pattern []byte, start int) int {
	for i := start; i <= len(data)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func getContext(data []byte, idx int) string {
	// Try to find the start of the FString (go back to find length prefix)
	// FString: int32 length + bytes
	// Try reading 4 bytes before the match as length
	for back := 1; back <= 200; back++ {
		if idx-back < 4 {
			break
		}
		length := int32(binary.LittleEndian.Uint32(data[idx-back:]))
		if length > 0 && length < 1000 {
			// Check if length covers our match
			strStart := idx - back + 4
			strEnd := strStart + int(length)
			if strEnd <= len(data) && strEnd > idx {
				s := string(data[strStart:strEnd])
				// Clean up null terminators
				s = strings.TrimRight(s, "\x00")
				return fmt.Sprintf("FString(len=%d, back=%d): %s", length, back, s)
			}
		}
	}
	// Just return raw context
	start := idx - 20
	if start < 0 {
		start = 0
	}
	end := idx + 60
	if end > len(data) {
		end = len(data)
	}
	return fmt.Sprintf("raw bytes: %x", data[start:end])
}
