package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	imgui "github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// Labels of the File menu's save actions and of the question a renamed DNA
// asks.
const (
	labelSave           = "Save"
	labelSaveToFile     = "Save To File..."
	labelCopyScript     = "Copy Script"
	labelCopyString     = "Copy DNA String"
	labelCopyPersistent = "Copy Persistent DNA"
	labelRenameDNA      = "Rename"
	labelAddNewDNA      = "Add as New DNA"
	labelCancelSave     = "Cancel"

	popupRename = "Save Under a New Name"
)

// dnaFile is where a portrait's DNA is kept: the file it was read from or last
// saved to, the configured folder that file is in, the key it is defined under
// there, and the DNA as it was read or saved.
type dnaFile struct {
	path   string
	source string
	key    string

	// original is the DNA as the file has it. What the creator does not edit
	// of it, the recessive alleles and genes it does not know, is written back
	// as it was.
	original *genes.DNA
}

// canSave reports whether there is a portrait's DNA to save: one is being
// made, rather than worn as the base of a portrait modifier project.
func (a *App) canSave() bool {
	return a.editing() && a.state.mode == modeCharacter
}

// editing reports whether the editor is open on a portrait, past the welcome
// screen and with no file dialog over it.
func (a *App) editing() bool {
	p := a.portrait

	return p != nil && p.loaded() && !a.state.welcome && a.state.dialog == nil
}

// save writes the portrait's DNA back into the file it came from. A DNA that
// is in no file yet, one of an ethnicity or from scratch, is saved the way
// Save To File does it, and so is one in the game's own folder, which is not
// written over.
func (a *App) save() {
	if !a.canSave() || !a.checkIdentifier() {
		return
	}

	file := a.portrait.basis.file
	if file == nil || a.inGameFolder(file.path) {
		a.saveToFile()

		return
	}

	a.saveTo(file.path)
}

// saveToFile asks for a DNA file and puts the portrait's DNA into it: in place
// of a DNA of the same identifier if the file has one, at its end otherwise. A
// file that does not exist yet is created. The DNA belongs to that file from
// then on.
func (a *App) saveToFile() {
	if !a.canSave() || !a.checkIdentifier() {
		return
	}

	identifier := a.portrait.basis.identifier
	request := fileRequest{
		title:  "Save " + identifier + " To File",
		start:  a.dnaStart(),
		save:   true,
		filter: dnaFiles,
	}

	a.ask(request, func(path string) {
		a.saveTo(withExtension(path, ".txt"))
	})
}

// saveTo puts the portrait's DNA into a file.
//
// A DNA whose identifier was changed since it was read poses a question when
// it goes back into its own file: does the DNA it came from take the new
// identifier, or does the portrait join it as a DNA of its own? Making a
// variant of a character means keeping both, so the answer is the user's.
func (a *App) saveTo(path string) {
	b := &a.portrait.basis

	switch {
	case b.file == nil || !samePath(path, b.file.path):
		// In any other file the DNA simply goes by its identifier.
		a.writeDNA(path, b.identifier)
	case b.identifier == b.file.key:
		a.writeDNA(path, b.file.key)
	default:
		a.state.renameAsked = true
	}
}

// writeDNA puts the portrait's DNA into a file under its identifier, in place
// of the DNA defined there under replace.
func (a *App) writeDNA(path, replace string) {
	b := &a.portrait.basis
	identifier := b.identifier

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		a.saveFailed(identifier, err)

		return
	}

	dna := a.exportDNA()

	merged, replaced, err := script.PutDefinition(string(existing), replace, identifier, a.definition(identifier, dna))
	if err != nil {
		a.saveFailed(identifier, err)

		return
	}

	if err := writeFileAtomically(path, []byte(merged)); err != nil {
		a.saveFailed(identifier, err)

		return
	}

	b.file = &dnaFile{path: path, source: a.folderOf(path), key: identifier, original: dna}

	verb := "Added"
	if replaced {
		verb = "Saved"
	}

	a.setStatus("%s %s to %s", verb, identifier, filepath.Base(path))
	a.reloadCharacters()
}

// copyScript puts the portrait's DNA on the clipboard as a file of
// common/dna_data would define it, to paste into a file by hand or into one
// the creator does not write, such as one open in an editor.
func (a *App) copyScript() {
	if !a.canSave() || !a.checkIdentifier() {
		return
	}

	identifier := a.portrait.basis.identifier

	rl.SetClipboardText(a.definition(identifier, a.exportDNA()))
	a.setStatus("Copied the script of %s", identifier)
}

// copyDNAString puts the portrait's DNA on the clipboard as a DNA string. It
// needs no identifier, since a string names nothing.
func (a *App) copyDNAString() {
	if !a.canSave() {
		return
	}

	rl.SetClipboardText(a.dnaString(a.exportDNA()))
	a.setStatus("Copied the DNA string")
}

