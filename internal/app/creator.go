package app

import (
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/genes"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// The character panel lists the genes by category: a rail of the categories
// down its left, each with how many of its genes are changed, and the genes of
// the one picked beside it. A search over every gene takes the place of the
// category while it holds anything. Pointing at a gene, or at a category,
// highlights on the portrait what it changes.

// How the mouse turns and moves the camera: radians per pixel dragged, and
// how much closer one notch of the wheel brings it.
const (
	turnPerPixel = 0.01
	zoomPerNotch = 1.15
)

// The sizes of the panel, unscaled: the width of the rail of categories, the
// tallest a palette is drawn, and the margin of what is drawn over the
// portrait.
const (
	railWidth     = 170
	paletteHeight = 170
	overlayMargin = 12
)

// previewPanel shows the portrait, which the mouse turns: dragged with the
// left button to turn it, with the right or the middle button to move it
// along, the wheel to come closer, a double click to frame it again. Over it are what the portrait started from, its
// sex and age, and the views and the highlight.
func (a *App) previewPanel() {
	defer imgui.End()

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{})
	visible := imgui.Begin(panelPreview)
	imgui.PopStyleVar()

	if !visible {
		return
	}

	switch {
	case a.portrait == nil:
		imgui.TextDisabled("The 3D view could not be started")

		return

	case !a.hasFolders():
		emptyState("No game folder configured", "Add the game's folder in Settings (Ctrl+,)")

		return

	case a.portrait.problem != nil:
		emptyState("The portrait could not be loaded", a.portrait.problem.Error())

		return

	case !a.portrait.loaded():
		emptyState("Loading the portrait...")

		return
	}

	p := a.portrait

	available := imgui.ContentRegionAvail()
	p.width, p.height = int32(max(available.X, 1)), int32(max(available.Y, 1))

	// The portrait is hidden until a starting point has been chosen, on the
	// welcome screen or in the picker, so that what is drawn behind them is
	// not taken for a choice. It is drawn at the panel's size all the same,
	// ready for when it is shown.
	if a.hidden() {
		return
	}

	width, height := p.viewer.Size()
	origin := imgui.CursorScreenPos()

	// The picture is drawn at the size the panel had last frame, which is
	// what it is shown at too, so that resizing the panel never stretches it.
	a.window.Backend().DrawImageRenderTexture(p.viewer.Target(), imgui.Vec2{X: float32(width), Y: float32(height)}, true, false)

	// An invisible button over the picture takes the dragging, and lets the
	// controls drawn over it after take the clicks meant for them.
	imgui.SetCursorScreenPos(origin)
	imgui.SetNextItemAllowOverlap()
	imgui.InvisibleButtonV("##turn", imgui.Vec2{X: float32(width), Y: float32(height)},
		imgui.ButtonFlagsMouseButtonLeft|imgui.ButtonFlagsMouseButtonRight|imgui.ButtonFlagsMouseButtonMiddle)

	io := imgui.CurrentIO()
	delta := io.MouseDelta()

	if imgui.IsItemActive() {
		if imgui.IsMouseDragging(imgui.MouseButtonLeft) {
			p.viewer.Rotate(float64(delta.X)*turnPerPixel, float64(delta.Y)*turnPerPixel)
		}

		// The right or the middle button moves the view along, as in the
		// model viewer: what was under the pointer stays under it.
		if imgui.IsMouseDragging(imgui.MouseButtonRight) || imgui.IsMouseDragging(imgui.MouseButtonMiddle) {
			perPixel := p.viewer.UnitsPerPixel()
			p.viewer.Pan(-float64(delta.X)*perPixel, float64(delta.Y)*perPixel)
		}
	}

	if imgui.IsItemHovered() {
		if wheel := io.MouseWheel(); wheel != 0 {
			p.viewer.Zoom(math.Pow(zoomPerNotch, float64(wheel)))
		}

		if imgui.IsMouseDoubleClicked(imgui.MouseButtonLeft) {
			p.frame()
		}
	}

	margin := gui.Scaled(overlayMargin)

	imgui.SetCursorScreenPos(imgui.Vec2{X: origin.X + margin, Y: origin.Y + margin})
	a.portraitBar()

	a.viewBar(origin, imgui.Vec2{X: float32(width), Y: float32(height)}, margin)
	a.stagingBar(origin, imgui.Vec2{X: float32(width), Y: float32(height)}, margin)
}

