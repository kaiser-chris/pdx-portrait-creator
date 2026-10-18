package app

import (
	"image"
	"math"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"
)

// writeImage writes a picture of one colour to a file, as PNG or JPEG by its
// extension.
func writeImage(t *testing.T, path string, width, height int) {
	t.Helper()

	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for index := 0; index < len(picture.Pix); index += 4 {
		copy(picture.Pix[index:], []byte{200, 40, 40, 255})
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if filepath.Ext(path) == ".jpg" {
		err = jpeg.Encode(file, picture, nil)
	} else {
		err = png.Encode(file, picture)
	}

	if err != nil {
		t.Fatal(err)
	}
}

// The common image files open as references, whatever the case of their
// extension, and nothing else does.
func TestIsReference(t *testing.T) {
	for path, want := range map[string]bool{
		"photo.png": true, "photo.JPG": true, "photo.jpeg": true, "photo.gif": true,
		"photo.bmp": true, "photo.webp": true, "photo.tiff": true,
		"dna.txt": false, "head.dds": false, "noextension": false,
	} {
		if got := isReference(path); got != want {
			t.Errorf("isReference(%q) = %v, want %v", path, got, want)
		}
	}
}

// A reference is read as it is, a large one scaled down to fit in
// maxReferenceSide keeping its proportions, and a file that is no image says
// so.
func TestReadReference(t *testing.T) {
	dir := t.TempDir()

	small := filepath.Join(dir, "small.jpg")
	writeImage(t, small, 30, 20)

	picture, err := readReference(small)
	if err != nil || picture.Bounds().Dx() != 30 || picture.Bounds().Dy() != 20 {
		t.Fatalf("small image read as %v, err %v", picture.Bounds(), err)
	}

	if got := picture.RGBAAt(5, 5); got.R < 180 || got.G > 70 {
		t.Errorf("small image pixel %v, want the red it was written in", got)
	}

	large := filepath.Join(dir, "large.png")
	writeImage(t, large, maxReferenceSide*2, maxReferenceSide)

	picture, err = readReference(large)
	if err != nil || picture.Bounds().Dx() != maxReferenceSide || picture.Bounds().Dy() != maxReferenceSide/2 {
		t.Errorf("large image read as %v, err %v, want it halved", picture.Bounds(), err)
	}

	if got := picture.RGBAAt(100, 100); got != (color.RGBA{200, 40, 40, 255}) {
		t.Errorf("scaled image pixel %v, want the colour kept", got)
	}

	broken := filepath.Join(dir, "broken.png")
	if err := os.WriteFile(broken, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := readReference(broken); err == nil {
		t.Error("a file that is no image was read")
	}
}

func nearly(a, b float32) bool {
	return math.Abs(float64(a-b)) < 1e-4
}

// A picture fits its window at a zoom of one, whole and in the middle, and
// twice zoomed in shows half of it, at twice the size, however large the
// window is: a window twice as large shows the same half, twice as large.
func TestViewOfZoom(t *testing.T) {
	centre := imgui.Vec2{X: 0.5, Y: 0.5}

	fitted := viewOf(imgui.Vec2{X: 400, Y: 300}, 200, 100, 1, &centre)
	if fitted.size != (imgui.Vec2{X: 400, Y: 200}) || fitted.uv0 != (imgui.Vec2{}) || fitted.uv1 != (imgui.Vec2{X: 1, Y: 1}) {
		t.Errorf("fitted: %+v, want the whole picture at 400 by 200", fitted)
	}

	for _, space := range []imgui.Vec2{{X: 400, Y: 300}, {X: 800, Y: 600}} {
		centre := imgui.Vec2{X: 0.5, Y: 0.5}
		zoomed := viewOf(space, 200, 100, 2, &centre)

		if !nearly(zoomed.uv1.X-zoomed.uv0.X, 0.5) || !nearly(zoomed.pictureSize.X, space.X*2) || !nearly(zoomed.size.X, space.X) {
			t.Errorf("zoomed in %v: %+v, want half the picture across the window", space, zoomed)
		}
	}

	// Zoomed out, the whole picture is shown smaller, in the middle.
	centre = imgui.Vec2{X: 0.9, Y: 0.1}

	out := viewOf(imgui.Vec2{X: 400, Y: 300}, 200, 100, 0.5, &centre)
	if out.size != (imgui.Vec2{X: 200, Y: 100}) || out.uv0 != (imgui.Vec2{}) || centre != (imgui.Vec2{X: 0.5, Y: 0.5}) {
		t.Errorf("zoomed out: %+v, centre %v, want the whole picture at half the size", out, centre)
	}
}

// A picture moved past its edge stops at the edge.
func TestViewOfStopsAtTheEdge(t *testing.T) {
	centre := imgui.Vec2{X: 0.99, Y: -1}

	view := viewOf(imgui.Vec2{X: 400, Y: 200}, 200, 100, 4, &centre)
	if !nearly(view.uv1.X, 1) || !nearly(view.uv0.Y, 0) || !nearly(centre.X, 0.875) || !nearly(centre.Y, 0.125) {
		t.Errorf("moved past the edge: %+v, centre %v, want the corner", view, centre)
	}
}

// Zooming keeps the point zoomed at where it was, stays within its limits,
// and a drag moves the picture with the pointer.
func TestReferenceZoomAndPan(t *testing.T) {
	r := &reference{zoom: 1, centre: imgui.Vec2{X: 0.5, Y: 0.5}}

	r.zoomAround(2, imgui.Vec2{X: 0.75, Y: 0.5})
	if r.zoom != 2 || !nearly(r.centre.X, 0.625) {
		t.Errorf("zoomed at three quarters: zoom %v centre %v, want 2 and 0.625", r.zoom, r.centre)
	}

	for range 40 {
		r.zoomBy(zoomStep)
	}

	if r.zoom != maxZoom {
		t.Errorf("zoomed in far: %v, want %v", r.zoom, float32(maxZoom))
	}

	for range 80 {
		r.zoomBy(1 / zoomStep)
	}

	if r.zoom != minZoom {
		t.Errorf("zoomed out far: %v, want %v", r.zoom, float32(minZoom))
	}

	r.fit()
	r.zoom = 2

	view := viewOf(imgui.Vec2{X: 400, Y: 200}, 200, 100, r.zoom, &r.centre)
	r.pan(imgui.Vec2{X: 80, Y: 0}, view)

	// Dragged right by a tenth of the picture's drawn width, it shows what is
	// a tenth further left.
	if !nearly(r.centre.X, 0.4) {
		t.Errorf("dragged: centre %v, want 0.4", r.centre)
	}
}
