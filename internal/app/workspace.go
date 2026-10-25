package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// The workspace of a portrait modifier project has windows of its own in the
// place of the character panel: the Portrait Modifiers window on the left,
// which lists the groups and the elements of the one picked, the Element
// window on the right, which edits the element picked, and the Weight window,
// which shows when the game uses it. The portrait in the middle wears the
// element picked, on a base the bar over it swaps for another.

// workflow is what the windows are for.
type workflow int

const (
	modeCharacter workflow = iota
	modeModifiers
)

// workspaceState is what the windows of a project remember between frames.
type workspaceState struct {
	groupSearch, elementSearch string

	// listSearch is the search of the list open in the Element window, which
	// they share, as only one is open at a time.
	listSearch string

	showWeight bool

	// The dialogs of the project and their forms; see projectdialogs.go and
	// modsave.go.
	newElement newElementForm
	newGroup   newGroupForm
	save       saveForm
}

// setMode changes the workflow the windows are for. The character being
// made keeps its looks through a project, which dresses its base its own
// way.
func (a *App) setMode(mode workflow) {
	if a.state.mode == mode {
		return
	}

	if p := a.portrait; p != nil && p.character != nil {
		groups := p.character.looks.groups

		if mode == modeModifiers {
			a.state.characterLooks = p.character.looks
			p.character.looks = looks{groups: groups}
		} else {
			kept := a.state.characterLooks
			kept.groups = groups
			p.character.looks = kept
		}

		p.character.redress()
	}

	a.state.mode = mode
	if mode == modeCharacter {
		a.state.selectCharacter = selectFrames
	}
}

// syncLooks gives the looks the groups as the project draws them, and in a
// project dresses the base in the element picked: alone, or with the other
// groups as the game would dress it.
func (a *App) syncLooks() {
	p := a.portrait
	if p == nil || p.character == nil {
		return
	}

	l := &p.character.looks
	l.groups = a.project.over(p.modifiers)

	if a.state.mode != modeModifiers {
		return
	}

	pr := a.project

	only := ""
	if !pr.wearOthers {
		only = pr.group
	}

	if !l.on || l.only != only || l.tryingGroup != pr.group || l.trying != pr.element {
		l.on, l.only = true, only
		l.try(pr.group, pr.element)
		p.character.redress()
	}
}

// baseBar is drawn over the top of the portrait of a project: the base that
// wears the portrait modifiers, with a way to another, its sex and age, and
// whether it wears the other groups too.
func (a *App) baseBar() {
	p := a.portrait
	c := p.character

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Base")
	imgui.SameLine()

	if imgui.Button(gui.Iconed(gui.IconPerson, p.basis.describe())) {
		a.startOver()
	}

	imgui.SetItemTooltip("Wear the portrait modifiers on another character; what the project has changed stays as it is")

	imgui.SameLineV(0, gui.Scaled(24))

	for _, sex := range genes.Sexes {
		if toggle(sexLabel(sex), c.sex == sex) && c.sex != sex {
			a.start(p.basis, c.dna(), sex)
		}

		imgui.SameLine()
	}

	// The portrait of a project shares the window with two others, so the
	// age and what the base wears go on a line of their own.
	imgui.NewLine()
	imgui.SetCursorPosX(gui.Scaled(overlayMargin))
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(gui.Iconed(gui.IconAge, "Age"))
	imgui.SameLine()
	imgui.SetNextItemWidth(gui.Scaled(140))

	if imgui.SliderIntV("##age", &c.age, 0, maxAge, "%d years", imgui.SliderFlagsAlwaysClamp) {
		c.changed = true
	}

	imgui.SameLineV(0, gui.Scaled(24))
	imgui.Checkbox("Also wear the other portrait modifiers", &a.project.wearOthers)
	imgui.SetItemTooltip("Dress the base in the other groups as the game would, or in the element picked alone")
}

