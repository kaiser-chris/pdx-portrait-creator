package app

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"
	scale "golang.org/x/image/draw"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// Image files dropped onto the window open as references: pictures to model a
// portrait on, such as a photograph of the person it is of, each in a window
// of its own that can float or be docked beside the portrait.

// referenceExtensions are the image files that open as references.
var referenceExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tif", ".tiff"}

// referenceKinds names them for a message saying a file is not one.
const referenceKinds = "PNG, JPEG, GIF, BMP, WebP or TIFF"

// maxReferenceSide is the most pixels a side of a reference is kept at: a
// photograph straight from a camera is larger than any screen shows it, and
// larger than some graphics cards take as one texture.
const maxReferenceSide = 4096

// referenceTitlePrefix starts the identity of every reference window, which
// tells them apart from the other windows.
const referenceTitlePrefix = "##reference"

// reference is one picture shown as a reference.
type reference struct {
	id      int
	name    string
	picture *image.RGBA
	texture imgui.TextureRef

	// open is cleared when the window is closed. Its texture is let go of
	// the frame after, once nothing draws it any more.
	open bool

	// zoom is how large the picture is shown, as a share of the size that
	// fits it in its window, so that it stays as zoomed in however the
	// window is sized; centre is the point of the picture in the middle of
	// the window, from 0 to 1 across and down.
	zoom   float32
	centre imgui.Vec2
}

// title is what the window of a reference is called: the file's name, and an
// identity of its own, so that a picture dropped twice opens twice.
func (r *reference) title() string {
	return fmt.Sprintf("%s%s%d", r.name, referenceTitlePrefix, r.id)
}

// decodedReference is a dropped file, read, or what went wrong reading it.
type decodedReference struct {
	name    string
	picture *image.RGBA
	err     error
}

// references are the reference windows and what is on its way to one.
type references struct {
	shown   []*reference
	next    int
	decoded chan decodedReference
}

// isReference reports whether a file is an image that opens as a reference,
// by its extension.
func isReference(path string) bool {
	return slices.Contains(referenceExtensions, strings.ToLower(filepath.Ext(path)))
}

// pollDroppedFiles opens the image files dropped onto the window as
// references, each read on a goroutine of its own, and says which files are
// not images.
func (a *App) pollDroppedFiles() {
	if !rl.IsFileDropped() {
		return
	}

	dropped := rl.LoadDroppedFiles()
	rl.UnloadDroppedFiles()

	a.dropFiles(dropped)
}

// dropFiles opens files as references.
func (a *App) dropFiles(paths []string) {
	if a.references.decoded == nil {
		a.references.decoded = make(chan decodedReference, 16)
	}

	var refused []string

	for _, path := range paths {
		if !isReference(path) {
			refused = append(refused, filepath.Base(path))

			continue
		}

		decoded := a.references.decoded

		go func() {
			picture, err := readReference(path)
			decoded <- decodedReference{name: filepath.Base(path), picture: picture, err: err}
		}()
	}

	if len(refused) > 0 {
		a.setError("%s cannot be shown as a reference: drop %s images", strings.Join(refused, ", "), referenceKinds)
	}
}

// readReference reads an image file into pixels the interface can upload,
// scaled down to maxReferenceSide.
func readReference(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoded, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("the image is empty")
	}

	if side := max(width, height); side > maxReferenceSide {
		width = max(1, width*maxReferenceSide/side)
		height = max(1, height*maxReferenceSide/side)
	}

	picture := image.NewRGBA(image.Rect(0, 0, width, height))

	if width == bounds.Dx() && height == bounds.Dy() {
		draw.Draw(picture, picture.Bounds(), decoded, bounds.Min, draw.Src)
	} else {
		scale.CatmullRom.Scale(picture, picture.Bounds(), decoded, bounds, draw.Src, nil)
	}

	return picture, nil
}

// pollReferences opens the windows of the references that have been read.
func (a *App) pollReferences() {
	for {
		select {
		case decoded := <-a.references.decoded:
			if decoded.err != nil {
				a.setError("%s could not be read: %v", decoded.name, decoded.err)

				continue
			}

			a.addReference(decoded.name, decoded.picture)

		default:
			return
		}
	}
}

