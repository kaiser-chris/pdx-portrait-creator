//go:build uitest

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/script"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// active reports whether a window was drawn in the last frame, and shown: a
// window docked behind the tab of another is drawn but hidden.
func active(name string) bool {
	window, ok := gui.FindWindow(name)

	return ok && window.WasActive() && !window.Hidden()
}

// With the game configured, a portrait modifier project takes the place of
// the character panel with windows of its own, dresses its base in the
// element picked, keeps what it changes in a copy, and saves a group as an
// inject into a file picked, in the folder of the last of the folders. Back
// in making a character, the windows are as they were. It is skipped unless
// PDX_GAME_DIR points at the game.
func TestPortraitModifierProject(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	mod := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mod, "gfx"), 0o755); err != nil {
		t.Fatal(err)
	}

	application, _ := startApp(t, fmt.Sprintf(`{"Folders":[{"Name":"game","Path":%q},{"Name":"mod","Path":%q}],"InterfaceScale":1}`, game, mod))
	defer application.Close()

	waitForPortrait(t, application)

	portrait := application.portrait
	application.state.welcome = false
	waitForRebuild(t, application)

	preview, _ := gui.FindWindow(panelPreview)
	wide := preview.Size().X

	// The character panel is the tab shown of those beside it, rather than
	// the Portrait Modifiers window, which is a later step.
	if !active(panelCharacter) || active(panelLooks) || active(panelProject) || active(panelElement) {
		t.Error("making a character, the character panel is not the one drawn")
	}

	// The welcome screen starts a project in its workflow.
	application.state.welcomeMode = modeModifiers
	application.setMode(modeModifiers)
	waitForRebuild(t, application)

	if active(panelCharacter) || !active(panelProject) || !active(panelElement) {
		t.Fatal("in a project, its windows are not the ones drawn")
	}

	// The DNA of the base and what it wears are tabs beside the portrait
	// modifiers, which are the tab shown.
	for _, tab := range []string{panelBaseDNA, panelWear} {
		if window, ok := gui.FindWindow(tab); !ok || !window.WasActive() || !window.Hidden() {
			t.Errorf("%s is not a tab behind the portrait modifiers", tab)
		}
	}

	// The element picked of the clothes dresses the base, with nothing of the
	// other groups but what the game always applies, until it is picked.
	clothes, ok := portrait.character.looks.group("clothes")
	if !ok {
		t.Fatal("the game has no clothes")
	}

	at := slices.IndexFunc(clothes.Modifiers, func(m genes.PortraitModifier) bool {
		return m.Name != clothes.Fallback && len(m.Genes) > 0 && m.Genes[0].Kind == "accessory"
	})
	if at < 0 {
		t.Fatal("the clothes have no element that puts any on")
	}

	element := clothes.Modifiers[at].Name
	pr := application.project
	pr.group, pr.element = "clothes", element
	waitForRebuild(t, application)

	want := []string{element}

	for _, group := range portrait.character.looks.groups {
		if modifier, ok := alwaysApplied(group); ok && group.Usage == usageGame {
			want = append(want, modifier.Name)
		}
	}

	got := names(portrait.character.looks.modifiers())
	slices.Sort(got)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("the base wears %v, want %v", got, want)
	}

	worn := slices.Clone(portrait.bodyParts)
	if len(worn) == 0 {
		t.Errorf("in %s, the body is drawn with nothing", element)
	}

	// Picked in the Worn With tab, another group is worn with the element edited,
	// which stays what the clothes are drawn with while others are tried on.
	hairstyles, ok := portrait.character.looks.group("hairstyles")
	if ok && len(hairstyles.Modifiers) > 1 {
		portrait.character.looks.pick("hairstyles", hairstyles.Modifiers[1].Name)
		portrait.character.looks.try("clothes", clothes.Fallback)
		portrait.character.redress()
		waitForRebuild(t, application)

		if got := names(portrait.character.looks.modifiers()); !slices.Contains(got, element) || !slices.Contains(got, hairstyles.Modifiers[1].Name) {
			t.Errorf("with the hair picked, the base wears %v, want %s and %s", got, element, hairstyles.Modifiers[1].Name)
		}

		portrait.character.looks.stopTrying()
		portrait.character.looks.unpick("hairstyles")
		waitForRebuild(t, application)
	}

	// The weight window and every dialog draw.
	application.state.workspace.showWeight = true
	application.openNewElement("clothes", nil)

	for range 3 {
		application.Step()
	}

	if !active(windowWeight) {
		t.Error("the weight window is not drawn")
	}

	// Changing the element changes a copy of the clothes, and adding one
	// adds it to the copy.
	group := pr.edit(clothes)
	group.Modifiers[at].Genes = group.Modifiers[at].Genes[:1]
	group.changed(element)
	group.add("test_clothes", &group.Modifiers[at])
	portrait.character.redress()
	waitForRebuild(t, application)

	if original, _ := portrait.modifiers.Get("clothes"); len(original.Modifiers[at].Genes) == len(group.Modifiers[at].Genes) && len(original.Modifiers[at].Genes) > 1 {
		t.Error("the game's clothes were changed along with the project's")
	}

	// Saved as an inject, the dialog opens at gfx, the deepest folder of
	// the portrait modifiers the mod has, and the file picked gets both.
	path := filepath.Join(mod, "gfx", "portraits", "portrait_modifiers", "test_inject.txt")
	dialogs := &fakeDialogs{answers: []string{path}}
	application.dialogs = dialogs

	if !application.canSaveModifier() {
		t.Fatal("the project has nothing to save")
	}

	application.askFile(group, true)
	answerDialog(t, application)

	if start := dialogs.lastAsked().start; start != filepath.Join(mod, "gfx")+string(filepath.Separator) {
		t.Errorf("the dialog opened at %q, want the mod's gfx folder", start)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the inject was not written: %v (status %q)", err, application.state.status)
	}

	block, ok := script.Parse(string(data)).Get(injectPrefix + "clothes")
	if !ok {
		t.Fatalf("the file holds no INJECT:clothes:\n%s", data)
	}

	for _, name := range []string{element, "test_clothes"} {
		if _, ok := block.Get(name); !ok {
			t.Errorf("the inject has no %s:\n%s", name, data)
		}
	}

	if !strings.Contains(string(data), "# TODO: This modifier has been generated with PDX Portrait Creator") || group.unsaved() || application.canSaveModifier() {
		t.Errorf("saved, unsaved is %v and the file is\n%s", group.unsaved(), data)
	}

	// Back in making a character, its panel is drawn again, at the width it
	// had, and its looks are as they were: off.
	application.setMode(modeCharacter)
	waitForRebuild(t, application)

	if !active(panelCharacter) || active(panelProject) || active(panelElement) {
		t.Error("back in making a character, the character panel is not the one drawn")
	}

	if preview, _ := gui.FindWindow(panelPreview); preview.Size().X < wide-1 {
		t.Errorf("the portrait is %v wide, want the %v it was before the project", preview.Size().X, wide)
	}

	if portrait.character.looks.on || len(portrait.bodyParts) > 0 {
		t.Errorf("back in making a character, it is dressed in %v", portrait.bodyParts)
	}
}

