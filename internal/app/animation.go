package app

import (
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-asset-go/anim"
	"github.com/kaiser-chris/pdx-asset-go/entity"
	"github.com/kaiser-chris/pdx-asset-go/model"
	"github.com/kaiser-chris/pdx-asset-go/render"
	states "github.com/kaiser-chris/pdx-asset-go/state"
	"github.com/kaiser-chris/pdx-parser-go/asset"
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/genes"
)

// A portrait plays an animation set of gfx/portraits/portrait_animations,
// idle by default, as the game does: the set says which state of the head and
// which of the torso the portrait starts in, for its sex, and the states of
// the entities' .asset files play the animations and go on from one to the
// next. The head and the torso each go their own way through their states,
// as in the game; the head follows the body's joints wherever its animation
// takes them.
//
// The samples of an animation are read when it is first played, off the
// thread that draws, and kept; until they are read the portrait stays in the
// pose it is in.

// defaultAnimation is the set a portrait plays unless another is picked.
const defaultAnimation = "idle"

// animating is what a portrait plays.
type animating struct {
	// sets are the animation sets of the folders, and states the states of
	// the body's and the head's entities.
	sets                   database.Table[genes.AnimationSet]
	bodyStates, headStates []asset.State

	// bodyFiles and headFiles are the files of the animations of the body
	// and the head, by their ids.
	bodyFiles, headFiles map[string]string

	// chosen is the set picked, and variant the variant of it, empty for its
	// default; none is set when the portrait plays nothing, and stands in the
	// pose its meshes are stored in. paused stops the clock.
	chosen, variant string
	none, paused    bool

	// body and head play the states.
	body, head *states.Machine
	seed       uint64

	// samples are the animations read, by their files, nil for one that
	// could not be read, and reading those being read.
	samples map[string]*anim.Animation
	reading map[string]bool
	read    chan animationRead
}

// animationRead is an animation read off the thread that draws.
type animationRead struct {
	file    string
	samples *anim.Animation
	err     error
}

// prepare takes what a load read: the animation sets, and the states and the
// animations of the body's and the head's entities. A set picked before
// stays picked where the folders still have it.
func (a *animating) prepare(sets database.Table[genes.AnimationSet], assets *asset.Assets, body, head *model.Model) {
	a.sets = sets
	a.bodyStates, a.bodyFiles = statesOf(assets, body)
	a.headStates, a.headFiles = statesOf(assets, head)

	if a.read == nil {
		a.read = make(chan animationRead, 16)
		a.samples, a.reading = map[string]*anim.Animation{}, map[string]bool{}
	}

	if a.chosen == "" || !sets.Has(a.chosen) {
		a.chosen, a.variant = a.defaultSet(), ""
	}
}

// defaultSet is idle where the folders have it, otherwise the first set.
func (a *animating) defaultSet() string {
	if a.sets.Has(defaultAnimation) {
		return defaultAnimation
	}

	if keys := a.sets.Keys(); len(keys) > 0 {
		return keys[0]
	}

	return ""
}

// statesOf are the states of the entity a model was loaded from, and the
// files of the animations its own mesh plays, by their ids.
func statesOf(assets *asset.Assets, loaded *model.Model) ([]asset.State, map[string]string) {
	files := map[string]string{}

	for _, animation := range loaded.Animations {
		if animation.Entity == loaded.Name && animation.Attachment == 0 {
			files[animation.ID] = animation.File
		}
	}

	if assets == nil {
		return nil, files
	}

	definition, ok := assets.Entities.Get(loaded.Name)
	if !ok {
		return nil, files
	}

	return definition.States, files
}

// headChanged takes the animations of a head read again with other parts,
// which are its own entity's as before.
func (a *animating) headChanged(head *model.Model) {
	_, a.headFiles = statesOf(nil, head)
}

