package gui

import (
	"unsafe"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-portrait-creator/assets"
)

// interfaceFontSize is the size the interface is drawn at. Dear ImGui's stock
// font is 13 pixels tall, which is cramped on a modern desktop display.
const interfaceFontSize = 16

// The icons of Material Symbols, by the code point the font maps them to.
// Each is a string rather than a rune, since that is what a label is.
const (
	IconAdd         = "\ue145" // add
	IconAddCircle   = "\ue990" // add_circle
	IconAge         = "\ue7e9" // cake
	IconAgeing      = "\uf21a" // elderly
	IconAnimation   = "\ue71c" // animation
	IconBack        = "\ue5c4" // arrow_back
	IconBody        = "\ue92c" // accessibility_new
	IconBoy         = "\ueb67" // boy
	IconBrows       = "\ueeb3" // eyebrow
	IconCamera      = "\ue412" // photo_camera
	IconCategory    = "\ue72c" // category
	IconCheeks      = "\ue813" // sentiment_satisfied
	IconColours     = "\ue40a" // palette
	IconCopy        = "\ue14d" // content_copy
	IconDelete      = "\ue92e" // delete
	IconDown        = "\ue5db" // arrow_downward
	IconEars        = "\ue023" // hearing
	IconExit        = "\ue9ba" // logout
	IconEyes        = "\ue8f4" // visibility
	IconFace        = "\uf008" // face
	IconFemale      = "\ue590" // female
	IconForward     = "\ue5c8" // arrow_forward
	IconGirl        = "\ueb68" // girl
	IconGroups      = "\uf233" // groups
	IconHair        = "\ue14e" // content_cut
	IconHead        = "\uf8de" // face_6
	IconHighlight   = "\ue25f" // highlight
	IconInfo        = "\ue88e" // info
	IconJaw         = "\uf8dd" // face_5
	IconLight       = "\ue518" // light_mode
	IconLink        = "\ue250" // link
	IconLinkOff     = "\ue16f" // link_off
	IconMale        = "\ue58e" // male
	IconMouth       = "\ueeb2" // lips
	IconNewPortrait = "\uea4d" // person_add
	IconNose        = "\uefd8" // air
	IconPause       = "\ue034" // pause
	IconPlay        = "\ue037" // play_arrow
	IconPerson      = "\uf0d3" // person
	IconReset       = "\uf053" // restart_alt
	IconSearch      = "\uef7a" // search
	IconSettings    = "\ue8b8" // settings
	IconUp          = "\ue5d8" // arrow_upward
	IconWarning     = "\uf083" // warning
)

// Iconed is a label with an icon before it, as buttons and menu items are
// drawn with one.
func Iconed(icon, label string) string {
	return icon + "  " + label
}

// iconFontSize is the size the icons are baked at when they stand among text.
// An icon drawn on its own is drawn at the size of whatever it is drawn in,
// which is what drawIcon does.
const iconFontSize = interfaceFontSize

// iconScale is how much larger than the text an icon is drawn, and iconDrop
// how far it is lowered, in pixels of the interface font.
const (
	iconScale = 1.25
	iconDrop  = 3
)

// strongFont is the heavier weight used for headings. It is kept here rather
// than passed around because there is exactly one interface.
var strongFont *imgui.Font

// configureFonts replaces Dear ImGui's built in font with Roboto.
func configureFonts() {
	atlas := imgui.CurrentIO().Fonts()
	if atlas == nil {
		return
	}

	// The icons are folded into both weights, so that a heading can hold one
	// as well as a button.
	addFont(atlas, assets.FontRegular)
	addIcons(atlas)

	strongFont = addFont(atlas, assets.FontMedium)
	addIcons(atlas)
}

// addIcons folds the icon font into the one added before it, so that an icon
// can be drawn wherever text can, as the code point it is mapped to.
func addIcons(atlas *imgui.FontAtlas) {
	config := imgui.NewFontConfig()
	defer config.Destroy()

	config.SetFontDataOwnedByAtlas(false)
	config.SetMergeMode(true)

	// The icons are drawn a quarter larger than the text they stand in,
	// whose capitals they would otherwise fall short of, and lowered to
	// sit on its line rather than above it.
	config.SetExtraSizeScale(iconScale)
	config.SetGlyphOffset(imgui.Vec2{Y: iconDrop})

	atlas.AddFontFromMemoryTTFV(
		uintptr(unsafe.Pointer(unsafe.StringData(assets.FontIcons))),
		int32(len(assets.FontIcons)),
		iconFontSize,
		config,
		nil,
	)
}

func addFont(atlas *imgui.FontAtlas, ttf string) *imgui.Font {
	config := imgui.NewFontConfig()
	defer config.Destroy()

	// The data lives in the binary and outlives the atlas, so Dear ImGui must
	// not try to free it.
	config.SetFontDataOwnedByAtlas(false)

	return atlas.AddFontFromMemoryTTFV(
		uintptr(unsafe.Pointer(unsafe.StringData(ttf))),
		int32(len(ttf)),
		interfaceFontSize,
		config,
		nil,
	)
}

// PushStrongFont switches to the heavier weight until PopFont is called. It is
// meant for headings and other short, emphasised runs of text.
func PushStrongFont() {
	if strongFont == nil {
		return
	}

	// A size of zero keeps whatever size is currently in effect.
	imgui.PushFont(strongFont, 0)
}

// PopFont undoes the most recent PushStrongFont.
func PopFont() {
	if strongFont == nil {
		return
	}

	imgui.PopFont()
}