// projectPanel is the Portrait Modifiers window of a project: the groups, and
// the elements of the one picked.
func (a *App) projectPanel() {
	if a.state.mode != modeModifiers {
		return
	}

	a.dockFirstTime(panelProject, panelPreview, imgui.DirLeft, projectListRatio)

	defer imgui.End()

	if !imgui.Begin(panelProject) {
		return
	}

	if a.portrait == nil || !a.portrait.loaded() || a.hidden() {
		imgui.TextDisabled("Pick a base first")

		return
	}

	pr := a.project
	ws := &a.state.workspace
	groups := a.portrait.character.looks.groups

	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##groups", gui.Iconed(gui.IconSearch, fmt.Sprintf("Filter %d portrait modifiers", len(groups))), &ws.groupSearch, 0, nil)

	if imgui.BeginChildStrV("##grouplist", imgui.Vec2{Y: imgui.ContentRegionAvail().Y * groupListShare}, imgui.ChildFlagsBorders, 0) {
		search := strings.ToLower(strings.TrimSpace(ws.groupSearch))

		for _, group := range groups {
			if search != "" && !strings.Contains(strings.ToLower(group.Key), search) {
				continue
			}

			detail := fmt.Sprintf("%d", len(group.Modifiers))
			if changed, ok := pr.groups[group.Key]; ok && changed.unsaved() {
				detail = fmt.Sprintf("%s · %d", unsavedSummary(changed), len(group.Modifiers))
			}

			if listRow(group.Key, detail, group.Key == pr.group, group.Usage != usageGame) && group.Key != pr.group {
				pr.group, pr.element, ws.elementSearch = group.Key, "", ""
			}

			if group.Usage != usageGame {
				imgui.SetItemTooltip("usage = " + group.Usage + ": only worn where script names it")
			}
		}
	}

	imgui.EndChild()

	if imgui.Button(gui.Iconed(gui.IconAdd, "New portrait modifier...")) {
		a.openNewGroup()
	}

	imgui.Separator()

	group, ok := a.portrait.character.looks.group(pr.group)
	if !ok {
		imgui.TextDisabled("Pick a portrait modifier to see its elements")

		return
	}

	gui.PushStrongFont()
	imgui.TextUnformatted("Elements of " + group.Key)
	gui.PopFont()

	if imgui.Button(gui.Iconed(gui.IconAdd, "New element...")) {
		a.openNewElement(group.Key, nil)
	}

	names := make([]string, len(group.Modifiers))
	for index := range group.Modifiers {
		names[index] = group.Modifiers[index].Name
	}

	shown := matching(names, ws.elementSearch)

	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##elements", gui.Iconed(gui.IconSearch, fmt.Sprintf("Search %d elements", len(names))), &ws.elementSearch, 0, nil)

	if strings.TrimSpace(ws.elementSearch) != "" {
		imgui.TextDisabled(fmt.Sprintf("%d of %d", len(shown), len(names)))
	}

	if imgui.BeginChildStrV("##elementlist", imgui.Vec2{}, imgui.ChildFlagsBorders, 0) {
		changed := pr.groups[group.Key]

		for _, position := range shown {
			name := names[position]

			detail := ""
			if changed != nil {
				detail = stateLabel(changed.elements[name])
			}

			imgui.PushIDInt(int32(position))

			if listRow(name, detail, name == pr.element, false) {
				pr.element = name
			}

			imgui.PopID()
		}
	}

	imgui.EndChild()
}

// The workspace's proportions: how much of the portrait's node the list of
// groups takes when it is first docked, and how much of its window the list
// of groups takes above the elements.
const (
	projectListRatio = 0.27
	groupListShare   = 0.4
)

