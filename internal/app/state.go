package app

import "github.com/kaiser-chris/pdx-portrait-creator/internal/config"

// folderEntry is one row of the settings window's folder table. The settings
// window edits these copies and only writes them back when the user saves, so
// abandoning an edit cannot corrupt the stored configuration.
type folderEntry struct {
	Name string
	Path string
}

// state is everything the interface remembers between frames that is not worth
// persisting to disk.
type state struct {
	// Windows that float above the docking layout.
	showSettings bool
	showAbout    bool

	// focusedWindow is the closable window the user touched most recently.
	// Escape closes that one.
	focusedWindow string

	// layoutBuilt tracks whether the default docking layout has been applied.
	// It starts true when a saved layout exists so that a user's arrangement is
	// not overwritten on startup.
	layoutBuilt bool

	// Settings window buffers.
	interfaceScale float32
	folders        []folderEntry

	// popupWasOpen records whether a popup, such as the list of a combo box,
	// was open as the frame began, which decides who a press of Escape was
	// meant for.
	popupWasOpen bool

	// dialog is the file dialog that is open, if one is.
	dialog *pendingDialog

	// status is the message shown in the status bar.
	status string

	// welcome is set while the welcome screen is wanted, which it is when
	// the application starts, and picker is the picker of a starting point.
	welcome bool
	picker  picker

	// category is the position of the category the character panel lists,
	// and geneSearch what it searches every gene for instead.
	category   int
	geneSearch string

	// showProblems shows the window of what went wrong reading the portrait.
	showProblems bool
}

func newState(settings *config.Settings) state {
	current := state{status: "Ready", welcome: true}

	current.loadFrom(settings)

	return current
}

// loadFrom resets the settings window buffers to the stored configuration.
func (s *state) loadFrom(settings *config.Settings) {
	s.interfaceScale = settings.InterfaceScale

	s.folders = make([]folderEntry, 0, len(settings.Folders))
	for _, folder := range settings.Folders {
		s.folders = append(s.folders, folderEntry{Name: folder.Name, Path: folder.Path})
	}
}
