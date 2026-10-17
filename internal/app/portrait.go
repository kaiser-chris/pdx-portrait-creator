package app

import (
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-asset-go/entity"
	"github.com/kaiser-chris/pdx-asset-go/model"
	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-asset-go/texture"
	"github.com/kaiser-chris/pdx-parser-go/asset"
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/config"
)

// The entities the portrait panel shows: the body and the head Victoria 3's
// portrait settings give a man, and the joints the head is attached to the
// body by, which gfx/portraits/portraitsettings.txt names. For now they are
// fixed.
const (
	bodyEntity = "male_body_entity"
	headEntity = "male_head_entity"

	bodyJoint = "bn_sp_thoracic"
	headJoint = "head_root"

	// faceJoint is the bone of the head the face moves with, which the
	// camera follows.
	faceJoint = "bn_h_head"
)

// headLinks are all the joints the head is attached to the body by, which it
// follows wherever the body's additive animations take them.
var headLinks = []render.JointLink{
	{Parent: bodyJoint, Child: headJoint},
	{Parent: "bn_l_clavicle", Child: "bn_h_l_clavicle"},
	{Parent: "bn_r_clavicle", Child: "bn_h_r_clavicle"},
}

// headParts are the accessory genes whose accessories are parts of the head
// rather than something worn: the eyes, the eyelashes and the teeth. They are
// drawn with the head, posed as it is.
var headParts = []string{"eye_accessory", "eye_lashes_accessory", "teeth_accessory"}

// previewNames name the ways an ethnicity's DNA is previewed, in the
// order of genes.Preview, and previewHelp says what each does.
var (
	previewNames = []string{"Neutral", "Likeliest"}
	previewHelp  = []string{
		"Every gene at its first template and the middle of its strength, as the game's portrait editor shows an ethnicity",
		"Every gene the ethnicity lists at its likeliest choice and the middle of its range",
	}
)

// skinPalette is the palette the colour of a portrait's skin is picked from,
// and skinColorGene the colour gene that picks it.
const (
	skinPalette   = "gfx/portraits/skin_palette.dds"
	skinColorGene = "skin_color"
)

// portraitEthnicity is the ethnicity of common/ethnicities whose DNA the
// portrait starts from, the one every other ethnicity of Victoria 3 builds
// on. For now it is fixed.
const portraitEthnicity = "ethnicity_template"

// How the mouse turns and moves the camera: radians per pixel dragged, and
// how much closer one notch of the wheel brings it.
const (
	turnPerPixel = 0.01
	zoomPerNotch = 1.15
)

// portrait is what the portrait panel shows: the models on the GPU, the
// viewer that draws them, the genes that morph them, and the loading that
// produces all of it.
type portrait struct {
	renderer *render.Renderer
	viewer   *render.Viewer

	// body and head are the models drawn, and nil until a load has
	// finished. framing is set when a load has finished and the camera has
	// yet to frame the head, once it has been morphed and posed.
	body, head *render.Model
	framing    bool

	// genes are the genes of the folders, ethnicity the ethnicity the
	// portrait started from, flattened, nil when the folders have no such
	// ethnicity, preview how its DNA is made, and morphs the morph genes as
	// the character panel sets them.
	genes     *genes.Genes
	ethnicity *genes.Ethnicity
	preview   genes.Preview

	// skinPalette is the palette the colour of the skin is picked from, nil
	// when the folders have none, which draws it in a stand in tone.
	skinPalette *texture.Image
	morphs      morphs

	// decalTextures are the textures the decals of the genes name, by their
	// paths, and decalProblem why the decals could last not be laid, so that
	// it is reported once.
	decalTextures map[string]*texture.Image
	decalProblem  string

	// loading is set while a load runs, and generation counts the loads
	// started, so that a load the folders have changed under since is
	// dropped rather than shown.
	loading    bool
	generation int
	results    chan portraitResult

	// problem is why the portrait could not be loaded or uploaded, and
	// diagnostics what was reported while loading it.
	problem     error
	diagnostics report.Diagnostics

	// width and height are the size the panel had to show the picture in,
	// which the next frame draws it at.
	width, height int32
}

