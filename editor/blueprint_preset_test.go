package editor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Pure-function tests for preset file parsing (no fixtures needed) ---

func TestParsePresetCategoriesValid(t *testing.T) {
	meta := struct {
		Categories []BlueprintCategoryRecord `json:"categories"`
	}{
		Categories: []BlueprintCategoryRecord{
			{CategoryName: "Alpha", IconID: 1, MenuPriority: 0.5},
			{CategoryName: "Beta", IconID: 2, MenuPriority: 1.0},
		},
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}

	got, err := parsePresetCategories(metaBytes, 0)
	if err != nil {
		t.Fatalf("parsePresetCategories: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d categories, want 2", len(got))
	}
	if got[0].CategoryName != "Alpha" || got[1].CategoryName != "Beta" {
		t.Errorf("names = %q, %q, want Alpha, Beta", got[0].CategoryName, got[1].CategoryName)
	}
}

func TestParsePresetCategoriesEmpty(t *testing.T) {
	emptyJSON := []byte(`{"categories":[]}`)
	got, err := parsePresetCategories(emptyJSON, 0)
	if err != nil {
		t.Fatalf("parsePresetCategories: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d categories, want 0", len(got))
	}
}

func TestParsePresetCategoriesInvalidJSON(t *testing.T) {
	bad := []byte("not json at all")
	_, err := parsePresetCategories(bad, 0)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParsePresetCategoriesNoFooter(t *testing.T) {
	_, err := parsePresetCategories([]byte{1, 2, 3}, 10)
	if err == nil {
		t.Fatal("expected error when jsonStart is past end of data, got nil")
	}
}

// buildCorruptPreset builds a byte slice that looks like a preset file but has
// the given magic, flag, propBinSize, and raw payload. Used for edge-case tests.
func buildCorruptPreset(magic []byte, useCompleteTag byte, propBinSize uint32, payload []byte) []byte {
	var buf bytes.Buffer
	buf.Write(magic)
	buf.WriteByte(useCompleteTag)
	binary.Write(&buf, binary.LittleEndian, propBinSize)
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.Write(payload)
	// Minimal JSON footer
	buf.Write([]byte(`{"categories":[]}`))
	return buf.Bytes()
}

func TestImportBlueprintPresetMagicMismatch(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "bad.sfbcat")
	badMagic := []byte("XXXXXXXX")
	data := buildCorruptPreset(badMagic, 0, 0, nil)
	if err := os.WriteFile(presetPath, data, 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	// We don't need a real target save — the magic check happens before loading it.
	_, _, err := ImportBlueprintPreset(presetPath, "nonexistent.sav", "replace")
	if err == nil {
		t.Fatal("expected error for magic mismatch, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("magic")) {
		t.Errorf("error should mention magic, got: %v", err)
	}
}

func TestImportBlueprintPresetTooSmall(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "tiny.sfbcat")
	// 16 bytes — one short of the 17-byte minimum header
	if err := os.WriteFile(presetPath, make([]byte, 16), 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	_, _, err := ImportBlueprintPreset(presetPath, "nonexistent.sav", "replace")
	if err == nil {
		t.Fatal("expected error for too-small file, got nil")
	}
}

func TestImportBlueprintPresetNegativePayload(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "neg.sfbcat")

	var buf bytes.Buffer
	buf.Write(presetMagic)
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, uint32(0))          // propBinSize
	binary.Write(&buf, binary.LittleEndian, uint32(0xFFFFFFFF)) // rawDataLen = -1 as int32
	buf.Write([]byte(`{"categories":[]}`))

	if err := os.WriteFile(presetPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	_, _, err := ImportBlueprintPreset(presetPath, "nonexistent.sav", "replace")
	if err == nil {
		t.Fatal("expected error for negative payload length, got nil")
	}
}

func TestImportBlueprintPresetTruncated(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "trunc.sfbcat")

	// Declare 100 bytes of payload but only provide 10
	var buf bytes.Buffer
	buf.Write(presetMagic)
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(100))
	buf.Write(make([]byte, 10))

	if err := os.WriteFile(presetPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	_, _, err := ImportBlueprintPreset(presetPath, "nonexistent.sav", "replace")
	if err == nil {
		t.Fatal("expected error for truncated file, got nil")
	}
}

