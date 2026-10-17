package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// firstPlacement is how the window opens the first time: maximized, and
// restored to the default size.
var firstPlacement = gui.Placement{Maximized: true}

// loadPlacement reads where the window was when the creator last closed. A
// file that is missing or cannot be read gives the first placement.
func loadPlacement(path string) gui.Placement {
	if path == "" {
		return firstPlacement
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			warn(err)
		}

		return firstPlacement
	}

	var placement gui.Placement
	if err := json.Unmarshal(data, &placement); err != nil {
		warn(fmt.Errorf("read %s: %w", path, err))

		return firstPlacement
	}

	return placement
}

// savePlacement keeps where the window is for the next run.
func savePlacement(path string, placement gui.Placement) {
	if path == "" {
		return
	}

	data, err := json.MarshalIndent(placement, "", "  ")
	if err != nil {
		warn(err)

		return
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		warn(err)
	}
}
