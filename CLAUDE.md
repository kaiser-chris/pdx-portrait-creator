# PDX Portrait Creator

A character portrait editor for Paradox games, written in Go with raylib and
Dear ImGui (cimgui-go). It is built step by step.

## Related repositories

- `../pdx-parser-go` (`github.com/kaiser-chris/pdx-parser-go`): use it for
  anything that parses game files. Extend it there when something is missing
  rather than parsing in this repository, and keep to its standards: standard
  library only, parsing never fails or panics, problems become `report`
  diagnostics with file, line and column, general packages stay game agnostic
  and game specific code goes in a package per game.
- `../pdx-asset-go`: see below.
- `../pdx-flag-builder-go`: an existing editor on the same stack. `internal/gui`
  here started as a copy of its `internal/gui`; look there first for how it
  solved a raylib or cimgui-go problem.

## pdx-asset-go

`../pdx-asset-go` (`github.com/kaiser-chris/pdx-asset-go`) reads and draws the
games' 3D assets: mesh and texture files, entities, rendering with raylib. Put
anything that is not specific to this application there, and keep to its
standards, which are those of pdx-parser-go plus GPU tests that check what is
actually drawn.

pdx-asset-go is not published yet, so this repository builds against the
checkout next door through an ignored `go.work` (`go work init .
../pdx-asset-go ../pdx-parser-go`), and go.mod does not require it until it is
released. The workspace takes pdx-parser-go from next door too, for its
`genes` package, which no release has yet.

## Conventions

- raylib and Dear ImGui are single threaded: everything touching them runs on
  the main goroutine, which `main` locks to its OS thread. Interface tests lock
  the thread themselves.
- Comments explain why, in full sentences, as in the existing code.
- Interface tests are behind the `uitest` build tag and run the real
  application in a hidden window.

## Commands

```
make build     # bin/<platform>/pdx-portrait-creator
make test
make uitest    # needs a display
make gametest PDX_GAME_DIR="E:/SteamLibrary/steamapps/common/Victoria 3/game"
make vet
```
