package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// The Windows file version comes from wails.json and the in-app version from
// AppVersion, and both have to be bumped by hand on each release.
func TestWailsVersionMatchesAppVersion(t *testing.T) {
	data, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Info.ProductVersion != AppVersion {
		t.Errorf("wails.json productVersion %q != AppVersion %q", cfg.Info.ProductVersion, AppVersion)
	}
}

func TestAppDir(t *testing.T) {
	cases := map[string]string{
		"/opt/tools/SatisFacts":                                  "/opt/tools",
		"/Applications/SatisFacts.app/Contents/MacOS/SatisFacts": "/Applications",
		"/home/me/Contents/MacOS/SatisFacts":                     "/home/me/Contents/MacOS",
	}
	for exe, want := range cases {
		if got := appDir(filepath.FromSlash(exe)); got != filepath.FromSlash(want) {
			t.Errorf("appDir(%q) = %q, want %q", exe, got, want)
		}
	}
}

func TestSaveTrackerWaitsUntilFileIsStable(t *testing.T) {
	var tr saveTracker
	t0 := time.Unix(1000, 0)
	steps := []struct {
		path string
		mod  time.Time
		size int64
		want bool
		why  string
	}{
		{"a.sav", t0, 10, false, "first sighting, might still be written"},
		{"a.sav", t0.Add(time.Second), 20, false, "still growing"},
		{"a.sav", t0.Add(time.Second), 20, true, "unchanged between looks"},
		{"a.sav", t0.Add(time.Second), 20, false, "already reported"},
		{"b.sav", t0.Add(2 * time.Second), 5, false, "new file, first sighting"},
		{"b.sav", t0.Add(2 * time.Second), 5, true, "new file, now stable"},
	}
	for i, s := range steps {
		if got := tr.observe(s.path, s.mod, s.size); got != s.want {
			t.Errorf("step %d (%s): got %v, want %v", i, s.why, got, s.want)
		}
	}
}

func TestIsToolOutput(t *testing.T) {
	for name, want := range map[string]bool{
		"Fabrik_autosave_0.sav": false,
		"Fabrik_optimized.sav":  true,
		"Fabrik_sessionfix.sav": true,
		"optimized_factory.sav": false,
		"Fabrik_blueprints.sav": true,
	} {
		if got := isToolOutput(name); got != want {
			t.Errorf("isToolOutput(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestSteamLibrariesReadsLibraryFolders(t *testing.T) {
	home := t.TempDir()
	steamapps := filepath.Join(home, ".local", "share", "Steam", "steamapps")
	if err := os.MkdirAll(steamapps, 0755); err != nil {
		t.Fatal(err)
	}
	vdf := "\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"/mnt/games/SteamLibrary\"\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(steamapps, "libraryfolders.vdf"), []byte(vdf), 0644); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, library := range steamLibraries(home) {
		found = found || library == "/mnt/games/SteamLibrary"
	}
	if !found {
		t.Error("library from libraryfolders.vdf not returned")
	}
}

// buildHeader makes a minimal v8 header whose mod metadata is modSize bytes long.
func buildHeader(modSize int) []byte {
	str := func(s string) []byte {
		n := int32(len(s) + 1)
		out := []byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}
		return append(append(out, s...), 0)
	}
	i32 := func(v int32) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }
	var b []byte
	b = append(b, i32(8)...)  // header type
	b = append(b, i32(42)...) // save version
	b = append(b, i32(1)...)  // build version
	b = append(b, str("Map")...)
	b = append(b, str("")...)
	b = append(b, str("Session")...)
	b = append(b, i32(0)...)          // play duration
	b = append(b, make([]byte, 8)...) // date
	b = append(b, 0)                  // visibility
	b = append(b, i32(0)...)          // editor object version
	b = append(b, str(strings.Repeat("x", modSize))...)
	b = append(b, i32(1)...)           // is modded
	b = append(b, make([]byte, 16)...) // checksum
	return b
}

func TestReadSaveHeaderGrowsForLargeModList(t *testing.T) {
	data := buildHeader(300 * 1024)
	path := filepath.Join(t.TempDir(), "big.sav")
	if err := os.WriteFile(path, append(data, make([]byte, 100)...), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, _ := f.Stat()

	header, _, end, err := readSaveHeader(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	if end != len(data) {
		t.Errorf("header end = %d, want %d", end, len(data))
	}
	if header.SessionName != "Session" || len(header.RawModMetadataString) != 300*1024 {
		t.Errorf("header parsed wrong: session %q, mod metadata %d bytes", header.SessionName, len(header.RawModMetadataString))
	}
}

func TestReadSaveHeaderTruncatedFile(t *testing.T) {
	data := buildHeader(300 * 1024)[:200*1024]
	path := filepath.Join(t.TempDir(), "cut.sav")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, _, _, err := readSaveHeader(f, int64(len(data))); err == nil {
		t.Error("expected an error for a header cut off by the end of the file")
	}
}

func TestNewMemoryCheck(t *testing.T) {
	const gb = 1024 * 1024 * 1024
	if c := newMemoryCheck(10*gb, 20*gb); !c.Enough || c.NeededGB != 10 || c.AvailableGB != 20 {
		t.Errorf("plenty free: %+v", c)
	}
	if c := newMemoryCheck(20*gb, 10*gb); c.Enough {
		t.Errorf("too little free should not be enough: %+v", c)
	}
	if c := newMemoryCheck(20*gb, 0); !c.Enough {
		t.Errorf("unknown free memory should not block: %+v", c)
	}
}

func TestLowMemoryGCRestoresAfterOverlap(t *testing.T) {
	want := debug.SetGCPercent(100)
	defer debug.SetGCPercent(want)

	endFirst := lowMemoryGC()
	endSecond := lowMemoryGC()
	endFirst()
	if got := debug.SetGCPercent(saveToolGCPercent); got != saveToolGCPercent {
		t.Errorf("setting changed while a call was still running: %d", got)
	}
	endSecond()
	if got := debug.SetGCPercent(100); got != 100 {
		t.Errorf("setting not restored: %d", got)
	}
}