// listRow is one row of a list: a name, and a detail after it at the right,
// such as how many elements a group has. It reports whether it was clicked.
func listRow(name, detail string, selected, dimmed bool) bool {
	width := imgui.ContentRegionAvail().X
	start := imgui.CursorPosX()

	if dimmed {
		imgui.PushStyleColorVec4(imgui.ColText, *imgui.StyleColorVec4(imgui.ColTextDisabled))
	}

	clicked := imgui.SelectableBoolV(name+"##"+name, selected, imgui.SelectableFlagsAllowOverlap, imgui.Vec2{})

	if dimmed {
		imgui.PopStyleColor()
	}

	if detail != "" {
		imgui.SameLineV(start+width-imgui.CalcTextSize(detail).X, 0)
		imgui.TextDisabled(detail)
	}

	return clicked
}

// stateLabel is what a list says of an element the project has done
// something to.
func stateLabel(state elementState) string {
	switch state {
	case elementChanged:
		return "changed"
	case elementNew:
		return "new"
	}

	return ""
}

// unsavedSummary says what a group has unsaved: that it is new, or how many
// elements are changed and made.
func unsavedSummary(group *projectGroup) string {
	if group.fresh {
		return "new"
	}

	changed, made := group.counts()

	var parts []string
	if changed > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", changed))
	}

	if made > 0 {
		parts = append(parts, fmt.Sprintf("%d new", made))
	}

	return strings.Join(parts, ", ")
}

// pickedElement is the group and the element the project shows, and its
// place among the group's elements.
func (a *App) pickedElement() (*genes.ModifierGroup, *genes.PortraitModifier, int, bool) {
	pr := a.project

	group, ok := a.portrait.character.looks.group(pr.group)
	if !ok {
		return nil, nil, 0, false
	}

	at := slices.IndexFunc(group.Modifiers, func(m genes.PortraitModifier) bool { return m.Name == pr.element })
	if at < 0 {
		return group, nil, 0, false
	}

	return group, &group.Modifiers[at], at, true
}

// elementPanel is the Element window of a project: the element picked, its
// changes of the genes to edit, and its weight.
func (a *App) elementPanel() {
	if a.state.mode != modeModifiers {
		return
	}

	a.dockFirstTime(panelElement, panelCharacter, imgui.DirNone, 0)

	defer imgui.End()

	if !imgui.Begin(panelElement) {
		return
	}

	if a.portrait == nil || !a.portrait.loaded() || a.hidden() {
		imgui.TextDisabled("Pick a base first")

		return
	}

	group, modifier, at, ok := a.pickedElement()
	if !ok {
		emptyState("Pick an element of a portrait modifier", "or make a new one")

		return
	}

	pr := a.project
	p := a.portrait
	changed := pr.groups[group.Key]

	state := elementSaved
	if changed != nil {
		state = changed.elements[modifier.Name]
	}

	gui.PushStrongFont()
	imgui.TextUnformatted(group.Key + " / " + modifier.Name)
	gui.PopFont()

	if label := stateLabel(state); label != "" {
		imgui.SameLine()
		gui.WarningText(label)
	}

	dimmedWrapped(a.elementOrigin(modifier, state))
	imgui.Separator()

	// The element is edited as a copy, which takes the place of the
	// project's once anything of it changes; the game's are copied into the
	// project then.
	edited := cloneModifier(modifier)

	if a.geneChanges(&edited) {
		project := pr.edit(group)
		project.Modifiers[at] = edited
		project.changed(edited.Name)
		p.character.redress()
	}

	imgui.Separator()
	a.weightRow(&edited)

	imgui.Separator()

	imgui.BeginDisabledV(state == elementSaved)

	if imgui.Button(gui.Iconed(gui.IconReset, "Revert element")) {
		pr.revert(p.modifiers, group.Key, modifier.Name)
		p.character.redress()
	}

	imgui.EndDisabled()

	if state == elementNew {
		imgui.SetItemTooltip("Take the new element away again")
	} else {
		imgui.SetItemTooltip("Set the element back to what the game's files say")
	}

	imgui.SameLine()

	if imgui.Button(gui.Iconed(gui.IconCopy, "Copy as new element...")) {
		a.openNewElement(group.Key, modifier)
	}

	imgui.SameLine()

	imgui.BeginDisabledV(!a.canSaveModifier())

	if imgui.Button(gui.Iconed(gui.IconSave, labelSaveModifier)) {
		a.saveModifier()
	}

	imgui.EndDisabled()
}

