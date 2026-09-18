package editor

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"satisfacts/parser"
)

// allFixtures is every save these tests will try. Missing ones are skipped.
var allFixtures = []string{fxBelts, fxBeltUpg, fxPipes, fxBlueprints, fxSession, fxSave12, fxHotbar}

// A save that goes in and straight back out must come back byte-for-byte
// identical once decompressed. If this breaks, nothing else in the package can
// be trusted, because every mutation builds on load/write.
func TestLoadWriteRoundTripIsLossless(t *testing.T) {
	for _, name := range allFixtures {
		t.Run(name, func(t *testing.T) {
			sd := loadFixture(t, name)
			original := append([]byte(nil), sd.Decompressed...)

			assertBodyIntact(t, sd, "as loaded")

			reloaded := writeAndReload(t, sd)

			if !bytes.Equal(original, reloaded.Decompressed) {
				t.Fatalf("decompressed body changed across write/reload: %d bytes in, %d bytes out",
					len(original), len(reloaded.Decompressed))
			}
			if reloaded.IsV2 != sd.IsV2 {
				t.Errorf("IsV2 changed: %v -> %v", sd.IsV2, reloaded.IsV2)
			}
			if reloaded.MaxChunkSize != sd.MaxChunkSize {
				t.Errorf("MaxChunkSize changed: %d -> %d", sd.MaxChunkSize, reloaded.MaxChunkSize)
			}
			assertBodyIntact(t, reloaded, "after round trip")
		})
	}
}

// The header is copied verbatim, so the output file must start with exactly the
// same bytes as the input.
func TestWritePreservesHeaderBytes(t *testing.T) {
	for _, name := range allFixtures {
		t.Run(name, func(t *testing.T) {
			path := fixture(t, name)
			sd, err := LoadSave(path)
			if err != nil {
				t.Fatalf("LoadSave: %v", err)
			}

			out := filepath.Join(t.TempDir(), "out.sav")
			if err := WriteSave(sd, out); err != nil {
				t.Fatalf("WriteSave: %v", err)
			}

			written, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("reading output: %v", err)
			}
			if len(written) < sd.HeaderEnd {
				t.Fatalf("output is %d bytes, shorter than the %d-byte header", len(written), sd.HeaderEnd)
			}
			if !bytes.Equal(sd.FileData[:sd.HeaderEnd], written[:sd.HeaderEnd]) {
				t.Errorf("header bytes differ between input and output")
			}
		})
	}
}

// Chunks larger than 128 KB crash the game in
// FArchiveLoadCompressedProxy64::DecompressMoreData, which uses a fixed buffer.
func TestRecompressBodyRespectsChunkLimit(t *testing.T) {
	const gameChunkLimit = 128 * 1024

	for _, name := range allFixtures {
		t.Run(name, func(t *testing.T) {
			sd := loadFixture(t, name)
			if sd.MaxChunkSize > gameChunkLimit {
				t.Errorf("save declares MaxChunkSize %d, above the game's %d limit",
					sd.MaxChunkSize, gameChunkLimit)
			}

			// Recompressing and decompressing must be a faithful round trip.
			compressed := RecompressBody(sd.Decompressed, sd.IsV2, sd.MaxChunkSize)
			reader := &SliceReader{Data: compressed}
			got, derr := parser.NewDecompressor(reader).DecompressAll()
			if derr != nil {
				t.Fatalf("cannot decompress what RecompressBody produced: %v", derr)
			}
			if !bytes.Equal(got, sd.Decompressed) {
				t.Errorf("recompress/decompress round trip lost data: %d bytes in, %d out",
					len(sd.Decompressed), len(got))
			}
		})
	}
}

