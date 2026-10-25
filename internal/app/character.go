package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/genes"
)

// The values a DNA gives a gene run from 0 to 255. neutralValue is the one in
// the middle that the shipped DNA write for a gene left as it is.
const (
	maxGeneValue = 255
	neutralValue = 127
)

// How old a portrait is, in years, which the age presets of the genes run
// over from 0 to 1: their curves are written as though 1 were a hundred
// years. It starts at twenty-five, a grown character in the prime of life.
const (
	defaultAge = 25
	maxAge     = 100
)

// character is the DNA a portrait is drawn from, as the character panel
// edits it: every morph gene, accessory gene and colour gene of the game,
// each at the template and the value it is set to, and how old the
// character is and whether a man or a woman.
type character struct {
	// all are the genes of the folders, whose age presets age the morphs.
	all *genes.Genes

	sex genes.Sex
	age int32

	morphs      []morphGene
	accessories []accessoryGene
	colours     []colourGene

	// basis is the DNA the character started from, which resetting goes
	// back to and the panel counts the changes from.
	basis *genes.DNA

	// changed is set when a morph gene or the age has changed since the
	// portrait was last morphed, partsChanged when an accessory gene has,
	// which may draw the head with other parts, and coloursChanged when a
	// colour gene has.
	changed, partsChanged, coloursChanged bool

	// drawable are the attributes that morph the portrait's models, and
	// idleAt the sex and the age the morph genes were last found to do
	// something or nothing at; see idle.go.
	drawable map[string]bool
	idleAt   idleFor

	// looks are the portrait modifiers picked to dress the character in, and
	// tags the tags what it is drawn with sets, such as shrink_legs under
	// trousers, which settings of genes require; see looks.go.
	looks looks
	tags  []string
}

// morphGene is one morph gene: which of its templates is chosen, and how
// strong it is, as a DNA writes them.
type morphGene struct {
	gene     *genes.MorphGene
	template int
	value    int32

	// category and name are where the catalogue lists the gene and what it
	// calls it.
	category int
	name     string

	// drawn says whether any template of the gene does anything the portrait
	// draws for the character's sex: sets an attribute that morphs its
	// models, a blend shape or an additive animation, lays a decal over its
	// skin, replaces the skin's textures or shifts a colour.
	drawn bool

	// idle says why the gene changes nothing at the character's sex and
	// age, for the panel to say, and is empty where it does; see idle.go.
	idle string
}

// accessoryGene is one accessory gene: which of its templates is chosen, in
// the order of their index, and the strength that picks one of its
// accessories.
type accessoryGene struct {
	gene      *genes.AccessoryGene
	templates []genes.AccessoryTemplate
	template  int
	value     int32

	category int
	name     string

	// hidden is set for the parts of the head the panel does not list,
	// and for the clothes the game dresses a character in itself.
	hidden bool
}

// colourGene is one colour gene: the place in its palette a DNA gives it.
type colourGene struct {
	gene  *genes.ColorGene
	value [2]int32

	category int
	name     string
}

// newCharacter lists the genes of the folders, set as a DNA has them, for a
// sex. drawable are the attributes that morph the portrait's models.
func newCharacter(all *genes.Genes, sex genes.Sex, dna *genes.DNA, drawable []string) *character {
	index := newCatalogue()
	c := &character{all: all, sex: sex, age: defaultAge}
	c.setDrawable(drawable)

	for _, gene := range all.ColorGenes.All() {
		category, name := index.place(gene.Key)
		c.colours = append(c.colours, colourGene{gene: gene, category: category, name: name})
	}

	for _, gene := range all.MorphGenes.All() {
		if len(gene.Templates) == 0 {
			continue
		}

		category, name := index.place(gene.Key)
		c.morphs = append(c.morphs, morphGene{gene: gene, value: neutralValue, category: category, name: name})
	}

	for _, gene := range all.AccessoryGenes.All() {
		if len(gene.Templates) == 0 {
			continue
		}

		category, name := index.place(gene.Key)

		c.accessories = append(c.accessories, accessoryGene{
			gene:      gene,
			templates: byIndex(gene.Templates),
			category:  category,
			name:      name,
			hidden:    hiddenPart(gene) || clothing(all, gene.Key),
		})
	}

	c.reset(dna)
	c.refreshIdle()

	return c
}

