# PDX Portrait Creator

A character portrait editor for Victoria 3 and Europa Universalis 5. It reads
the portraits straight from your game and mods and draws them in 3D, lit and
animated as the game shows them. You can edit a character's DNA and save it
back for the game to use. You can also preview and edit portrait modifiers,
the clothes and other looks the game puts on its characters.

It runs on Windows and Linux.

## Getting started

1. Download the latest build from the
   [Releases](https://github.com/kaiser-chris/pdx-portrait-creator/releases)
   page and start it.
2. Open **Settings** (`Ctrl+,`), click **Add Folder** and pick the game's
   `game` folder. Add any mods after it: a folder further down the list
   overrides the ones above it, as a mod overrides the game. Click **Save**.
3. On the welcome screen, choose what to work on:
   - **Character:** a portrait made from scratch, from an ethnicity, from a
     character of the game, or from DNA on the clipboard;
   - **Portrait Modifiers:** the game's portrait modifiers, worn by a
     character of your choice.

## Editing a character

- **The view:** drag to turn the portrait, drag with the right or middle
  button to move it, scroll to zoom, and double-click to frame it again. Above
  the portrait you can pick its sex, age, environment, camera and animation.
- **The Character panel:** lists every gene by category, with a slider and,
  where there are several, a choice of templates. Colours are picked from the
  game's palettes.
- **The Portrait Modifiers tab:** next to the Character panel. It dresses the
  character in the game's clothes and other looks, to preview them.
- **Reference pictures:** drop an image onto the window to see it beside the
  portrait.

## Saving

- **File > Save** (`Ctrl+S`) writes the DNA back into the file it came from.
  Everything else in that file is left as it was.
- **File > Save To File...** (`Ctrl+Shift+S`) writes it into a file you pick.
- **File > Copy Script**, **Copy DNA String** and **Copy Persistent DNA** put
  the DNA on the clipboard.

The game's own files are never written over. With a mod configured, a
character of the game is saved into the mod instead, where it overrides the
game's.

On Linux, the file dialogs need `zenity` or `kdialog`. Most desktops already
have one of them.

## Building

The application is written in Go 1.26, with
[Dear ImGui](https://github.com/ocornut/imgui) through
[cimgui-go](https://github.com/AllenDang/cimgui-go) and
[raylib](https://github.com/gen2brain/raylib-go). Game files are read with
[pdx-parser-go](https://github.com/kaiser-chris/pdx-parser-go), and models are
drawn with [pdx-asset-go](https://github.com/kaiser-chris/pdx-asset-go).

Building needs a C compiler: MinGW-w64 on Windows, GCC on Linux. On Linux it
also needs the OpenGL, X11, Wayland and xkbcommon headers:

```
sudo apt-get install libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev
```

Use `make build`, `make run` or `make test`. Without make, use `build.bat` on
Windows or `build.sh` on Linux. The first build compiles raylib and Dear ImGui,
which takes a few minutes.

## Licence

MIT, see [LICENSE](LICENSE).

The bundled fonts have their own licences:

- Roboto: SIL Open Font License (`assets/fonts/roboto/OFL.txt`).
- Material Symbols: Apache License 2.0
  (`assets/fonts/material-symbols/LICENSE.txt`).

The application icon is "design" by Resa Valen from
[Noun Project](https://thenounproject.com/browse/icons/term/design/), licensed
under CC BY 3.0. **Help > About** credits them all.
