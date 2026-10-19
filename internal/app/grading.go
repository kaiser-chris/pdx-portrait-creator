package app

import (
	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	staged "github.com/kaiser-chris/pdx-parser-go/portrait"
)

// The games light their portraits with the same shaders but grade them each
// their own way: how the light is exposed and tonemapped into colours, which
// environment a portrait starts in. Victoria 3 sets its grading in the engine,
// where no file shows it, so it is fitted to its portrait editor; Europa
// Universalis 5 writes its own into gfx/portraits/jomini_environment.txt. The
// two are kept apart, so that fitting one never moves the other.
//
// Which game the folders are is told by their layers: Europa Universalis 5
// splits its files into in_game, main_menu and loading_screen, and Victoria 3
// keeps them in one.

// grading is how a game grades its portraits.
type grading struct {
	// game names the game, for what the interface says.
	game string

	// defaultEnvironment is the environment a portrait is lit by unless
	// another is picked: the one the game's portrait editor starts in.
	defaultEnvironment string

	// grade sets how the light of a lighting is turned into colours, by the
	// settings every environment shares.
	grade func(lighting *render.Lighting, settings staged.Settings)

	// occludes says whether the portrait is darkened by the ambient
	// occlusion the settings ask for.
	occludes bool
}

// gradingOf is the grading of the game the folders are.
func gradingOf(set *folders.Set) grading {
	if set != nil && len(set.Layers()) > 0 {
		return europaUniversalis5
	}

	return victoria3
}

// victoria3 grades as Victoria 3's portrait editor does.
var victoria3 = grading{
	game:               "Victoria 3",
	defaultEnvironment: "environment_standard",
	occludes:           true,
	grade: func(lighting *render.Lighting, settings staged.Settings) {
		// The game tonemaps its portraits by the ACES filmic curve, which it
		// sets in the engine: its pictures match it and not the curve of John
		// Hable that the settings give, which the rest of the game is drawn
		// with. The exposure is fitted to them too.
		lighting.ACES = true
		lighting.Exposure = victoria3Exposure

		restore(lighting, settings)
	},
}

// victoria3Exposure is the fixed exposure Victoria 3 turns the light of an
// environment into colours with. The game sets it in the engine, which ships
// no file it can be read from; this is fitted to its portrait editor, whose
// pictures of environment_standard it matches in brightness and colour.
const victoria3Exposure = 0.6

// europaUniversalis5 grades as Europa Universalis 5 does, by its settings:
// a FixedExposure of exposure, and the Uncharted curve of John Hable that
// tonemap_curve shapes, as its restorescene.shader applies them.
//
// Its settings ask for ambient occlusion as well, but its portraits show
// next to none: the creases of the throat are pink in the game's pictures,
// where the occlusion of its ssao.shader, darkening the head towards black,
// draws them grey. How strongly it occludes rests on the points around a
// pixel the engine compares, which it does not ship, so the portrait is
// drawn without it rather than with a guess.
var europaUniversalis5 = grading{
	game:               "Europa Universalis 5",
	defaultEnvironment: "portrait_editor",
	occludes:           false,
	grade: func(lighting *render.Lighting, settings staged.Settings) {
		exposure := 1.0
		if settings.ExposureFunction == "FixedExposure" {
			exposure = settings.Exposure.Or(1)
		}

		lighting.Exposure = float32(exposure) * europaUniversalis5Brightness

		if curve, ok := settings.Tonemap.Get(); ok && settings.TonemapFunction == "Uncharted" {
			lighting.Tonemap = &render.Tonemap{
				ShoulderStrength: float32(curve.ShoulderStrength),
				LinearStrength:   float32(curve.LinearStrength),
				LinearAngle:      float32(curve.LinearAngle),
				ToeStrength:      float32(curve.ToeStrength),
				ToeNumerator:     float32(curve.ToeNumerator),
				ToeDenominator:   float32(curve.ToeDenominator),
				LinearWhite:      float32(curve.LinearWhite),
			}
		} else {
			lighting.ACES = true
		}

		restore(lighting, settings)
	},
}

// europaUniversalis5Brightness is how much less light Europa Universalis 5's
// engine turns into colours than the renderer does from the same lights and
// the same exposure: the game's pictures of gui_portrait at the exposure its
// settings give are as bright as the renderer's at that times this. It is the
// exposure Victoria 3 is fitted with too, which suggests the renderer's lights
// are that much brighter than the engine's, but each game keeps its own.
const europaUniversalis5Brightness = 0.6

// restore sets what both games do to the colours once tonemapped, as their
// RestoreScene does: scale their value and saturation and balance them.
func restore(lighting *render.Lighting, settings staged.Settings) {
	lighting.ValueScale = float32(settings.ValueScale.Or(1))
	lighting.SaturationScale = float32(settings.SaturationScale.Or(1))

	if balance, ok := settings.ColorBalance.Get(); ok {
		lighting.ColorBalance = [3]float32{float32(balance[0]), float32(balance[1]), float32(balance[2])}
	}
}