// portraitBar is drawn over the top of the portrait: what it started from,
// with a way back to it, and its sex and age.
func (a *App) portraitBar() {
	p := a.portrait
	c := p.character

	// The identifier the DNA is saved under, which a character's DNA starts
	// with and an ethnicity or scratch starts without.
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(gui.Iconed(gui.IconDNA, "DNA"))
	imgui.SameLine()
	imgui.SetNextItemWidth(gui.Scaled(200))

	alerted := a.identifierAlerted()
	if alerted {
		gui.PushErrorFrame()
	}

	imgui.InputTextWithHint("##identifier", "dna_identifier", &p.basis.identifier, 0, nil)

	if alerted {
		gui.PopErrorFrame()
	}

	imgui.SetItemTooltip(fmt.Sprintf("The key the DNA is saved under (Ctrl+S). Started from %s.", p.basis.describe()))
	imgui.SameLine()

	if imgui.Button(labelResetAll) {
		c.reset(c.basis)
		p.highlight.forget()
	}

	imgui.SetItemTooltip("Set every gene back to what the portrait started from: " + p.basis.describe())

	imgui.SameLineV(0, gui.Scaled(24))

	for _, sex := range genes.Sexes {
		if toggle(sexLabel(sex), c.sex == sex) && c.sex != sex {
			a.start(p.basis, c.dna(), sex)
		}

		imgui.SameLine()
	}

	imgui.SameLineV(0, gui.Scaled(24))
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(gui.Iconed(gui.IconAge, "Age"))
	imgui.SameLine()
	imgui.SetNextItemWidth(gui.Scaled(180))

	if imgui.SliderIntV("##age", &c.age, 0, maxAge, "%d years", imgui.SliderFlagsAlwaysClamp) {
		c.changed = true
	}

	imgui.SetItemTooltip("The genes' age presets grow a child's features and age a face by it")
}

// viewBar is drawn along the bottom of the portrait: the face and the whole
// figure, and whether pointing at a gene highlights what it changes.
func (a *App) viewBar(origin, size imgui.Vec2, margin float32) {
	p := a.portrait
	style := imgui.CurrentStyle()

	labels := []string{labelFace, labelFigure, labelHighlight}
	width := float32(0)

	for _, label := range labels {
		width += imgui.CalcTextSize(label).X + style.FramePadding().X*2 + style.ItemSpacing().X
	}

	width += a.animationControlsWidth()

	imgui.SetCursorScreenPos(imgui.Vec2{X: origin.X + (size.X-width)/2, Y: origin.Y + size.Y - margin - imgui.FrameHeight()})

	a.viewButtons()

	imgui.SameLine()

	if toggle(labelHighlight, p.highlight.enabled) {
		p.highlight.enabled = !p.highlight.enabled
	}

	imgui.SetItemTooltip("Point at a gene or a category to see what part of the portrait it changes")

	imgui.SameLineV(0, gui.Scaled(gapBeforeAnimation))
	a.animationControls()
}

// toggle is a button drawn in the accent colour while it is on, and reports
// whether it was clicked.
func toggle(label string, on bool) bool {
	if on {
		imgui.PushStyleColorVec4(imgui.ColButton, *imgui.StyleColorVec4(imgui.ColHeaderActive))
	}

	clicked := imgui.Button(label)

	if on {
		imgui.PopStyleColor()
	}

	return clicked
}

