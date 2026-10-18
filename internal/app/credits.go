package app

import (
	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// credit is a work the application is built on, as the About window credits
// it.
type credit struct {
	// work names the work and says what the application uses it for.
	work string

	// notice is the attribution in the words its licence asks for.
	notice string

	// link is where the work, or its licence, can be found.
	link, url string
}

// credits lists the works the About window credits: the application icon
// and the bundled fonts, whose licences ask to be credited, and the libraries
// the files are read and drawn with.
var credits = []credit{
	{
		work:   "Application icon",
		notice: "\"design\" by Resa Valen from Noun Project, licensed under CC BY 3.0.",
		link:   "thenounproject.com",
		url:    "https://thenounproject.com/browse/icons/term/design/",
	},
	{
		work: "Roboto, the interface font",
		notice: "Designed by Christian Robertson. Copyright 2011 The Roboto Project Authors. " +
			"Licensed under the SIL Open Font License, Version 1.1. Roboto is a trademark of Google.",
		link: "openfontlicense.org",
		url:  "https://openfontlicense.org",
	},
	{
		work: "Material Symbols, the icons",
		notice: "Copyright 2026 Google LLC. Licensed under the Apache License, Version 2.0. " +
			"The bundled font is cut down to the icons the interface draws.",
		link: "fonts.google.com/icons",
		url:  "https://fonts.google.com/icons",
	},
	{
		work:   "pdx-parser-go, reading the games' files",
		notice: "Reads the genes, DNA, ethnicities and portrait settings, and works out how a game, its DLCs and its mods combine. Copyright 2026 Chris Kaiser, licensed under the MIT License.",
		link:   "github.com/kaiser-chris/pdx-parser-go",
		url:    "https://github.com/kaiser-chris/pdx-parser-go",
	},
	{
		work:   "pdx-asset-go, loading and drawing the portrait",
		notice: "Reads the meshes, textures and animations and draws the portrait. Copyright 2026 Chris Kaiser, licensed under the MIT License.",
		link:   "github.com/kaiser-chris/pdx-asset-go",
		url:    "https://github.com/kaiser-chris/pdx-asset-go",
	},
}

func (c credit) show() {
	gui.PushStrongFont()
	imgui.TextUnformatted(c.work)
	gui.PopFont()

	dimmedWrapped(c.notice)
	imgui.TextLinkOpenURLV(c.link, c.url)
}