func TestOptimizeSaveKeepsBodyValid(t *testing.T) {
	for _, name := range allFixtures {
		t.Run(name, func(t *testing.T) {
			sd := loadFixture(t, name)

			_, levelsBefore, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
			if err != nil {
				t.Fatalf("parsing before optimize: %v", err)
			}
			objectsBefore := 0
			for _, l := range levelsBefore {
				objectsBefore += int(l.ObjectCount)
			}

			result, err := OptimizeSave(sd, nil)
			if err != nil {
				t.Fatalf("OptimizeSave: %v", err)
			}
			assertBodyIntact(t, sd, "after optimize")

			if result.TotalObjects != objectsBefore {
				t.Errorf("optimize reports %d objects, but %d were present before",
					result.TotalObjects, objectsBefore)
			}

			// Reloading is the real proof: the write path has to survive too.
			reloaded := writeAndReload(t, sd)
			assertBodyIntact(t, reloaded, "after optimize + reload")
			assertOutersFirst(t, reloaded)

			_, levelsAfter, _, _, err := ParseBodyLevels(reloaded.Decompressed, reloaded.VersionCtx)
			if err != nil {
				t.Fatalf("parsing after optimize: %v", err)
			}
			if len(levelsAfter) != len(levelsBefore) {
				t.Errorf("level count changed: %d -> %d", len(levelsBefore), len(levelsAfter))
			}
			objectsAfter := 0
			for _, l := range levelsAfter {
				objectsAfter += int(l.ObjectCount)
			}
			if objectsAfter != objectsBefore {
				t.Errorf("object count changed: %d -> %d", objectsBefore, objectsAfter)
			}

			// Optimize only reorders, so it must be idempotent in object terms.
			if _, err := OptimizeSave(reloaded, nil); err != nil {
				t.Errorf("optimizing an already-optimized save failed: %v", err)
			}
			assertBodyIntact(t, reloaded, "after second optimize")
		})
	}
}

// assertOutersFirst fails if any object is stored before its outer object in
// the same level. The game drops such objects on load - this is what wiped
// blueprint hotbar slots (FGBlueprintShortcut sorted before FGPlayerHotbar).
func assertOutersFirst(t *testing.T, sd *SaveData) {
	t.Helper()
	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		t.Fatalf("ParseBodyLevels: %v", err)
	}
	for _, l := range levels {
		if l.ObjectCount == 0 {
			continue
		}
		r := parser.NewBinaryReader(sd.Decompressed[l.TocDataStart:l.TocDataEnd])
		r.ReadInt32() // count
		headers, err := parser.ParseObjectHeaders(r, sd.VersionCtx, int(l.ObjectCount))
		if err != nil {
			t.Fatalf("level %q: ParseObjectHeaders: %v", l.Name, err)
		}
		pos := make(map[string]int, len(headers))
		for i, h := range headers {
			pos[h.Reference.PathName] = i
		}
		for i, h := range headers {
			if o, ok := pos[h.OuterPathName]; ok && o > i {
				t.Errorf("level %q: %s (%s) stored before its outer %s",
					l.Name, h.Reference.PathName, h.ClassName, h.OuterPathName)
			}
		}
	}
}

// Duplicate path names are what made belts vanish in-game: the game keeps one
// object per path name and silently drops the rest.
func TestUpgradeBeltsProducesNoDuplicatePathNames(t *testing.T) {
	for _, name := range []string{fxBelts, fxBeltUpg, fxSave12} {
		t.Run(name, func(t *testing.T) {
			sd := loadFixture(t, name)

			before, err := ScanBeltInventory(sd.Decompressed, sd.VersionCtx)
			if err != nil {
				t.Fatalf("ScanBeltInventory: %v", err)
			}
			totalBefore := 0
			for _, e := range before {
				totalBefore += e.Count
			}
			if totalBefore == 0 {
				t.Skipf("%s has no belts/lifts/pipes", name)
			}

			dupsBefore := countDuplicatePathNames(t, sd)

			if _, err := UpgradeBelts(sd, UpgradeBeltsOptions{
				UpgradeBelts: true,
				UpgradeLifts: true,
				TargetMk:     6,
			}); err != nil {
				t.Fatalf("UpgradeBelts: %v", err)
			}
			assertBodyIntact(t, sd, "after belt upgrade")

			reloaded := writeAndReload(t, sd)
			assertBodyIntact(t, reloaded, "after belt upgrade + reload")

			// Total count must be preserved - no belt may be lost.
			after, err := ScanBeltInventory(reloaded.Decompressed, reloaded.VersionCtx)
			if err != nil {
				t.Fatalf("ScanBeltInventory after upgrade: %v", err)
			}
			totalAfter := 0
			for _, e := range after {
				totalAfter += e.Count
				if e.Type != BeltTypePipe && e.Count > 0 && e.Mk != 6 {
					t.Errorf("%s Mk%d still has %d entries after upgrading to Mk6", e.Type, e.Mk, e.Count)
				}
			}
			if totalAfter != totalBefore {
				t.Errorf("belt count changed: %d -> %d", totalBefore, totalAfter)
			}

			if dups := countDuplicatePathNames(t, reloaded); dups > dupsBefore {
				t.Errorf("upgrade introduced %d duplicate path names (was %d) — belts will disappear in-game",
					dups-dupsBefore, dupsBefore)
			}
		})
	}
}