// characterPanel lists the genes of the portrait by category, to change.
func (a *App) characterPanel() {
	defer imgui.End()

	if !imgui.Begin(panelCharacter) {
		return
	}

	if a.portrait == nil || !a.portrait.loaded() {
		imgui.TextDisabled("Nothing loaded yet")

		return
	}

	if a.hidden() {
		imgui.TextDisabled("Pick a starting point first")

		return
	}

	p := a.portrait
	changed, counts := p.character.changes()

	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##search", gui.Iconed(gui.IconSearch, fmt.Sprintf("Search all %d genes", p.character.listed())), &a.state.geneSearch, 0, nil)

	if imgui.BeginChildStrV("##categories", imgui.Vec2{X: gui.Scaled(railWidth)}, imgui.ChildFlagsBorders, 0) {
		a.categoryRail(counts)
	}

	imgui.EndChild()
	imgui.SameLine()

	if imgui.BeginChildStrV("##genes", imgui.Vec2{}, imgui.ChildFlagsAlwaysUseWindowPadding, 0) {
		a.geneList(changed)
	}

	imgui.EndChild()
}

// categoryRail lists the categories, each with how many of its genes are
// changed, and highlights what a category changes while it is pointed at.
func (a *App) categoryRail(counts map[int]int) {
	p := a.portrait
	searching := strings.TrimSpace(a.state.geneSearch) != ""

	for index, category := range categories {
		if !p.character.lists(index) {
			continue
		}

		label := gui.Iconed(category.icon, category.name)
		if count := counts[index]; count > 0 {
			label = fmt.Sprintf("%s (%d)", label, count)
		}

		if imgui.SelectableBoolV(label+"###"+category.id, !searching && a.state.category == index, 0, imgui.Vec2{}) {
			a.state.category = index
			a.state.geneSearch = ""
		}

		if imgui.IsItemHovered() {
			if keys := p.character.morphKeys(index); len(keys) > 0 {
				p.highlight.point("category "+category.id, keys)
			}
		}
	}
}

// geneList lists the genes of the category picked, or those the search finds.
func (a *App) geneList(changed map[string]bool) {
	p := a.portrait
	c := p.character
	search := strings.ToLower(strings.TrimSpace(a.state.geneSearch))

	gui.PushStrongFont()

	if search != "" {
		imgui.TextUnformatted("Search Results")
	} else {
		imgui.TextUnformatted(categories[a.state.category].name)
	}

	gui.PopFont()
	imgui.Separator()

	matches := func(name, key string) bool {
		return search == "" || strings.Contains(strings.ToLower(name), search) || strings.Contains(key, search)
	}

	in := func(category int) bool {
		return search != "" || category == a.state.category
	}

	if search == "" && categories[a.state.category].id == categoryColours {
		a.colourList(changed)

		return
	}

	shown := 0

	for _, index := range c.morphOrder() {
		morph := &c.morphs[index]
		if !in(morph.category) || !matches(morph.name, morph.gene.Key) {
			continue
		}

		a.morphRow(morph, changed[morph.gene.Key], search != "")
		shown++
	}

	for index := range c.accessories {
		accessory := &c.accessories[index]
		if accessory.hidden || !in(accessory.category) || !matches(accessory.name, accessory.gene.Key) {
			continue
		}

		a.accessoryRow(accessory, changed[accessory.gene.Key], search != "")
		shown++
	}

	switch {
	case shown == 0 && search != "":
		imgui.TextDisabled("No gene is called that")
	case search == "" && categories[a.state.category].id == categoryOther:
		imgui.Spacing()
		dimmedWrapped("Genes the creator has no name for, such as those of a mod, are listed here under their own key.")
	}
}

// geneHeading is the name of a gene, dimmed for one that draws nothing yet,
// with the category it is in while searching, and its key to the right.
func geneHeading(name, key string, drawn bool, category int, searching bool) {
	if drawn {
		imgui.TextUnformatted(name)
	} else {
		imgui.TextDisabled(name)
		imgui.SetItemTooltip("Changes nothing drawn yet: a texture")
	}

	if searching {
		imgui.SameLine()
		imgui.TextDisabled("in " + categories[category].name)
	}

	if name == key {
		return
	}

	// The key is shown to the right where it fits beside the name, and on a
	// line of its own below it where it does not.
	width := imgui.CalcTextSize(key).X
	imgui.SameLine()

	if room := imgui.ContentRegionAvail().X; room >= width {
		imgui.SetCursorPosX(imgui.CursorPosX() + room - width)
	} else {
		imgui.NewLine()
	}

	imgui.TextDisabled(key)
}