// The Portrait Modifiers window of a character draws turned on, with the
// list of an element open, and the dialogs of saving draw.
func TestPortraitModifierWindowsDraw(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	application, _ := startApp(t, fmt.Sprintf(`{"Folders":[{"Name":"game","Path":%q}],"InterfaceScale":1}`, game))
	defer application.Close()

	waitForPortrait(t, application)
	application.state.welcome = false
	application.portrait.character.looks.on = true
	application.portrait.character.redress()
	waitForRebuild(t, application)

	// The window is a tab behind the character panel, which is brought
	// forward as a click on its tab would.
	imgui.SetWindowFocusStr(panelLooks)
	waitForRebuild(t, application)

	if !active(panelLooks) {
		t.Fatal("the Portrait Modifiers window is not drawn")
	}

	application.setMode(modeModifiers)
	application.openNewGroup()

	for range 3 {
		application.Step()
	}

	clothes, _ := application.portrait.character.looks.group("clothes")
	application.project.edit(clothes).add("first_test", nil)
	application.project.edit(clothes).add("second_test", nil)

	auras := &newGroupForm{key: "test_auras", usage: usageGame, behavior: behaviorWeighted, first: "glow"}
	application.createGroup(auras)

	application.saveModifier()

	for range 3 {
		application.Step()
	}

	if form := application.state.workspace.save; form.group == "" {
		t.Error("saving two groups did not ask which")
	}
}
