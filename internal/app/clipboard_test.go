package app

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// pastedGenes are two genes, which a DNA string holds eight bytes of.
func pastedGenes(t *testing.T) *genes.Genes {
	t.Helper()

	return genes.Read(script.Parse(`
color_genes = { hair_color = { color = hair } }
morph_genes = { gene_nose = { small = { index = 0 } large = { index = 1 } } }
`), folders.File{Relative: "common/genes/test.txt"}, &report.Collector{})
}

// stringOf is a DNA string of the bytes of pastedGenes.
func stringOf(data ...byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Each of the three ways the game writes a DNA is told apart, with its key
// but for a string; comments are left alone, so a whole file is read for its
// definition rather than the string in the comment above it; and what is no
// DNA, or a string of other genes, cannot be started from.
func TestReadPasted(t *testing.T) {
	all := pastedGenes(t)

	tests := []struct {
		name, text string
		kind       pastedKind
		key        string
		nose       int
		sex        string
		why        string
	}{
		{
			name: "a whole file",
			text: string(rune(0xFEFF)) + "# " + stringOf(1, 2, 3, 4, 1, 99, 1, 99) + "\n\ndna_test = {\n\tportrait_info = { genes = { gene_nose = { \"large\" 10 \"small\" 20 } } }\n\tenabled = yes\n}\n",
			kind: pastedDefinition, key: "dna_test", nose: 10,
		},
		{
			name: "persistent DNA",
			text: "dna_old={\n\ttype=girl\n\tid=0\n\tage=0.100000\n\tgenes={ gene_nose={ \"small\" 30 \"small\" 30 } }\n\tentity={ 0 0 }\n}",
			kind: pastedPersistent, key: "dna_old", nose: 30, sex: "girl",
		},
		{
			name: "a definition of a string",
			text: `dna_short = { dna = "` + stringOf(1, 2, 3, 4, 1, 40, 0, 50) + `" }`,
			kind: pastedDefinition, key: "dna_short", nose: 40,
		},
		{
			name: "a string",
			text: "  " + stringOf(1, 2, 3, 4, 1, 50, 0, 60) + "\r\n",
			kind: pastedString, nose: 50,
		},
		{
			name: "a string of other genes",
			text: stringOf(1, 2, 3, 4, 1, 50, 0, 60, 0, 0, 0, 0),
			kind: pastedString, why: "other genes",
		},
		{name: "a string in a comment", text: "# " + stringOf(1, 2, 3, 4, 1, 50, 0, 60)},
		{name: "words", text: "a portrait of my uncle"},
		{name: "a word", text: "hello"},
		{name: "other script", text: "building_steel_mill = { level = 3 }"},
		{name: "nothing", text: "   "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found := readPasted(test.text, all)

			usable := test.kind != pastedNothing && test.why == ""

			if found.kind != test.kind || found.key != test.key || found.usable() != usable || !strings.Contains(found.why, test.why) {
				t.Fatalf("found %+v, want kind %d, key %q, why %q", found, test.kind, test.key, test.why)
			}

			if !found.usable() {
				return
			}

			nose, ok := found.dna.Gene("gene_nose")
			if !ok || nose.Dominant.Value != test.nose || found.dna.Sex != test.sex {
				t.Errorf("read %+v, want the nose at %d and sex %q", found.dna, test.nose, test.sex)
			}

			if found.kind != pastedString && !strings.Contains(found.describe(), test.key) {
				t.Errorf("described as %q, without its key %s", found.describe(), test.key)
			}
		})
	}
}