// resetButton is a small button that sets a gene back to what the portrait
// started from, drawn only for a gene that is changed, and reports whether it
// was clicked. An unchanged gene keeps its place empty, so that the sliders
// line up.
func resetButton(changed bool) bool {
	width := imgui.CalcTextSize(resetLabel).X + imgui.CurrentStyle().FramePadding().X*2

	imgui.SameLine()

	if !changed {
		imgui.Dummy(imgui.Vec2{X: width, Y: imgui.FrameHeight()})

		return false
	}

	clicked := imgui.ButtonV(resetLabel, imgui.Vec2{X: width})
	imgui.SetItemTooltip("Set the gene back to what the portrait started from")

	return clicked
}

// resetLabel is the label of the buttons that set a gene back.
var resetLabel = gui.Iconed(gui.IconReset, "Reset")

// sliderWidth is the width a slider takes beside a reset button.
func sliderWidth() float32 {
	style := imgui.CurrentStyle()

	return imgui.ContentRegionAvail().X - imgui.CalcTextSize(resetLabel).X - style.FramePadding().X*2 - style.ItemSpacing().X
}

// morphRow is one morph gene: its name, a choice of its templates where it
// has more than one, and its strength, shown from -100 to 100 around the
// middle a neutral DNA leaves it at.
func (a *App) morphRow(morph *morphGene, changed, searching bool) {
	p := a.portrait
	c := p.character

	imgui.PushIDStr(morph.gene.Key)
	imgui.BeginGroup()

	geneHeading(morph.name, morph.gene.Key, morph.drawn, morph.category, searching)

	if len(morph.gene.Templates) > 1 {
		names := make([]string, len(morph.gene.Templates))
		for position, template := range morph.gene.Templates {
			names[position] = template.Name
		}

		if picked, ok := a.templateCombo(names, morph.template); ok {
			morph.template = picked
			c.changed = true
		}
	}

	imgui.SetNextItemWidth(sliderWidth())

	if imgui.SliderIntV("##value", &morph.value, 0, maxGeneValue, signed(morph.value), imgui.SliderFlagsAlwaysClamp) {
		c.changed = true
	}

	// While the slider is dragged the portrait is shown plain, so that the
	// change can be seen rather than the highlight over it.
	if imgui.IsItemActive() {
		p.highlight.hold()
	}

	if resetButton(changed) {
		c.resetGene(morph.gene.Key)
	}

	imgui.EndGroup()

	if imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenBlockedByActiveItem) {
		p.highlight.point(morph.gene.Key, []string{morph.gene.Key})
	}

	rowGap()
	imgui.PopID()
}

// templateCombo is the list of a gene's templates to pick one of, by the
// names the game's files give them, with a search above it: a gene such as
// the hairstyles has dozens. It reports the position of the template picked,
// when another is.
//
// Only one list is open at a time, so they share the one search, which is
// emptied and takes the keyboard each time a list opens.
func (a *App) templateCombo(names []string, current int) (int, bool) {
	imgui.SetNextItemWidth(-1)

	if !imgui.BeginComboV("##template", names[current], imgui.ComboFlagsHeightLarge) {
		return 0, false
	}
	defer imgui.EndCombo()

	if imgui.IsWindowAppearing() {
		a.state.templateSearch = ""
		imgui.SetKeyboardFocusHere()
	}

	imgui.SetNextItemWidth(-1)
	entered := imgui.InputTextWithHint("##search", gui.Iconed(gui.IconSearch, "Search"), &a.state.templateSearch, imgui.InputTextFlagsEnterReturnsTrue, nil)

	shown := matching(names, a.state.templateSearch)

	// Enter takes the first template found.
	if entered && len(shown) > 0 {
		imgui.CloseCurrentPopup()

		return shown[0], shown[0] != current
	}

	if len(shown) == 0 {
		imgui.TextDisabled("No template matches")

		return 0, false
	}

	// The templates scroll beneath the search, which stays where it is.
	appearing := imgui.IsWindowAppearing()
	rows := min(len(shown), templateRows)

	imgui.BeginChildStrV("##templates", imgui.Vec2{Y: float32(rows) * imgui.TextLineHeightWithSpacing()}, 0, 0)
	defer imgui.EndChild()

	picked, ok := 0, false

	for _, position := range shown {
		if imgui.SelectableBoolV(names[position], position == current, 0, imgui.Vec2{}) && position != current {
			picked, ok = position, true
			imgui.CloseCurrentPopup()
		}

		if position == current && appearing {
			imgui.SetScrollHereY()
		}
	}

	return picked, ok
}

