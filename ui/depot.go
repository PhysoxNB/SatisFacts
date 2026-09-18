package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"satisfacts/editor"
	"satisfacts/extraction"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type SaveFileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	ModTime  string `json:"modTime"`
	Selected bool   `json:"selected"`
}

// ScanSaveDir scans a directory for .sav files and returns them sorted by modification time (newest first)
func (a *App) ScanSaveDir(dirPath string) ([]SaveFileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var files []SaveFileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".sav") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, SaveFileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(dirPath, entry.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime > files[j].ModTime
	})

	return files, nil
}

// StartAutoDetect starts a background goroutine that polls the given save directory
// for the newest .sav file. When a new or modified save is detected, it emits a
// "save:auto-detected" event with the file path. Calling again stops any previous goroutine.
func (a *App) StartAutoDetect(saveDir string) error {
	a.mu.Lock()
	// Every settings change re-syncs the watcher. Restarting it would forget the
	// current save and re-select the newest one, wiping the user's open pages.
	if a.autoDetectCancel != nil && a.autoDetectDir == saveDir {
		a.mu.Unlock()
		return nil
	}
	if a.autoDetectCancel != nil {
		close(a.autoDetectCancel)
		a.autoDetectCancel = nil
	}
	a.mu.Unlock()

	if saveDir == "" {
		return fmt.Errorf("no save directory specified")
	}
	if _, err := os.Stat(saveDir); err != nil {
		return fmt.Errorf("save directory not accessible: %w", err)
	}

	a.mu.Lock()
	a.autoDetectCancel = make(chan struct{})
	a.autoDetectDir = saveDir
	cancel := a.autoDetectCancel
	a.mu.Unlock()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var tracker saveTracker

		for {
			select {
			case <-cancel:
				return
			case <-ticker.C:
			}

			// Switching saves mid-run would reset the page showing its progress.
			// The new save is picked up on a later tick instead.
			a.mu.Lock()
			busy := a.analyzeRunning || a.optimizeRunning
			a.mu.Unlock()
			if busy {
				continue
			}

			newest, modTime, size, ok := newestSavFile(saveDir)
			if !ok {
				continue
			}
			if tracker.observe(newest, modTime, size) {
				wailsruntime.EventsEmit(a.ctx, "save:auto-detected", newest)
			}
		}
	}()

	return nil
}

// saveTracker decides when a watched save is worth reporting: only when it's new
// or changed, and only once it has stopped changing between two looks, so a save
// the game is still writing isn't opened half-finished.
type saveTracker struct {
	lastPath    string
	lastModTime time.Time
	pendingPath string
	pendingMod  time.Time
	pendingSize int64
}

func (t *saveTracker) observe(path string, modTime time.Time, size int64) bool {
	if path == t.lastPath && !modTime.After(t.lastModTime) {
		return false
	}
	if path == t.pendingPath && modTime.Equal(t.pendingMod) && size == t.pendingSize {
		t.lastPath, t.lastModTime = path, modTime
		t.pendingPath = ""
		return true
	}
	t.pendingPath, t.pendingMod, t.pendingSize = path, modTime, size
	return false
}

// StopAutoDetect stops the auto-detect goroutine if running.
func (a *App) StopAutoDetect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.autoDetectCancel != nil {
		close(a.autoDetectCancel)
		a.autoDetectCancel = nil
	}
	a.autoDetectDir = ""
}

// toolOutputSuffixes are the suffixes this app adds to the saves it writes.
// Writing one next to the original must not make the watcher jump over to it.
var toolOutputSuffixes = []string{"_optimized", "_reordered", "_upgraded", "_multipliers", "_blueprints", "_sessionfix"}

func isToolOutput(name string) bool {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	for _, suffix := range toolOutputSuffixes {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	return false
}

func newestSavFile(dir string) (path string, modTime time.Time, size int64, ok bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", time.Time{}, 0, false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".sav") || isToolOutput(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !ok || info.ModTime().After(modTime) {
			modTime = info.ModTime()
			size = info.Size()
			path = filepath.Join(dir, entry.Name())
			ok = true
		}
	}
	return path, modTime, size, ok
}

// dialogText picks the file dialog wording for the current language. Dialogs are
// opened by the backend, so they don't go through the frontend's translations.
func dialogText(en, de string) string {
	if s := LoadSettings(); s != nil && s.Language == "de" {
		return de
	}
	return en
}

// BrowseDir opens a directory picker dialog and returns the selected path
func (a *App) BrowseDir() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: dialogText("Select Directory", "Verzeichnis auswählen"),
	})
}

// BrowseSaveFile opens a file picker for .sav files
func (a *App) BrowseSaveFile() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: dialogText("Select Save File", "Save-File auswählen"),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Satisfactory Save (*.sav)", Pattern: "*.sav"},
		},
	})
}

