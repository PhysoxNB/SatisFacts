#!/usr/bin/env bash
#
# Cross-platform build script for SatisFacts.
# Produces self-contained binaries (data files are embedded via go:embed) into ./dist.
#
# Usage:
#   ./build.sh            # build for all platforms
#   ./build.sh linux      # build a single target (linux|windows|macos-arm|macos-intel)
#
set -euo pipefail

APP="SatisFacts"          # base binary name (linux/macOS)
WIN_APP="SatisFacts.exe"  # Windows binary name
OUT="dist"

# Optional version string baked into Windows resources / shown by --version if wired up.
VERSION="${VERSION:-dev}"

# Start fresh: remove previous output and any generated Windows resource file
# so renamed/stale binaries don't linger between builds.
echo "Cleaning $OUT/ ..."
rm -rf "$OUT" resource_windows_amd64.syso
mkdir -p "$OUT"

build_linux() {
  echo "Building Linux (amd64)..."
  GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$OUT/${APP}-linux-amd64" .
}

build_windows() {
  echo "Building Windows (amd64)..."
  # If a versioninfo.json + icon.ico exist, generate the .syso resource so the
  # .exe gets an icon and version metadata (requires goversioninfo, see build.sh notes).
  if [ -f versioninfo.json ]; then
    # Locate goversioninfo: prefer PATH, else fall back to Go's install bin
    # (GOBIN or GOPATH/bin), which is often not on PATH after `go install`.
    gvi="$(command -v goversioninfo 2>/dev/null || true)"
    if [ -z "$gvi" ]; then
      gobin="$(go env GOBIN)"
      [ -z "$gobin" ] && gobin="$(go env GOPATH)/bin"
      [ -x "$gobin/goversioninfo" ] && gvi="$gobin/goversioninfo"
    fi
    if [ -n "$gvi" ]; then
      "$gvi" -o resource_windows_amd64.syso versioninfo.json
    else
      echo "  (skipping icon: goversioninfo not found; run 'go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest')"
    fi
  fi
  GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$OUT/${WIN_APP}" .
}

build_macos_arm() {
  echo "Building macOS (Apple Silicon)..."
  GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o "$OUT/${APP}-macos-arm64" .
}

build_macos_intel() {
  echo "Building macOS (Intel)..."
  GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$OUT/${APP}-macos-amd64" .
}

target="${1:-all}"
case "$target" in
  linux)        build_linux ;;
  windows)      build_windows ;;
  macos-arm)    build_macos_arm ;;
  macos-intel)  build_macos_intel ;;
  all)
    build_linux
    build_windows
    build_macos_arm
    build_macos_intel
    ;;
  *)
    echo "Unknown target: $target (expected: linux|windows|macos-arm|macos-intel|all)"
    exit 1
    ;;
esac

# Generate SHA256 checksums for all release binaries.
echo "Generating SHA256 checksums..."
cd "$OUT"
sha256sum * > checksums.txt
cd ..

echo "Done. Binaries in ./$OUT:"
ls -lh "$OUT"
echo ""
echo "Checksums:"
cat "$OUT/checksums.txt"