// templateRows is how many templates the list shows before it scrolls.
const templateRows = 16

// matching are the positions of the names that hold a search, whatever their
// case, in order; all of them for an empty search.
func matching(names []string, search string) []int {
	search = strings.ToLower(strings.TrimSpace(search))

	var found []int

	for position, name := range names {
		if search == "" || strings.Contains(strings.ToLower(name), search) {
			found = append(found, position)
		}
	}

	return found
}

// signed shows a gene's strength from -100 to 100 around the middle of its
// values, as the slider's label. It holds no verb of a format, so the slider
// shows it as it is.
func signed(value int32) string {
	shown := int(math.Round((float64(value) - maxGeneValue/2.0) / (maxGeneValue / 2.0) * 100))
	if shown == 0 {
		return "0"
	}

	return fmt.Sprintf("%+d", shown)
}

// accessoryRow is one accessory gene: its template, and the strength that
// picks one of the template's accessories, shown as the accessory picked.
func (a *App) accessoryRow(accessory *accessoryGene, changed, searching bool) {
	c := a.portrait.character

	imgui.PushIDStr(accessory.gene.Key)
	imgui.BeginGroup()

	geneHeading(accessory.name, accessory.gene.Key, true, accessory.category, searching)

	names := make([]string, len(accessory.templates))
	for position, template := range accessory.templates {
		names[position] = template.Name
	}

	if picked, ok := a.templateCombo(names, accessory.template); ok {
		accessory.template = picked
		c.partsChanged = true
	}

	label := "Nothing"
	if picked, ok := c.picked(accessory); ok && picked != emptyAccessory {
		label = picked
	}

	imgui.SetNextItemWidth(sliderWidth())

	if imgui.SliderIntV("##value", &accessory.value, 0, maxGeneValue, strings.ReplaceAll(label, "%", "%%"), imgui.SliderFlagsAlwaysClamp) {
		c.partsChanged = true
	}

	imgui.SetItemTooltip("Picks one of the template's accessories by their weights")

	if resetButton(changed) {
		c.resetGene(accessory.gene.Key)
	}

	imgui.EndGroup()
	rowGap()
	imgui.PopID()
}

// colourList shows the palettes of the colour genes, each a picture to pick
// the colour from, with where the ethnicity the portrait started from picks
// its colours outlined.
func (a *App) colourList(changed map[string]bool) {
	p := a.portrait
	c := p.character

	for _, what := range []string{"skin", "hair", "eye"} {
		colour, ok := c.colour(what)
		if !ok {
			continue
		}

		imgui.PushIDStr(colour.gene.Key)

		geneHeading(colour.name, colour.gene.Key, true, colour.category, false)

		swatch := p.swatches[what]
		if swatch == nil {
			imgui.TextDisabled("Its palette could not be read")
			imgui.PopID()

			continue
		}

		left, width := a.palettePicker(colour, swatch)

		// Where in the palette the colour is, to type in, as wide as the
		// palette and beneath it, and the way back to the basis's beside it.
		imgui.SetCursorPosX(left)
		imgui.SetNextItemWidth(width)

		place := colour.value
		if imgui.InputInt2("##place", &place) {
			for axis := range place {
				place[axis] = min(max(place[axis], 0), maxGeneValue)
			}

			if place != colour.value {
				colour.value = place
				c.coloursChanged = true
			}
		}

		imgui.SetItemTooltip("Where the colour is in the palette, across and down, from 0 to 255")

		if changed[colour.gene.Key] {
			imgui.SameLine()

			if imgui.Button(resetLabel) {
				c.resetGene(colour.gene.Key)
			}

			imgui.SetItemTooltip("Set the colour back to what the portrait started from")
		}

		rowGap()
		imgui.PopID()
	}
}

