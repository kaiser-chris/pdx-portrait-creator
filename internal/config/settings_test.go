package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func newTestStore(t *testing.T) Store {
	t.Helper()

	store, err := NewStore(filepath.Join(t.TempDir(), "config"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	return store
}

func writeSettings(t *testing.T, store Store, content string) {
	t.Helper()

	if err := os.WriteFile(store.SettingsPath(), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadWithoutAFileGivesTheDefaults(t *testing.T) {
	settings, err := newTestStore(t).Load()
	if err != nil {
		t.Fatalf("Load on a first start: %v", err)
	}

	if !reflect.DeepEqual(settings, Default()) {
		t.Errorf("Load = %+v, want the defaults", settings)
	}
}

func TestSaveAndLoad(t *testing.T) {
	store := newTestStore(t)

	want := &Settings{
		Folders: []Folder{
			{Name: "game", Path: `E:\SteamLibrary\steamapps\common\Victoria 3\game`},
			{Name: "mod", Path: "/home/user/mods/gate"},
		},
		InterfaceScale: 1.5,
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %+v, want what was saved, %+v", got, want)
	}
}

// Saving goes through a temporary file, which must not be left behind.
func TestSaveLeavesOnlyTheSettingsFile(t *testing.T) {
	store := newTestStore(t)

	for range 2 {
		if err := store.Save(Default()); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	entries, err := os.ReadDir(filepath.Dir(store.SettingsPath()))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name() != settingsFileName {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		t.Errorf("config folder holds %v, want only %s", names, settingsFileName)
	}
}

// A file written by a later version, with settings this one does not know,
// still loads: what it knows is read and the rest is ignored rather than
// refused. A file from before a setting existed gets that setting's default.
func TestLoadIgnoresUnknownSettings(t *testing.T) {
	store := newTestStore(t)

	writeSettings(t, store, `{"Theme":"light","Folders":[{"Name":"game","Path":"C:\\Games\\Victoria 3\\game"}]}`)

	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := []Folder{{Name: "game", Path: `C:\Games\Victoria 3\game`}}
	if !reflect.DeepEqual(settings.Folders, want) {
		t.Errorf("folders = %+v, want %+v", settings.Folders, want)
	}

	if settings.InterfaceScale != 0 {
		t.Errorf("interface scale = %v, want 0 for automatic", settings.InterfaceScale)
	}
}

func TestLoadWithoutFoldersGivesAnEmptyList(t *testing.T) {
	store := newTestStore(t)

	writeSettings(t, store, `{"Folders":null,"InterfaceScale":2}`)

	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if settings.Folders == nil || len(settings.Folders) != 0 {
		t.Errorf("folders = %#v, want an empty list", settings.Folders)
	}
}

func TestLoadABrokenFile(t *testing.T) {
	store := newTestStore(t)

	writeSettings(t, store, `{"Folders": [`)

	settings, err := store.Load()
	if err == nil {
		t.Error("Load of a broken file gave no error, want one to report")
	}

	// The application still starts, with the defaults.
	if !reflect.DeepEqual(settings, Default()) {
		t.Errorf("Load = %+v, want the defaults to start with", settings)
	}
}

func TestLayoutLivesNextToTheSettings(t *testing.T) {
	store := newTestStore(t)

	if filepath.Dir(store.LayoutPath()) != filepath.Dir(store.SettingsPath()) {
		t.Errorf("layout at %s and settings at %s, want them in one folder", store.LayoutPath(), store.SettingsPath())
	}
}
