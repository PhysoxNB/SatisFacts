package main

import (
	"fmt"
	"math"
	"runtime/debug"

	"satisfacts/editor"
)

type MultiplierInfo struct {
	Name    string  `json:"name"`
	Label   string  `json:"label"`
	Current float32 `json:"current"`
	Target  float32 `json:"target"`
	UIMax   float32 `json:"uiMax"`
}

// GetMultipliers loads a save and reads the current multiplier values
func (a *App) GetMultipliers(savePath string) (result []MultiplierInfo, err error) {
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

	gsi, err := editor.FindGameState(sd)
	if err != nil {
		return nil, err
	}

	objData := sd.Decompressed[gsi.VPos : gsi.VPos+int64(gsi.Size)]

	multipliers := []struct {
		name  string
		label string
		uiMax float32
	}{
		{"mPartsCostMultiplier", "Recipe Cost", 2.0},
		{"mSpacePartsCostMultiplier", "Space Elevator Cost", 100.0},
		{"mEnergyCostMultiplier", "Power Consumption", 5.0},
	}

	result = make([]MultiplierInfo, len(multipliers))
	for i, m := range multipliers {
		offset, val := editor.FindFloatPropOffset(objData, m.name, gsi.IsActor, gsi.UseCompleteTag)
		mi := MultiplierInfo{
			Name:    m.name,
			Label:   m.label,
			Current: val,
			Target:  val,
			UIMax:   m.uiMax,
		}
		if offset < 0 {
			mi.Current = -1
			mi.Target = 1.0
		}
		result[i] = mi
	}

	return result, nil
}

// PatchMultipliers applies multiplier changes to a save file
func (a *App) PatchMultipliers(savePath, outputPath string, multipliers []MultiplierInfo) (err error) {
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

	gsi, err := editor.FindGameState(sd)
	if err != nil {
		return err
	}

	// First pass: insert any missing multipliers
	needsInsert := false
	for _, m := range multipliers {
		if m.Target < 0 {
			continue
		}
		objData := sd.Decompressed[gsi.VPos : gsi.VPos+int64(gsi.Size)]
		offset, _ := editor.FindFloatPropOffset(objData, m.Name, gsi.IsActor, gsi.UseCompleteTag)
		if offset < 0 {
			needsInsert = true
			break
		}
	}

	if needsInsert {
		gsiFull, err := editor.FindGameStateFull(sd)
		if err != nil {
			return err
		}
		for _, m := range multipliers {
			if m.Target < 0 {
				continue
			}
			objData := sd.Decompressed[gsiFull.BinaryDataStart : gsiFull.BinaryDataStart+gsiFull.BinaryDataSize]
			offset, _ := editor.FindFloatPropOffset(objData, m.Name, gsiFull.IsActor, gsiFull.UseCompleteTag)
			if offset < 0 {
				if err := editor.InsertFloatProperty(sd, gsiFull, m.Name, m.Target); err != nil {
					return fmt.Errorf("inserting %s: %w", m.Name, err)
				}
				gsiFull, err = editor.FindGameStateFull(sd)
				if err != nil {
					return err
				}
			}
		}
		// Re-find GameState with simple info for in-place patching of existing ones
		gsi, err = editor.FindGameState(sd)
		if err != nil {
			return err
		}
	}

	// Second pass: patch existing multipliers in-place
	for _, m := range multipliers {
		if m.Target < 0 {
			continue
		}
		objData := sd.Decompressed[gsi.VPos : gsi.VPos+int64(gsi.Size)]
		offset, _ := editor.FindFloatPropOffset(objData, m.Name, gsi.IsActor, gsi.UseCompleteTag)
		if offset < 0 {
			continue
		}
		absOffset := int(gsi.VPos) + offset
		bits := math.Float32bits(m.Target)
		sd.Decompressed[absOffset] = byte(bits)
		sd.Decompressed[absOffset+1] = byte(bits >> 8)
		sd.Decompressed[absOffset+2] = byte(bits >> 16)
		sd.Decompressed[absOffset+3] = byte(bits >> 24)
	}

	return editor.WriteSave(sd, outputPath)
}

// OptimizeResultInfo holds the result of an optimize operation for the frontend
