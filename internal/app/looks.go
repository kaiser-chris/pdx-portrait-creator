package app

import (
	"cmp"
	"slices"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"
)

// Looks are what the game's portrait modifiers dress a character in over its
// DNA: the clothes of its station, a hat, a beard of its culture. The game
// picks one modifier of each group for a character by weights it works out
// from the character, which the creator does not: the person using it picks
// one of each group themselves, or none, and the portrait is drawn dressed in
// what they picked. A group nothing is picked of is drawn with none of its
// elements, even one with a fallback, such as clothes, which the game draws a
// character in where it can pick none of the others: the fallback is one of
// the elements to pick, so that the character is seen in nothing but what was
// picked.
//
// Dressing the character is a later step of making one, to see it in the
// game's clothes once its DNA is done, so it is off until it is turned on in
// the Portrait Modifiers window; until then the portrait is drawn as its DNA
// alone.

// usageGame is the usage of a group the game picks a modifier of for every
// character; a group of usage none is only used where something names one of
// its modifiers, as an animation does its props.
const usageGame = "game"

// looks are the groups of portrait modifiers, whether the character is
// dressed in them, and the modifier picked of each, by the group's key; an
// empty name picks nothing of the group.
type looks struct {
	// groups are the groups as they are drawn: the game's, and those a
	// project changes or makes in their place; see project.go.
	groups []*genes.ModifierGroup

	on     bool
	picked map[string]string

	// trying is a modifier tried on in the place of the one picked of its
	// group, while it is pointed at in a list.
	trying, tryingGroup string

	// only draws the character in the modifiers of one group alone, by its
	// key, as the project's workspace does to show one element by itself.
	only string

	// strengths are the strengths picked within the ranges the elements give
	// the genes they change, by the group and the element, and then by the
	// place of the change: the game picks one of a range by chance, and the
	// middle of it is drawn where none is picked.
	strengths map[string]map[int]float64
}

// strengthKey is what the strengths of an element of a group are kept by.
func strengthKey(group, element string) string {
	return group + "/" + element
}

// strength is the strength picked for a change of an element, if one is.
func (l *looks) strength(group, element string, change int) (float64, bool) {
	value, ok := l.strengths[strengthKey(group, element)][change]

	return value, ok
}

// setStrength picks a strength within the range a change of an element
// gives.
func (l *looks) setStrength(group, element string, change int, value float64) {
	if l.strengths == nil {
		l.strengths = map[string]map[int]float64{}
	}

	key := strengthKey(group, element)
	if l.strengths[key] == nil {
		l.strengths[key] = map[int]float64{}
	}

	l.strengths[key][change] = value
}

// unsetStrength draws a change of an element at the middle of its range
// again.
func (l *looks) unsetStrength(group, element string, change int) {
	delete(l.strengths[strengthKey(group, element)], change)
}

// withStrengths is a modifier with the strengths picked for it: a copy in
// which each change picked a strength of gives that strength in the place of
// its range, or the modifier itself where none is picked.
func (l *looks) withStrengths(group string, modifier *genes.PortraitModifier) *genes.PortraitModifier {
	picked := l.strengths[strengthKey(group, modifier.Name)]
	if len(picked) == 0 {
		return modifier
	}

	copied := *modifier
	copied.Genes = slices.Clone(modifier.Genes)

	for change, value := range picked {
		if change < len(copied.Genes) && copied.Genes[change].Range.Set {
			copied.Genes[change].Value, copied.Genes[change].Range = database.Some(value), database.Opt[genes.Range]{}
		}
	}

	return &copied
}

// groupList is a table of groups as the list looks are given.
func groupList(table database.Table[genes.ModifierGroup]) []*genes.ModifierGroup {
	var groups []*genes.ModifierGroup

	for _, group := range table.All() {
		groups = append(groups, group)
	}

	return groups
}

// dressing reports whether the character is drawn dressed.
func (l *looks) dressing() bool {
	return l.on
}