// GetSatisfactorySaveDir returns the default Satisfactory save directory for the current OS.
func (a *App) GetSatisfactorySaveDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	var candidates []string
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		fgBase := filepath.Join(localAppData, "FactoryGame", "Saved", "SaveGames")
		// FactoryGame has subdirectories per user ID - scan for them
		candidates = []string{fgBase}
	case "darwin":
		candidates = []string{
			filepath.Join(home, "Library", "Application Support", "Satisfactory", "Saved", "SaveGames"),
		}
	default: // linux (Steam Play/Proton)
		for _, library := range steamLibraries(home) {
			candidates = append(candidates, filepath.Join(protonFactoryGameDir(library), "SaveGames"))
		}
	}

	// FactoryGame saves are in subdirectories per user ID (e.g. <steamid>).
	// If we found the SaveGames dir, scan its subdirs and return the one with .sav files.
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			// Look for subdirectories containing .sav files
			entries, err := os.ReadDir(p)
			if err != nil {
				return p, nil
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				sub := filepath.Join(p, entry.Name())
				if hasSavFiles(sub) {
					return sub, nil
				}
			}
			// No subdir with .sav files - return the parent
			return p, nil
		}
	}

	return candidates[0], fmt.Errorf("%s", candidates[0])
}

func hasSavFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".sav") {
			return true
		}
	}
	return false
}

// GetAutoDetectPath returns the directory to watch for auto-detect.
// If the user has set a custom save dir in settings, it is used (if valid).
// Otherwise, the default Satisfactory save directory is returned.
func (a *App) GetAutoDetectPath() (string, error) {
	s := LoadSettings()
	if s.CustomSaveDir != "" {
		if info, err := os.Stat(s.CustomSaveDir); err == nil && info.IsDir() {
			return s.CustomSaveDir, nil
		}
	}
	return a.GetSatisfactorySaveDir()
}

// SaveSummary holds basic info about a loaded save file
type SaveSummary struct {
	FileName       string  `json:"fileName"`
	FilePath       string  `json:"filePath"`
	FileSize       int64   `json:"fileSize"`
	DecompressedMB float64 `json:"decompressedMB"`
	SessionName    string  `json:"sessionName"`
	SaveVersion    int32   `json:"saveVersion"`
	IsV2           bool    `json:"isV2"`
	MaxChunkSize   int32   `json:"maxChunkSize"`
}

// LoadSaveSummary loads a save file's header and returns basic info without decompressing the body.
func (a *App) LoadSaveSummary(savePath string) (summary *SaveSummary, err error) {
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not parse %q — the file may be corrupt or from an unsupported game version (details: %v)", savePath, r)
		}
	}()

	sh, err := editor.LoadSaveHeader(savePath)
	if err != nil {
		return nil, err
	}

	// Track in recent saves
	saveName := filepath.Base(savePath)
	AddRecentSave(savePath, saveName)

	fileSize, err := statSize(savePath)
	if err != nil {
		return nil, err
	}

	return &SaveSummary{
		FileName:       filepath.Base(savePath),
		FilePath:       savePath,
		FileSize:       fileSize,
		DecompressedMB: 0,
		SessionName:    sh.Header.SessionName,
		SaveVersion:    sh.Header.SaveVersion,
		IsV2:           sh.IsV2,
		MaxChunkSize:   sh.MaxChunkSize,
	}, nil
}

// DepotItemInfo represents a depot item for the frontend
type DepotItemInfo struct {
	Index       int    `json:"index"`
	ItemClass   string `json:"itemClass"`
	DisplayName string `json:"displayName"`
	Amount      int32  `json:"amount"`
	StackSize   int    `json:"stackSize"`
}

// LoadDepotItems loads a save file and extracts the Dimensional Depot items
func (a *App) LoadDepotItems(savePath string) (items []DepotItemInfo, err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not parse %q — the file may be corrupt (details: %v)", savePath, r)
		}
	}()

	sd, err := editor.LoadSave(savePath)
	if err != nil {
		return nil, err
	}

	_, levels, _, _, err := editor.ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		return nil, fmt.Errorf("parsing body: %w", err)
	}

	for _, l := range levels {
		if !l.IsPersistent || l.ObjectCount == 0 {
			continue
		}
		tocData := sd.Decompressed[l.TocDataStart:l.TocDataEnd]
		idx, header, ferr := editor.FindCentralStorageSubsystem(tocData, sd.VersionCtx, int(l.ObjectCount))
		if ferr != nil {
			continue
		}

		objStart := l.DataObjOffsets[idx]
		objEnd := objStart + l.DataObjSizes[idx]
		objData := sd.Decompressed[l.DataBlobStart+objStart : l.DataBlobStart+objEnd]
		saveCustomVersion := l.SaveCustomVersions[idx]

		items, err := editor.FindStoredItemsArray(objData, header, sd.VersionCtx, saveCustomVersion)
		if err != nil {
			return nil, err
		}

		result := make([]DepotItemInfo, len(items))
		for i, item := range items {
			name := extraction.CleanItemName(item.ItemClass)
			result[i] = DepotItemInfo{
				Index:       i,
				ItemClass:   item.ItemClass,
				DisplayName: name,
				Amount:      item.Amount,
				StackSize:   extraction.GetStackSize(name),
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("FGCentralStorageSubsystem not found in save")
}

// ReorderDepot reorders the depot items and writes a new save file
func (a *App) ReorderDepot(savePath, outputPath string, newOrder []int) (err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not parse %q — the file may be corrupt (details: %v)", savePath, r)
		}
	}()

	sd, err := editor.LoadSave(savePath)
	if err != nil {
		return err
	}

	if err := editor.ReorderDepotItems(sd, newOrder); err != nil {
		return err
	}

	return editor.WriteSave(sd, outputPath)
}

// MultiplierInfo holds current and target multiplier values
