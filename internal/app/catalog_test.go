package app

import (
	"slices"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/genes"
)

// alike are the genes of Europa Universalis 5 that are Victoria 3's under
// another key, by their key, which share its name: a game has only one of
// them, so the panel never shows both.
var alike = map[string]string{
	"gene_eye_spacing":         "gene_eye_distance",
	"gene_eye_open":            "gene_eye_shut",
	"gene_mouth_lower_lip_pad": "gene_mouth_lower_lip_pads",
	"poses_height":             "gene_height",
	"gene_aging_eyes":          "gene_old_eyes",
	"gene_aging_forehead":      "gene_old_forehead",
	"gene_aging_mouth":         "gene_old_mouth",
	"hair_styles":              "hairstyles",
}

// Every gene the catalogue knows is in one category, under a name of its
// own there but for the genes alike, and Other and Colours are where the
// panel expects them.
func TestCatalogue(t *testing.T) {
	seen := map[string]string{}

	for _, category := range categories {
		names := map[string]string{}

		for _, gene := range category.genes {
			if other, ok := seen[gene.key]; ok {
				t.Errorf("%s is in %s and %s", gene.key, other, category.id)
			}

			if other, ok := names[gene.name]; ok && alike[gene.key] != other && alike[other] != gene.key {
				t.Errorf("%s is named %q twice", category.id, gene.name)
			}

			seen[gene.key], names[gene.name] = category.id, gene.key
		}
	}

	index := newCatalogue()

	if category, name := index.place("gene_nose_size"); categories[category].id != "nose" || name != "Nose Size" {
		t.Errorf("gene_nose_size is %s %q", categories[category].id, name)
	}

	// A gene the catalogue does not know, such as a mod's, is Other, by its
	// key.
	if category, name := index.place("gene_mod_freckles"); categories[category].id != categoryOther || name != "gene_mod_freckles" {
		t.Errorf("gene_mod_freckles is %s %q", categories[category].id, name)
	}
}

// The keys of the game's files are shown as words.
func TestFriendly(t *testing.T) {
	for key, want := range map[string]string{
		"european_hairstyles":   "European Hairstyles",
		"male_hair_european_01": "Male Hair European 01",
		"abraham_lincoln":       "Abraham Lincoln",
		"":                      "",
		"__odd__key":            "Odd Key",
	} {
		if got := friendly(key); got != want {
			t.Errorf("friendly(%q) = %q, want %q", key, got, want)
		}
	}
}

// A strength is shown from -100 to 100 around the middle a neutral DNA leaves
// a gene at, as a label that holds no verb of a format.
func TestSigned(t *testing.T) {
	for value, want := range map[int32]string{0: "-100", 127: "0", 128: "0", 140: "+10", 255: "+100"} {
		if got := signed(value); got != want {
			t.Errorf("signed(%d) = %q, want %q", value, got, want)
		}
	}
}

// The search of a list of templates finds them by any part of the names the
// game gives them, whatever the case, and an empty one finds them all.
func TestMatching(t *testing.T) {
	names := []string{"no_hairstyles", "arabic_hairstyles", "european_hairstyles", "Arabian_test"}

	for _, test := range []struct {
		search string
		want   []int
	}{
		{"", []int{0, 1, 2, 3}},
		{"  ", []int{0, 1, 2, 3}},
		{"arab", []int{1, 3}},
		{"EURO", []int{2}},
		{"styles", []int{0, 1, 2}},
		{"nothing", nil},
	} {
		if got := matching(names, test.search); !slices.Equal(got, test.want) {
			t.Errorf("matching %q = %v, want %v", test.search, got, test.want)
		}
	}
}

// A category is listed only where the character has a gene in it: the
// colours, a morph gene, or an accessory gene that is not hidden.
func TestListedCategories(t *testing.T) {
	head, props, colours, hair := 0, 0, 0, 0

	for index, category := range categories {
		switch category.id {
		case "head":
			head = index
		case "props":
			props = index
		case categoryColours:
			colours = index
		case "hair":
			hair = index
		}
	}

	c := &character{
		morphs:      []morphGene{{category: head}},
		accessories: []accessoryGene{{category: hair}, {category: props, hidden: true}},
		colours:     []colourGene{{category: colours}},
	}

	for index, category := range categories {
		want := index == head || index == hair || index == colours
		if got := c.lists(index); got != want {
			t.Errorf("%s listed %v, want %v", category.id, got, want)
		}
	}
}

// A gene that is not drawn says why: its templates set nothing, as Europa
// Universalis 5's head size, only change a texture, or set attributes no
// model has.
func TestIdleReason(t *testing.T) {
	gene := func(morph genes.Morph) *genes.MorphGene {
		template := genes.MorphTemplate{Name: "template_1"}
		template.Sexes[genes.Male] = morph

		return &genes.MorphGene{Templates: []genes.MorphTemplate{template}}
	}

	for _, test := range []struct {
		morph genes.Morph
		want  string
	}{
		{genes.Morph{}, idleEmpty},
		{genes.Morph{TextureOverrides: []genes.TextureOverride{{}}}, idleTexture},
		{genes.Morph{Settings: []genes.Setting{{Attribute: "head_size"}}}, idleAttributes},
	} {
		if got := idleReason(gene(test.morph), genes.Male); got != test.want {
			t.Errorf("%+v: %q, want %q", test.morph, got, test.want)
		}
	}
}
