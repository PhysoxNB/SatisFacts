package main

import (
	"fmt"

	"satisfacts/editor"
	"satisfacts/parser"
)

// CheckSessionName validates a session name for characters that break blueprints.
func (a *App) CheckSessionName(name string) *editor.SessionNameCheckResult {
	return editor.CheckSessionName(name)
}

// FixSessionName patches the session name in both the header and body of a save file.
// If overwrite is true, the original file is overwritten; otherwise a _sessionfix copy is created.
func (a *App) FixSessionName(savePath string, newSessionName string, overwrite bool, outputDir string) (result *editor.FixSessionNameResult, err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not fix session name (details: %v)", r)
		}
	}()

	sd, err := editor.LoadSave(savePath)
	if err != nil {
		return nil, fmt.Errorf("loading save: %w", err)
	}

	headerReader := parser.NewBinaryReader(sd.FileData)
	header, _, err := parser.ParseHeader(headerReader)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}

	return editor.FixSessionName(sd, header.SessionName, newSessionName, overwrite, savePath, outputDir)
}