// countDuplicatePathNames counts conveyor path names that appear on more than
// one TOC entry across the whole save.
func countDuplicatePathNames(t *testing.T, sd *SaveData) int {
	t.Helper()

	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		t.Fatalf("ParseBodyLevels: %v", err)
	}

	seen := map[string]int{}
	for _, l := range levels {
		if l.ObjectCount == 0 {
			continue
		}
		r := parser.NewBinaryReader(sd.Decompressed[l.TocDataStart:l.TocDataEnd])
		r.ReadInt32() // object count
		headers, err := parser.ParseObjectHeaders(r, sd.VersionCtx, int(l.ObjectCount))
		if err != nil {
			continue
		}
		for _, h := range headers {
			if strings.Contains(h.ClassName, "Conveyor") {
				seen[h.Reference.PathName]++
			}
		}
	}

	dups := 0
	for _, n := range seen {
		if n > 1 {
			dups++
		}
	}
	return dups
}

// Pipes must get a new class name but keep their path name, or every connection
// to a building breaks.
func TestUpgradePipesKeepsPathNames(t *testing.T) {
	sd := loadFixture(t, fxPipes)

	before := pipeClassByPath(t, sd)
	if len(before) == 0 {
		t.Skip("fixture has no pipes")
	}

	mk1Before := 0
	for _, class := range before {
		if !strings.Contains(class, "MK2") {
			mk1Before++
		}
	}

	count, err := UpgradePipes(sd)
	if err != nil {
		t.Fatalf("UpgradePipes: %v", err)
	}
	assertBodyIntact(t, sd, "after pipe upgrade")

	reloaded := writeAndReload(t, sd)
	assertBodyIntact(t, reloaded, "after pipe upgrade + reload")

	after := pipeClassByPath(t, reloaded)
	if len(after) != len(before) {
		t.Errorf("pipe count changed: %d -> %d (upgraded %d)", len(before), len(after), count)
	}

	// Path names are object identities. Other objects reference pipes by path
	// name, so renaming one silently breaks every connection to it.
	for p := range before {
		if _, ok := after[p]; !ok {
			t.Errorf("pipe path name %q disappeared", p)
		}
	}

	// The TOC class name is what the game uses to decide the entity type, so
	// every pipe must read as Mk2 there. Mk1 strings deliberately survive inside
	// the data blob (recipe refs in mCustomizationData), so don't scan raw bytes.
	for p, class := range after {
		if !strings.Contains(class, "MK2") {
			t.Errorf("pipe %q still has a Mk1 class name after upgrade: %s", p, class)
		}
	}
	if count != mk1Before {
		t.Errorf("UpgradePipes reported %d upgrades, but %d Mk1 pipes were present", count, mk1Before)
	}
}

// pipeClassByPath maps each pipeline segment's path name to its TOC class name.
// Supports, junctions and pumps are excluded - they're separate buildables that
// must never be rewritten.
func pipeClassByPath(t *testing.T, sd *SaveData) map[string]string {
	t.Helper()

	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		t.Fatalf("ParseBodyLevels: %v", err)
	}

	out := map[string]string{}
	for _, l := range levels {
		if l.ObjectCount == 0 {
			continue
		}
		r := parser.NewBinaryReader(sd.Decompressed[l.TocDataStart:l.TocDataEnd])
		r.ReadInt32()
		headers, err := parser.ParseObjectHeaders(r, sd.VersionCtx, int(l.ObjectCount))
		if err != nil {
			continue
		}
		for _, h := range headers {
			if strings.Contains(h.ClassName, "Build_Pipeline.") ||
				strings.Contains(h.ClassName, "Build_PipelineMK2.") ||
				strings.Contains(h.ClassName, "Build_Pipeline_NoIndicator.") ||
				strings.Contains(h.ClassName, "Build_PipelineMK2_NoIndicator.") {
				out[h.Reference.PathName] = h.ClassName
			}
		}
	}
	return out
}