// addReference opens a window showing a picture.
func (a *App) addReference(name string, picture *image.RGBA) *reference {
	bounds := picture.Bounds()

	shown := &reference{
		id:      a.references.next,
		name:    name,
		picture: picture,
		texture: a.window.Backend().CreateTextureRgba(picture, bounds.Dx(), bounds.Dy()),
		open:    true,
		zoom:    1,
		centre:  imgui.Vec2{X: 0.5, Y: 0.5},
	}

	a.references.next++
	a.references.shown = append(a.references.shown, shown)
	a.setStatus("Opened %s as a reference", name)

	return shown
}

// referenceWindows draws the window of every reference. A window opens at a
// size that shows the whole picture, each a little below and beside the one
// before, and shows the picture as large as it fits, wherever the window is
// docked or however it is sized.
func (a *App) referenceWindows() {
	a.releaseClosedReferences()

	viewport := imgui.MainViewport()
	centre := viewport.Center()

	for _, shown := range a.references.shown {
		bounds := shown.picture.Bounds()
		width, height := float32(bounds.Dx()), float32(bounds.Dy())

		// The picture opens no larger than this, smaller if it is.
		fit := min(1, gui.Scaled(480)/max(width, height))
		size := imgui.Vec2{X: width*fit + gui.Scaled(16), Y: height*fit + imgui.FrameHeight() + imgui.FrameHeightWithSpacing() + gui.Scaled(16)}
		offset := float32(shown.id%8) * gui.Scaled(28)

		imgui.SetNextWindowPosV(imgui.Vec2{X: centre.X + offset, Y: centre.Y + offset}, imgui.CondAppearing, imgui.Vec2{X: 0.5, Y: 0.5})
		imgui.SetNextWindowSizeV(size, imgui.CondAppearing)

		visible := imgui.BeginV(shown.title(), &shown.open, imgui.WindowFlagsNoSavedSettings|imgui.WindowFlagsNoScrollbar|imgui.WindowFlagsNoScrollWithMouse)
		a.trackFocus(shown.title())

		if visible && shown.open {
			shown.draw(width, height)
		}

		imgui.End()
	}
}

// The zoom of a reference: from a quarter of the size that fits it in its
// window to sixteen times that, a step of the buttons or the mouse wheel
// apart.
const (
	minZoom  = 0.25
	maxZoom  = 16
	zoomStep = 1.25
)

// draw draws the zoom buttons of a reference, and the picture below them,
// zoomed and moved as the user has it: dragged to move it, the mouse wheel
// zooming in and out around the pointer, and a double click fitting it to
// the window again.
func (r *reference) draw(width, height float32) {
	if imgui.Button(gui.IconZoomOut + "##zoomOut") {
		r.zoomBy(1 / zoomStep)
	}

	imgui.SetItemTooltip("Zoom out")
	imgui.SameLine()

	if imgui.Button(gui.IconZoomIn + "##zoomIn") {
		r.zoomBy(zoomStep)
	}

	imgui.SetItemTooltip("Zoom in")
	imgui.SameLine()

	if imgui.Button(gui.IconFit + "##fit") {
		r.fit()
	}

	imgui.SetItemTooltip("Fit the picture to the window (double click it)")
	imgui.SameLine()
	imgui.AlignTextToFramePadding()
	// Dear ImGui reads the text as a format of its own, so its percent sign
	// is written twice.
	imgui.TextDisabled(fmt.Sprintf("%.0f%%%%", r.zoom*100))

	space := imgui.ContentRegionAvail()
	if space.X <= 1 || space.Y <= 1 {
		return
	}

	view := viewOf(space, width, height, r.zoom, &r.centre)

	cursor := imgui.CursorPos()
	imgui.SetCursorPos(imgui.Vec2{X: cursor.X + (space.X-view.size.X)/2, Y: cursor.Y + (space.Y-view.size.Y)/2})

	// The picture is a button no one sees, which takes the drags and the
	// wheel over it, and is drawn over.
	imgui.InvisibleButtonV("##picture", view.size, imgui.ButtonFlagsMouseButtonLeft)

	low, high := imgui.ItemRectMin(), imgui.ItemRectMax()
	imgui.WindowDrawList().AddImageV(r.texture, low, high, view.uv0, view.uv1, 0xFFFFFFFF)

	io := imgui.CurrentIO()

	if imgui.IsItemActive() && imgui.IsMouseDragging(imgui.MouseButtonLeft) {
		r.pan(io.MouseDelta(), view)
	}

	if !imgui.IsItemHovered() {
		return
	}

	if view.uv1.X-view.uv0.X < 1 || view.uv1.Y-view.uv0.Y < 1 {
		imgui.SetMouseCursor(imgui.MouseCursorResizeAll)
	}

	if imgui.IsMouseDoubleClicked(imgui.MouseButtonLeft) {
		r.fit()
	}

	if wheel := io.MouseWheel(); wheel != 0 {
		pointer := imgui.MousePos()
		at := imgui.Vec2{
			X: view.uv0.X + (pointer.X-low.X)/view.size.X*(view.uv1.X-view.uv0.X),
			Y: view.uv0.Y + (pointer.Y-low.Y)/view.size.Y*(view.uv1.Y-view.uv0.Y),
		}

		step := float32(zoomStep)
		if wheel < 0 {
			step = 1 / zoomStep
		}

		r.zoomAround(step, at)
	}
}

