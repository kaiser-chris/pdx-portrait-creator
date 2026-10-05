# PDX Portrait Creator

A character portrait editor for Paradox games. It is being built step by step;
for now it is the application shell: a window with dockable panels that the
editor will fill.

It runs on Windows and Linux. On Linux its file dialogs need `zenity` or
`kdialog`, one of which most desktops already have.

## Settings

Open **Settings** (`Ctrl+,`) to choose the interface scale and the game and mod
folders to read from. Click **Add Folder**, then **Browse...** to pick the
game's `game` folder or the root folder of a mod. A folder further down the
list overrides the ones above it, the way a mod overrides the game. Nothing is
stored until you click **Save**.

The settings are kept in `settings.json`, next to the saved window layout, in
`%AppData%\pdx-portrait-creator` on Windows and `~/.config/pdx-portrait-creator`
on Linux. Removing that folder resets the application.

## Development

The application is written in Go. Its interface is
[Dear ImGui](https://github.com/ocornut/imgui) through
[cimgui-go](https://github.com/AllenDang/cimgui-go), drawn with
[raylib](https://github.com/gen2brain/raylib-go), which also renders the
portrait itself. Game files are read with
[pdx-parser-go](https://github.com/kaiser-chris/pdx-parser-go).

### Requirements

- Go 1.26
- A C compiler for cgo, since raylib and Dear ImGui are compiled from C and
  C++: MinGW-w64 on Windows, GCC on Linux.
- On Linux, the headers of OpenGL, X11, Wayland and xkbcommon. On Debian or
  Ubuntu:

  ```
  sudo apt-get install libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev
  ```

The first build compiles raylib and Dear ImGui and takes a few minutes; later
builds reuse Go's build cache.

### Building

| Command | What it does |
|---|---|
| `make build` | Builds a development binary into `bin/<platform>`. |
| `make run` | Builds and starts the application. |
| `make release` | Builds an optimised binary, without a console window on Windows. |
| `make test` | Runs the tests. |
| `make uitest` | Runs the interface tests, which drive the real application in a hidden window and need a display. |
| `make vet` | Runs `go vet`. |

Without make, `build.bat` on Windows and `build.sh` on Linux build a release
binary.

### Working on pdx-parser-go alongside

To build against a checkout of pdx-parser-go next to this repository rather
than the released module, create a Go workspace, which git ignores:

```
go work init . ../pdx-parser-go
```

### Layout

| Path | What is in it |
|---|---|
| `cmd/pdx-portrait-creator` | The entry point. |
| `internal/app` | The application shell: the window, its panels and the state they share. |
| `internal/config` | Reading and writing the settings file. |
| `internal/gui` | Dear ImGui on top of raylib: the window, the frame loop, fonts, theme and interface scale. |
| `assets` | Files built into the executable, such as the fonts. |

### Releases

Releases are made by hand from the **Release** workflow in the Actions tab. It
works out the next version from the latest release tag, builds and tests on
Linux and Windows, tags the commit and publishes the builds.

## Licence

MIT, see [LICENSE](LICENSE). The bundled fonts come with their own licences:
Roboto under the SIL Open Font License (`assets/fonts/roboto/OFL.txt`) and
Material Symbols under the Apache License 2.0
(`assets/fonts/material-symbols/LICENSE.txt`).
