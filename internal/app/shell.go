package app

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// Panel and window titles.
//
// Dear ImGui identifies a window by its title and the saved docking layout
// refers to the same strings, so renaming one of these drops the user's stored
// position for that panel.
const (
	panelPreview   = "Portrait"
	panelCharacter = "Character"
	panelLooks     = "Portrait Modifiers###looks"

	// The windows of a portrait modifier project; see workspace.go.
	panelProject = "Portrait Modifiers###project"
	panelElement = "Element###element"
	windowWeight = "Weight###weight"

	windowSettings = "Settings"
	windowProblems = "Problems"
)

// dockNodeFlagsDockSpace is ImGuiDockNodeFlags_DockSpace from imgui_internal.h.
// cimgui-go only generates the public flags, but building a default layout
// needs this one: it marks the root node as able to host a dock space.
const dockNodeFlagsDockSpace imgui.DockNodeFlags = 1 << 10

// sidebarRatio is the share of the window the sidebar takes when the default
// layout is built.
const sidebarRatio = 0.36

// dockSpace covers the remaining viewport with a dock space so every panel can
// be rearranged, tabbed or torn off by the user.
func (a *App) dockSpace() {
	// PassthruCentralNode keeps the central node transparent when no panel is
	// docked into it, so the window background stays visible.
	id := imgui.DockSpaceOverViewportV(0, imgui.MainViewport(), imgui.DockNodeFlagsPassthruCentralNode, a.dockWindowClass)

	if !a.state.layoutBuilt {
		a.state.layoutBuilt = true
		a.buildDefaultLayout(id)
	}
}

// buildDefaultLayout puts the portrait in the middle of the window and the
// character panel beside it. It only runs when there is no saved layout.
func (a *App) buildDefaultLayout(dockSpaceID imgui.ID) {
	imgui.InternalDockBuilderRemoveNode(dockSpaceID)
	imgui.InternalDockBuilderAddNodeV(dockSpaceID, dockNodeFlagsDockSpace)
	imgui.InternalDockBuilderSetNodeSize(dockSpaceID, imgui.MainViewport().WorkSize())

	var sidebar, center imgui.ID
	imgui.InternalDockBuilderSplitNode(dockSpaceID, imgui.DirRight, sidebarRatio, &sidebar, &center)

	imgui.InternalDockBuilderDockWindow(panelPreview, center)
	imgui.InternalDockBuilderDockWindow(panelCharacter, sidebar)
	imgui.InternalDockBuilderDockWindow(panelLooks, sidebar)
	imgui.InternalDockBuilderDockWindow(panelElement, sidebar)

	imgui.InternalDockBuilderFinish(dockSpaceID)
}

// dockFirstTime docks a window the first time it is shown, where the layout,
// saved before it was there, has no place for it: as a tab beside another
// window for no direction, or in a split of that window's node to the side a
// direction says, which ratio of the node takes. A window shown before, or
// one whose neighbour has never been, is left where it is.
func (a *App) dockFirstTime(name, beside string, side imgui.Dir, ratio float32) {
	if _, ok := gui.FindWindow(name); ok {
		return
	}

	if settings := imgui.InternalFindWindowSettingsByID(imgui.InternalImHashStr(name)); settings != nil && settings.CData != nil {
		return
	}

	neighbour, ok := gui.FindWindow(beside)
	if !ok || neighbour.DockId() == 0 {
		return
	}

	if side == imgui.DirNone {
		imgui.SetNextWindowDockIDV(neighbour.DockId(), imgui.CondFirstUseEver)

		return
	}

	var at, rest imgui.ID

	imgui.InternalDockBuilderSplitNode(neighbour.DockId(), side, ratio, &at, &rest)
	imgui.InternalDockBuilderDockWindow(name, at)
	imgui.InternalDockBuilderFinish(neighbour.DockId())
}

