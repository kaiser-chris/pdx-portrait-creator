package app

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/config"
	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// How wide the settings window opens, and how narrow it can be made, in
// unscaled units.
const (
	settingsWidth    = 640
	settingsMinWidth = 460
)

// Labels the settings window and its tests share.
const (
	// labelBrowse is the button beside each folder that opens the folder
	// dialog.
	labelBrowse = "Browse..."

	// labelUnsavedSettings says that the settings window shows edits which
	// have not been saved, and so are not in effect for the folders yet.
	labelUnsavedSettings = "Unsaved changes"
)

// settingsWindow edits the interface scale and the configured game and mod
// folders. Edits are made on copies and only written to disk on save.
func (a *App) settingsWindow() {
	if !a.state.showSettings {
		return
	}

	// The window fits its contents in height, however many folders there are,
	// and can be made as wide as the paths in it need. Asking Dear ImGui to fit
	// the height outright would switch off its resize grips, width and all, so
	// the height is pinned with size constraints instead, at what fitting it
	// would come to. A height of zero on first use has it fitted to begin with.
	imgui.SetNextWindowSizeV(gui.ScaledVec2(settingsWidth, 0), imgui.CondFirstUseEver)

	if window, found := gui.FindWindow(windowSettings); found {
		height := imgui.InternalCalcWindowNextAutoFitSize(window).Y

		imgui.SetNextWindowSizeConstraints(
			imgui.Vec2{X: gui.Scaled(settingsMinWidth), Y: height},
			imgui.Vec2{X: math.MaxFloat32, Y: height},
		)
	}

	if imgui.BeginV(windowSettings, &a.state.showSettings, imgui.WindowFlagsNoDocking) {
		a.trackFocus(windowSettings)

		sectionHeader("Appearance")

		a.scaleField()

		sectionHeader("Game and Mod Folders")
		dimmedWrapped("Point these at a game folder or a mod folder to read its characters from. " +
			"A folder listed further down overrides the ones above it, the way a mod overrides the game.")

		a.folderTable()

		if imgui.Button(gui.Iconed(gui.IconAdd, "Add Folder")) {
			a.state.folders = append(a.state.folders, folderEntry{})
		}

		imgui.SameLine()

		if imgui.Button("Save") {
			a.saveSettings()
		}

		imgui.SameLine()

		unsaved := a.settingsChanged()

		imgui.BeginDisabledV(!unsaved)
		if imgui.Button(gui.Iconed(gui.IconReset, "Revert")) {
			a.state.loadFrom(a.settings)
			a.setStatus("Settings reverted")
		}
		imgui.EndDisabled()

		if unsaved {
			imgui.SameLine()
			imgui.AlignTextToFramePadding()
			gui.WarningText(labelUnsavedSettings)
		}
	}

	imgui.End()
}

func (a *App) folderTable() {
	// Some breathing room around the header and between the rows. Dear ImGui
	// reads the padding row by row, so it stays pushed until the table ends.
	padding := imgui.CurrentStyle().CellPadding()
	imgui.PushStyleVarVec2(imgui.StyleVarCellPadding, imgui.Vec2{X: padding.X, Y: gui.Scaled(5)})
	defer imgui.PopStyleVar()

	// No height of its own: the table grows by a row for every folder added,
	// and the window grows with it. Without outer borders Dear ImGui leaves
	// the outer edges unpadded, which puts the first column flush against the
	// edge while every other one is indented, so the padding is asked for.
	flags := imgui.TableFlagsBordersInnerH | imgui.TableFlagsRowBg | imgui.TableFlagsPadOuterX
	if !imgui.BeginTableV("folders", 3, flags, imgui.Vec2{}, 0) {
		return
	}
	defer imgui.EndTable()

	imgui.TableSetupColumnV("Name", imgui.TableColumnFlagsWidthFixed, gui.Scaled(150), 0)
	imgui.TableSetupColumnV("Folder", imgui.TableColumnFlagsWidthStretch, 0, 0)
	imgui.TableSetupColumnV("", imgui.TableColumnFlagsWidthFixed, gui.Scaled(28), 0)
	imgui.TableHeadersRow()

	remove := -1

	for index := range a.state.folders {
		entry := &a.state.folders[index]

		imgui.PushIDInt(int32(index))

		imgui.TableNextRow()

		imgui.TableSetColumnIndex(0)
		imgui.SetNextItemWidth(-1)
		imgui.InputTextWithHint("##name", "game", &entry.Name, 0, nil)

		imgui.TableSetColumnIndex(1)
		// The path can be typed or pasted, or picked in the system's dialog.
		browseWidth := imgui.CalcTextSize(labelBrowse).X + imgui.CurrentStyle().FramePadding().X*2
		imgui.SetNextItemWidth(imgui.ContentRegionAvail().X - browseWidth - imgui.CurrentStyle().ItemSpacing().X)
		imgui.InputTextWithHint("##path", "path to a game or mod folder", &entry.Path, 0, nil)

		imgui.SameLine()

		if imgui.Button(labelBrowse) {
			a.browseFolder(index)
		}

		imgui.TableSetColumnIndex(2)
		if imgui.Button(gui.IconDelete) {
			remove = index
		}

		imgui.SetItemTooltip("Remove the folder")

		imgui.PopID()
	}

	if remove >= 0 {
		a.state.folders = append(a.state.folders[:remove], a.state.folders[remove+1:]...)
	}
}

