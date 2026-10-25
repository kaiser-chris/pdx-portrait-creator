package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/genes"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// The Portrait Modifiers window dresses the character being made in the
// game's portrait modifiers, to see it in its clothes once its DNA is done.
// It is a window of its own, docked beside the character panel to begin
// with, which can be dragged out. Dressing is off until it is turned on, and
// then every group the game picks from lists its elements to pick one of:
// pointing at one in the list tries it on.

// lookFilter is which of the groups the window lists.
type lookFilter int

const (
	lookAll lookFilter = iota
	lookPicked
	lookFromMods
)

// lookState is what the window remembers between frames: its filter, the
// search of the list open, and whether gruesome groups are hidden.
type lookState struct {
	filter       lookFilter
	search       string
	listSearch   string
	showGruesome bool
}

// The size of the lists of elements, unscaled: how many rows show before
// they scroll.
const elementRows = 16

// looksPanel is the Portrait Modifiers window of a character being made.
func (a *App) looksPanel() {
	if a.state.mode != modeCharacter {
		return
	}

	a.dockFirstTime(panelLooks, panelCharacter, imgui.DirNone, 0)

	defer imgui.End()

	if !imgui.Begin(panelLooks) {
		return
	}

	if a.portrait == nil || !a.portrait.loaded() || a.hidden() {
		imgui.TextDisabled("Pick a starting point first")

		return
	}

	p := a.portrait
	l := &p.character.looks
	state := &a.state.looks

	if imgui.Checkbox("Wear portrait modifiers", &l.on) {
		l.stopTrying()
		p.character.redress()
	}

	if !l.on {
		dimmedWrapped("Off, the portrait shows the DNA alone. Turn it on once the DNA is done, to see the character in the game's clothes, hair and beards.")
	}

	imgui.Separator()
	imgui.BeginDisabledV(!l.on)
	defer imgui.EndDisabled()

	shown, hidden := a.listedGroups(l)

	resetLabel := gui.Iconed(gui.IconReset, "Reset all")
	style := imgui.CurrentStyle()

	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X - imgui.CalcTextSize(resetLabel).X - style.FramePadding().X*2 - style.ItemSpacing().X)
	imgui.InputTextWithHint("##filter", gui.Iconed(gui.IconSearch, fmt.Sprintf("Filter %d portrait modifiers", len(shown))), &state.search, 0, nil)
	imgui.SameLine()

	if imgui.Button(resetLabel) {
		l.picked = nil
		l.stopTrying()
		p.character.redress()
	}

	imgui.SetItemTooltip("Dress every portrait modifier as the game would, untouched")

	picked, fromMods := 0, 0

	for _, group := range shown {
		if _, ok := l.picked[group.Key]; ok {
			picked++
		}

		if a.fromMod(group) {
			fromMods++
		}
	}

	if toggle(fmt.Sprintf("All %d", len(shown)), state.filter == lookAll) {
		state.filter = lookAll
	}

	imgui.SameLine()

	if toggle(fmt.Sprintf("Picked %d", picked), state.filter == lookPicked) {
		state.filter = lookPicked
	}

	if fromMods > 0 {
		imgui.SameLine()

		if toggle(fmt.Sprintf("From mods %d", fromMods), state.filter == lookFromMods) {
			state.filter = lookFromMods
		}
	}

	if gruesome := countGruesome(shown); gruesome > 0 {
		imgui.SameLine()
		imgui.Checkbox(fmt.Sprintf("Gruesome %d", gruesome), &state.showGruesome)
		imgui.SetItemTooltip("Show the groups the game marks gruesome, such as plagues and scars")
	}

	// The notes go under the table, which takes the room that leaves, so
	// that the window has nothing to scroll but the table.
	notes := []string{"To change portrait modifiers or make new ones, start a portrait modifier project from File > New Portrait."}
	if hidden > 0 {
		notes = append([]string{fmt.Sprintf("%d with usage = none are only worn where script names them, as animations name their props.", hidden)}, notes...)
	}

	footer := float32(0)
	for _, note := range notes {
		footer += imgui.CalcTextSizeV(note, false, imgui.ContentRegionAvail().X).Y + style.ItemSpacing().Y
	}

	a.looksTable(shown, footer)

	for _, note := range notes {
		dimmedWrapped(note)
	}
}

// listedGroups are the groups the window lists, those the game picks from
// for every character, in order of their keys, and how many it leaves out.
func (a *App) listedGroups(l *looks) (shown []*genes.ModifierGroup, hidden int) {
	for _, group := range l.groups {
		if group.Usage == usageGame {
			shown = append(shown, group)
		} else {
			hidden++
		}
	}

	return shown, hidden
}