// portraitResult is what a load comes back with.
type portraitResult struct {
	generation  int
	body, head  *model.Model
	genes       *genes.Genes
	ethnicity   *genes.Ethnicity
	skinPalette *texture.Image
	decals      map[string]*texture.Image
	diagnostics report.Diagnostics
	err         error
}

// newPortrait prepares the renderer and the viewer. It needs the OpenGL
// context.
func newPortrait() (*portrait, error) {
	renderer, err := render.NewRenderer()
	if err != nil {
		return nil, err
	}

	viewer := renderer.NewViewer(1, 1)

	return &portrait{renderer: renderer, viewer: viewer, results: make(chan portraitResult, 1)}, nil
}

// load starts reading the portrait from the configured folders on a
// goroutine of its own: reading the asset files and the genes and decoding
// the textures takes long enough that the window would stop answering
// meanwhile.
func (p *portrait) load(configured []config.Folder) {
	p.generation++
	p.loading = true
	p.problem = nil

	generation := p.generation

	sources := make([]folders.Source, 0, len(configured))
	for _, folder := range configured {
		sources = append(sources, folders.Source{Name: folder.Name, Path: folder.Path})
	}

	go func() {
		result := readPortrait(folders.Open(sources))
		result.generation = generation

		// A result nobody waits for any more is dropped rather than left
		// blocking the goroutine.
		select {
		case p.results <- result:
		default:
			select {
			case <-p.results:
			default:
			}
			p.results <- result
		}
	}()
}

// readPortrait reads the body and the head, seats the head on the body, reads
// the genes and the ethnicity, and adds the eyes, the eyelashes and the teeth
// its neutral DNA picks to the head. It is plain Go, which runs off the thread
// that draws.
func readPortrait(set *folders.Set) portraitResult {
	result := portraitResult{}
	loader := entity.NewLoader(set, asset.Load(set))

	body, diagnostics, err := loader.Load(bodyEntity)
	if err != nil {
		return portraitResult{err: err}
	}

	result.diagnostics = append(result.diagnostics, diagnostics...)

	head, diagnostics, err := loader.Load(headEntity)
	if err != nil {
		return portraitResult{err: err}
	}

	result.diagnostics = append(result.diagnostics, diagnostics...)

	// A head that cannot be seated is still drawn, where its mesh puts it.
	if err := seat(body, head); err != nil {
		result.diagnostics = append(result.diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "entity " + headEntity, Message: err.Error()})
	}

	result.body, result.head = body, head

	// What the genes files hold that is worth knowing is a warning or worse;
	// the rest is about fields the parser keeps without reading.
	result.genes = genes.Load(set)
	result.diagnostics = append(result.diagnostics, result.genes.Diagnostics.Filter(report.SeverityWarning)...)

	ethnicities, diagnostics := genes.LoadEthnicities(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	if ethnicity, err := genes.Flatten(ethnicities, portraitEthnicity); err == nil {
		result.ethnicity = ethnicity
	} else {
		result.diagnostics = append(result.diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "ethnicity " + portraitEthnicity, Message: err.Error() + "; the portrait is neutral"})
	}

	// The palette the colour of the skin is picked from by the two numbers a
	// DNA gives it.
	palette, err := readTexture(set, skinPalette)
	if err != nil {
		result.diagnostics = append(result.diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: skinPalette, Message: err.Error() + "; the skin is drawn in a stand in tone"})
	}

	result.skinPalette = palette

	// The textures of the decals the genes lay over the skin.
	decals, diagnostics := readDecalTextures(set, result.genes)
	result.diagnostics = append(result.diagnostics, diagnostics...)
	result.decals = decals

	// The parts of the head are the same however the DNA is previewed:
	// every ethnicity of the game lists none of them.
	neutral := result.genes.Preview(result.ethnicity, genes.PreviewNeutral)

	accessories, diagnostics := genes.LoadAccessories(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	for _, gene := range headParts {
		name, err := headPart(result.genes, accessories, neutral, gene)
		if err == nil {
			var built *model.Model

			built, diagnostics, err = loader.Load(name)
			result.diagnostics = append(result.diagnostics, diagnostics...)

			if err == nil && !head.SharePose(built) {
				err = errors.New("the head has no skeleton to pose it with")
			}
		}

		if err != nil {
			result.diagnostics = append(result.diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "gene " + gene, Message: err.Error() + "; drawn without it"})
		}
	}

	return result
}

