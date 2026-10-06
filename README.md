# GnistaEngine

*Gnista* is Swedish for a spark.

GnistaEngine is a game engine in Go (on [Ebitengine](https://ebitengine.org/)) that runs the Amiga
game *Persian Gulf Inferno* (Innerprise, 1989) from your own copy of its disk. It runs the game's logic
at the original's 50 Hz and plays the original's graphics, sound and music. It can also run your own
game from a *pack*: your own pictures, map, sounds, music and scripts.

**This repository contains none of the game's data, and it is not affiliated with or endorsed by
the game's authors or rights holders.** The engine reads the original files from a disk image that
you provide, and a tool in this repository prepares them for you. The game, its graphics, sounds and
music belong to their rights holders. Nothing of it is distributed here, and the tools are not meant
to be used with a copy you have no right to.

## Quick start

You need [Go](https://go.dev/dl/) 1.25 or newer, and a disk image of the game (an `.adf` file, or a
zip file that has one in it). On Linux you also need the usual Ebitengine libraries
(`libgl1-mesa-dev libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev`
on Debian and Ubuntu).

```
go run ./cmd/extract "Persian Gulf Inferno.adf"     # reads the disk into assets-local/
go run ./cmd/game                                   # plays
```

`extract` writes every file from the disk to `assets-local/adf/` and the tables it needs from the
game's program to `assets-local/extracted/`. It recognises the disk the engine was made for (a
checksum of the program) and says so if yours is different. The `assets-local` directory is for your
own use; do not share it.

## Playing

| Key | |
|---|---|
| Arrows or WASD | walk, climb, duck (down), turn |
| Ctrl | shoot |
| Space | lay a charge on a door |
| F1 to F3 | weapons |
| P | pause |
| Esc | menu (minimap, save, load, restart) |
| F9 / F10 | save and load a state |
| F4 / F5 / F6 | cheats: everything, invulnerable, double speed |
| R | start again |

By default the game is a plain copy of the original. Everything that is not in the original (the
cheats, WASD, the minimap, the extra indicators, the quality-of-life changes in
`docs/improvements.md`) is behind one switch that is off by default: `go run ./cmd/game -improvements`
turns it all on (also in the Esc menu while playing). `-mute`, `-scale N`, `-title=false`, `-load N`,
`-ticks N -shot file.png` are also there; `-h` lists them.

## What is here

```
cmd/game        the game
cmd/extract     reads the disk image: files and tables
cmd/viewer      browse pictures, sprites and the level
cmd/tiles       print the level's tile ids
cmd/solve       a bot that plays the whole game, and records it for the playthrough test
cmd/packtool    make, check and compare game packs (see below)
cmd/scriptasm   assemble scripts, or print the original's (for those who have the disk)
pkg/amiga       file formats: ADF disks, IFF pictures, sprite banks, samples, fonts, hunks
pkg/content     the one place the game's files are read from
pkg/game        the simulation: pure Go, deterministic, no graphics or sound
pkg/render      drawing, pkg/audio the mixer and music, pkg/title the title sequence
pkg/pack        game packs: your own art, map, sounds, music and scripts
docs/           how it works: file formats, the script engine, the rooms, enemies and so on
tools/          Python tools used to study the game, and to test against an emulator
```

The game logic was rewritten from descriptions of how the original behaves (`docs/re/`), not
translated from its code. `go test ./...` runs the unit tests, and, if you have extracted a disk,
a recorded playthrough of the whole game (72,642 ticks, all 28 rooms) that must end in a win with
the same clock every time. The documentation is mostly in Swedish.

## Game packs

The engine can run with your own material in place of the original's. A **pack** is a directory
with PNG pictures, a [Tiled](https://www.mapeditor.org/) map, WAV sounds and music, and scripts in a
small language with plain words. Whatever a pack leaves out is taken from the original, and a pack
that brings everything runs with no original files at all:

```
go run ./cmd/packtool new mygame      # a starter pack
go run ./cmd/packtool check mygame
go run ./cmd/game -pack mygame
```

See `docs/packs.md` for the pack format and `docs/script-friendly.md` for the script language.
`packtool leakcheck` compares a pack with the original and reports copies, so that a pack of your own
stays your own.

## Making a release

`python scripts/dist.py` builds a zip of the source and one with the programs for the system it runs
on. Neither contains any game data.

## Licence

No licence has been chosen yet. The original game is not covered by anything in this repository.
Third-party libraries and their licences are listed in `NOTICE`.
