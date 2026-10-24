package app

import (
	"fmt"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/genes"
)

// A morph gene the character panel lists may change nothing of the portrait
// as it is: the game's files give it nothing for this sex, or nothing at this
// age, or it sets attributes no model of the portrait has. The panel still
// lists it, dimmed, and says why, so that a gene does not seem to be broken
// where the game would draw nothing either. What a gene does for a sex and
// an age is worked out again whenever either changes; see refreshIdle.

// The reasons a morph gene changes nothing that do not depend on the sex or
// the age.
const (
	idleEmpty      = "Changes nothing: the game's files give it nothing to do"
	idleAttributes = "Changes nothing: no model has the attributes it sets"
)

// idleStrengths are the strengths a gene is tried at to find whether it does
// anything at an age: its ends and the points between, which the curves of
// the shipped files all bend at or between.
var idleStrengths = []float64{0, 0.25, 0.5, 0.75, 1}

// idleFor is the sex and the age the panel last worked out what the morph
// genes do for, so that it is not worked out again every frame.
type idleFor struct {
	sex   genes.Sex
	age   int32
	valid bool
}

// refreshIdle works out for every morph gene whether it changes anything of
// the portrait at the character's sex and age, and why not where it does
// not, if either has changed since it last did.
func (c *character) refreshIdle() {
	now := idleFor{sex: c.sex, age: c.age, valid: true}
	if c.idleAt == now {
		return
	}

	c.idleAt = now

	for index := range c.morphs {
		morph := &c.morphs[index]
		morph.drawn = c.does(morph.gene, c.sex)
		morph.idle = c.idleReason(morph.gene)
	}
}

// setDrawable sets the attributes that morph the portrait's models, which
// other models than it was drawn with have other ones of, such as clothes,
// and has what the genes do worked out again.
func (c *character) setDrawable(attributes []string) {
	c.drawable = map[string]bool{}

	for _, attribute := range attributes {
		c.drawable[attribute] = true
	}

	c.idleAt = idleFor{}
}

// does reports whether a gene does anything the portrait draws for a sex, at
// any age: a template of it sets an attribute a model of the portrait has,
// lays a decal, replaces the textures of the skin or shifts a colour.
func (c *character) does(gene *genes.MorphGene, sex genes.Sex) bool {
	for index := range gene.Templates {
		morph := gene.Templates[index].For(sex)
		if len(morph.Decals) > 0 || len(morph.TextureOverrides) > 0 || len(morph.HSVShifts) > 0 {
			return true
		}

		for _, setting := range morph.Settings {
			if c.drawable[setting.Attribute] {
				return true
			}
		}
	}

	return false
}

// gives reports whether the game's files give a gene anything to do for a
// sex: any setting, decal, override or shift at all. It is what tells the
// sexes a gene is for, whose models may have other attributes than the
// portrait's.
func gives(gene *genes.MorphGene, sex genes.Sex) bool {
	for index := range gene.Templates {
		morph := gene.Templates[index].For(sex)
		if len(morph.Settings) > 0 || len(morph.Decals) > 0 || len(morph.TextureOverrides) > 0 || len(morph.HSVShifts) > 0 {
			return true
		}
	}

	return false
}

// idleReason says why a gene changes nothing of the portrait as it is, or
// nothing where it does: it is for other sexes only, as Victoria 3's stubble
// is for men and boys; its templates set nothing at all, as Europa
// Universalis 5's gene_head_size, whose setting the game's file leaves
// commented out; they set attributes no model of the portrait has; or what
// it does starts or ends at other ages, as the wrinkles of the old.
func (c *character) idleReason(gene *genes.MorphGene) string {
	if !c.does(gene, c.sex) {
		var others []string

		for _, sex := range genes.Sexes {
			if sex != c.sex && gives(gene, sex) {
				others = append(others, friendly(sex.String()))
			}
		}

		switch {
		case len(others) > 0:
			return "Only applies to " + joinedAnd(others)
		case gives(gene, c.sex):
			return idleAttributes
		default:
			return idleEmpty
		}
	}

	if c.activeAt(gene, c.age) {
		return ""
	}

	var active []int32

	for age := int32(0); age <= maxAge; age++ {
		if c.activeAt(gene, age) {
			active = append(active, age)
		}
	}

	if len(active) == 0 {
		return "Changes nothing at any age"
	}

	return fmt.Sprintf("Changes nothing at %d years: only applies %s", c.age, ageSpans(active))
}

// activeAt reports whether a gene does anything the portrait draws at an
// age in years, at some template and strength: an attribute a model has set
// to other than nothing, a decal not wholly transparent, a colour shifted,
// or the skin's textures replaced, which no age changes.
func (c *character) activeAt(gene *genes.MorphGene, age int32) bool {
	at := float64(age) / maxAge

	for index := range gene.Templates {
		morph := gene.Templates[index].For(c.sex)
		if len(morph.TextureOverrides) > 0 {
			return true
		}

		for _, strength := range idleStrengths {
			for decal := range morph.Decals {
				if c.all.DecalAlpha(&morph.Decals[decal], strength, at) > 0 {
					return true
				}
			}

			for _, setting := range morph.Settings {
				if c.drawable[setting.Attribute] && c.all.AtAge(setting.At(strength), setting.Age, at) != 0 {
					return true
				}
			}

			for _, colour := range []string{"hair", "skin", "eye"} {
				if c.all.HSVShift(morph, colour, strength, at) != [3]float64{} {
					return true
				}
			}
		}
	}

	return false
}

// ageSpans says over which ages in years a gene applies, as the runs of the
// ages given, which are in order: from 40 years, up to 12 years, from 3 to
// 12 years.
func ageSpans(ages []int32) string {
	var spans []string

	for start := 0; start < len(ages); {
		end := start
		for end+1 < len(ages) && ages[end+1] == ages[end]+1 {
			end++
		}

		first, last := ages[start], ages[end]

		switch {
		case first == 0 && last == maxAge:
			spans = append(spans, "at every age")
		case last == maxAge:
			spans = append(spans, fmt.Sprintf("from %d years", first))
		case first == 0:
			spans = append(spans, fmt.Sprintf("up to %d years", last))
		case first == last:
			spans = append(spans, fmt.Sprintf("at %d years", first))
		default:
			spans = append(spans, fmt.Sprintf("from %d to %d years", first, last))
		}

		start = end + 1
	}

	return joinedAnd(spans)
}

// joinedAnd joins words as a sentence lists them: "Male", "Male and Boy",
// "Male, Female and Boy".
func joinedAnd(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}

	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}