// countGruesome counts the gruesome groups.
func countGruesome(groups []*genes.ModifierGroup) int {
	count := 0

	for _, group := range groups {
		if group.Gruesome.Or(false) {
			count++
		}
	}

	return count
}

// looksTable lists the groups that pass the filter, each with its element to
// pick and a way back to none, in the height of the window the footer under
// it leaves.
func (a *App) looksTable(groups []*genes.ModifierGroup, footer float32) {
	p := a.portrait
	l := &p.character.looks
	state := &a.state.looks
	search := strings.ToLower(strings.TrimSpace(state.search))

	// Some breathing room between the rows, as the folders of the settings
	// have. Dear ImGui reads the padding row by row, so it stays pushed until
	// the table ends.
	padding := imgui.CurrentStyle().CellPadding()
	imgui.PushStyleVarVec2(imgui.StyleVarCellPadding, imgui.Vec2{X: padding.X, Y: gui.Scaled(3)})
	defer imgui.PopStyleVar()

	// Without outer borders Dear ImGui leaves the outer edges unpadded, which
	// puts the first column flush against the edge while every other one is
	// indented, so the padding is asked for.
	flags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersInnerH | imgui.TableFlagsScrollY | imgui.TableFlagsSizingStretchProp | imgui.TableFlagsPadOuterX
	if !imgui.BeginTableV("##looks", 2, flags, imgui.Vec2{Y: -max(footer, 1)}, 0) {
		return
	}

	imgui.TableSetupColumnV("Portrait modifier", imgui.TableColumnFlagsWidthStretch, 0.45, 0)
	imgui.TableSetupColumnV("Element", imgui.TableColumnFlagsWidthStretch, 0.55, 0)
	imgui.TableSetupScrollFreeze(0, 1)
	imgui.TableHeadersRow()

	tried, triedGroup := "", ""

	for _, group := range groups {
		_, isPicked := l.picked[group.Key]

		switch {
		case state.filter == lookPicked && !isPicked,
			state.filter == lookFromMods && !a.fromMod(group),
			group.Gruesome.Or(false) && !state.showGruesome,
			search != "" && !matchesGroup(group, search):
			continue
		}

		imgui.PushIDStr(group.Key)
		imgui.TableNextRow()

		imgui.TableNextColumn()
		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted(group.Key)

		if source := a.modName(group); source != "" {
			imgui.SameLine()
			imgui.TextDisabled(source)
		}

		imgui.SetItemTooltip(fmt.Sprintf("%d elements", len(group.Modifiers)))

		imgui.TableNextColumn()

		if name, ok, pointed := a.elementCombo(group, l, comboWidth(isPicked)); ok {
			l.pick(group.Key, name)
			l.stopTrying()
			p.character.redress()
		} else if pointed != "" {
			tried, triedGroup = pointed, group.Key
		}

		if isPicked {
			imgui.SameLine()

			if imgui.Button(gui.IconReset) {
				l.unpick(group.Key)
				p.character.redress()
			}

			imgui.SetItemTooltip("Back to what the game would dress the character in untouched")
		}

		a.strengthRows(group, l)

		imgui.PopID()
	}

	imgui.EndTable()

	// What is pointed at in an open list is tried on, and taken off once the
	// list closes.
	if triedGroup != l.tryingGroup || tried != l.trying {
		l.try(triedGroup, tried)
		p.character.redress()
	}
}

// strengthRows are a row under a group for every change of the element it is
// drawn with that gives a gene a range, rather than one strength, to pick the
// strength of: the accessory it dresses the character in, out of those the
// range can pick, or how strong a morph is within it. The game picks one by
// chance; the middle of the range is drawn until one is picked. A range that
// can pick only one accessory, and a change for another sex, have none.
func (a *App) strengthRows(group *genes.ModifierGroup, l *looks) {
	p := a.portrait
	c := p.character

	element, ok := l.choice(group)
	if !ok {
		return
	}

	for index, change := range element.Genes {
		spread, ranged := change.Range.Get()
		if !ranged || change.Type != "" && change.Type != c.sex.String() {
			continue
		}

		value, picked := l.strength(group.Key, element.Name, index)
		if !picked {
			value = change.Strength()
		}

		imgui.PushIDInt(int32(index))

		switch change.Kind {
		case "accessory":
			template, picks := rangePicks(p.genes, change, c.sex)
			if len(picks) < 2 {
				break
			}

			a.strengthLabel(change.Gene)

			current, _ := template.For(c.sex).Pick(value)
			imgui.SetNextItemWidth(comboWidth(picked))

			if imgui.BeginComboV("##strength", accessoryLabel(current.Accessory), imgui.ComboFlagsHeightLarge) {
				for at, choice := range picks {
					if imgui.SelectableBoolV(fmt.Sprintf("%s##%d", accessoryLabel(choice.Accessory), at), choice.Choice == current, 0, imgui.Vec2{}) {
						l.setStrength(group.Key, element.Name, index, choice.At(0.5))
						c.redress()
					}
				}

				imgui.EndCombo()
			}

			imgui.SetItemTooltip(fmt.Sprintf("The game picks one of these %d by chance", len(picks)))
			a.strengthReset(group.Key, element.Name, index, picked)

		case "morph":
			if spread.Min == spread.Max {
				break
			}

			a.strengthLabel(change.Gene)

			shown := float32(value)
			imgui.SetNextItemWidth(comboWidth(picked))

			if imgui.SliderFloatV("##strength", &shown, float32(min(spread.Min, spread.Max)), float32(max(spread.Min, spread.Max)), "%.2f", imgui.SliderFlagsAlwaysClamp) {
				l.setStrength(group.Key, element.Name, index, float64(shown))
				c.redress()
			}

			imgui.SetItemTooltip(fmt.Sprintf("The game picks a strength from %s to %s by chance", formatNumber(spread.Min), formatNumber(spread.Max)))
			a.strengthReset(group.Key, element.Name, index, picked)
		}

		imgui.PopID()
	}
}