// byIndex returns the templates of an accessory gene in the order of their
// index, which is the order a DNA's neutral value starts from.
func byIndex(templates []genes.AccessoryTemplate) []genes.AccessoryTemplate {
	sorted := slices.Clone(templates)
	slices.SortStableFunc(sorted, func(a, b genes.AccessoryTemplate) int { return a.Index - b.Index })

	return sorted
}

// reset sets every gene as a DNA has it, its dominant allele, and keeps the
// DNA as what the character started from. A gene the DNA does not give, or
// that names a template the gene does not have, is at its first template and
// neutral, as is every gene for no DNA at all.
func (c *character) reset(dna *genes.DNA) {
	c.basis = dna

	for index := range c.morphs {
		morph := &c.morphs[index]
		morph.template, morph.value = 0, neutralValue

		if allele, ok := dominant(dna, morph.gene.Key); ok {
			if position := slices.IndexFunc(morph.gene.Templates, func(t genes.MorphTemplate) bool { return t.Name == allele.Template }); position >= 0 {
				morph.template, morph.value = position, int32(allele.Value)
			}
		}
	}

	for index := range c.accessories {
		accessory := &c.accessories[index]
		accessory.template, accessory.value = 0, 0

		if allele, ok := dominant(dna, accessory.gene.Key); ok {
			if position := slices.IndexFunc(accessory.templates, func(t genes.AccessoryTemplate) bool { return t.Name == allele.Template }); position >= 0 {
				accessory.template, accessory.value = position, int32(allele.Value)
			}
		}
	}

	for index := range c.colours {
		colour := &c.colours[index]
		colour.value = [2]int32{}

		if allele, ok := dominant(dna, colour.gene.Key); ok {
			colour.value = [2]int32{int32(allele.Value), int32(allele.Second)}
		}
	}

	c.changed, c.partsChanged, c.coloursChanged = true, true, true
}

// dominant is the allele a DNA shows of a gene.
func dominant(dna *genes.DNA, gene string) (genes.Allele, bool) {
	if dna == nil {
		return genes.Allele{}, false
	}

	value, ok := dna.Gene(gene)
	if !ok {
		return genes.Allele{}, false
	}

	return value.Dominant, true
}

// dna is the character as a DNA, every gene's dominant and recessive allele
// alike: what a sex is loaded again with, so that changing it keeps the
// face.
func (c *character) dna() *genes.DNA {
	dna := &genes.DNA{Enabled: true}

	add := func(key string, allele genes.Allele) {
		dna.Genes = append(dna.Genes, genes.GeneValue{Gene: key, Dominant: allele, Recessive: allele})
	}

	for _, colour := range c.colours {
		allele := genes.Allele{Value: int(colour.value[0]), Second: int(colour.value[1])}
		dna.Genes = append(dna.Genes, genes.GeneValue{Gene: colour.gene.Key, Colour: true, Dominant: allele, Recessive: allele})
	}

	for _, morph := range c.morphs {
		add(morph.gene.Key, genes.Allele{Template: morph.gene.Templates[morph.template].Name, Value: int(morph.value)})
	}

	for _, accessory := range c.accessories {
		add(accessory.gene.Key, genes.Allele{Template: accessory.templates[accessory.template].Name, Value: int(accessory.value)})
	}

	return dna
}

// at is how far through a life the character is, from 0 to 1, as the age
// presets take it.
func (c *character) at() float64 {
	return float64(c.age) / maxAge
}

// attributes are the values the genes give the attributes of the portrait,
// by name, at its age: the age presets of the genes grow a child's features
// into an adult's and age the face.
func (c *character) attributes() map[string]float64 {
	return c.attributesWith(nil, 0)
}

// attributesWith are the attributes with some of the morph genes, by their
// keys, at another value than they are set to, which is how the portrait is
// morphed to find what a gene moves.
func (c *character) attributesWith(keys []string, value int32) map[string]float64 {
	if c.looks.dressing() {
		return c.dressedAttributes(keys, value)
	}

	attributes := map[string]float64{}

	for _, morph := range c.morphs {
		strength := morph.value
		if slices.Contains(keys, morph.gene.Key) {
			strength = value
		}

		template := &morph.gene.Templates[morph.template]
		c.all.SetAttributes(template.For(c.sex), float64(strength)/maxGeneValue, c.at(), attributes)
	}

	return attributes
}

