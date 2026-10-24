package app

import (
	"fmt"
	"slices"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// drawnParts are the entities the accessory genes of a portrait draw, in the
// order of the genes: those posed with the head, such as the eyes, the teeth,
// the hair and the beard; those posed with the torso, such as the clothes
// the portrait modifiers dress it in; and those hung from a joint, such as a
// cane the props gene puts in a hand. tags are the tags the accessories and
// their templates set, which choose among the entities of an accessory and
// the settings of genes.
type drawnParts struct {
	head, body []string
	hung       []propSpec
	tags       []string
}

// pickedAccessory is the accessory a gene picks, with the template it picked
// it from.
type pickedAccessory struct {
	gene      string
	template  *genes.AccessoryTemplate
	accessory *genes.Accessory
}

// accessoryParts are the entities a dressed portrait draws: what the
// accessory genes of its DNA pick, and in place of a gene's, or beside them,
// what its portrait modifiers add. A gene the DNA gives the clothes the game
// dresses a character in itself is left out, as the game leaves it, where no
// modifier adds it. A gene that picks an accessory that draws nothing, such
// as no hair, draws nothing; one that picks an accessory that is not there is
// reported.
//
// Every accessory is picked before any is drawn: the tags they and their
// templates set, such as a hat's hair_covered, decide which entity of each is
// drawn, such as a hairstyle that draws nothing under a hat.
func accessoryParts(all *genes.Genes, accessories database.Table[genes.Accessory], dressed genes.Dressed, sex genes.Sex) (drawn drawnParts, diagnostics report.Diagnostics) {
	var picked []pickedAccessory

	pick := func(key string, gene *genes.AccessoryGene, templateName string, strength float64, named string) {
		templates := byIndex(gene.Templates)
		if len(templates) == 0 {
			return
		}

		template := &templates[0]
		if chosen, ok := gene.Template(templateName); ok {
			template = chosen
		}

		name := named
		if name == "" {
			choice, ok := template.For(sex).Pick(strength)
			if !ok {
				return
			}

			name = choice.Accessory
		}

		if name == emptyAccessory {
			return
		}

		accessory, ok := accessories.Get(name)
		if !ok {
			diagnostics = append(diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "gene " + key, Message: fmt.Sprintf("there is no portrait accessory %s; drawn without it", name)})

			return
		}

		picked = append(picked, pickedAccessory{gene: key, template: template, accessory: accessory})
	}

	for key, gene := range all.AccessoryGenes.All() {
		if added, ok := dressed.Accessory(key); ok {
			pick(key, gene, added.Template, added.Strength, added.Accessory)

			continue
		}

		if clothing(all, key) {
			continue
		}

		allele, _ := dominant(dressed.DNA, key)
		pick(key, gene, allele.Template, allele.Strength(), "")
	}

	// The genes the modifiers add that no DNA gives, such as the clothes,
	// each once, as its last addition has it.
	var added []string

	for _, gene := range dressed.Added {
		if gene.Kind == "accessory" && !all.AccessoryGenes.Has(gene.Gene) && !slices.Contains(added, gene.Gene) {
			added = append(added, gene.Gene)
		}
	}

	for _, key := range added {
		gene, ok := all.SpecialAccessoryGenes.Get(key)
		if !ok {
			diagnostics = append(diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: "gene " + key, Message: "a portrait modifier adds a gene that is not there; drawn without it"})

			continue
		}

		last, _ := dressed.Accessory(key)
		pick(key, gene, last.Template, last.Strength, last.Accessory)
	}

	for _, accessory := range picked {
		drawn.tags = appendNew(drawn.tags, accessory.template.SetTags...)
		drawn.tags = appendNew(drawn.tags, accessory.accessory.SetTags...)
	}

	for _, accessory := range picked {
		chosen, ok := accessory.accessory.Choose(drawn.tags)

		switch {
		case !ok || chosen.Entity == "":
		case chosen.Node != "":
			drawn.hung = append(drawn.hung, propSpec{entity: chosen.Entity, joint: chosen.Node})
		case chosen.SharedPoseEntity == headPose:
			drawn.head = append(drawn.head, chosen.Entity)
		case chosen.SharedPoseEntity == torsoPart:
			drawn.body = append(drawn.body, chosen.Entity)
		}
	}

	return drawn, diagnostics
}

// appendNew appends the values not in a list already.
func appendNew(list []string, values ...string) []string {
	for _, value := range values {
		if !slices.Contains(list, value) {
			list = append(list, value)
		}
	}

	return list
}
