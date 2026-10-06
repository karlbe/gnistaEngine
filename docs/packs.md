# Content packs: your own level, graphics and sounds

This file describes how to make your own version of the game with a **content pack**: a directory of your own files in open formats (PNG, JSON, WAV, Tiled) that replaces the original's data piece by piece. Whatever the pack lacks is taken from the original, so a pack can start small (a few screens) and grow until nothing of the original remains.

Status, and what remains for a fully publishable game, is at the bottom ("What is not free yet").

## Quick start

```
go run ./cmd/packtool new mygame      # creates a starter pack (own palette, placeholder tiles, a small level)
go run ./cmd/packtool check mygame    # reads the pack the way the game does and reports errors
go run ./cmd/game -pack mygame        # runs the game with the pack
```

`packtool new` creates a starter pack. **Never put original graphics, or anything traced or painted over the original, in a pack.**

`go run ./cmd/packtool inventory` lists what the original has (sizes, counts) and what a pack can replace. It needs `assets-local/` and only prints dimensions and counts.

## What a pack contains

Everything is optional. Whatever is missing is taken from the original.

| File | What |
|---|---|
| `pack.json` | name, palette, texts, volumes (see below) |
| `tiles.png` | the map's tiles: a grid of 16x16 cells, tile id = index row by row (at most 256) |
| `tiles_fg.png` | the same grid; pixels drawn here end up **in front of** the figures (railings, floor edges) |
| `sprites/N.png` | figure image number N, any size, transparent where the figure is not |
| `sprites/map.json` | `{"335": {"same": 333, "flip": true}}`: image 335 is image 333 mirrored |
| `screens/NAME.png` | full-screen pictures: `NSILoader` (logo), `IT`, `TA`, `PF`, `ST` (title and intro), `ET`, `EX`, `WT`, `HC`, `EN` (ending), `hud` (the panel) |
| `rooms/N.png` | the pictures behind doors, picture number N (see "Rooms") |
| `font.png` | 256x64: 128 characters of 16x8, 16 per row. Pixels that are drawn and not black count |
| `level.tmj` | the level, made in Tiled (see "The level") |
| `scripts/*.gs`, `program.json` | own scripts and their entry points; without them the original's are run |
| `sounds/N.wav` | sound effect N, mono or stereo, 8 or 16 bit, any sample rate |
| `music/title.wav`, `music/game.wav` | recorded music: the title plays once, the game music loops |

`pack.json`:

```json
{
  "name": "Mygame",
  "palette": ["#0b1020", "..."],        // the game view's colours, up to 256 (tiles, sprites, room pictures)
  "flash": ["#...", "..."],              // the explosion's flash, 32 colours
  "title_palettes": {"7316": ["..."]},   // colour switches for the silhouette picture (TA), otherwise the picture's own palette
  "credits": ["LINE 1", "LINE 2"],       // the text that rolls over the title
  "messages": ["", "TEXT\nNEW LINE"],    // texts in rooms, index = message number
  "clock": {"top": [..10..], "bottom": [..10..]},  // character codes for the clock's digits
  "sound_volumes": {"3": 40},            // volume 0-64 per effect (default 64)
  "music_volume": 0.6,                   // 0-1
  "empty_tile": 115                      // tile for empty cells in the map (default 0x73)
}
```

### Colours

- If the pack has a `palette`, every pixel in `tiles.png` and `sprites/` is matched to the nearest colour. If it has none, the original's colours are used.
- The original's tiles and sprites that the pack has not replaced are recoloured to the nearest colour in the pack's palette, so mixed packs look coherent.
- Full-screen pictures and room pictures have a palette of their own: an indexed PNG keeps its indices, an ordinary PNG gets a palette made from its colours. If the picture has more than 256 colours they are reduced automatically (median cut). Save as an indexed PNG if you want to control the colours.
- The panel (`hud`) only shows indices 0-15. Indices 13-15 are the frames of the weapon boxes (highlighted on a switch) and 4-9 the weapon icons, which change colour when a weapon is picked up. Keep those roles.
- The title's rolling text is drawn as "odd indices" on top of the silhouette picture (`TA`): the picture may only use even colour indices in the text area at the bottom.

