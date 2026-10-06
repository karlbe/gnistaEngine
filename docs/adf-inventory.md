# Inventory of the original disk (ADF)

Step 1 in studying how to make a clean port from the original binaries. The source is `assets-local/raw/*.adf`, unpacked with `go run ./cmd/extract` into `assets-local/adf/`. The material is the original's, so what is read from it is kept locally in `assets-local/`.

Status per file: **Verified** means the format has been interpreted with a header or chunk structure. **Hypothesis** means it is a guess from byte patterns that has not been checked against code.

## The disk

- Volume name `PGI`, OFS (boot block `DOS\0`, flag 0), 880 KB, 901,120 bytes.
- The only startup step is `s/startup-sequence`, which contains just `ns`. Everything is therefore started from a single binary.
- The disk has 33 files, about 565 KB of data. Levels and graphics are divided into separate files that are loaded from disk, and the code is in `ns`.

## Summary

| Category | Files | Format |
|---|---|---|
| Executable | `ns` | AmigaOS hunk binary, 68000 |
| Full-screen pictures | 11, with two-letter names, `NSILoader`, `NSIMenu`, 10 `NSIRoomN` | IFF ILBM, 320x200, 5 bitplanes (32 colours) |
| Sprite banks | `NSIBobs`, `NSIIcons` | IFF-like `FORM JBOB` (its own format) |
| Sound | `DAS`, `DBY`, `IAZ`, `NSISound`, `NSIMusicSound` | Raw 8-bit sound with a 6-byte header (hypothesis) |
| Game data | `RoomData`, `NSIA` | Unknown own format |
| Font | `NSIAscii` | 256 characters x 8 bytes, 1 bitplane (hypothesis, strong) |
| System | `devs/system-configuration` | AmigaOS Preferences struct (232 bytes) |

## The executable

### `ns` (55,980 bytes): verified

- A hunk file (`0x3F3`) with 2 hunks:
  - Hunk 0: CODE 47,816 bytes, with 2,024 RELOC32 entries.
  - Hunk 1: BSS 4 bytes.
- No symbol table and no debug info. A disassembly is therefore anonymous, without function names.
- No readable strings in the code. File names (`NSIRoom1` and so on) are probably built dynamically (for example `NSIRoom` + a number), so they do not show up as whole strings. The file names in the disk's root are probably the only "API" the code uses towards the data.
- It is small (47 KB of code) for a whole game, so it is manageable to disassemble and port.
- The game does not mention AmigaDOS libraries in clear text. That suggests that it writes directly to the hardware (blitter, copper, Paula) and uses its own track loader. This has to be confirmed by disassembly. It affects the port strongly, because the hardware-related code has to be replaced.

## Full-screen pictures, IFF ILBM: verified

All are 320x200, 5 bitplanes, mask=2 (transparent colour), compression 1 (ByteRun1), 32 colours in CMAP. They also contain the chunks `DPPV` (Deluxe Paint perspective, 104 bytes) and 4 `CRNG` (colour cycling). The colour cycling is thus in the data and should be possible to recreate in the port.

| File | Size | Transparent colour | Interpretation (hypothesis) |
|---|---|---|---|
| `NSILoader` | 25,910 | 0 | Loading/title picture |
| `NSIMenu` | 6,736 | 0 | Menu picture |
| `NSIRoom1` | 14,848 | 0 | Background/room 1 |
| `NSIRoom5` | 10,594 | 0 | Room 5 |
| `NSIRoom9` | 12,932 | 13 | Room 9 |
| `NSIRoom10` | 12,932 | 0 | Room 10 |
| `NSIRoom11` | 13,264 | 13 | Room 11 |
| `NSIRoom12` | 12,924 | 0 | Room 12 |
| `NSIRoom13` | 11,986 | 0 | Room 13 |
| `NSIRoom14` | 14,946 | 0 | Room 14 |
| `NSIRoom15` | 11,026 | 0 | Room 15 |
| `BB` | 16,024 | 0 | Full-screen picture |
| `DO` | 4,280 | 2 | Full-screen picture |
| `EN` | 19,762 | 8 | Full-screen picture |
| `ET` | 4,560 | 0 | Full-screen picture |
| `EX` | 27,904 | 0 | Full-screen picture |
| `HC` | 12,876 | 0 | Full-screen picture |
| `IT` | 5,618 | 0 | Full-screen picture |
| `PF` | 2,884 | 0 | Full-screen picture |
| `ST` | 6,858 | 0 | Full-screen picture |
| `TA` | 8,338 | 0 | Full-screen picture |
| `WT` | 7,784 | 0 | Full-screen picture |