// dressedAttributes are attributesWith for a character dressed in looks: its
// morph genes as the modifiers leave them, with the settings their tags
// require made only where the character has them, and the morph genes the
// modifiers add beside them, such as the shrinking of a body under its
// clothes.
func (c *character) dressedAttributes(keys []string, value int32) map[string]float64 {
	attributes := map[string]float64{}
	at := c.at()

	for _, morph := range c.drawnMorphs() {
		strength := morph.strength
		if !morph.added && slices.Contains(keys, morph.gene.Key) {
			strength = float64(value) / maxGeneValue
		}

		c.all.SetAttributesTagged(morph.template.For(c.sex), strength, at, c.tags, attributes)
	}

	return attributes
}

// drawnMorph is a morph gene as the portrait is drawn with it: its template
// and its strength from 0 to 1, and whether a portrait modifier added it
// beside the DNA's genes.
type drawnMorph struct {
	gene     *genes.MorphGene
	template *genes.MorphTemplate
	strength float64
	added    bool
}

// drawnMorphs are the morph genes the portrait is drawn with: the DNA's, as
// the portrait modifiers it is dressed in leave them, and the genes they add
// beside them, special genes among them, such as the decals of a wound. They
// are what lays the decals, replaces the skin's textures and shifts the
// colours, as well as what morphs the models.
func (c *character) drawnMorphs() []drawnMorph {
	drawn := make([]drawnMorph, 0, len(c.morphs))

	var dressed genes.Dressed
	if c.looks.dressing() {
		dressed = c.dressed()
	}

	for _, morph := range c.morphs {
		template, strength := &morph.gene.Templates[morph.template], float64(morph.value)/maxGeneValue

		if allele, ok := dominant(dressed.DNA, morph.gene.Key); ok {
			if changed, ok := morph.gene.Template(allele.Template); ok {
				template, strength = changed, float64(allele.Value)/maxGeneValue
			}
		}

		drawn = append(drawn, drawnMorph{gene: morph.gene, template: template, strength: strength})
	}

	for _, added := range dressed.Morphs() {
		gene, ok := c.all.MorphGene(added.Gene)
		if !ok {
			continue
		}

		if template, ok := gene.Template(added.Template); ok {
			drawn = append(drawn, drawnMorph{gene: gene, template: template, strength: added.Strength, added: true})
		}
	}

	return drawn
}

// picked is the accessory an accessory gene picks as it is set, and false
// where it picks none.
func (c *character) picked(accessory *accessoryGene) (string, bool) {
	choice, ok := accessory.templates[accessory.template].For(c.sex).Pick(float64(accessory.value) / maxGeneValue)
	if !ok {
		return "", false
	}

	return choice.Accessory, true
}

// neutralBasis is the basis with every gene it leaves out at its neutral
// value, as reset sets them, so that a gene the basis does not give counts
// as changed only once it is moved.
func (c *character) neutralBasis() *genes.DNA {
	reset := &character{all: c.all, sex: c.sex, morphs: slices.Clone(c.morphs), accessories: slices.Clone(c.accessories), colours: slices.Clone(c.colours)}
	reset.reset(c.basis)

	return reset.dna()
}

// alleleOf is a gene's dominant allele in a DNA, as numbers and a name to
// compare.
func alleleOf(dna *genes.DNA, key string) []string {
	allele, ok := dominant(dna, key)
	if !ok {
		return nil
	}

	return []string{allele.Template, strconv.Itoa(allele.Value), strconv.Itoa(allele.Second)}
}

