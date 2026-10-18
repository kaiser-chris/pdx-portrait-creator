package gui

import "github.com/AllenDang/cimgui-go/imgui"

// WarningText is a line of text in the warning colour.
func WarningText(text string) {
	imgui.TextColored(colorWarning, text)
}

// ErrorText is a line of text in the error colour.
func ErrorText(text string) {
	imgui.TextColored(colorError, text)
}

// PushErrorFrame draws the frames of the widgets that follow, until
// PopErrorFrame, in the error colour, to point at a field that has to be put
// right.
func PushErrorFrame() {
	imgui.PushStyleColorVec4(imgui.ColFrameBg, withAlpha(colorError, 0.6))
	imgui.PushStyleColorVec4(imgui.ColFrameBgHovered, withAlpha(colorError, 0.75))
	imgui.PushStyleColorVec4(imgui.ColFrameBgActive, withAlpha(colorError, 0.9))
}

// PopErrorFrame ends what PushErrorFrame started.
func PopErrorFrame() {
	imgui.PopStyleColorV(3)
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