Remarks:

- The rooms that exist are 1, 5 and 9-15. Rooms 2-4 and 6-8 are missing on this disk. Either they are generated (see `RoomData`), or they are on another disk, or pictures are shared between rooms. `NSIRoom9` and `NSIRoom10` have the same size (12,932 bytes) but different `trans`, which suggests a close relationship.
- The two-letter files (`BB`, `DO`, `EN`, `ET`, `EX`, `HC`, `IT`, `PF`, `ST`, `TA`, `WT`) have an unclear role. They have the same ILBM layout as the rooms. Whether they are fixed screens, room backgrounds or something else is an open question. Rendering them to PNG settles it quickly.

## Sprite banks, `FORM JBOB`: partly verified

- `NSIBobs` (109,428 bytes) and `NSIIcons` (40,204 bytes).
- An IFF-like container with the chunks `BHDR` and `BODY`. It is a format of its own, probably from an Amiga sprite/BOB tool (Blitter Objects).
- `BHDR` seems to be a table of 24-byte records, with a 4-character name/tag (`OTRL`, `WTRL`, `CTRL`). That gives about 649 records in `NSIBobs` (15,576 / 24) and 362 in `NSIIcons` (8,688 / 24). The tags also show up in strings such as `Adr.Mask`.
- The first record points at offset 0, and the next record starts at 0x80. That fits 4 bitplanes of 32 bytes each (16 rows x 16 pixels). **Hypothesis:** 16x16 sprites with planar data and a mask plane. This has to be confirmed when a converter is written.
- This is the game's main graphics, that is, the player, enemies, bombs, hostages and tiles.

## Sound: hypothesis

All five files begin with the same pattern: a 32-bit length (big endian), a 16-bit word, then data that looks like 8-bit PCM (signed, Paula style).

| File | File size | Length field | Word after the length | Note |
|---|---|---|---|---|
| `DAS` | 4,006 | 4,000 | `0x3E1C` | Short effect |
| `DBY` | 2,006 | 2,000 | `0x396C` | Short effect |
| `IAZ` | 5,006 | 5,000 | `0x3DB8` | Short effect |
| `NSISound` | 82,806 | 82,800 | `0x2710` | Probably a bank of several effects |
| `NSIMusicSound` | 140,258 | 140,252 | `0x2710` | Probably music or instrument samples |

- The length field + 6 = the file size in all cases. That is verified.
- The word (`0x2710` = 10,000, the others around 14,700-15,900) is probably a sample rate or a period value. That is not confirmed.
- `NSISound` and `NSIMusicSound` begin with almost identical bytes, but that may be a coincidence. If they are banks with several sounds, an index table is needed, which is probably in `ns`.

## Game data

### `RoomData` (20,480 bytes): unknown

- Exactly 20 KB = 20 x 1,024. Begins with many repeated `0x73`, which is probably an "empty" tile/value.
- Low entropy (4.45) and 31 % zeros. That fits tile maps or a level layout.
- It is the most likely source of the level layout, object placements (bombs, hostages) and room transitions. The whole file has to be interpreted, and it is the core of a port.

### `NSIA` (10,072 bytes): unknown

