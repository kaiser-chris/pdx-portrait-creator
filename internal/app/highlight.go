package app

import (
	"fmt"

	"github.com/kaiser-chris/pdx-asset-go/render"
)

// Pointing at a gene in the character panel highlights the part of the
// portrait it changes. What a morph gene changes is found by morphing the
// portrait with the gene at its weakest and at its strongest, everything else
// as it is, and measuring how far each vertex moved between the two; a
// category is measured the same way with every gene in it at once. The
// portrait is then morphed back.
//
// Measuring morphs the portrait three times, which is quick but not free, so
// a measurement is kept until something it depends on changes: the template
// a gene is at, the age, or the models themselves. And a gene is only
// measured once it has been pointed at for a few frames, so that running the
// mouse down the list does not measure every gene it crosses.

// highlightDelay is how many frames something has to be pointed at before it
// is highlighted.
const highlightDelay = 6

// highlighting is what the portrait highlights.
type highlighting struct {
	// enabled is whether pointing at a gene highlights it at all.
	enabled bool

	// pointed is what the panel points at this frame, under a name that
	// tells one set of genes from another, empty for nothing, and keys the
	// keys of those genes.
	pointed string
	keys    []string

	// held is set when the panel hides the highlight this frame, as it does
	// while a slider is dragged.
	held bool

	// pending is what has been pointed at, as measured, and since for how
	// many frames. shown is what is highlighted now.
	pending, shown string
	since          int

	// measured are the measurements kept, by what was measured.
	measured map[string]measurement
}

// measurement is how far a gene moves the vertices of the body and of the
// head, and the furthest it moves any.
type measurement struct {
	body, head [][]float32
	furthest   float32
}

// point says what the panel points at this frame: the genes, by their keys,
// whose change to highlight, under a name that tells one set of them from
// another. Nothing pointed at by the end of a frame clears the highlight.
func (h *highlighting) point(name string, keys []string) {
	h.pointed, h.keys = name, keys
}

// hold hides the highlight for this frame, keeping what is pointed at, so
// that it comes back as it was once the panel lets go.
func (h *highlighting) hold() {
	h.held = true
}

// forget drops every measurement, for models or a character that changed.
// What is highlighted is taken to be something no measurement is, so that the
// next update highlights again what is pointed at, or clears the highlight.
func (h *highlighting) forget() {
	h.measured = nil
	h.shown = staleHighlight
}

// staleHighlight is what is highlighted after the measurements were
// forgotten: no name a measurement is kept by.
const staleHighlight = "stale"

// update highlights what the panel pointed at, once it has been for long
// enough, and clears the highlight when it points at nothing. It runs before
// the panel, which says again every frame what it points at.
func (h *highlighting) update(p *portrait) {
	pointed, keys, held := h.pointed, h.keys, h.held
	h.pointed, h.keys, h.held = "", nil, false

	if !h.enabled || pointed == "" || held {
		h.pending, h.since = "", 0

		if h.shown != "" {
			p.body.Highlight(nil, 0)
			p.head.Highlight(nil, 0)
			h.shown = ""
		}

		return
	}

	// What a gene moves depends on the template it is at and on the age,
	// which the age presets change it by.
	key := fmt.Sprintf("%s@%s@%d", pointed, p.character.templatesOf(keys), p.character.age)
	if key != h.pending {
		h.pending, h.since = key, 0
	}

	h.since++

	if key == h.shown || h.since < highlightDelay {
		return
	}

	found, ok := h.measured[key]
	if !ok {
		found = measure(p, keys)

		if h.measured == nil {
			h.measured = map[string]measurement{}
		}

		h.measured[key] = found
	}

	p.body.Highlight(found.body, found.furthest)
	p.head.Highlight(found.head, found.furthest)
	h.shown = key
}

// measure morphs the portrait with genes at their weakest and their
// strongest, and measures how far each vertex moved between the two. The
// body is morphed before the head, which follows the joints of the body as
// it is posed. The portrait is morphed back as the character has it.
func measure(p *portrait, keys []string) measurement {
	weakest := p.character.attributesWith(keys, 0)
	strongest := p.character.attributesWith(keys, maxGeneValue)

	p.body.Morph(weakest)
	p.head.Morph(weakest)
	bodyBefore, headBefore := p.body.Snapshot(), p.head.Snapshot()

	p.body.Morph(strongest)
	p.head.Morph(strongest)
	bodyAfter, headAfter := p.body.Snapshot(), p.head.Snapshot()

	current := p.character.attributes()
	p.body.Morph(current)
	p.head.Morph(current)

	body, bodyFurthest := render.Movement(bodyBefore, bodyAfter)
	head, headFurthest := render.Movement(headBefore, headAfter)

	return measurement{body: body, head: head, furthest: max(bodyFurthest, headFurthest)}
}
