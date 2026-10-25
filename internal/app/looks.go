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
// what they picked. A group with a fallback, such as clothes, is drawn in its
// fallback where nothing is picked, as the game draws a character none of
// whose modifiers it can pick.
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
// empty name picks nothing of a group with no fallback.
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
// picked; its fallback where nothing is picked and it has one; and for a
// group nothing was ever picked of, the first modifier the game considers
// for every character alike, at a weight above nothing that no condition
// changes, as it always shrinks a body under its clothes by
// special_shrink_body. It reports false where the group is drawn with
// nothing.
func (l *looks) choice(group *genes.ModifierGroup) (*genes.PortraitModifier, bool) {
	name, touched := l.picked[group.Key]
	if l.tryingGroup == group.Key {
		name, touched = l.trying, true
	}

	if name == "" {
		name = group.Fallback
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
// character alike: at a weight above nothing that no condition changes.
func alwaysApplied(group *genes.ModifierGroup) (*genes.PortraitModifier, bool) {
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
			modifiers = append(modifiers, modifier)
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
