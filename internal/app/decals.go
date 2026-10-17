package app

import (
	"fmt"
	"slices"

	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-asset-go/texture"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// The decals of the morph genes are textures laid over the skin: wrinkles,
// crow's feet, stubble, the colour of the lips, the eyebrows. Which of them
// are laid, and how opaque, the template a gene picks, its strength and the
// age of the portrait say, so they are worked out again whenever the genes
// change, and handed to the renderer to lay over the head and the body.

// readDecalTextures reads every texture the decals of the morph genes name,
// for every sex alike, by their paths below the folders' root, once
// each however many decals name it. A texture that cannot be read is
// reported, and the decals that name it are laid without it. So is a blend
// mode the renderer does not know, and those decals are laid as an overlay,
// the games' first mode.
//
// They are read with the portrait, off the thread that draws, so that
// changing a gene or the sex never waits for a file; a man's genes alone
// name some eighty of them.
func readDecalTextures(set *folders.Set, all *genes.Genes) (map[string]*texture.Image, report.Diagnostics) {
	textures := map[string]*texture.Image{}

	var diagnostics report.Diagnostics

	reported := map[string]bool{}

	note := func(subject, message string) {
		if reported[subject+message] {
			return
		}

		reported[subject+message] = true
		diagnostics = append(diagnostics, report.Diagnostic{Severity: report.SeverityWarning, Subject: subject, Message: message})
	}

	for _, gene := range all.MorphGenes.All() {
		for index := range gene.Templates {
			var decals []genes.Decal
			for _, sex := range genes.Sexes {
				decals = append(decals, gene.Templates[index].For(sex).Decals...)
			}

			for _, decal := range decals {
				for _, mode := range []string{decal.BlendModes.Diffuse, decal.BlendModes.Normal, decal.BlendModes.Properties} {
					if _, ok := render.ParseBlendMode(mode); mode != "" && !ok {
						note("gene "+gene.Key, fmt.Sprintf("a decal is blended by %s, which is not overlay, replace, hard_light or multiply; laid as an overlay", mode))
					}
				}

				for _, path := range []string{decal.Textures.Diffuse, decal.Textures.Normal, decal.Textures.Properties} {
					if _, done := textures[path]; done || path == "" {
						continue
					}

					read, err := readTexture(set, path)
					if err != nil {
						note("gene "+gene.Key, err.Error()+"; its decal is laid without it")
					}

					// A texture that could not be read is kept as nil, so
					// that it is not read again.
					textures[path] = read
				}
			}
		}
	}

	return textures, diagnostics
}

// blendMode is the blend mode a gene file names, an overlay where it names
// none or one the renderer does not know, as the games take it.
func blendMode(name string) render.BlendMode {
	mode, ok := render.ParseBlendMode(name)
	if !ok {
		return render.BlendOverlay
	}

	return mode
}

// decals are the decals the genes lay over the portrait at its age, in the
// order of the genes: how opaque each is is its alpha curve at the gene's
// strength, changed by its age. Those the age or the strength leave
// transparent are left out.
func (c *character) decals(textures map[string]*texture.Image) []render.Decal {
	var laid []render.Decal

	at := c.at()

	for _, morph := range c.morphs {
		template := &morph.gene.Templates[morph.template]
		strength := float64(morph.value) / maxGeneValue

		for index := range template.For(c.sex).Decals {
			decal := &template.For(c.sex).Decals[index]

			weight := c.all.DecalAlpha(decal, strength, at)
			if weight <= 0 {
				continue
			}

			laid = append(laid, render.Decal{
				BodyPart:       decal.BodyPart,
				Diffuse:        textures[decal.Textures.Diffuse],
				Normal:         textures[decal.Textures.Normal],
				Properties:     textures[decal.Textures.Properties],
				DiffuseMode:    blendMode(decal.BlendModes.Diffuse),
				NormalMode:     blendMode(decal.BlendModes.Normal),
				PropertiesMode: blendMode(decal.BlendModes.Properties),
				Weight:         weight,
				Priority:       decal.Priority,
				PostSkinColor:  decal.PostSkinColor(),
			})
		}
	}

	return laid
}

// laysDecals reports whether a template of a gene lays any decal over a
// character of a sex.
func laysDecals(gene *genes.MorphGene, sex genes.Sex) bool {
	return slices.ContainsFunc(gene.Templates, func(template genes.MorphTemplate) bool {
		return len(template.For(sex).Decals) > 0
	})
}
