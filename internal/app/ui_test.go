//go:build uitest

package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kaiser-chris/pdx-asset-go/render"
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

	// The genes are listed, set as the ethnicity previews neutral, as the
	// game's portrait editor does, and most of them morph what is drawn.
	portrait := application.portrait
	if portrait.ethnicity == nil || portrait.ethnicity.Key != portraitEthnicity {
		t.Errorf("ethnicity = %+v, want %s", portrait.ethnicity, portraitEthnicity)
	}

	for _, morph := range portrait.morphs.genes {
		if morph.value != 127 || morph.template != 0 && morph.gene.Templates[morph.template].Index != 0 {
			t.Errorf("%s is at %s %d, want its first template at 127", morph.gene.Key, morph.gene.Templates[morph.template].Name, morph.value)
		}
	}

	morphing := 0

	var nose *morphGene

	for index := range portrait.morphs.genes {
		morph := &portrait.morphs.genes[index]
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

	if len(portrait.morphs.genes) < 50 || morphing < len(portrait.morphs.genes)/3 || nose == nil || !nose.drawn {
		t.Fatalf("%d morph genes, %d of them drawn, the nose %+v", len(portrait.morphs.genes), morphing, nose)
	}

	// Changing a gene changes the picture.
	before := portrait.viewer.Image()

	// To the other end from where the character has it.
	if nose.value < maxGeneValue/2 {
		nose.value = maxGeneValue
	} else {
		nose.value = 0
	}

	portrait.morphs.changed = true
	application.Step()

	if changed := differingPixels(before, portrait.viewer.Image()); changed == 0 {
		t.Error("changing the size of the nose changed nothing drawn")
	}

	// Turning the camera turns the body.
	portrait.viewer.Rotate(1, 0)
	application.Step()

	if portrait.viewer.Yaw != 1 {
		t.Errorf("yaw = %v after turning by 1", portrait.viewer.Yaw)
	}

	// The panels draw with the genes in them, and a filter that matches none.
	portrait.morphs.filter = "nose"
	application.Step()

	portrait.morphs.filter = "nothing at all"
	application.Step()

	agedDecals(t, application)
}

// agedDecals checks that the genes lay their decals over the face of an old
// man, wrinkles and crow's feet among them, with every texture they name,
// and that they are drawn. With PDX_DUMP_DIR set, the face is written out at
// half and at the whole of a life, to compare with the game's portrait
// editor.
func agedDecals(t *testing.T, application *App) {
	t.Helper()

	portrait := application.portrait
	portrait.apply(portrait.genes.Preview(portrait.ethnicity, portrait.preview))
	portrait.frameFace()

	dump := os.Getenv("PDX_DUMP_DIR")

	for _, age := range []int32{defaultAge, maxAge} {
		portrait.morphs.age = age
		portrait.morphs.changed = true
		application.Step()

		if dump == "" {
			continue
		}

		if err := writePNG(filepath.Join(dump, fmt.Sprintf("portrait_face_age_%d.png", age)), portrait.viewer.Image()); err != nil {
			t.Error(err)
		}
	}

	decals := portrait.morphs.decals(portrait.decalTextures)

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

		for _, morph := range portrait.morphs.genes {
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

	portrait.morphs.changed = true
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
