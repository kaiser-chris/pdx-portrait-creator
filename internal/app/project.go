package app

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// A portrait modifier project changes the game's portrait modifiers, adds
// elements to them and makes new ones, on a character picked as a base,
// which it is drawn dressed in. It is a workflow of its own beside making a
// character's DNA: the welcome screen starts one, and the windows of the
// workspace take the place of the character panel; see workspace.go.
//
// The project keeps a copy of every group it changes, which is drawn in the
// place of the game's, and of every group it makes, until each is saved; see
// modsave.go. The groups the game reads are never changed.

// generatedWeight is the weight a new element of a project is given: at base
// 0 the game never picks it on its own, and the comment says to whoever reads
// the file that its conditions are still to be written.
const generatedWeight = "weight = {\n\t# TODO: This modifier has been generated with PDX Portrait Creator\n\tbase = 0\n}"

// groupSettings are the keys of a group that are its settings rather than
// its elements, which an element cannot be named.
var groupSettings = []string{"usage", "selection_behavior", "fallback", "priority", "gruesome"}

// elementState is what a project has done to an element of a group.
type elementState int

const (
	elementSaved elementState = iota
	elementChanged
	elementNew
)

// project is a portrait modifier project: the groups it changes or makes, by
// key, and the group and the element shown.
type project struct {
	groups map[string]*projectGroup

	group, element string

	// weights are the weights of the game's elements as they are written,
	// by the group and the element, which are read once and kept: writing
	// into a file moves what follows in it, after which the places the
	// parser read them at no longer hold them.
	weights map[string]string
}

// projectGroup is a group a project changes or makes: its copy, whether it
// is new, which no file defines until it is saved, whether its settings are
// changed since then, where it was saved to, and what was done to each of
// its elements, by name.
type projectGroup struct {
	genes.ModifierGroup

	fresh, settingsChanged bool
	savedTo                string

	elements map[string]elementState

	// created are the weights of the elements the project made, by name,
	// which the game's files do not hold.
	created map[string]string
}

// newProject is a project that changes nothing yet.
func newProject() *project {
	return &project{groups: map[string]*projectGroup{}, weights: map[string]string{}}
}

// over are the groups as the project draws them: the game's, each in its
// copy where the project changes it, and those the project makes, by key.
func (pr *project) over(table database.Table[genes.ModifierGroup]) []*genes.ModifierGroup {
	var groups []*genes.ModifierGroup

	for key, group := range table.All() {
		if changed, ok := pr.groups[key]; ok {
			group = &changed.ModifierGroup
		}

		groups = append(groups, group)
	}

	for _, group := range pr.groups {
		if _, ok := table.Get(group.Key); !ok {
			groups = append(groups, &group.ModifierGroup)
		}
	}

	slices.SortFunc(groups, func(a, b *genes.ModifierGroup) int { return cmp.Compare(a.Key, b.Key) })

	return groups
}

// edit is the project's copy of a group, made from the group as drawn the
// first time it is changed.
func (pr *project) edit(group *genes.ModifierGroup) *projectGroup {
	if changed, ok := pr.groups[group.Key]; ok {
		return changed
	}

	changed := &projectGroup{ModifierGroup: cloneGroup(group), elements: map[string]elementState{}, created: map[string]string{}}
	pr.groups[group.Key] = changed

	return changed
}

// cloneGroup copies a group, its elements and their changes of the genes
// too, so that changing the copy leaves the group as the game read it.
func cloneGroup(group *genes.ModifierGroup) genes.ModifierGroup {
	copied := *group
	copied.Modifiers = make([]genes.PortraitModifier, len(group.Modifiers))

	for index := range group.Modifiers {
		copied.Modifiers[index] = cloneModifier(&group.Modifiers[index])
	}

	return copied
}

// cloneModifier copies an element and its changes of the genes.
func cloneModifier(modifier *genes.PortraitModifier) genes.PortraitModifier {
	copied := *modifier
	copied.Genes = slices.Clone(modifier.Genes)
	copied.OutfitTags = slices.Clone(modifier.OutfitTags)

	return copied
}

// changed marks an element of a group changed, unless the project made it.
func (g *projectGroup) changed(name string) {
	if g.elements[name] != elementNew {
		g.elements[name] = elementChanged
	}
}

// add adds an element to a group under a name, a copy of another or nothing,
// with the weight new elements are given.
func (g *projectGroup) add(name string, from *genes.PortraitModifier) {
	element := genes.PortraitModifier{Name: name}
	if from != nil {
		element = cloneModifier(from)
		element.Name, element.Source = name, database.Span{}
	}

	element.Weight = database.Some(genes.Weight{Base: database.Some(0.0)})

	g.Modifiers = append(g.Modifiers, element)
	g.elements[name], g.created[name] = elementNew, generatedWeight
}