// palettePicker draws a palette to pick a colour gene's place in by clicking
// or dragging, with the place marked and the rectangles of the ethnicity
// outlined. A palette held to paletteHeight is narrower than the panel, and
// stands in the middle of it; it returns where the palette's left edge is,
// as a cursor position, and its width, for what is drawn beneath it.
func (a *App) palettePicker(colour *colourGene, swatch *image.RGBA) (left, width float32) {
	p := a.portrait

	bounds := swatch.Bounds()
	available := imgui.ContentRegionAvail().X
	width = available
	height := width * float32(bounds.Dy()) / float32(bounds.Dx())

	if limit := gui.Scaled(paletteHeight); height > limit {
		width, height = limit*float32(bounds.Dx())/float32(bounds.Dy()), limit
	}

	left = imgui.CursorPosX() + (available-width)/2
	imgui.SetCursorPosX(left)

	size := imgui.Vec2{X: width, Y: height}
	origin := imgui.CursorScreenPos()

	imgui.ImageV(a.swatchTexture(swatch), size, imgui.Vec2{}, imgui.Vec2{X: 1, Y: 1})
	imgui.SetCursorScreenPos(origin)
	imgui.InvisibleButtonV("##palette", size, imgui.ButtonFlagsMouseButtonLeft)

	if imgui.IsItemActive() {
		mouse := imgui.MousePos()
		u := min(max((mouse.X-origin.X)/width, 0), 1)
		v := min(max((mouse.Y-origin.Y)/height, 0), 1)

		next := [2]int32{int32(math.Round(float64(u) * maxGeneValue)), int32(math.Round(float64(v) * maxGeneValue))}
		if next != colour.value {
			colour.value = next
			p.character.coloursChanged = true
		}
	}

	imgui.SetItemTooltip("Click or drag to pick the colour")

	draw := imgui.WindowDrawList()
	at := func(u, v float64) imgui.Vec2 {
		return imgui.Vec2{X: origin.X + float32(u)*width, Y: origin.Y + float32(v)*height}
	}

	// The ethnicity picks its colours from these rectangles, each by its
	// weight.
	if p.basis.ethnicity != nil {
		if colours, ok := p.basis.ethnicity.Color(colour.gene.Key); ok {
			outline := imgui.ColorConvertFloat4ToU32(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.55})

			for _, choice := range colours.Choices {
				draw.AddRectV(at(choice.From[0], choice.From[1]), at(choice.To[0], choice.To[1]), outline, 0, gui.Scaled(1), 0)
			}
		}
	}

	marker := at(float64(colour.value[0])/maxGeneValue, float64(colour.value[1])/maxGeneValue)
	draw.AddCircleV(marker, gui.Scaled(6), imgui.ColorConvertFloat4ToU32(imgui.Vec4{W: 1}), 0, gui.Scaled(3))
	draw.AddCircleV(marker, gui.Scaled(6), imgui.ColorConvertFloat4ToU32(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1}), 0, gui.Scaled(1.5))

	return left, width
}

