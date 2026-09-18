package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"satisfacts/editor"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) OpenBlueprintDir(savePath string) error {
	// Read the session name from the save header
	sh, err := editor.LoadSaveHeader(savePath)
	if err != nil {
		return fmt.Errorf("could not read save header: %w", err)
	}
	sessionName := sh.Header.SessionName
	if sessionName == "" {
		return fmt.Errorf("save file has no session name")
	}

	// Save files are in <SaveGames>/<steamid>/<file>.sav
	// Blueprints are in <SaveGames>/blueprints/<SessionName>/
	saveDir := filepath.Dir(savePath)     // <SaveGames>/<steamid>
	saveGamesDir := filepath.Dir(saveDir) // <SaveGames>
	bpDir := filepath.Join(saveGamesDir, "blueprints", sessionName)

	if _, err := os.Stat(bpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(bpDir, 0755); err != nil {
			return fmt.Errorf("could not create blueprint directory: %w", err)
		}
	}

	return a.OpenFile(bpDir)
}

// GetBlueprintCategories loads a save and returns its blueprint category structure.
func (a *App) GetBlueprintCategories(savePath string) (categories []editor.BlueprintCategoryRecord, err error) {
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

	info, err := editor.FindBlueprintSubsystem(sd)
	if err != nil {
		return nil, err
	}

	return editor.ExtractCategoryRecords(info, sd)
}

// CopyBlueprintCategoriesResult holds the result of a copy operation for the frontend.
type CopyBlueprintCategoriesResult struct {
	SourceCategories int    `json:"sourceCategories"`
	TargetCategories int    `json:"targetCategories"`
	FinalCategories  int    `json:"finalCategories"`
	Mode             string `json:"mode"`
}

// CopyBlueprintCategories copies blueprint categories from source save to target save.
func (a *App) CopyBlueprintCategories(sourcePath, targetPath, outputPath, mode string) (result *CopyBlueprintCategoriesResult, err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not copy blueprint categories (details: %v)", r)
		}
	}()

	sourceSD, err := editor.LoadSave(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("loading source: %w", err)
	}

	targetSD, err := editor.LoadSave(targetPath)
	if err != nil {
		return nil, fmt.Errorf("loading target: %w", err)
	}

	res, err := editor.CopyBlueprintCategories(sourceSD, targetSD, mode)
	if err != nil {
		return nil, err
	}

	if err := editor.WriteSave(targetSD, outputPath); err != nil {
		return nil, fmt.Errorf("writing output: %w", err)
	}

	return &CopyBlueprintCategoriesResult{
		SourceCategories: res.SourceCategories,
		TargetCategories: res.TargetCategories,
		FinalCategories:  res.FinalCategories,
		Mode:             res.Mode,
	}, nil
}

// BrowsePresetFile opens a file picker for .sfbcat preset files.
func (a *App) BrowsePresetFile() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: dialogText("Select Blueprint Preset File", "Bauplan-Preset auswählen"),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SatisFacts Blueprint Preset (*.sfbcat)", Pattern: "*.sfbcat"},
		},
	})
}

// ReadPresetCategories reads a .sfbcat preset file and returns its category records for preview.
func (a *App) ReadPresetCategories(presetPath string) (categories []editor.BlueprintCategoryRecord, err error) {
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not read preset file (details: %v)", r)
		}
	}()

	return editor.ReadPresetCategories(presetPath)
}

// ExportBlueprintPresetResult holds the result of an export operation for the frontend.
type ExportBlueprintPresetResult struct {
	Categories int `json:"categories"`
}

// ExportBlueprintPreset extracts blueprint categories from a save and writes them to a .sfbcat file.
func (a *App) ExportBlueprintPreset(savePath, outputPath string) (result *ExportBlueprintPresetResult, err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not export blueprint preset (details: %v)", r)
		}
	}()

	cats, err := editor.ExportBlueprintPreset(savePath, outputPath)
	if err != nil {
		return nil, err
	}

	return &ExportBlueprintPresetResult{Categories: len(cats)}, nil
}

// ImportBlueprintPreset imports categories from a .sfbcat preset file into a target save.
func (a *App) ImportBlueprintPreset(presetPath, targetPath, outputPath, mode string) (result *CopyBlueprintCategoriesResult, err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			err = fmt.Errorf("could not import blueprint preset (details: %v)", r)
		}
	}()

	res, targetSD, err := editor.ImportBlueprintPreset(presetPath, targetPath, mode)
	if err != nil {
		return nil, err
	}

	if err := editor.WriteSave(targetSD, outputPath); err != nil {
		return nil, fmt.Errorf("writing output: %w", err)
	}

	return &CopyBlueprintCategoriesResult{
		SourceCategories: res.SourceCategories,
		TargetCategories: res.TargetCategories,
		FinalCategories:  res.FinalCategories,
		Mode:             res.Mode,
	}, nil
}
