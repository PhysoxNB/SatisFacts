#!/usr/bin/env bash
# Minifies viewer JS files into a single embedded file using esbuild.
# Called by build.sh before compilation.
set -euo pipefail

VIEWER_DIR="extraction/report/viewer"
OUT="extraction/report/viewer_minified.js"

cat "$VIEWER_DIR/utils.js" \
    "$VIEWER_DIR/dom.js" \
    "$VIEWER_DIR/data.js" \
    "$VIEWER_DIR/overview.js" \
    "$VIEWER_DIR/structures.js" \
    "$VIEWER_DIR/buildings.js" \
    "$VIEWER_DIR/clockspeeds.js" \
    "$VIEWER_DIR/power.js" \
    "$VIEWER_DIR/extraction.js" \
    "$VIEWER_DIR/manufacturing.js" \
    "$VIEWER_DIR/storage.js" \
    "$VIEWER_DIR/logistics.js" \
    "$VIEWER_DIR/transport.js" \
    "$VIEWER_DIR/nuclear.js" \
    "$VIEWER_DIR/collectibles.js" \
    "$VIEWER_DIR/map_core.js" \
    "$VIEWER_DIR/map_overlay.js" \
    "$VIEWER_DIR/map_webgl.js" \
    "$VIEWER_DIR/extras.js" \
    "$VIEWER_DIR/app.js" \
    | npx --yes esbuild --minify > "$OUT"

echo "  Minified viewer JS → $(wc -c < "$OUT") bytes"
