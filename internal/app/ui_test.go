//go:build uitest

package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-parser-go/genes"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// startApp runs the real application in a hidden window, with the settings
// file given, or none when it is empty.
func startApp(t *testing.T, settings string) (*App, string) {
	t.Helper()

	// raylib and OpenGL belong to the thread that created the window, and a
	// test runs on a goroutine the scheduler may move between threads.
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	dir := t.TempDir()

	if settings != "" {
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	application, err := New(Options{ConfigDir: dir, Hidden: true})
	if err != nil {
		t.Fatalf("start the application: %v", err)
	}

	return application, dir
}

// The application starts in a hidden window, draws a few frames and closes
// again, writing its layout into the configuration folder as it does.
func TestApplicationStartsAndCloses(t *testing.T) {
	application, dir := startApp(t, "")

	for range 3 {
		application.Step()
	}

	application.Close()

	if !fileExists(filepath.Join(dir, "layout.ini")) {
		t.Error("the layout was not saved when the window closed")
	}
}

// The interface scale stored in the settings is the one the interface is
// drawn at from the first frame on.
func TestStoredInterfaceScaleIsApplied(t *testing.T) {
	application, _ := startApp(t, `{"Folders":[],"InterfaceScale":1.5}`)
	defer application.Close()

	application.Step()

	if got := application.window.Scale(); got != 1.5 {
		t.Errorf("interface scale = %v, want the stored 1.5", got)
	}
}

// A settings file that cannot be read does not keep the application from
// starting: it comes up with the defaults and says so.
func TestBrokenSettingsStartWithTheDefaults(t *testing.T) {
	application, _ := startApp(t, `{"Folders": [`)
	defer application.Close()

	application.Step()

	if len(application.settings.Folders) != 0 {
		t.Errorf("folders = %+v, want none", application.settings.Folders)
	}

	if application.state.status != "Settings could not be loaded, using defaults" {
		t.Errorf("status = %q, want it to say the settings were not loaded", application.state.status)
	}
}

// The settings window draws, with folders in it, without upsetting Dear
// ImGui's checks on the window and table stacks.
func TestSettingsWindowDraws(t *testing.T) {
	application, _ := startApp(t, `{"Folders":[{"Name":"game","Path":"C:\\Games\\Victoria 3\\game"}]}`)
	defer application.Close()

	application.state.showSettings = true

	for range 3 {
		application.Step()
	}

	if !application.state.showSettings {
		t.Error("the settings window closed by itself")
	}
}

// Help > About opens a window over everything else, which credits every work
// the application uses, and Escape closes it again.
func TestAboutWindow(t *testing.T) {
	application, _ := startApp(t, "")
	defer application.Close()

	open := func() bool {
		return imgui.IsPopupOpenStrV("", imgui.PopupFlagsAnyPopupId|imgui.PopupFlagsAnyPopupLevel)
	}

	application.state.showAbout = true
	application.Step()
	application.Step()

	if _, ok := gui.FindWindow(popupAbout); !ok || !open() {
		t.Fatal("Help > About opened no window")
	}

	for _, credit := range credits {
		if credit.work == "" || credit.notice == "" || credit.url == "" {
			t.Errorf("credit %+v leaves something out", credit)
		}
	}

	io := imgui.CurrentIO()
	io.AddKeyEvent(imgui.KeyEscape, true)
	application.Step()
	io.AddKeyEvent(imgui.KeyEscape, false)
	application.Step()

	if open() {
		t.Error("the About window is still open after Escape")
	}
}

// waitForPortrait steps the application until its portrait has finished
// loading, one way or the other.
func waitForPortrait(t *testing.T, application *App) {
	t.Helper()

	deadline := time.Now().Add(time.Minute)

	for application.portrait.loading {
		if time.Now().After(deadline) {
			t.Fatal("the portrait did not finish loading within a minute")
		}

		application.Step()
	}

	// One more frame, so that what was loaded is drawn.
	application.Step()
}

// A folder that holds no body is reported in the panel, and the application
// carries on.
func TestPortraitOfAFolderWithoutABody(t *testing.T) {
	folder := t.TempDir()
	if err := os.MkdirAll(filepath.Join(folder, "gfx"), 0o755); err != nil {
		t.Fatal(err)
	}

	settings := fmt.Sprintf(`{"Folders":[{"Name":"empty","Path":%q}]}`, folder)

	application, _ := startApp(t, settings)
	defer application.Close()

	if application.portrait == nil {
		t.Fatal("the 3D view did not start")
	}

	waitForPortrait(t, application)

	if application.portrait.problem == nil || application.portrait.loaded() {
		t.Errorf("problem %v, loaded %v; want the missing entity reported", application.portrait.problem, application.portrait.loaded())
	}

	if !strings.Contains(application.state.status, "could not be loaded") {
		t.Errorf("status = %q", application.state.status)
	}
}

// With the game configured, the male body and head are loaded, morphed by
// the genes of a character and drawn into the portrait panel. It is skipped
// unless PDX_GAME_DIR points at the game.
func TestPortraitOfTheInstalledGame(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	settings := fmt.Sprintf(`{"Folders":[{"Name":"game","Path":%q}],"InterfaceScale":1}`, game)

	application, _ := startApp(t, settings)
	defer application.Close()

	waitForPortrait(t, application)

	if application.portrait.problem != nil || !application.portrait.loaded() {
		t.Fatalf("problem %v, loaded %v", application.portrait.problem, application.portrait.loaded())
	}

	// The panel sized the picture to itself, and the body is in it.
	width, height := application.portrait.viewer.Size()
	if width < 100 || height < 100 {
		t.Errorf("the picture is %dx%d, want it sized to the panel", width, height)
	}

	picture := application.portrait.viewer.Image()

	drawn := 0
	for index := 3; index < len(picture.Pix); index += 4 {
		if picture.Pix[index] > 0 {
			drawn++
		}
	}

	if drawn < int(width*height)/20 {
		t.Errorf("%d of %d pixels of the picture drawn", drawn, width*height)
	}

	// The eyes, the eyelashes and the teeth were found and drawn with the
	// head.
	for _, diagnostic := range application.portrait.diagnostics {
		if strings.HasPrefix(diagnostic.Subject, "gene ") {
			t.Errorf("a part of the head is missing: %s", diagnostic)
		}
	}

	// The portrait starts from scratch: the template ethnicity, every gene
	// at its first template at 127, as the game's portrait editor shows it,
	// and the welcome screen asks what to start from.
	portrait := application.portrait
	if portrait.basis.kind != basisScratch || portrait.basis.ethnicity == nil || portrait.basis.ethnicity.Key != templateEthnicity {
		t.Errorf("basis = %+v, want %s from scratch", portrait.basis, templateEthnicity)
	}

	if !application.state.welcome {
		t.Error("the welcome screen is not shown at the start")
	}

	application.state.welcome = false

	character := portrait.character

	for _, morph := range character.morphs {
		if morph.value != 127 || morph.template != 0 && morph.gene.Templates[morph.template].Index != 0 {
			t.Errorf("%s is at %s %d, want its first template at 127", morph.gene.Key, morph.gene.Templates[morph.template].Name, morph.value)
		}

		// Every gene of the game has a category and a name.
		if categories[morph.category].id == categoryOther {
			t.Errorf("%s is not catalogued", morph.gene.Key)
		}
	}

	for _, accessory := range character.accessories {
		if !accessory.hidden && categories[accessory.category].id == categoryOther {
			t.Errorf("%s is not catalogued", accessory.gene.Key)
		}
	}

	if character.lists(otherCategory()) {
		t.Error("Other is listed, with every gene of the game catalogued")
	}

	morphing := 0

	var nose *morphGene

	for index := range character.morphs {
		morph := &character.morphs[index]
		if morph.drawn {
			morphing++
		}

		if morph.gene.Key == "gene_nose_size" {
			nose = morph
		}

		// The chin is shaped by an additive animation rather than a blend
		// shape.
		if morph.gene.Key == "gene_chin_forward" && !morph.drawn {
			t.Error("gene_chin_forward is not drawn")
		}
	}

	if len(character.morphs) < 50 || morphing < len(character.morphs)/3 || nose == nil || !nose.drawn {
		t.Fatalf("%d morph genes, %d of them drawn, the nose %+v", len(character.morphs), morphing, nose)
	}

	// Changing a gene changes the picture, and is counted as a change in its
	// category.
	before := portrait.viewer.Image()
	nose.value = maxGeneValue
	character.changed = true
	application.Step()

	if changed := differingPixels(before, portrait.viewer.Image()); changed == 0 {
		t.Error("changing the size of the nose changed nothing drawn")
	}

	if keys, counts := character.changes(); !keys["gene_nose_size"] || counts[nose.category] != 1 || len(keys) != 1 {
		t.Errorf("changes = %v, %v; want the nose alone", keys, counts)
	}

	character.resetGene("gene_nose_size")

	if nose.value != neutralValue {
		t.Errorf("the nose is at %d after resetting it, want %d", nose.value, neutralValue)
	}

	highlighted(t, application, nose)
	stagedInEnvironments(t, application)
	animated(t, application)

	// Turning the camera turns the body.
	portrait.viewer.Rotate(1, 0)
	application.Step()

	if portrait.viewer.Yaw != 1 {
		t.Errorf("yaw = %v after turning by 1", portrait.viewer.Yaw)
	}

	// The panel draws every category, a search, and a search that finds
	// nothing.
	for index := range categories {
		application.state.category = index
		application.Step()
	}

	for _, search := range []string{"nose", "nothing at all"} {
		application.state.geneSearch = search
		application.Step()
	}

	application.state.geneSearch = ""

	agedDecals(t, application)
	startingPoints(t, application)
}

// highlighted checks that pointing at a gene highlights what it moves, once
// it has been pointed at for long enough, and that the highlight goes when
// nothing is pointed at.
func highlighted(t *testing.T, application *App, nose *morphGene) {
	t.Helper()

	portrait := application.portrait

	// The portrait stands still, so that only the highlight changes what is
	// drawn.
	portrait.animation.none = true
	portrait.animation.restart(portrait.sex)

	defer func() {
		portrait.animation.none = false
		portrait.animation.restart(portrait.sex)
	}()

	portrait.highlight.enabled = true
	portrait.frameFace()
	application.Step()

	plain := portrait.viewer.Image()

	for range highlightDelay + 2 {
		portrait.highlight.point(nose.gene.Key, []string{nose.gene.Key})
		application.Step()
	}

	measured, ok := portrait.highlight.measured[portrait.highlight.shown]
	if !ok || measured.furthest <= 0 {
		t.Fatalf("highlighting %s measured %+v", nose.gene.Key, measured)
	}

	if changed := differingPixels(plain, portrait.viewer.Image()); changed < 100 {
		t.Errorf("highlighting the nose changes %d pixels, want it to show", changed)
	}

	if dump := os.Getenv("PDX_DUMP_DIR"); dump != "" {
		if err := writePNG(filepath.Join(dump, "portrait_highlight_nose.png"), portrait.viewer.Image()); err != nil {
			t.Error(err)
		}
	}

	// Held, as while its slider is dragged, the gene is pointed at but the
	// portrait is shown plain.
	portrait.highlight.point(nose.gene.Key, []string{nose.gene.Key})
	portrait.highlight.hold()
	application.Step()

	if changed := differingPixels(plain, portrait.viewer.Image()); changed != 0 {
		t.Errorf("%d pixels differ while the highlight is held", changed)
	}

	// Nothing pointed at: the highlight goes.
	application.Step()
	application.Step()

	if changed := differingPixels(plain, portrait.viewer.Image()); changed != 0 {
		t.Errorf("%d pixels still differ once nothing is pointed at", changed)
	}
}

// stagedInEnvironments checks that the portrait is lit by the game's standard environment
// by default, with its lights and its cubemap, that the studio lights it
// otherwise, and that a camera of the game moves the view.
func stagedInEnvironments(t *testing.T, application *App) {
	t.Helper()

	portrait := application.portrait
	staging := &portrait.staging

	if staging.environment != staging.grading.defaultEnvironment || len(staging.cameraKeys()) == 0 {
		t.Fatalf("environment %q, %d cameras; want %s and the game's cameras", staging.environment, len(staging.cameraKeys()), staging.grading.defaultEnvironment)
	}

	// The cubemap is read off the thread that draws.
	deadline := time.Now().Add(30 * time.Second)

	for application.Step(); len(staging.reading) > 0; application.Step() {
		if time.Now().After(deadline) {
			t.Fatal("the cubemap was not read within half a minute")
		}
	}

	lighting := staging.lighting(portrait)
	if lighting == nil || len(lighting.Lights) == 0 || lighting.Environment == nil || lighting.Shadow == nil || !lighting.ACES {
		t.Fatalf("lighting = %+v, want the environment's lights, cubemap, shadow and the ACES tonemap", lighting)
	}

	// The ambient occlusion is drawn where the settings ask for it.
	ssao, ok := staging.settings.SSAO.Get()
	if asked := ok && ssao.Enabled; asked != (lighting.Occlusion != nil) {
		t.Errorf("occlusion %+v, want one %v as the settings ask", lighting.Occlusion, asked)
	}

	portrait.animation.paused = true
	application.Step()

	lit := portrait.viewer.Image()

	staging.environment = ""
	application.Step()

	if staging.lighting(portrait) != nil {
		t.Error("no environment picked lights with one")
	}

	if changed := differingPixels(lit, portrait.viewer.Image()); changed < 1000 {
		t.Errorf("the studio lights the portrait as the environment does, %d pixels apart", changed)
	}

	// A camera of the game stands where it says, and framing lets go of it.
	staging.camera = "camera_body"
	portrait.frame()

	if fov := portrait.viewer.FieldOfView; fov != 20 {
		t.Errorf("camera_body sees %v degrees, want its 20", fov)
	}

	staging.environment, staging.camera = staging.grading.defaultEnvironment, ""
	portrait.animation.paused = false
	portrait.frame()
	application.Step()
}

// startingPoints checks that the portrait can start from an ethnicity, which
// gives it the ethnicity's colours, and from a character, whose hair and
// beard are drawn with the head, and that changing the sex loads a woman.
func startingPoints(t *testing.T, application *App) {
	t.Helper()

	portrait := application.portrait

	application.openPicker(pickEthnicities)
	application.pick("caucasian_blond")
	application.Step()

	if portrait.basis.kind != basisEthnicity || portrait.basis.key != "caucasian_blond" {
		t.Errorf("basis = %+v, want caucasian_blond", portrait.basis)
	}

	if hair, ok := portrait.character.colour("hair"); !ok || hair.value == [2]int32{} {
		t.Errorf("the hair colour of caucasian_blond is %+v, want the ethnicity's", hair)
	}

	if portrait.viewer.Look.HairColor == render.DefaultLook.HairColor {
		t.Error("the hair is drawn in the stand in colour")
	}

	application.state.picker.kind = pickCharacters
	application.pick("dna_abraham_lincoln")
	waitForHead(t, application)

	if portrait.basis.kind != basisCharacter || len(portrait.parts) <= 3 {
		t.Errorf("basis %+v drawn with %v, want Lincoln's hair and beard as well as his eyes, eyelashes and teeth", portrait.basis, portrait.parts)
	}

	if dump := os.Getenv("PDX_DUMP_DIR"); dump != "" {
		portrait.frameFace()
		application.Step()

		if err := writePNG(filepath.Join(dump, "portrait_lincoln.png"), portrait.viewer.Image()); err != nil {
			t.Error(err)
		}
	}

	application.closePicker(false)

	// A woman is another body and head, loaded anew with the same DNA.
	application.start(portrait.basis, portrait.character.dna(), genes.Female)
	waitForPortrait(t, application)

	if portrait.problem != nil || !portrait.loaded() || portrait.sex != genes.Female || portrait.character.sex != genes.Female {
		t.Fatalf("problem %v, sex %v, want a woman", portrait.problem, portrait.sex)
	}

	if portrait.basis.key != "dna_abraham_lincoln" {
		t.Errorf("basis = %+v after changing the sex, want it kept", portrait.basis)
	}

	// A girl is drawn with the woman's body and head, so becoming one loads
	// nothing; only the settings of the genes change.
	application.start(portrait.basis, portrait.character.dna(), genes.Girl)

	if portrait.loading || portrait.sex != genes.Girl || portrait.character.sex != genes.Girl {
		t.Errorf("loading %v, sex %v; want a girl without loading anew", portrait.loading, portrait.sex)
	}

	application.Step()
}

// waitForHead steps the application until the head has been read again with
// the parts its genes pick.
func waitForHead(t *testing.T, application *App) {
	t.Helper()

	deadline := time.Now().Add(time.Minute)

	for application.Step(); application.portrait.rebuilding; application.Step() {
		if time.Now().After(deadline) {
			t.Fatal("the head was not read again within a minute")
		}
	}

	application.Step()
}

// agedDecals checks that the genes lay their decals over the face of an old
// man, wrinkles and crow's feet among them, with every texture they name,
// and that they are drawn. With PDX_DUMP_DIR set, the face is written out at
// the default age and at the whole of a life, to compare with the game's portrait
// editor.
func agedDecals(t *testing.T, application *App) {
	t.Helper()

	portrait := application.portrait
	character := portrait.character
	character.reset(character.basis)
	portrait.frameFace()

	dump := os.Getenv("PDX_DUMP_DIR")

	for _, age := range []int32{defaultAge, maxAge} {
		character.age = age
		character.changed = true
		application.Step()

		if dump == "" {
			continue
		}

		if err := writePNG(filepath.Join(dump, fmt.Sprintf("portrait_face_age_%d.png", age)), portrait.viewer.Image()); err != nil {
			t.Error(err)
		}
	}

	decals := character.decals(portrait.decalTextures)

	laid := map[string]bool{}

	for _, decal := range decals {
		if decal.Diffuse == nil && decal.Normal == nil && decal.Properties == nil {
			t.Errorf("a decal of the %s is laid without any of its textures", decal.BodyPart)
		}

		laid[decal.BodyPart] = true
	}

	if !laid["head"] || len(decals) < 5 {
		t.Fatalf("%d decals are laid at the whole of a life, over %v; want wrinkles over the head", len(decals), laid)
	}

	for _, gene := range []string{"gene_crowfeet", "gene_old_forehead"} {
		found := false

		for _, morph := range character.morphs {
			found = found || morph.gene.Key == gene && morph.drawn
		}

		if !found {
			t.Errorf("%s is not drawn", gene)
		}
	}

	// The decals are drawn: the face without them is another.
	aged := portrait.viewer.Image()

	for _, uploaded := range []*render.Model{portrait.body, portrait.head} {
		if err := uploaded.LayDecals(nil, portrait.viewer.Look); err != nil {
			t.Fatal(err)
		}
	}

	application.Step()

	if dump != "" {
		if err := writePNG(filepath.Join(dump, "portrait_face_age_100_without_decals.png"), portrait.viewer.Image()); err != nil {
			t.Error(err)
		}
	}

	if changed := differingPixels(aged, portrait.viewer.Image()); changed < 100 {
		t.Errorf("the decals of an old man change %d pixels of his face, want them to show", changed)
	}

	character.age = defaultAge
	character.changed = true
	application.Step()
}

// writePNG writes a picture out, for a person to look at.
func writePNG(path string, picture image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := png.Encode(file, picture); err != nil {
		file.Close()

		return err
	}

	return file.Close()
}

// differingPixels counts the pixels two pictures of the same size differ in.
func differingPixels(a, b *image.RGBA) int {
	count := 0

	for index := 0; index+3 < len(a.Pix) && index+3 < len(b.Pix); index += 4 {
		if [4]uint8(a.Pix[index:index+4]) != [4]uint8(b.Pix[index:index+4]) {
			count++
		}
	}

	return count
}

// animated checks that the portrait plays idle by default, which moves it as
// time goes on once its animations are read, and that playing none stands it
// in the pose its meshes are stored in.
func animated(t *testing.T, application *App) {
	t.Helper()

	portrait := application.portrait
	playing := &portrait.animation

	if playing.chosen != defaultAnimation || playing.none {
		t.Fatalf("playing %q, none %v; want %s", playing.chosen, playing.none, defaultAnimation)
	}

	// The animation's samples are read off the thread that draws.
	deadline := time.Now().Add(30 * time.Second)

	for {
		application.Step()

		if _, _, _, ok := playing.body.Playing(); ok && len(playing.samples) > 0 && len(playing.reading) == 0 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("the idle animation was not read within half a minute")
		}
	}

	before := portrait.viewer.Image()
	time.Sleep(300 * time.Millisecond)
	application.Step()

	if changed := differingPixels(before, portrait.viewer.Image()); changed == 0 {
		t.Error("the portrait stood still while idle played")
	}

	// Paused, it stands still.
	playing.paused = true
	application.Step()

	paused := portrait.viewer.Image()
	time.Sleep(100 * time.Millisecond)
	application.Step()

	if changed := differingPixels(paused, portrait.viewer.Image()); changed != 0 {
		t.Errorf("%d pixels changed while the animation was paused", changed)
	}

	playing.paused = false

	// Every set can be picked for every sex the portrait can be.
	if choices := playing.choices(portrait.sex); len(choices) == 0 || choices[0] != (animationChoice{set: defaultAnimation}) {
		t.Errorf("choices = %v, want %s first", choices, defaultAnimation)
	}

	playing.none = true
	playing.restart(portrait.sex)
	application.Step()

	if _, _, _, ok := playing.body.Playing(); ok {
		t.Error("the body plays something with no animation picked")
	}

	playing.none = false
	playing.restart(portrait.sex)
	application.Step()
}