// unsaved reports whether a group has anything the project has not saved.
func (g *projectGroup) unsaved() bool {
	if g.fresh || g.settingsChanged {
		return true
	}

	for _, state := range g.elements {
		if state != elementSaved {
			return true
		}
	}

	return false
}

// counts are how many elements of a group the project has changed and made
// since it was last saved.
func (g *projectGroup) counts() (changed, made int) {
	for _, state := range g.elements {
		switch state {
		case elementChanged:
			changed++
		case elementNew:
			made++
		}
	}

	return changed, made
}

// unsavedElements are the elements of a group changed or made since it was
// last saved, in the order of the group.
func (g *projectGroup) unsavedElements() []*genes.PortraitModifier {
	var found []*genes.PortraitModifier

	for index := range g.Modifiers {
		if g.elements[g.Modifiers[index].Name] != elementSaved {
			found = append(found, &g.Modifiers[index])
		}
	}

	return found
}

// saved marks everything of a group saved, into a file for a group the
// project made.
func (g *projectGroup) saved(path string) {
	if g.fresh {
		g.savedTo = path
	}

	g.fresh, g.settingsChanged = false, false

	for name := range g.elements {
		g.elements[name] = elementSaved
	}
}

// unsavedGroups are the groups with anything unsaved, by key.
func (pr *project) unsavedGroups() []*projectGroup {
	var found []*projectGroup

	for _, group := range pr.groups {
		if group.unsaved() {
			found = append(found, group)
		}
	}

	slices.SortFunc(found, func(a, b *projectGroup) int { return cmp.Compare(a.Key, b.Key) })

	return found
}

// revert sets an element back to what the game reads, or takes away one the
// project made.
func (pr *project) revert(table database.Table[genes.ModifierGroup], key, name string) {
	group, ok := pr.groups[key]
	if !ok {
		return
	}

	at := slices.IndexFunc(group.Modifiers, func(m genes.PortraitModifier) bool { return m.Name == name })
	if at < 0 {
		return
	}

	if group.elements[name] == elementNew {
		group.Modifiers = slices.Delete(group.Modifiers, at, at+1)
		delete(group.elements, name)
		delete(group.created, name)

		if pr.element == name {
			pr.element = ""
		}

		return
	}

	if original, ok := table.Get(key); ok {
		if modifier, ok := original.Modifier(name); ok {
			group.Modifiers[at] = cloneModifier(modifier)
		}
	}

	delete(group.elements, name)
}

// weight is the weight of an element as script: the one the project gave an
// element it made, or the game's as it is written, read once and kept; none
// for an element written without one.
func (pr *project) weight(group *projectGroup, modifier *genes.PortraitModifier) (string, error) {
	if weight, ok := group.created[modifier.Name]; ok {
		return weight, nil
	}

	return pr.gameWeight(group.Key, modifier)
}

// gameWeight is the weight of one of the game's elements as it is written,
// read once and kept.
func (pr *project) gameWeight(key string, modifier *genes.PortraitModifier) (string, error) {
	weight, ok := modifier.Weight.Get()
	if !ok || weight.Source.Origin.Path == "" {
		return "", nil
	}

	id := key + "/" + modifier.Name
	if text, ok := pr.weights[id]; ok {
		return text, nil
	}

	text, err := weight.Source.Text()
	if err != nil {
		return "", err
	}

	pr.weights[id] = text

	return text, nil
}

// keepWeights reads the weights of every element of the groups drawn that is
// written in a file, before the file is written into, after which they could
// no longer be read where the parser found them.
func (pr *project) keepWeights(groups []*genes.ModifierGroup, path string) error {
	for _, group := range groups {
		for index := range group.Modifiers {
			modifier := &group.Modifiers[index]

			if weight, ok := modifier.Weight.Get(); ok && sameFile(weight.Source.Origin.Path, path) {
				if _, err := pr.gameWeight(group.Key, modifier); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// elementNameProblem says what is wrong with a name for a new element of a
// group, or nothing for a good one: one the game's files can hold, and no
// other element of the group already has.
func elementNameProblem(group *genes.ModifierGroup, name string) string {
	switch {
	case name == "":
		return "Give it a name"
	case !script.IsKey(name) || strings.Contains(name, ":"):
		return "Use letters, digits and _ only"
	case slices.Contains(groupSettings, name):
		return name + " is a setting of the group"
	}

	if _, ok := group.Modifier(name); ok {
		return "Another element of " + group.Key + " has this name"
	}

	return ""
}

// groupKeyProblem says what is wrong with a key for a new group, or nothing
// for a good one, which no group of the game, a mod or the project has.
func groupKeyProblem(groups []*genes.ModifierGroup, key string) string {
	switch {
	case key == "":
		return "Give it a key"
	case !script.IsKey(key) || strings.Contains(key, ":"):
		return "Use letters, digits and _ only"
	}

	for _, group := range groups {
		if group.Key == key {
			return "The game or a mod already has a portrait modifier " + key
		}
	}

	return ""
}
