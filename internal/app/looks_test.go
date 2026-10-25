package app

import (
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// lookGroups are groups of portrait modifiers as the games write them: clothes
// with a fallback, a group the game always applies at a weight no condition
// changes, a group of conditional modifiers, and one only animations use.
func lookGroups(t *testing.T) database.Table[genes.ModifierGroup] {
	t.Helper()

	collector := &report.Collector{}
	table := genes.ReadModifiers(script.Parse(`
clothes = {
	usage = game
	fallback = default_clothes
	priority = 1
	default_clothes = { dna_modifiers = { accessory = { mode = add gene = outfits template = peasant_outfits value = 0.5 } } weight = { base = 1 } }
	coat = { dna_modifiers = { accessory = { mode = add gene = coats template = european_coats value = 1 } } weight = { base = 0 modifier = { add = 10 is_ruler = yes } } }
}
special_shrink_body = {
	usage = game
	shrink_body = { dna_modifiers = { morph = { mode = add gene = gene_shrink_body template = shrink_all value = 1 } } weight = { base = 10 } }
}
headgear = {
	usage = game
	priority = 3
	top_hat = { dna_modifiers = { accessory = { mode = add gene = headgear template = top_hats value = 1 } } weight = { base = 0 modifier = { add = 10 is_ruler = yes } } }
}
animations_props = {
	usage = none
	saber = { dna_modifiers = { accessory = { mode = add gene = props template = sabers value = 1 } } }
}
`), folders.File{Relative: "gfx/portraits/portrait_modifiers/test.txt", Path: "test.txt", Source: "test"}, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected: %s", diagnostic)
	}

	return table
}

// names are the names of modifiers.
func names(modifiers []*genes.PortraitModifier) []string {
	var found []string
	for _, modifier := range modifiers {
		found = append(found, modifier.Name)
	}

	return found
}

// Until dressing is turned on the portrait is not dressed, whatever is
// picked; once it is, every group the game picks from on its own dresses it
// in the look picked, its fallback, or the modifier it always applies, in the
// order of their priorities, and a group set to nothing draws nothing but its
// fallback.
func TestLooksDressInThePicked(t *testing.T) {
	l := looks{groups: groupList(lookGroups(t))}

	l.pick("headgear", "top_hat")

	if l.dressing() || len(l.modifiers()) != 0 {
		t.Fatalf("looks not turned on give %v", names(l.modifiers()))
	}

	l.on = true

	if got := names(l.modifiers()); len(got) != 3 || got[0] != "shrink_body" || got[1] != "default_clothes" || got[2] != "top_hat" {
		t.Errorf("with a hat = %v, want the body shrunk, the fallback clothes and the hat, by priority", got)
	}

	l.pick("clothes", "coat")
	l.pick("special_shrink_body", "")
	l.pick("headgear", "")

	if got := names(l.modifiers()); len(got) != 1 || got[0] != "coat" {
		t.Errorf("with a coat and nothing else = %v, want the coat alone", got)
	}

	l.pick("clothes", "")

	if got := names(l.modifiers()); len(got) != 1 || got[0] != "default_clothes" {
		t.Errorf("with no clothes picked = %v, want the fallback", got)
	}
}

// A strength picked within the range an element gives a gene takes the place
// of the range, for that element alone, and the element as the game reads it
// keeps its range.
func TestLooksPickAStrengthWithinARange(t *testing.T) {
	collector := &report.Collector{}
	table := genes.ReadModifiers(script.Parse(`
clothes = {
	usage = game
	coat = { dna_modifiers = { accessory = { mode = add gene = outfits template = coats range = { 0 1 } } } }
	cape = { dna_modifiers = { accessory = { mode = add gene = outfits template = capes range = { 0 1 } } } }
}
`), folders.File{Relative: "gfx/portraits/portrait_modifiers/test.txt", Path: "test.txt", Source: "test"}, collector)

	l := looks{groups: groupList(table), on: true}
	l.pick("clothes", "coat")
	l.setStrength("clothes", "coat", 0, 0.9)
	l.setStrength("clothes", "cape", 0, 0.1)

	drawn := l.modifiers()
	if len(drawn) != 1 || drawn[0].Genes[0].Value != database.Some(0.9) || drawn[0].Genes[0].Range.Set {
		t.Fatalf("the coat is drawn with %+v, want the strength 0.9 in the place of its range", drawn)
	}

	if clothes, _ := table.Get("clothes"); !clothes.Modifiers[0].Genes[0].Range.Set {
		t.Error("the coat as the game reads it lost its range")
	}

	l.unsetStrength("clothes", "coat", 0)

	if drawn := l.modifiers(); !drawn[0].Genes[0].Range.Set {
		t.Errorf("unset, the coat is drawn with %+v, want its range again", drawn[0].Genes[0])
	}
}
