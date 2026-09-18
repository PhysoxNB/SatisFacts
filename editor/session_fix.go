package editor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf16"

	"satisfacts/parser"
)

// SessionNameIssue describes a single problem with a session name.
type SessionNameIssue struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Chars   string `json:"chars,omitempty"` // the offending characters (invalid_chars only)
}

// SessionNameCheckResult holds the validation result for a session name.
type SessionNameCheckResult struct {
	HasIssues bool               `json:"hasIssues"`
	Issues    []SessionNameIssue `json:"issues"`
	AutoFix   string             `json:"autoFix"`
}

// CheckSessionName validates a session name for characters that break blueprints.
func CheckSessionName(name string) *SessionNameCheckResult {
	result := &SessionNameCheckResult{}

	if name == "" {
		return result
	}

	// The characters Windows actually rejects in a path component. Square
	// brackets are legal in folder names, so they must not appear here - an
	// earlier version used a regex-shaped literal and stripped them by mistake.
	const invalidChars = `\/:*?"<>|`
	var issues []SessionNameIssue
	autoFix := name

	if strings.TrimRightFunc(name, unicode.IsSpace) != name {
		issues = append(issues, SessionNameIssue{
			Type:    "trailing_whitespace",
			Message: "Session name has trailing whitespace. Windows strips trailing spaces from folder names, so the game cannot find the blueprint folder.",
		})
	}
	if strings.TrimLeftFunc(name, unicode.IsSpace) != name {
		issues = append(issues, SessionNameIssue{
			Type:    "leading_whitespace",
			Message: "Session name has leading whitespace. This may cause blueprint path issues.",
		})
	}
	autoFix = strings.TrimSpace(autoFix)

	if strings.HasSuffix(autoFix, ".") {
		issues = append(issues, SessionNameIssue{
			Type:    "trailing_period",
			Message: "Session name ends with a period. Windows strips trailing dots from folder names, causing the game to crash when saving blueprints.",
		})
		autoFix = strings.TrimRight(autoFix, ".")
	}

	if strings.ContainsAny(autoFix, invalidChars) {
		var found []rune
		seen := map[rune]bool{}
		for _, c := range autoFix {
			if strings.ContainsRune(invalidChars, c) && !seen[c] {
				seen[c] = true
				found = append(found, c)
			}
		}
		issues = append(issues, SessionNameIssue{
			Type:    "invalid_chars",
			Message: fmt.Sprintf("Session name contains invalid characters (%s) that cannot be used in Windows folder names. Blueprints will not save or load.", string(found)),
			Chars:   string(found),
		})
		autoFix = strings.Map(func(r rune) rune {
			if strings.ContainsRune(invalidChars, r) {
				return -1
			}
			return r
		}, autoFix)
	}

	// Removing dots or invalid characters can expose fresh trailing whitespace
	// ("Save . " -> "Save "), which is the very thing we are trying to avoid.
	autoFix = strings.TrimSpace(autoFix)

	if len(issues) > 0 {
		result.HasIssues = true
		result.Issues = issues
		result.AutoFix = autoFix
	}

	return result
}

// encodeFStringContent encodes s the way parser.BinaryReader.ReadString would
// have decoded it: raw ASCII bytes if utf16 is false, or UTF-16LE code units
// if true. It does not include the length prefix or null terminator.
func encodeFStringContent(s string, utf16Encoded bool) []byte {
	if !utf16Encoded {
		return []byte(s)
	}
	units := utf16.Encode([]rune(s))
	out := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(out[i*2:], u)
	}
	return out
}

// buildFStringBytes builds a full FString byte sequence (length prefix +
// encoded content + null terminator) in the given encoding, matching what
// parser.BinaryReader.ReadString expects on the way back in.
func buildFStringBytes(s string, utf16Encoded bool) []byte {
	content := encodeFStringContent(s, utf16Encoded)
	if utf16Encoded {
		content = append(content, 0, 0) // UTF-16 null terminator (2 bytes)
		charCount := int32(len(content) / 2)
		out := make([]byte, 4+len(content))
		binary.LittleEndian.PutUint32(out[:4], uint32(-charCount))
		copy(out[4:], content)
		return out
	}
	content = append(content, 0) // ASCII null terminator
	out := make([]byte, 4+len(content))
	binary.LittleEndian.PutUint32(out[:4], uint32(int32(len(content))))
	copy(out[4:], content)
	return out
}

// FixSessionNameResult holds the result of a session name fix operation.
type FixSessionNameResult struct {
	OutputPath       string `json:"outputPath"`
	OriginalName     string `json:"originalName"`
	NewName          string `json:"newName"`
	BodyReplacements int    `json:"bodyReplacements"`
}

