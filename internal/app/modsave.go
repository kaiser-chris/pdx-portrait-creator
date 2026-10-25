package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/script"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// A project saves one group at a time, wherever the person saving it says:
// the creator does not take it that they save into a mod.
//
// Where several groups have something unsaved, it first asks which to save.
// A group the project made is then saved into a file picked in the system's
// own dialog, at its end, or in the place of a definition of the same key
// the file has. A group that is in a file already is either appended to that
// file, its changed elements written where they stand and its new ones after
// the last, or saved as an inject, an INJECT: block of the group in a file
// picked in the dialog: into the block the file has for the group, or a new
// one at its end, or a new file.

// The titles of the dialogs of saving, which Dear ImGui knows the popups by.
const (
	popupSaveWhich = "Save a portrait modifier###savewhich"
	popupSaveHow   = "Save###savehow"
)

// labelSaveModifier is what saving a group is called in the menu and the
// Element window.
var labelSaveModifier = "Save Portrait Modifier..."

// modifierFiles are the script files portrait modifiers are kept in.
var modifierFiles = fileFilter{name: "Portrait modifiers", patterns: []string{"*.txt"}}

// injectPrefix makes a block an injection into the definition of its key.
const injectPrefix = "INJECT:"

// saveForm is where saving is: the dialog to open, the group picked to save,
// and whether it is appended to its file rather than injected.
type saveForm struct {
	openWhich, openHow bool
	group              string
	appendFile         bool
}

// canSaveModifier reports whether a project has anything to save.
func (a *App) canSaveModifier() bool {
	p := a.portrait

	return a.state.mode == modeModifiers && p != nil && p.loaded() && !a.hidden() && a.state.dialog == nil &&
		len(a.project.unsavedGroups()) > 0
}

// saveModifier saves a group of the project: the one there is, or the one
// picked of those there are.
func (a *App) saveModifier() {
	if !a.canSaveModifier() {
		return
	}

	unsaved := a.project.unsavedGroups()
	form := &a.state.workspace.save

	if len(unsaved) == 1 {
		a.saveGroup(unsaved[0].Key)

		return
	}

	*form = saveForm{openWhich: true, group: unsaved[0].Key}

	for _, group := range unsaved {
		if group.Key == a.project.group {
			form.group = group.Key
		}
	}
}

// saveGroup saves a group: a new one straight into a file to pick, another
// as its file or an inject, which is asked first.
func (a *App) saveGroup(key string) {
	group, ok := a.project.groups[key]
	if !ok {
		return
	}

	if group.fresh {
		a.askFile(group, false)

		return
	}

	a.state.workspace.save = saveForm{openHow: true, group: key, appendFile: false}
}

// definingFile is the file a group is defined in: the one the game read it
// from, or the one a group of the project was saved to.
func definingFile(group *projectGroup) string {
	if group.savedTo != "" {
		return group.savedTo
	}

	if len(group.Origins) > 0 {
		return group.Origins[0].Path
	}

	return ""
}

// saveModifierPopups are the dialogs of saving: which group to save, and how.
func (a *App) saveModifierPopups() {
	form := &a.state.workspace.save

	if form.openWhich {
		form.openWhich = false
		imgui.OpenPopupStr(popupSaveWhich)
	}

	if form.openHow {
		form.openHow = false
		imgui.OpenPopupStr(popupSaveHow)
	}

	a.saveWhichPopup()
	a.saveHowPopup()
}

