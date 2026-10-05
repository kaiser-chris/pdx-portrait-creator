//go:build uitest

package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// startApp runs the real application in a hidden window, with the settings
// file given, or none when it is empty.
func startApp(t *testing.T, settings string) (*App, string) {
	t.Helper()

	// raylib and OpenGL belong to the thread that created the window, and a
	// test runs on a goroutine the scheduler may move between threads.
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	dir := t.TempDir()

	if settings != "" {
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	application, err := New(Options{ConfigDir: dir, Hidden: true})
	if err != nil {
		t.Fatalf("start the application: %v", err)
	}

	return application, dir
}

// The application starts in a hidden window, draws a few frames and closes
// again, writing its layout into the configuration folder as it does.
func TestApplicationStartsAndCloses(t *testing.T) {
	application, dir := startApp(t, "")

	for range 3 {
		application.Step()
	}

	application.Close()

	if !fileExists(filepath.Join(dir, "layout.ini")) {
		t.Error("the layout was not saved when the window closed")
	}
}

// The interface scale stored in the settings is the one the interface is
// drawn at from the first frame on.
func TestStoredInterfaceScaleIsApplied(t *testing.T) {
	application, _ := startApp(t, `{"Folders":[],"InterfaceScale":1.5}`)
	defer application.Close()

	application.Step()

	if got := application.window.Scale(); got != 1.5 {
		t.Errorf("interface scale = %v, want the stored 1.5", got)
	}
}

// A settings file that cannot be read does not keep the application from
// starting: it comes up with the defaults and says so.
func TestBrokenSettingsStartWithTheDefaults(t *testing.T) {
	application, _ := startApp(t, `{"Folders": [`)
	defer application.Close()

	application.Step()

	if len(application.settings.Folders) != 0 {
		t.Errorf("folders = %+v, want none", application.settings.Folders)
	}

	if application.state.status != "Settings could not be loaded, using defaults" {
		t.Errorf("status = %q, want it to say the settings were not loaded", application.state.status)
	}
}

// The settings window draws, with folders in it, without upsetting Dear
// ImGui's checks on the window and table stacks.
func TestSettingsWindowDraws(t *testing.T) {
	application, _ := startApp(t, `{"Folders":[{"Name":"game","Path":"C:\\Games\\Victoria 3\\game"}]}`)
	defer application.Close()

	application.state.showSettings = true

	for range 3 {
		application.Step()
	}

	if !application.state.showSettings {
		t.Error("the settings window closed by itself")
	}
}
