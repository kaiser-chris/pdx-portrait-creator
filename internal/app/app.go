// Package app is the application shell: the window, its panels and the state
// they share.
//
// The interface is built with Dear ImGui through internal/gui, and the
// portrait will be drawn by raylib into an offscreen target that the preview
// panel samples. Panels are described from scratch every frame, so the whole
// interface is a function of the state in this package.
package app

import (
	"fmt"
	"image"
	"os"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/config"
	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

const (
	applicationName = "PDX Portrait Creator"

	defaultWindowWidth  = 1440
	defaultWindowHeight = 900

	minimumWindowWidth  = 800
	minimumWindowHeight = 600
)

// App owns the window and the state its panels read and write.
type App struct {
	window   *gui.Window
	store    config.Store
	settings *config.Settings
	state    state

	// dialogs asks the user for files and folders.
	dialogs fileDialogs

	// portrait is what the portrait panel shows. It is nil when the 3D view
	// could not be started.
	portrait *portrait

	// swatchTextures are the palettes uploaded for the interface to draw, by
	// the pixels they were uploaded from; see swatchTexture.
	swatchTextures map[*image.RGBA]imgui.TextureRef

	// dockWindowClass is handed to the dock space every frame. cimgui-go
	// dereferences that argument even when it is nil, so one default instance
	// is allocated for the lifetime of the application instead.
	dockWindowClass *imgui.WindowClass
}

// Options changes how the application starts. The zero value is what a user
// gets; the interface tests fill it in.
type Options struct {
	// ConfigDir holds the settings and the saved layout. Empty means the user's
	// configuration directory.
	ConfigDir string

	// Hidden runs the application without showing its window.
	Hidden bool
}

// Run starts the application and returns when its window has been closed.
func Run() error {
	application, err := New(Options{})
	if err != nil {
		return err
	}

	application.window.Run(application.frameSpec())

	return nil
}

// New creates the window and everything the application needs. It must be
// called from the main goroutine.
func New(options Options) (*App, error) {
	store, err := openStore(options.ConfigDir)
	if err != nil {
		return nil, err
	}

	settings, err := store.Load()
	if err != nil {
		// A settings file that cannot be read should not keep the application
		// from starting: it comes up with defaults and says so in the status
		// bar, and the user can repair it from the settings window.
		warn(err)
	}

	application := &App{
		store:    store,
		settings: settings,
		state:    newState(settings),
	}
	if err != nil {
		application.state.status = "Settings could not be loaded, using defaults"
	}

	application.state.layoutBuilt = fileExists(store.LayoutPath())

	application.window = gui.NewWindow(gui.Config{
		Title:      applicationName,
		Width:      defaultWindowWidth,
		Height:     defaultWindowHeight,
		MinWidth:   minimumWindowWidth,
		MinHeight:  minimumWindowHeight,
		LayoutFile: store.LayoutPath(),
		Hidden:     options.Hidden,
	})

	// The window opens maximized the first time, and after that the way it
	// was closed. The scale is asked for here as well as every frame, so that
	// the first frame is already drawn at it rather than the one after.
	placement := loadPlacement(store.PlacementPath())
	if placement.Width == 0 || placement.Height == 0 {
		application.window.SizeForScale(defaultWindowWidth, defaultWindowHeight, application.interfaceScale())
	}

	application.window.Place(placement, options.Hidden)
	application.window.SetScale(application.interfaceScale())
	application.dialogs = systemDialogs{parent: dialogParent()}

	application.dockWindowClass = imgui.NewWindowClass()

	// The 3D view needs the OpenGL context the window just created. Without
	// it the rest of the application still works, so a view that cannot be
	// started is reported rather than refused.
	application.portrait, err = newPortrait()
	if err != nil {
		warn(err)
		application.state.status = "The 3D view could not be started"
	}

	application.window.OnShutdown(func() {
		savePlacement(store.PlacementPath(), application.window.Placement())

		if application.portrait != nil {
			application.portrait.unload()
		}

		application.dockWindowClass.Destroy()
	})

	application.startPortrait()

	return application, nil
}

// Step runs exactly one frame. The interface tests advance the application
// with it.
func (a *App) Step() {
	a.window.Step(a.frameSpec())
}

// Close closes the window and releases everything that needs the OpenGL
// context.
func (a *App) Close() {
	a.window.Close()
}

func (a *App) frameSpec() gui.Frame {
	return gui.Frame{Offscreen: a.offscreen, UI: a.frame}
}

// offscreen draws what the interface shows of the GPU's own drawing, before
// the interface samples it.
func (a *App) offscreen() {
	a.pollPortrait()

	if a.portrait != nil {
		a.portrait.draw()
	}
}

func openStore(dir string) (config.Store, error) {
	if dir == "" {
		return config.UserStore()
	}

	return config.NewStore(dir)
}

// frame builds one frame of the interface.
//
// The order matters: the menu bar and the status bar claim their strip of the
// viewport first, so the dock space that follows covers exactly what is left.
func (a *App) frame() {
	a.pollDialog()

	// Checked every frame so that "Automatic" follows the window from one
	// monitor to another. Nothing happens unless the scale actually changes.
	a.window.SetScale(a.interfaceScale())

	a.state.popupWasOpen = imgui.IsPopupOpenStrV("", imgui.PopupFlagsAnyPopup)

	a.menuBar()
	a.statusBar()
	a.dockSpace()

	a.previewPanel()
	a.characterPanel()

	a.settingsWindow()
	a.aboutWindow()
	a.problemsWindow()
	a.welcomeWindow()
	a.pickerWindow()

	a.handleShortcuts()
}

// trackFocus records which closable window the user interacted with last so
// that Escape closes the one on top.
func (a *App) trackFocus(title string) {
	if imgui.IsWindowFocusedV(imgui.FocusedFlagsRootAndChildWindows) {
		a.state.focusedWindow = title
	}
}

// setStatus replaces the message shown in the status bar.
func (a *App) setStatus(format string, args ...any) {
	a.state.status = fmt.Sprintf(format, args...)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// warn reports a non fatal problem. The application keeps running.
func warn(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "pdx-portrait-creator: %v\n", err)
}
