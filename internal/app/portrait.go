package app

import (
	"errors"
	"fmt"
	"image"
	"os"
	"slices"

	"github.com/kaiser-chris/pdx-asset-go/entity"
	"github.com/kaiser-chris/pdx-asset-go/model"
	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-asset-go/texture"
	"github.com/kaiser-chris/pdx-parser-go/asset"
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	staged "github.com/kaiser-chris/pdx-parser-go/portrait"
	"github.com/kaiser-chris/pdx-parser-go/report"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/config"
)

// faceJoints are the bones of the head the face moves with, which the
// camera follows: Victoria 3's and Europa Universalis 5's, whose cameras look
// at them. No file says which a head has, so the first it has is taken.
var faceJoints = []string{"bn_h_head", "Head_B"}

// seamTolerance is how near a vertex of the head has to lie to one of the
// body to be taken for the same, in the units of the meshes, centimetres;
// the rings where they meet lie at the same places to a hundredth of that.
const seamTolerance = 0.01

// weldTolerance is how far the ring of the head may lie from the body's for
// its vertices to be welded to the body's nearest, in centimetres: Europa
// Universalis 5's rings lie up to half of one apart, a woman's head from her
// body and a man's head from the body of a boy, whose vertices are a little
// more than one apart, which this stays short of half of.
const weldTolerance = 0.5

// headPose is the entity the parts of the head are posed with, as the
// portrait accessories name it: the eyes, the teeth, the hair and the beard.
const headPose = "head"

// emptyAccessory is what the genes name for an accessory that draws nothing,
// such as the hairstyle of no hair, which no portrait accessory defines.
const emptyAccessory = "empty"

// palettePaths are the palettes the colours of a portrait are picked from, by
// what each colours, as the colour genes name it.
var palettePaths = map[string]string{
	"skin": "gfx/portraits/skin_palette.dds",
	"hair": "gfx/portraits/hair_palette.dds",
	"eye":  "gfx/portraits/eye_palette.dds",
}

// templateEthnicity is the ethnicity of common/ethnicities every other
// ethnicity of Victoria 3 builds on, which a portrait started from scratch is
// made from.
const templateEthnicity = "ethnicity_template"

