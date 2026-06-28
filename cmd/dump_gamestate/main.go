package main

import (
	"encoding/json"
	"fmt"
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
		fmt.Println("Usage: dump_gamestate <savefile>")
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

	_, err = bp.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dbl int64, level *parser.Level) error {
		if dbl <= 0 {
			return nil
		}

		// Check if this is the persistent level
		if !level.IsPersistent {
			return sr.Skip(int(dbl))
		}

		fmt.Printf("\n=== Persistent Level ===\n")
		fmt.Printf("ObjectHeaderCount: %d\n", level.ObjectHeaderCount)

		dbp := parser.NewDataBlobParser(ctx, level.ObjectHeaders, "")
		_, err := dbp.StreamObjects(sr, dbl, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, idx int) {
			if obj.Header == nil {
				return
			}
			className := obj.Header.ClassName

			cl := strings.ToLower(className)

			// For SchematicManager: dump all purchased schematics, look for XMas/FICSMAS
			if strings.Contains(cl, "schematicmanager") {
				fmt.Printf("\n--- SchematicManager (Object %d) ---\n", idx)
				fmt.Printf("BinarySize: %d\n", obj.BinarySize)
				if schematics, ok := obj.Properties["mPurchasedSchematics"]; ok {
					schemaJSON, _ := json.Marshal(schematics)
					var schemaData struct {
						Value struct {
							Count int `json:"count"`
							Items []struct {
								LevelName string `json:"levelName"`
								PathName  string `json:"pathName"`
							} `json:"items"`
						} `json:"Value"`
					}
					json.Unmarshal(schemaJSON, &schemaData)
					fmt.Printf("mPurchasedSchematics count: %d\n", schemaData.Value.Count)
					xmasCount := 0
					for i, item := range schemaData.Value.Items {
						pl := strings.ToLower(item.PathName)
						if strings.Contains(pl, "xmas") || strings.Contains(pl, "ficsmas") || strings.Contains(pl, "xmass") {
							fmt.Printf("  [%d] XMAS: %s\n", i, item.PathName)
							xmasCount++
						}
					}
					fmt.Printf("  Total XMas/FICSMAS schematics: %d\n", xmasCount)
				}
			}

			// For GamePhaseManager: dump full details
			if strings.Contains(cl, "gamephasemanager") {
				fmt.Printf("\n--- GamePhaseManager (Object %d) ---\n", idx)
				fmt.Printf("BinarySize: %d\n", obj.BinarySize)
				for k, v := range obj.Properties {
					valJSON, _ := json.Marshal(v)
					fmt.Printf("  %s = %s\n", k, string(valJSON))
				}
			}

			// For ResearchManager: dump full details
			if strings.Contains(cl, "researchmanager") {
				fmt.Printf("\n--- ResearchManager (Object %d) ---\n", idx)
				fmt.Printf("BinarySize: %d\n", obj.BinarySize)
				for k, v := range obj.Properties {
					valJSON, _ := json.Marshal(v)
					fmt.Printf("  %s = %s\n", k, string(valJSON))
				}
			}

			// For GameState: dump arrays with counts
			if strings.Contains(cl, "gamestate") {
				fmt.Printf("\n--- GameState (Object %d) ---\n", idx)
				fmt.Printf("BinarySize: %d, Properties: %d\n", obj.BinarySize, len(obj.Properties))
				for k, v := range obj.Properties {
					valJSON, _ := json.Marshal(v)
					valStr := string(valJSON)
					// Show full value for arrays and short properties
					if strings.HasPrefix(valStr, "{\"Type\":\"ArrayProperty") || len(valStr) < 500 {
						fmt.Printf("  %s = %s\n", k, valStr)
					} else {
						fmt.Printf("  %s = %s...(truncated)\n", k, valStr[:200])
					}
				}
			}
		})
		return err
	})

	if err != nil {
		fmt.Printf("Error streaming levels: %v\n", err)
	}
}
