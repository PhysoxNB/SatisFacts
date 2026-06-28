package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

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
	if len(os.Args) < 2 {
		fmt.Println("Usage: scan_colorslot <savefile>")
		os.Exit(1)
	}

	fileData, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	headerReader := parser.NewBinaryReader(fileData)
	_, ctx, err := parser.ParseHeader(headerReader)
	if err != nil {
		fmt.Printf("Error parsing header: %v\n", err)
		os.Exit(1)
	}

	headerEnd := headerReader.Position()
	bodyData := fileData[headerEnd:]
	bodyReader := &sliceReader{data: bodyData, pos: 0}

	decompressor := parser.NewDecompressor(bodyReader)
	chunkCh, err := decompressor.StreamChunks()
	if err != nil {
		fmt.Printf("Error setting up decompressor: %v\n", err)
		os.Exit(1)
	}

	sr := parser.NewStreamingReader(chunkCh)
	bp := parser.NewBodyParser(sr, ctx)

	body, err := bp.ParseBody()
	if err != nil {
		fmt.Printf("Error parsing body: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("SublevelCount: %d\n", body.SublevelCount)

	slotCounts := map[int32]int{}
	var highSlots []struct {
		className string
		slot      int32
		ref       string
	}

	_, err = bp.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dbl int64, level *parser.Level) error {
		if dbl <= 0 {
			return nil
		}

		dbp := parser.NewDataBlobParser(ctx, level.ObjectHeaders, "")
		_, err := dbp.StreamObjects(sr, dbl, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, idx int) {
			if obj.Header == nil {
				return
			}
			className := obj.Header.ClassName

			// Check for mColorSlot property
			if prop, ok := obj.Properties["mColorSlot"]; ok {
				var slot int32
				switch v := prop.Value.(type) {
				case int32:
					slot = v
				case int:
					slot = int32(v)
				case int64:
					slot = int32(v)
				}
				slotCounts[slot]++
				if slot >= 14 {
					highSlots = append(highSlots, struct {
						className string
						slot      int32
						ref       string
					}{className, slot, obj.Header.Reference.PathName})
				}
			}
		})
		return err
	})

	if err != nil {
		fmt.Printf("Error streaming levels: %v\n", err)
	}

	fmt.Println("\n=== mColorSlot distribution ===")
	keys := make([]int32, 0, len(slotCounts))
	for k := range slotCounts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		marker := ""
		if k >= 14 {
			marker = " *** OUT OF BOUNDS (>=14) ***"
		}
		fmt.Printf("  Slot %d: %d objects%s\n", k, slotCounts[k], marker)
	}

	fmt.Printf("\n=== Objects with mColorSlot >= 14 (total: %d) ===\n", len(highSlots))
	for _, h := range highSlots {
		fmt.Printf("  %s (slot=%d) %s\n", h.className, h.slot, h.ref)
	}

	// Also check for mSwatchSlot or similar
	_ = strings.Contains
}