// headPart finds the entity of a part of the head that an accessory gene
// draws: the template a DNA picks, or the gene's first, the accessory its
// strength picks for a man, and the entity of that accessory posed with the
// head.
func headPart(all *genes.Genes, accessories database.Table[genes.Accessory], dna *genes.DNA, name string) (string, error) {
	gene, ok := all.AccessoryGenes.Get(name)
	if !ok || len(gene.Templates) == 0 {
		return "", fmt.Errorf("there is no accessory gene %s", name)
	}

	template, strength := &gene.Templates[0], 0.0

	if dna != nil {
		if value, ok := dna.Gene(name); ok {
			if chosen, ok := gene.Template(value.Dominant.Template); ok {
				template, strength = chosen, value.Dominant.Strength()
			}
		}
	}

	choice, ok := template.For(portraitSex).Pick(strength)
	if !ok {
		return "", fmt.Errorf("template %s picks no accessory for a man", template.Name)
	}

	accessory, ok := accessories.Get(choice.Accessory)
	if !ok {
		return "", fmt.Errorf("there is no portrait accessory %s", choice.Accessory)
	}

	chosen, ok := accessory.Choose(nil)
	if !ok || chosen.Entity == "" || chosen.SharedPoseEntity != "head" {
		return "", fmt.Errorf("portrait accessory %s draws no entity posed with the head", choice.Accessory)
	}

	return chosen.Entity, nil
}

// seat places the head where the game attaches it to the body: with its
// joint on the body's, in the pose their meshes are stored in.
func seat(body, head *model.Model) error {
	thoracic, ok := body.Joint(bodyJoint)
	if !ok {
		return fmt.Errorf("%s has no joint %s to attach the head to; the head is drawn where its mesh puts it", bodyEntity, bodyJoint)
	}

	root, ok := head.Joint(headJoint)
	if !ok {
		return fmt.Errorf("%s has no joint %s to attach it by; it is drawn where its mesh puts it", headEntity, headJoint)
	}

	undo, ok := root.Inverse()
	if !ok {
		return errors.New("the joint the head is attached by cannot be undone; it is drawn where its mesh puts it")
	}

	head.Place(undo.Then(thoracic))

	return nil
}

// poll picks up a finished load and uploads its models. It reports whether a
// load finished, and runs on the goroutine that owns the OpenGL context.
func (p *portrait) poll() bool {
	select {
	case result := <-p.results:
		if result.generation != p.generation {
			return false
		}

		p.loading = false
		p.diagnostics = result.diagnostics
		p.problem = result.err

		if result.err != nil {
			p.unloadModels()

			return true
		}

		body, err := p.renderer.Upload(result.body)
		if err != nil {
			p.problem = err

			return true
		}

		head, err := p.renderer.Upload(result.head)
		if err != nil {
			body.Unload()
			p.problem = err

			return true
		}

		p.unloadModels()
		p.body, p.head = body, head

		// The head follows the joints of the body it is attached by, which
		// the body's additive animations move.
		head.Follow(body, headLinks...)

		p.genes, p.ethnicity = result.genes, result.ethnicity
		p.skinPalette = result.skinPalette
		p.decalTextures, p.decalProblem = result.decals, ""
		p.morphs = newMorphs(result.genes, nil, append(body.Attributes(), head.Attributes()...))
		p.apply(result.genes.Preview(result.ethnicity, p.preview))

		p.framing = true

		return true

	default:
		return false
	}
}

// apply sets the portrait as a DNA has it: its morph genes, and the colour of
// its skin, picked from the skin palette by the dominant allele of its
// skin_color.
func (p *portrait) apply(dna *genes.DNA) {
	p.morphs.reset(dna)
	p.viewer.Look.PaletteColor = render.DefaultLook.PaletteColor

	value, ok := dna.Gene(skinColorGene)
	if !ok {
		return
	}

	if colour, ok := p.skinPalette.Sample(float64(value.Dominant.Value)/maxGeneValue, float64(value.Dominant.Second)/maxGeneValue); ok {
		p.viewer.Look.PaletteColor = render.PaletteColour(colour)
	}
}

