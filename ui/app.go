package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"satisfacts/extraction"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx              context.Context
	mu               sync.Mutex
	autoDetectCancel chan struct{}
	autoDetectDir    string
	analyzeCancel    chan struct{}
	optimizeCancel   chan struct{}
	analyzeRunning   bool
	optimizeRunning  bool
}

// ErrAnalyzeBusy / ErrOptimizeBusy are returned when a second run is started
// while one is already in flight. Without this guard the second run overwrites
// the shared cancel channel and the first run's deferred cleanup nils it out,
// leaving Cancel a no-op for the run that is actually still going.
var (
	errAnalyzeBusy  = fmt.Errorf("an analysis is already running")
	errOptimizeBusy = fmt.Errorf("an optimization is already running")
)

// statSize returns the size of path, or an error naming the file.
func statSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("reading size of %s: %w", filepath.Base(path), err)
	}
	return info.Size(), nil
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load data files (item display names, recipes, etc.) for use across all features
	_ = extraction.LoadDataFiles("")

	// Apply saved settings on startup
	if s := LoadSettings(); s != nil {
		extraction.SetItemDisplayLanguage(s.Language)

		// Restore window size if saved
		if s.WindowWidth > 0 && s.WindowHeight > 0 {
			wailsruntime.WindowSetSize(ctx, s.WindowWidth, s.WindowHeight)
		}
	}

	// Register file drop handler - emit event when a .sav file is dropped
	wailsruntime.OnFileDrop(ctx, func(x, y int, paths []string) {
		for _, p := range paths {
			if strings.HasSuffix(strings.ToLower(p), ".sav") {
				wailsruntime.EventsEmit(ctx, "file:drop", p)
				return
			}
		}
	})
}

// OpenFile opens a file in the system default application (e.g. HTML report in browser)
func (a *App) OpenFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// GetExeDir returns the directory next to the app, for output file placement.
func (a *App) GetExeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return appDir(exePath)
}

// appDir returns the folder the app sits in. On macOS the executable lives inside
// SatisFacts.app/Contents/MacOS, and output must land next to the bundle, not in it.
func appDir(exePath string) string {
	dir := filepath.Dir(exePath)
	suffix := filepath.Join("Contents", "MacOS")
	if strings.HasSuffix(dir, suffix) {
		bundle := strings.TrimSuffix(strings.TrimSuffix(dir, suffix), string(filepath.Separator))
		if strings.HasSuffix(bundle, ".app") {
			return filepath.Dir(bundle)
		}
	}
	return dir
}

// ValidateSaveFile checks that a save file exists, is readable, non-empty, and has a .sav extension.
// Returns nil if valid, or an error with a user-friendly message.
func (a *App) ValidateSaveFile(path string) error {
	if path == "" {
		return fmt.Errorf("no save file selected")
	}
	if !strings.HasSuffix(strings.ToLower(path), ".sav") {
		return fmt.Errorf("selected file is not a .sav file: %s", filepath.Base(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", filepath.Base(path))
		}
		return fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("selected path is a directory, not a file: %s", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("file is empty (0 bytes): %s", filepath.Base(path))
	}
	return nil
}

// ValidateOutputDir checks that a directory exists and is writable.
func (a *App) ValidateOutputDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("no output directory specified")
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory does not exist: %s", dir)
		}
		return fmt.Errorf("cannot access output directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path is not a directory: %s", dir)
	}
	// Try creating a temp file to verify write permission
	tmp := filepath.Join(dir, ".satisfacts_write_test")
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("output directory is not writable: %s", dir)
	}
	f.Close()
	os.Remove(tmp)
	return nil
}

// SaveFileInfo represents a .sav file found in a directory

func (a *App) shutdown(ctx context.Context) {
	a.StopAutoDetect()
	a.CancelAnalyze()
	a.CancelOptimize()
}

// CancelAnalyze cancels a running analysis if one is in progress.
func (a *App) CancelAnalyze() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.analyzeCancel != nil {
		close(a.analyzeCancel)
		a.analyzeCancel = nil
	}
}

// CancelOptimize cancels a running optimization if one is in progress.
func (a *App) CancelOptimize() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.optimizeCancel != nil {
		close(a.optimizeCancel)
		a.optimizeCancel = nil
	}
}
