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

## [2.0.0] - The Big One – Desktop App, Save Editor & Analytics Overhaul (2026-11-05)

### Added
- New interactive Map tab with WebGL2 rendering, zoom/pan, object hover tooltips, category filters, Z-level filtering, and radius search.
- New Train Details table showing train names, locomotive/wagon counts, current station, timetables and total route distance in km.
- New Train Stations table showing station names, connected platforms, docked trains and docking times.
- New Vehicle Details table with vehicle names, types, fuel status and warning badges for vehicles without fuel.
- New Vehicle Stations table displaying station types, docking status and power warnings.
- New Drone Details table including fuel type, docking status and home station.
- New Drone Stations table showing paired station connections.
- Vehicle overview now includes Trucks, Fluid Trucks, Tractors, Explorers, Golf Carts, Cyber Wagons and all station types.
- Vehicle Tracks card displaying the combined route length of all vehicle paths (Save Version 1.2+).
- Consumables section showing everything the player has consumed (berries, nuts, inhalers and more).
- New Storage Breakdown table showing the total amount of each item stored in each container.
- Added remaining locations for Power Slugs, Somersloops, Mercer Spheres and Crash Sites in collapsible dropdowns.
- Added icons for all structures, building pieces and items across the report.
- Desktop GUI application (Wails) with tabbed interface replacing the command-line tool.
- Depot Reorder: drag-and-drop reordering of Dimensional Depot items with live preview before saving.
- Belt & Pipe Upgrade: upgrade all belts and lifts to a target Mk tier and pipes to Mk2.
- Multipliers: patch custom game cost multipliers (recipe, space elevator, power consumption) beyond in-game UI limits.
- Blueprint Category Copy: copy blueprint folder structure between saves with replace or merge mode.
- Blueprint Export/Import: export blueprint categories to a `.sfbcat` file and import them into another save.
- Game Config Editor: edit `Engine.ini` and `Input.ini` directly from the app without hunting for the config files manually.
- Multilanguage support: full English and German translations for the desktop UI and HTML report, switchable via flag buttons.
- Zoom control: adjustable UI zoom level (80%–150%) from the header dropdown, saved across sessions.
- Window size persistence: the app remembers its window size between launches.
- Session Name Fix: automatic detection and repair of problematic session names (trailing spaces, invalid Windows characters).
- Save Optimization: reduce save file size (typically by 10–23%) by sorting objects by class name for better compression.
- Analyze with real-time progress bar.
- Auto-detect save directory and auto-analyze options.
- Settings page with auto-save confirmation.
- Help page with usage guide, FAQ and support contact links.

### Changed
- Complete HTML report overhaul across every analytics tab with a new, consistent design, cleaner layouts, improved organization, side-by-side sections and better readability.
- Logistics infrastructure has been moved from the Transport tab into its own dedicated **Logistics** tab.
- Transport tab now focuses exclusively on transportation systems.
- Renamed **Production** tab to **Extraction** for better clarity.
- Search now covers the whole report instead of just the current tab: results are grouped per tab and clicking one jumps straight to it. The query is also kept when switching tabs rather than being cleared.
- Miner and Fluid Extractor details have been merged into a single unified table.
- Resource names now use proper in-game display names instead of internal identifiers throughout the application.
- Card colors now convey meaning rather than tab identity: plain summary and count cards are neutral, color is reserved for values that actually mean something.
- Buildings tab: "Storage Player" renamed to "Personal Storage", "Storage Integrated" renamed to "Hub Storage".
- Buildings tab: water pumps moved from Production to Mining category.
- Buildings tab: removed rail track segments, switch controls, flow indicators, equipment items, and blueprint storage - these are infrastructure or items, not buildings.
- Pagination optimized for easier browsing.
- Removed Expert Mode.
- Reworked README with Features section and accurate tab/feature descriptions.
- Updated screenshots and preview images.

### Fixed
- Fixed collectible counting where several collectibles were previously ignored.
- Fixed vehicle detection causing certain vehicle types to never appear.
- Fixed train station counting where freight platforms were incorrectly counted as train stations.
- Fixed HUB milestone progression counting.
- Fixed fracking extractor resource and purity detection.
- Fixed multiple resource names displaying internal identifiers instead of proper in-game names.
- Fixed oil pump purity always showing "normal" and capacity not accounting for node purity.
- Fixed MAM Research - Nutrients and Mycelia are now shown as separate trees (previously merged into one "Nutrients (Mycelia)" tree).
- Fixed sorting on the large paginated tables (Individual Generators, Power Consumers, Extractor Details, Nuclear Plants, per-recipe buildings) - clicking a column header now sorts the entire table instead of only the rows on the current page.
- Fixed Personal Elevator not shown as Power Consumer.
- Fixed AWESOME Sink not shown as Power Consumer.

### Removed
- Removed command-line interface (replaced by desktop GUI).
- Removed QUICK-mode.

### Acknowledgments
- Special thanks to Paul for his invaluable contributions to the map renderer and many v2.0 features.

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

[Unreleased]: https://github.com/PhysoxNB/SatisFacts/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/PhysoxNB/SatisFacts/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/PhysoxNB/SatisFacts/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/PhysoxNB/SatisFacts/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/PhysoxNB/SatisFacts/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/PhysoxNB/SatisFacts/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/PhysoxNB/SatisFacts/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/PhysoxNB/SatisFacts/releases/tag/v1.0.0
