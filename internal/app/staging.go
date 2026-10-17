package app

import (
	"fmt"
	"math"
	"os"
	"slices"

	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-asset-go/texture"
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	staged "github.com/kaiser-chris/pdx-parser-go/portrait"
)

// The game stages a portrait in an environment, which lights it, and looks
// at it through a camera, both of gfx/portraits. The portrait is lit as the
// environment picked says, environment_standard unless another is, or by the
// renderer's own studio of lights for none; and seen through the camera
// picked, or framed freely on the face or the whole figure for none.
//
// An environment's lights stand at places measured from the game's origin,
// the portrait's feet, and point at bones of the portrait; a camera stands at
// a place measured from one bone and looks at another. They are placed every
// frame, so that they follow the bones wherever the portrait's animation
// takes them. The game's coordinates are the mirror of the renderer's across
// x.

// defaultEnvironment is the environment a portrait is lit by unless another
// is picked: the one the game's portrait editor starts in.
const defaultEnvironment = "environment_standard"

// staging is the environment and the camera of a portrait.
type staging struct {
	// environments, cameras and settings are what the folders hold, and set
	// the folders, which the cubemaps are read from.
	environments database.Table[staged.Environment]
	cameras      database.Table[staged.PortraitCamera]
	settings     staged.Settings
	set          *folders.Set

	// environment is the environment picked, empty for the studio, and camera
	// the camera picked, empty to frame freely. chosen is set once an
	// environment has been picked, so that a load does not pick the default
	// over a choice of none.
	environment, camera string
	chosen              bool

	// cubemaps are the cubemaps on the GPU, by their paths, nil for one that
	// could not be read, and reading those being read.
	cubemaps map[string]*render.Environment
	reading  map[string]bool
	read     chan cubemapRead
}

// cubemapRead is a cubemap read off the thread that draws.
type cubemapRead struct {
	path  string
	faces [6]*texture.Image
	err   error
}

// prepare takes what a load read. An environment picked before stays picked
// where the folders still have it; otherwise the default is.
func (s *staging) prepare(set *folders.Set, environments database.Table[staged.Environment], cameras database.Table[staged.PortraitCamera], settings staged.Settings) {
	s.set, s.environments, s.cameras, s.settings = set, environments, cameras, settings

	if s.read == nil {
		s.read = make(chan cubemapRead, 4)
		s.cubemaps, s.reading = map[string]*render.Environment{}, map[string]bool{}
	}

	if !s.chosen || (s.environment != "" && !environments.Has(s.environment)) {
		s.environment = ""
		if environments.Has(defaultEnvironment) {
			s.environment = defaultEnvironment
		}
	}

	if s.camera != "" && !cameras.Has(s.camera) {
		s.camera = ""
	}
}

// unload releases the cubemaps on the GPU. It needs the OpenGL context.
func (s *staging) unload() {
	for path, cubemap := range s.cubemaps {
		cubemap.Unload()
		delete(s.cubemaps, path)
	}
}

// environmentKeys and cameraKeys are what can be picked, by name.
func (s *staging) environmentKeys() []string {
	keys := s.environments.Keys()
	slices.Sort(keys)

	return keys
}

func (s *staging) cameraKeys() []string {
	keys := s.cameras.Keys()
	slices.Sort(keys)

	return keys
}