func TestReorderDepotItems(t *testing.T) {
	sd := loadFixture(t, fxSave12)

	items, err := depotItems(sd)
	if err != nil {
		t.Skipf("no depot in %s: %v", fxSave12, err)
	}
	if len(items) < 2 {
		t.Skipf("depot has %d items, need at least 2 to reorder", len(items))
	}

	originalLen := len(sd.Decompressed)
	originalClasses := make([]string, len(items))
	for i, it := range items {
		originalClasses[i] = it.ItemClass
	}

	// Reverse the order - every item moves.
	order := make([]int, len(items))
	for i := range order {
		order[i] = len(items) - 1 - i
	}

	if err := ReorderDepotItems(sd, order); err != nil {
		t.Fatalf("ReorderDepotItems: %v", err)
	}
	assertBodyIntact(t, sd, "after depot reorder")

	// A permutation shuffles the same bytes, so nothing should resize.
	if len(sd.Decompressed) != originalLen {
		t.Errorf("body length changed: %d -> %d; a permutation must not resize anything",
			originalLen, len(sd.Decompressed))
	}

	reloaded := writeAndReload(t, sd)
	assertBodyIntact(t, reloaded, "after depot reorder + reload")

	got, err := depotItems(reloaded)
	if err != nil {
		t.Fatalf("reading depot after reorder: %v", err)
	}
	if len(got) != len(items) {
		t.Fatalf("depot item count changed: %d -> %d", len(items), len(got))
	}
	for i, it := range got {
		want := originalClasses[order[i]]
		if it.ItemClass != want {
			t.Errorf("position %d: got %q, want %q", i, it.ItemClass, want)
		}
	}
}

// The identity permutation is a no-op, and must leave the file untouched. This
// is the regression guard for the UI bug that sent [0,1,2,...] every time.
func TestReorderDepotIdentityIsNoOp(t *testing.T) {
	sd := loadFixture(t, fxSave12)

	items, err := depotItems(sd)
	if err != nil {
		t.Skipf("no depot in %s: %v", fxSave12, err)
	}
	if len(items) < 2 {
		t.Skip("need at least 2 depot items")
	}

	before := append([]byte(nil), sd.Decompressed...)

	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	if err := ReorderDepotItems(sd, order); err != nil {
		t.Fatalf("ReorderDepotItems: %v", err)
	}
	if !bytes.Equal(before, sd.Decompressed) {
		t.Error("identity reorder changed the body; it should be a no-op")
	}
}

func TestReorderDepotRejectsBadOrders(t *testing.T) {
	sd := loadFixture(t, fxSave12)

	items, err := depotItems(sd)
	if err != nil {
		t.Skipf("no depot in %s: %v", fxSave12, err)
	}
	if len(items) < 2 {
		t.Skip("need at least 2 depot items")
	}

	before := append([]byte(nil), sd.Decompressed...)

	bad := map[string][]int{
		"too short":    make([]int, len(items)-1),
		"duplicates":   make([]int, len(items)),
		"out of range": make([]int, len(items)),
	}
	for i := range bad["duplicates"] {
		bad["duplicates"][i] = 0
	}
	for i := range bad["out of range"] {
		bad["out of range"][i] = i
	}
	bad["out of range"][0] = len(items)

	for label, order := range bad {
		t.Run(label, func(t *testing.T) {
			if err := ReorderDepotItems(sd, order); err == nil {
				t.Error("expected an error, got nil")
			}
			if !bytes.Equal(before, sd.Decompressed) {
				t.Error("body was modified despite the invalid order")
			}
		})
	}
}

// depotItems pulls the current depot contents out of a save.
func depotItems(sd *SaveData) ([]DepotItem, error) {
	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		return nil, err
	}
	for _, l := range levels {
		if !l.IsPersistent || l.ObjectCount == 0 {
			continue
		}
		tocData := sd.Decompressed[l.TocDataStart:l.TocDataEnd]
		idx, header, ferr := FindCentralStorageSubsystem(tocData, sd.VersionCtx, int(l.ObjectCount))
		if ferr != nil {
			continue
		}
		objStart := l.DataObjOffsets[idx]
		objEnd := objStart + l.DataObjSizes[idx]
		objData := sd.Decompressed[l.DataBlobStart+objStart : l.DataBlobStart+objEnd]
		return FindStoredItemsArray(objData, header, sd.VersionCtx, l.SaveCustomVersions[idx])
	}
	return nil, os.ErrNotExist
}

