package app

import (
	"fmt"

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

// readGeneTextures reads every texture the decals and the texture overrides
// of the morph genes name, for every sex alike, by their paths below the
// folders' root, once each however many name it. A texture that cannot be
// read is reported, and the decals that name it are laid without it, as the
// overrides that name it keep the skin's own map in its place. So is a blend
// mode the renderer does not know, and those decals are laid as an overlay,
// the games' first mode.
//
// They are read with the portrait, off the thread that draws, so that
// changing a gene or the sex never waits for a file; a man's genes alone
// name some eighty of them.
func readGeneTextures(set *folders.Set, all *genes.Genes) (map[string]*texture.Image, report.Diagnostics) {
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

	read := func(gene *genes.MorphGene, path, without string) {
		if _, done := textures[path]; done || path == "" {
			return
		}

		image, err := readTexture(set, path)
		if err != nil {
			note("gene "+gene.Key, err.Error()+"; "+without)
		}

		// A texture that could not be read is kept as nil, so that it is
		// not read again.
		textures[path] = image
	}

	for _, gene := range all.MorphGenes.All() {
		for index := range gene.Templates {
			for _, sex := range genes.Sexes {
				morph := gene.Templates[index].For(sex)

				for _, decal := range morph.Decals {
					for _, mode := range []string{decal.BlendModes.Diffuse, decal.BlendModes.Normal, decal.BlendModes.Properties} {
						if _, ok := render.ParseBlendMode(mode); mode != "" && !ok {
							note("gene "+gene.Key, fmt.Sprintf("a decal is blended by %s, which is not overlay, replace, hard_light or multiply; laid as an overlay", mode))
						}
					}

					for _, path := range []string{decal.Textures.Diffuse, decal.Textures.Normal, decal.Textures.Properties} {
						read(gene, path, "its decal is laid without it")
					}
				}

				for _, override := range morph.TextureOverrides {
					for _, path := range []string{override.Diffuse, override.Normal, overrideProperties(override)} {
						read(gene, path, "the skin keeps its own map in its place")
					}
				}
			}
		}
	}

	return textures, diagnostics
}

// overrideProperties is the properties map of a texture override, which the
// games' files write as its specular map.
func overrideProperties(override genes.TextureOverride) string {
	if override.Properties != "" {
		return override.Properties
	}

	return override.Specular
}

// skinOverride is the texture override the genes draw the skin of the head
// with, as they are set: of the overrides the templates the genes are at give
// the sex, the one of the greatest weight, the first of them where several
// weigh the same. The games pick one by its weight at random where a DNA
// gives several, which the shipped genes never do, so that a portrait keeps
// one face rather than another each time it is drawn. No override draws the
// skin with its own maps.
func (c *character) skinOverride(textures map[string]*texture.Image) render.SkinMaps {
	var (
		chosen *genes.TextureOverride
		maps   render.SkinMaps
	)

	for _, morph := range c.morphs {
		overrides := morph.gene.Templates[morph.template].For(c.sex).TextureOverrides

		for index := range overrides {
			if chosen == nil || overrides[index].Weight > chosen.Weight {
				chosen = &overrides[index]
			}
		}
	}

	if chosen != nil {
		maps = render.SkinMaps{Diffuse: textures[chosen.Diffuse], Normal: textures[chosen.Normal], Properties: textures[overrideProperties(*chosen)]}
	}

	return maps
}

// colourShift is how far the genes shift the hue, saturation and value of a
// colour of the portrait, hair, skin or eye, as they are set, at the
// portrait's age: how the games grey the hair of the old.
func (c *character) colourShift(colour string) [3]float64 {
	var total [3]float64

	at := c.at()

	for _, morph := range c.morphs {
		template := &morph.gene.Templates[morph.template]
		shift := c.all.HSVShift(template.For(c.sex), colour, float64(morph.value)/maxGeneValue, at)

		for channel := range total {
			total[channel] += shift[channel]
		}
	}

	return total
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
