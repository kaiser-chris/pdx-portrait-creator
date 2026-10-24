package app

import (
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"
)

// A gene that changes nothing of the portrait as it is says why: it is for
// other sexes, as Victoria 3's stubble is for men and boys; its templates set
// nothing, as Europa Universalis 5's head size; they set attributes no model
// has; or it applies at other ages, as the wrinkles of the old. A gene that
// changes something says nothing.
func TestIdleReason(t *testing.T) {
	gene := func(sexes map[genes.Sex]genes.Morph) *genes.MorphGene {
		template := genes.MorphTemplate{Name: "template_1"}
		for sex, morph := range sexes {
			template.Sexes[sex] = morph
		}

		return &genes.MorphGene{Templates: []genes.MorphTemplate{template}}
	}

	stubble := genes.Morph{Decals: []genes.Decal{{BodyPart: "head"}}}

	// Wrinkles that appear from 40 years, as the files' age curves have it,
	// and a blend shape that only a child has.
	wrinkles := genes.Morph{Decals: []genes.Decal{{BodyPart: "head", Age: genes.Age{Mode: genes.ModeMultiply, Curve: genes.Curve{{X: 0.39, Y: 0}, {X: 0.4, Y: 1}}}}}}
	childish := genes.Morph{Settings: []genes.Setting{{
		Attribute: "head_bs_child",
		Value:     database.Some(genes.Range{Min: 0, Max: 1}),
		Age:       genes.Age{Mode: genes.ModeMultiply, Curve: genes.Curve{{X: 0.12, Y: 1}, {X: 0.13, Y: 0}}},
	}}}

	for _, test := range []struct {
		name  string
		gene  *genes.MorphGene
		sex   genes.Sex
		age   int32
		want  string
		drawn bool
	}{
		{"for men", gene(map[genes.Sex]genes.Morph{genes.Male: stubble}), genes.Male, 25, "", true},
		{"for other sexes", gene(map[genes.Sex]genes.Morph{genes.Male: stubble, genes.Boy: stubble}), genes.Girl, 25, "Only applies to Male and Boy", false},
		{"nothing", gene(nil), genes.Male, 25, idleEmpty, false},
		{"no model's attributes", gene(map[genes.Sex]genes.Morph{genes.Male: {Settings: []genes.Setting{{Attribute: "head_size"}}}}), genes.Male, 25, idleAttributes, false},
		{"old", gene(map[genes.Sex]genes.Morph{genes.Male: wrinkles}), genes.Male, 25, "Changes nothing at 25 years: only applies from 40 years", true},
		{"old enough", gene(map[genes.Sex]genes.Morph{genes.Male: wrinkles}), genes.Male, 60, "", true},
		{"young", gene(map[genes.Sex]genes.Morph{genes.Male: childish}), genes.Male, 25, "Changes nothing at 25 years: only applies up to 12 years", true},
		{"texture", gene(map[genes.Sex]genes.Morph{genes.Male: {TextureOverrides: []genes.TextureOverride{{Weight: 100}}}}), genes.Male, 25, "", true},
		{"colour", gene(map[genes.Sex]genes.Morph{genes.Male: {HSVShifts: []genes.HSVShift{{Color: "hair", Curve: []genes.HSVPoint{{X: 0, Shift: [3]float64{0, -1, 0}}}}}}}), genes.Male, 25, "", true},
	} {
		c := &character{all: &genes.Genes{}, sex: test.sex, age: test.age, drawable: map[string]bool{"head_bs_child": true}}
		c.morphs = []morphGene{{gene: test.gene}}
		c.refreshIdle()

		if morph := c.morphs[0]; morph.idle != test.want || morph.drawn != test.drawn {
			t.Errorf("%s: idle %q, drawn %v; want %q, %v", test.name, morph.idle, morph.drawn, test.want, test.drawn)
		}
	}
}

// What a gene does is worked out again once the sex or the age changes.
func TestIdleFollowsTheSexAndTheAge(t *testing.T) {
	template := genes.MorphTemplate{Name: "template_1"}
	template.Sexes[genes.Male] = genes.Morph{Decals: []genes.Decal{{BodyPart: "head"}}}

	c := &character{all: &genes.Genes{}, sex: genes.Male, age: 25, drawable: map[string]bool{}}
	c.morphs = []morphGene{{gene: &genes.MorphGene{Templates: []genes.MorphTemplate{template}}}}
	c.refreshIdle()

	if c.morphs[0].idle != "" {
		t.Fatalf("a man's gene is idle for a man: %q", c.morphs[0].idle)
	}

	c.sex = genes.Female
	c.refreshIdle()

	if c.morphs[0].idle != "Only applies to Male" {
		t.Errorf("a man's gene for a woman: %q", c.morphs[0].idle)
	}
}

// The ages a gene applies at are said as the runs of them.
func TestAgeSpans(t *testing.T) {
	span := func(from, to int32) []int32 {
		var ages []int32
		for age := from; age <= to; age++ {
			ages = append(ages, age)
		}

		return ages
	}

	for _, test := range []struct {
		ages []int32
		want string
	}{
		{span(40, maxAge), "from 40 years"},
		{span(0, 12), "up to 12 years"},
		{span(3, 12), "from 3 to 12 years"},
		{[]int32{7}, "at 7 years"},
		{append(span(0, 5), span(60, maxAge)...), "up to 5 years and from 60 years"},
		{span(0, maxAge), "at every age"},
	} {
		if got := ageSpans(test.ages); got != test.want {
			t.Errorf("%v = %q, want %q", test.ages, got, test.want)
		}
	}
}
