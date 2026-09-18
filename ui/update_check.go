package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AppVersion is the current version of the application.
// This must be bumped manually on each release.
const AppVersion = "2.0.0"

// UpdateCheckResult holds the result of a version check.
type UpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseURL"`
	ReleaseNotes    string `json:"releaseNotes"`
}

// CheckForUpdates queries the GitHub API for the latest release and compares
// it against the compiled-in AppVersion.
func (a *App) CheckForUpdates() (*UpdateCheckResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", "https://api.github.com/repos/PhysoxNB/SatisFacts/releases/latest", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release info: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	updateAvailable := compareVersions(AppVersion, latest) < 0

	notes := release.Body
	if len(notes) > 500 {
		notes = notes[:497] + "..."
	}

	return &UpdateCheckResult{
		CurrentVersion:  AppVersion,
		LatestVersion:   latest,
		UpdateAvailable: updateAvailable,
		ReleaseURL:      release.HTMLURL,
		ReleaseNotes:    notes,
	}, nil
}

// GetAppVersion returns the current application version to the frontend.
func (a *App) GetAppVersion() string {
	return AppVersion
}

// compareVersions compares two semver strings (e.g. "2.0.0" vs "2.1.0").
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var na, nb int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &na)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &nb)
		}
		if na < nb {
			return -1
		}
		if na > nb {
			return 1
		}
	}
	return 0
}
