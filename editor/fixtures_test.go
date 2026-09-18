package editor

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"satisfacts/parser"
)

// .sav files aren't committed (they're large and contain personal worlds), so
// every fixture-backed test skips when the file isn't there. Point
// SATISFACTS_FIXTURES at a directory to use saves from somewhere else.
const fixtureEnv = "SATISFACTS_FIXTURES"

// Small saves that live in the repo root during development. Kept small on
// purpose - these tests decompress the whole body several times each.
const (
	fxBelts      = "TestBelts.sav"
	fxBeltUpg    = "TestBeltUpgrade.sav"
	fxPipes      = "TestPipes.sav"
	fxBlueprints = "TestBlueprints.sav"
	fxSession    = "Save _autosave_2.sav" // session name has a trailing space
	fxSave12     = "Save1.2.sav"
	fxHotbar     = "Hotbar_Blueprints.sav" // blueprints on the hotbar
)

func fixtureDir() string {
	if d := os.Getenv(fixtureEnv); d != "" {
		return d
	}
	return ".."
}

// fixture returns the path to a named save, skipping the test if it's missing.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(fixtureDir(), name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture %q not available (set %s to override) — .sav files are not committed", name, fixtureEnv)
	}
	return path
}

// loadFixture loads a save, skipping if the fixture is missing.
func loadFixture(t *testing.T, name string) *SaveData {
	t.Helper()
	sd, err := LoadSave(fixture(t, name))
	if err != nil {
		t.Fatalf("LoadSave(%s): %v", name, err)
	}
	return sd
}

// writeAndReload writes sd to a temp file and loads it back. This is the check
// that matters most: it proves the game's parser (well, ours, which mirrors it)
// can read what we produced.
func writeAndReload(t *testing.T, sd *SaveData) *SaveData {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := WriteSave(sd, out); err != nil {
		t.Fatalf("WriteSave: %v", err)
	}
	reloaded, err := LoadSave(out)
	if err != nil {
		t.Fatalf("reloading written save: %v", err)
	}
	return reloaded
}

// assertBodyIntact checks the structural invariants that, when broken, produce a
// save the game either refuses to load or loads with objects silently truncated.
//
// Every one of these corresponds to a bug that actually shipped at some point:
// a stale totalBodyRestSize truncated pipes and crashed the game, and stale
// per-level blob lengths desync the whole object walk.
func assertBodyIntact(t *testing.T, sd *SaveData, stage string) {
	t.Helper()

	body := sd.Decompressed
	if len(body) < 8 {
		t.Fatalf("%s: body is only %d bytes", stage, len(body))
	}

	// totalBodyRestSize excludes the size field itself, plus the UE5 zero that
	// follows it on UE5-era saves.
	got := int32(binary.LittleEndian.Uint32(body[0:4]))
	want := int32(len(body) - 4)
	if sd.VersionCtx.SaveIsAtLeast(parser.SaveUnrealEngine5) {
		want = int32(len(body) - 8)
	}
	if got != want {
		t.Errorf("%s: totalBodyRestSize = %d, want %d (body is %d bytes)", stage, got, want, len(body))
	}

	// The body must still walk cleanly end to end.
	_, levels, trailingStart, trailingEnd, err := ParseBodyLevels(body, sd.VersionCtx)
	if err != nil {
		t.Fatalf("%s: ParseBodyLevels failed — body is corrupt: %v", stage, err)
	}
	if len(levels) == 0 {
		t.Fatalf("%s: no levels parsed", stage)
	}
	if trailingEnd < trailingStart {
		t.Errorf("%s: trailing range inverted (%d..%d)", stage, trailingStart, trailingEnd)
	}
	if trailingEnd > len(body) {
		t.Errorf("%s: trailing end %d past body end %d", stage, trailingEnd, len(body))
	}

	sawPersistent := false
	for i, l := range levels {
		if l.IsPersistent {
			sawPersistent = true
		}
		if l.TocDataEnd > len(body) || l.DataBlobEnd > len(body) || l.FooterEnd > len(body) {
			t.Errorf("%s: level %d (%s) has ranges past the end of the body", stage, i, l.Name)
			continue
		}
		if l.TocDataStart > l.TocDataEnd {
			t.Errorf("%s: level %d (%s) TOC range inverted", stage, i, l.Name)
		}
		if l.DataBlobStart > l.DataBlobEnd {
			t.Errorf("%s: level %d (%s) data blob range inverted", stage, i, l.Name)
		}
		// One offset+size pair per object, in both the TOC and the data blob.
		if len(l.TocHeaderOffsets) != len(l.TocHeaderSizes) {
			t.Errorf("%s: level %d (%s) TOC offset/size count mismatch: %d vs %d",
				stage, i, l.Name, len(l.TocHeaderOffsets), len(l.TocHeaderSizes))
		}
		if len(l.DataObjOffsets) != len(l.DataObjSizes) {
			t.Errorf("%s: level %d (%s) data offset/size count mismatch: %d vs %d",
				stage, i, l.Name, len(l.DataObjOffsets), len(l.DataObjSizes))
		}
		if l.ObjectCount > 0 && len(l.TocHeaderOffsets) != int(l.ObjectCount) {
			t.Errorf("%s: level %d (%s) says %d objects but TOC has %d headers",
				stage, i, l.Name, l.ObjectCount, len(l.TocHeaderOffsets))
		}
		// No object may claim bytes outside its own data blob.
		for j := range l.DataObjOffsets {
			end := l.DataObjOffsets[j] + l.DataObjSizes[j]
			if l.DataObjOffsets[j] < 0 || l.DataBlobStart+end > l.DataBlobEnd {
				t.Errorf("%s: level %d (%s) object %d spills outside its data blob (offset %d size %d, blob %d..%d)",
					stage, i, l.Name, j, l.DataObjOffsets[j], l.DataObjSizes[j], l.DataBlobStart, l.DataBlobEnd)
				break
			}
		}
	}
	if !sawPersistent {
		t.Errorf("%s: no persistent level found", stage)
	}
}
