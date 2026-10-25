package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/config"
)

// gameClothes is a file of the game's clothes, with a weight of conditions
// and a comment that saving has to keep as they are written.
const gameClothes = byteOrderMark + "clothes = {\n" +
	"\tusage = game\n" +
	"\tfallback = default_clothes\n" +
	"\n" +
	"\tdefault_clothes = {\n" +
	"\t\tdna_modifiers = {\n" +
	"\t\t\taccessory = { mode = add gene = outfits template = peasant_outfits range = { 0 1 } }\n" +
	"\t\t}\n" +
	"\t\tweight = { base = 1 }\n" +
	"\t}\n" +
	"\n" +
	"\tagitator_clothes = {\n" +
	"\t\tdna_modifiers = {\n" +
	"\t\t\taccessory = { mode = add gene = outfits template = european_outfits_agitator range = { 0 1 } }\n" +
	"\t\t\taccessory = { mode = add gene = legwear template = european_legwear range = { 0 1 } }\n" +
	"\t\t}\n" +
	"\t\tweight = {\n" +
	"\t\t\tbase = 0\n" +
	"\t\t\t# Agitators of Europe.\n" +
	"\t\t\tmodifier = { add = 100 agitator_character_trigger = yes }\n" +
	"\t\t}\n" +
	"\t}\n" +
	"}\n"

// gameFolder writes the game's clothes into a folder of its own, and reads
// the portrait modifiers of it as the creator does.
func gameFolder(t *testing.T) (string, database.Table[genes.ModifierGroup]) {
	t.Helper()

	game := t.TempDir()
	path := filepath.Join(game, "gfx", "portraits", "portrait_modifiers", "01_clothes.txt")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(gameClothes), 0o644); err != nil {
		t.Fatal(err)
	}

	table, diagnostics := genes.LoadModifiers(folders.Open([]folders.Source{{Name: "game", Path: game}}))
	for _, diagnostic := range diagnostics.Filter(report.SeverityWarning) {
		t.Errorf("unexpected: %s", diagnostic)
	}

	return path, table
}

// readGroups reads the groups of a file's contents, failing on anything
// worth a warning.
func readGroups(t *testing.T, contents string) database.Table[genes.ModifierGroup] {
	t.Helper()

	document := script.Parse(contents)
	for _, warning := range document.Warnings {
		t.Errorf("parse: %s in\n%s", warning, contents)
	}

	collector := &report.Collector{}
	table := genes.ReadModifiers(document, folders.File{Relative: "gfx/portraits/portrait_modifiers/test.txt", Path: "test.txt", Source: "test"}, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("read: %s", diagnostic)
	}

	return table
}

// changedProject is a project that has changed the legwear of the
// agitator's clothes and made a copy of them, as a person working on the
// game's clothes would.
func changedProject(t *testing.T, table database.Table[genes.ModifierGroup]) (*project, *projectGroup) {
	t.Helper()

	pr := newProject()
	clothes, _ := table.Get("clothes")

	group := pr.edit(clothes)
	agitator, _ := group.Modifier("agitator_clothes")
	agitator.Genes[1].Template = "slavic_legwear"
	group.changed("agitator_clothes")

	group.add("mage_clothes", agitator)

	return pr, group
}

// A project changes a copy of a group, which it draws in the place of the
// game's, and leaves the group the game read as it was.
func TestProjectChangesACopy(t *testing.T) {
	_, table := gameFolder(t)
	pr, group := changedProject(t, table)

	clothes, _ := table.Get("clothes")
	if original, _ := clothes.Modifier("agitator_clothes"); original.Genes[1].Template != "european_legwear" {
		t.Errorf("the game's legwear is %s, want it left as european_legwear", original.Genes[1].Template)
	}

	drawn := pr.over(table)
	if len(drawn) != 1 || drawn[0] != &group.ModifierGroup {
		t.Errorf("the project draws %v, want its copy of the clothes", drawn)
	}

	if changed, made := group.counts(); changed != 1 || made != 1 || !group.unsaved() || unsavedSummary(group) != "1 changed, 1 new" {
		t.Errorf("counts %d changed, %d new, summary %q", changed, made, unsavedSummary(group))
	}

	// Reverting puts back the game's element, and takes away a new one.
	pr.revert(table, "clothes", "agitator_clothes")
	pr.revert(table, "clothes", "mage_clothes")

	if agitator, _ := group.Modifier("agitator_clothes"); agitator.Genes[1].Template != "european_legwear" || group.unsaved() {
		t.Errorf("reverted, the legwear is %s and unsaved is %v", agitator.Genes[1].Template, group.unsaved())
	}

	if _, ok := group.Modifier("mage_clothes"); ok {
		t.Error("the new element is still there after reverting it")
	}
}

