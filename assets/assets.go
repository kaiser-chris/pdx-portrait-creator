// Package assets bundles every file the application needs at runtime into the
// executable.
package assets

import _ "embed"

// The interface fonts are embedded as strings rather than through an embed.FS
// on purpose. Dear ImGui keeps the pointer to the font data for as long as its
// atlas lives, because it rasterises glyphs on demand. A string declared with
// go:embed is backed by memory in the binary itself, which is never moved and
// never freed; FS.ReadFile would hand out a garbage collected copy instead, and
// the atlas would end up reading freed memory.
var (
	//go:embed fonts/roboto/Roboto-Regular.ttf
	FontRegular string

	//go:embed fonts/roboto/Roboto-Medium.ttf
	FontMedium string

	// FontIcons is Material Symbols, cut down to the icons the interface
	// draws. See the README beside it for what is in it and how to add to it.
	//go:embed fonts/material-symbols/MaterialSymbolsOutlined-Subset.ttf
	FontIcons string
)
