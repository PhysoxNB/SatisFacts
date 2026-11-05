package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"satisfacts/editor"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type OptimizeResultInfo struct {
	OriginalSizeMB float64 `json:"originalSizeMB"`
	NewSizeMB      float64 `json:"newSizeMB"`
	SavingsMB      float64 `json:"savingsMB"`
	SavingsPct     float64 `json:"savingsPct"`
	TotalObjects   int     `json:"totalObjects"`
	LevelCount     int     `json:"levelCount"`
}

// OptimizeSave reorganizes objects for better compression and writes the output
func (a *App) OptimizeSave(savePath, outputPath string) (result *OptimizeResultInfo, err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not parse %q — the file may be corrupt (details: %v)", savePath, r)
		}
	}()

	a.mu.Lock()
	if a.optimizeRunning {
		a.mu.Unlock()
		return nil, errOptimizeBusy
	}
	a.optimizeRunning = true
	a.optimizeCancel = make(chan struct{})
	optimizeCancel := a.optimizeCancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.optimizeCancel = nil
		a.optimizeRunning = false
		a.mu.Unlock()
	}()

	errCancelled := fmt.Errorf("optimization cancelled")
	checkCancel := func() error {
		select {
		case <-optimizeCancel:
			return errCancelled
		default:
			return nil
		}
	}

	sd, err := editor.LoadSave(savePath)
	if err != nil {
		return nil, err
	}
	// Free the compressed file data — we only need the header bytes for writing.
	sd.FreeFileData()
	if err := checkCancel(); err != nil {
		return nil, err
	}

	origSize, err := statSize(savePath)
	if err != nil {
		return nil, err
	}

	wailsruntime.EventsEmit(a.ctx, "optimize:progress", map[string]interface{}{
		"phase":   "optimizing",
		"current": 0,
		"total":   0,
	})

	// Cancellation is checked from inside the progress callback rather than only
	// around the call, so pressing Cancel aborts partway through instead of waiting
	// out the whole run on a multi-GB save.
	optResult, err := editor.OptimizeSaveToFile(sd, outputPath, func(current, total int, _ string) error {
		if err := checkCancel(); err != nil {
			return err
		}
		phase := "optimizing"
		if current >= total {
			// The last chunk still has to be written and the file synced to disk.
			// A total of 0 animates the bar instead of leaving it sitting full.
			phase, current, total = "finishing", 0, 0
		}
		wailsruntime.EventsEmit(a.ctx, "optimize:progress", map[string]interface{}{
			"phase":   phase,
			"current": current,
			"total":   total,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Release the decompressed body (~16 GB on large saves) before computing
	// the result. Without this, sd stays alive until the method returns and
	// the GC can't reclaim the memory, leaving the UI holding 30+ GB.
	sd.Decompressed = nil
	sd.FileData = nil
	sd.HeaderBytes = nil
	runtime.GC()
	debug.FreeOSMemory()

	newSize, err := statSize(outputPath)
	if err != nil {
		return nil, err
	}
	savings := origSize - newSize

	result = &OptimizeResultInfo{
		OriginalSizeMB: float64(origSize) / 1024 / 1024,
		NewSizeMB:      float64(newSize) / 1024 / 1024,
		SavingsMB:      float64(savings) / 1024 / 1024,
		SavingsPct:     float64(savings) / float64(origSize) * 100,
		TotalObjects:   optResult.TotalObjects,
		LevelCount:     len(optResult.LevelStats),
	}
	return result, nil
}

// FormatDuration formats a duration for display
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

// OpenBlueprintDir opens the blueprint directory for the given save file's session.
// Satisfactory stores blueprints at <SaveGames>/blueprints/<SessionName>/,
// where SaveGames is the parent of the per-user save folder.
// If the folder doesn't exist yet, it is created so the user can paste files into it.
