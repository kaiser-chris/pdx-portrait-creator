package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/config"
)

// newSettingsApp is an application without a window, which is all the
// settings window's logic needs: it works on the state and the store alone.
func newSettingsApp(t *testing.T, settings *config.Settings) *App {
	t.Helper()

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config"))
	if err != nil {
		t.Fatal(err)
	}

	return &App{store: store, settings: settings, state: newState(settings)}
}

func TestSaveSettingsWritesTheEdits(t *testing.T) {
	application := newSettingsApp(t, config.Default())

	application.state.interfaceScale = 1.25
	application.state.folders = []folderEntry{
		{Name: "game", Path: `C:\Games\Victoria 3\game`},
		{Name: "mod", Path: `C:\mods\my_mod`},
	}

	if !application.settingsChanged() {
		t.Fatal("the edits do not count as unsaved")
	}

	application.saveSettings()

	stored, err := application.store.Load()
	if err != nil {
		t.Fatal(err)
	}

	want := &config.Settings{
		Folders: []config.Folder{
			{Name: "game", Path: `C:\Games\Victoria 3\game`},
			{Name: "mod", Path: `C:\mods\my_mod`},
		},
		InterfaceScale: 1.25,
	}

	if !reflect.DeepEqual(stored, want) {
		t.Errorf("stored settings = %+v, want %+v", stored, want)
	}

	if application.settingsChanged() {
		t.Error("the settings still count as unsaved after saving")
	}
}

// A row with only a name or only a path is an edit in progress. It is not
// stored, and once saved the window no longer shows it.
func TestSaveSettingsSkipsHalfFilledRows(t *testing.T) {
	application := newSettingsApp(t, config.Default())

	application.state.folders = []folderEntry{
		{Name: "game", Path: `C:\Games\Victoria 3\game`},
		{Name: "unfinished"},
		{Path: `C:\mods\nameless`},
	}

	application.saveSettings()

	want := []config.Folder{{Name: "game", Path: `C:\Games\Victoria 3\game`}}
	if !reflect.DeepEqual(application.settings.Folders, want) {
		t.Errorf("folders in effect = %+v, want %+v", application.settings.Folders, want)
	}

	if len(application.state.folders) != 1 {
		t.Errorf("the window shows %d rows after saving, want the 1 that was stored", len(application.state.folders))
	}
}

func TestRevertingRestoresTheStoredSettings(t *testing.T) {
	stored := &config.Settings{
		Folders:        []config.Folder{{Name: "game", Path: "/games/victoria3/game"}},
		InterfaceScale: 2,
	}
	application := newSettingsApp(t, stored)

	application.state.interfaceScale = 1
	application.state.folders[0].Path = "/somewhere/else"
	application.state.folders = append(application.state.folders, folderEntry{Name: "mod"})

	application.state.loadFrom(application.settings)

	if application.settingsChanged() {
		t.Errorf("after reverting the window shows %+v at scale %v, want what is stored",
			application.state.folders, application.state.interfaceScale)
	}
}

// When the file cannot be written the settings in effect stay as they were,
// and the window keeps the edits for another try.
func TestFailedSaveKeepsTheEdits(t *testing.T) {
	application := newSettingsApp(t, config.Default())

	// A folder where the settings file belongs cannot be replaced by a file.
	if err := os.Mkdir(application.store.SettingsPath(), 0o755); err != nil {
		t.Fatal(err)
	}

	application.state.folders = []folderEntry{{Name: "game", Path: "/games/victoria3/game"}}
	application.saveSettings()

	if len(application.settings.Folders) != 0 {
		t.Errorf("folders in effect = %+v after a failed save, want none", application.settings.Folders)
	}

	if !application.settingsChanged() {
		t.Error("the edits were lost when saving failed")
	}
}
