//go:build uitest

package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

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

	// Victoria 3's Lincoln, or the first character of a game without him,
	// and a gene of the nose either game has.
	character, noseGene := "dna_abraham_lincoln", "gene_nose_size"
	if !p.characters.Has(character) {
		keys := p.characters.Keys()
		slices.Sort(keys)
		character, noseGene = keys[0], "gene_nose_width"
	}

	application.state.picker.kind = pickCharacters
	application.pick(character)
	waitForHead(t, application)
	application.closePicker(true)
	application.state.welcome = false

	original, _ := p.characters.Get(character)

	if p.basis.identifier != character || p.basis.file == nil || !within(game, p.basis.file.path) {
		t.Fatalf("basis %+v, want the character's identifier and its file in the game", p.basis)
	}

	// A gene changed, and saved: the game's file is not written over, the
	// dialog opens at the same place in the mod instead.
	nose := morphOf(t, p.character, noseGene)
	nose.value = 200
	p.character.changed = true
	application.Step()

	// The mod's file is at the place of the game's, layer and all.
	relative, err := filepath.Rel(game, p.basis.file.path)
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(mod, relative)
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

	kept, ok := saved[character]
	if !ok {
		t.Fatalf("the mod's file defines %v, want %s", saved, character)
	}

	written, _ := kept.Gene(noseGene)
	read, _ := original.Gene(noseGene)

	if written.Dominant.Value != 200 || written.Recessive != read.Recessive {
		t.Errorf("the nose was written as %+v, want 200 over the recessive %+v the game's file has", written, read.Recessive)
	}

	if len(kept.Genes) < len(original.Genes) {
		t.Errorf("%d genes written, the game's file has %d", len(kept.Genes), len(original.Genes))
	}

	// What else the game's file says of the DNA is kept, such as the
	// priority and tags of Europa Universalis 5's.
	if kept.Priority != original.Priority || !slices.Equal(kept.Tags, original.Tags) {
		t.Errorf("written with priority %v and tags %v, the game's file has %v and %v", kept.Priority, kept.Tags, original.Priority, original.Tags)
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
	// The comment goes after the byte order mark a new file starts with,
	// which belongs at the very start.
	contents = bytes.TrimPrefix(contents, []byte(string(rune(0xFEFF))))
	if err := os.WriteFile(target, append([]byte("# kept by hand\n"), contents...), 0o644); err != nil {
		t.Fatal(err)
	}

	nose.value = 30
	application.save()
	application.Step()

	text, saved := readDNAFile(t, target)
	if written, _ := saved[character].Gene(noseGene); written.Dominant.Value != 30 || !strings.HasPrefix(text, "# kept by hand") || strings.Count(text, character+" =") != 1 {
		t.Errorf("saved again:\n%s", text)
	}

	// Each save puts the DNA string on the line above the definition, and
	// leaves the strings of earlier saves above it.
	before, _, _ := strings.Cut(text, character+" = {")
	comments := strings.Split(strings.TrimSpace(before), "\n")

	if len(comments) != 3 || !strings.HasPrefix(comments[1], "# ") || !strings.HasPrefix(comments[2], "# ") || comments[1] == comments[2] {
		t.Fatalf("above the definition:\n%s\nwant the comment kept by hand and the strings of two saves", before)
	}

	fromString, problems := p.genes.ReadString(strings.TrimPrefix(comments[2], "# "))
	if nose, _ := fromString.Gene(noseGene); len(problems) > 0 || nose.Dominant.Value != 30 {
		t.Errorf("the string above the definition reads as a nose of %+v, %v; want 30", nose, problems)
	}

	// Under a new identifier, saving asks what to do with the DNA it came
	// from; added as new, both are there.
	p.basis.identifier = character + "_old"
	application.save()

	if !application.state.renameAsked {
		t.Fatal("saving under a new identifier asked nothing")
	}

	application.Step()
	application.writeDNA(target, p.basis.identifier)

	if _, saved := readDNAFile(t, target); saved[character] == nil || saved[character+"_old"] == nil {
		t.Errorf("added as new, the file defines %v, want both", keys(saved))
	}

	// Renamed, the DNA it was read as takes the new identifier.
	p.basis.identifier = character + "_young"
	application.save()
	application.Step()
	application.writeDNA(target, p.basis.file.key)

	if _, saved := readDNAFile(t, target); saved[character+"_old"] != nil || saved[character+"_young"] == nil || saved[character] == nil {
		t.Errorf("renamed, the file defines %v, want %s and its young version", keys(saved), character)
	}

	// Copy Script puts the same DNA on the clipboard, as the file has it.
	previous := rl.GetClipboardText()
	t.Cleanup(func() { rl.SetClipboardText(previous) })

	application.copyScript()

	copied := rl.GetClipboardText()
	if text, _ := readDNAFile(t, target); !strings.Contains(text, copied) || !strings.Contains(copied, "\n"+character+"_young = {") {
		t.Errorf("copied:\n%s\nwhich the file does not hold as it was saved", copied)
	}

	// The welcome screen finds each of the three on the clipboard; started
	// from the persistent DNA, the portrait is as it was and has its key.
	if found := readPasted(copied, p.genes); found.kind != pastedDefinition || found.key != character+"_young" {
		t.Errorf("the script is found as %+v", found)
	}

	application.copyDNAString()

	if found := readPasted(rl.GetClipboardText(), p.genes); found.kind != pastedString || !found.usable() {
		t.Errorf("the DNA string is found as %+v", found)
	}

	application.copyPersistentDNA()

	found := readPasted(rl.GetClipboardText(), p.genes)
	if found.kind != pastedPersistent || found.key != character+"_young" || found.dna.Sex != p.sex.String() {
		t.Fatalf("the persistent DNA is found as %+v", found)
	}

	application.state.clipboard = clipboardState{}
	application.fromClipboard()
	application.Step()

	if p.basis.kind != basisClipboard || p.basis.identifier != character+"_young" || p.basis.file != nil {
		t.Errorf("started from the clipboard, the basis is %+v", p.basis)
	}

	if nose := morphOf(t, p.character, noseGene); nose.value != 30 {
		t.Errorf("started from the clipboard, the nose is %v, want 30", nose.value)
	}

	// The picker lists what was saved.
	if _, ok := p.characters.Get(character + "_young"); !ok {
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
	other := filepath.Join(filepath.Dir(target), "other.txt")
	if err := os.WriteFile(other, []byte("dna_someone = {\n\tenabled = no\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p.basis.identifier = "dna_my_character"
	dialogs.answers = []string{other}
	application.save()
	answerDialog(t, application)

	if asked := dialogs.lastAsked(); asked.start != filepath.Join(filepath.Dir(target), "dna_my_character.txt") {
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
