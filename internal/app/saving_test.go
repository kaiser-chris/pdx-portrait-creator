//go:build uitest

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// fakeDialogs answers the file dialogs with paths given in advance, and keeps
// what each dialog asked.
type fakeDialogs struct {
	mutex   sync.Mutex
	answers []string
	asked   []fileRequest
}

func (f *fakeDialogs) choose(request fileRequest) (string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.asked = append(f.asked, request)

	if len(f.answers) == 0 {
		return "", nil
	}

	answer := f.answers[0]
	f.answers = f.answers[1:]

	return answer, nil
}

// lastAsked is the request of the last dialog shown.
func (f *fakeDialogs) lastAsked() fileRequest {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.asked) == 0 {
		return fileRequest{}
	}

	return f.asked[len(f.asked)-1]
}

// answerDialog steps the application until the dialog it opened has been
// answered.
func answerDialog(t *testing.T, application *App) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for application.state.dialog != nil {
		if time.Now().After(deadline) {
			t.Fatal("the dialog was not answered")
		}

		application.Step()
	}
}

// readDNAFile reads the DNA a file defines, and fails on anything the parser
// had to read past.
func readDNAFile(t *testing.T, path string) (string, map[string]*genes.DNA) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	document := script.Parse(string(data))
	for _, warning := range document.Warnings {
		t.Errorf("%s: %s", path, warning)
	}

	table := genes.ReadDNA(document, folders.File{Relative: "common/dna_data/test.txt", Path: path, Source: "mod"}, &report.Collector{})

	found := map[string]*genes.DNA{}
	for _, dna := range table.All() {
		found[dna.Key] = dna
	}

	return string(data), found
}