// saveWhichPopup asks which of the groups with something unsaved to save.
func (a *App) saveWhichPopup() {
	imgui.SetNextWindowSizeV(gui.ScaledVec2(dialogWidth, 0), imgui.CondAlways)

	if !imgui.BeginPopupModalV(popupSaveWhich, nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	form := &a.state.workspace.save
	unsaved := a.project.unsavedGroups()

	gui.PushStrongFont()
	imgui.TextUnformatted("Which one do you want to save?")
	gui.PopFont()
	dimmedWrapped(fmt.Sprintf("%d portrait modifiers have unsaved changes. Each is saved on its own; the others keep theirs until they are saved too.", len(unsaved)))

	for _, group := range unsaved {
		if imgui.RadioButtonBool(group.Key+"##"+group.Key, form.group == group.Key) {
			form.group = group.Key
		}

		imgui.SameLine()
		imgui.TextDisabled(unsavedSummary(group))
	}

	imgui.Separator()

	if imgui.Button(gui.Iconed(gui.IconForward, "Next")) {
		imgui.CloseCurrentPopup()
		a.saveGroup(form.group)
	}

	imgui.SameLine()

	if imgui.Button("Cancel") {
		imgui.CloseCurrentPopup()
	}
}

// saveHowPopup asks whether a group that is in a file already is appended to
// that file or saved as an inject.
func (a *App) saveHowPopup() {
	imgui.SetNextWindowSizeV(gui.ScaledVec2(dialogWidth, 0), imgui.CondAlways)

	if !imgui.BeginPopupModalV(popupSaveHow, nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	form := &a.state.workspace.save

	group, ok := a.project.groups[form.group]
	if !ok {
		imgui.CloseCurrentPopup()

		return
	}

	file := definingFile(group)

	gui.PushStrongFont()
	imgui.TextUnformatted("How should " + group.Key + " be saved?")
	gui.PopFont()
	dimmedWrapped(unsavedSummary(group) + ".")

	imgui.Spacing()

	if imgui.RadioButtonBool("Append to its file", form.appendFile) {
		form.appendFile = true
	}

	imgui.Indent()
	dimmedWrapped(file)
	dimmedWrapped("Changed elements are written where they stand, new ones after the last.")

	if a.inGame(file) {
		gui.WarningText(gui.Iconed(gui.IconWarning, "This is a file of the game itself."))
		dimmedWrapped("The game's own file is changed, and an update or a check of the game's files puts it back.")
	}

	imgui.Unindent()
	imgui.Spacing()

	if imgui.RadioButtonBool("Save as an inject", !form.appendFile) {
		form.appendFile = false
	}

	imgui.Indent()
	dimmedWrapped(fmt.Sprintf("An %s%s block in a file you pick: added to the file's own block for %s where it has one, at its end otherwise, or a new file.", injectPrefix, group.Key, group.Key))
	imgui.Unindent()

	imgui.Separator()

	label := gui.Iconed(gui.IconSaveAs, "Choose File...")
	if form.appendFile {
		label = gui.Iconed(gui.IconSave, "Append")
	}

	if imgui.Button(label) {
		imgui.CloseCurrentPopup()

		if form.appendFile {
			a.writeGroup(group, file, false)
		} else {
			a.askFile(group, true)
		}
	}

	imgui.SameLine()

	if imgui.Button("Cancel") {
		imgui.CloseCurrentPopup()
	}
}

// askFile asks in the system's own dialog for the file to save a group into,
// as an inject or, for a group the project made, whole, and saves it there.
func (a *App) askFile(group *projectGroup, inject bool) {
	title := "Save the new portrait modifier " + group.Key
	name := group.Key + ".txt"

	if inject {
		title, name = "Save "+group.Key+" as an inject", ""
	}

	request := fileRequest{title: title, start: a.saveStart(name), save: true, filter: modifierFiles}

	a.ask(request, func(path string) {
		a.writeGroup(group, path, inject)
	})
}

// saveStart is where the dialog of saving a group opens: in the last of the
// folders, the one the others give way to, as far down the folder of the
// portrait modifiers as it has, so that a mod with gfx but no portraits
// folder yet opens at gfx; with a file's name given, at that file there.
func (a *App) saveStart(name string) string {
	if len(a.settings.Folders) == 0 {
		return ""
	}

	root := a.settings.Folders[len(a.settings.Folders)-1].Path
	folder := filepath.Join(root, filepath.FromSlash(genes.FolderModifiers.Path))

	for !isFolder(folder) {
		parent := filepath.Dir(folder)
		if parent == folder || !strings.HasPrefix(folder, root) {
			return ""
		}

		folder = parent
	}

	return filepath.Join(folder, name) + trailingSeparator(name)
}

// trailingSeparator ends a folder the dialog opens at with a separator, which
// tells it a folder from a file's name, and nothing for a file.
func trailingSeparator(name string) string {
	if name != "" {
		return ""
	}

	return string(filepath.Separator)
}

// isFolder reports whether a path is a folder there is.
func isFolder(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// sameFile reports whether two paths name the same file, whatever their case
// on Windows, where it does not matter.
func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

// writeGroup writes what is unsaved of a group into a file and marks it
// saved: a group the project made whole, the elements of another into the
// INJECT: block of it for an inject, or into its own definition otherwise.
// The file is made, with its folders, where there is none.
func (a *App) writeGroup(group *projectGroup, path string, inject bool) {
	merged, err := mergeGroup(a.project, a.portrait.character.looks.groups, group, path, inject)
	if err != nil {
		a.setError("%s could not be saved: %v", group.Key, err)

		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.setError("%s could not be saved: %v", group.Key, err)

		return
	}

	if err := os.WriteFile(path, []byte(merged), 0o644); err != nil {
		a.setError("%s could not be saved: %v", group.Key, err)

		return
	}

	group.saved(path)

	how := "into"
	if inject {
		how = "as an inject into"
	}

	a.setStatus("Saved %s %s %s", group.Key, how, path)
}

// mergeGroup is the contents of a file with what is unsaved of a group of a
// project put into it. drawn are the groups as the project draws them, whose
// weights written in the file are kept before it is written into.
func mergeGroup(pr *project, drawn []*genes.ModifierGroup, group *projectGroup, path string, inject bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	file := string(data)

	// The weights of the game's elements written in the file are read
	// before it is written into, which moves them.
	if err := pr.keepWeights(drawn, path); err != nil {
		return "", err
	}

	weights := map[string]string{}

	for index := range group.Modifiers {
		modifier := &group.Modifiers[index]

		weight, err := pr.weight(group, modifier)
		if err != nil {
			return "", fmt.Errorf("the weight of %s: %w", modifier.Name, err)
		}

		weights[modifier.Name] = weight
	}

	if group.fresh {
		written := genes.WriteModifierGroup(&group.ModifierGroup, func(modifier *genes.PortraitModifier) string { return weights[modifier.Name] })
		merged, _, err := script.PutDefinition(file, group.Key, group.Key, written)

		return merged, err
	}

	block := group.Key
	if inject {
		block = injectPrefix + group.Key
	}

	for _, modifier := range group.unsavedElements() {
		file, err = script.PutMember(file, block, modifier.Name, genes.WriteModifier(modifier, weights[modifier.Name]))
		if err != nil {
			return "", err
		}
	}

	return file, nil
}
