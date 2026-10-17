package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/genes"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// The welcome screen opens over the portrait when the application starts,
// and again from File > New Portrait: it asks what the portrait starts from.
// From scratch is the template ethnicity, every gene neutral, which is what is
// loaded behind it, so choosing it only closes the screen. An ethnicity or a
// character of the game is chosen in the picker, which draws each in the
// portrait as it is pointed at.

// The titles of the welcome screen and of the picker, which Dear ImGui knows
// the windows by.
const (
	windowWelcome = "Welcome###welcome"
	windowPicker  = "Choose a Starting Point###picker"
	windowCover   = "##cover"
)

// coverShade is how dark the cover over the panels is.
const coverShade = 0.55

// The sizes of the welcome screen and the picker, unscaled.
const (
	welcomeWidth  = 860
	choiceHeight  = 150
	pickerWidth   = 1040
	pickerHeight  = 680
	pickerListing = 400
)

// pickerKind is what the picker lists.
type pickerKind int

const (
	pickCharacters pickerKind = iota
	pickEthnicities
)

// picker is what the picker remembers while it is open: what it lists, the
// search, what is picked and for which sex, and what the portrait was drawn
// from before, which going back draws again.
type picker struct {
	open   bool
	kind   pickerKind
	search string
	picked string
	sex    genes.Sex

	before       basis
	beforeDNA    *genes.DNA
	beforeSex    genes.Sex
	beforeLoaded bool
}

// centreNext places the next window in the middle of the main viewport's
// work area, between the menu bar and the status bar.
func centreNext(width, height float32) {
	viewport := imgui.MainViewport()
	centre := imgui.Vec2{X: viewport.WorkPos().X + viewport.WorkSize().X/2, Y: viewport.WorkPos().Y + viewport.WorkSize().Y/2}

	imgui.SetNextWindowPosV(centre, imgui.CondAlways, imgui.Vec2{X: 0.5, Y: 0.5})
	imgui.SetNextWindowSizeV(gui.ScaledVec2(width, height), imgui.CondAlways)
}

// The welcome screen and the picker are not modal: a modal window of Dear
// ImGui keeps the pointer from everything else, the menu bar among them,
// which has to stay usable for the settings a portrait needs first. They
// stand instead over a cover of the work area, which dims the panels and
// takes the pointer from them, and leaves the menu bar and the status bar,
// which lie outside the work area, as they are.
//
// The cover and the window over it are put in front of the panels every
// frame, unless a menu or a window of the menu bar stands over them, such as
// the settings, so that those stay in front.

// cover dims the panels and keeps the pointer from them.
func (a *App) cover() {
	viewport := imgui.MainViewport()

	imgui.SetNextWindowPosV(viewport.WorkPos(), imgui.CondAlways, imgui.Vec2{})
	imgui.SetNextWindowSizeV(viewport.WorkSize(), imgui.CondAlways)

	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.Vec4{W: coverShade})
	imgui.PushStyleVarFloat(imgui.StyleVarWindowBorderSize, 0)
	imgui.PushStyleVarFloat(imgui.StyleVarWindowRounding, 0)

	flags := imgui.WindowFlagsNoDecoration | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoDocking |
		imgui.WindowFlagsNoNav | imgui.WindowFlagsNoFocusOnAppearing | imgui.WindowFlagsNoBringToFrontOnFocus

	imgui.BeginV(windowCover, nil, flags)
	a.toFront()
	imgui.End()

	imgui.PopStyleVarV(2)
	imgui.PopStyleColor()
}

// toFront puts the window being drawn in front of the panels, unless a menu,
// a popup or a window of the menu bar is open, which stays in front.
func (a *App) toFront() {
	if imgui.IsPopupOpenStrV("", imgui.PopupFlagsAnyPopupId|imgui.PopupFlagsAnyPopupLevel) {
		return
	}

	if a.state.showSettings || a.state.showAbout || a.state.showProblems {
		return
	}

	imgui.InternalBringWindowToDisplayFront(imgui.InternalCurrentWindow())
}