// elementOrigin says where an element comes from, and where what is done to
// it goes.
func (a *App) elementOrigin(modifier *genes.PortraitModifier, state elementState) string {
	if state == elementNew || modifier.Source.Origin.Path == "" {
		return "Made in this project. Nothing is written until its portrait modifier is saved."
	}

	origin := modifier.Source.Origin
	where := fmt.Sprintf("%s:%d", origin.File, modifier.Source.Line)

	if a.inGame(origin.Path) {
		return "From the game, " + where + ". Nothing is written until its portrait modifier is saved."
	}

	return fmt.Sprintf("From %s, %s. Nothing is written until its portrait modifier is saved.", origin.Source, where)
}

// The kinds of changes of the genes, as the games' files write them, and the
// modes each takes.
var (
	geneKinds = []string{"accessory", "morph", "color"}

	geneModes = map[string][]string{
		"accessory": {genes.ModifierAdd, genes.ModifierReplace, genes.ModifierModify},
		"morph":     {genes.ModifierAdd, genes.ModifierReplace, genes.ModifierModify, genes.ModifierModifyMultiply},
		"color":     {genes.ModifierReplace, genes.ModifierModify},
	}

	// modeHints say what each mode does, as the games' own example of
	// Europa Universalis 5 describes them.
	modeHints = map[string]string{
		genes.ModifierAdd:            "Added beside the DNA's; an accessory is worn in the place of the DNA's",
		genes.ModifierReplace:        "Takes the place of the DNA's template and strength",
		genes.ModifierModify:         "Added to the DNA's strength, where its template is the one named or none is",
		genes.ModifierModifyMultiply: "Multiplies the DNA's strength (Europa Universalis 5)",
	}
)

// sexTypes are who a change can be for, as the games' files write it, an
// empty one for everyone.
var sexTypes = []string{"", "male", "female", "boy", "girl"}

// geneChanges edits the changes of the genes of an element, and reports
// whether anything changed.
func (a *App) geneChanges(modifier *genes.PortraitModifier) bool {
	changed := false

	gui.PushStrongFont()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(fmt.Sprintf("DNA changes · %d", len(modifier.Genes)))
	gui.PopFont()
	imgui.SameLine()

	imgui.SetCursorPosX(imgui.CursorPosX() + max(imgui.ContentRegionAvail().X-gui.Scaled(140), 0))
	imgui.SetNextItemWidth(-1)

	if imgui.BeginCombo("##add", gui.Iconed(gui.IconAdd, "Add change")) {
		for _, kind := range geneKinds {
			if imgui.SelectableBool(kind) {
				mode := genes.ModifierAdd
				if kind == "color" {
					mode = genes.ModifierReplace
				}

				modifier.Genes = append(modifier.Genes, genes.GeneModifier{Kind: kind, Mode: mode, Value: database.Some(1.0)})
				changed = true
			}
		}

		imgui.EndCombo()
	}

	if len(modifier.Genes) == 0 {
		imgui.TextDisabled("It changes no gene yet")
	}

	removed := -1

	for index := range modifier.Genes {
		imgui.PushIDInt(int32(index))

		edited, remove := a.geneChange(&modifier.Genes[index])
		changed = changed || edited || remove

		if remove {
			removed = index
		}

		imgui.PopID()
	}

	if removed >= 0 {
		modifier.Genes = slices.Delete(modifier.Genes, removed, removed+1)
	}

	if a.usesOutfitTags() || len(modifier.OutfitTags) > 0 {
		tags := strings.Join(modifier.OutfitTags, " ")

		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted("outfit_tags")
		imgui.SameLine()
		imgui.SetNextItemWidth(-1)

		if imgui.InputTextWithHint("##outfittags", "none", &tags, 0, nil) {
			modifier.OutfitTags = strings.Fields(tags)
			changed = true
		}
	}

	return changed
}