// swatchTexture is a palette uploaded for the interface to draw, uploaded the
// first time it is drawn. The palettes of a load that was replaced are
// released.
func (a *App) swatchTexture(swatch *image.RGBA) imgui.TextureRef {
	if reference, ok := a.swatchTextures[swatch]; ok {
		return reference
	}

	for kept, reference := range a.swatchTextures {
		current := false

		for _, shown := range a.portrait.swatches {
			current = current || shown == kept
		}

		if !current {
			a.window.Backend().DeleteTexture(reference)
			delete(a.swatchTextures, kept)
		}
	}

	if a.swatchTextures == nil {
		a.swatchTextures = map[*image.RGBA]imgui.TextureRef{}
	}

	bounds := swatch.Bounds()
	reference := a.window.Backend().CreateTextureRgba(swatch, bounds.Dx(), bounds.Dy())
	a.swatchTextures[swatch] = reference

	return reference
}

// emptyState draws dimmed lines in the middle of a panel, saying why it has
// nothing in it.
func emptyState(lines ...string) {
	available := imgui.ContentRegionAvail()
	lineHeight := imgui.TextLineHeightWithSpacing()

	imgui.Dummy(imgui.Vec2{Y: max((available.Y-lineHeight*float32(len(lines)))/2, 0)})

	for _, line := range lines {
		width := imgui.CalcTextSize(line).X
		imgui.SetCursorPosX(imgui.CursorPosX() + max((available.X-width)/2, 0))
		imgui.TextDisabled(line)
	}
}

// rowGap is the space between one gene and the next, more than between the
// lines of one, so that each reads as a group.
func rowGap() {
	imgui.Dummy(gui.ScaledVec2(0, 10))
}

// hidden reports whether the portrait and its genes are hidden, which they
// are until a starting point has been chosen: while the welcome screen or
// the picker is open.
func (a *App) hidden() bool {
	return a.state.welcome || a.state.picker.open
}

// viewButtons switch the camera between the face and the whole figure, which
// is kept for the portraits drawn after.
func (a *App) viewButtons() {
	p := a.portrait

	free := p.staging.camera == ""

	// Framing the face or the figure lets go of the camera picked.
	if toggle(labelFace, free && !p.figure) {
		p.figure, p.staging.camera = false, ""
		p.frame()
	}

	imgui.SameLine()

	if toggle(labelFigure, free && p.figure) {
		p.figure, p.staging.camera = true, ""
		p.frame()
	}
}

// sexLabel is what a sex is picked by: its icon and its name. The game knows
// four, a man, a woman, a boy and a girl, whose genes can be set apart.
func sexLabel(sex genes.Sex) string {
	icons := map[genes.Sex]string{genes.Male: gui.IconMale, genes.Female: gui.IconFemale, genes.Boy: gui.IconBoy, genes.Girl: gui.IconGirl}

	return gui.Iconed(icons[sex], friendly(sex.String()))
}

// The labels of the buttons over the portrait and in the picker, with their
// icons.
var (
	labelFace      = gui.Iconed(gui.IconFace, "Face")
	labelFigure    = gui.Iconed(gui.IconBody, "Whole Figure")
	labelHighlight = gui.Iconed(gui.IconHighlight, "Highlight Areas")
	labelResetAll  = gui.Iconed(gui.IconReset, "Reset All")
)

// The animation controls beside the views: how far apart from them they
// stand, and how wide the list of animations is, unscaled.
const (
	gapBeforeAnimation = 24
	animationListWidth = 260
)

// noAnimation is what the list of animations calls playing none.
const noAnimation = "none"

// animationControlsWidth is how wide the animation controls are, for the bar
// to be centred with them.
func (a *App) animationControlsWidth() float32 {
	style := imgui.CurrentStyle()

	return gui.Scaled(gapBeforeAnimation) + imgui.CalcTextSize(gui.IconAnimation).X + style.ItemSpacing().X +
		gui.Scaled(animationListWidth) + style.ItemSpacing().X + imgui.FrameHeight() + style.FramePadding().X*2
}