// A character of the game is saved into the mod, not over the game's file,
// with what the creator does not edit of it kept; saved again it takes its own
// place in that file; under a new identifier it is renamed or added beside
// itself as the user answers; and a portrait from scratch is saved into a file
// of the user's choosing, after what is in it.
func TestSavingDNA(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	mod := t.TempDir()
	settings := fmt.Sprintf(`{"Folders":[{"Name":"game","Path":%q},{"Name":"mod","Path":%q}],"InterfaceScale":1}`, game, mod)

	application, _ := startApp(t, settings)
	defer application.Close()

	dialogs := &fakeDialogs{}
	application.dialogs = dialogs

	waitForPortrait(t, application)

	p := application.portrait

	// Nothing to save while the welcome screen is up.
	if application.canSave() {
		t.Error("Save is offered over the welcome screen")
	}

	application.state.picker.kind = pickCharacters
	application.pick("dna_abraham_lincoln")
	waitForHead(t, application)
	application.closePicker(true)
	application.state.welcome = false

	original, _ := p.characters.Get("dna_abraham_lincoln")

	if p.basis.identifier != "dna_abraham_lincoln" || p.basis.file == nil || !within(game, p.basis.file.path) {
		t.Fatalf("basis %+v, want the character's identifier and its file in the game", p.basis)
	}

	// A gene changed, and saved: the game's file is not written over, the
	// dialog opens at the same place in the mod instead.
	nose := morphOf(t, p.character, "gene_nose_size")
	nose.value = 200
	p.character.changed = true
	application.Step()

	target := filepath.Join(mod, "common", "dna_data", filepath.Base(p.basis.file.path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}

	dialogs.answers = []string{target}
	application.save()
	answerDialog(t, application)

	if asked := dialogs.lastAsked(); asked.start != target || !asked.save {
		t.Errorf("the dialog asked %+v, want a save dialog at %s", asked, target)
	}

	_, saved := readDNAFile(t, target)

	lincoln, ok := saved["dna_abraham_lincoln"]
	if !ok {
		t.Fatalf("the mod's file defines %v, want dna_abraham_lincoln", saved)
	}

	written, _ := lincoln.Gene("gene_nose_size")
	read, _ := original.Gene("gene_nose_size")

	if written.Dominant.Value != 200 || written.Recessive != read.Recessive {
		t.Errorf("the nose was written as %+v, want 200 over the recessive %+v the game's file has", written, read.Recessive)
	}

	if len(lincoln.Genes) < len(original.Genes) {
		t.Errorf("%d genes written, the game's file has %d", len(lincoln.Genes), len(original.Genes))
	}

	if !samePath(p.basis.file.path, target) {
		t.Errorf("the DNA is kept in %s after saving, want %s", p.basis.file.path, target)
	}

	if application.state.statusError {
		t.Errorf("a save that worked reports %q in red", application.state.status)
	}

	// Saved again into its own file, it takes its own place, and what was
	// written around it by hand stays.
	contents, _ := os.ReadFile(target)
	if err := os.WriteFile(target, append([]byte("# kept by hand\n"), contents...), 0o644); err != nil {
		t.Fatal(err)
	}

	nose.value = 30
	application.save()
	application.Step()

	text, saved := readDNAFile(t, target)
	if written, _ := saved["dna_abraham_lincoln"].Gene("gene_nose_size"); written.Dominant.Value != 30 || !strings.HasPrefix(text, "# kept by hand") || strings.Count(text, "dna_abraham_lincoln =") != 1 {
		t.Errorf("saved again:\n%s", text)
	}

	// Under a new identifier, saving asks what to do with the DNA it came
	// from; added as new, both are there.
	p.basis.identifier = "dna_abraham_lincoln_old"
	application.save()

	if !application.state.renameAsked {
		t.Fatal("saving under a new identifier asked nothing")
	}

	application.Step()
	application.writeDNA(target, p.basis.identifier)

	if _, saved := readDNAFile(t, target); saved["dna_abraham_lincoln"] == nil || saved["dna_abraham_lincoln_old"] == nil {
		t.Errorf("added as new, the file defines %v, want both", keys(saved))
	}

	// Renamed, the DNA it was read as takes the new identifier.
	p.basis.identifier = "dna_abraham_lincoln_young"
	application.save()
	application.Step()
	application.writeDNA(target, p.basis.file.key)

	if _, saved := readDNAFile(t, target); saved["dna_abraham_lincoln_old"] != nil || saved["dna_abraham_lincoln_young"] == nil || saved["dna_abraham_lincoln"] == nil {
		t.Errorf("renamed, the file defines %v, want dna_abraham_lincoln and dna_abraham_lincoln_young", keys(saved))
	}

	// The picker lists what was saved.
	if _, ok := p.characters.Get("dna_abraham_lincoln_young"); !ok {
		t.Error("the saved DNA is not among the characters")
	}

	// From scratch there is no identifier, and nothing is saved without one.
	application.fromScratch()
	waitForHead(t, application)

	if p.basis.identifier != "" || p.basis.file != nil {
		t.Errorf("from scratch the basis is %+v, want no identifier and no file", p.basis)
	}

	asked := len(dialogs.asked)
	application.save()

	if len(dialogs.asked) != asked || application.state.dialog != nil {
		t.Error("a DNA without an identifier asked where to save it")
	}

	// It says why in red, and the field blinks: red at once, and no longer
	// after a few seconds.
	if !application.state.statusError || !application.identifierAlerted() {
		t.Errorf("refused for no identifier: status %q in red %v, field red %v", application.state.status, application.state.statusError, application.identifierAlerted())
	}

	application.state.identifierAlert = time.Now().Add(-blinkFor)
	if application.identifierAlerted() {
		t.Error("the identifier field still blinks after a few seconds")
	}

	// The error goes away after a while; an error that just came stays.
	application.Step()

	if application.state.status == "" {
		t.Error("the error went away at once")
	}

	application.state.statusSince = time.Now().Add(-errorShownFor)
	application.Step()

	if application.state.status != "" || application.state.statusError {
		t.Errorf("the error %q is still shown after %v", application.state.status, errorShownFor)
	}

	// With one, it asks for a file, and goes after what is in it.
	other := filepath.Join(mod, "common", "dna_data", "other.txt")
	if err := os.WriteFile(other, []byte("dna_someone = {\n\tenabled = no\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p.basis.identifier = "dna_my_character"
	dialogs.answers = []string{other}
	application.save()
	answerDialog(t, application)

	if asked := dialogs.lastAsked(); asked.start != filepath.Join(mod, "common", "dna_data", "dna_my_character.txt") {
		t.Errorf("the dialog opened at %s, want the mod's DNA folder", asked.start)
	}

	text, saved = readDNAFile(t, other)
	if saved["dna_someone"] == nil || saved["dna_my_character"] == nil || !strings.HasPrefix(text, "dna_someone") {
		t.Errorf("saved after what was there:\n%s", text)
	}

	// Save To File asks again, at the file the DNA is kept in now.
	application.saveToFile()
	answerDialog(t, application)

	if asked := dialogs.lastAsked(); !samePath(asked.start, other) {
		t.Errorf("Save To File opened at %s, want %s", asked.start, other)
	}
}

// morphOf is the morph gene of a key.
func morphOf(t *testing.T, c *character, key string) *morphGene {
	t.Helper()

	for index := range c.morphs {
		if c.morphs[index].gene.Key == key {
			return &c.morphs[index]
		}
	}

	t.Fatalf("no morph gene %s", key)

	return nil
}

func keys(found map[string]*genes.DNA) []string {
	var listed []string
	for key := range found {
		listed = append(listed, key)
	}

	return listed
}
