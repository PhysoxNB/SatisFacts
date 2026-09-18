package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"satisfacts/extraction"
)

// Settings holds user preferences persisted to disk.
// RecentSaves is loaded from a separate file and merged in for the frontend API.
type Settings struct {
	RecentSaves      []RecentSave      `json:"recentSaves"`
	DefaultOutputLoc string            `json:"defaultOutputLoc"`
	CustomOutputDir  string            `json:"customOutputDir"`
	AutoDetectSave   bool              `json:"autoDetectSave"`
	AutoAnalyze      bool              `json:"autoAnalyze"`
	CustomSaveDir    string            `json:"customSaveDir"`
	Language         string            `json:"language"`
	ZoomFactor       float64           `json:"zoomFactor"`
	WindowWidth      int               `json:"windowWidth"`
	WindowHeight     int               `json:"windowHeight"`
	MapColors        map[string]string `json:"mapColors,omitempty"`
	IncludeMap       *bool             `json:"includeMap,omitempty"`
}

// preferencesFile is the on-disk format for settings.json (no recent saves).
type preferencesFile struct {
	DefaultOutputLoc string            `json:"defaultOutputLoc"`
	CustomOutputDir  string            `json:"customOutputDir"`
	AutoDetectSave   bool              `json:"autoDetectSave"`
	AutoAnalyze      bool              `json:"autoAnalyze"`
	CustomSaveDir    string            `json:"customSaveDir"`
	Language         string            `json:"language"`
	ZoomFactor       float64           `json:"zoomFactor"`
	WindowWidth      int               `json:"windowWidth"`
	WindowHeight     int               `json:"windowHeight"`
	MapColors        map[string]string `json:"mapColors,omitempty"`
	IncludeMap       *bool             `json:"includeMap,omitempty"`
}

// RecentSave represents a recently opened save file.
type RecentSave struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

const maxRecentSaves = 10
const settingsDirName = "SatisFacts"

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(dir, settingsDirName)
}

func settingsPath() string {
	return filepath.Join(configDir(), "settings.json")
}

func recentSavesPath() string {
	return filepath.Join(configDir(), "recent_saves.json")
}

// LoadSettings reads settings from disk, returning defaults if not found.
// Preferences and recent saves are loaded from separate files and merged.
func LoadSettings() *Settings {
	s := &Settings{
		DefaultOutputLoc: "exe",
		ZoomFactor:       1.0,
	}

	if data, err := os.ReadFile(settingsPath()); err == nil {
		var prefs preferencesFile
		if json.Unmarshal(data, &prefs) == nil {
			// A settings.json written by an older build (or hand-edited) can be
			// missing keys, and copying the zero value in blanked the defaults.
			if prefs.DefaultOutputLoc != "" {
				s.DefaultOutputLoc = prefs.DefaultOutputLoc
			}
			s.CustomOutputDir = prefs.CustomOutputDir
			s.AutoDetectSave = prefs.AutoDetectSave
			s.AutoAnalyze = prefs.AutoAnalyze
			s.CustomSaveDir = prefs.CustomSaveDir
			s.Language = prefs.Language
			s.MapColors = prefs.MapColors
			s.IncludeMap = prefs.IncludeMap
			if prefs.ZoomFactor > 0.0 {
				s.ZoomFactor = prefs.ZoomFactor
			}
			if prefs.WindowWidth > 0 && prefs.WindowHeight > 0 {
				s.WindowWidth = prefs.WindowWidth
				s.WindowHeight = prefs.WindowHeight
			}
		}
	}
	if s.Language == "" {
		s.Language = "en"
	}

	s.RecentSaves = loadRecentSaves()
	return s
}

func loadRecentSaves() []RecentSave {
	data, err := os.ReadFile(recentSavesPath())
	if err != nil {
		return nil
	}
	var saves []RecentSave
	if json.Unmarshal(data, &saves) == nil {
		return saves
	}
	return nil
}

func saveRecentSaves(saves []RecentSave) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(saves, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(recentSavesPath(), data, 0644)
}

// SaveSettings writes user preferences to disk (settings.json only).
func SaveSettings(s *Settings) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	prefs := preferencesFile{
		DefaultOutputLoc: s.DefaultOutputLoc,
		CustomOutputDir:  s.CustomOutputDir,
		AutoDetectSave:   s.AutoDetectSave,
		AutoAnalyze:      s.AutoAnalyze,
		CustomSaveDir:    s.CustomSaveDir,
		Language:         s.Language,
		ZoomFactor:       s.ZoomFactor,
		WindowWidth:      s.WindowWidth,
		WindowHeight:     s.WindowHeight,
		MapColors:        s.MapColors,
		IncludeMap:       s.IncludeMap,
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), data, 0644)
}

// GetSettings returns the current settings to the frontend.
func (a *App) GetSettings() (*Settings, error) {
	return LoadSettings(), nil
}

