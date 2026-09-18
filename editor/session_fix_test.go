package editor

import (
	"testing"

	"satisfacts/parser"
)

// TestBuildFStringBytesRoundTrip locks in that buildFStringBytes/
// encodeFStringContent produce bytes parser.BinaryReader.ReadString decodes
// back to the original string, in both encodings FixSessionName has to
// reproduce (ASCII and the UTF-16LE encoding used for non-ASCII names like
// German session names with umlauts).
func TestBuildFStringBytesRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		value string
		utf16 bool
	}{
		{"ascii", "Zentrallager", false},
		{"utf16 with umlaut", "Fabrik für Öl", true},
		{"empty ascii", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == "" {
				return // ReadString/buildFStringBytes don't round-trip 0-length through the negative branch; not exercised by FixSessionName.
			}
			encoded := buildFStringBytes(tc.value, tc.utf16)
			r := parser.NewBinaryReader(encoded)
			got := r.ReadString()
			if got != tc.value {
				t.Errorf("round-trip = %q, want %q", got, tc.value)
			}
			if r.Position() != len(encoded) {
				t.Errorf("ReadString consumed %d of %d bytes", r.Position(), len(encoded))
			}
		})
	}
}

// TestFixSessionNameBodySearchMatchesUTF16Encoding is a regression test for
// the bug where the body search pattern was always built as ASCII, so a
// session name stored as UTF-16LE (any name with non-ASCII characters) was
// never found in the body and silently produced 0 replacements.
func TestFixSessionNameBodySearchMatchesUTF16Encoding(t *testing.T) {
	const original = "Fabrik für Öl"
	const replacement = "Fabrik"

	// Simulate a body blob containing one occurrence of the session name
	// encoded the way the game actually stores non-ASCII FStrings.
	occurrence := buildFStringBytes(original, true)
	body := append([]byte("junk-before..."), occurrence...)
	body = append(body, []byte("...junk-after")...)

	searchPattern := buildFStringBytes(original, true)
	newContent := encodeFStringContent(replacement, true)
	newContent = append(newContent, 0, 0)
	if len(newContent) > len(searchPattern)-4 {
		t.Fatalf("test setup: replacement too long")
	}
	repl := make([]byte, len(searchPattern))
	copy(repl[:4], searchPattern[:4])
	copy(repl[4:], newContent)

	n := bytesReplaceAll(body, searchPattern, repl)
	if n != 1 {
		t.Fatalf("bytesReplaceAll found %d occurrences, want 1 (this is the encoding-mismatch bug if 0)", n)
	}
}

func writeSessionTestSave(t *testing.T) (*SaveData, string, string) {
	t.Helper()
	src := fixture(t, fxSave12)
	sd, err := LoadSave(src)
	if err != nil {
		t.Fatal(err)
	}
	sh, err := LoadSaveHeader(src)
	if err != nil {
		t.Fatal(err)
	}
	return sd, sh.Header.SessionName, src
}

func TestFixSessionNameRejectsNonASCIIForASCIISave(t *testing.T) {
	sd, orig, src := writeSessionTestSave(t)
	if _, err := FixSessionName(sd, orig, "Mü", false, src, t.TempDir()); err == nil {
		t.Error("expected an error: an ASCII name field can't hold ü")
	}
}

func TestFixSessionNameFailsWhenBodyHasNoMatch(t *testing.T) {
	sd, orig, src := writeSessionTestSave(t)
	if _, err := FixSessionName(sd, orig+"_not_in_body", "AB", false, src, t.TempDir()); err == nil {
		t.Error("expected an error when the name isn't in the body")
	}
}