// changes are the genes that are set otherwise than the character started
// from, by key, and how many of them each category holds, by the position of
// the category.
func (c *character) changes() (keys map[string]bool, counts map[int]int) {
	now, basis := c.dna(), c.neutralBasis()
	keys, counts = map[string]bool{}, map[int]int{}

	count := func(key string, category int) {
		if !slices.Equal(alleleOf(now, key), alleleOf(basis, key)) {
			keys[key] = true
			counts[category]++
		}
	}

	for _, morph := range c.morphs {
		count(morph.gene.Key, morph.category)
	}

	for _, accessory := range c.accessories {
		if !accessory.hidden {
			count(accessory.gene.Key, accessory.category)
		}
	}

	for _, colour := range c.colours {
		count(colour.gene.Key, colour.category)
	}

	return keys, counts
}

// colour is the colour gene that colours one of hair, skin or eye.
func (c *character) colour(what string) (*colourGene, bool) {
	for index := range c.colours {
		if c.colours[index].gene.Color == what {
			return &c.colours[index], true
		}
	}

	return nil, false
}

// ageOf is the age in years of an age a DNA writes, from 0 to 1.
func ageOf(age float64) int32 {
	return int32(min(max(age, 0), 1) * maxAge)
}

// templatesOf names the templates some morph genes are at, by their keys,
// which is what tells apart what they move.
func (c *character) templatesOf(keys []string) string {
	var names []string

	for _, morph := range c.morphs {
		if slices.Contains(keys, morph.gene.Key) {
			names = append(names, morph.gene.Templates[morph.template].Name)
		}
	}

	return strings.Join(names, ",")
}

// listed counts the genes the character panel lists.
func (c *character) listed() int {
	count := len(c.morphs) + len(c.colours)

	for _, accessory := range c.accessories {
		if !accessory.hidden {
			count++
		}
	}

	return count
}

// lists reports whether the character panel lists any gene under a category,
// by its position. A category the game has no gene of is left out, such as
// Props in a game without a props gene, and Other when every gene is known.
func (c *character) lists(category int) bool {
	for _, colour := range c.colours {
		if colour.category == category {
			return true
		}
	}

	for _, morph := range c.morphs {
		if morph.category == category {
			return true
		}
	}

	for _, accessory := range c.accessories {
		if !accessory.hidden && accessory.category == category {
			return true
		}
	}

	return false
}

// morphKeys are the keys of the morph genes of a category, by its position,
// which pointing at the category highlights the change of.
func (c *character) morphKeys(category int) []string {
	var keys []string

	for _, morph := range c.morphs {
		if morph.category == category {
			keys = append(keys, morph.gene.Key)
		}
	}

	return keys
}

// morphOrder are the positions of the morph genes in the order the panel
// lists them: by category, in the order the catalogue gives them, and those
// it does not know in the order of the files.
func (c *character) morphOrder() []int {
	place := map[string]int{}

	for _, category := range categories {
		for position, gene := range category.genes {
			place[gene.key] = position
		}
	}

	order := make([]int, len(c.morphs))
	for index := range order {
		order[index] = index
	}

	slices.SortStableFunc(order, func(a, b int) int {
		first, second := c.morphs[a], c.morphs[b]
		if first.category != second.category {
			return first.category - second.category
		}

		at, known := place[first.gene.Key]
		bt, alsoKnown := place[second.gene.Key]

		switch {
		case known && alsoKnown:
			return at - bt
		case known:
			return -1
		case alsoKnown:
			return 1
		default:
			return 0
		}
	})

	return order
}

// resetGene sets one gene back to what the character started from, or
// neutral where that does not give it.
func (c *character) resetGene(key string) {
	basis := &character{all: c.all, sex: c.sex, morphs: slices.Clone(c.morphs), accessories: slices.Clone(c.accessories), colours: slices.Clone(c.colours)}
	basis.reset(c.basis)

	for index := range c.morphs {
		if c.morphs[index].gene.Key == key {
			c.morphs[index].template, c.morphs[index].value = basis.morphs[index].template, basis.morphs[index].value
			c.changed = true
		}
	}

	for index := range c.accessories {
		if c.accessories[index].gene.Key == key {
			c.accessories[index].template, c.accessories[index].value = basis.accessories[index].template, basis.accessories[index].value
			c.partsChanged = true
		}
	}

	for index := range c.colours {
		if c.colours[index].gene.Key == key {
			c.colours[index].value = basis.colours[index].value
			c.coloursChanged = true
		}
	}
}