### The level (`level.tmj`)

Make the level in [Tiled](https://www.mapeditor.org/) with `tiles.png` as the tileset (16x16, firstgid 1). Save as JSON (`.tmj`) and set the tile layer format to CSV. `packtool new` creates a ready-made map to start from.

- **Tile layer** `tiles`: one tile per cell. Empty cells (gid 0) become `empty_tile`. The map is padded to whole blocks: **blocks are 20 x 4 tiles** and the map can have at most **256 different blocks** (identical blocks are shared automatically). That is the real limit: repeat floors, walls and stairwells.
- **Object layer** `objects`, with objects of these classes (points or rectangles, the position is taken as the top-left corner):

| Class | Properties | Meaning |
|---|---|---|
| `start` | – | where the player starts. There must be at least 10 tiles to the left and 6 above (the view is 20 x 8 tiles and the player stands 10 in and 6 down) |
| `door` | `lock`, `card`, `weapon`, `mags1`–`mags3`, `charges`, `picture`, `message` | the room behind a door tile (id 0x26–0x28) in the same cell. `lock`: cards that open it ("1,3"), `card`: the card the room gives (1-5), `weapon`: 2 or 3, `mags*`: magazines 0-9, `charges`: explosive charges, `picture`: the number of the room picture, `message`: the number of the text |
| `spawn` | `wave` 0-4 | enemy trigger. 0 = one enemy, 1-4 = one enemy and then 3, 6, 8 or 15 more |
| `wave_zone` | – (rectangle) | blocks in the zone allow enemy waves to appear |

- Property on the map: `lift_cards`, a comma list of card bits per block column ("1,1,1,2,2,2,4,4,4"), the cards needed to call the lift there. Empty = no cards.
- **Cells.** Enemy triggers and doors belong to a *cell*: a region of 20 x 4 tiles counted from the player's position (the view starts 10 tiles to the left of and 6 above the player). **A cell has at most one door.** An enemy trigger fires as soon as the player enters the cell, so do not put a door in the same cell as a trigger if the player is to have time to use the door: the enemies block the up button as long as they are visible.
- **Stopping positions.** The player only stops on **every second tile** counted from the start tile (a walk cycle is two tiles). Doors, lifts and stair feet must therefore stand on tiles with the same parity as the start tile, or they cannot be reached.

### Tile ids: what the game does with each id

The scripts and sensors look at tile ids. A tileset of your own must put the right kind of tile on the right id (the guide `guides/tiles_guide.png`, which `packtool new` writes, shows all the ids colour-coded):

| Id | Meaning |
|---|---|
| 0x00–0x0A | solid: blocks walking (walls, floors you cannot pass through) |
| 0x0B–0x0F, 0x13–0x15 | stairs rising to the right ("/"), seen from the left |
| 0x16–0x1D | stairs rising to the left ("\") |
| 0x1E–0x20 | stair steps downwards (a way down) |
| 0x21–0x25 | ladder |
| 0x26–0x28 | door (one tile, the player stands on it). 0x29 can also take an explosive charge |
| 0x29, 0x2A | lift, one tile per floor, **four rows (64 px) between the floors** |
| other ids | free: background, scenery, floor decoration. Every id ≥ 0x0B that is not listed above can be walked through |

All ids up to 255 can be used. Ids that are not drawn in `tiles.png` are taken from the original. `packtool check` warns about that.

### Figures (sprites)

The game draws each figure as **two parts**, upper body and legs, 16 x 16 px each, on image numbers that the scripts calculate: `n + offset + base × weapon`. That means **the image numbers the original's scripts use decide which pose is shown**. The original has 649 images: 622 of 16 x 16 (player, enemies) and 27 of 32 x 38 (lift cabin, wire marker and so on).

What you do now:

- Draw the poses that are needed and point out all the image numbers that should look the same with `sprites/map.json` (the same image, possibly mirrored). Images that the pack does not have are drawn by the original (recoloured to the pack's palette).
- For a fully own game, write own scripts with own image numbers. Then only the images the scripts use are needed.

The hotspot is the top-left corner, just like in the original. The image's dimensions can be anything.

### Rooms

The picture behind a door is chosen by `picture`. A few numbers have a meaning of their own in the rules and should be kept: **0** = an empty room (shown for rooms already visited), **5** = first aid (refills the hits, is never marked as "done"), **8** = the bomb room (fire leaves, space starts the wire cutting), **9** = the close-up of the bomb's wires. The wire cutting uses four positions in the picture (x ≈ 0x92, 0xA5, 0xC0, 0xD7 from the left) and the fourth wire defuses. The other numbers are free. Room pictures are 320 x 200, but only the top 128 rows are visible in the game view.

### Sound and music

`sounds/N.wav` replaces effect N. The effects are numbered 0–19 in the original (`packtool inventory` lists length and frequency); which one plays when is controlled by the scripts (`docs/re/sound.md`). The game plays them on four channels (0 and 3 left, 1 and 2 right, with soft stereo). The frequency is rounded to a period (3546895 / frequency), so 22050 Hz or 44100 Hz is a good choice.

`music/title.wav` and `music/game.wav` replace the original's played tunes. The title waits for the music to finish; the game music loops, goes quiet when an enemy is visible and then continues. Normalise to about −6 dB, the game multiplies by `music_volume`.

### Texts

The texts are capitals in the character set that `font.png` has (ASCII). Line breaks in the rooms: `\n`.

## Workflow for your own level

1. `packtool new mygame`, open `level.tmj` in Tiled.
2. Draw tiles in `tiles.png` (one cell at a time; cells you leave fully transparent are taken from the original), following the id table above.
3. Build the level, place `start`, `door`, `spawn`, `wave_zone`. Keep track of parity and cells.
4. `packtool check mygame`, then `go run ./cmd/game -pack mygame`. F4 (cheat) shows the whole map on the minimap and F5 makes you invulnerable, which helps testing.
5. Draw sprites and screens, add sound and music. Start with what is most visible: title, panel, tiles.
6. The bomb and the goal: the bomb room is a room with `picture` 8 and a door lock that no card opens (`lock` 32), so the door has to be blown. Defusing the bomb is the win. There is no other win condition in the rules than that and the clock; an own goal requires changed code.

## What is fixed in the game's rules (not in the pack yet)

- The clock counts down and when it reaches zero the bomb goes off. The player has 9 lives and 3 hits per life, and starts with the pistol (7 shots in the magazine, 5 magazines) and 2 explosive charges.
- The enemies: three places at a time, their behaviour, hit points (2) and shots. The spawner uses four templates (first enemy right/left, wave right/left).
- The player's speed, jump, roll and stair rules. All of this lives in the original's scripts.
- There are five cards (lift and door cards), three weapons (F1–F3).

## Without the original

If the pack has a palette, a level and scripts, the game starts without any original files at all: `go run ./cmd/game -assets /does/not/exist -pack mygame`. Whatever the pack lacks is otherwise taken from the original, so if you leave something out (for example the panel or the font) it is not drawn. `packtool leakcheck <pack>` compares a pack with the original and reports copies.

## What is not free yet

A pack gives own graphics, level, sound, music, texts **and own scripts**. A pack with own scripts runs without the original, but then it is the pack's responsibility that everything in it is its own. The script language is described in `docs/script-friendly.md` and `docs/script-reference.md`.

What remains for a publishable game, in order:

1. Own graphics, level, sound, music and texts (the pack, ready to use now).
2. Own scripts and own tables (`own-scripts.md`, steps 2–4).
3. An own title, name and logos; the original's name must not be used.
4. Go through every file to make sure it has an origin and a licence, and remove anything that derives from the original.
