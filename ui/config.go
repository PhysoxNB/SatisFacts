package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ConfigFileInfo holds the current state of a game config file.
type ConfigFileInfo struct {
	Exists    bool   `json:"exists"`
	Path      string `json:"path"`
	HasEntry  bool   `json:"hasEntry"`
	HasBackup bool   `json:"hasBackup"`
	ReadOnly  bool   `json:"readOnly"`
	Content   string `json:"content"`
	FileError string `json:"error"`
}

// GameConfigStatus holds the status of both Engine.ini and Input.ini.
type GameConfigStatus struct {
	ConfigDir string         `json:"configDir"`
	EngineIni ConfigFileInfo `json:"engineIni"`
	InputIni  ConfigFileInfo `json:"inputIni"`
}

const (
	gcSection = "[/Script/Engine.GarbageCollectionSettings]"
	gcEditor  = "gc.MaxObjectsInEditor=100000000"
	gcGame    = "gc.MaxObjectsInGame=100000000"

	inputSection = "[/Script/Engine.InputSettings]"
)

// detectGameConfigDir returns the Satisfactory config directory.
// On Windows: %LOCALAPPDATA%\FactoryGame\Saved\Config\Windows
// On Linux: Steam Proton prefix under compatdata/526870
func detectGameConfigDir() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			dir, err := os.UserConfigDir()
			if err != nil {
				return ""
			}
			localAppData = dir
		}
		candidate := filepath.Join(localAppData, "FactoryGame", "Saved", "Config", "Windows")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		return ""
	}

	// Linux: Satisfactory runs via Proton. Config is inside the Wine prefix.
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	for _, library := range steamLibraries(home) {
		candidate := filepath.Join(protonFactoryGameDir(library), "Config", "Windows")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// protonFactoryGameDir returns the game's Saved folder inside a Steam library's
// Proton prefix.
func protonFactoryGameDir(library string) string {
	const steamAppID = "526870" // Satisfactory
	return filepath.Join(library, "steamapps", "compatdata", steamAppID, "pfx", "drive_c", "users", "steamuser", "AppData", "Local", "FactoryGame", "Saved")
}

var vdfLibraryPath = regexp.MustCompile(`"path"\s+"([^"]+)"`)

// steamLibraries returns every Steam library folder on Linux: the usual install
// locations, plus any others (e.g. on a second drive) they list in libraryfolders.vdf.
func steamLibraries(home string) []string {
	installs := []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
	}

	libraries := append([]string{}, installs...)
	seen := make(map[string]bool)
	for _, install := range installs {
		seen[install] = true
	}
	for _, install := range installs {
		data, err := os.ReadFile(filepath.Join(install, "steamapps", "libraryfolders.vdf"))
		if err != nil {
			continue
		}
		for _, match := range vdfLibraryPath.FindAllStringSubmatch(string(data), -1) {
			if !seen[match[1]] {
				seen[match[1]] = true
				libraries = append(libraries, match[1])
			}
		}
	}
	return libraries
}

// GetGameConfigStatus reads the current state of Engine.ini and Input.ini.
func (a *App) GetGameConfigStatus(configDir string) (*GameConfigStatus, error) {
	if configDir == "" {
		configDir = detectGameConfigDir()
	}

	status := &GameConfigStatus{ConfigDir: configDir}
	if configDir == "" {
		return status, nil
	}

	status.EngineIni = readConfigFile(filepath.Join(configDir, "Engine.ini"))
	status.InputIni = readConfigFile(filepath.Join(configDir, "Input.ini"))
	return status, nil
}

// DetectGameConfigDir returns the auto-detected config directory path.
func (a *App) DetectGameConfigDir() string {
	return detectGameConfigDir()
}

func readConfigFile(path string) ConfigFileInfo {
	info := ConfigFileInfo{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			info.Exists = false
			return info
		}
		info.FileError = err.Error()
		return info
	}
	info.Exists = true
	info.Content = string(data)

	// Check read-only flag
	fi, err := os.Stat(path)
	if err == nil {
		info.ReadOnly = fi.Mode()&0200 == 0
	}

	// Check if our patched entry is already present (not just the section header,
	// which may exist in a default file without our modifications)
	if strings.HasSuffix(path, "Input.ini") {
		info.HasEntry = strings.Contains(info.Content, "ConsoleKeys=")
	} else {
		info.HasEntry = strings.Contains(info.Content, "gc.MaxObjectsInGame=")
	}

	// Check if a .bak backup exists
	if _, err := os.Stat(path + ".bak"); err == nil {
		info.HasBackup = true
	}
	return info
}

