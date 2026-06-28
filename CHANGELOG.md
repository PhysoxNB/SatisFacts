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

## [1.0.0] - 2026-06-24

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

[Unreleased]: https://github.com/PhysoxNB/SatisFacts/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/PhysoxNB/SatisFacts/releases/tag/v1.0.0