// Images dropped onto the window open as references in windows of their own,
// a file that is no image is refused in red, and a closed window lets go of
// its picture.
func TestReferences(t *testing.T) {
	application, _ := startApp(t, "")
	defer application.Close()

	dir := t.TempDir()
	photo := filepath.Join(dir, "photo.png")
	writeImage(t, photo, 64, 48)

	application.dropFiles([]string{photo, photo, filepath.Join(dir, "notes.txt")})

	if !application.state.statusError || !strings.Contains(application.state.status, "notes.txt") {
		t.Errorf("status %q, want notes.txt refused in red", application.state.status)
	}

	deadline := time.Now().Add(10 * time.Second)
	for len(application.references.shown) < 2 && time.Now().Before(deadline) {
		application.Step()
	}

	shown := application.references.shown
	if len(shown) != 2 || shown[0].title() == shown[1].title() {
		t.Fatalf("%d references, want the picture twice in windows of their own", len(shown))
	}

	application.Step()

	if _, ok := gui.FindWindow(shown[0].title()); !ok {
		t.Errorf("no window %q", shown[0].title())
	}

	// Escape closes the reference that was touched last, and it is let go of
	// a frame later.
	application.state.focusedWindow = shown[1].title()
	application.closeFocusedWindow()
	application.Step()
	application.Step()

	if len(application.references.shown) != 1 || application.references.shown[0] != shown[0] {
		t.Errorf("after closing one, %d references are left", len(application.references.shown))
	}

	// View > Open Reference asks for an image and opens it as a dropped one
	// is opened, and the next time asks in the same folder.
	dialogs := &fakeDialogs{answers: []string{photo}}
	application.dialogs = dialogs

	application.openReference()
	answerDialog(t, application)

	for len(application.references.shown) < 2 && time.Now().Before(deadline) {
		application.Step()
	}

	if len(application.references.shown) != 2 {
		t.Errorf("%d references after opening one, want 2", len(application.references.shown))
	}

	if asked := dialogs.lastAsked(); asked.save || len(asked.filter.patterns) != len(referenceExtensions) {
		t.Errorf("the dialog asked %+v, want an image to open", asked)
	}

	application.openReference()
	answerDialog(t, application)

	if asked := dialogs.lastAsked(); filepath.Clean(asked.start) != dir {
		t.Errorf("the second dialog opens at %q, want %q", asked.start, dir)
	}
}

