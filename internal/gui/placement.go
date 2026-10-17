package gui

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Placement is where the window is and how large, kept from one run to the
// next. X, Y, Width and Height are the window's size and position when it is
// not maximized, so that it comes back to them when it is restored.
type Placement struct {
	X, Y          int
	Width, Height int
	Maximized     bool
}

// minimumVisible is how much of the window, from its top left corner, has to
// be on a monitor for a saved position to be used: enough to grab its title
// bar and drag it into view.
const minimumVisible = 100

// Place puts the window where a placement says. A position that is on no
// monitor any more, such as one on a monitor since unplugged, is left to the
// system. A hidden window is not maximized, since maximizing shows it.
func (w *Window) Place(placement Placement, hidden bool) {
	if placement.Width > 0 && placement.Height > 0 {
		rl.SetWindowSize(placement.Width, placement.Height)

		if onAMonitor(placement) {
			rl.SetWindowPosition(placement.X, placement.Y)
		}
	}

	w.trackPlacement()

	if placement.Maximized && !hidden {
		rl.MaximizeWindow()
		w.placement.Maximized = true
	}
}

// Placement is where the window is now, for the next run to start from.
func (w *Window) Placement() Placement {
	return w.placement
}

// trackPlacement notes the window's size and position while it is neither
// maximized nor minimized, which is the size it is restored to.
func (w *Window) trackPlacement() {
	if rl.IsWindowMinimized() {
		return
	}

	if rl.IsWindowMaximized() {
		w.placement.Maximized = true

		return
	}

	position := rl.GetWindowPosition()

	w.placement = Placement{
		X:      int(position.X),
		Y:      int(position.Y),
		Width:  rl.GetScreenWidth(),
		Height: rl.GetScreenHeight(),
	}
}

func onAMonitor(placement Placement) bool {
	for monitor := range rl.GetMonitorCount() {
		origin := rl.GetMonitorPosition(monitor)

		if Overlaps(placement, int(origin.X), int(origin.Y), rl.GetMonitorWidth(monitor), rl.GetMonitorHeight(monitor)) {
			return true
		}
	}

	return false
}

// Overlaps reports whether enough of the top left corner of a placement is
// within a monitor's area to drag the window by.
func Overlaps(placement Placement, x, y, width, height int) bool {
	return placement.X+minimumVisible <= x+width && placement.X+placement.Width-minimumVisible >= x &&
		placement.Y >= y && placement.Y+minimumVisible <= y+height
}