func (a *App) menuBar() {
	if !imgui.BeginMainMenuBar() {
		return
	}
	defer imgui.EndMainMenuBar()

	if imgui.BeginMenu("File") {
		if imgui.MenuItemBoolV(gui.Iconed(gui.IconNewPortrait, "New Portrait..."), "Ctrl+N", false, a.canStartOver()) {
			a.startOver()
		}

		// Why a new portrait cannot be started, where it cannot.
		if !a.hasFolders() && imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
			imgui.SetTooltip("Add the game's folder in Settings (Ctrl+,) first")
		}

		imgui.Separator()

		if a.state.mode == modeModifiers {
			if imgui.MenuItemBoolV(gui.Iconed(gui.IconSave, labelSaveModifier), "Ctrl+S", false, a.canSaveModifier()) {
				a.saveModifier()
			}

			imgui.Separator()
		}

		if imgui.MenuItemBoolV(gui.Iconed(gui.IconSave, labelSave), "Ctrl+S", false, a.canSave()) {
			a.save()
		}

		if imgui.MenuItemBoolV(gui.Iconed(gui.IconSaveAs, labelSaveToFile), "Ctrl+Shift+S", false, a.canSave()) {
			a.saveToFile()
		}

		imgui.Separator()

		if imgui.MenuItemBoolV(gui.Iconed(gui.IconCopy, labelCopyScript), "", false, a.canSave()) {
			a.copyScript()
		}

		if imgui.MenuItemBoolV(gui.Iconed(gui.IconCopy, labelCopyString), "", false, a.canSave()) {
			a.copyDNAString()
		}

		if imgui.MenuItemBoolV(gui.Iconed(gui.IconCopy, labelCopyPersistent), "", false, a.canSave()) {
			a.copyPersistentDNA()
		}

		imgui.Separator()

		if imgui.MenuItemBoolV(gui.Iconed(gui.IconExit, "Exit"), "Alt+F4", false, true) {
			a.window.RequestClose()
		}

		imgui.EndMenu()
	}

	if imgui.BeginMenu("View") {
		if imgui.MenuItemBoolV(gui.Iconed(gui.IconAddPhoto, labelOpenReference), "", false, a.editing()) {
			a.openReference()
		}

		imgui.Separator()

		imgui.MenuItemBoolPtr(gui.Iconed(gui.IconWarning, "Problems"), "", &a.state.showProblems)

		imgui.EndMenu()
	}

	if imgui.BeginMenu("Settings") {
		imgui.MenuItemBoolPtr(gui.Iconed(gui.IconSettings, "Open Settings"), "Ctrl+,", &a.state.showSettings)

		imgui.EndMenu()
	}

	if imgui.BeginMenu("Help") {
		if imgui.MenuItemBool(gui.Iconed(gui.IconInfo, "About")) {
			a.state.showAbout = true
		}

		imgui.EndMenu()
	}
}

// statusBar is the strip along the bottom of the window: the latest message
// on the left, how the interface is doing on the right.
func (a *App) statusBar() {
	flags := imgui.WindowFlagsNoScrollbar | imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsMenuBar

	if imgui.InternalBeginViewportSideBar("##StatusBar", imgui.MainViewport(), imgui.DirDown, imgui.FrameHeight(), flags) {
		if imgui.BeginMenuBar() {
			if a.state.statusError {
				gui.ErrorText(a.state.status)
			} else {
				imgui.TextUnformatted(a.state.status)
			}

			right := fmt.Sprintf("%.0f FPS", imgui.CurrentIO().Framerate())

			// What went wrong reading the portrait is a click away.
			if a.portrait != nil && len(a.portrait.diagnostics) > 0 {
				problems := gui.Iconed(gui.IconWarning, fmt.Sprintf("%d problems", len(a.portrait.diagnostics)))

				imgui.SameLine()
				imgui.SetCursorPosX(imgui.ContentRegionAvail().X - imgui.CalcTextSize(right).X - imgui.CalcTextSize(problems).X - imgui.CurrentStyle().ItemSpacing().X*3)

				if imgui.SmallButton(problems) {
					a.state.showProblems = true
				}
			}

			// Right align the diagnostics.
			imgui.SameLine()
			imgui.SetCursorPosX(imgui.ContentRegionAvail().X - imgui.CalcTextSize(right).X)
			imgui.TextDisabled(right)

			imgui.EndMenuBar()
		}

		imgui.End()
	}
}