// animationControls pick the animation the portrait plays, among the sets of
// the folders and their variants for its sex, and stop and start its clock.
func (a *App) animationControls() {
	p := a.portrait
	playing := &p.animation

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(gui.IconAnimation)
	imgui.SetItemTooltip("The animation the portrait plays")
	imgui.SameLine()

	current := noAnimation
	if !playing.none {
		current = animationChoice{set: playing.chosen, variant: playing.variant}.label()
	}

	imgui.SetNextItemWidth(gui.Scaled(animationListWidth))

	if imgui.BeginCombo("##animation", current) {
		if imgui.SelectableBoolV(noAnimation, playing.none, 0, imgui.Vec2{}) && !playing.none {
			playing.none = true
			playing.restart(p.sex)
		}

		imgui.SetItemTooltip("The pose the meshes are stored in")

		for _, choice := range playing.choices(p.sex) {
			selected := !playing.none && choice.set == playing.chosen && choice.variant == playing.variant

			label := choice.label()
			if choice.variant != "" {
				label = "    " + choice.variant
			}

			if imgui.SelectableBoolV(label+"###"+choice.label(), selected, 0, imgui.Vec2{}) && !selected {
				playing.none, playing.chosen, playing.variant = false, choice.set, choice.variant
				playing.restart(p.sex)
			}
		}

		imgui.EndCombo()
	}

	imgui.SameLine()

	icon, tip := gui.IconPause, "Pause the animation"
	if playing.paused {
		icon, tip = gui.IconPlay, "Play the animation"
	}

	imgui.BeginDisabledV(playing.none)

	if imgui.Button(icon) {
		playing.paused = !playing.paused
	}

	imgui.EndDisabled()
	imgui.SetItemTooltip(tip)
}

// The staging bar over the view bar: how wide each of its lists is, unscaled.
const stagingListWidth = 280

// What the staging lists call the renderer's own lights and a camera framed
// freely.
const (
	noEnvironment = "studio"
	noCamera      = "free"
)

// stagingBar picks the environment that lights the portrait and the camera
// that looks at it, among the game's own, over the views along the bottom.
func (a *App) stagingBar(origin, size imgui.Vec2, margin float32) {
	p := a.portrait
	style := imgui.CurrentStyle()

	listWidth := gui.Scaled(stagingListWidth)
	iconWidth := imgui.CalcTextSize(gui.IconLight).X + style.ItemSpacing().X
	width := 2*(iconWidth+listWidth) + gui.Scaled(gapBeforeAnimation)

	imgui.SetCursorScreenPos(imgui.Vec2{
		X: origin.X + (size.X-width)/2,
		Y: origin.Y + size.Y - margin - imgui.FrameHeight()*2 - style.ItemSpacing().Y,
	})

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(gui.IconLight)
	imgui.SetItemTooltip("The environment of gfx/portraits/environments that lights the portrait")
	imgui.SameLine()
	imgui.SetNextItemWidth(listWidth)

	if picked, ok := pickOne("##environment", p.staging.environment, noEnvironment, p.staging.environmentKeys()); ok {
		p.staging.environment, p.staging.chosen = picked, true
	}

	imgui.SameLineV(0, gui.Scaled(gapBeforeAnimation))
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(gui.IconCamera)
	imgui.SetItemTooltip("The camera of gfx/portraits/cameras that looks at the portrait")
	imgui.SameLine()
	imgui.SetNextItemWidth(listWidth)

	if picked, ok := pickOne("##camera", p.staging.camera, noCamera, p.staging.cameraKeys()); ok {
		p.staging.camera = picked
		p.frame()
	}
}

// pickOne is a list of keys to pick one of, or none, which it shows as a name
// of its own. It reports the key picked, empty for none, when one is.
func pickOne(id, current, none string, keys []string) (string, bool) {
	shown := current
	if shown == "" {
		shown = none
	}

	picked, changed := "", false

	if imgui.BeginCombo(id, shown) {
		if imgui.SelectableBoolV(none, current == "", 0, imgui.Vec2{}) && current != "" {
			picked, changed = "", true
		}

		for _, key := range keys {
			if imgui.SelectableBoolV(key, key == current, 0, imgui.Vec2{}) && key != current {
				picked, changed = key, true
			}
		}

		imgui.EndCombo()
	}

	return picked, changed
}
