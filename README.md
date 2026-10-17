# PDX Portrait Creator

A character portrait editor for Paradox games. It is being built step by step;
for now it edits the portraits of Victoria 3 in 3D, read from the game folder
configured in the settings: a man, a woman, a boy or a girl with their eyes,
teeth, hair and beard, their skin, hair and eyes coloured from the game's
palettes, and the head joined to the body the way the game joins them. Saving and exporting a
portrait are not there yet.

## Starting a portrait

The welcome screen asks what a portrait starts from, and **File > New
Portrait** (`Ctrl+N`) asks again:

- **From Scratch** is `ethnicity_template`, the ethnicity every other builds
  on, as the game's portrait editor previews it: every gene on its first
  template, at the middle.
- **From an Ethnicity** is one of the ethnicities of `common/ethnicities`, at
  its likeliest look: each gene at its likeliest choice, each range at its
  middle.
- **From a Character** is the DNA of one of the characters of
  `common/dna_data`, as the game writes it.

The picker lists ethnicities and characters by the keys the game defines them
under, such as `dna_abraham_lincoln`, since the files give them no other
name, and draws each as it is picked, as a man, a woman, a boy or a girl: a
character's DNA does not say which. A boy is drawn with the body and head of
a man and a girl with a woman's, the genes' settings for children making a
child of them. The picture shows the face or the whole figure.

## Creating

The portrait fills the window; the mouse turns it: drag to turn, drag with
the right or the middle button to move the view along, scroll to zoom, double
click to frame it again. Over it are what it started from, with **Reset All**
back to that, its sex and its age, twenty-five to start with, and the views
of the face and the whole figure.

The menu bar stays usable while the welcome screen or the picker is open,
for the settings first of all: without a game folder configured, nothing can
be started, and the welcome screen says so.

The **Character** panel lists the genes by category: the head, forehead and
brows, eyes, nose, cheeks, mouth, jaw and chin, ears, neck and body, ageing
and skin, hair and beard, and colours, with how many genes of each are changed
from what the portrait started from. Each gene has a name to read, its key
beside it, a choice of its templates where it has more than one, and a slider
from -100 to 100 around the middle; **Reset** sets one back. A hairstyle, a
beard or a moustache is a template and the accessory of it the slider picks.
The colours are picked from the palettes of the skin, the hair and the eyes,
with the rectangles the ethnicity picks them from outlined. The search above
the panel looks through every gene. A gene the creator has no name for, such
as a mod's, is listed under **Other** by its key.

The portrait is lit as the game lights it, by an environment of
`gfx/portraits/environments`, `environment_standard` unless another is picked
in the list over the views: its cubemap, its lights, the shadows they cast,
the ambient occlusion the settings of `gfx/portraits/jomini_environment.txt`
ask for, which darkens the creases of the skin towards a deep red, and the
game's tonemap. **studio** lights it with the renderer's own lights instead.
The list beside it picks one of the game's cameras of
`gfx/portraits/cameras`, such as `portrait_editor` or `camera_body`, which
stands where the game's does; **free**, or Face and Whole Figure, frame it
freely again. The game sets the exposure and the tonemap of its portraits in
its engine, which ships no file they can be read from, so both are fitted to
the game's portrait editor.

The portrait plays an animation set of
`gfx/portraits/portrait_animations`, `idle` unless another is picked in the
list beside the views, as the game does: the set gives the head and the
torso a state each to start in, and their states go on from one animation to
the next. The list holds every set and the variants it has for the
portrait's sex, and **none** for the pose the meshes are stored in; the
button beside it pauses the animation.

With **Highlight Areas** on, pointing at a gene or a category tints the part
of the portrait it changes in light blue, the more the further it moves it,
and shows the portrait plain while a slider is dragged, to see the change.

**View > Problems** lists what could not be read, as does the count in the
status bar.

It runs on Windows and Linux. On Linux its file dialogs need `zenity` or
`kdialog`, one of which most desktops already have.

## Settings

Open **Settings** (`Ctrl+,`) to choose the interface scale and the game and mod
folders to read from. Click **Add Folder**, then **Browse...** to pick the
game's `game` folder or the root folder of a mod. A folder further down the
list overrides the ones above it, the way a mod overrides the game. Nothing is
stored until you click **Save**.

The window opens maximized the first time, and after that the way it was
closed. The settings are kept in `settings.json`, next to the saved window
layout and placement, in `%AppData%\pdx-portrait-creator` on Windows and
`~/.config/pdx-portrait-creator` on Linux. Removing that folder resets the
application.

## Development

The application is written in Go. Its interface is
[Dear ImGui](https://github.com/ocornut/imgui) through
[cimgui-go](https://github.com/AllenDang/cimgui-go), drawn with
[raylib](https://github.com/gen2brain/raylib-go), which also renders the
portrait itself. Game files are read with
[pdx-parser-go](https://github.com/kaiser-chris/pdx-parser-go), and the 3D
models are read and drawn with
[pdx-asset-go](https://github.com/kaiser-chris/pdx-asset-go).

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
| `make uitest` | Runs the interface and GPU tests, which drive the real application in a hidden window and need a display. |
| `make gametest PDX_GAME_DIR=...` | Also runs the tests against a real installation, such as `"C:/Steam/steamapps/common/Victoria 3/game"`. |
| `make vet` | Runs `go vet`. |

Without make, `build.bat` on Windows and `build.sh` on Linux build a release
binary.

### pdx-asset-go and pdx-parser-go

pdx-asset-go is not released yet, and the genes the application reads are in
no release of pdx-parser-go yet, so for now the application builds against
checkouts of both next to this repository, through a Go workspace, which git
ignores:

```
go work init . ../pdx-asset-go ../pdx-parser-go
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