// handleShortcuts implements the keyboard shortcuts that are not attached to a
// single panel.
func (a *App) handleShortcuts() {
	io := imgui.CurrentIO()

	if io.KeyCtrl() && imgui.IsKeyPressedBool(imgui.KeyComma) {
		a.state.showSettings = !a.state.showSettings
	}

	if io.KeyCtrl() && imgui.IsKeyPressedBool(imgui.KeyN) && a.canStartOver() {
		a.startOver()
	}

	if io.KeyCtrl() && imgui.IsKeyPressedBool(imgui.KeyS) && a.canSaveModifier() {
		a.saveModifier()
	}

	if io.KeyCtrl() && imgui.IsKeyPressedBool(imgui.KeyS) && a.canSave() {
		if io.KeyShift() {
			a.saveToFile()
		} else {
			a.save()
		}
	}

	// Escape closes the window on top. While a text field is being edited Dear
	// ImGui uses Escape to revert the edit, so leave it alone then. A popup
	// that was open answers Escape itself. It has closed by now, so what
	// counts is whether one was open when the frame began.
	if imgui.IsKeyPressedBool(imgui.KeyEscape) && !io.WantTextInput() && !a.state.popupWasOpen {
		a.closeFocusedWindow()
	}
}

func (a *App) closeFocusedWindow() {
	switch a.state.focusedWindow {
	case windowSettings:
		a.state.showSettings = false
	case windowProblems:
		a.state.showProblems = false
	case windowWeight:
		a.state.workspace.showWeight = false
	default:
		a.closeReference(a.state.focusedWindow)
	}

	a.state.focusedWindow = ""
}

// aboutPopup names the application and its version, says what it is and
// what it is built with, and credits the works it uses. Help > About asks for
// it by showAbout, and Close or Escape closes it again.
func (a *App) aboutPopup() {
	if a.state.showAbout {
		a.state.showAbout = false
		imgui.OpenPopupStr(popupAbout)
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(460, 0), imgui.CondAlways)

	if !imgui.BeginPopupModalV(popupAbout, nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	gui.PushStrongFont()
	imgui.TextUnformatted(applicationName + " " + applicationVersion)
	gui.PopFont()

	dimmedWrapped(aboutDescription)

	imgui.Separator()

	dimmedWrapped("Rendered with raylib, interface built with Dear ImGui.")

	gui.PushStrongFont()
	imgui.SeparatorText("Credits")
	gui.PopFont()

	for _, credit := range credits {
		credit.show()
		imgui.Spacing()
	}

	imgui.Separator()

	if imgui.Button("Close") || imgui.IsKeyPressedBool(imgui.KeyEscape) {
		imgui.CloseCurrentPopup()
	}
}

// aboutDescription says what the application is, under its name.
const aboutDescription = "Character portrait editor for Victoria 3 and Europa Universalis 5 mods."

// popupAbout is the title of the About window.
const popupAbout = "About"

// canStartOver reports whether the welcome screen can be opened again: once a
// portrait is loaded, and while neither it nor the picker is open.
func (a *App) canStartOver() bool {
	return a.portrait != nil && a.portrait.loaded() && !a.state.welcome && !a.state.picker.open
}

// startOver opens the welcome screen again, to start the portrait from
// something else, set to the workflow the windows are for.
func (a *App) startOver() {
	a.state.welcome = true
	a.state.welcomeMode = a.state.mode
}

// problemsWindow lists what went wrong reading the portrait.
func (a *App) problemsWindow() {
	if !a.state.showProblems {
		return
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(640, 360), imgui.CondFirstUseEver)

	if imgui.BeginV(windowProblems, &a.state.showProblems, imgui.WindowFlagsNoDocking) {
		a.trackFocus(windowProblems)

		if a.portrait == nil || len(a.portrait.diagnostics) == 0 {
			imgui.TextDisabled("Nothing went wrong")
		} else {
			for _, diagnostic := range a.portrait.diagnostics {
				dimmedWrapped(diagnostic.String())
			}
		}
	}

	imgui.End()
}