// usesOutfitTags reports whether any element of the game's tags its outfit,
// as Europa Universalis 5's do and Victoria 3's do not.
func (a *App) usesOutfitTags() bool {
	for _, group := range a.portrait.character.looks.groups {
		for index := range group.Modifiers {
			if len(group.Modifiers[index].OutfitTags) > 0 {
				return true
			}
		}
	}

	return false
}

// geneChange edits one change of a gene, in a box of its own, and reports
// whether it changed and whether it is to be taken away.
func (a *App) geneChange(change *genes.GeneModifier) (changed, remove bool) {
	imgui.BeginChildStrV("##change", imgui.Vec2{}, imgui.ChildFlagsBorders|imgui.ChildFlagsAutoResizeY|imgui.ChildFlagsAlwaysUseWindowPadding, 0)
	defer imgui.EndChild()

	gui.PushStrongFont()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(change.Kind)
	gui.PopFont()

	label := gui.Iconed(gui.IconDelete, "")
	imgui.SameLine()
	imgui.SetCursorPosX(imgui.CursorPosX() + max(imgui.ContentRegionAvail().X-imgui.CalcTextSize(label).X-imgui.CurrentStyle().FramePadding().X*2, 0))

	if imgui.Button(label) {
		remove = true
	}

	imgui.SetItemTooltip("Take this change away")

	field := func(name string) {
		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted(name)
		imgui.SameLineV(gui.Scaled(fieldLabelWidth), 0)
		imgui.SetNextItemWidth(-1)
	}

	field("mode")

	if picked, ok := pickOne("##mode", change.Mode, "none", geneModes[change.Kind]); ok {
		change.Mode, changed = picked, true
	}

	if hint, ok := modeHints[change.Mode]; ok {
		imgui.SetCursorPosX(gui.Scaled(fieldLabelWidth))
		dimmedWrapped(hint)
	}

	field("gene")

	if picked, ok := a.searchCombo("##gene", change.Gene, "", a.geneKeys(change.Kind)); ok {
		change.Gene, change.Template, changed = picked, "", true
	}

	if change.Kind != "color" {
		field("template")

		if picked, ok := a.searchCombo("##template", change.Template, "none", a.templateNames(change.Kind, change.Gene)); ok {
			change.Template, changed = picked, true
		}
	}

	if change.Kind == "color" {
		x, y := float32(change.X.Or(0)), float32(change.Y.Or(0))

		field("x")

		if imgui.SliderFloat("##x", &x, -1, 1) {
			change.X, changed = database.Some(float64(x)), true
		}

		field("y")

		if imgui.SliderFloat("##y", &y, -1, 1) {
			change.Y, changed = database.Some(float64(y)), true
		}
	} else {
		changed = strengthFields(change, field) || changed
	}

	if change.Kind == "accessory" {
		field("accessory")

		if imgui.InputTextWithHint("##accessory", "picked by the strength", &change.Accessory, 0, nil) {
			changed = true
		}
	}

	field("for")

	for _, sex := range sexTypes {
		label := sex
		if label == "" {
			label = "everyone"
		}

		if toggle(label, change.Type == sex) && change.Type != sex {
			change.Type, changed = sex, true
		}

		imgui.SameLine()
	}

	imgui.NewLine()

	return changed, remove
}

// fieldLabelWidth is how wide the names of the fields of a change are,
// unscaled.
const fieldLabelWidth = 90