// strengthLabel starts a row of a strength, under the group's own, with the
// gene it is the strength of.
//
// The room the row is indented by holds a line from the group's row above,
// turning towards the gene, as a tree draws a branch, to say that the row
// belongs to the group's.
func (a *App) strengthLabel(gene string) {
	imgui.TableNextRow()
	imgui.TableNextColumn()

	origin := imgui.CursorScreenPos()
	indent := imgui.CurrentStyle().IndentSpacing()
	padding := imgui.CurrentStyle().CellPadding().Y

	branch := origin.X + indent*0.45
	turn := origin.Y + imgui.FrameHeight()/2
	colour := imgui.ColorConvertFloat4ToU32(*imgui.StyleColorVec4(imgui.ColTextDisabled))
	thickness := gui.Scaled(1)

	// The lines are thin rectangles, which keep their thickness at every
	// scale of the interface.
	draw := imgui.WindowDrawList()
	draw.AddRectFilled(imgui.Vec2{X: branch, Y: origin.Y - padding*2}, imgui.Vec2{X: branch + thickness, Y: turn + thickness}, colour)
	draw.AddRectFilled(imgui.Vec2{X: branch, Y: turn}, imgui.Vec2{X: origin.X + indent - gui.Scaled(3), Y: turn + thickness}, colour)

	imgui.Indent()
	imgui.AlignTextToFramePadding()
	imgui.TextDisabled(gene)
	imgui.Unindent()
	imgui.TableNextColumn()
}

// strengthReset ends a row of a strength with a way back to the middle of
// its range, beside its list, where one is picked.
func (a *App) strengthReset(group, element string, change int, picked bool) {
	if !picked {
		return
	}

	imgui.SameLine()

	if imgui.Button(gui.IconReset) {
		a.portrait.character.looks.unsetStrength(group, element, change)
		a.portrait.character.redress()
	}

	imgui.SetItemTooltip("Back to the middle of the range")
}

// rangePicks are the accessories the range a change of an accessory gene
// gives can pick for a sex, and the template they are picked from: of the
// gene's special ones as of the DNA's, such as the shirt and the legwear of
// Victoria 3's clothes. There are none for a change of no range, or of a gene
// or a template the genes do not have.
func rangePicks(all *genes.Genes, change genes.GeneModifier, sex genes.Sex) (*genes.AccessoryTemplate, []genes.Picked) {
	spread, ok := change.Range.Get()
	if !ok {
		return nil, nil
	}

	gene, ok := all.AccessoryGene(change.Gene)
	if !ok {
		return nil, nil
	}

	template, ok := templateNamed(gene, change.Template)
	if !ok {
		return nil, nil
	}

	return template, template.For(sex).Picks(spread)
}

// templateNamed is a template of an accessory gene by its name.
func templateNamed(gene *genes.AccessoryGene, name string) (*genes.AccessoryTemplate, bool) {
	for index := range gene.Templates {
		if gene.Templates[index].Name == name {
			return &gene.Templates[index], true
		}
	}

	return nil, false
}

// accessoryLabel is what a list calls an accessory a template picks: its
// name, or nothing for the empty one the files write for none.
func accessoryLabel(accessory string) string {
	if accessory == "" || accessory == emptyAccessory {
		return "nothing"
	}

	return accessory
}

// comboWidth is how wide a list of the Element column is: all of it, or what
// a reset button beside it leaves.
func comboWidth(reset bool) float32 {
	if !reset {
		return -1
	}

	style := imgui.CurrentStyle()

	return imgui.ContentRegionAvail().X - imgui.CalcTextSize(gui.IconReset).X - style.FramePadding().X*2 - style.ItemSpacing().X
}

