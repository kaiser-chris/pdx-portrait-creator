package app

import (
	"fmt"
	"slices"

	"github.com/kaiser-chris/pdx-asset-go/model"
	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// An animation variant can give the portrait props to hold, such as a sabre
// or a pipe, through the portrait modifiers it names:
//
//	saber_pop_idle = {
//		animation = { head = "idle_head_pop_saber" torso = "idle_body_saber" }
//		portrait_modifier = { animations_props = saber_attachment }
//	}
//
// A modifier sets a template of an accessory gene, props, which picks a
// portrait accessory, which draws an entity: hung from a joint of the body
// or the head, as a sabre is from the hand and a pipe from the lips, or
// posed with the head, as a top hat is. A prop that hangs from a joint is a
// model of its own, which moves with the joint; one posed with the head is a
// part of it, as the hair is.

// propSpec is a prop hung from a joint: the entity drawn, and the joint.
type propSpec struct {
	entity, joint string
}

// hungProp is a prop hung from a joint, uploaded.
type hungProp struct {
	propSpec
	model *render.Model
}

// loadedProp is a prop read off the thread that draws, to upload.
type loadedProp struct {
	propSpec
	model *model.Model
}

// propsOf are the props the portrait modifiers of an animation variant give
// a sex: the entities posed with the head, and those hung from a joint. A
// modifier, gene, template or accessory that is not there, and a prop drawn
// in a way the creator does not draw, are reported and left out.
func propsOf(modifiers database.Table[genes.ModifierGroup], all *genes.Genes, accessories database.Table[genes.Accessory], choices []genes.ModifierChoice, sex genes.Sex) (head []string, hung []propSpec, diagnostics report.Diagnostics) {
	problem := func(subject, format string, args ...any) {
		diagnostics = append(diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: subject, Message: fmt.Sprintf(format, args...) + "; drawn without it"})
	}

	for _, choice := range choices {
		subject := "portrait modifier " + choice.Group + " " + choice.Modifier

		group, ok := modifiers.Get(choice.Group)
		if !ok {
			problem(subject, "there is no portrait modifier group %s", choice.Group)

			continue
		}

		modifier, ok := group.Modifier(choice.Modifier)
		if !ok {
			problem(subject, "there is no portrait modifier %s in %s", choice.Modifier, choice.Group)

			continue
		}

		for _, change := range modifier.Genes {
			// Only accessories are props; a morph a modifier changes is
			// part of the face rather than something held.
			if change.Kind != "accessory" {
				continue
			}

			name, ok := propAccessory(all, change, sex)
			if !ok {
				problem(subject, "accessory gene %s has no template %s", change.Gene, change.Template)

				continue
			}

			if name == "" || name == emptyAccessory {
				continue
			}

			accessory, ok := accessories.Get(name)
			if !ok {
				problem(subject, "there is no portrait accessory %s", name)

				continue
			}

			chosen, ok := accessory.Choose(nil)
			switch {
			case !ok || chosen.Entity == "":
				continue
			case chosen.Node != "":
				hung = append(hung, propSpec{entity: chosen.Entity, joint: chosen.Node})
			case chosen.SharedPoseEntity == headPose:
				head = append(head, chosen.Entity)
			default:
				problem(subject, "%s is posed with the %s, which only the head's parts are", chosen.Entity, chosen.SharedPoseEntity)
			}
		}
	}

	return head, hung, diagnostics
}

// propAccessory is the portrait accessory a modifier of an accessory gene
// draws for a sex: the one it names, or the one its template picks at its
// strength.
func propAccessory(all *genes.Genes, change genes.GeneModifier, sex genes.Sex) (string, bool) {
	if change.Accessory != "" {
		return change.Accessory, true
	}

	gene, ok := all.AccessoryGene(change.Gene)
	if !ok {
		return "", false
	}

	template, ok := gene.Template(change.Template)
	if !ok {
		return "", false
	}

	choice, ok := template.For(sex).Pick(change.Strength())
	if !ok {
		return "", true
	}

	return choice.Accessory, true
}

// wantProps works out the props of what the portrait plays, when that has
// changed, and has the head read again with those posed with it, and the
// props hung from joints read, by rebuildHead.
func (p *portrait) wantProps() {
	key := p.animation.key(p.sex)
	if key == p.propsFor {
		return
	}

	p.propsFor = key

	head, hung, diagnostics := propsOf(p.modifiers, p.genes, p.accessories, p.animation.modifiers(p.sex), p.sex)
	p.diagnostics = append(p.diagnostics, diagnostics...)

	if !slices.Equal(head, p.headProps) || !slices.Equal(hung, p.wantedProps) {
		p.headProps, p.wantedProps = head, hung
		p.character.partsChanged = true
	}
}

// missingProps are the props hung from joints that are wanted and not hung
// yet.
func (p *portrait) missingProps() []propSpec {
	var missing []propSpec

	for _, wanted := range p.wantedProps {
		if !slices.ContainsFunc(p.props, func(hung hungProp) bool { return hung.propSpec == wanted }) {
			missing = append(missing, wanted)
		}
	}

	return missing
}

// takeProps uploads the props read, lets go of those no longer wanted, and
// hangs every prop from its joint, on the body or on the head, whichever has
// it.
func (p *portrait) takeProps(loaded []loadedProp) {
	for _, prop := range loaded {
		if !slices.Contains(p.wantedProps, prop.propSpec) {
			continue
		}

		uploaded, err := p.renderer.Upload(prop.model)
		if err != nil {
			warn(err)

			continue
		}

		p.props = append(p.props, hungProp{propSpec: prop.propSpec, model: uploaded})
	}

	p.props = slices.DeleteFunc(p.props, func(hung hungProp) bool {
		if slices.Contains(p.wantedProps, hung.propSpec) {
			return false
		}

		hung.model.Unload()

		return true
	})

	p.hangProps()
}

// hangProps hangs the props from their joints, which the head has to be
// done again for when it is read again.
func (p *portrait) hangProps() {
	for _, prop := range p.props {
		parent := p.body
		if _, _, ok := p.body.JointPosition(prop.joint); !ok {
			parent = p.head
		}

		prop.model.Hang(parent, prop.joint)

		if !prop.model.Hanging() {
			p.diagnostics = append(p.diagnostics, report.Diagnostic{
				Severity: report.SeverityWarning,
				Subject:  "entity " + prop.entity,
				Message:  fmt.Sprintf("neither the body nor the head has the joint %s it hangs from; drawn where its mesh puts it", prop.joint),
			})
		}
	}
}

// unloadProps lets go of every prop.
func (p *portrait) unloadProps() {
	for _, prop := range p.props {
		prop.model.Unload()
	}

	p.props, p.wantedProps, p.headProps, p.propsFor = nil, nil, nil, ""
}

// models are the models the portrait is drawn with: the body, the head and
// the props.
func (p *portrait) models() []*render.Model {
	drawn := []*render.Model{p.body, p.head}

	for _, prop := range p.props {
		drawn = append(drawn, prop.model)
	}

	return drawn
}
