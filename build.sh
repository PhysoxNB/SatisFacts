#!/usr/bin/env bash
#
# Local build script for SatisFacts Wails UI.
# Builds Linux and Windows binaries into ui/build/bin/.
#
# Usage:
#   ./build.sh           # build Linux
#   ./build.sh windows   # cross-compile Windows .exe
#   ./build.sh all       # build both
#
set -euo pipefail

UI_DIR="ui"
GOPATH_BIN="$(go env GOPATH)/bin"
export PATH="$PATH:$GOPATH_BIN"
export GOTOOLCHAIN=local

# Copy Map.webp into extraction/assets/ for go:embed
mkdir -p extraction/assets
cp assets/Map.webp extraction/assets/Map.webp

# Minify viewer JS for embedding
./minify_viewer.sh

build_linux() {
  echo "Building Linux..."
  cd "$UI_DIR"
  wails build -platform linux/amd64 -tags webkit2_41
  cd ..
  echo "→ $UI_DIR/build/bin/SatisFacts"
}

build_windows() {
  echo "Building Windows..."
  cd "$UI_DIR"
  # Regenerate icon.ico from appicon.png so the custom SF icon is always used
  if [ -f build/appicon.png ]; then
    python3 -c "
from PIL import Image
img = Image.open('build/appicon.png')
img.save('build/windows/icon.ico', format='ICO', sizes=[(256,256),(128,128),(64,64),(48,48),(32,32),(16,16)])
" 2>/dev/null && echo "  (icon.ico regenerated)" || true
  fi
  wails build -platform windows/amd64
  cd ..
  echo "→ $UI_DIR/build/bin/SatisFacts.exe"
}

target="${1:-linux}"
case "$target" in
  linux)   build_linux ;;
  windows) build_windows ;;
  all)     build_linux; build_windows ;;
  *) echo "Usage: ./build.sh [linux|windows|all]"; exit 1 ;;
esac

echo "Done."
