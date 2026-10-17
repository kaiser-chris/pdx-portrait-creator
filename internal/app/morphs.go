package app

import (
	"slices"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-parser-go/genes"
)

// The values a DNA gives a gene run from 0 to 255. neutralValue is the one in
// the middle that the shipped DNA write for a gene left as it is.
const (
	maxGeneValue = 255
	neutralValue = 127
)

// How old a portrait is, in percent of a life, as the game's portrait editor
// gives it, which the age presets of the genes run over from 0 to 1. It starts
// halfway, where the editor's view of an ethnicity was compared.
const (
	defaultAge = 50
	maxAge     = 100
)

// portraitSex is whose settings of a gene morph the portrait: for now always
// a man's, as the portrait is the male body and head.
const portraitSex = genes.Male

// morphs are the morph genes of a portrait, as the character panel sets
// them.
type morphs struct {
	genes []morphGene

	// all are the genes of the folders, whose age presets age the morphs,
	// and age is how old the portrait is, in percent of a life.
	all *genes.Genes
	age int32

	// changed is set when a gene has changed since the portrait was last
	// morphed.
	changed bool

	// filter narrows the list in the panel to the genes whose name holds it.
	filter string
}

// morphGene is one morph gene: which of its templates is chosen, and how
// strong it is, as a DNA writes them.
type morphGene struct {
	gene     *genes.MorphGene
	template int
	value    int32

	// drawn says whether any template of the gene sets an attribute that
	// morphs the portrait's models, a blend shape or an additive animation,
	// or lays a decal over its skin. A gene that only replaces textures,
	// which are not drawn yet, changes nothing.
	drawn bool
}

// newMorphs lists the morph genes, set as a DNA has them. drawable are the attributes that morph the portrait's models.
func newMorphs(all *genes.Genes, dna *genes.DNA, drawable []string) morphs {
	list := morphs{all: all, age: defaultAge, changed: true}

	for _, gene := range all.MorphGenes.All() {
		if len(gene.Templates) == 0 {
			continue
		}

		morph := morphGene{gene: gene, value: neutralValue, drawn: laysDecals(gene)}

		for _, template := range gene.Templates {
			for _, setting := range template.For(portraitSex).Settings {
				morph.drawn = morph.drawn || slices.Contains(drawable, setting.Attribute)
			}
		}

		list.genes = append(list.genes, morph)
	}

	list.reset(dna)

	return list
}

// reset sets every gene as a DNA has it, its dominant allele, or to its first
// template at a neutral strength where the DNA does not say, or for no DNA at
// all.
func (m *morphs) reset(dna *genes.DNA) {
	for index := range m.genes {
		morph := &m.genes[index]
		morph.template, morph.value = 0, neutralValue

		if dna == nil {
			continue
		}

		value, ok := dna.Gene(morph.gene.Key)
		if !ok {
			continue
		}

		for position, template := range morph.gene.Templates {
			if template.Name == value.Dominant.Template {
				morph.template = position
				morph.value = int32(value.Dominant.Value)
			}
		}
	}

	m.changed = true
}

// attributes are the values the genes give the attributes of the portrait,
// by name, at its age: the age presets of the genes grow a child's features
// into a man's and age his face.
func (m *morphs) attributes() map[string]float64 {
	attributes := map[string]float64{}
	at := float64(m.age) / maxAge

	for _, morph := range m.genes {
		template := &morph.gene.Templates[morph.template]
		m.all.SetAttributes(template.For(portraitSex), float64(morph.value)/maxGeneValue, at, attributes)
	}

	return attributes
}

// panel lists the genes, each with a choice of its templates where it has
// more than one and a slider for its strength.
func (m *morphs) panel() {
	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##filter", "Filter genes", &m.filter, 0, nil)

	if !imgui.BeginChildStr("##genes") {
		imgui.EndChild()

		return
	}

	filter := strings.ToLower(strings.TrimSpace(m.filter))

	for index := range m.genes {
		morph := &m.genes[index]
		name := strings.TrimPrefix(morph.gene.Key, "gene_")

		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}

		imgui.PushIDStr(morph.gene.Key)

		if m.row(morph, name) {
			m.changed = true
		}

		imgui.PopID()
	}

	imgui.EndChild()
}

// labelShare is the share of the panel's width the name of a gene takes.
const labelShare = 0.4

// row draws one gene, and reports whether it was changed.
func (m *morphs) row(morph *morphGene, name string) bool {
	changed := false
	width := imgui.ContentRegionAvail().X

	if morph.drawn {
		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted(name)
	} else {
		imgui.AlignTextToFramePadding()
		imgui.TextDisabled(name)
		imgui.SetItemTooltip("Morphs nothing drawn yet: a texture")
	}

	imgui.SameLineV(width*labelShare, -1)
	imgui.SetNextItemWidth(-1)

	if imgui.SliderIntV("##value", &morph.value, 0, maxGeneValue, "%d", imgui.SliderFlagsAlwaysClamp) {
		changed = true
	}

	if len(morph.gene.Templates) < 2 {
		return changed
	}

	imgui.SetCursorPosX(imgui.CursorPosX() + width*labelShare)
	imgui.SetNextItemWidth(-1)

	if imgui.BeginCombo("##template", morph.gene.Templates[morph.template].Name) {
		for position, template := range morph.gene.Templates {
			if imgui.SelectableBoolV(template.Name, position == morph.template, 0, imgui.Vec2{}) && position != morph.template {
				morph.template = position
				changed = true
			}
		}

		imgui.EndCombo()
	}

	return changed
}
