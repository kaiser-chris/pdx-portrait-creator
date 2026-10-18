package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// The props of a variant are followed from its modifiers through the props
// gene to the accessories: one hung from a joint, one posed with the head,
// one the modifier names outright, and what cannot be found is reported.
func TestPropsOf(t *testing.T) {
	file := folders.File{Relative: "test.txt"}
	collector := &report.Collector{}

	all := genes.Read(script.Parse(`
accessory_genes = {
	props = {
		no_prop = { index = 0 male = { 1 = empty } }
		gene_saber = { index = 1 male = { 1 = prop_saber } }
		gene_hat = { index = 2 male = { 1 = prop_top_hat } }
	}
}`), file, collector)

	accessories := genes.ReadAccessories(script.Parse(`
prop_saber = { entity = { required_tags = "" node = "bn_l_prop" entity = "prop_saber_entity" } }
prop_top_hat = { entity = { required_tags = "" shared_pose_entity = head entity = "prop_top_hat_entity" } }
prop_pipe = { entity = { required_tags = "" node = "bn_h_mouth_r_lowerLip" entity = "prop_pipe_entity" } }
prop_apron = { entity = { required_tags = "" shared_pose_entity = torso entity = "prop_apron_entity" } }
`), file, collector)

	modifiers := genes.ReadModifiers(script.Parse(`
animations_props = {
	usage = none
	saber_attachment = { dna_modifiers = { accessory = { mode = add gene = props template = gene_saber value = 1 } } }
	hat_attachment = { dna_modifiers = { accessory = { mode = add gene = props template = gene_hat value = 1 } } }
	pipe_attachment = { dna_modifiers = { accessory = { mode = add gene = props template = no_prop accessory = prop_pipe } } }
	nothing_attachment = { dna_modifiers = { accessory = { mode = add gene = props template = no_prop value = 1 } } }
	apron_attachment = { dna_modifiers = { accessory = { mode = add gene = props template = no_prop accessory = prop_apron } } }
	broken_attachment = { dna_modifiers = { accessory = { mode = add gene = props template = gene_missing value = 1 } } }
}`), file, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Fatalf("the test's own files: %s", diagnostic)
	}

	choose := func(names ...string) []genes.ModifierChoice {
		var choices []genes.ModifierChoice
		for _, name := range names {
			choices = append(choices, genes.ModifierChoice{Group: "animations_props", Modifier: name})
		}

		return choices
	}

	played, diagnostics := propsOf(modifiers, all, accessories, choose("saber_attachment", "hat_attachment", "pipe_attachment", "nothing_attachment"), genes.Male)

	if !slices.Equal(played.head, []string{"prop_top_hat_entity"}) {
		t.Errorf("posed with the head: %v, want the top hat", played.head)
	}

	if want := []propSpec{{entity: "prop_saber_entity", joint: "bn_l_prop"}, {entity: "prop_pipe_entity", joint: "bn_h_mouth_r_lowerLip"}}; !slices.Equal(played.hung, want) {
		t.Errorf("hung: %v, want %v", played.hung, want)
	}

	if len(diagnostics) != 0 {
		t.Errorf("diagnostics: %v", diagnostics)
	}

	// A prop posed with the torso, a missing template and a missing
	// modifier are each reported, and drawn without.
	played, diagnostics = propsOf(modifiers, all, accessories, choose("apron_attachment", "broken_attachment", "lost_attachment"), genes.Male)

	if len(played.head) != 0 || len(played.hung) != 0 || len(diagnostics) != 3 {
		t.Fatalf("played %+v, diagnostics %v; want nothing, and three reasons", played, diagnostics)
	}

	for index, fragment := range []string{"posed with the torso", "no template gene_missing", "no portrait modifier lost_attachment"} {
		if !strings.Contains(diagnostics[index].Message, fragment) {
			t.Errorf("diagnostic %q, want it to say %q", diagnostics[index].Message, fragment)
		}
	}
}
