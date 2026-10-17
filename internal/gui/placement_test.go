package gui

import "testing"

// A saved position is used only while enough of the window's top is on a
// monitor to drag it by.
func TestOverlaps(t *testing.T) {
	const x, y, width, height = 0, 0, 1920, 1080

	for _, test := range []struct {
		placement Placement
		want      bool
	}{
		{Placement{X: 100, Y: 100, Width: 800, Height: 600}, true},
		// Partly off the left or the right edge, with its title bar on it.
		{Placement{X: -600, Y: 100, Width: 800, Height: 600}, true},
		{Placement{X: 1800, Y: 100, Width: 800, Height: 600}, true},
		// Too far to the left or the right, as on a monitor since unplugged.
		{Placement{X: -2000, Y: 100, Width: 800, Height: 600}, false},
		{Placement{X: 1900, Y: 100, Width: 800, Height: 600}, false},
		// The title bar above the top, or too near the bottom.
		{Placement{X: 100, Y: -50, Width: 800, Height: 600}, false},
		{Placement{X: 100, Y: 1000, Width: 800, Height: 600}, false},
	} {
		if got := Overlaps(test.placement, x, y, width, height); got != test.want {
			t.Errorf("Overlaps(%+v) = %v, want %v", test.placement, got, test.want)
		}
	}
}