// portrait is what the portrait panel shows: the models on the GPU, the
// viewer that draws them, the character whose DNA they are drawn from, the
// game data a character is made from, and the loading that produces all of
// it.
type portrait struct {
	renderer *render.Renderer
	viewer   *render.Viewer

	// body and head are the models drawn, and nil until a load has
	// finished. framing is set when the camera has yet to frame the head,
	// once it has been morphed and posed.
	body, head *render.Model
	framing    bool

	// figure is set when the camera frames the whole figure rather than the
	// face, which it keeps doing for every portrait drawn after.
	figure bool

	// sex is whose body and head are loaded.
	sex genes.Sex

	// loader reads the parts of the head again when the hair or the beard
	// changes, and bodySource is the body as read, which the head is seated
	// on. The loader is only used by one goroutine at a time: the load that
	// made it, then one rebuilding of the head after another.
	loader     *entity.Loader
	bodySource *model.Model

	// assembly is how the game puts the portrait together: which body and
	// head each sex is drawn with, and the joints that join them.
	assembly assembly

	// The game data a character is made from: the genes, the ethnicities as
	// written, the DNA of the game's characters, the portrait accessories
	// the accessory genes pick, and the palettes the colours are picked
	// from, by what each colours, as read and as pixels for the interface.
	genes       *genes.Genes
	ethnicities database.Table[genes.Ethnicity]
	characters  database.Table[genes.DNA]

	// characterFolder is the folder below a game or mod folder its characters
	// are kept in, common/dna_data or, in Europa Universalis 5,
	// common/persistent_dna, whose DNA is written with a priority; see
	// characterFolderOf.
	characterFolder string
	accessories     database.Table[genes.Accessory]
	palettes        map[string]*texture.Image
	swatches        map[string]*image.RGBA

	// character is the DNA the portrait is drawn from, and basis what it
	// started from.
	character *character
	basis     basis

	// parts are the entities the head is drawn with now, and bodyParts
	// those the body is dressed in, and rebuilding is set while the head or
	// the body is read again with other parts, or props are read.
	parts      []string
	bodyParts  []string
	rebuilding bool
	heads      chan headResult

	// modifiers are the portrait modifiers, which give an animation its
	// props. propsFor is what is played that played, the props of the
	// animation, were worked out for; wantedProps are the props hung from
	// joints the DNA and the animation give, and props those hung and
	// uploaded; see props.go.
	modifiers   database.Table[genes.ModifierGroup]
	propsFor    string
	played      playedProps
	wantedProps []propSpec
	props       []hungProp

	// geneTextures are the textures the decals and texture overrides of the
	// genes name, by their paths, and decalProblem why the decals could last
	// not be laid, so that it is reported once.
	geneTextures map[string]*texture.Image
	decalProblem string

	// highlight is what the portrait highlights; see highlight.go.
	highlight highlighting

	// animation is what the portrait plays; see animation.go.
	animation animating

	// staging is the environment and the camera; see staging.go.
	staging staging

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

// basis is what a character started from: nothing but the template
// ethnicity, an ethnicity, or a character of the game.
type basis struct {
	kind basisKind

	// key is the ethnicity's or the character's, and ethnicity the
	// ethnicity, flattened, whose colours the colour pickers outline: the
	// template ethnicity's for a portrait started from scratch.
	key       string
	ethnicity *genes.Ethnicity

	// identifier is the key the portrait's DNA is saved under, which the
	// user can change: a character's own to start with, nothing for an
	// ethnicity or scratch. file is where the DNA is kept, the file a
	// character's was read from or the one it was saved to since, nil for a
	// DNA that is in no file yet; see saving.go.
	identifier string
	file       *dnaFile
}

type basisKind int

const (
	basisScratch basisKind = iota
	basisEthnicity
	basisCharacter
	basisClipboard
)

// describe is what the portrait shows of a basis: what it is and the key the
// game defines it under. The game files give a character or an ethnicity no
// name of its own, so it is not made one up.
func (b basis) describe() string {
	switch b.kind {
	case basisEthnicity:
		return "Ethnicity " + b.key
	case basisCharacter:
		return b.key
	case basisClipboard:
		if b.key == "" {
			return "a DNA string from the clipboard"
		}

		return b.key + " from the clipboard"
	default:
		return "Scratch"
	}
}

// portraitResult is what a load comes back with.
type portraitResult struct {
	generation int
	sex        genes.Sex
	dna        *genes.DNA
	basis      basis

	assembly        assembly
	loader          *entity.Loader
	assets          *asset.Assets
	animations      database.Table[genes.AnimationSet]
	set             *folders.Set
	environments    database.Table[staged.Environment]
	cameras         database.Table[staged.PortraitCamera]
	settings        staged.Settings
	body, head      *model.Model
	parts           []string
	tags            []string
	genes           *genes.Genes
	ethnicities     database.Table[genes.Ethnicity]
	characters      database.Table[genes.DNA]
	characterFolder string
	accessories     database.Table[genes.Accessory]
	modifiers       database.Table[genes.ModifierGroup]
	palettes        map[string]*texture.Image
	swatches        map[string]*image.RGBA
	decals          map[string]*texture.Image
	diagnostics     report.Diagnostics
	err             error
}

// headResult is what reading the head again with other parts comes back
// with.
type headResult struct {
	generation  int
	head        *model.Model
	parts       []string
	body        *model.Model
	bodyParts   []string
	props       []loadedProp
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

	return &portrait{renderer: renderer, viewer: viewer, results: make(chan portraitResult, 1), heads: make(chan headResult, 1)}, nil
}

// load starts reading the portrait of a sex from the configured folders on a
// goroutine of its own: reading the asset files and the genes and decoding
// the textures takes long enough that the window would stop answering
// meanwhile. The portrait is drawn from a DNA once it is loaded, and from the
// template ethnicity for no DNA.
func (p *portrait) load(configured []config.Folder, sex genes.Sex, dna *genes.DNA, from basis) {
	p.generation++
	p.loading = true
	p.problem = nil

	generation := p.generation

	sources := make([]folders.Source, 0, len(configured))
	for _, folder := range configured {
		sources = append(sources, folders.Source{Name: folder.Name, Path: folder.Path})
	}

	go func() {
		result := readPortrait(folders.Open(sources), sex, dna, from)
		result.generation = generation

		send(p.results, result)
	}()
}

// send hands a result over without blocking: one nobody waits for any more is
// replaced rather than left blocking the goroutine.
func send[T any](results chan T, result T) {
	select {
	case results <- result:
	default:
		select {
		case <-results:
		default:
		}
		results <- result
	}
}

// readPortrait reads the body and the head of a sex, seats the head on the
// body, reads the game data a character is made from, and draws the head
// with the parts the DNA picks, the template ethnicity's for none. It is
// plain Go, which runs off the thread that draws.
func readPortrait(set *folders.Set, sex genes.Sex, dna *genes.DNA, from basis) portraitResult {
	result := portraitResult{sex: sex, basis: from}
	assets := asset.Load(set)
	loader := entity.NewLoader(set, assets)

	// How the game puts a portrait together decides which body is read.
	built, diagnostics := loadAssembly(set)
	result.assembly = built
	result.diagnostics = append(result.diagnostics, diagnostics...)

	bodyEntity, _ := built.entities(sex)

	body, read, err := loader.Load(bodyEntity)
	if err != nil {
		return portraitResult{err: err}
	}

	result.diagnostics = append(result.diagnostics, read...)
	result.loader, result.body = loader, body
	result.assets = assets

	result.animations, diagnostics = genes.LoadAnimations(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	// How the game stages a portrait: the environments that light it, the
	// cameras that look at it, and what every environment shares.
	result.set = set

	result.environments, diagnostics = staged.LoadEnvironments(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	result.cameras, diagnostics = staged.LoadCameras(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	result.settings, diagnostics = staged.LoadSettings(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	// What the genes files hold that is worth knowing is a warning or worse;
	// the rest is about fields the parser keeps without reading.
	result.genes = genes.Load(set)
	result.diagnostics = append(result.diagnostics, result.genes.Diagnostics.Filter(report.SeverityWarning)...)

	result.ethnicities, diagnostics = genes.LoadEthnicities(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	result.characters, diagnostics = genes.LoadDNA(set)
	result.characterFolder = characterFolderOf(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	result.accessories, diagnostics = genes.LoadAccessories(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	result.modifiers, diagnostics = genes.LoadModifiers(set)
	result.diagnostics = append(result.diagnostics, diagnostics.Filter(report.SeverityWarning)...)

	// A portrait started from scratch is the template ethnicity, every
	// gene neutral, as the game's portrait editor shows it.
	if result.basis.kind == basisScratch {
		ethnicity, err := genes.Flatten(result.ethnicities, templateEthnicity)
		if err != nil {
			result.diagnostics = append(result.diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "ethnicity " + templateEthnicity, Message: err.Error() + "; the portrait is neutral"})
		}

		result.basis.kind, result.basis.key, result.basis.ethnicity = basisScratch, templateEthnicity, ethnicity
	}

	if dna == nil {
		dna = result.genes.Preview(result.basis.ethnicity, genes.PreviewNeutral)
	}

	result.dna = dna

	// The palettes the colours of the skin, the hair and the eyes are
	// picked from by the two numbers a DNA gives each.
	result.palettes, result.swatches = map[string]*texture.Image{}, map[string]*image.RGBA{}

	for what, path := range palettePaths {
		palette, err := readTexture(set, path)
		if err != nil {
			result.diagnostics = append(result.diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: path, Message: err.Error() + "; drawn in a stand in colour"})

			continue
		}

		result.palettes[what], result.swatches[what] = palette, swatchOf(palette)
	}

	// The textures of the decals the genes lay over the skin, and of the
	// texture overrides they draw it with.
	decals, diagnostics := readGeneTextures(set, result.genes)
	result.diagnostics = append(result.diagnostics, diagnostics...)
	result.decals = decals

	// The props hung from joints are read once the portrait plays, with
	// those of its animation, and the looks it is dressed in once it is
	// drawn; see props.go and looks.go.
	drawn, diagnostics := accessoryParts(result.genes, result.accessories, result.genes.ApplyModifiers(dna, sex), sex)
	result.diagnostics = append(result.diagnostics, diagnostics...)
	result.tags = drawn.tags

	head, diagnostics, err := buildHead(loader, built, sex, body, drawn.head)
	if err != nil {
		return portraitResult{err: err}
	}

	result.diagnostics = append(result.diagnostics, diagnostics...)
	result.head, result.parts = head, drawn.head

	return result
}

// swatchOf is a palette as pixels the interface can show, worked out from
// its blocks where it is compressed.
func swatchOf(palette *texture.Image) *image.RGBA {
	swatch := image.NewRGBA(image.Rect(0, 0, palette.Width, palette.Height))

	for y := range palette.Height {
		for x := range palette.Width {
			colour, _ := palette.Colour(x, y)
			copy(swatch.Pix[swatch.PixOffset(x, y):], colour[:])
		}
	}

	return swatch
}

// buildHead reads the head of a sex, seats it on the body, and adds the parts
// posed with it. A part that cannot be read is reported, and the head drawn
// without it.
func buildHead(loader *entity.Loader, built assembly, sex genes.Sex, body *model.Model, parts []string) (*model.Model, report.Diagnostics, error) {
	_, headEntity := built.entities(sex)

	head, diagnostics, err := loader.Load(headEntity)
	if err != nil {
		return nil, nil, err
	}

	// A head that cannot be seated is still drawn, where its mesh puts it.
	if err := seat(body, head, built.links()[0]); err != nil {
		diagnostics = append(diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "entity " + headEntity, Message: err.Error()})
	} else {
		// Where the neck meets the body the two meshes share a ring of
		// vertices whose normals the game's files have disagree, which the
		// light would draw as a seam; the head takes the body's there.
		head.MatchNormals(body, seamTolerance)
	}

	for _, name := range parts {
		part, read, err := loader.Load(name)
		diagnostics = append(diagnostics, read...)

		if err == nil && !head.SharePose(part) {
			err = errors.New("the head has no skeleton to pose it with")
		}

		if err != nil {
			diagnostics = append(diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "entity " + name, Message: err.Error() + "; drawn without it"})
		}
	}

	return head, diagnostics, nil
}

// seat places the head where the game attaches it to the body: with the
// joint of the link on the body's, in the pose their meshes are stored in.
func seat(body, head *model.Model, link render.JointLink) error {
	thoracic, ok := body.Joint(link.Parent)
	if !ok {
		return fmt.Errorf("%s has no joint %s to attach the head to; the head is drawn where its mesh puts it", body.Name, link.Parent)
	}

	root, ok := head.Joint(link.Child)
	if !ok {
		return fmt.Errorf("%s has no joint %s to attach it by; it is drawn where its mesh puts it", head.Name, link.Child)
	}

	undo, ok := root.Inverse()
	if !ok {
		return errors.New("the joint the head is attached by cannot be undone; it is drawn where its mesh puts it")
	}

	head.Place(undo.Then(thoracic))

	return nil
}

// poll picks up a finished load, or a head read again, and uploads its
// models. It reports whether a load finished, and runs on the goroutine that
// owns the OpenGL context.
func (p *portrait) poll() bool {
	p.pollHead()

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
		// the body's additive animations move, and the ring where its neck
		// meets the body is drawn where the body's is, which its own blend
		// shapes and bones would part from it.
		head.Follow(body, result.assembly.links()...)
		head.Weld(body, weldTolerance)

		p.sex, p.loader, p.bodySource, p.parts = result.sex, result.loader, result.body, result.parts
		p.assembly = result.assembly
		p.genes, p.ethnicities, p.characters, p.accessories = result.genes, result.ethnicities, result.characters, result.accessories
		p.characterFolder = result.characterFolder
		p.modifiers = result.modifiers
		p.palettes, p.swatches = result.palettes, result.swatches
		p.geneTextures, p.decalProblem = result.decals, ""
		p.basis = result.basis

		age := int32(defaultAge)
		if p.character != nil {
			age = p.character.age
		}

		if given, ok := result.dna.Age.Get(); ok {
			age = ageOf(given)
		}

		// The looks picked stay picked through another sex or another start,
		// and the new body is dressed in them as soon as it is drawn.
		var picked map[string]string
		if p.character != nil {
			picked = p.character.looks.picked
		}

		p.character = newCharacter(result.genes, result.sex, result.dna, append(body.Attributes(), head.Attributes()...))
		p.character.age = age
		p.character.looks = looks{groups: result.modifiers, picked: picked}
		p.character.tags = result.tags
		p.character.partsChanged = p.character.looks.dressing()
		p.bodyParts = nil

		p.animation.prepare(result.animations, result.assets, result.body, result.head)
		p.staging.prepare(result.set, result.environments, result.cameras, result.settings)
		p.animation.restart(p.sex)
		p.highlight.forget()

		p.framing = true

		return true

	default:
		return false
	}
}

// pollHead picks up a head read again with other parts, and draws it in the
// place of the one drawn.
func (p *portrait) pollHead() {
	select {
	case result := <-p.heads:
		p.rebuilding = false

		if result.generation != p.generation || p.body == nil {
			return
		}

		p.diagnostics = append(p.diagnostics, result.diagnostics...)

		if result.err != nil {
			warn(result.err)

			return
		}

		// A body dressed in other clothes takes the place of the one drawn,
		// and the head drawn follows it and meets it at the neck as it did
		// the old one; a new head does the same below.
		if result.body != nil {
			body, err := p.renderer.Upload(result.body)
			if err != nil {
				warn(err)

				return
			}

			p.body.Unload()
			p.body, p.bodySource, p.bodyParts = body, result.body, result.bodyParts
			p.head.Follow(p.body, p.assembly.links()...)
			p.head.Weld(p.body, weldTolerance)
			p.animation.bodyChanged(result.body)
			p.animation.restart(p.sex)
			p.character.setDrawable(append(p.body.Attributes(), p.head.Attributes()...))

			p.character.changed, p.character.coloursChanged = true, true
			p.highlight.forget()
		}

		// A head that needed no reading again comes back as nil, with the
		// props that did.
		if result.head != nil {
			head, err := p.renderer.Upload(result.head)
			if err != nil {
				warn(err)

				return
			}

			p.head.Unload()
			p.head, p.parts = head, result.parts
			head.Follow(p.body, p.assembly.links()...)
			head.Weld(p.body, weldTolerance)
			p.animation.headChanged(result.head)
			p.animation.restart(p.sex)

			// The new head is morphed and has its decals laid like the old
			// one.
			p.character.changed = true
			p.highlight.forget()
		}

		p.takeProps(result.props)

	default:
	}
}

// rebuildHead reads the head again on a goroutine of its own when the
// accessory genes, or the looks the character is dressed in, pick other parts
// for it than it is drawn with, and the body again when they dress it in
// other clothes. While they are being read, another change waits for them.
func (p *portrait) rebuildHead() {
	if p.rebuilding || p.loader == nil || !p.character.partsChanged {
		return
	}

	p.character.partsChanged = false

	// The props of the animation are held beside those of the DNA, as the
	// game holds them: the DNA's are mostly glasses and the like.
	played := p.played
	drawn, diagnostics := accessoryParts(p.genes, p.accessories, p.character.dressed(), p.sex)
	parts := append(drawn.head, played.head...)
	p.wantedProps = nil

	// A prop both give is held once.
	for _, prop := range append(drawn.hung, played.hung...) {
		if !slices.Contains(p.wantedProps, prop) {
			p.wantedProps = append(p.wantedProps, prop)
		}
	}

	p.diagnostics = append(p.diagnostics, diagnostics...)

	// Other tags set other settings of the genes, such as the shrinking of
	// the legs under trousers.
	if !slices.Equal(drawn.tags, p.character.tags) {
		p.character.tags = drawn.tags
		p.character.changed = true
	}

	// The props are read with the head, since the loader reads one thing at
	// a time; props no longer wanted are let go of at once.
	missing := p.missingProps()
	needHead := !slices.Equal(parts, p.parts)
	needBody := !slices.Equal(drawn.body, p.bodyParts)

	if !needHead && !needBody && len(missing) == 0 {
		p.takeProps(nil)

		return
	}

	p.rebuilding = true
	generation, loader, built, sex, body := p.generation, p.loader, p.assembly, p.sex, p.bodySource

	go func() {
		result := headResult{generation: generation, parts: parts}

		if needBody {
			result.body, result.diagnostics, result.err = dressBody(loader, built, sex, drawn.body)
			if result.err != nil {
				send(p.heads, result)

				return
			}

			result.bodyParts = drawn.body
		}

		if needHead {
			var diagnostics report.Diagnostics

			result.head, diagnostics, result.err = buildHead(loader, built, sex, body, parts)
			result.diagnostics = append(result.diagnostics, diagnostics...)
		}

		for _, prop := range missing {
			read, diagnostics, err := loader.Load(prop.entity)
			result.diagnostics = append(result.diagnostics, diagnostics...)

			if err != nil {
				result.diagnostics = append(result.diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "entity " + prop.entity, Message: err.Error() + "; drawn without it"})

				continue
			}

			result.props = append(result.props, loadedProp{propSpec: prop, model: read})
		}

		send(p.heads, result)
	}()
}

// dressBody reads the body of a sex again, dressed in the parts the looks
// put on it: each posed with it, as the game draws an accessory whose
// shared_pose_entity is the torso. A part that cannot be read is reported,
// and the body dressed without it.
func dressBody(loader *entity.Loader, built assembly, sex genes.Sex, parts []string) (*model.Model, report.Diagnostics, error) {
	bodyEntity, _ := built.entities(sex)

	body, diagnostics, err := loader.Load(bodyEntity)
	if err != nil {
		return nil, nil, err
	}

	for _, name := range parts {
		part, read, err := loader.Load(name)
		diagnostics = append(diagnostics, read...)

		if err == nil && !body.SharePose(part) {
			err = errors.New("the body has no skeleton to pose it with")
		}

		if err != nil {
			diagnostics = append(diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "entity " + name, Message: err.Error() + "; drawn without it"})
		}
	}

	return body, diagnostics, nil
}

// colour sets the colours of the skin, the hair and the eyes from the
// palettes, where the colour genes pick them, as the DNA's two numbers for
// each from 0 to 255. A palette that could not be read leaves its colour as
// the renderer has it. A game whose portrait settings say its palettes are
// sRGB, as Europa Universalis 5's do, multiplies by the colour in linear
// light; the others by the colour as it is stored. The morph genes shift the
// colour picked first, by its hue, saturation and value, as they grey the
// hair of the old.
func (p *portrait) colour() {
	look := &p.viewer.Look
	look.PaletteColor, look.HairColor, look.EyeColor = render.DefaultLook.PaletteColor, render.DefaultLook.HairColor, render.DefaultLook.EyeColor

	// The looks may move a colour, as a portrait modifier's colour change
	// does.
	var dressed *genes.DNA
	if p.character.looks.dressing() {
		dressed = p.character.dressed().DNA
	}

	for what, target := range map[string]*[3]float32{"skin": &look.PaletteColor, "hair": &look.HairColor, "eye": &look.EyeColor} {
		gene, ok := p.character.colour(what)
		if !ok {
			continue
		}

		place := gene.value
		if allele, ok := dominant(dressed, gene.gene.Key); ok {
			place = [2]int32{int32(allele.Value), int32(allele.Second)}
		}

		across, down := float64(place[0])/maxGeneValue, float64(place[1])/maxGeneValue

		lookUp := p.palettes[what].Sample
		if what == "hair" {
			lookUp = func(across, down float64) ([4]byte, bool) {
				return p.staging.grading.hairColour(p.palettes[what], across, down)
			}
		}

		if colour, ok := lookUp(across, down); ok {
			colour = render.ShiftHSV(colour, p.character.colourShift(what))

			*target = render.PaletteColour(colour)
			if p.assembly.SRGBColorPalettes {
				*target = render.SRGBPaletteColour(colour)
			}
		}
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

	for _, joint := range faceJoints {
		rest, posed, ok := p.head.JointPosition(joint)
		if !ok {
			continue
		}

		for axis := range 3 {
			box[0][axis] += posed[axis] - rest[axis]
			box[1][axis] += posed[axis] - rest[axis]
		}

		return box
	}

	return box
}

// frame points the camera at the face or the whole figure, whichever it is
// set to frame.
func (p *portrait) frame() {
	if p.staging.look(p) {
		return
	}

	if p.figure {
		p.frameFigure()
	} else {
		p.frameFace()
	}
}

// frameFace points the camera at the head from the front, which is where the
// morph genes show.
func (p *portrait) frameFace() {
	face := p.face()
	p.viewer.FieldOfView = 0
	p.viewer.Frame(face[0], face[1])
	p.viewer.Reset()
}

// frameFigure points the camera at the whole figure from the front: the body
// and the head where it is now.
func (p *portrait) frameFigure() {
	face := p.face()
	p.viewer.FieldOfView = 0

	for axis := range 3 {
		face[0][axis] = min(face[0][axis], p.body.Min[axis])
		face[1][axis] = max(face[1][axis], p.body.Max[axis])
	}

	p.viewer.Frame(face[0], face[1])
	p.viewer.Reset()
}

// loaded reports whether there is a portrait to show.
func (p *portrait) loaded() bool {
	return p.body != nil && p.head != nil && p.character != nil
}

// draw morphs the models by the genes if they changed, highlights what is
// pointed at, and draws the models into the viewer's picture, at the size
// the panel last had for it. It runs before the interface, while raylib
// drawing is active.
func (p *portrait) draw() {
	if p.width > 0 && p.height > 0 {
		p.viewer.Resize(p.width, p.height)
	}

	if !p.loaded() {
		p.viewer.Draw()

		return
	}

	p.wantProps()
	p.rebuildHead()

	// The morph genes and the age shift the colours as well.
	if p.character.coloursChanged || p.character.changed {
		p.colour()
		p.character.coloursChanged = false

		// The skin colour is blended into the decals as they are laid.
		p.character.changed = true
	}

	if p.character.changed {
		attributes := p.character.attributes()
		p.body.Morph(attributes)
		p.head.Morph(attributes)
		p.layDecals()
		p.character.changed = false
	}

	p.animation.pose(p.loader, p.body, p.head)
	p.highlight.update(p)

	// The face is framed once the panel has given the picture its size,
	// which the framing fits the face to.
	if p.framing && p.width > 1 && p.height > 1 {
		p.frame()
		p.framing = false
	}

	p.viewer.Look.Lighting = p.staging.lighting(p)
	p.viewer.Draw(p.models()...)
}

// layDecals draws the skin of the head with the texture override the genes
// pick, and lays the decals the genes lay at the portrait's age over the
// skin of the body and the head, in the skin colour of the look. A decal
// that cannot be laid is reported once, and the portrait drawn without it.
// The games' overrides are all of faces, so they are laid over the head.
func (p *portrait) layDecals() {
	decals := p.character.decals(p.geneTextures)

	if err := p.head.OverrideSkin(headPart, p.character.skinOverride(p.geneTextures)); err != nil && err.Error() != p.decalProblem {
		p.decalProblem = err.Error()
		warn(err)
	}

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
	p.unloadProps()

	for _, uploaded := range []*render.Model{p.body, p.head} {
		if uploaded != nil {
			uploaded.Unload()
		}
	}

	p.body, p.head = nil, nil
	p.character, p.parts = nil, nil
	p.highlight.forget()
}

// unload releases everything on the GPU. It needs the OpenGL context.
func (p *portrait) unload() {
	p.unloadModels()
	p.staging.unload()
	p.viewer.Unload()
	p.renderer.Unload()
}

// start draws the portrait from a DNA of a sex: the character's genes are set
// as the DNA has them, and the head drawn with the parts it picks. Another
// sex is another body and head, which are loaded anew.
func (a *App) start(from basis, dna *genes.DNA, sex genes.Sex) {
	p := a.portrait
	if p == nil {
		return
	}

	// Where a boy and a man, or a girl and a woman, share their body and
	// head, as in Victoria 3, only the settings of the genes change between
	// them.
	body, _ := p.assembly.entities(sex)
	loaded, _ := p.assembly.entities(p.sex)

	if !p.loaded() || body != loaded {
		p.load(a.settings.Folders, sex, dna, from)
		a.setStatus("Loading the portrait")

		return
	}

	p.basis = from
	p.sex, p.character.sex = sex, sex
	p.character.reset(dna)
	p.animation.restart(sex)
	p.framing = true

	if age, ok := dna.Age.Get(); ok {
		p.character.age = ageOf(age)
	}

	p.highlight.forget()
}

// startPortrait loads the portrait from the folders in the settings, from
// scratch, or says what is missing.
func (a *App) startPortrait() {
	if a.portrait == nil {
		return
	}

	if !a.hasFolders() {
		a.portrait.unloadModels()
		a.portrait.problem = nil

		return
	}

	a.portrait.load(a.settings.Folders, genes.Male, nil, basis{})
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