// lighting is the lighting of the environment picked, with its lights placed
// on the portrait as it stands now, or nil for the studio. The cubemap is read
// the first time it is wanted, and the portrait is lit without it meanwhile.
func (s *staging) lighting(p *portrait) *render.Lighting {
	environment, ok := s.environments.Get(s.environment)
	if s.environment == "" || !ok {
		return nil
	}

	s.collect(p.renderer)

	lighting := &render.Lighting{EnvironmentIntensity: float32(environment.CubemapIntensity.Or(1)), Exposure: portraitExposure}

	if environment.Cubemap != "" {
		lighting.Environment = s.cubemap(environment.Cubemap)
	}

	for _, light := range environment.Lights {
		position := mirrored(light.Position.Cartesian())
		target := add(p.node(light.LookAtNode), mirrored(light.LookAt))

		kind := render.LightSpot

		switch light.Type {
		case staged.LightPoint:
			kind = render.LightPoint
		case staged.LightDirectional:
			kind = render.LightDirectional
		}

		// The files give a spot light's cone as the whole of its angle;
		// the renderer takes the angle from its middle.
		lighting.Lights = append(lighting.Lights, render.Light{
			Kind:             kind,
			Position:         position,
			Direction:        [3]float32{target[0] - position[0], target[1] - position[1], target[2] - position[2]},
			Color:            [3]float32{float32(light.Color[0]), float32(light.Color[1]), float32(light.Color[2])},
			Radius:           float32(light.Radius),
			Falloff:          float32(light.Falloff),
			InnerCone:        float32(light.InnerConeAngle / 2 * math.Pi / 180),
			OuterCone:        float32(light.OuterConeAngle / 2 * math.Pi / 180),
			AffectedByShadow: light.AffectedByShadow,
		})
	}

	// The shadows are cast from the environment's shadow camera, which
	// stands where the lights do, measured from the game's origin.
	if camera, ok := environment.ShadowCamera.Get(); ok {
		nearFar := camera.NearFar.Or([2]float64{10, 2000})

		lighting.Shadow = &render.Shadow{
			Position:    add(p.node(camera.PositionNode), mirrored(camera.Position.Cartesian())),
			Target:      add(p.node(camera.LookAtNode), mirrored(camera.LookAt)),
			FieldOfView: float32(camera.FOV.Or(45)),
			Near:        float32(nearFar[0]),
			Far:         float32(nearFar[1]),
			KernelScale: float32(s.settings.ShadowmapKernelScale.Or(1)),
			FadeFactor:  float32(s.settings.ShadowmapFadeFactor.Or(1)),
		}
	}

	settings := s.settings

	// The games tonemap their portraits by the ACES filmic curve, which they
	// set in the engine: their pictures match it and not the curve of
	// John Hable that the settings give, which the rest of the game is drawn
	// with.
	lighting.ACES = true

	lighting.ValueScale = float32(settings.ValueScale.Or(1))
	lighting.SaturationScale = float32(settings.SaturationScale.Or(1))

	if balance, ok := settings.ColorBalance.Get(); ok {
		lighting.ColorBalance = [3]float32{float32(balance[0]), float32(balance[1]), float32(balance[2])}
	}

	if ssao, ok := settings.SSAO.Get(); ok && ssao.Enabled {
		lighting.Occlusion = &render.Occlusion{
			Samples:        ssao.Samples,
			Radius:         float32(ssao.Radius),
			MaxRadius:      float32(ssao.MaxRadius),
			DepthDistance:  float32(ssao.DepthDistance),
			MaxDepth:       float32(ssao.MaxDepth),
			Scale:          float32(ssao.ApplyScale),
			Bias:           float32(ssao.ApplyBias),
			Power:          float32(ssao.ApplyPower),
			BlendFactor:    float32(ssao.BlendFactor),
			Blur:           ssao.BlurEnabled,
			BlurWidth:      float32(ssao.BlurWidth),
			BlurScale:      float32(ssao.BlurScale),
			BlurDepthPower: float32(ssao.BlurDepthPower),
			BlurWeights:    ssao.BlurWeights,
		}
	}

	return lighting
}

// cubemap is a cubemap on the GPU, and nil while it is read, which this
// starts the first time it is asked, or when it could not be read.
func (s *staging) cubemap(path string) *render.Environment {
	if cubemap, ok := s.cubemaps[path]; ok {
		return cubemap
	}

	if !s.reading[path] && s.set != nil {
		s.reading[path] = true
		set := s.set

		go func() {
			faces, err := readCubemap(set, path)
			s.read <- cubemapRead{path: path, faces: faces, err: err}
		}()
	}

	return nil
}

// collect uploads the cubemaps read since the last frame. One that could not
// be read or uploaded is reported, and kept as nil, so that it is not read
// again.
func (s *staging) collect(renderer *render.Renderer) {
	for {
		select {
		case read := <-s.read:
			delete(s.reading, read.path)

			var uploaded *render.Environment

			err := read.err
			if err == nil {
				uploaded, err = renderer.UploadEnvironment(read.faces)
			}

			if err != nil {
				warn(fmt.Errorf("%s: %w", read.path, err))
			}

			s.cubemaps[read.path] = uploaded

		default:
			return
		}
	}
}

// readCubemap reads the faces of a cubemap of the folders.
func readCubemap(set *folders.Set, relative string) ([6]*texture.Image, error) {
	found, ok := set.Find(relative)
	if !ok {
		return [6]*texture.Image{}, fmt.Errorf("%s is in none of the folders", relative)
	}

	data, err := os.ReadFile(found.Path)
	if err != nil {
		return [6]*texture.Image{}, err
	}

	return texture.DecodeCubeDDS(data)
}

// look stands the viewer's camera where the camera picked stands, and reports
// whether one is picked.
func (s *staging) look(p *portrait) bool {
	camera, ok := s.cameras.Get(s.camera)
	if s.camera == "" || !ok {
		return false
	}

	position := add(p.node(camera.Camera.PositionNode), mirrored(camera.Camera.Position.Cartesian()))
	target := add(p.node(camera.Camera.LookAtNode), mirrored(camera.Camera.LookAt))

	p.viewer.LookFrom(position, target, camera.Camera.FOV.Or(0))

	return true
}

// node is where a bone of the portrait is now, in the coordinates the models
// are drawn in: of the body, or failing that of the head; the game's origin
// for none, or one neither has.
func (p *portrait) node(name string) [3]float32 {
	if name == "" {
		return [3]float32{}
	}

	for _, model := range []*render.Model{p.body, p.head} {
		if model == nil {
			continue
		}

		if _, posed, ok := model.JointPosition(name); ok {
			return posed
		}
	}

	return [3]float32{}
}

// mirrored turns a place of the game's coordinates into the renderer's, the
// mirror of them across x.
func mirrored(place [3]float64) [3]float32 {
	return [3]float32{float32(-place[0]), float32(place[1]), float32(place[2])}
}

// add adds two places.
func add(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

// portraitExposure is the fixed exposure the light of an environment is
// turned into colours with. The games set theirs in the engine, which ships
// no file it can be read from; this is fitted to their portrait editor, whose
// pictures of environment_standard it matches in brightness and colour.
const portraitExposure = 0.6
