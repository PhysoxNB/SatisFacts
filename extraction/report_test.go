package extraction

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"satisfacts/parser"
)

func TestOutputThroughput(t *testing.T) {
	if total, label := outputThroughput(nil, false); total != 0 || label != "None" {
		t.Errorf("no outputs: got %v %q", total, label)
	}
	if total, label := outputThroughput([]string{"Pipe Mk2", "Pipe Mk1"}, true); total != 900 || label != "Pipe Mk2, Pipe Mk1" {
		t.Errorf("pipes: got %v %q", total, label)
	}
	if total, label := outputThroughput([]string{"Pipe Mk1"}, false); total != 0 || label != "None" {
		t.Errorf("a pipe isn't a solid output: got %v %q", total, label)
	}
}

func TestGenerateHTMLFillsTemplate(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.html")
	report := ReportData{Header: map[string]interface{}{"SessionName": `</script><b>"x"`}}
	if err := GenerateHTML(report, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, placeholder := range []string{"__EMBEDDED_DATA__", "__MAP_IMAGE__", "__CSS__", "__VIEWER_JS__"} {
		if strings.Contains(html, placeholder) {
			t.Errorf("%s was not replaced", placeholder)
		}
	}
	if strings.Contains(html, "</script><b>") {
		t.Error("save data must stay encoded, not appear as raw HTML")
	}
}

// Runs a real save through the parser and extraction when one is available.
func TestExtractionOnRealSave(t *testing.T) {
	dir := os.Getenv("SATISFACTS_FIXTURES")
	if dir == "" {
		dir = ".."
	}
	path := filepath.Join(dir, "Save1.2.sav")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("fixture not available: ", path)
	}

	headerReader := parser.NewBinaryReader(data)
	_, ctx, err := parser.ParseHeader(headerReader)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint32(data[headerReader.Position():]) != 0x9E2A83C1 {
		t.Fatal("no chunk tag after the header")
	}
	var body io.ReadSeeker = bytes.NewReader(data[headerReader.Position():])
	chunks, err := parser.NewDecompressor(body).StreamChunks()
	if err != nil {
		t.Fatal(err)
	}
	sr := parser.NewStreamingReader(chunks)
	bp := parser.NewBodyParser(sr, ctx)
	parsed, err := bp.ParseBody()
	if err != nil {
		t.Fatal(err)
	}

	if err := LoadRecipes(""); err != nil {
		t.Fatal(err)
	}
	if err := LoadDataFiles(""); err != nil {
		t.Fatal(err)
	}
	ext := NewExtractedData(true)
	objects := 0
	_, err = bp.StreamLevels(parsed.SublevelCount, func(sr *parser.StreamingReader, length int64, level *parser.Level) error {
		ext.ProcessCollectables1(level.Collectables1)
		if level.ObjectHeaderCount == 0 || length == 0 {
			return sr.Skip(int(length))
		}
		dbp := parser.NewDataBlobParser(ctx, level.ObjectHeaders, "DEEP")
		_, err := dbp.StreamObjects(sr, length, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, _ int) {
			objects++
			ext.ProcessObject(obj, true)
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if objects == 0 || len(ext.BuildingFacts) == 0 {
		t.Errorf("expected objects and buildings, got %d objects, %d buildings", objects, len(ext.BuildingFacts))
	}
}
