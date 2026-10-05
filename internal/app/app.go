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

	// The scale is asked for here as well as every frame, so that the first
	// frame is already drawn at it rather than the one after.
	application.window.SizeForScale(defaultWindowWidth, defaultWindowHeight, application.interfaceScale())
	application.window.SetScale(application.interfaceScale())
	application.dialogs = systemDialogs{parent: dialogParent()}

	application.dockWindowClass = imgui.NewWindowClass()
	application.window.OnShutdown(func() {
		application.dockWindowClass.Destroy()
	})

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
	return gui.Frame{UI: a.frame}
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