- Begins with `02 05 00 7f 40 04 00 04 01 e0 10 40 ...` followed by repeated records such as `00 00 01 e0 08 36`. The pattern of fixed 6 bytes suggests a list of records (for example animation or object definitions).
- Low entropy (3.42). The role is unclear. It could be an animation definition, an object definition or a table for `NSIBobs`.

### `NSIAscii` (2,048 bytes): hypothesis, strong

- Exactly 256 x 8 bytes. The first glyph (`ff 81 bd a5 a5 bd 81 ff`) is a box with a pattern, that is, an 8x8 bitmap font. Probably a character table that also contains graphic characters.

## System

### `devs/system-configuration` (232 bytes): verified

- The size 232 bytes is the AmigaOS `struct Preferences`. It contains, among other things, the string `generic` (a printer name) and settings for the keyboard and screen. It has no game function and does not need to be ported.

### `s/startup-sequence` (3 bytes)

- Contains `ns`. The only thing that starts the game.

## Conclusions for portability

1. **Favourable:** only one binary (`ns`, 47 KB of code). Everything else is data. The graphics are in a standard format (IFF ILBM) that is easy to convert, and the colour cycling is in the data.
2. **Medium difficulty:** the sprite banks (`JBOB`) and the sounds are formats of their own. Both look simple, though, with planar data and raw PCM.
3. **The biggest work:** `RoomData` and `NSIA` and all the game logic in `ns`. The level format and the object data have to be interpreted, and the logic needs a disassembly. There are no symbols, so it goes by tracing data flow towards the file names and sprite indices.
4. **Risk:** if `ns` writes directly to the Amiga hardware (likely) all drawing, sound and input has to be replaced. That is fine for a rewrite where we only recreate the logic, but it is not an emulation of the binary.
5. **An alternative to porting:** emulate the whole binary in the browser (a WebAssembly-based Amiga emulator with the ADF). It is considerably easier technically but gives no "clean port" and does not solve the rights question. It does not meet the requirement of code of our own.

## Open questions / next steps

- Convert all ILBM files to PNG and see what the two-letter files are.
- Disassemble `ns` (for example with Ghidra, which has 68000 support and can read Amiga hunks) and map the functions that read in the data files.
- Decode `JBOB` fully and dump sprite sheets to PNG.
- Decode `RoomData` and `NSIA`.
- Decide whether `NSISound` and `NSIMusicSound` are banks and how they are indexed.
- The limit of "clean port": a rewrite that recreates the logic, not running the original binary.

## Addendum: what the pictures show (converted with `tools/ilbm2png.py`)

The conversion was written to `assets-local/png/` (gitignored). The two-letter files are **story and game-over screens**, not room backgrounds:

| File | Contents |
|---|---|
| `NSILoader` | Title picture with main characters and a logo |
| `PF` | "Press fire to start" |
| `IT` | Intro: the premise, a terror threat against the world's largest oil reserve |
| `EN` | Intro: a transport plane and a troop lined up |
| `HC` | Intro: a helicopter in silhouette |
| `ST` | Story after the crash, arrival at the rig |
| `WT` | Win screen with an epilogue |
| `DO` | Death screen (text) |
| `ET` | End screen: explosion (text) |
| `EX` | Game over: mushroom cloud |
| `BB` | Bomb defusing screen (a diagram of the bomb with wires, plus pliers) |
| `TA` | Unknown graphics, a colourful silhouette. Probably a picture that is based on colour cycling or transparency |
| `NSIMenu` | The game's HUD (weapons, magazines, explosive charges, cards, floor, hits, clock) |
| `NSIRoomN` | Room pictures in the game view, with a frame at the bottom for the HUD. The game is therefore a **side view per room**, not isometric |

## Addendum: sprite and map formats (verified with `tools/jbob2png.py` and `tools/levelmap2png.py`)

