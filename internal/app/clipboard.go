package app

import (
	"fmt"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// A portrait can start from a DNA on the clipboard, in any of the three ways
// the game writes one: a definition of common/dna_data, the persistent DNA
// the game writes out, or a DNA string. The welcome screen looks at the
// clipboard while it is open and says what it holds.

// pastedKind is the way a DNA on the clipboard is written.
type pastedKind int

const (
	pastedNothing pastedKind = iota
	pastedDefinition
	pastedPersistent
	pastedString
)

// maxPasted is the most text the clipboard is read for a DNA from. A DNA is a
// few kilobytes; a whole file of them is not much more, and anything far
// larger is something else.
const maxPasted = 1 << 20

// clipboardCheckEvery is how often the welcome screen looks at the
// clipboard, which the user changes in other applications without the
// window knowing.
const clipboardCheckEvery = 500 * time.Millisecond

// pasted is what the clipboard holds of a DNA.
type pasted struct {
	kind pastedKind

	// key is what a definition or a persistent DNA is defined under.
	key string

	// dna is the DNA, nil when the clipboard holds none that can be used:
	// then why says what is wrong with it, or nothing when it is not a DNA
	// at all.
	dna *genes.DNA
	why string
}

// usable reports whether the clipboard holds a DNA to start from.
func (p pasted) usable() bool {
	return p.dna != nil
}

// describe says what the clipboard holds: the way the DNA is written and,
// for any but a string, which names nothing, its key.
func (p pasted) describe() string {
	switch {
	case p.why != "":
		return p.why
	case p.kind == pastedDefinition:
		return "A DNA definition: " + p.key
	case p.kind == pastedPersistent:
		return "Persistent DNA: " + p.key
	case p.kind == pastedString:
		return fmt.Sprintf("A DNA string of %d genes", len(p.dna.Genes))
	default:
		return "Copy a DNA definition, persistent DNA or a DNA string to start from it."
	}
}

// clipboardState is what the welcome screen last found on the clipboard.
type clipboardState struct {
	checked time.Time
	text    string
	genes   *genes.Genes
	pasted  pasted
}

// pollClipboard reads the clipboard again for a DNA, at most every
// clipboardCheckEvery, and only reads text that changed since, or that was
// read with other genes.
func (a *App) pollClipboard() pasted {
	state := &a.state.clipboard
	p := a.portrait

	if p == nil || p.genes == nil {
		return pasted{}
	}

	if time.Since(state.checked) < clipboardCheckEvery && state.genes == p.genes {
		return state.pasted
	}

	state.checked = time.Now()

	text := rl.GetClipboardText()
	if text == state.text && state.genes == p.genes {
		return state.pasted
	}

	state.text, state.genes = text, p.genes
	state.pasted = readPasted(text, p.genes)

	return state.pasted
}

// readPasted finds a DNA in text: a definition of common/dna_data or a
// persistent DNA, the first of either in it, or else a DNA string standing
// on its own. Comments are what the game takes them for, so the DNA string
// the game's files hold in a comment above a definition is not read: the
// definition is.
func readPasted(text string, all *genes.Genes) pasted {
	if len(text) > maxPasted {
		return pasted{}
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return pasted{}
	}

	// A string is one word of base64, which has no braces and no spaces.
	if !strings.ContainsAny(trimmed, "{} \t\r\n#") {
		return readPastedString(trimmed, all)
	}

	document := script.Parse(text)
	table := genes.ReadDNA(document, folders.File{}, &report.Collector{})

	for _, field := range document.Fields {
		kind := definedAs(field)
		if kind == pastedNothing {
			continue
		}

		dna, ok := table.Get(field.Key)
		if !ok {
			continue
		}

		found := pasted{kind: kind, key: field.Key, dna: dna}

		// A definition can give its DNA as a string rather than as genes.
		if len(dna.Genes) == 0 && dna.String != "" {
			fromString := readPastedString(dna.String, all)
			if fromString.dna == nil {
				return pasted{kind: kind, key: field.Key, why: field.Key + ": " + fromString.why}
			}

			dna.Genes = fromString.dna.Genes
		}

		if len(dna.Genes) == 0 {
			return pasted{kind: kind, key: field.Key, why: field.Key + " gives no genes"}
		}

		return found
	}

	return pasted{}
}

// definedAs tells a definition of common/dna_data, which holds its genes in
// a portrait_info or as a DNA string, from a persistent DNA, which holds
// them at the top, from anything else.
func definedAs(field script.Field) pastedKind {
	if !field.Value.IsBlock() {
		return pastedNothing
	}

	has := func(key string) bool {
		_, ok := field.Value.Get(key)

		return ok
	}

	switch {
	case has("portrait_info") || has("dna"):
		return pastedDefinition
	case has("genes"):
		return pastedPersistent
	default:
		return pastedNothing
	}
}

// readPastedString reads a DNA string, which is only of use when it holds as
// many genes as the game has: one written with other genes reads as other
// genes than it was written with.
func readPastedString(text string, all *genes.Genes) pasted {
	dna, problems := all.ReadString(text)
	if problems.Count(report.SeverityError) > 0 || len(dna.Genes) == 0 {
		return pasted{}
	}

	if len(problems) > 0 {
		return pasted{kind: pastedString, why: "A DNA string written with other genes than the game's: " + problems[0].Message}
	}

	return pasted{kind: pastedString, dna: dna}
}

// fromClipboard draws the portrait from the DNA on the clipboard, as the sex
// a persistent DNA says, or the one the portrait is. It keeps the key of a
// definition or a persistent DNA as its identifier; it is in no file yet.
func (a *App) fromClipboard() {
	found := a.pollClipboard()
	if !found.usable() {
		return
	}

	p := a.portrait
	from := basis{kind: basisClipboard, key: found.key, identifier: found.key}

	a.start(from, found.dna, sexOf(found.dna.Sex, p.sex))
	a.setStatus("Started from the clipboard: %s", found.describe())
}

// sexOf is the sex a DNA says it is of, or fallback when it does not say.
func sexOf(key string, fallback genes.Sex) genes.Sex {
	for _, sex := range genes.Sexes {
		if sex.String() == key {
			return sex
		}
	}

	return fallback
}