// strengthFields edit the strength of a change: one value, or a range the
// game picks one from by chance. They report whether it changed.
func strengthFields(change *genes.GeneModifier, field func(string)) bool {
	changed := false
	ranged := change.Range.Set

	field("strength")

	if toggle("value", !ranged) && ranged {
		change.Value, change.Range = database.Some(change.Strength()), database.Opt[genes.Range]{}
		changed, ranged = true, false
	}

	imgui.SameLine()

	if toggle("range", ranged) && !ranged {
		change.Range, change.Value = database.Some(genes.Range{Min: 0, Max: 1}), database.Opt[float64]{}
		changed, ranged = true, true
	}

	imgui.SetCursorPosX(gui.Scaled(fieldLabelWidth))
	imgui.SetNextItemWidth(-1)

	if ranged {
		spread := change.Range.Value
		values := [2]float32{float32(spread.Min), float32(spread.Max)}

		if imgui.SliderFloat2("##range", &values, 0, 1) {
			change.Range, changed = database.Some(genes.Range{Min: float64(values[0]), Max: float64(values[1])}), true
		}

		return changed
	}

	value := float32(change.Value.Or(0))

	if imgui.SliderFloat("##value", &value, -1, 1) {
		change.Value, changed = database.Some(float64(value)), true
	}

	return changed
}

// geneKeys are the keys of the genes of a kind, in order.
func (a *App) geneKeys(kind string) []string {
	all := a.portrait.genes

	var keys []string

	switch kind {
	case "accessory":
		keys = all.AccessoryGenes.Keys()
	case "morph":
		keys = all.MorphGenes.Keys()
	case "color":
		keys = all.ColorGenes.Keys()
	}

	slices.Sort(keys)

	return keys
}

// templateNames are the names of the templates of a gene, in the order
// written.
func (a *App) templateNames(kind, key string) []string {
	all := a.portrait.genes

	var names []string

	switch kind {
	case "accessory":
		if gene, ok := all.AccessoryGenes.Get(key); ok {
			for _, template := range gene.Templates {
				names = append(names, template.Name)
			}
		}

	case "morph":
		if gene, ok := all.MorphGenes.Get(key); ok {
			for _, template := range gene.Templates {
				names = append(names, template.Name)
			}
		}
	}

	return names
}

// searchCombo is a list of keys to pick one of with a search above it, or
// none where none is named, which it shows as that name. It reports the key
// picked, empty for none, when one is.
func (a *App) searchCombo(id, current, none string, keys []string) (string, bool) {
	shown := current
	if shown == "" {
		shown = none
	}

	if !imgui.BeginComboV(id, shown, imgui.ComboFlagsHeightLarge) {
		return "", false
	}
	defer imgui.EndCombo()

	ws := &a.state.workspace

	if imgui.IsWindowAppearing() {
		ws.listSearch = ""
		imgui.SetKeyboardFocusHere()
	}

	imgui.SetNextItemWidth(-1)
	entered := imgui.InputTextWithHint("##search", gui.Iconed(gui.IconSearch, "Search"), &ws.listSearch, imgui.InputTextFlagsEnterReturnsTrue, nil)

	found := matching(keys, ws.listSearch)

	if entered && len(found) > 0 {
		imgui.CloseCurrentPopup()

		return keys[found[0]], true
	}

	picked, ok := "", false

	if none != "" && imgui.SelectableBoolV(none+"##none", current == "", 0, imgui.Vec2{}) && current != "" {
		picked, ok = "", true
	}

	rows := min(max(len(found), 1), templateRows)
	appearing := imgui.IsWindowAppearing()

	imgui.BeginChildStrV("##keys", imgui.Vec2{Y: float32(rows) * imgui.TextLineHeightWithSpacing()}, 0, 0)

	for _, position := range found {
		key := keys[position]

		if imgui.SelectableBoolV(key, key == current, 0, imgui.Vec2{}) && key != current {
			picked, ok = key, true
			imgui.CloseCurrentPopup()
		}

		if key == current && appearing {
			imgui.SetScrollHereY()
		}
	}

	imgui.EndChild()

	return picked, ok
}

