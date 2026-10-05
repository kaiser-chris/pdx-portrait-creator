package gui

import "github.com/AllenDang/cimgui-go/imgui"

// WarningText is a line of text in the warning colour.
func WarningText(text string) {
	imgui.TextColored(colorWarning, text)
}

// FindWindow looks a window up by name. cimgui-go wraps whatever Dear ImGui
// returns, a null pointer included, so a missing window would otherwise look
// found and crash on first use.
func FindWindow(name string) (*imgui.Window, bool) {
	window := imgui.InternalFindWindowByName(name)
	if window == nil || window.CData == nil {
		return nil, false
	}

	return window, true
}
