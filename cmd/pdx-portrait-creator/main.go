// Command pdx-portrait-creator is a character portrait editor for Paradox
// games.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/app"
)

func init() {
	// raylib creates the window and the OpenGL context on the calling thread
	// and both have to stay there, so the main goroutine is pinned before
	// anything else runs.
	runtime.LockOSThread()
}

func main() {
	if err := app.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "pdx-portrait-creator: %v\n", err)
		os.Exit(1)
	}
}