// copyPersistentDNA puts the portrait's DNA on the clipboard as the game
// writes it out, with the sex and age it is shown at.
func (a *App) copyPersistentDNA() {
	if !a.canSave() || !a.checkIdentifier() {
		return
	}

	p := a.portrait
	identifier := p.basis.identifier

	dna := a.exportDNA()
	dna.Sex = p.sex.String()
	dna.Age = database.Some(p.character.at())

	rl.SetClipboardText(genes.WritePersistentDNA(identifier, dna, "\n"))
	a.setStatus("Copied the persistent DNA of %s", identifier)
}

// definition is a DNA as a definition of common/dna_data, with its DNA string
// in a comment on the line above, as the game's own files have it.
//
// The string is only written, never read back: a DNA saved again into its
// file is given a new comment above it, and the comments of earlier saves
// stay where they were, a history of what the DNA was.
func (a *App) definition(identifier string, dna *genes.DNA) string {
	return "# " + a.dnaString(dna) + "\n" + genes.WriteDNA(identifier, dna, "\n")
}

// dnaString is a DNA as a DNA string of the genes the portrait was read
// with.
func (a *App) dnaString(dna *genes.DNA) string {
	text, problems := a.portrait.genes.WriteString(dna)

	// A gene the DNA does not give, or one the string has no place for, is
	// no reason not to write it; the string just holds what it can.
	for _, problem := range problems.Filter(report.SeverityWarning) {
		warn(errors.New(problem.String()))
	}

	return text
}

func (a *App) saveFailed(identifier string, err error) {
	warn(err)
	a.setError("%s could not be saved: %v", identifier, err)
}

// checkIdentifier refuses to save a DNA under an identifier the game could
// not read back as the key of a definition, says why in red, and has the
// identifier field blink.
func (a *App) checkIdentifier() bool {
	identifier := a.portrait.basis.identifier

	switch {
	case identifier == "":
		a.setError("Give the DNA an identifier in the field at the top left first, such as dna_my_character")
	case !script.IsKey(identifier):
		a.setError("%q cannot be saved: an identifier starts with a letter or an underscore and has no spaces", identifier)
	default:
		return true
	}

	a.state.identifierAlert = time.Now()

	return false
}

// exportDNA is the portrait as the DNA to write: the dominant alleles the
// creator shows, laid over the DNA the file had, so that the order of its
// genes, their recessive alleles and any gene the creator does not know are
// written back as they were. A DNA new to a file has every gene's recessive
// allele as its dominant one. The priority and tags of a DNA of Europa
// Universalis 5 are kept too, and a new one is given a priority of 1, as
// the game's own have, so that it is written as common/persistent_dna holds
// its DNA.
func (a *App) exportDNA() *genes.DNA {
	edited := a.portrait.character.dna()

	file := a.portrait.basis.file
	if file == nil || file.original == nil {
		if a.portrait.persistent() {
			edited.Priority = database.Some(1.0)
		}

		return edited
	}

	shown := make(map[string]genes.Allele, len(edited.Genes))
	for _, gene := range edited.Genes {
		shown[gene.Gene] = gene.Dominant
	}

	merged := &genes.DNA{Enabled: file.original.Enabled, Priority: file.original.Priority, Tags: file.original.Tags}

	for _, gene := range file.original.Genes {
		if dominant, ok := shown[gene.Gene]; ok {
			gene.Dominant = dominant
			delete(shown, gene.Gene)
		}

		merged.Genes = append(merged.Genes, gene)
	}

	for _, gene := range edited.Genes {
		if _, ok := shown[gene.Gene]; ok {
			merged.Genes = append(merged.Genes, gene)
		}
	}

	return merged
}

// reloadCharacters reads the DNA of the folders again, so that the picker
// lists what was just saved.
func (a *App) reloadCharacters() {
	sources := make([]folders.Source, 0, len(a.settings.Folders))
	for _, folder := range a.settings.Folders {
		sources = append(sources, folders.Source{Name: folder.Name, Path: folder.Path})
	}

	characters, _ := genes.LoadDNA(folders.Open(sources))
	a.portrait.characters = characters
}

// dnaStart is where the save dialog opens: at the file the DNA is kept in, or
// for one in the game's own folder at the same place in the last configured
// folder, the mod being worked on, and for a DNA in no file yet in the DNA
// folder of that last folder, under the DNA's identifier.
func (a *App) dnaStart() string {
	b := a.portrait.basis

	if b.file != nil {
		if mirrored := a.inModFolder(b.file.path); mirrored != "" {
			return mirrored
		}

		return b.file.path
	}

	folder := a.workingFolder()
	if folder == "" {
		return b.identifier + ".txt"
	}

	return filepath.Join(folder, b.identifier+".txt")
}