// FixSessionName patches the session name in both the header and body of a save file.
// If overwrite is true, the original file is overwritten; otherwise a _sessionfix copy is created.
func FixSessionName(sd *SaveData, originalName, newSessionName string, overwrite bool, savePath string, outputDir string) (*FixSessionNameResult, error) {
	// --- Step 1: Patch the header session name FString ---
	// FileData may have been freed by FreeFileData; fall back to the saved header bytes.
	data := sd.FileData
	if data == nil && sd.HeaderBytes != nil {
		data = sd.HeaderBytes
	}
	if data == nil {
		return nil, fmt.Errorf("no file data available for session name fix (FreeFileData was called)")
	}
	pos := 0

	readInt32At := func() int32 {
		v := int32(binary.LittleEndian.Uint32(data[pos:]))
		pos += 4
		return v
	}
	skipString := func() {
		length := readInt32At()
		if length == 0 {
			return
		}
		if length < 0 {
			pos += int(-length) * 2
		} else {
			pos += int(length)
		}
	}

	// Re-parse header to get SaveHeaderType for field ordering
	headerReader := parser.NewBinaryReader(sd.FileData)
	header, _, err := parser.ParseHeader(headerReader)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}

	readInt32At() // SaveHeaderType
	readInt32At() // SaveVersion
	readInt32At() // BuildVersion
	if header.SaveHeaderType >= 14 {
		skipString() // SaveName
	}
	skipString() // MapName
	skipString() // MapOptions

	sessionStart := pos
	sessionLen := readInt32At()
	// A negative length prefix means the original name was stored as
	// UTF-16LE (used whenever it contains non-ASCII characters, e.g.
	// umlauts); positive means raw ASCII. Write the new name back in the
	// same encoding the file already uses for this field.
	sessionIsUTF16 := sessionLen < 0
	sessionDataEnd := pos + int(sessionLen)
	if sessionIsUTF16 {
		sessionDataEnd = pos + int(-sessionLen)*2
	}

	// An ASCII field can't hold non-ASCII characters, and the body is patched in
	// place, so there's no room to switch this save's name over to UTF-16.
	if !sessionIsUTF16 {
		for _, r := range newSessionName {
			if r > unicode.MaxASCII {
				return nil, fmt.Errorf("new session name contains non-ASCII characters, which this save's name field cannot store")
			}
		}
	}

	// Build new header with patched session name FString
	var newHeader []byte
	newHeader = append(newHeader, data[:sessionStart]...)
	newHeader = append(newHeader, buildFStringBytes(newSessionName, sessionIsUTF16)...)
	newHeader = append(newHeader, data[sessionDataEnd:]...)

	// Update sd.FileData with the patched header
	sd.FileData = newHeader
	sd.HeaderBytes = newHeader // keep HeaderBytes in sync for the nil-FileData path

	// Recalculate HeaderEnd by re-parsing
	verifyReader := parser.NewBinaryReader(sd.FileData)
	_, _, err = parser.ParseHeader(verifyReader)
	if err != nil {
		return nil, fmt.Errorf("patched header verification failed: %w", err)
	}
	sd.HeaderEnd = verifyReader.Position()

	// --- Step 2: Patch the body (decompressed) ---
	// Search for the old session name FString bytes and replace with new ones.
	// We use the null-padding trick: keep the original FString length prefix,
	// write the new name + null terminator, and pad remaining bytes with 0x00.
	// The FString parser stops at the first null, so it reads the shorter name.
	// This is safe because the total byte count doesn't change - no offset shifts.
	bodyReplacements := 0

	if originalName != newSessionName {
		// The body may store the session name in the same FString encoding
		// (ASCII or UTF-16LE) as the header field we just read; search using
		// that same encoding so non-ASCII names are actually found instead
		// of silently matching zero occurrences.
		searchPattern := buildFStringBytes(originalName, sessionIsUTF16)
		newContent := encodeFStringContent(newSessionName, sessionIsUTF16)
		if sessionIsUTF16 {
			newContent = append(newContent, 0, 0)
		} else {
			newContent = append(newContent, 0)
		}

		if len(newContent) > len(searchPattern)-4 {
			return nil, fmt.Errorf("new session name is longer than the original (%d vs %d encoded bytes). "+
				"Please use a name that is not longer than the original to avoid corrupting the save body",
				len(newContent), len(searchPattern)-4)
		}

		// Build replacement: same total length, null-padded
		replacement := make([]byte, len(searchPattern))
		copy(replacement[:4], searchPattern[:4]) // keep original length prefix
		copy(replacement[4:], newContent)
		// Remaining bytes are already 0 (null padding)

		bodyReplacements = bytesReplaceAll(sd.Decompressed, searchPattern, replacement)
		if bodyReplacements == 0 {
			// Writing now would leave the header and body disagreeing about the name.
			return nil, fmt.Errorf("session name %q was not found in the save body, nothing was changed", originalName)
		}
	}

	// --- Step 3: Determine output path and write ---
	outputPath := savePath
	if !overwrite {
		dir := filepath.Dir(savePath)
		if outputDir != "" {
			dir = outputDir
		}
		base := filepath.Base(savePath)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		outputPath = filepath.Join(dir, name+"_sessionfix"+ext)
	}

	if err := WriteSave(sd, outputPath); err != nil {
		return nil, fmt.Errorf("writing fixed save: %w", err)
	}

	return &FixSessionNameResult{
		OutputPath:       outputPath,
		OriginalName:     originalName,
		NewName:          newSessionName,
		BodyReplacements: bodyReplacements,
	}, nil
}

// bytesReplaceAll replaces all occurrences of old with new in data (in-place).
// Both old and new must be the same length. Returns the number of replacements made.
func bytesReplaceAll(data []byte, old, new []byte) int {
	if len(old) == 0 || len(data) < len(old) || len(old) != len(new) {
		return 0
	}

	count := 0
	i := 0
	for i <= len(data)-len(old) {
		j := bytes.Index(data[i:], old)
		if j < 0 {
			break
		}
		i += j
		copy(data[i:i+len(new)], new)
		count++
		i += len(old)
	}
	return count
}
