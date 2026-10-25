package app

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// A project adds elements to the groups there are, and makes groups of its
// own, each in a dialog of its own. Neither is written anywhere until the
// group is saved; see modsave.go.

// The titles of the dialogs, which Dear ImGui knows the popups by.
const (
	popupNewElement = "New element###newelement"
	popupNewGroup   = "New portrait modifier###newgroup"
)

// The width of the dialogs, unscaled.
const dialogWidth = 600

// startFrom is what a new element starts from.
type startFrom int

const (
	fromNothing startFrom = iota
	fromWorn
	fromOther
)

// newElementForm is what the dialog of a new element has been given: the
// group, the name, what it starts from, and the element copied for another.
type newElementForm struct {
	open        bool
	group, name string
	from        startFrom
	other       string
}

// openNewElement opens the dialog of a new element of a group, a copy of an
// element where one is given.
func (a *App) openNewElement(group string, copied *genes.PortraitModifier) {
	form := newElementForm{open: true, group: group, from: fromWorn}

	if copied != nil {
		form.from, form.other = fromOther, copied.Name
	}

	a.state.workspace.newElement = form
}

// newElementPopup is the dialog of a new element.
func (a *App) newElementPopup() {
	form := &a.state.workspace.newElement

	if form.open {
		form.open = false
		imgui.OpenPopupStr(popupNewElement)
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(dialogWidth, 0), imgui.CondAlways)

	if !imgui.BeginPopupModalV(popupNewElement, nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	l := &a.portrait.character.looks
	groups := l.groups

	keys := make([]string, len(groups))
	for index, group := range groups {
		keys[index] = group.Key
	}

	imgui.TextUnformatted("Portrait modifier")
	imgui.SetNextItemWidth(-1)

	if picked, ok := a.searchCombo("##group", form.group, "", keys); ok {
		form.group, form.other = picked, ""
	}

	group, ok := l.group(form.group)
	if !ok {
		imgui.TextDisabled("Pick the portrait modifier to add the element to")

		if imgui.Button("Cancel") {
			imgui.CloseCurrentPopup()
		}

		return
	}

	imgui.Spacing()
	imgui.TextUnformatted("Name")
	imgui.SetNextItemWidth(-1)

	if imgui.IsWindowAppearing() {
		imgui.SetKeyboardFocusHere()
	}

	imgui.InputTextWithHint("##name", "my_mod_element", &form.name, 0, nil)

	problem := elementNameProblem(group, form.name)
	if problem != "" {
		gui.WarningText(problem)
	} else {
		imgui.TextDisabled("No other element of " + group.Key + " has this name")
	}

	imgui.Spacing()
	imgui.TextUnformatted("Start from")

	worn, wearing := l.choice(group)

	if imgui.RadioButtonBool("Nothing: no DNA changes yet", form.from == fromNothing) {
		form.from = fromNothing
	}

	wornLabel := "A copy of what the portrait wears now: nothing of " + group.Key
	if wearing {
		wornLabel = fmt.Sprintf("A copy of what the portrait wears now: %s, %d DNA changes", worn.Name, len(worn.Genes))
	}

	imgui.BeginDisabledV(!wearing)

	if imgui.RadioButtonBool(wornLabel, form.from == fromWorn && wearing) {
		form.from = fromWorn
	}

	imgui.EndDisabled()

	if imgui.RadioButtonBool("A copy of another element", form.from == fromOther) {
		form.from = fromOther
	}

	if form.from == fromOther {
		names := make([]string, len(group.Modifiers))
		for index := range group.Modifiers {
			names[index] = group.Modifiers[index].Name
		}

		imgui.SetNextItemWidth(-1)

		if picked, ok := a.searchCombo("##other", form.other, "", names); ok {
			form.other = picked
		}
	}

	var copied *genes.PortraitModifier

	switch form.from {
	case fromWorn:
		if wearing {
			copied = worn
		}
	case fromOther:
		copied, _ = group.Modifier(form.other)
	}

	imgui.Spacing()
	imgui.TextUnformatted("Weight")

	weight := generatedWeight
	imgui.InputTextMultiline("##weight", &weight, imgui.Vec2{X: -1, Y: imgui.TextLineHeight() * 5}, imgui.InputTextFlagsReadOnly, nil)
	dimmedWrapped("At base 0 the game never picks it on its own. It can still be tried on here and named from script; write its conditions in the file once it is saved.")

	imgui.Spacing()
	imgui.Separator()

	ready := problem == "" && (form.from != fromOther || copied != nil)

	imgui.BeginDisabledV(!ready)

	if imgui.Button(gui.Iconed(gui.IconAdd, "Create and edit")) {
		a.project.edit(group).add(form.name, copied)
		a.project.group, a.project.element = group.Key, form.name
		a.portrait.character.redress()
		imgui.CloseCurrentPopup()
	}

	imgui.EndDisabled()
	imgui.SameLine()

	if imgui.Button("Cancel") {
		imgui.CloseCurrentPopup()
	}
}

// newGroupForm is what the dialog of a new group has been given: its key and
// settings, and the name of its first element.
type newGroupForm struct {
	open      bool
	key       string
	usage     string
	behavior  string
	priority  int32
	gruesome  bool
	first     string
	fallsBack bool
}

// openNewGroup opens the dialog of a new group, set as the games' groups
// mostly are.
func (a *App) openNewGroup() {
	a.state.workspace.newGroup = newGroupForm{open: true, usage: usageGame, behavior: behaviorWeighted, priority: 1}
}

// The ways the game picks an element of a group: at random by weight, or
// always the heaviest.
const (
	behaviorWeighted = "weighted_random"
	behaviorMax      = "max"
)

// newGroupPopup is the dialog of a new group.
func (a *App) newGroupPopup() {
	form := &a.state.workspace.newGroup

	if form.open {
		form.open = false
		imgui.OpenPopupStr(popupNewGroup)
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(dialogWidth, 0), imgui.CondAlways)

	if !imgui.BeginPopupModalV(popupNewGroup, nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	l := &a.portrait.character.looks

	setting := func(name string) {
		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted(name)
		imgui.SameLineV(gui.Scaled(settingLabelWidth), 0)
	}

	setting("Key")
	imgui.SetNextItemWidth(-1)

	if imgui.IsWindowAppearing() {
		imgui.SetKeyboardFocusHere()
	}

	imgui.InputTextWithHint("##key", "my_mod_auras", &form.key, 0, nil)

	keyProblem := groupKeyProblem(l.groups, form.key)
	imgui.SetCursorPosX(gui.Scaled(settingLabelWidth))

	if keyProblem != "" {
		gui.WarningText(keyProblem)
	} else {
		imgui.TextDisabled("Not used by the game or a mod")
	}

	setting("usage")

	for _, usage := range []string{usageGame, "none"} {
		if toggle(usage, form.usage == usage) {
			form.usage = usage
		}

		imgui.SameLine()
	}

	imgui.NewLine()
	imgui.SetCursorPosX(gui.Scaled(settingLabelWidth))
	dimmedWrapped("game: the game picks one element for every character. none: an element is only worn where script names it, as animations name their props.")

	setting("selection_behavior")

	for _, behavior := range []string{behaviorWeighted, behaviorMax} {
		if toggle(behavior, form.behavior == behavior) {
			form.behavior = behavior
		}

		imgui.SameLine()
	}

	imgui.NewLine()

	setting("priority")
	imgui.SetNextItemWidth(gui.Scaled(120))
	imgui.InputInt("##priority", &form.priority)
	imgui.SameLine()
	imgui.TextDisabled("Applied after lower ones, so it wins where both change a gene")

	setting("gruesome")
	imgui.Checkbox("Marked gruesome, as the game marks its plagues and scars", &form.gruesome)

	imgui.Separator()

	setting("First element")
	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##first", "my_mod_element", &form.first, 0, nil)

	draft := &genes.ModifierGroup{Definition: database.Definition{Key: form.key}}
	firstProblem := elementNameProblem(draft, form.first)

	if firstProblem != "" {
		imgui.SetCursorPosX(gui.Scaled(settingLabelWidth))
		gui.WarningText(firstProblem)
	}

	setting("fallback")
	imgui.Checkbox("The first element, worn where no element's weight is above 0", &form.fallsBack)

	imgui.Separator()

	ready := keyProblem == "" && firstProblem == ""

	imgui.BeginDisabledV(!ready)

	if imgui.Button(gui.Iconed(gui.IconAdd, "Create and edit")) {
		a.createGroup(form)
		imgui.CloseCurrentPopup()
	}

	imgui.EndDisabled()
	imgui.SameLine()

	if imgui.Button("Cancel") {
		imgui.CloseCurrentPopup()
	}
}

// settingLabelWidth is how wide the names of the settings of a new group
// are, unscaled.
const settingLabelWidth = 150

// createGroup makes the group a dialog has been given, with its first
// element, and shows it.
func (a *App) createGroup(form *newGroupForm) {
	group := &projectGroup{
		ModifierGroup: genes.ModifierGroup{
			Definition:        database.Definition{Key: form.key},
			Usage:             form.usage,
			SelectionBehavior: form.behavior,
			Priority:          database.Some(float64(form.priority)),
		},
		fresh:    true,
		elements: map[string]elementState{},
		created:  map[string]string{},
	}

	if form.gruesome {
		group.Gruesome = database.Some(true)
	}

	if form.fallsBack {
		group.Fallback = form.first
	}

	group.add(form.first, nil)

	a.project.groups[form.key] = group
	a.project.group, a.project.element = form.key, form.first
	a.portrait.character.redress()
}