// welcomeWindow is the welcome screen, while it is wanted.
func (a *App) welcomeWindow() {
	if !a.state.welcome {
		return
	}

	a.cover()
	centreNext(welcomeWidth, 0)

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoSavedSettings |
		imgui.WindowFlagsAlwaysAutoResize | imgui.WindowFlagsNoDocking | imgui.WindowFlagsNoCollapse

	visible := imgui.BeginV(windowWelcome, nil, flags)
	defer imgui.End()

	a.toFront()

	if !visible {
		return
	}

	p := a.portrait
	ready := p != nil && p.loaded() && !p.loading

	imgui.Dummy(gui.ScaledVec2(0, 8))
	imgui.TextDisabled(strings.ToUpper(applicationName))
	gui.PushStrongFont()
	imgui.TextUnformatted("Who are we making today?")
	gui.PopFont()
	dimmedWrapped("Pick a starting point. Every gene stays editable afterwards, and File > New Portrait comes back here.")
	imgui.Dummy(gui.ScaledVec2(0, 8))

	spacing := imgui.CurrentStyle().ItemSpacing().X
	width := (imgui.ContentRegionAvail().X - spacing*2) / 3

	imgui.BeginDisabledV(!ready)

	if choice(gui.Iconed(gui.IconAddCircle, "From Scratch"), "The neutral ethnicity_template: every gene on its first template, at the middle.", width) {
		a.state.welcome = false

		if p.basis.kind != basisScratch {
			a.fromScratch()
		}

	}

	imgui.SameLine()

	ethnicities, characters := 0, 0
	if ready {
		ethnicities, characters = p.ethnicities.Len(), p.characters.Len()
	}

	if choice(gui.Iconed(gui.IconGroups, "From an Ethnicity"), fmt.Sprintf("One of the %d ethnicities in common/ethnicities, at its likeliest look.", ethnicities), width) {
		a.openPicker(pickEthnicities)
	}

	imgui.SameLine()

	if choice(gui.Iconed(gui.IconPerson, "From a Character"), fmt.Sprintf("One of the %d characters in common/dna_data, with their exact DNA.", characters), width) {
		a.openPicker(pickCharacters)
	}

	imgui.EndDisabled()

	imgui.Dummy(gui.ScaledVec2(0, 6))
	imgui.Separator()

	switch {
	case !a.hasFolders():
		// The welcome screen stays open over the settings: once a folder is
		// saved the portrait is read, and the starting points are there.
		gui.WarningText(gui.Iconed(gui.IconWarning, "No game folder is configured"))
		dimmedWrapped("A portrait is made from the game's own files. Add the game's game folder, and the folders of any mods, in Settings, then pick a starting point here.")

		if imgui.Button(gui.Iconed(gui.IconSettings, "Open Settings")) {
			a.state.showSettings = true
		}

	case p == nil:
		gui.WarningText("The 3D view could not be started.")

	case p.problem != nil:
		gui.WarningText("The portrait could not be loaded: " + p.problem.Error())

	case !ready:
		imgui.TextDisabled("Reading the game's files...")

	default:
		imgui.TextDisabled(fmt.Sprintf("Read %d genes, %d ethnicities and %d characters", p.character.listed(), p.ethnicities.Len(), p.characters.Len()))

		if len(p.diagnostics) > 0 {
			imgui.SameLine()
			imgui.TextDisabled(fmt.Sprintf("· %d problems", len(p.diagnostics)))
		}
	}
}

// choice is one of the welcome screen's starting points: a large button with
// a title and what it starts from beneath it. It reports whether it was
// clicked.
func choice(title, detail string, width float32) bool {
	imgui.PushIDStr(title)
	defer imgui.PopID()

	origin := imgui.CursorScreenPos()
	size := imgui.Vec2{X: width, Y: gui.Scaled(choiceHeight)}

	clicked := imgui.ButtonV("##choice", size)

	// The title and the detail are drawn over the button, each from the
	// left of it: a new line of a window starts at the window's left.
	padding := gui.Scaled(14)
	imgui.SetCursorScreenPos(imgui.Vec2{X: origin.X + padding, Y: origin.Y + padding})

	gui.PushStrongFont()
	imgui.TextUnformatted(title)
	gui.PopFont()

	imgui.SetCursorScreenPos(imgui.Vec2{X: origin.X + padding, Y: imgui.CursorScreenPos().Y + gui.Scaled(4)})
	imgui.PushTextWrapPosV(imgui.CursorPosX() + width - padding*2)
	imgui.PushStyleColorVec4(imgui.ColText, *imgui.StyleColorVec4(imgui.ColTextDisabled))
	imgui.TextWrapped(detail)
	imgui.PopStyleColor()
	imgui.PopTextWrapPos()

	imgui.SetCursorScreenPos(imgui.Vec2{X: origin.X + width, Y: origin.Y})
	imgui.Dummy(imgui.Vec2{Y: size.Y})

	return clicked
}

