package main

import (
	"fmt"

	"satisfacts/editor"
)

// BeltInventoryInfo is the frontend-facing inventory response.
type BeltInventoryInfo struct {
	Items     []editor.BeltInventoryEntry `json:"items"`
	HubTier   int                         `json:"hubTier"`
	MaxBeltMk int                         `json:"maxBeltMk"`
	MaxLiftMk int                         `json:"maxLiftMk"`
	MaxPipeMk int                         `json:"maxPipeMk"`
}

// BeltUpgradeOptionsFrontend mirrors editor.UpgradeBeltsOptions for the frontend.
type BeltUpgradeOptionsFrontend struct {
	UpgradeBelts bool `json:"upgradeBelts"`
	UpgradeLifts bool `json:"upgradeLifts"`
	UpgradePipes bool `json:"upgradePipes"`
	TargetMk     int  `json:"targetMk"`
	TargetPipeMk int  `json:"targetPipeMk"`
}

// GetBeltInventory loads a save and returns counts of belts/lifts/pipes per Mk,
// plus the detected HUB tier and max unlocked Mk for each type.
func (a *App) GetBeltInventory(savePath string) (info *BeltInventoryInfo, err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not parse %q — the file may be corrupt (details: %v)", savePath, r)
		}
	}()

	sd, err := editor.LoadSave(savePath)
	if err != nil {
		return nil, err
	}

	items, err := editor.ScanBeltInventory(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		return nil, err
	}

	hubTier := editor.DetectHubTier(sd.Decompressed)
	maxBelt, maxLift, maxPipe := editor.MaxUnlockedMk(hubTier)

	return &BeltInventoryInfo{
		Items:     items,
		HubTier:   hubTier,
		MaxBeltMk: maxBelt,
		MaxLiftMk: maxLift,
		MaxPipeMk: maxPipe,
	}, nil
}

// UpgradeBeltsSave applies belt/lift/pipe upgrades to a save file and writes the output.
func (a *App) UpgradeBeltsSave(savePath, outputPath string, opts BeltUpgradeOptionsFrontend) (err error) {
	defer lowMemoryGC()()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not parse %q — the file may be corrupt (details: %v)", savePath, r)
		}
	}()

	sd, err := editor.LoadSave(savePath)
	if err != nil {
		return err
	}

	_, err = editor.UpgradeBelts(sd, editor.UpgradeBeltsOptions{
		UpgradeBelts: opts.UpgradeBelts,
		UpgradeLifts: opts.UpgradeLifts,
		UpgradePipes: opts.UpgradePipes,
		TargetMk:     opts.TargetMk,
		TargetPipeMk: opts.TargetPipeMk,
	})
	if err != nil {
		return err
	}

	return editor.WriteSave(sd, outputPath)
}