// restart starts the head and the torso in the states the set picked gives a
// sex.
func (a *animating) restart(sex genes.Sex) {
	a.seed++
	a.body, a.head = nil, nil

	if a.none {
		return
	}

	set, ok := a.sets.Get(a.chosen)
	if !ok {
		return
	}

	pair := set.For(sex).Default

	for _, variant := range set.For(sex).Variants {
		if variant.Name == a.variant {
			pair = variant.Animation
		}
	}

	a.body = states.New(a.bodyStates, pair.Torso, a.seed)
	a.head = states.New(a.headStates, pair.Head, a.seed+1)
}

// pose moves the clock on by the time the last frame took and poses the body,
// then the head, which follows the body's joints, as their states have them.
// A model whose animation has not been read yet stays as it is.
func (a *animating) pose(loader *entity.Loader, body, head *render.Model) {
	a.collect()

	if !a.paused {
		seconds := float64(rl.GetFrameTime())
		a.body.Advance(seconds, a.length(loader, a.bodyFiles))
		a.head.Advance(seconds, a.length(loader, a.headFiles))
	}

	a.poseOne(loader, body, a.body, a.bodyFiles)
	a.poseOne(loader, head, a.head, a.headFiles)
}

// poseOne poses a model as its machine has it, or in the pose its meshes are
// stored in for none.
func (a *animating) poseOne(loader *entity.Loader, uploaded *render.Model, machine *states.Machine, files map[string]string) {
	id, seconds, loop, ok := machine.Playing()
	if !ok {
		uploaded.Pose(nil, 0, false, nil)

		return
	}

	samples, ok := a.samplesOf(loader, files[id])
	if !ok {
		return
	}

	uploaded.Pose(samples, seconds, loop, []int{0})
}

// length says how long an animation of a model runs, by its id, once it has
// been read.
func (a *animating) length(loader *entity.Loader, files map[string]string) func(string) (float64, bool) {
	return func(id string) (float64, bool) {
		samples, ok := a.samplesOf(loader, files[id])
		if !ok || samples.FPS <= 0 {
			return 0, false
		}

		return float64(samples.Frames) / float64(samples.FPS), true
	}
}

// samplesOf are the samples of an animation file, and false while they are
// being read, which this starts the first time it is asked, or when the file
// could not be read.
func (a *animating) samplesOf(loader *entity.Loader, file string) (*anim.Animation, bool) {
	if file == "" || loader == nil {
		return nil, false
	}

	if samples, ok := a.samples[file]; ok {
		return samples, samples != nil
	}

	if !a.reading[file] {
		a.reading[file] = true

		go func() {
			samples, err := loader.ReadAnimation(file)
			a.read <- animationRead{file: file, samples: samples, err: err}
		}()
	}

	return nil, false
}

// collect keeps the animations read since the last frame. One that could not
// be read is reported, and kept as nil, so that it is not read again.
func (a *animating) collect() {
	for {
		select {
		case read := <-a.read:
			delete(a.reading, read.file)

			if read.err != nil {
				warn(read.err)
			}

			a.samples[read.file] = read.samples

		default:
			return
		}
	}
}

// choices are the sets and their variants a sex can play, in order: the
// default set first, then the others by name, each followed by its variants.
func (a *animating) choices(sex genes.Sex) []animationChoice {
	keys := a.sets.Keys()
	slices.Sort(keys)

	if at := slices.Index(keys, defaultAnimation); at > 0 {
		keys = append([]string{defaultAnimation}, slices.Delete(keys, at, at+1)...)
	}

	var choices []animationChoice

	for _, key := range keys {
		set, _ := a.sets.Get(key)
		choices = append(choices, animationChoice{set: key})

		for _, variant := range set.For(sex).Variants {
			choices = append(choices, animationChoice{set: key, variant: variant.Name})
		}
	}

	return choices
}

// animationChoice is a set, or a variant of it, to play.
type animationChoice struct {
	set, variant string
}

// label is what the choice is shown as: the keys the game gives the set and
// the variant.
func (c animationChoice) label() string {
	if c.variant == "" {
		return c.set
	}

	return c.set + " / " + c.variant
}