func TestBlueprintCategoryCopyKeepsBodyValid(t *testing.T) {
	for _, mode := range []string{"replace", "merge"} {
		t.Run(mode, func(t *testing.T) {
			source := loadFixture(t, fxSave12)
			target := loadFixture(t, fxBlueprints)

			srcInfo, err := FindBlueprintSubsystem(source)
			if err != nil {
				t.Skipf("no blueprint subsystem in source: %v", err)
			}
			srcCats, err := ExtractCategoryRecords(srcInfo, source)
			if err != nil {
				t.Fatalf("reading source categories: %v", err)
			}

			result, err := CopyBlueprintCategories(source, target, mode)
			if err != nil {
				t.Fatalf("CopyBlueprintCategories(%s): %v", mode, err)
			}
			assertBodyIntact(t, target, "after blueprint copy")

			reloaded := writeAndReload(t, target)
			assertBodyIntact(t, reloaded, "after blueprint copy + reload")

			// The categories must survive the write, not just the in-memory edit.
			info, err := FindBlueprintSubsystem(reloaded)
			if err != nil {
				t.Fatalf("blueprint subsystem missing after copy: %v", err)
			}
			got, err := ExtractCategoryRecords(info, reloaded)
			if err != nil {
				t.Fatalf("reading categories after copy: %v", err)
			}
			if len(got) != result.FinalCategories {
				t.Errorf("result claims %d categories, save actually has %d",
					result.FinalCategories, len(got))
			}
			if mode == "replace" && len(got) != len(srcCats) {
				t.Errorf("replace produced %d categories, want %d from source", len(got), len(srcCats))
			}
			if mode == "merge" && len(got) < result.TargetCategories {
				t.Errorf("merge lost categories: %d after, %d before", len(got), result.TargetCategories)
			}
		})
	}
}

func TestMultiplierPatchRoundTrip(t *testing.T) {
	sd := loadFixture(t, fxSave12)

	gsi, err := FindGameState(sd)
	if err != nil {
		t.Skipf("no GameState in %s: %v", fxSave12, err)
	}

	const propName = "mPartsCostMultiplier"
	objData := sd.Decompressed[gsi.VPos : gsi.VPos+int64(gsi.Size)]
	offset, original := FindFloatPropOffset(objData, propName, gsi.IsActor, gsi.UseCompleteTag)
	if offset < 0 {
		t.Skipf("%s not present in this save", propName)
	}

	originalLen := len(sd.Decompressed)

	// Deliberately above the in-game UI cap of 2.0 - the whole point of the
	// feature is that the game honours raw values.
	const target = float32(5.0)
	writeFloat32(sd.Decompressed, int(gsi.VPos)+offset, target)

	if len(sd.Decompressed) != originalLen {
		t.Errorf("patching a float changed the body length: %d -> %d", originalLen, len(sd.Decompressed))
	}
	assertBodyIntact(t, sd, "after multiplier patch")

	reloaded := writeAndReload(t, sd)
	assertBodyIntact(t, reloaded, "after multiplier patch + reload")

	gsi2, err := FindGameState(reloaded)
	if err != nil {
		t.Fatalf("GameState missing after patch: %v", err)
	}
	objData2 := reloaded.Decompressed[gsi2.VPos : gsi2.VPos+int64(gsi2.Size)]
	_, got := FindFloatPropOffset(objData2, propName, gsi2.IsActor, gsi2.UseCompleteTag)
	if got != target {
		t.Errorf("%s = %v after round trip, want %v (was %v)", propName, got, target, original)
	}
}

// writeFloat32 mirrors what the UI's PatchMultipliers does.
func writeFloat32(buf []byte, at int, v float32) {
	bits := math.Float32bits(v)
	buf[at] = byte(bits)
	buf[at+1] = byte(bits >> 8)
	buf[at+2] = byte(bits >> 16)
	buf[at+3] = byte(bits >> 24)
}

func TestOptimizedSaveReportsOptimizedOrder(t *testing.T) {
	sd, err := LoadSave(fixture(t, fxHotbar))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OptimizeSave(sd, nil); err != nil {
		t.Fatal(err)
	}
	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range levels {
		if l.ObjectCount == 0 {
			continue
		}
		r := parser.NewBinaryReader(sd.Decompressed[l.TocDataStart:l.TocDataEnd])
		r.ReadInt32()
		headers, err := parser.ParseObjectHeaders(r, sd.VersionCtx, int(l.ObjectCount))
		if err != nil {
			t.Fatal(err)
		}
		if !IsOptimizedOrder(headers) {
			t.Fatalf("level %q still reports as not optimized after optimizing", l.Name)
		}
	}
}