// readTexture reads a texture of the folders by its path below their root.
func readTexture(set *folders.Set, relative string) (*texture.Image, error) {
	found, ok := set.Find(relative)
	if !ok {
		return nil, fmt.Errorf("%s is in none of the folders", relative)
	}

	data, err := os.ReadFile(found.Path)
	if err != nil {
		return nil, err
	}

	return texture.Decode(found.Path, data)
}

// face is the box the head fits in where it is now. The box of a model is
// that of its meshes as they rest, and the face goes where the body it
// follows and its own animations take it, which is as far as the bone of the
// head has moved.
func (p *portrait) face() [2][3]float32 {
	box := [2][3]float32{p.head.Min, p.head.Max}

	rest, posed, ok := p.head.JointPosition(faceJoint)
	if !ok {
		return box
	}

	for axis := range 3 {
		box[0][axis] += posed[axis] - rest[axis]
		box[1][axis] += posed[axis] - rest[axis]
	}

	return box
}

// frameFace points the camera at the head from the front, which is where the
// morph genes show.
func (p *portrait) frameFace() {
	face := p.face()
	p.viewer.Frame(face[0], face[1])
	p.viewer.Reset()
}

// frameFigure points the camera at the whole figure from the front: the body
// and the head where it is now.
func (p *portrait) frameFigure() {
	face := p.face()

	for axis := range 3 {
		face[0][axis] = min(face[0][axis], p.body.Min[axis])
		face[1][axis] = max(face[1][axis], p.body.Max[axis])
	}

	p.viewer.Frame(face[0], face[1])
	p.viewer.Reset()
}

// loaded reports whether there is a portrait to show.
func (p *portrait) loaded() bool {
	return p.body != nil && p.head != nil
}

// draw morphs the models by the genes if they changed, and draws them into
// the viewer's picture, at the size the panel last had for it. It runs
// before the interface, while raylib drawing is active.
func (p *portrait) draw() {
	if p.width > 0 && p.height > 0 {
		p.viewer.Resize(p.width, p.height)
	}

	if !p.loaded() {
		p.viewer.Draw()

		return
	}

	if p.morphs.changed {
		attributes := p.morphs.attributes()
		p.body.Morph(attributes)
		p.head.Morph(attributes)
		p.layDecals()
		p.morphs.changed = false
	}

	if p.framing {
		p.frameFace()
		p.framing = false
	}

	p.viewer.Draw(p.body, p.head)
}

// layDecals lays the decals the genes lay at the portrait's age over the
// skin of the body and the head, in the skin colour of the look. A decal
// that cannot be laid is reported once, and the portrait drawn without it.
func (p *portrait) layDecals() {
	decals := p.morphs.decals(p.decalTextures)

	for _, uploaded := range []*render.Model{p.body, p.head} {
		err := uploaded.LayDecals(decals, p.viewer.Look)
		if err == nil || err.Error() == p.decalProblem {
			continue
		}

		p.decalProblem = err.Error()
		warn(err)
	}
}

func (p *portrait) unloadModels() {
	for _, uploaded := range []*render.Model{p.body, p.head} {
		if uploaded != nil {
			uploaded.Unload()
		}
	}

	p.body, p.head = nil, nil
	p.genes, p.ethnicity, p.morphs = nil, nil, morphs{}
}

// unload releases everything on the GPU. It needs the OpenGL context.
func (p *portrait) unload() {
	p.unloadModels()
	p.viewer.Unload()
	p.renderer.Unload()
}

// startPortrait loads the portrait from the folders in the settings, or says
// what is missing.
func (a *App) startPortrait() {
	if a.portrait == nil {
		return
	}

	if !a.hasFolders() {
		a.portrait.unloadModels()
		a.portrait.problem = nil

		return
	}

	a.portrait.load(a.settings.Folders)
	a.setStatus("Loading the portrait")
}

// pollPortrait picks up a finished load and reports it in the status bar.
func (a *App) pollPortrait() {
	if a.portrait == nil || !a.portrait.poll() {
		return
	}

	switch {
	case a.portrait.problem != nil:
		warn(a.portrait.problem)
		a.setStatus("The portrait could not be loaded")
	case len(a.portrait.diagnostics) > 0:
		a.setStatus("Loaded the portrait, with %d problems", len(a.portrait.diagnostics))
	default:
		a.setStatus("Loaded the portrait")
	}
}