func TestImportBlueprintPresetNonExistentFile(t *testing.T) {
	_, _, err := ImportBlueprintPreset("does-not-exist.sfbcat", "also-not-exist.sav", "replace")
	if err == nil {
		t.Fatal("expected error for non-existent preset file, got nil")
	}
}

func TestReadPresetCategoriesNonExistentFile(t *testing.T) {
	_, err := ReadPresetCategories("does-not-exist.sfbcat")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestReadPresetCategoriesMagicMismatch(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "bad.sfbcat")
	data := buildCorruptPreset([]byte("XXXXXXXX"), 0, 0, nil)
	if err := os.WriteFile(presetPath, data, 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	_, err := ReadPresetCategories(presetPath)
	if err == nil {
		t.Fatal("expected error for magic mismatch, got nil")
	}
}

func TestReadPresetCategoriesTooSmall(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "tiny.sfbcat")
	if err := os.WriteFile(presetPath, make([]byte, 10), 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	_, err := ReadPresetCategories(presetPath)
	if err == nil {
		t.Fatal("expected error for too-small file, got nil")
	}
}

func TestReadPresetCategoriesNegativePayload(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "neg.sfbcat")

	var buf bytes.Buffer
	buf.Write(presetMagic)
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(0xFFFFFFFF))
	buf.Write([]byte(`{"categories":[]}`))

	if err := os.WriteFile(presetPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	_, err := ReadPresetCategories(presetPath)
	if err == nil {
		t.Fatal("expected error for negative payload, got nil")
	}
}

func TestReadPresetCategoriesTruncated(t *testing.T) {
	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "trunc.sfbcat")

	// Valid header but no JSON footer (rawDataLen points past end of file)
	var buf bytes.Buffer
	buf.Write(presetMagic)
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(50))
	buf.Write(make([]byte, 10)) // less than declared 50

	if err := os.WriteFile(presetPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("writing preset: %v", err)
	}

	_, err := ReadPresetCategories(presetPath)
	if err == nil {
		t.Fatal("expected error for truncated file, got nil")
	}
}

// --- Fixture-backed tests ---

// TestExportBlueprintPreset writes a preset from a save that has a blueprint
// subsystem and verifies the file structure and category contents.
func TestExportBlueprintPreset(t *testing.T) {
	for _, name := range []string{fxSave12, fxBlueprints} {
		t.Run(name, func(t *testing.T) {
			savePath := fixture(t, name)

			tmp := t.TempDir()
			presetPath := filepath.Join(tmp, "export.sfbcat")

			cats, err := ExportBlueprintPreset(savePath, presetPath)
			if err != nil {
				t.Fatalf("ExportBlueprintPreset: %v", err)
			}
			if len(cats) == 0 {
				t.Skipf("%s has no blueprint categories", name)
			}

			// Verify the file exists and has the right structure.
			fileData, err := os.ReadFile(presetPath)
			if err != nil {
				t.Fatalf("reading preset: %v", err)
			}
			if len(fileData) < 17 {
				t.Fatalf("preset file is only %d bytes", len(fileData))
			}
			if !bytes.Equal(fileData[:8], presetMagic) {
				t.Errorf("magic mismatch: got %q, want %q", fileData[:8], presetMagic)
			}

			rawDataLen := int32(binary.LittleEndian.Uint32(fileData[13:17]))
			if rawDataLen <= 0 {
				t.Errorf("raw data length = %d, want > 0", rawDataLen)
			}
			if len(fileData) < 17+int(rawDataLen) {
				t.Errorf("file is truncated: declared %d payload bytes but file is only %d",
					rawDataLen, len(fileData))
			}

			// The JSON footer must parse and match the returned categories.
			jsonStart := 17 + int(rawDataLen)
			if jsonStart >= len(fileData) {
				t.Fatalf("no JSON footer in preset file")
			}
			var meta struct {
				Categories []BlueprintCategoryRecord `json:"categories"`
			}
			if err := json.Unmarshal(fileData[jsonStart:], &meta); err != nil {
				t.Fatalf("parsing JSON footer: %v", err)
			}
			if len(meta.Categories) != len(cats) {
				t.Errorf("JSON footer has %d categories, ExportBlueprintPreset returned %d",
					len(meta.Categories), len(cats))
			}
			for i, c := range cats {
				if meta.Categories[i].CategoryName != c.CategoryName {
					t.Errorf("category %d: JSON says %q, function returned %q",
						i, meta.Categories[i].CategoryName, c.CategoryName)
				}
			}
		})
	}
}