**`JBOB` (NSIBobs, NSIIcons).** `BHDR` is a table of 24-byte records, big endian: `u32` body offset, `u32` mask plane offset, `u32` plane size, `u16` flags, `u16` byte width+2, `u16` height, `u16` width in words, a 4-character tag. The colour planes are in sequence in `BODY` and the mask plane last. The number of colour planes is `(mask offset - offset) / plane size`. The format is confirmed because all 649 records in `NSIBobs` pass all the consistency checks.

- `NSIBobs`: 649 sprites, mostly 16x16 with 3 planes (574), plus 27 of 32x38. It holds the figures' animation pictures in many poses (walking, lying, shooting and so on), and lifts and doors.
- `NSIIcons`: 240 valid records, mostly 16x16 with 3-4 planes. It is the map's **tile set** (walls, floors, stairs, pipes, doors, furniture, objects). Records 240-361 are identical empty records (`Adr.Mask`, tag `CTRL`) and should be ignored.
- The tags `OTRL`, `WTRL`, `CTRL` and `_TRL` are unknown. They may indicate the object type.
- The colour depth (3 planes = 8 colours) means that the sprite banks share a palette with the game view, and that the palette is not in the file. It has to be taken from the room picture or the code.

**`RoomData`.** *Correction:* the first interpretation (a tile map of 80 x 256) was wrong. The file is **256 blocks of 20 x 4 tiles** (80 bytes per block, row by row, 1 byte per tile, an index into `NSIIcons`), that is, 320 x 64 pixels per block. The order of the blocks in the level is not decided by the file but by a separate map matrix in `ns` (see the next section).

## Addendum: the first disassembly of `ns` (`tools/disasm_hunk.py`, Capstone 5, 68000)

The output is written to `assets-local/ns.asm` (gitignored, derived from the original).

- 47,816 bytes of code gave 13,036 lines, of which 1,284 could not be decoded. 3,380 `ori.b` are almost certainly zero-filled **data embedded in the code hunk**, which linear disassembly interprets as code. The hunk mixes code and data, so an ordinary linear disassembly is not enough. It needs recursive traversal from known entry points.
- **Hardware directly.** The code writes to the custom chips (`$DFF096` DMACON 22 times, `$DFF09A/9C` interrupts, the sound channels `$DFF0A0-$DFF0D8`, copper and bitplane pointers) and to the CIA (`$BFE001` joystick/fire button, `$BFD100` disk drive control). It is thus a **track loader with its own hardware code**. Drawing, sound and input have to be rewritten from scratch, and the OS calls are few.
- The entry point is a main loop at address 0 that calls seven initialisation routines and then switches on a state word at `$21EA` (1, 2, 3 = different modes, probably intro, game and end). The game state is in absolutely addressed memory data (`$21EA`, `$7234`, `$7C5C`, `$9A88`), and `ns` has 2,024 relocations that point at them.
- The wait loops are counter loops (`subi.l #1,d0 / bne`), so they depend on **CPU time** and should be replaced with real timers.

**How well can it be decompiled?** The disassembly is clean and readable. The game logic appears to be hand-written assembler without compiler-generated structure, so a C-like decompilation from Ghidra will be helpful but shallow. It is often good for control flow and data access, but variable names and types we have to set ourselves. With 47 KB of code it is manageable. Recommendation: recursive disassembly in Ghidra (needs JDK 17+), then manual mapping of the state machine, the input handling and the map reading.

## Addendum: a recursive disassembler in Python (`tools/recdis.py`), no Ghidra needed

The script follows jumps and calls from address 0 and uses the 2,024 relocations as extra entry points. The result is written to `assets-local/ns_rec.asm` and covers 64 % of the code (the rest is data or code that has not been analysed). Code and data are now separated. 17 indirect `jsr -n(a6)` are OS library calls that the script does not follow.

Findings:

