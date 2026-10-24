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
// whose modifiers it can pick. Until something is picked, the portrait is
// drawn as its DNA alone, undressed, as it always was.

// usageGame is the usage of a group the game picks a modifier of for every
// character; a group of usage none is only used where something names one of
// its modifiers, as an animation does its props.
const usageGame = "game"

// looks are the groups of portrait modifiers and the modifier picked of
// each, by the group's key; an empty name picks nothing of a group with no
// fallback.
type looks struct {
	groups database.Table[genes.ModifierGroup]
	picked map[string]string
}

// dressing reports whether the character is drawn dressed: whether anything
// has been picked, even nothing of a group.
func (l *looks) dressing() bool {
	return l.picked != nil
}

// pick picks a modifier of a group by name, or nothing for an empty name,
// which dresses the character from then on.
func (l *looks) pick(group, modifier string) {
	if l.picked == nil {
		l.picked = map[string]string{}
	}

	l.picked[group] = modifier
}

// choice is the modifier a group is drawn with: the one picked; its fallback
// where nothing is picked and it has one; and for a group nothing was ever
// picked of, the first modifier the game considers for every character
// alike, at a weight above nothing that no condition changes, as it always
// shrinks a body under its clothes by special_shrink_body. It reports false
// where the group is drawn with nothing.
func (l *looks) choice(group *genes.ModifierGroup) (*genes.PortraitModifier, bool) {
	name, touched := l.picked[group.Key]
	if name == "" {
		name = group.Fallback
	}

	if name == "" && !touched {
		for index := range group.Modifiers {
			if weight, ok := group.Modifiers[index].Weight.Get(); ok && weight.Unconditional() && weight.Base.Or(0) > 0 {
				return &group.Modifiers[index], true
			}
		}
	}

	if name == "" {
		return nil, false
	}

	return group.Modifier(name)
}

// modifiers are the modifiers the character is drawn with, in the order the
// game applies them: by the priority of their groups, the highest last, so
// that it has the last word on a gene several change, and by the groups'
// keys where they are equal. None while the character is not dressed, and
// none of the groups the game does not pick from on its own.
func (l *looks) modifiers() []*genes.PortraitModifier {
	if !l.dressing() {
		return nil
	}

	var groups []*genes.ModifierGroup

	for _, group := range l.groups.All() {
		if group.Usage == usageGame {
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
