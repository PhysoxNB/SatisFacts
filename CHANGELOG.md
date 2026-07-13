# Changelog

All notable changes to **SatisFacts** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!--
How to use this file:
- Add new entries under "Unreleased" as you work, in the right category
  (Added / Changed / Fixed / Removed).
- When you release, rename "Unreleased" to the version + date, e.g.
  "## [1.1.0] - 2026-07-01", then start a fresh empty "Unreleased" section.
- Tag the release in git (e.g. `git tag v1.1.0`) and paste that section into
  the GitHub Release notes.
-->

## [Unreleased]

## [1.3.0] - 2026-07-13

### Added
- Creatures Killed section in Extras tab: shows total kills and per-creature-type breakdown

### Fixed
- Table column alignment in Extras tab: number columns (Count/Kills) no longer shift when toggling Expert Mode.
- itch.io Link updated

## [1.2.1] - 2026-07-12

### Fixed
- Foundation extraction: increased lightweight buildable `instanceCount` limit from 200,000 to 10,000,000 to support large modded saves with millions of foundations.
- Modded MAM research trees (e.g. InfiniteZoop) no longer show 0/0 — unknown trees display completed node count with "?" total and graceful "Modded tree" labeling.

## [1.2.0] - 2026-07-11

### Added
- Game Progression section in Overview: Space Elevator phases with completed status, milestone tier progress (X/Y milestones per tier).
- Starting Location displayed in Save Information.
- Player Stats in Extras tab: per-player name, inventory slots, visited areas, and played messages (supports multiplayer saves).
- AWESOME Sink coupons shown in Extras tab.
- Output file paths (absolute) shown in console after extraction.

### Changed
- Renamed "Project Assembly" section to "Game Progression".
- Space Elevator phase display now correctly handles 5 phases (was capped at 4) and shows "Phase 5 (Completed)" for post-game saves.
- Target Phase card hidden when the game is completed.
- AWESOME Sink points removed from UI (only coupons shown); point data still in JSON.
- Console UI widened from 64 to 88 characters to support longer save file names and 3-digit save indices.

### Fixed
- Space Elevator target phase showed "Phase 1" on completed saves (raw target wraps post-game); now correctly shows completed status.

## [1.1.0] - 2026-07-03

### Added
- Interactive mode: unified prompt accepts a file number or a folder path directly.
- Save file list in interactive mode is now sorted by date modified (newest first).
- Modification date displayed alongside each save file in the interactive list.
- Environment variables in folder paths are expanded (e.g. `%LOCALAPPDATA%` on Windows).
- Direct `.sav` file paths can be pasted at the prompt to process immediately.

### Changed
- Output files (`.json` and `.html`) are now written next to the executable instead of next to the save file, keeping the save folder clean.
- Console UI widened and aligned to support 3-digit save indices (100+ saves).
- All console border lines now use consistent cyan coloring.
- "No saves found" message is centered in the console box.

## [1.0.1] - 2026-06-29

### Fixed
- MAM research tree progress now includes the FICSMAS XMas tree (13 nodes).
- FICSMAS bonus research nodes in other trees no longer inflate completed counts beyond the in-game tree total.

## [1.0.0] - 2026-06-28

First public release.

### Added
- Streaming parser for Satisfactory `.sav` files with O(1) memory per object during parsing.
- Support for save versions v46 (1.0), v52 (1.1), and v60 (1.2).
- Two extraction modes: **QUICK** (basic counts) and **DEEP** (full analysis:
  power, manufacturing, clock speeds, recipes, storage, inventories, connections, game progression).
- Self-contained interactive HTML report (tabbed UI, search, sortable tables, charts).
- Detailed JSON extraction output.
- Embedded data files via `go:embed` — the binary is fully self-contained.
- Interactive console mode (run with no arguments) plus direct CLI usage and `--help`/`--version` flags.
- Cross-platform build script (`build.sh`) producing Windows, Linux, and macOS binaries.

### Changed
- Peak memory reduced by ~60% on large saves via streaming extraction and compact fact retention.

[Unreleased]: https://github.com/PhysoxNB/SatisFacts/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/PhysoxNB/SatisFacts/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/PhysoxNB/SatisFacts/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/PhysoxNB/SatisFacts/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/PhysoxNB/SatisFacts/releases/tag/v1.0.0