// hasFolders reports whether any game or mod folder is configured.
func (a *App) hasFolders() bool {
	return len(a.settings.Folders) > 0
}

// previewPanel shows the portrait, which the mouse turns: dragged with the
// left button to turn it, the wheel to come closer, a double click to look at
// it from the front again.
func (a *App) previewPanel() {
	defer imgui.End()

	if !imgui.Begin(panelPreview) {
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

	available := imgui.ContentRegionAvail()
	a.portrait.width, a.portrait.height = int32(max(available.X, 1)), int32(max(available.Y, 1))

	width, height := a.portrait.viewer.Size()

	// The picture is drawn at the size the panel had last frame, which is
	// what it is shown at too, so that resizing the panel never stretches it.
	a.window.Backend().DrawImageRenderTexture(a.portrait.viewer.Target(), imgui.Vec2{X: float32(width), Y: float32(height)}, true, false)

	if !imgui.IsItemHovered() {
		return
	}

	io := imgui.CurrentIO()

	if imgui.IsMouseDragging(imgui.MouseButtonLeft) {
		delta := io.MouseDelta()
		a.portrait.viewer.Rotate(float64(delta.X)*turnPerPixel, float64(delta.Y)*turnPerPixel)
	}

	if wheel := io.MouseWheel(); wheel != 0 {
		a.portrait.viewer.Zoom(math.Pow(zoomPerNotch, float64(wheel)))
	}

	if imgui.IsMouseDoubleClicked(imgui.MouseButtonLeft) {
		a.portrait.viewer.Reset()
	}

	imgui.SetTooltip("Drag to turn, scroll to zoom, double click to reset")
}

// characterPanel shows whose genes the portrait has, lets the morph genes be
// changed, and says what went wrong loading it.
func (a *App) characterPanel() {
	defer imgui.End()

	if !imgui.Begin(panelCharacter) {
		return
	}

	if a.portrait == nil || !a.portrait.loaded() {
		imgui.TextDisabled("Nothing loaded yet")

		return
	}

	p := a.portrait

	sectionHeader("Ethnicity")

	if p.ethnicity != nil {
		imgui.TextUnformatted(p.ethnicity.Key)
	} else {
		imgui.TextDisabled(portraitEthnicity + " is not among the game's ethnicities")
	}

	labelWidth := func(label string) float32 {
		return -imgui.CalcTextSize(label).X - imgui.CurrentStyle().ItemInnerSpacing().X
	}

	imgui.SetNextItemWidth(labelWidth("Preview"))

	if imgui.BeginCombo("Preview", previewNames[p.preview]) {
		for index, name := range previewNames {
			preview := genes.Preview(index)

			if imgui.SelectableBoolV(name, preview == p.preview, 0, imgui.Vec2{}) {
				p.preview = preview
				p.apply(p.genes.Preview(p.ethnicity, preview))
			}

			imgui.SetItemTooltip(previewHelp[index])
		}

		imgui.EndCombo()
	}

	if imgui.Button("Reset to Ethnicity") {
		p.apply(p.genes.Preview(p.ethnicity, p.preview))
	}

	imgui.SetNextItemWidth(labelWidth("Age"))

	if imgui.SliderIntV("Age", &p.morphs.age, 0, maxAge, "%d%%", imgui.SliderFlagsAlwaysClamp) {
		p.morphs.changed = true
	}

	sectionHeader("View")

	if imgui.Button("Face") {
		p.frameFace()
	}

	imgui.SameLine()

	if imgui.Button("Whole Figure") {
		p.frameFigure()
	}

	imgui.SameLine()
	imgui.TextDisabled(fmt.Sprintf("turned %.0f°, tilted %.0f°", p.viewer.Yaw*180/math.Pi, p.viewer.Pitch*180/math.Pi))

	if len(p.diagnostics) > 0 && imgui.CollapsingHeaderTreeNodeFlags(fmt.Sprintf("Problems (%d)###problems", len(p.diagnostics))) {
		for _, diagnostic := range p.diagnostics {
			dimmedWrapped(diagnostic.String())
		}
	}

	sectionHeader("Morph Genes")
	p.morphs.panel()
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