// TestExportImportPresetRoundTrip exports a preset from one save and imports it
// into another with replace mode, then verifies the target has the source's categories.
func TestExportImportPresetRoundTrip(t *testing.T) {
	sourcePath := fixture(t, fxSave12)
	targetPath := fixture(t, fxBlueprints)

	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "roundtrip.sfbcat")

	srcCats, err := ExportBlueprintPreset(sourcePath, presetPath)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(srcCats) == 0 {
		t.Skip("source save has no blueprint categories")
	}

	result, targetSD, err := ImportBlueprintPreset(presetPath, targetPath, "replace")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Mode != "replace" {
		t.Errorf("mode = %q, want replace", result.Mode)
	}
	if result.SourceCategories != len(srcCats) {
		t.Errorf("source categories = %d, want %d", result.SourceCategories, len(srcCats))
	}
	if result.FinalCategories != len(srcCats) {
		t.Errorf("final categories = %d, want %d (replace should have same count as source)",
			result.FinalCategories, len(srcCats))
	}

	assertBodyIntact(t, targetSD, "after preset import (replace)")

	reloaded := writeAndReload(t, targetSD)
	assertBodyIntact(t, reloaded, "after preset import + reload")

	// Verify the categories survived the write/reload cycle.
	info, err := FindBlueprintSubsystem(reloaded)
	if err != nil {
		t.Fatalf("blueprint subsystem missing after import: %v", err)
	}
	got, err := ExtractCategoryRecords(info, reloaded)
	if err != nil {
		t.Fatalf("extracting categories after import: %v", err)
	}
	if len(got) != len(srcCats) {
		t.Errorf("after reload: %d categories, want %d", len(got), len(srcCats))
	}

	// In replace mode, every source category must be present.
	srcNames := map[string]bool{}
	for _, c := range srcCats {
		srcNames[c.CategoryName] = true
	}
	for _, c := range got {
		if !srcNames[c.CategoryName] {
			t.Errorf("unexpected category %q after replace import", c.CategoryName)
		}
	}
}

// TestExportImportPresetMerge exports from source and merges into target,
// verifying that target categories are preserved and new ones are added.
func TestExportImportPresetMerge(t *testing.T) {
	sourcePath := fixture(t, fxSave12)
	targetPath := fixture(t, fxBlueprints)

	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "merge.sfbcat")

	srcCats, err := ExportBlueprintPreset(sourcePath, presetPath)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(srcCats) == 0 {
		t.Skip("source save has no blueprint categories")
	}

	// Load target to know its original categories.
	targetSD := loadFixture(t, fxBlueprints)
	tgtInfo, err := FindBlueprintSubsystem(targetSD)
	if err != nil {
		t.Skipf("no blueprint subsystem in target: %v", err)
	}
	tgtCats, err := ExtractCategoryRecords(tgtInfo, targetSD)
	if err != nil {
		t.Fatalf("extracting target categories: %v", err)
	}

	result, importedSD, err := ImportBlueprintPreset(presetPath, targetPath, "merge")
	if err != nil {
		t.Fatalf("import merge: %v", err)
	}
	if result.Mode != "merge" {
		t.Errorf("mode = %q, want merge", result.Mode)
	}

	// Merge: final = target + source categories not already in target.
	tgtNames := map[string]bool{}
	for _, c := range tgtCats {
		tgtNames[c.CategoryName] = true
	}
	expectedNew := 0
	for _, c := range srcCats {
		if !tgtNames[c.CategoryName] {
			expectedNew++
		}
	}
	wantFinal := len(tgtCats) + expectedNew
	if result.FinalCategories != wantFinal {
		t.Errorf("final categories = %d, want %d (%d target + %d new)",
			result.FinalCategories, wantFinal, len(tgtCats), expectedNew)
	}

	assertBodyIntact(t, importedSD, "after preset import (merge)")

	reloaded := writeAndReload(t, importedSD)
	assertBodyIntact(t, reloaded, "after preset merge + reload")

	info, err := FindBlueprintSubsystem(reloaded)
	if err != nil {
		t.Fatalf("blueprint subsystem missing after merge: %v", err)
	}
	got, err := ExtractCategoryRecords(info, reloaded)
	if err != nil {
		t.Fatalf("extracting categories after merge: %v", err)
	}
	if len(got) != wantFinal {
		t.Errorf("after reload: %d categories, want %d", len(got), wantFinal)
	}

	// All original target categories must survive a merge.
	gotNames := map[string]bool{}
	for _, c := range got {
		gotNames[c.CategoryName] = true
	}
	for _, c := range tgtCats {
		if !gotNames[c.CategoryName] {
			t.Errorf("target category %q was lost in merge", c.CategoryName)
		}
	}
}

