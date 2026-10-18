# PDX Portrait Creator

A character portrait editor for Paradox games. It is being built step by step;
for now it edits the portraits of Victoria 3 in 3D, read from the game folder
configured in the settings: a man, a woman, a boy or a girl with their eyes,
teeth, hair and beard, their skin, hair and eyes coloured from the game's
palettes, and the head joined to the body the way the game joins them, and
saves them as the DNA of a character, which the game reads from
`common/dna_data`.

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
- **From the Clipboard** is a DNA copied from anywhere, written in any of the
  game's three ways: a definition as `common/dna_data` holds one, the
  persistent DNA the game writes out, or a DNA string. The button says
  which is on the clipboard and, for a definition or persistent DNA, its
  key, which the portrait keeps as its identifier. A persistent DNA also
  gives its sex and age. Comments are left alone, as the game leaves them,
  so a whole file copied is read for the DNA it defines, not for a DNA
  string in a comment above it. The button cannot be used while the
  clipboard holds no DNA, or a DNA string with a different number of genes
  from the game, which reads as the wrong genes.

The picker lists ethnicities and characters by the keys the game defines them
under, such as `dna_abraham_lincoln`, since the files give them no other
name, and draws each as it is picked, as a man, a woman, a boy or a girl: a
character's DNA does not say which. A boy is drawn with the body and head of
a man and a girl with a woman's, the genes' settings for children making a
child of them. The picture shows the face or the whole figure.

## Creating

The portrait fills the window; the mouse turns it: drag to turn, drag with
the right or the middle button to move the view along, scroll to zoom, double
click to frame it again. Over it are the identifier its DNA is saved under,
with **Reset All** back to what it started from, its sex and its age,
twenty-five to start with, and the views of the face and the whole figure.

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

Pictures to model a portrait on, such as a photograph of the person it is
of, are dropped onto the window or picked with **View > Open Reference...**:
a PNG, JPEG, GIF, BMP, WebP or TIFF image opens in a window of its own, which floats over the portrait or docks beside
it like any panel, and shows the picture as large as the window holds it.
The buttons above the picture zoom in and out, as does the mouse wheel around
the pointer, and fit it to the window again, as a double click does; zoomed
in, the picture is dragged to move it. The zoom is a share of the size that
fits the window, so a window made larger shows the same part of the picture,
larger. Escape or the window's cross closes it again.

**View > Problems** lists what could not be read, as does the count in the
status bar.

It runs on Windows and Linux. On Linux its file dialogs need `zenity` or
`kdialog`, one of which most desktops already have.

## Saving

A portrait is saved as the DNA of a character, under the identifier in the
field at the top left, the way the files of `common/dna_data` define one:

```
dna_my_character = {
    portrait_info = {
        genes = {
            hair_color = { 3 248 3 248 }
            gene_cheek_fat = { "cheek_fat" 155 "cheek_fat" 139 }
            ...
        }
    }
    enabled = yes
}
```

A portrait started from a character has its identifier to start with, and
saves back into the file the character came from; one started from an
ethnicity or from scratch has none, and is given one before it is saved:
saving without one, or with one the game could not read, says so in red in
the status bar for ten seconds and has the field blink red for a few.

- **File > Save** (`Ctrl+S`) puts the DNA into the file it is kept in, in
  place of the DNA it was read as, and leaves everything else in the file,
  comments included, as it was. With its identifier changed since, it asks
  whether to **Rename** the DNA it was read as or to **Add as New DNA**
  beside it. A DNA in no file yet asks for a file, as Save To File does.
- **File > Save To File...** (`Ctrl+Shift+S`) asks for a file, a new one or
  one that holds DNA already: a DNA of the same identifier in it is
  replaced where it stands, and anything else is added at its end. The
  dialog opens at the file the DNA is kept in.
- **File > Copy Script** puts the DNA on the clipboard as a file would hold
  it, to paste wherever it is wanted.
- **File > Copy DNA String** puts it on the clipboard as a DNA string, and
  **File > Copy Persistent DNA** as the persistent DNA the game writes out,
  with the sex and age the portrait is shown at.

A saved or copied DNA has its DNA string on the line above it, in a comment,
as the game's own files have it:

```
# z78g+0cGKAYrmni4AAAAfACIAKEAWgB3AH8AdQCAAIAAiwCLAIgAdgBnAHsAegBbAGkAjAC...
dna_my_character = {
```

The string is never read back. A DNA saved again gets a new string above
it, and the strings of earlier saves stay above that, as a history of the
DNA. A DNA string holds only the genes, in the order the game defines them,
so it can only be read with the same genes it was written with.

The game's own files are not written over. With a mod configured after the
game, a character of the game is saved into the mod, at the same place: the
dialog opens at the file of the same name in the mod's `common/dna_data`,
where its DNA overrides the game's. Its recessive alleles, and any gene the
creator does not edit, are written as the game's file has them.

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

The application depends on released versions of both, which go.mod names,
and builds without anything else checked out. To work on one of them and the
application together, check it out next to this repository and add a Go
workspace, which git ignores:

```
go work init . ../pdx-asset-go ../pdx-parser-go
```

Release the library first, and require its new version in go.mod, before the
application relies on what changed in it.

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
