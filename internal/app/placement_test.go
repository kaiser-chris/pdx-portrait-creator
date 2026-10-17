package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// The first time, and whenever the saved placement cannot be read, the
// window opens maximized.
func TestFirstPlacementIsMaximized(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.json")

	if err := os.WriteFile(broken, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"", filepath.Join(dir, "missing.json"), broken} {
		if got := loadPlacement(path); got != firstPlacement || !got.Maximized {
			t.Errorf("loadPlacement(%q) = %+v, want the first placement, maximized", path, got)
		}
	}
}

func TestPlacementIsKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window.json")

	for _, placement := range []gui.Placement{
		{X: 40, Y: 60, Width: 900, Height: 600},
		{X: -1500, Y: 20, Width: 1280, Height: 800, Maximized: true},
	} {
		savePlacement(path, placement)

		if got := loadPlacement(path); got != placement {
			t.Errorf("saved %+v, read back %+v", placement, got)
		}
	}
}