func TestIsOptimizedOrder(t *testing.T) {
	h := func(class, path string) parser.ObjectHeader {
		return parser.ObjectHeader{ClassName: class, Reference: parser.ObjectRef{PathName: path}}
	}
	sorted := []parser.ObjectHeader{h("A", "L:P.a"), h("B", "L:P.b"), h("A", "L:P.a.c")}
	if !IsOptimizedOrder(sorted) {
		t.Error("depth first, then class, is optimized")
	}
	if IsOptimizedOrder([]parser.ObjectHeader{h("B", "L:P.b"), h("A", "L:P.a")}) {
		t.Error("classes out of order at the same depth")
	}
	if IsOptimizedOrder([]parser.ObjectHeader{h("A", "L:P.a.c"), h("A", "L:P.a")}) {
		t.Error("subobject before its outer")
	}
}

func TestWriteSaveLeavesExistingFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "save.sav")
	if err := os.WriteFile(out, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteSave(&SaveData{}, out); err == nil {
		t.Fatal("expected an error without header bytes")
	}
	if data, _ := os.ReadFile(out); string(data) != "original" {
		t.Errorf("existing file was changed: %q", data)
	}

	sd := &SaveData{HeaderBytes: []byte("HDR"), Decompressed: []byte("body"), MaxChunkSize: 1024}
	if err := WriteSave(sd, out); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(out); len(data) < 3 || string(data[:3]) != "HDR" {
		t.Errorf("output not replaced: %q", data)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

// The streaming path must produce exactly what the in-memory path produces.
func TestOptimizeSaveToFileMatchesInMemory(t *testing.T) {
	src := fixture(t, fxHotbar)
	dir := t.TempDir()

	sd, err := LoadSave(src)
	if err != nil {
		t.Fatal(err)
	}
	inMemory := filepath.Join(dir, "memory.sav")
	if _, err := OptimizeSave(sd, nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteSave(sd, inMemory); err != nil {
		t.Fatal(err)
	}

	sd, err = LoadSave(src)
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(sd.Decompressed)
	sd.FreeFileData()
	streamed := filepath.Join(dir, "stream.sav")
	if _, err := OptimizeSaveToFile(sd, streamed, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sd.Decompressed, original) {
		t.Error("OptimizeSaveToFile changed the loaded body")
	}

	want, _ := os.ReadFile(inMemory)
	got, _ := os.ReadFile(streamed)
	if !bytes.Equal(want, got) {
		t.Errorf("outputs differ: in-memory %d bytes, streamed %d bytes", len(want), len(got))
	}
}

func TestOptimizeSaveToFileCancelLeavesNothingBehind(t *testing.T) {
	sd, err := LoadSave(fixture(t, fxSave12))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out.sav")
	if err := os.WriteFile(out, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	cancelled := errors.New("cancelled")
	_, err = OptimizeSaveToFile(sd, out, func(current, total int, _ string) error {
		if current > 0 {
			return cancelled
		}
		return nil
	})
	if !errors.Is(err, cancelled) {
		t.Fatalf("got error %v, want the cancel error", err)
	}
	if data, _ := os.ReadFile(out); string(data) != "existing" {
		t.Errorf("existing file was changed: %q", data)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

// objectSortKeys avoids full header parsing, so it has to agree with it exactly.
func TestObjectSortKeysMatchParsedHeaders(t *testing.T) {
	sd, err := LoadSave(fixture(t, fxHotbar))
	if err != nil {
		t.Fatal(err)
	}
	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range levels {
		if l.ObjectCount == 0 {
			continue
		}
		toc := sd.Decompressed[l.TocDataStart:l.TocDataEnd]
		r := parser.NewBinaryReader(toc)
		r.ReadInt32()
		headers, err := parser.ParseObjectHeaders(r, sd.VersionCtx, int(l.ObjectCount))
		if err != nil {
			t.Fatal(err)
		}
		wantDepths, wantOK := objectDepths(headers)
		depths, classes, ok := objectSortKeys(toc, l.TocHeaderOffsets, sd.VersionCtx)
		if ok != wantOK {
			t.Fatalf("level %q: ok = %v, want %v", l.Name, ok, wantOK)
		}
		for i, h := range headers {
			if classes[i] != h.ClassName {
				t.Fatalf("level %q object %d: class %q, want %q", l.Name, i, classes[i], h.ClassName)
			}
			if wantOK && depths[i] != wantDepths[i] {
				t.Fatalf("level %q object %d: depth %d, want %d", l.Name, i, depths[i], wantDepths[i])
			}
		}
	}
}
