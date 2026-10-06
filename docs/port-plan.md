# Plan: port to Go (Windows first)

Goal: a faithful port of the original game to Go, running on Windows and using the original's data files unchanged during development. The game logic is recreated from the disassembly of `ns`. No emulation of the 68000 code.

Background: [adf-inventory.md](adf-inventory.md).

## Principles

- **The original files are read directly.** The game loads the files extracted from the disk in their original format (ILBM, JBOB, RoomData, sound). No conversion up front, so there is a single source of truth. The level matrix and other tables that live in `ns` are extracted by a tool into a data file of its own (`assets-local/extracted/`), so the game never has to read the binary.
- **One asset loader with a replaceable source.** All reading goes through a `content` package that points at a directory. Switching to own assets is a configuration change.
- **Game state is plain data.** The simulation is deterministic, with a fixed time step of **50 Hz** (PAL, the same rate as the original's VBlank loop). Rendering and sound read the state but never change it. That makes network play and replays possible later.
- **Behaviour is documented before implementation.** Every routine we map in `ns` is described in `docs/re/` (what it does, variables, constants). The Go code is written from the description, not by translating assembler line by line. That gives readable code and keeps "understanding" and "building" apart.
- **The reference is the original running in an emulator.** WinUAE with the same ADF is used to compare behaviour (speeds, timer, damage) and screenshots.

## Technology

- **Go + Ebitengine** (`github.com/hajimehoshi/ebiten/v2`). It provides a window, scaling, input (keyboard and gamepad) and sound on Windows without cgo, and also works on macOS, Linux and the web (WASM) if that becomes relevant later.
- Internal resolution **320 x 256** (PAL screen: 200 lines of game view + HUD), scaled by integer factors to the window.
- Images are kept as palette indices (8 bits) in memory and coloured at draw time. This is needed for colour cycling (`CRNG`) and palette switches, which the original uses.
- Sound: the original's 8-bit samples are played through a small mixer of our own with 4 channels (like Paula) on top of Ebitengine's audio stream.

## Package structure

```
cmd/
  game/          the game
  viewer/        tool: browse pictures, sprites, blocks and the map
  extract/       tool: read tables out of ns into assets-local/extracted/
pkg/
  amiga/         file formats: ilbm, jbob, sample, font, hunk
  content/       asset loader (source directory via config), cache
  game/          game state and simulation (plain Go, no Ebitengine dependencies)
    world/       map, blocks, tiles, collision
    actor/       player, enemies, hostages
    item/        bombs, cards, explosive charges, weapons
    flow/        game states: intro, play, death, win, game over
  input/         abstraction: Action flags per tick (keyboard, controller, replay)
  render/        draws the state: map, sprites, HUD, screens, colour cycling
  audio/         mixer, effects, music
tools/           the Python tools for reverse engineering (already exist)
docs/re/         descriptions of mapped routines
```

`pkg/game` must not import `render`, `audio` or Ebitengine. That is what keeps the state clean and testable.

## Phases

Each phase ends with something that can be run and shown.

### Phase 0: foundation (small)

- `git init`, a Go module, Ebitengine, an empty window with a fixed time step.
- Add `assets-local/extracted/` and any `cmd/*/` binaries to `.gitignore`.

**Done when:** an empty window opens and the loop ticks 50 times per second.

### Phase 1: file formats in Go (small to medium)

- Port the verified Python parsers to `pkg/amiga`: ILBM (including `CRNG`), JBOB, the sound format, the 8x8 font.
- `cmd/extract`: read the hunk file, pick out the level matrix (`$A333`, 29 x 208), the block constants and more tables as we go.
- `cmd/viewer`: show full-screen pictures (with colour cycling), the sprite banks with indices, blocks, and the whole map with scrolling.
- Unit tests for each format (sizes, known values, for example that all 649 JBOB records validate).

**Done when:** the viewer shows the whole level with the right colours and can scroll in it.

**Status (2026-10-05): done.** `pkg/amiga` (IFF, ILBM with CRNG, JBOB with the tile plane mask, sound, font, hunk), `pkg/content` (loader with a configurable root), `pkg/game/world` (level), `cmd/extract` (level matrix, 7 palettes, start view) and `cmd/viewer` (pictures with colour cycling, sprite banks, the whole level). The tile colours are verified against the emulator's screen (98.5 % of the pixels; the rest is sprites). Bobs are still drawn with the unverified interpretation.

### Phase 2: mapping the core of the game (large, runs in parallel with phases 3 to 6)

The most uncertain work. Done piece by piece as the functions are needed.

- Improve `tools/recdis.py`: a symbol file (`address -> name, comment`) that the disassembler reads in, so the listing becomes more readable the more we understand. Follow jump tables to raise coverage above 64 %.
- Map in roughly this order:
  1. Main loop, VBlank rate, game state (`$21EA`).
  2. Reading the joystick and keyboard (`$BFE001`, `$DFF00C`).
  3. The player's movement, animation tables (probably `NSIA`), collision against tiles.
  4. Doors, lifts, stairs, cards.
  5. Enemies: placement, behaviour, shots, hits.
  6. Bombs, hostages, timer, explosive charges, the bomb defusing screen (`BB`).
  7. Palettes, colour cycling, sound and music playback.
- Each area is documented in `docs/re/<area>.md` before it is built.

**Status (2026-10-05): in progress.** The script engine is mapped in large part and ported to Go (`pkg/game/script`). It runs against the original's bytecode and matches a recorded trace step by step (walking right + stopping at a wall). See `docs/re/script-engine.md`. The insight that changes the plan: **behaviour lives largely in scripts (data)**, so the port runs the original scripts instead of rewriting every movement.

### Phase 3: the player in the world (medium)

- An input abstraction (`input.Actions` per tick) with keyboard and gamepad.
- Camera and scrolling as in the original (start position from `$8F56`/`$8F58`).
- The player's movement, animations and collision according to phase 2.
- HUD (`NSIMenu`): weapons, magazines, floor, hits, clock.

**Done when:** you can walk around the building and the HUD updates.

### Phase 4: interaction (medium)

- Doors and cards, lifts and stairs, changing floors.
- Items: picking up magazines, explosive charges, cards.

### Phase 5: enemies and combat (medium to large)

- Enemy placement, movement and AI.
- Shots, hits, death (the `DO` screen).

### Phase 6: the objectives (medium)

- Bombs and the countdown timer, hostages.
- The bomb defusing sequence (`BB`).
- Win (`WT`), explosion (`ET`), game over (`EX`).

### Phase 7: flow and presentation (small to medium)

- Title (`NSILoader`), "press fire" (`PF`), intro (`IT`, `EN`, `HC`, `ST`), end screens.
- Colour cycling, transitions.

### Phase 8: sound and music (medium)

- Effects from `NSISound`, `DAS`, `DBY`, `IAZ` (the sample rate and the split are confirmed in phase 2).
- Music from `NSIMusicSound`. The format is unknown. It may be a sequencer of its own, and in that case it is a larger task.

### Phase 9: fidelity and polish (medium)

- Compare against WinUAE: speeds, timer, difficulty, screenshots.
- Replay test: record input sequences and verify that the simulation gives the same state every time.
- Settings: window size, keys, full screen.

### Phase 10: own assets (later, a project of its own)

- Replace graphics, sound, texts and level with own material (see the content pack).
- The port contains no original assets and no data read out of `ns`; they are read from the user's own disk.

## Tests

- `pkg/amiga`: format tests against the real files. Skip the test if `assets-local/` is missing, so that the repository works without the original files.
- `pkg/game`: unit tests for collision, timer, bomb logic and damage. Use small synthetic maps, not original data.
- Determinism: the same input sequence must give the same state hash after N ticks.

## Risks

| Risk | Impact | Action |
|---|---|---|
| The game logic is harder to map than expected (hand-written assembler, no symbols) | Phase 2 takes longer | A symbol file and iterative mapping; compare against the emulator instead of understanding everything in detail |
| The music format is a sequencer of its own | Phase 8 grows | Do the music last; the game works without it |
| The missing file `HK` is referred to in the code | A screen may be missing | Check in phase 7 when it is loaded; possibly another disk version |
| Behaviour that depends on Amiga timing (counter loops, blitter waits) | Wrong pace | Tie everything to 50 Hz ticks; measure against the emulator |

## Decisions (2026-10-05)

- The port is an identical Go port. The engine can later be used for own variants.
- Ebitengine is used.
- The port should be **identical** to the original as far as possible. Deviations are documented.
- An emulator (WinUAE) is the reference for the tests.
- The mechanics in `pkg/game` should be reusable in an own variant, so they are kept free of platform dependencies.