// weightRow says what the weight of an element is, and opens the Weight
// window, which shows when the game uses it.
func (a *App) weightRow(modifier *genes.PortraitModifier) {
	gui.PushStrongFont()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Weight")
	gui.PopFont()
	imgui.SameLine()
	imgui.TextDisabled(weightSummary(modifier))
	imgui.SameLine()

	label := "When does the game use it?"
	imgui.SetCursorPosX(imgui.CursorPosX() + max(imgui.ContentRegionAvail().X-imgui.CalcTextSize(label).X-imgui.CurrentStyle().FramePadding().X*2, 0))

	if toggle(label, a.state.workspace.showWeight) {
		a.state.workspace.showWeight = !a.state.workspace.showWeight
	}
}

// weightSummary says what a weight starts from and how many conditions
// change it.
func weightSummary(modifier *genes.PortraitModifier) string {
	weight, ok := modifier.Weight.Get()
	if !ok {
		return "none written"
	}

	summary := "base " + formatNumber(weight.Base.Or(0))

	switch weight.Conditions {
	case 0:
		return summary
	case 1:
		return summary + " · 1 condition"
	}

	return fmt.Sprintf("%s · %d conditions", summary, weight.Conditions)
}

// formatNumber writes a number as briefly as it reads.
func formatNumber(value float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
}

// weightWindow shows the weight of the element picked as the game's files
// write it, which the game weighs the element by for a character.
func (a *App) weightWindow() {
	ws := &a.state.workspace
	if !ws.showWeight || a.state.mode != modeModifiers || a.portrait == nil || !a.portrait.loaded() || a.hidden() {
		return
	}

	viewport := imgui.MainViewport()
	centre := imgui.Vec2{X: viewport.WorkPos().X + viewport.WorkSize().X/2, Y: viewport.WorkPos().Y + viewport.WorkSize().Y/2}

	imgui.SetNextWindowPosV(centre, imgui.CondFirstUseEver, imgui.Vec2{X: 0.5, Y: 0.5})
	imgui.SetNextWindowSizeV(gui.ScaledVec2(560, 460), imgui.CondFirstUseEver)

	defer imgui.End()

	if !imgui.BeginV(windowWeight, &ws.showWeight, imgui.WindowFlagsNoDocking) {
		return
	}

	a.trackFocus(windowWeight)

	group, modifier, _, ok := a.pickedElement()
	if !ok {
		imgui.TextDisabled("Pick an element to see its weight")

		return
	}

	pr := a.project

	var (
		text string
		err  error
	)

	if changed, ok := pr.groups[group.Key]; ok {
		text, err = pr.weight(changed, modifier)
	} else {
		text, err = pr.gameWeight(group.Key, modifier)
	}

	gui.PushStrongFont()
	imgui.TextUnformatted(modifier.Name)
	gui.PopFont()

	if weight, ok := modifier.Weight.Get(); ok && weight.Source.Origin.Path != "" {
		imgui.SameLine()
		imgui.TextDisabled(fmt.Sprintf("%s:%d", weight.Source.Origin.File, weight.Source.Line))
	}

	dimmedWrapped(fmt.Sprintf("When the game dresses a character in it: %s. It can be tried on here whatever it says.", weightSummary(modifier)))

	switch {
	case err != nil:
		gui.ErrorText("The weight could not be read: " + err.Error())

		return
	case text == "":
		imgui.TextDisabled("The element is written without a weight")

		return
	}

	imgui.InputTextMultiline("##weight", &text, imgui.Vec2{X: -1, Y: -imgui.FrameHeightWithSpacing()}, imgui.InputTextFlagsReadOnly, nil)

	if imgui.Button(gui.Iconed(gui.IconCopy, "Copy")) {
		imgui.SetClipboardText(text)
	}

	imgui.SameLine()

	if changed := pr.groups[group.Key]; changed != nil && changed.created[modifier.Name] != "" {
		imgui.TextDisabled("New elements get this; write its conditions in the file once it is saved")
	} else {
		imgui.TextDisabled("Kept as it is written when the element is saved")
	}
}