// ApplyEngineIniPatch adds or updates the UObject limit section in Engine.ini.
// Creates a .bak backup before modifying (preserves the first backup if one exists).
func (a *App) ApplyEngineIniPatch(configDir string) error {
	if configDir == "" {
		configDir = detectGameConfigDir()
	}
	if configDir == "" {
		return os.ErrNotExist
	}
	path := filepath.Join(configDir, "Engine.ini")
	return patchIniFile(path, gcSection, []string{gcEditor, gcGame})
}

// ApplyInputIniPatch adds the console key section to Input.ini.
// Sets the file to read-only after writing (game deletes contents otherwise).
func (a *App) ApplyInputIniPatch(configDir string, consoleKey string) error {
	if configDir == "" {
		configDir = detectGameConfigDir()
	}
	if configDir == "" {
		return os.ErrNotExist
	}
	if consoleKey == "" {
		consoleKey = "F6"
	}
	path := filepath.Join(configDir, "Input.ini")
	entry := "ConsoleKeys=" + consoleKey
	return patchIniFile(path, inputSection, []string{entry})
}

// SetInputIniReadOnly sets Input.ini to read-only (needed so the game doesn't wipe it).
func (a *App) SetInputIniReadOnly(configDir string, readOnly bool) error {
	if configDir == "" {
		configDir = detectGameConfigDir()
	}
	if configDir == "" {
		return os.ErrNotExist
	}
	path := filepath.Join(configDir, "Input.ini")
	if readOnly {
		return os.Chmod(path, 0444)
	}
	return os.Chmod(path, 0644)
}

// RestoreConfigFile restores a config file from its .bak backup.
func (a *App) RestoreConfigFile(configDir string, filename string) error {
	if configDir == "" {
		configDir = detectGameConfigDir()
	}
	if configDir == "" {
		return os.ErrNotExist
	}
	path := filepath.Join(configDir, filename)
	bakPath := path + ".bak"

	// Make sure we can write (Input.ini may be read-only)
	os.Chmod(path, 0644)

	data, err := os.ReadFile(bakPath)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// patchIniFile handles the common logic: backup, read, insert/update section, write.
func patchIniFile(path string, section string, entries []string) error {
	// Ensure file exists (create empty if not)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			return err
		}
	}

	// Make writable (Input.ini may be read-only)
	os.Chmod(path, 0644)

	// Create backup if one doesn't already exist
	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(bakPath, data, 0644); err != nil {
			return err
		}
	}

	// Read current content
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	// Build the section block
	var sectionBlock strings.Builder
	sectionBlock.WriteString(section + "\n")
	for _, e := range entries {
		sectionBlock.WriteString(e + "\n")
	}

	// Check if section already exists
	idx := strings.Index(content, section)
	if idx >= 0 {
		// Find the end of this section (next section header or EOF)
		rest := content[idx:]
		nextSection := findNextSection(rest, len(section))
		var sectionEnd int
		if nextSection >= 0 {
			sectionEnd = idx + nextSection
		} else {
			sectionEnd = len(content)
		}

		// Merge entries into the existing section instead of replacing it.
		// For each entry, replace the matching line if the key already exists,
		// otherwise append the entry before the section ends.
		existingBlock := content[idx:sectionEnd]
		for _, entry := range entries {
			key := entry
			if eq := strings.Index(entry, "="); eq >= 0 {
				key = entry[:eq+1]
			}
			// Check if this key already exists in the section
			keyIdx := strings.Index(existingBlock, key)
			if keyIdx >= 0 {
				// Find the line boundaries
				lineStart := strings.LastIndex(existingBlock[:keyIdx], "\n") + 1
				lineEnd := strings.Index(existingBlock[lineStart:], "\n")
				if lineEnd < 0 {
					lineEnd = len(existingBlock) - lineStart
				}
				existingBlock = existingBlock[:lineStart] + entry + existingBlock[lineStart+lineEnd:]
			} else {
				// Append before the section ends
				existingBlock = strings.TrimRight(existingBlock, "\n\r ") + "\n" + entry + "\n"
			}
		}
		content = content[:idx] + existingBlock + content[sectionEnd:]
	} else {
		// Append at the end with exactly one blank line separator
		content = strings.TrimRight(content, "\n\r ")
		if len(content) > 0 {
			content += "\n\n"
		}
		content += sectionBlock.String()
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// findNextSection finds the index of the next [Section] header after offset.
func findNextSection(s string, offset int) int {
	for i := offset; i < len(s); i++ {
		if s[i] == '[' && i > offset {
			// Make sure it's not inside a value - check it's at line start
			if i == 0 || s[i-1] == '\n' {
				return i
			}
		}
	}
	return -1
}