// openPicker opens the picker over a list, and remembers what the portrait
// is drawn from now, for going back.
func (a *App) openPicker(kind pickerKind) {
	p := a.portrait

	a.state.welcome = false
	a.state.picker = picker{open: true, kind: kind, sex: p.sex, before: p.basis, beforeDNA: p.character.dna(), beforeSex: p.sex, beforeLoaded: true}
}

// pickables are what the picker lists, by the keys the game defines them
// under, in alphabetical order. The game files give a character or an
// ethnicity no name of its own beside its key.
func (a *App) pickables() []string {
	p := a.portrait

	var keys []string

	if a.state.picker.kind == pickEthnicities {
		keys = p.ethnicities.Keys()
	} else {
		keys = p.characters.Keys()
	}

	slices.Sort(keys)

	return keys
}

// pickerWindow is the picker, while it is open: the list on the left, the
// portrait as it is drawn from what is picked on the right.
func (a *App) pickerWindow() {
	state := &a.state.picker
	if !state.open || a.portrait == nil {
		return
	}

	a.cover()
	centreNext(pickerWidth, pickerHeight)

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoCollapse | imgui.WindowFlagsNoDocking

	visible := imgui.BeginV(windowPicker, nil, flags)
	defer imgui.End()

	a.toFront()

	if !visible {
		return
	}

	if imgui.Button(gui.Iconed(gui.IconBack, "Back")) {
		a.closePicker(true)
		a.state.welcome = true

		return
	}

	imgui.SameLine()

	for _, kind := range []pickerKind{pickCharacters, pickEthnicities} {
		label := gui.Iconed(gui.IconPerson, "Characters")
		if kind == pickEthnicities {
			label = gui.Iconed(gui.IconGroups, "Ethnicities")
		}

		if toggle(label, state.kind == kind) && state.kind != kind {
			state.kind, state.search, state.picked = kind, "", ""
		}

		imgui.SameLine()
	}

	imgui.NewLine()
	imgui.Separator()

	if imgui.BeginChildStrV("##list", imgui.Vec2{X: gui.Scaled(pickerListing)}, imgui.ChildFlagsAlwaysUseWindowPadding, 0) {
		a.pickerList()
	}

	imgui.EndChild()
	imgui.SameLine()

	if imgui.BeginChildStrV("##preview", imgui.Vec2{}, imgui.ChildFlagsAlwaysUseWindowPadding|imgui.ChildFlagsBorders, 0) {
		a.pickerPreview()
	}

	imgui.EndChild()

}

// pickerList is the search and the list of the picker; picking an entry draws
// the portrait from it.
func (a *App) pickerList() {
	state := &a.state.picker

	hint := "Search the characters"
	if state.kind == pickEthnicities {
		hint = "Search the ethnicities"
	}

	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##search", gui.Iconed(gui.IconSearch, hint), &state.search, 0, nil)

	if !imgui.BeginChildStrV("##entries", imgui.Vec2{}, imgui.ChildFlagsBorders, 0) {
		imgui.EndChild()

		return
	}

	search := strings.ToLower(strings.TrimSpace(state.search))

	for _, key := range a.pickables() {
		if search != "" && !strings.Contains(strings.ToLower(key), search) {
			continue
		}

		if imgui.SelectableBoolV(key, key == state.picked, 0, imgui.Vec2{}) && key != state.picked {
			a.pick(key)
		}
	}

	imgui.EndChild()
}

// pick draws the portrait from an entry of the picker: an ethnicity at its
// likeliest look, or a character's DNA as written, for the sex the picker is
// set to, or the one the character's DNA says.
func (a *App) pick(key string) {
	state := &a.state.picker
	p := a.portrait

	state.picked = key

	if state.kind == pickEthnicities {
		ethnicity, err := genes.Flatten(p.ethnicities, key)
		if err != nil {
			a.setStatus("Ethnicity %s could not be read: %v", key, err)

			return
		}

		a.start(basis{kind: basisEthnicity, key: key, ethnicity: ethnicity}, p.genes.Preview(ethnicity, genes.PreviewLikeliest), state.sex)

		return
	}

	dna, ok := p.characters.Get(key)
	if !ok {
		return
	}

	switch dna.Sex {
	case "female":
		state.sex = genes.Female
	case "male":
		state.sex = genes.Male
	case "girl":
		state.sex = genes.Girl
	case "boy":
		state.sex = genes.Boy
	}

	a.start(basis{kind: basisCharacter, key: key}, dna, state.sex)
}