// An animation variant that holds a prop has it hung from its joint, a sabre
// from the hand and a pipe from the lips, and the default of the set lets go
// of it again.
func TestAnimationProps(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	application, _ := startApp(t, fmt.Sprintf(`{"Folders":[{"Name":"game","Path":%q}],"InterfaceScale":1}`, game))
	defer application.Close()

	waitForPortrait(t, application)
	application.state.welcome = false

	p := application.portrait
	playing := &p.animation

	settle := func() {
		t.Helper()

		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			application.Step()

			if !p.rebuilding && !p.character.partsChanged && len(p.missingProps()) == 0 && len(p.props) == len(p.wantedProps) {
				return
			}
		}

		t.Fatal("the props were not read in time")
	}

	for _, test := range []struct{ variant, entity, joint string }{
		{"saber_pop_idle", "prop_saber_entity", "bn_l_prop"},
		{"smoker_character_idle", "", "bn_h_mouth_"},
	} {
		playing.chosen, playing.variant = "idle", test.variant
		playing.restart(p.sex)
		settle()

		if len(p.props) != 1 {
			t.Fatalf("%s holds %d props, want 1", test.variant, len(p.props))
		}

		prop := p.props[0]
		if (test.entity != "" && prop.entity != test.entity) || !strings.HasPrefix(prop.joint, test.joint) || !prop.model.Hanging() {
			t.Errorf("%s holds %s from %s, hanging %v; want %s from %s", test.variant, prop.entity, prop.joint, prop.model.Hanging(), test.entity, test.joint)
		}

		if models := p.models(); len(models) != 3 || models[2] != prop.model {
			t.Errorf("%s: the portrait is drawn with %d models, want the prop among them", test.variant, len(models))
		}
	}

	playing.variant = ""
	playing.restart(p.sex)
	settle()

	if len(p.props) != 0 || len(p.models()) != 2 {
		t.Errorf("the default of idle holds %d props", len(p.props))
	}

	// The props gene is listed, and a cane picked in it is held from the
	// right hand.
	var prop *accessoryGene
	for index := range p.character.accessories {
		if p.character.accessories[index].gene.Key == "props" {
			prop = &p.character.accessories[index]
		}
	}

	if prop == nil || prop.hidden || categories[prop.category].id != "props" {
		t.Fatalf("the props gene is %+v, want it listed under Props", prop)
	}

	prop.template = slices.IndexFunc(prop.templates, func(template genes.AccessoryTemplate) bool { return template.Name == "gene_walkingcane_01" })
	prop.value = 255
	p.character.partsChanged = true
	settle()

	if len(p.props) != 1 || p.props[0].entity != "prop_walkingcane_entity" || p.props[0].joint != "bn_r_prop" {
		t.Errorf("a cane picked holds %+v, want the cane from the right hand", p.props)
	}

	// An animation's prop is held beside the one the DNA picks, as the game
	// holds them.
	playing.variant = "saber_pop_idle"
	playing.restart(p.sex)
	settle()

	held := []string{}
	for _, hung := range p.props {
		held = append(held, hung.entity)
	}

	slices.Sort(held)

	if !slices.Equal(held, []string{"prop_saber_entity", "prop_walkingcane_entity"}) || len(p.models()) != 4 {
		t.Errorf("with a sabre played and a cane picked, the portrait holds %v, want both", held)
	}
}