// workingFolder is the folder of the last configured folder the game keeps
// its characters in, directly or in one of its layers, such as
// in_game/common/persistent_dna, or that folder itself when it has none yet.
func (a *App) workingFolder() string {
	configured := a.settings.Folders
	if len(configured) == 0 {
		return ""
	}

	root := configured[len(configured)-1].Path
	folder := filepath.FromSlash(a.portrait.characterFolder)

	candidates := []string{filepath.Join(root, folder)}
	if layered, err := filepath.Glob(filepath.Join(root, "*", folder)); err == nil {
		candidates = append(candidates, layered...)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return root
}

// inGameFolder reports whether a file is in the game's own folder, the first
// of several configured folders, which the creator does not write over
// without asking where to: the mods after it are where changes belong.
func (a *App) inGameFolder(path string) bool {
	configured := a.settings.Folders

	return len(configured) > 1 && within(configured[0].Path, path)
}

// inModFolder is where a file of the game's own folder would be in the last
// configured folder, or nothing for a file that is not the game's.
func (a *App) inModFolder(path string) string {
	if !a.inGameFolder(path) {
		return ""
	}

	configured := a.settings.Folders

	relative, err := filepath.Rel(configured[0].Path, path)
	if err != nil {
		return ""
	}

	return filepath.Join(configured[len(configured)-1].Path, relative)
}

// folderOf names the configured folder a file is in. A file in several, a
// mod kept inside the game folder say, belongs to the last, as it would when
// the folders are read.
func (a *App) folderOf(path string) string {
	found := ""

	for _, folder := range a.settings.Folders {
		if within(folder.Path, path) {
			found = folder.Name
		}
	}

	return found
}

// renamePopup asks what a DNA whose identifier was changed since it was read
// does to the DNA it came from: take its place under the new identifier, or
// leave it alone and join it in the file.
func (a *App) renamePopup() {
	if a.state.renameAsked {
		imgui.OpenPopupStr(popupRename)
		a.state.renameAsked = false
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(460, 0), imgui.CondAlways)

	if !imgui.BeginPopupModalV(popupRename, nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	b := a.portrait.basis
	if b.file == nil || b.identifier == b.file.key {
		imgui.CloseCurrentPopup()

		return
	}

	imgui.TextWrapped(fmt.Sprintf("%s was read as %s from %s.", b.identifier, b.file.key, filepath.Base(b.file.path)))
	imgui.TextWrapped(fmt.Sprintf("%s gives that DNA the new identifier. %s leaves %s as it is and writes %s beside it.",
		labelRenameDNA, labelAddNewDNA, b.file.key, b.identifier))

	imgui.Spacing()

	if imgui.Button(labelRenameDNA) {
		a.writeDNA(b.file.path, b.file.key)
		imgui.CloseCurrentPopup()
	}

	imgui.SameLine()

	// Enter takes the choice that leaves the DNA in the file as it was.
	if imgui.Button(labelAddNewDNA) || imgui.IsKeyPressedBool(imgui.KeyEnter) {
		a.writeDNA(b.file.path, b.identifier)
		imgui.CloseCurrentPopup()
	}

	imgui.SameLine()

	if imgui.Button(labelCancelSave) || imgui.IsKeyPressedBool(imgui.KeyEscape) {
		imgui.CloseCurrentPopup()
	}
}

// within reports whether a path lies inside a folder.
func within(folder, path string) bool {
	if folder == "" || path == "" {
		return false
	}

	relative, err := filepath.Rel(folder, path)

	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// samePath reports whether two paths name the same file.
func samePath(first, second string) bool {
	if first == "" || second == "" {
		return false
	}

	relative, err := filepath.Rel(first, second)

	return err == nil && relative == "."
}

// withExtension adds an extension to a file name typed without one.
func withExtension(path, extension string) string {
	if filepath.Ext(path) == "" {
		return path + extension
	}

	return path
}

// writeFileAtomically writes a file so that it is never left half written:
// the contents go to a file next to it, which then takes its place.
func writeFileAtomically(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}

	// Removing it after a successful rename fails harmlessly.
	defer os.Remove(temporary.Name())

	// The file keeps the permissions it had; a new one gets the usual ones
	// rather than the private ones a temporary file is created with.
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()

		return err
	}

	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()

		return err
	}

	if err := temporary.Close(); err != nil {
		return err
	}

	return os.Rename(temporary.Name(), path)
}

// How the identifier field blinks after a save was refused for it: on and off
// every blinkPeriod, for blinkFor.
const (
	blinkFor    = 3 * time.Second
	blinkPeriod = 300 * time.Millisecond
)

// identifierAlerted reports whether the identifier field is drawn in red this
// frame: on and off for a few seconds after a save was refused for it, so
// that the eye finds it, and not at all once it is put right.
func (a *App) identifierAlerted() bool {
	since := time.Since(a.state.identifierAlert)
	if a.state.identifierAlert.IsZero() || since >= blinkFor || script.IsKey(a.portrait.basis.identifier) {
		return false
	}

	return since/blinkPeriod%2 == 0
}

// characterFolderOf is the folder below a game or mod folder the game keeps
// the DNA of its characters in: common/persistent_dna where the folders have
// one, as Europa Universalis 5 does, and common/dna_data otherwise, as
// Victoria 3 does.
func characterFolderOf(set *folders.Set) string {
	if files, _ := set.Files(genes.FolderPersistentDNA); len(files) > 0 {
		return genes.FolderPersistentDNA.Path
	}

	return genes.FolderDNA.Path
}

// persistent says whether the game keeps its characters in
// common/persistent_dna, whose DNA is written with a priority.
func (p *portrait) persistent() bool {
	return p.characterFolder == genes.FolderPersistentDNA.Path
}
