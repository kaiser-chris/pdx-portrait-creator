// Package config loads and stores the user's settings.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// folderName is created inside the user's configuration directory:
	// %AppData%\pdx-portrait-creator on Windows, ~/.config/pdx-portrait-creator
	// on Linux.
	folderName = "pdx-portrait-creator"

	settingsFileName  = "settings.json"
	layoutFileName    = "layout.ini"
	placementFileName = "window.json"
)

// Folder is a game or mod folder that the game data is read from.
type Folder struct {
	Name string
	Path string
}

// Settings is the persisted application configuration.
type Settings struct {
	// Folders are read in order, so a folder further down overrides the ones
	// above it, the way a mod overrides the game.
	Folders []Folder

	// InterfaceScale is how large the interface is drawn, where one is its
	// designed size. Zero follows the display scale of the monitor.
	InterfaceScale float32 `json:",omitempty"`
}

// Default returns the settings used when no settings file exists yet.
func Default() *Settings {
	return &Settings{
		Folders: []Folder{},
	}
}

// Store is the directory the configuration lives in.
//
// The application uses the one in the user's configuration directory. Tests
// point a store at a temporary directory instead, so that nothing they do ever
// touches a real installation.
type Store struct {
	dir string
}

// UserStore returns the store in the user's configuration directory, creating
// the directory when it does not exist yet.
func UserStore() (Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("determine user config directory: %w", err)
	}

	return NewStore(filepath.Join(base, folderName))
}

// NewStore returns a store in the given directory, creating it if necessary.
func NewStore(dir string) (Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Store{}, fmt.Errorf("create config directory %q: %w", dir, err)
	}

	return Store{dir: dir}, nil
}

// SettingsPath returns the full path of the settings file.
func (s Store) SettingsPath() string {
	return filepath.Join(s.dir, settingsFileName)
}

// LayoutPath returns the file Dear ImGui persists the window layout in. It
// lives next to the settings so that removing the config directory resets the
// application completely.
func (s Store) LayoutPath() string {
	return filepath.Join(s.dir, layoutFileName)
}

// PlacementPath returns the file where the window was when the application
// last closed is kept in, next to the settings.
func (s Store) PlacementPath() string {
	return filepath.Join(s.dir, placementFileName)
}

// Load reads the settings file. A missing file is not an error: it yields the
// defaults, which is what a first start looks like.
func (s Store) Load() (*Settings, error) {
	path := s.SettingsPath()

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), fmt.Errorf("read settings %q: %w", path, err)
	}

	settings := Default()
	if err := json.Unmarshal(data, settings); err != nil {
		return Default(), fmt.Errorf("parse settings %q: %w", path, err)
	}

	// A file that says "Folders": null would otherwise hand out a nil list,
	// which saves back as null rather than as an empty list.
	if settings.Folders == nil {
		settings.Folders = []Folder{}
	}

	return settings, nil
}

// Save writes settings to disk.
//
// The file is written next to the old one and then moved over it, so that a
// save interrupted halfway, by a crash or a full disk, leaves the previous
// settings in place rather than half a file.
func (s Store) Save(settings *Settings) error {
	path := s.SettingsPath()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	temporary := path + ".tmp"

	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return fmt.Errorf("write settings %q: %w", path, err)
	}

	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)

		return fmt.Errorf("write settings %q: %w", path, err)
	}

	return nil
}