// A name for a new element or group is one the files can hold that nothing
// else has.
func TestProjectNames(t *testing.T) {
	_, table := gameFolder(t)
	clothes, _ := table.Get("clothes")

	for name, bad := range map[string]bool{"": true, "default_clothes": true, "my clothes": true, "INJECT:x": true, "usage": true, "mage_clothes": false} {
		if problem := elementNameProblem(clothes, name); (problem != "") != bad {
			t.Errorf("element %q: problem %q, want one: %v", name, problem, bad)
		}
	}

	for key, bad := range map[string]bool{"": true, "clothes": true, "my auras": true, "auras": false} {
		if problem := groupKeyProblem(groupList(table), key); (problem != "") != bad {
			t.Errorf("group %q: problem %q, want one: %v", key, problem, bad)
		}
	}
}

// Appended to the game's file, a changed element is written where it
// stands, its weight as it was, and a new one after the last, with the
// weight new elements get; the rest of the file stays as it was.
func TestSaveAppendsToTheFile(t *testing.T) {
	path, table := gameFolder(t)
	pr, group := changedProject(t, table)

	merged, err := mergeGroup(pr, pr.over(table), group, path, false)
	if err != nil {
		t.Fatal(err)
	}

	clothes, ok := readGroups(t, merged).Get("clothes")
	if !ok || len(clothes.Modifiers) != 3 {
		t.Fatalf("merged\n%s\nwant the clothes with three elements", merged)
	}

	if names := names([]*genes.PortraitModifier{&clothes.Modifiers[0], &clothes.Modifiers[1], &clothes.Modifiers[2]}); strings.Join(names, " ") != "default_clothes agitator_clothes mage_clothes" {
		t.Errorf("elements %v, want the new one after the others", names)
	}

	if clothes.Modifiers[1].Genes[1].Template != "slavic_legwear" {
		t.Errorf("legwear %s, want the changed slavic_legwear", clothes.Modifiers[1].Genes[1].Template)
	}

	if !strings.Contains(merged, "# Agitators of Europe.") || !strings.Contains(merged, "# TODO: This modifier has been generated with PDX Portrait Creator") {
		t.Errorf("merged\n%s\nwant the game's weight with its comment and the new one's TODO", merged)
	}

	if !strings.HasPrefix(merged, byteOrderMark+"clothes = {\n\tusage = game\n\tfallback = default_clothes\n\n\tdefault_clothes = {") {
		t.Errorf("merged\n%s\nwant the start of the file as it was", merged)
	}

	// Saved, nothing of the group is left unsaved.
	group.saved(path)

	if group.unsaved() {
		t.Error("the group is unsaved after saving it")
	}
}