// pick picks a modifier of a group by name, or nothing for an empty name.
func (l *looks) pick(group, modifier string) {
	if l.picked == nil {
		l.picked = map[string]string{}
	}

	l.picked[group] = modifier
}

// unpick lets a group be drawn as the game would draw it untouched again.
func (l *looks) unpick(group string) {
	delete(l.picked, group)
}

// try tries a modifier of a group on, until stopTrying; an empty group tries
// nothing on.
func (l *looks) try(group, modifier string) {
	l.tryingGroup, l.trying = group, modifier
}

// stopTrying takes off what is tried on.
func (l *looks) stopTrying() {
	l.tryingGroup, l.trying = "", ""
}

// group returns a group by its key.
func (l *looks) group(key string) (*genes.ModifierGroup, bool) {
	for _, group := range l.groups {
		if group.Key == key {
			return group, true
		}
	}

	return nil, false
}

// choice is the modifier a group is drawn with: the one tried on, or the one
// picked; and for a group nothing was ever picked of, the first modifier the
// game considers for every character alike, at a weight above nothing that
// no condition changes, as it always shrinks a body under its clothes by
// special_shrink_body. Its fallback is drawn only where it is picked. It
// reports false where the group is drawn with nothing.
func (l *looks) choice(group *genes.ModifierGroup) (*genes.PortraitModifier, bool) {
	name, touched := l.picked[group.Key]
	if l.tryingGroup == group.Key {
		name, touched = l.trying, true
	}

	if name == "" && !touched {
		if modifier, ok := alwaysApplied(group); ok {
			return modifier, true
		}
	}

	if name == "" {
		return nil, false
	}

	return group.Modifier(name)
}

// alwaysApplied is the first modifier of a group the game applies to every
// character alike: at a weight above nothing that no condition changes, in a
// group without a fallback. A group with one, such as clothes, is one of
// looks to pick from, of which none is drawn until one is picked, even where
// its fallback is weighed alike for every character.
func alwaysApplied(group *genes.ModifierGroup) (*genes.PortraitModifier, bool) {
	if group.Fallback != "" {
		return nil, false
	}

	for index := range group.Modifiers {
		if weight, ok := group.Modifiers[index].Weight.Get(); ok && weight.Unconditional() && weight.Base.Or(0) > 0 {
			return &group.Modifiers[index], true
		}
	}

	return nil, false
}

// modifiers are the modifiers the character is drawn with, in the order the
// game applies them: by the priority of their groups, the highest last, so
// that it has the last word on a gene several change, and by the groups'
// keys where they are equal. None while the character is not dressed, and
// none of the groups the game does not pick from on its own, unless one is
// shown alone.
func (l *looks) modifiers() []*genes.PortraitModifier {
	if !l.dressing() {
		return nil
	}

	var groups []*genes.ModifierGroup

	for _, group := range l.groups {
		switch {
		case l.only != "":
			if group.Key == l.only {
				groups = append(groups, group)
			}

		case group.Usage == usageGame:
			groups = append(groups, group)
		}
	}

	slices.SortStableFunc(groups, func(a, b *genes.ModifierGroup) int {
		if order := cmp.Compare(a.Priority.Or(0), b.Priority.Or(0)); order != 0 {
			return order
		}

		return cmp.Compare(a.Key, b.Key)
	})

	var modifiers []*genes.PortraitModifier

	for _, group := range groups {
		if modifier, ok := l.choice(group); ok {
			modifiers = append(modifiers, l.withStrengths(group.Key, modifier))
		}
	}

	return modifiers
}

// dressed is the character as the game draws it in the looks picked: its DNA
// with the modifiers applied, and the genes they add beside it.
func (c *character) dressed() genes.Dressed {
	return c.all.ApplyModifiers(c.dna(), c.sex, c.looks.modifiers()...)
}

// redress has the portrait drawn again in the looks as they are now: its
// parts, its morphs and its colours, any of which a modifier may change.
func (c *character) redress() {
	c.changed, c.partsChanged, c.coloursChanged = true, true, true
}