- **File names exist as clear text** in the data part, with a `df0:` prefix: `NSIascii`, `RoomData`, `NSISound`, `IAZ`, `DAS`, `DBY`, `NSIBobs`, `NSIIcons`, `NSIMenu`, `NSIRoom1/5/9-15`, `BB`, `HK`, `NSILoader`, `ST`, `WT`, `DO`, `ET`, `EX`, `EN`, `HC`, `IT`, `TA`, `PF`, `NSIA`, `NSIMusicSound`. All are on the disk **except `HK`**, so that file is referred to but missing here. Rooms 2-4 and 6-8 are never referred to, so the game only has the 11 room backgrounds.
- **Libraries:** `graphics.library`, `dos.library` and `intuition.library`. The files are read through DOS (`jsr -$1e/-$24/-$2a(a6)` = Open/Close/Read). Drawing and sound still go directly to the hardware.
- **The game's outcome** is in the word `$21EA`. It is 0 while the game is on. 1, 2 and 3 load `DO` (death), `ET` (end: explosion) and `WT` (win) respectively. That corresponds to the screens I saw in the pictures.
- The code's data structures are in absolutely addressed data, so the port builds a game state of its own instead of recreating the memory layout.

## Addendum: the map matrix in the code (verified, `tools/levelmap2png.py`)

The level is **not** in any data file. It is a static byte matrix built into the code hunk of `ns`, and `RoomData` is only a library of blocks that the matrix points out. The addresses below are hunk-relative (file offset = address + 36).

**The matrix**

| Property | Value |
|---|---|
| Address | `$A333` (file offset 41,815) |
| Format | 1 byte per cell, row by row. The cell is a **block id** (0-255) |
| Row length (stride) | **29** cells. The value is in the word `$910E` |
| Number of rows | **26**. The value is in the word `$9110`. *Correction:* 208 rows (to the end of the hunk) was assumed at first, but after row 26 (`$A5A5`) other tables follow |
| After the matrix | From `$A5A5` a table with the same width and almost only 0/1. **Hypothesis:** a flag per block (for example lit/visited). Then more tables with another structure |
| Id 0 | An empty block, drawn as background |

**Blocks.** An id `n` points at `RoomData[n * 80 .. n * 80 + 79]`. That is 20 x 4 tiles, row by row (20 bytes per row, 4 rows), and each tile is an index into `NSIIcons` (16x16 pixels). A block is therefore 320 x 64 pixels. It is one screen width of one floor, which fits the floors being 64 pixels apart in the game. The constants are in data: `$9112`=20 (width), `$9114`=4 (height), `$9116`=80 (bytes per block).

**The whole level** is 29 x 26 blocks, that is, 9,280 x 1,664 pixels. In cross-section it consists of three buildings of different heights joined by a ground level, with floors, stairs, lifts and rooms. Show it with `go run ./cmd/viewer` (zoom out with the mouse wheel).

**How the code draws** (the routine `$8DE0`, with the helper routine `$8E50`):

1. The map position is converted from pixels to blocks at `$8EDA`. `$8F56` (x) and `$8F58` (y) are the view's pixel position. It is divided by the block size (320 and 64) and gives block column `$8F72`, block row `$8F74` and a remaining tile offset `$8F76`.
2. `$8DE0` calculates the address in the matrix: `$A333 + $8F72 + $8F74 * $910E`.
3. It reads the block ids for a window of 3 x 4 blocks (`$94D8`=3, `$94DA`=4) and calls `$8E50` for each.
4. `$8E50` copies the block's 4 rows of 20 tiles from `RoomData` (the pointer `$94E6`, 20,480 bytes) to a **tile buffer** that is 60 tiles wide (`$94DC`=60, that is, 3 blocks) and 16 tiles high. The buffer is used for scrolling. The screen shows 20 tiles at a time.
5. The tile buffer is then drawn to the screen with the blitter (see the routine at `$8D9C`, which writes to `$DFF040-$DFF058`).

