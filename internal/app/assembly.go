package app

import (
	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	staged "github.com/kaiser-chris/pdx-parser-go/portrait"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// assembly is how the game puts a portrait together, as its
// gfx/portraits/portraitsettings.txt says: the body and the head each sex is
// drawn with, and the joints the head is attached to the body by. The games
// differ in both. Victoria 3 draws a boy with the body of a man and attaches
// the head at the chest and the collarbones; Europa Universalis 5 gives a boy
// a body of his own and attaches the head at the base of the neck.
type assembly struct {
	staged.Assembly
}

// fallbackAssembly is Victoria 3's, for folders that do not say, which
// draws a portrait the way the creator always has.
var fallbackAssembly = assembly{staged.Assembly{
	Entities: map[string]staged.Entities{
		"male":   {Head: "male_head_entity", Torso: "male_body_entity"},
		"female": {Head: "female_head_entity", Torso: "female_body_entity"},
		"boy":    {Head: "male_head_entity", Torso: "male_body_entity"},
		"girl":   {Head: "female_head_entity", Torso: "female_body_entity"},
	},
	Attachments: []staged.Attachment{{What: headPart, Where: torsoPart, Joints: []staged.JointAttachment{
		{Parent: "bn_sp_thoracic", Child: "head_root"},
		{Parent: "bn_l_clavicle", Child: "bn_h_l_clavicle"},
		{Parent: "bn_r_clavicle", Child: "bn_h_r_clavicle"},
	}}},
	Root: torsoPart,
}}

// The parts of a portrait as the assembly names them.
const (
	headPart  = "head"
	torsoPart = "torso"
)

// loadAssembly reads how the game puts a portrait together. A game whose
// files do not say is put together as Victoria 3 is, which is reported.
func loadAssembly(set *folders.Set) (assembly, report.Diagnostics) {
	read, diagnostics := staged.LoadAssembly(set)

	return assembly{read}, diagnostics.Filter(report.SeverityWarning)
}

// entities are the body and the head a sex is drawn with. A boy and a girl
// are drawn as the game draws them, which the genes' settings for children
// and the age presets make a child of, whatever the body.
func (a assembly) entities(sex genes.Sex) (body, head string) {
	entities, ok := a.For(sex.String())
	if !ok || entities.Head == "" || entities.Torso == "" {
		entities, _ = fallbackAssembly.For(sex.String())
	}

	return entities.Torso, entities.Head
}

// links are the joints the head is attached to the body by, the first of
// which seats it, and all of which it follows wherever the body's
// animations take them.
func (a assembly) links() []render.JointLink {
	where := a.Root
	if where == "" {
		where = torsoPart
	}

	attachment, ok := a.Attachment(headPart, where)
	if !ok || len(attachment.Joints) == 0 {
		attachment, _ = fallbackAssembly.Attachment(headPart, torsoPart)
	}

	links := make([]render.JointLink, 0, len(attachment.Joints))
	for _, joint := range attachment.Joints {
		links = append(links, render.JointLink{Parent: joint.Parent, Child: joint.Child})
	}

	return links
}