// zoomBy zooms in by a factor above one, out by one below, around the middle
// of what is shown.
func (r *reference) zoomBy(factor float32) {
	r.zoomAround(factor, r.centre)
}

// zoomAround zooms by a factor so that the point of the picture at, from 0 to
// 1 across and down, stays where it is in the window, as it does under the
// pointer when the mouse wheel zooms.
func (r *reference) zoomAround(factor float32, at imgui.Vec2) {
	zoom := min(max(r.zoom*factor, minZoom), maxZoom)
	kept := r.zoom / zoom

	r.centre = imgui.Vec2{X: at.X - (at.X-r.centre.X)*kept, Y: at.Y - (at.Y-r.centre.Y)*kept}
	r.zoom = zoom
}

// pan moves the picture by a distance on the screen, as a drag moves it.
func (r *reference) pan(moved imgui.Vec2, view referenceView) {
	r.centre.X -= moved.X / view.pictureSize.X
	r.centre.Y -= moved.Y / view.pictureSize.Y
}

// fit shows the whole picture again, as large as the window holds it.
func (r *reference) fit() {
	r.zoom, r.centre = 1, imgui.Vec2{X: 0.5, Y: 0.5}
}

// referenceView is what of a picture a window shows: how large it draws it,
// and which part of it, from uv0 to uv1, as 0 to 1 across and down.
type referenceView struct {
	size, uv0, uv1 imgui.Vec2

	// pictureSize is how large the whole picture is drawn at this zoom, of
	// which size is the part the window holds.
	pictureSize imgui.Vec2
}

// viewOf works out what of a picture of a size a space shows at a zoom, as a
// share of the size that fits it in the space, around a centre. The centre is
// moved, where it has to be, so that the window shows no more than the edge
// of a picture larger than it, and the middle of a smaller one.
func viewOf(space imgui.Vec2, width, height, zoom float32, centre *imgui.Vec2) referenceView {
	scale := min(space.X/width, space.Y/height) * zoom
	picture := imgui.Vec2{X: width * scale, Y: height * scale}

	view := referenceView{
		size:        imgui.Vec2{X: min(space.X, picture.X), Y: min(space.Y, picture.Y)},
		pictureSize: picture,
	}

	half := imgui.Vec2{X: view.size.X / picture.X / 2, Y: view.size.Y / picture.Y / 2}

	centre.X = min(max(centre.X, half.X), 1-half.X)
	centre.Y = min(max(centre.Y, half.Y), 1-half.Y)

	view.uv0 = imgui.Vec2{X: centre.X - half.X, Y: centre.Y - half.Y}
	view.uv1 = imgui.Vec2{X: centre.X + half.X, Y: centre.Y + half.Y}

	return view
}

// releaseClosedReferences lets go of the references whose windows were closed
// last frame, textures and all.
func (a *App) releaseClosedReferences() {
	a.references.shown = slices.DeleteFunc(a.references.shown, func(shown *reference) bool {
		if shown.open {
			return false
		}

		a.window.Backend().DeleteTexture(shown.texture)

		return true
	})
}

// closeReference closes the reference whose window has a title.
func (a *App) closeReference(title string) {
	for _, shown := range a.references.shown {
		if shown.title() == title {
			shown.open = false
		}
	}
}