// Saved as an inject, the elements go into the file's INJECT: block of the
// group where it has one, into a new one at its end otherwise, and into a
// new file where there is none.
func TestSaveInjects(t *testing.T) {
	_, table := gameFolder(t)
	pr, group := changedProject(t, table)
	folder := t.TempDir()

	withBlock := filepath.Join(folder, "with_block.txt")
	if err := os.WriteFile(withBlock, []byte("INJECT:clothes = {\n\tother_clothes = { }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	without := filepath.Join(folder, "without.txt")
	if err := os.WriteFile(without, []byte("headgear = {\n\tusage = game\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{withBlock, without, filepath.Join(folder, "new.txt")} {
		merged, err := mergeGroup(pr, pr.over(table), group, path, true)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(path), err)
		}

		document := script.Parse(merged)

		blocks := document.All(injectPrefix + "clothes")
		if len(blocks) != 1 {
			t.Fatalf("%s: merged\n%s\nwant one INJECT:clothes block", filepath.Base(path), merged)
		}

		for _, name := range []string{"agitator_clothes", "mage_clothes"} {
			if _, ok := blocks[0].Get(name); !ok {
				t.Errorf("%s: merged\n%s\nwant %s in the block", filepath.Base(path), merged, name)
			}
		}

		if _, ok := blocks[0].Get("default_clothes"); ok {
			t.Errorf("%s: merged\n%s\nwant only what is unsaved in the block", filepath.Base(path), merged)
		}
	}
}

// A group the project made is written whole, with its settings.
func TestSaveWritesANewGroup(t *testing.T) {
	pr := newProject()
	group := &projectGroup{
		ModifierGroup: genes.ModifierGroup{Definition: database.Definition{Key: "auras"}, Usage: usageGame, SelectionBehavior: behaviorWeighted, Fallback: "glow"},
		fresh:         true,
		elements:      map[string]elementState{},
		created:       map[string]string{},
	}
	group.add("glow", nil)
	pr.groups["auras"] = group

	path := filepath.Join(t.TempDir(), "auras.txt")

	merged, err := mergeGroup(pr, pr.over(database.Table[genes.ModifierGroup]{}), group, path, false)
	if err != nil {
		t.Fatal(err)
	}

	auras, ok := readGroups(t, merged).Get("auras")
	if !ok || auras.Fallback != "glow" || auras.Usage != usageGame || len(auras.Modifiers) != 1 || !strings.Contains(merged, "# TODO") {
		t.Errorf("merged\n%s\nwant the auras with glow and its TODO", merged)
	}
}

// The dialog of saving opens in the last of the folders, as far down the
// folder of the portrait modifiers as it has.
func TestSaveStart(t *testing.T) {
	game, mod := t.TempDir(), t.TempDir()
	a := &App{settings: &config.Settings{Folders: []config.Folder{{Name: "game", Path: game}, {Name: "mod", Path: mod}}}}

	if start := a.saveStart(""); start != mod+string(filepath.Separator) {
		t.Errorf("a mod of nothing opens at %q, want its folder", start)
	}

	if err := os.MkdirAll(filepath.Join(mod, "gfx"), 0o755); err != nil {
		t.Fatal(err)
	}

	if start := a.saveStart(""); start != filepath.Join(mod, "gfx")+string(filepath.Separator) {
		t.Errorf("a mod with gfx opens at %q, want gfx", start)
	}

	folder := filepath.Join(mod, "gfx", "portraits", "portrait_modifiers")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}

	if start := a.saveStart("auras.txt"); start != filepath.Join(folder, "auras.txt") {
		t.Errorf("a mod with the folder opens at %q, want auras.txt in it", start)
	}

	if !a.inGame(filepath.Join(game, "gfx", "x.txt")) || a.inGame(filepath.Join(mod, "gfx", "x.txt")) {
		t.Error("the game's files are not told from the mod's")
	}
}

// Looks try an element on in the place of the one picked until they stop,
// and show one group alone where asked to.
func TestLooksTryOnAndShowAlone(t *testing.T) {
	l := looks{groups: groupList(lookGroups(t)), on: true}

	l.try("clothes", "coat")

	if got := names(l.modifiers()); strings.Join(got, " ") != "shrink_body coat" {
		t.Errorf("trying the coat on = %v, want the body shrunk and the coat", got)
	}

	l.only = "clothes"

	if got := names(l.modifiers()); strings.Join(got, " ") != "coat" {
		t.Errorf("the clothes alone = %v, want the coat", got)
	}

	l.only = ""
	l.stopTrying()

	if got := names(l.modifiers()); strings.Join(got, " ") != "shrink_body default_clothes" {
		t.Errorf("stopped trying = %v, want the fallback back", got)
	}
}

// byteOrderMark is what the games' files start with.
const byteOrderMark = "\xef\xbb\xbf"