// saveSettings writes the settings window's buffers to the settings file.
func (a *App) saveSettings() {
	folders := make([]config.Folder, 0, len(a.state.folders))
	for _, entry := range a.state.folders {
		if entry.Name == "" || entry.Path == "" {
			// A half filled row is an edit in progress, not a folder to store.
			continue
		}

		folders = append(folders, config.Folder{Name: entry.Name, Path: entry.Path})
	}

	saved := &config.Settings{
		Folders:        folders,
		InterfaceScale: a.state.interfaceScale,
	}

	if err := a.store.Save(saved); err != nil {
		// The settings in effect stay what is on disk, and the window keeps
		// the edits, so the user can try again.
		warn(err)
		a.setStatus("Settings could not be saved: %v", err)

		return
	}

	foldersChanged := !slices.Equal(a.settings.Folders, saved.Folders)
	a.settings = saved

	// The half filled rows were dropped, so the window shows what was stored.
	a.state.loadFrom(a.settings)
	a.setStatus("Settings saved")

	// What the portrait is read from may have changed.
	if foldersChanged {
		a.startPortrait()
	}
}

// settingsChanged reports whether the settings window holds edits that have
// not been saved. The interface scale shows straight away, but counts as well:
// it is lost on the next start unless saved.
func (a *App) settingsChanged() bool {
	if a.state.interfaceScale != a.settings.InterfaceScale {
		return true
	}

	if len(a.state.folders) != len(a.settings.Folders) {
		return true
	}

	for index, entry := range a.state.folders {
		saved := a.settings.Folders[index]
		if entry.Name != saved.Name || entry.Path != saved.Path {
			return true
		}
	}

	return false
}

// scaleChoices are the interface scales offered besides following the monitor.
var scaleChoices = []float32{1, 1.25, 1.5, 1.75, 2, 2.5, 3}

func scaleLabel(scale float32) string {
	return fmt.Sprintf("%.0f%%", scale*100)
}

// automaticScaleLabel names the choice that follows the monitor, with the
// scale that works out to right now.
func automaticScaleLabel() string {
	return fmt.Sprintf("Automatic (%s)", scaleLabel(gui.MonitorScale()))
}

// scaleField chooses how large the interface is drawn. A choice shows straight
// away but is only stored when the settings are saved.
func (a *App) scaleField() {
	current := a.state.interfaceScale

	preview := automaticScaleLabel()
	if current > 0 {
		preview = scaleLabel(current)
	}

	if !imgui.BeginCombo("Interface Scale", preview) {
		return
	}
	defer imgui.EndCombo()

	if imgui.SelectableBoolV(automaticScaleLabel(), current == 0, 0, imgui.Vec2{}) {
		a.state.interfaceScale = 0
	}

	for _, choice := range scaleChoices {
		if imgui.SelectableBoolV(scaleLabel(choice), current == choice, 0, imgui.Vec2{}) {
			a.state.interfaceScale = choice
		}
	}
}

// interfaceScale is the scale the interface should be drawn at: the one chosen
// in the settings window, or the monitor's.
func (a *App) interfaceScale() float32 {
	if a.state.interfaceScale > 0 {
		return a.state.interfaceScale
	}

	return gui.MonitorScale()
}

// browseFolder picks the folder of a row of the folder table in the system's
// dialog. Like typing it, this only changes the row until the settings are
// saved.
func (a *App) browseFolder(row int) {
	start := ""
	if row < len(a.state.folders) {
		start = a.state.folders[row].Path
	}

	request := fileRequest{title: "Choose a Game or Mod Folder", start: start, folder: true}

	a.ask(request, func(path string) {
		// The rows may have changed while the dialog was open.
		if row >= len(a.state.folders) {
			return
		}

		entry := &a.state.folders[row]
		entry.Path = path

		// A row without a name would not be saved, so it gets the folder's.
		if entry.Name == "" {
			entry.Name = filepath.Base(path)
		}
	})
}

// sectionHeader separates the parts of a window with a heading.
func sectionHeader(title string) {
	gui.PushStrongFont()
	imgui.SeparatorText(title)
	gui.PopFont()
}

// dimmedWrapped is an explanation in the dimmed text colour, wrapped to the
// width of the window.
func dimmedWrapped(text string) {
	colors := imgui.CurrentStyle().Colors()

	imgui.PushStyleColorVec4(imgui.ColText, colors[imgui.ColTextDisabled])
	imgui.TextWrapped(text)
	imgui.PopStyleColor()
}