// TestReadPresetCategoriesFromExport verifies that ReadPresetCategories reads
// back the same categories that ExportBlueprintPreset wrote.
func TestReadPresetCategoriesFromExport(t *testing.T) {
	savePath := fixture(t, fxSave12)

	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "read.sfbcat")

	exported, err := ExportBlueprintPreset(savePath, presetPath)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(exported) == 0 {
		t.Skip("source save has no blueprint categories")
	}

	read, err := ReadPresetCategories(presetPath)
	if err != nil {
		t.Fatalf("ReadPresetCategories: %v", err)
	}
	if len(read) != len(exported) {
		t.Fatalf("ReadPresetCategories returned %d categories, export returned %d",
			len(read), len(exported))
	}
	for i, c := range exported {
		if read[i].CategoryName != c.CategoryName {
			t.Errorf("category %d: read %q, exported %q",
				i, read[i].CategoryName, c.CategoryName)
		}
		if read[i].IconID != c.IconID {
			t.Errorf("category %d: iconID %d vs %d",
				i, read[i].IconID, c.IconID)
		}
	}
}

// TestExportImportPresetSameSaveReplace verifies that exporting from a save and
// importing back into the same save with replace produces identical categories.
func TestExportImportPresetSameSaveReplace(t *testing.T) {
	savePath := fixture(t, fxSave12)

	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "self.sfbcat")

	srcCats, err := ExportBlueprintPreset(savePath, presetPath)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(srcCats) == 0 {
		t.Skip("save has no blueprint categories")
	}

	result, targetSD, err := ImportBlueprintPreset(presetPath, savePath, "replace")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.FinalCategories != len(srcCats) {
		t.Errorf("final = %d, want %d", result.FinalCategories, len(srcCats))
	}

	assertBodyIntact(t, targetSD, "after self-replace")

	reloaded := writeAndReload(t, targetSD)
	assertBodyIntact(t, reloaded, "after self-replace + reload")

	info, err := FindBlueprintSubsystem(reloaded)
	if err != nil {
		t.Fatalf("blueprint subsystem missing: %v", err)
	}
	got, err := ExtractCategoryRecords(info, reloaded)
	if err != nil {
		t.Fatalf("extracting after self-replace: %v", err)
	}
	if len(got) != len(srcCats) {
		t.Errorf("category count changed: %d -> %d", len(srcCats), len(got))
	}
}

// TestImportPresetInvalidMode verifies that an unknown mode is handled gracefully.
// The replace/merge branch only checks for "merge"; anything else falls through to
// replace, so this test confirms that an unknown mode doesn't crash.
func TestImportPresetInvalidMode(t *testing.T) {
	sourcePath := fixture(t, fxSave12)
	targetPath := fixture(t, fxBlueprints)

	tmp := t.TempDir()
	presetPath := filepath.Join(tmp, "mode.sfbcat")

	_, err := ExportBlueprintPreset(sourcePath, presetPath)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	// "nonsense" is not "merge", so it falls through to replace logic.
	result, targetSD, err := ImportBlueprintPreset(presetPath, targetPath, "nonsense")
	if err != nil {
		t.Fatalf("import with unknown mode: %v", err)
	}
	if result.Mode != "nonsense" {
		t.Errorf("mode = %q, want nonsense (echoed back)", result.Mode)
	}
	assertBodyIntact(t, targetSD, "after unknown mode import")
}