// matchesGroup reports whether a group's key, or the name of one of its
// elements, holds a search.
func matchesGroup(group *genes.ModifierGroup, search string) bool {
	if strings.Contains(strings.ToLower(group.Key), search) {
		return true
	}

	for _, modifier := range group.Modifiers {
		if strings.Contains(strings.ToLower(modifier.Name), search) {
			return true
		}
	}

	return false
}

// shownChoice is what a list of a group's elements shows as the element it
// is drawn with, and why where it is not picked.
func shownChoice(group *genes.ModifierGroup, l *looks) (name, why string) {
	picked, touched := l.picked[group.Key]

	switch {
	case picked != "":
		return picked, ""
	case !touched:
		if modifier, ok := alwaysApplied(group); ok {
			return modifier.Name, "always applied"
		}
	}

	return "none", ""
}

// elementCombo is the list of a group's elements to pick one of, with a
// search above it, as the list of a gene's templates has: the clothes of
// Victoria 3 have hundreds. It reports the element picked, when one is, and
// otherwise the one pointed at, to try on.
func (a *App) elementCombo(group *genes.ModifierGroup, l *looks, width float32) (picked string, ok bool, pointed string) {
	name, why := shownChoice(group, l)

	preview := name
	if why != "" {
		preview = fmt.Sprintf("%s (%s)", name, why)
	}

	imgui.SetNextItemWidth(width)

	if !imgui.BeginComboV("##element", preview, imgui.ComboFlagsHeightLarge) {
		return "", false, ""
	}
	defer imgui.EndCombo()

	if imgui.IsWindowAppearing() {
		a.state.looks.listSearch = ""
		imgui.SetKeyboardFocusHere()
	}

	names := make([]string, len(group.Modifiers))
	for index := range group.Modifiers {
		names[index] = group.Modifiers[index].Name
	}

	imgui.SetNextItemWidth(-1)
	entered := imgui.InputTextWithHint("##search", gui.Iconed(gui.IconSearch, "Search"), &a.state.looks.listSearch, imgui.InputTextFlagsEnterReturnsTrue, nil)

	shown := matching(names, a.state.looks.listSearch)
	imgui.TextDisabled(fmt.Sprintf("%d of %d", len(shown), len(names)))

	// Enter takes the first element found.
	if entered && len(shown) > 0 {
		imgui.CloseCurrentPopup()

		return names[shown[0]], true, ""
	}

	if imgui.SelectableBoolV("None##none", why == "" && name == "none", 0, imgui.Vec2{}) {
		return "", true, ""
	}

	imgui.Separator()

	rows := min(len(shown), elementRows)
	appearing := imgui.IsWindowAppearing()

	imgui.BeginChildStrV("##elements", imgui.Vec2{Y: float32(max(rows, 1)) * imgui.TextLineHeightWithSpacing()}, 0, 0)
	defer imgui.EndChild()

	for _, position := range shown {
		element := names[position]

		// The games' files name an element twice now and then, as Europa
		// Universalis 5's neckware does, so each is told apart by its place.
		// The fallback is said to be one, which the game dresses a character
		// in where it can pick none of the others.
		label := element
		if element == group.Fallback {
			label += " (fallback)"
		}

		if imgui.SelectableBoolV(fmt.Sprintf("%s##%d", label, position), element == name, 0, imgui.Vec2{}) {
			picked, ok = element, true
			imgui.CloseCurrentPopup()
		}

		// Pointing at an element, or reaching it with the arrow keys, tries
		// it on.
		if imgui.IsItemHovered() || imgui.IsItemFocused() {
			pointed = element
		}

		if element == name && appearing {
			imgui.SetScrollHereY()
		}
	}

	return picked, ok, pointed
}

// fromMod reports whether a group is defined by a mod rather than by the
// game, the first of the folders.
func (a *App) fromMod(group *genes.ModifierGroup) bool {
	return a.modName(group) != ""
}

// modName is the folder a group is defined in, where it is a mod's rather
// than the game's, and "new" for a group of the project no file defines yet.
func (a *App) modName(group *genes.ModifierGroup) string {
	if len(group.Origins) == 0 {
		return "new"
	}

	origin := group.Origins[0]
	if a.inGame(origin.Path) {
		return ""
	}

	return origin.Source
}

// inGame reports whether a file is in the game's folder, the first of the
// folders, rather than in a mod's.
func (a *App) inGame(path string) bool {
	if len(a.settings.Folders) == 0 {
		return false
	}

	relative, err := filepath.Rel(a.settings.Folders[0].Path, path)

	return err == nil && !strings.HasPrefix(relative, "..") && !filepath.IsAbs(relative)
}