// SaveSettingsFromFrontend saves user preferences from the frontend.
// RecentSaves in the payload is ignored - recent saves are managed exclusively
// by the backend via AddRecentSave, RemoveRecentSave, and ClearRecentSaves.
// WindowWidth/WindowHeight are preserved from existing settings since the
// frontend doesn't manage window dimensions.
func (a *App) SaveSettingsFromFrontend(s Settings) error {
	existing := LoadSettings()
	if s.WindowWidth == 0 {
		s.WindowWidth = existing.WindowWidth
	}
	if s.WindowHeight == 0 {
		s.WindowHeight = existing.WindowHeight
	}
	if s.Language == "" {
		s.Language = existing.Language
	}
	return SaveSettings(&s)
}

// SettingsPatch is a partial settings update. Only fields the caller actually
// set (non-nil pointers) are applied; everything else keeps its stored value.
//
// This exists because SaveSettingsFromFrontend is a whole-object write, so any
// caller that only wanted to change one thing had to resend all twelve fields.
// Every such call site was a chance to clobber an unrelated preference with a
// stale or defaulted value. Pointers also let us distinguish "set this bool to
// false" from "don't touch this bool", which a plain bool cannot express.
type SettingsPatch struct {
	DefaultOutputLoc *string            `json:"defaultOutputLoc,omitempty"`
	CustomOutputDir  *string            `json:"customOutputDir,omitempty"`
	AutoDetectSave   *bool              `json:"autoDetectSave,omitempty"`
	AutoAnalyze      *bool              `json:"autoAnalyze,omitempty"`
	CustomSaveDir    *string            `json:"customSaveDir,omitempty"`
	Language         *string            `json:"language,omitempty"`
	ZoomFactor       *float64           `json:"zoomFactor,omitempty"`
	WindowWidth      *int               `json:"windowWidth,omitempty"`
	WindowHeight     *int               `json:"windowHeight,omitempty"`
	MapColors        *map[string]string `json:"mapColors,omitempty"`
	IncludeMap       *bool              `json:"includeMap,omitempty"`
}

// UpdateSettings applies a partial update on top of the stored settings and
// returns the result, so the frontend doesn't need a follow-up GetSettings call.
func (a *App) UpdateSettings(p SettingsPatch) (*Settings, error) {
	s := LoadSettings()
	if p.DefaultOutputLoc != nil {
		s.DefaultOutputLoc = *p.DefaultOutputLoc
	}
	if p.CustomOutputDir != nil {
		s.CustomOutputDir = *p.CustomOutputDir
	}
	if p.AutoDetectSave != nil {
		s.AutoDetectSave = *p.AutoDetectSave
	}
	if p.AutoAnalyze != nil {
		s.AutoAnalyze = *p.AutoAnalyze
	}
	if p.CustomSaveDir != nil {
		s.CustomSaveDir = *p.CustomSaveDir
	}
	if p.Language != nil && *p.Language != "" {
		s.Language = *p.Language
	}
	if p.ZoomFactor != nil && *p.ZoomFactor > 0 {
		s.ZoomFactor = *p.ZoomFactor
	}
	if p.WindowWidth != nil && *p.WindowWidth > 0 {
		s.WindowWidth = *p.WindowWidth
	}
	if p.WindowHeight != nil && *p.WindowHeight > 0 {
		s.WindowHeight = *p.WindowHeight
	}
	if p.MapColors != nil {
		s.MapColors = *p.MapColors
	}
	if p.IncludeMap != nil {
		s.IncludeMap = p.IncludeMap
	}
	if err := SaveSettings(s); err != nil {
		return nil, err
	}
	return LoadSettings(), nil
}

// SetLanguage updates the backend item display language so that item names
// in the depot, analytics HTML, etc. are localized. Called by the frontend
// when the user switches language.
func (a *App) SetLanguage(lang string) error {
	extraction.SetItemDisplayLanguage(lang)
	return nil
}

// AddRecentSave adds a save file to the recent saves list and persists it.
func AddRecentSave(path, name string) {
	saves := loadRecentSaves()

	for i, r := range saves {
		if r.Path == path {
			saves = append(saves[:i], saves[i+1:]...)
			break
		}
	}

	saves = append([]RecentSave{{Path: path, Name: name}}, saves...)

	if len(saves) > maxRecentSaves {
		saves = saves[:maxRecentSaves]
	}

	_ = saveRecentSaves(saves)
}

// RemoveRecentSave removes a single entry from the recent saves list by index.
func (a *App) RemoveRecentSave(index int) error {
	saves := loadRecentSaves()
	if index < 0 || index >= len(saves) {
		return nil
	}
	saves = append(saves[:index], saves[index+1:]...)
	return saveRecentSaves(saves)
}

// ClearRecentSaves empties the recent saves list.
func (a *App) ClearRecentSaves() error {
	return saveRecentSaves(nil)
}
