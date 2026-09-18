package assets

import "embed"

// IconFS embeds all WebP icons from the icons/ directory.
// These icons are copyrighted by Coffee Stain Studios and are NOT
// licensed under the project's GPL-3.0 license. See icons/NOTICE.md
// and THIRD_PARTY_ASSETS.md for details.
//
//go:embed icons/*.webp
var IconFS embed.FS