// pickerPreview is the portrait as it is drawn from what is picked, what it
// is, its sex, and the way on to the creator.
func (a *App) pickerPreview() {
	state := &a.state.picker
	p := a.portrait

	footer := imgui.FrameHeightWithSpacing()*3 + imgui.TextLineHeightWithSpacing()*2
	available := imgui.ContentRegionAvail()
	box := imgui.Vec2{X: available.X, Y: max(available.Y-footer, 1)}

	// The portrait's picture is drawn at the size of the portrait panel, and
	// shown here as large as fits without being stretched. Until something
	// is picked there is nothing of it to show.
	switch {
	case state.picked == "":
		emptyBox(box, "Pick one from the list to see it")
	case !p.loaded() || p.loading || p.rebuilding:
		emptyBox(box, "Loading the portrait...")
	default:
		width, height := p.viewer.Size()
		scale := min(box.X/float32(width), box.Y/float32(height))
		shown := imgui.Vec2{X: float32(width) * scale, Y: float32(height) * scale}
		top := imgui.CursorPosY()

		imgui.SetCursorPosX(imgui.CursorPosX() + (box.X-shown.X)/2)
		a.window.Backend().DrawImageRenderTexture(p.viewer.Target(), shown, true, false)

		// The box is taken up as the placeholders take it, spacing and all,
		// so that what is below it stays where it is when the picture comes.
		imgui.SetCursorPosY(top)
		imgui.Dummy(imgui.Vec2{Y: box.Y})
	}

	// The views, centred under the picture.
	style := imgui.CurrentStyle()
	views := imgui.CalcTextSize(labelFace).X + imgui.CalcTextSize(labelFigure).X + style.FramePadding().X*4 + style.ItemSpacing().X

	imgui.SetCursorPosX(imgui.CursorPosX() + max((box.X-views)/2, 0))
	imgui.BeginDisabledV(state.picked == "")
	a.viewButtons()
	imgui.EndDisabled()

	gui.PushStrongFont()

	if state.picked == "" {
		imgui.TextUnformatted("Pick one from the list")
	} else {
		imgui.TextUnformatted(state.picked)
	}

	gui.PopFont()

	if state.kind == pickEthnicities {
		imgui.TextDisabled("Each gene at its likeliest choice, each range at its middle")
	} else {
		imgui.TextDisabled("The character's DNA as the game writes it")
	}

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Sex")
	imgui.SameLine()

	for _, sex := range genes.Sexes {
		if toggle(sexLabel(sex), state.sex == sex) && state.sex != sex {
			state.sex = sex

			if state.picked != "" {
				a.pick(state.picked)
			}
		}

		imgui.SameLine()
	}

	label := gui.Iconed(gui.IconForward, "Start Creating")
	imgui.SetCursorPosX(imgui.CursorPosX() + max(imgui.ContentRegionAvail().X-imgui.CalcTextSize(label).X-imgui.CurrentStyle().FramePadding().X*2, 0))

	imgui.BeginDisabledV(state.picked == "")

	if imgui.Button(label) {
		a.closePicker(false)
	}

	imgui.EndDisabled()
}

// closePicker closes the picker, and draws the portrait as it was before it
// opened when going back.
func (a *App) closePicker(back bool) {
	state := a.state.picker
	a.state.picker = picker{}

	if back && state.beforeLoaded && state.picked != "" {
		a.start(state.before, state.beforeDNA, state.beforeSex)
	}
}

// fromScratch draws the portrait from the template ethnicity, every gene
// neutral, keeping its sex.
func (a *App) fromScratch() {
	p := a.portrait

	ethnicity, err := genes.Flatten(p.ethnicities, templateEthnicity)
	if err != nil {
		a.setStatus("%s could not be read: %v; the portrait is neutral", templateEthnicity, err)
	}

	a.start(basis{kind: basisScratch, key: templateEthnicity, ethnicity: ethnicity}, p.genes.Preview(ethnicity, genes.PreviewNeutral), p.sex)
}

// emptyBox fills a box with a dimmed line in its middle, saying why there is
// nothing in it.
func emptyBox(box imgui.Vec2, line string) {
	origin := imgui.CursorPos()
	size := imgui.CalcTextSize(line)

	imgui.SetCursorPos(imgui.Vec2{X: origin.X + max((box.X-size.X)/2, 0), Y: origin.Y + max((box.Y-size.Y)/2, 0)})
	imgui.TextDisabled(line)
	imgui.SetCursorPos(origin)
	imgui.Dummy(box)
}