**Start position.** At `$03A0`, `$8F56`=7984 and `$8F58`=1360 are set. That corresponds to block column 24 and block row 21 (7984 / 320 = 24.95 and 1360 / 64 = 21.25). It is probably the player's starting view, but it has not yet been checked against the screenshot.

**Still unknown**
- What the word `$9110` (=26) means. It is next to the stride value and could be the visible width or a map limit.
- Which ids are only decoration and which are lifts, doors or stairs. Collision and interaction probably need a table of their own, because blocks only contain tile ids.
- Where bombs, hostages and enemies are placed. They are not in the matrix, and `NSIA` is one of the possible places.
- The palette. The screenshot has a 16-colour gameplay palette (plus 2 extra) where the order differs from the room pictures' CMAP. I use the screenshot's palette for the tiles, and the colours in the map look reasonable. Where the game itself sets the palette has not been mapped.

The room pictures are side views with a background and a character, in a game with rooms, doors and lifts. The game is therefore a side view, not isometric.

## Addendum: screen, palette and how tiles are drawn (verified against the emulator)

Verified by reading the copper list and the screen's bitplanes from chip RAM in WinUAE (`tools/uae/uaectl.py`), starting from a savestate in game mode.

**The screen in game mode** (the copper list is at `$FB6C` in chip RAM in that run):

- `BPLCON0=$4000`: **4 bitplanes**, one playfield, 16 colours. The copper list sets 6 plane pointers, but only 4 are used.
- The planes are 46 bytes wide per row (`DDFSTRT=$30`, `DDFSTOP=$D0` give 42 bytes, plus a modulo of 4), that is, 368 pixels. The extra width is used for soft scrolling (`BPLCON1`).
- The game view has a palette with 16 colours (the other 16 registers are 0). On line 172 the copper list changes the plane pointers and the palette for the HUD.

**Palettes.** The game has 7 palette tables of 32 colours (12-bit `$0RGB`) in the code hunk: `$7256`, `$72D6`, `$7316`, `$7356`, `$7396`, `$73D6`, `$7416`. The active one is set with `move.l #table,$7234.l`, and the routine `$723C` builds the copper list's `COLOR00-31`. The game mode uses `$7256` (set at `$6950`). The order is the same as in the room pictures' CMAP. WinUAE's own PNG screenshots reorder the palette, so they cannot be used as a source for indices.

**Tiles are drawn with a plane mask** (the routine `$8D66`, called per tile from `$8D32`):

1. Tile id x 24 gives the tile's `BHDR` record in `NSIIcons`. The tile id is thus directly a record number.
2. The data pointer is the record's body offset plus the `BODY` base.
3. `d7` = **the record's byte 20, that is, the first character of the "tag"**. It is a plane mask, not a name.
4. For screen planes 0-3: if bit *p* in the mask is set, the tile's next stored 32-byte plane (16 rows x 1 word) is copied to screen plane *p*. Otherwise the plane is cleared.
5. The blitter writes the whole square without a mask, so map tiles are **opaque**.

The tags that occur: `'W'`=`$57` (planes 0-2), `'_'`=`$5F` (planes 0-3), `'O'`=`$4F` (planes 0-3), `'C'`=`$43` (planes 0-1), `'D'`=`$44` (plane 2 only), `'@'`=`$40` (no planes, that is, colour 0). The same stored data can thus give different colours depending on the mask. That explains why the same motif looks grey in corridors and blue in rooms.

With this model 98.5 % of the pixels in the game view agree with the emulator's screen. The rest is sprites (the figure and so on), which are drawn on top. The game view's left edge is 16 pixels to the right of the view position in `$8F56` (the extra scroll column).

**Hypothesis:** the sprite bank's first tag byte (for example `'O'` in `NSIBobs`) probably works the same way for figures, but they are drawn with another routine and mask. That has not been verified. `pkg/amiga` therefore decodes bobs with the old interpretation (colour planes + mask plane), and tiles through `Bank.Tile`.
