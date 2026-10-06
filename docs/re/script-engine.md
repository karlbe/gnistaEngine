# The script engine (actors, animation, movement)

These are the technical details of the script engine; overview and background are in the README. Addresses are hunk-relative in `ns`. This note was written while the engine was being mapped, and some of its statements are out of date: the lift, the enemies, the rooms and the ending are described in their own notes (`lift.md`, `enemies.md`, `rooms.md`, `ending.md`), which supersede the "next steps" and the remarks below about what is not ported yet.

Status per statement: **verified** (confirmed in the emulator), **code** (read from the disassembly but not tried), **hypothesis**.

## Tools

- `py tools/scriptdis.py ops` lists opcodes, handler addresses and the number of operand bytes.
- `py tools/scriptdis.py scripts assets-local/scripts.txt` disassembles all scripts in the script table. The output is kept locally because it is derived from the original's data.

The operand count is automatic: the handler is followed linearly, and both reads through `(a5)+` and skips with `adda/addq #n,a5` are counted. The latter is needed because the conditional jumps read their operands through `a0` and skip over them. All 170 scripts decode without any invalid opcode. The conditional jumps to `$18BE` and the loop of op37 (through `$10F8`) are treated as "continue if the condition is false". Only op40 and op41 (large handlers with endings of their own) still stop the disassembler.

Use in the scripts (number of occurrences): op00 5,591, op22 895, op27 542, op02 320, op16 310, op23 223, op24/25 120 each, op03 116, op01 111, op32/33 96 each.

## The actor loop (`$C36`)

**Code + verified.**

- 5 actor slots of 30 bytes from `$2B28` (counter in `$C7C`, start value 4).
- Slot `+$0C`: script pointer (absolute address), loaded into `a5`. Slot `+$10`: pointer to the actor's data, loaded into `a4`. A slot where `+$10` is 0 is skipped.
- After the interpreter `a5` is written back to `+$0C`. Next time the script continues where it stopped.
- In game mode from a savestate only slot 0 is used (the player, actor data at `$8140`). The other slots are empty.

## The interpreter (`$C7E`)

**Code.**

The interpreter fetches the opcode byte, looks up the handler's address in a table with 4 bytes per opcode (at `$21EC`) and jumps there. The handler reads its operands from the script stream and jumps back to the interpreter.

91 opcodes (0 to 90). The table ends where the values are no longer code addresses.

## The script table (`$2BBE`)

**Code.** 170 longwords with absolute pointers to scripts. The scripts are in the code hunk from `$2E6A`. Opcode 34 (`goto n`) jumps to script *n* through the table.

## Known opcodes

| Opcode | Operands | Meaning | Status |
|---|---|---|---|
| 0 | - | Wait a frame: leave the interpreter, continue here the next frame | **verified** (about 50 per second when the player walks; measured as 44/s plus 5 jumps that were not counted, over 3 s) |
| 1, 2, 3 | 2 | Set the actor's two picture numbers (`$4(a4)`, `$1C(a4)`), counted from a base (`$EA0`/`$EA2`) and a direction factor (`$4F2`). Op2 and op3 add 116 (`$74`) and another addition respectively, probably other directions or variants | code; the meaning is a hypothesis |
| 7 | 1 | Upper picture = *n* + 116 + base `$EA0` x direction `$4F2` | code |
| 10 | 1 | Upper picture = *n* + 622 (`$26E`) | code |
| 13 | 1 | Lower picture = *n* + 254 (`$FE`) + base `$EA2` x direction | code |
| 16 | 2 | Upper and lower picture = *n* + 394 (`$18A`) | code |
| 17 | 1 | Upper picture = *n* + 116 | code |
| 20 | 1 | Lower picture = *n* + 394 | code |
| 21 | 2 | Set the picture bases `$EA0` and `$EA2` | code |
| 26 | 2 (signed) | The upper sprite part's position = the actor's position (slot `+0/+2`) + (dx, dy) to actor data `+0/+2` | code |
| 27 | 2 (signed) | The same for the lower part to actor data `+$18/+$1A`. Occurs before every picture change in the walking scripts (for example -4, -6, -16) | code |
| 22-25 | - | Move the actor 1 pixel right/left/up/down (both sprite parts and the slot's position). When a tile border is passed the sensors are read again (see below) | code |
| 35 | 1 | Call script *n* (gosub). The return address is saved in `$1130` (one level) | code |
| 37 | 2 (*count*, *n*) | Counted loop: goto *n* in total *count* times in a row, then continue. The counter is in `$117E` (`$FF` = no loop running) | code; verified (trace, walk left) |
| 38 / 39 | - | Turn right/left: read the sensors `$16-$1A` around the actor's own cell in that direction (not `$1B`), clear/set bit 1 in `$506` | code; verified (trace) |
| 43 | - | Return to the saved script position (`$17E0`) | code |
| 61 | - | If fire: set the latch flag `$1A24` | code |
| 62 / 63 | - | If the latch flag is set: clear it. Otherwise, if the actor stands on tile `$29` (the lift): start the lift cabin in slot 4 (`$2BA0`) with a script of its own and change the actor's own script | code |
| 64 | 2 (*a*, *b*) | Fire: goto *a*. If the last pressed key (`$69AD`) is F1-F3 (`$50-$52`): continue. If no direction: goto *b* | code |
| 65 | 2 (*a*, *b*) | Fire: goto *a*. Down: goto *b* | code |
| 66 | 5 (damage, *a*, *b*, *c*, *d*) | If the actor has been hit (`$1BFC` is non-zero): subtract the damage from a counter (`$1C05`, starts at 3) and choose one of four scripts depending on which side the hit came from and whether the counter ran out | code; the meaning is partly a hypothesis |
| 75 | - | Clear the slot's byte `$1C` and restart the actor's first script | code |
| 78 | 2 (*n*, sound) | Shot: for each weapon there are 4 bytes at `$4F6 + weapon*4` (rounds in the magazine, magazines, magazine size, HUD flag). Empty magazine: reload from the reserve (set the HUD flag), or goto *n* if there is none. Take a round and play the sound (continues into op81) | code |
| 79 | - | Play the next sound from a zero-terminated cyclic list (`$20C2`, position in `$20CE`). Used for footsteps. After one lap the first sound is played twice, just as in the original | code |
| 81 | 1 | Play sound *n* through `$64FE` | code |
| 90 | - | Set the outcome `$21EA`=2 (the explosion ending) and end the frame | code |
| 82 / 83 | 1 | Sound effect *n* from the table at `$6432` (10 bytes per record) through `$652C` and `$655A` respectively, probably start/stop | code; the meaning is a hypothesis |
| 85 / 86 / 87 | - | Sound channel 0/1/2: switch on DMA and set the length to 1 word, that is, silence the channel after the current sample | code; the meaning is a hypothesis |
| 28 | 3 | `dx` to `$8F5A`, `dy` to `$8F5C`, the third byte to `$8F5F`. Marks movement (`$8F60`=1) | code |
| 29 | - | Clear `dx`, `dy`, `$8F5E/$8F5F` and the movement flag | code |
| 30 / 31 | - | `dx` = +1 / -1, `dy` = 0, the movement flag is cleared | code |
| 32 / 33 | - | `dx` = 0, `dy` = -1 / +1, the movement flag is cleared | code |
| 34 | 1 | `goto` script *n* (through `$2BBE`) | code; used to chain and loop the walk cycle (seen in the emulator) |
| 40 | 12 (script numbers) | **The player's controls when standing still** (`$11DC/$1384`). F1-F3 change weapon (`$4F2`=0-2, needs a bit in `$504` for F2/F3), mark the weapon in the HUD palette (`$7296`) and restart the weapon's idle script (`$2E6A`/`$30F8`/`$33FC`), after which the frame ends. Otherwise, in turn: the down flag (`$18BC`) goes to operand 10. Space (`$40`) on tile `$26-$29` with charges left (`$502`) counts down and goes to operand 6 (lay a charge). Right and the side sensor >= 11 goes to operand 0. Left goes to operand 1. Up (if not `$C1C`): tile under the feet `$26-$28` calls routine `$23CC`; side sensor `$0B-$0F` goes to operand 2; two rows up `$21-$25` goes to operand 3; otherwise routine `$16A8` (the lift). Down: `$C1C` goes to operand 9; diagonally below `$1B-$1D` goes to 7; below `$1E-$20` goes to 8; otherwise 9. Fire goes to operand 11. It also sets `$506` (bit 0/1 = walk to the right/left started), `$C34` (up/down) and `$17DC` (its own address for op42) | code; walk right + wall stop **verified** (trace) |
| 41 | 12 | **The player's controls facing left** (`$1442/$15EA`), a mirror image of op40: sets bit 1 in `$506`, saves its address in `$17E0`, walks forward (left) only if the side sensor is >= 11, right turns (operand 0). Up: side sensor `$16-$1A` (instead of `$0B-$0F`). Down: `$1A` in `$13-$15` (instead of `$1B-$1D`). Idle scripts after F1-F3: `$2EE2/$3170/$3474` | code; verified (trace left, stairs up) |
| 42 | - | Back to the last op40 (`$17DC`) | code; verified (trace) |
| 43 | - | Back to the last op41 (`$17E0`) | code; verified (trace) |
| 44 | 2 (*a*, *b*) | Floor change: calls `$FFC` (the actor's cell in the tile buffer). Stick up and the tile 4 rows above (-240) is `$29` or `$2A`: goto *a*. Stick down and the tile 4 rows below (+240) is `$29`/`$2A`: goto *b*. Otherwise continue. `$29/$2A` is the lift, one tile per floor (see "The lift" below) | code |
| 45 | 1 (*n*) | If the stick is right and sensor `$19` >= 11: goto *n* | code; wall stop **verified** |
| 46 | 1 (*n*) | If the stick is left and sensor `$19` >= 11: goto *n* | code; wall stop **verified** |
| 47 | 1 (*n*) | If the stick is down and sensor `$1B` >= 11: set `$18BC`=1 and goto *n* | code |
| 48-51 | - | The lift cabin (slot 4, actor data `$8128`): start script `$4943`/`$49B2`/`$4A21`/`$4A92` | code |
| 52 / 53 | - | Set the status of the actor's two bobs (`+6`, `+$1E`) to 1 and 2 respectively (2 = hide) | code |
| 54-57 | - | The lift cabin: status 2; y -64 and status 1; y +64 and status 1; status 1 | code; the meaning is a hypothesis (one floor = 64 px) |
| 58 / 59 | - | `$17C6`: clear bit 0 and set bit 1 / set bit 0 | code |
| 60 | - | Clear `$17C6` and empty slot 4 (the lift cabin is gone), end the frame | code |
| 68 / 69 | - | Shot right/left (hitscan): the nearest living actor (`+$10` non-zero, `+$1C` = 0) in slots 1 to 3 on that side, at any height. Hit: weapon 1 kills at once, otherwise the hit points in the enemy record (`+8` to `+$14`) are reduced and the enemy changes to its hurt script (`+$10`); at 0, `+$1C`=1 is set and the death script (`+8`) is started. `$1F44` clears `$1BFC` if it was the one hit | code; without a hit verified (trace fire); the hit was not ported when this was written |
| 80 | - | Like op79 but with the list at `$2114` (position in `$2120`), probably footsteps on stairs | code; verified (trace stairs) |

The common ending of the conditional jumps: `$18BE` reads the operand byte and does a `goto` through `$2BBE`. The player's script thus works as a state machine, roughly "stand still; if right and free, change to walk right".

## The lift (`$16A8`)

**Code; that `$29/$2A` is the lift is a strong hypothesis** (the map has them one floor apart in column 489 with the same frame, and the walkthrough mentions lifts). The lift is described in full in `lift.md`.

Op40/op41 call `$16A8` when up is pressed and no other up case applies. First `$16D8`: if the view's block column (`$8F72`) is 0-2, 3-5 or 6-8, bit 0, 1 or 2 in `$505` is required (the cards, which are set when items are picked up and shown under "ELEVATOR/DOOR CARDS" in the HUD). If the card is missing nothing happens. Further to the right no card is needed. Then, if the player stands on `$29/$2A` and `$17C6` bit 0 is not set:

- `$17C6` bit 1 set: the lift cabin (slot 4) restarts at `$46A6`, and the player continues with op40's operand 5.
- Otherwise: set bit 0, put the cabin in slot 4 at the player's position - (8, 22) with actor data `$8128` and script `$455E`. The player continues with operand 4.

## Movement and scrolling per frame (`$8888`)

**Code + verified (trace stairs).** `$8F5E` is a countdown: when it is not 0 it is decremented and nothing else happens. Otherwise it is reloaded from `$8F5F`, the view is moved by dx/dy (`$892C`), and if bit 0 of `$8F60` is not set dx/dy are cleared. Op28 thus sets a lasting movement with a delay (walking: `op28 1 0 0`, stairs: `op28 255 255 2` = a step every third frame), while op30-33 only apply for one frame.

## The actor's position and sensors

**Code + verified.**

- The slot's byte `$14`/`$15` is the actor's tile position in the view. `$FFC` calculates the actor's cell in the **tile buffer at `$9118`** (60 tiles wide, 16 rows): `$9118 + ($14 + $8F76) + ($15 + $8F78) * 60`.
- The slot's bytes `$18-$1B` are **sensors**: tile ids from the cells around the actor, filled in by the handlers around `$F2A-$FF4` and `$1190-$11CA`. Turned right: two rows up (`-$78`), the cell to the right (`+1`), the row below and two steps to the side (`+$3E`), two cells to the right (`+2`). Turned left: the same with `-1`, `-2` and `+$3A`.
- **Collision rule: tile id < 11 blocks.** No separate collision table is needed for walls. **Verified:** when walking right from the start the figure stops at x=8112 with `$19`=0, and to the left at x=7920 with `$19`=6. The walking bridge is 12 tiles long.

The other opcodes were not mapped when this was written. Their handler addresses and operand counts are in `scriptdis.py ops`.

## Movement per frame (`$892C`)

**Code + verified.**

- `$8F56 += dx` (the view's and the player's x position in the world) and `$8F7A += dx` (the pixel within the tile, 0-15).
- When `$8F7A` wraps, the tile column within the block (`$8F76`), the block column (`$8F72`) and **`$4EE`** (the player's position index, see below) are adjusted. The same goes for y through `$8F58`, `$8F7C`, `$8F78`, `$8F74`.
- Bits in `$8F6C` mark that a tile border has been passed to the left/right/up/down. The map drawer uses them to draw in new columns and rows.
- Measured: when walking right `$8F56` increases by 1 per frame. When the stick is released the figure continues to the next even 16-pixel border. When the direction is changed it takes about 0.6 s before the figure starts to walk (the turn). **Verified** with `tools/uae/uaectl.py`.

## Attributes per position

**Code, the meaning is a hypothesis.** At `$23D0` `$4EE` is used as an index into a byte table at `$A9F4`. The value x 12 points out a 12-byte record in a table at `$ACAC`, and the record's flags control what happens (for example `btst #5,1(a0)`). The tables follow directly after the map matrix (`$A333`, 29 x 26 bytes) and the 0/1 table at `$A5A5`. They are probably the game's collision and interaction data (floors, stairs, doors, lifts). The rooms are described in `rooms.md`.

## The sensors read through the view, with a one-frame delay

**Verified (trace).** `$FFC` calculates the actor's cell as the *view's* tile position (`floor(view_x/16)`, where `view_x` = `$8F56`) plus the slot's `$14/$15`. The view is scrolled by `$892C` **after** the actor loop in each frame. Sensors that are read in a frame therefore see the view from the previous frame. For the player (fixed on screen cell (10, 6), world x = view x + 160) that gives the base `floor((x - dx)/16)`, not `floor(x/16)`. With `floor(x/16)` the port went one tile too far before it stopped at the wall.

## The Go port of the engine (`pkg/game/script`)

- `Program` (the code hunk + the script table), `VM` with 5 `Slot`s, `Globals` and an `Env` interface for the stick, keys, tiles (world coordinates), helper actor, hit and sound.
- `VM.Frame()` runs all active slots to the next wait and then scrolls the view like `$8888`. A slot that reaches an unmapped opcode stays on it, and the rest of the frame is run anyway.
- Unmapped opcodes give `ErrUnimplemented`, and unmapped routines of the original give `ErrUnmapped`. Nothing is guessed.
- **Comparison with the original:** `TestAgainstOriginalTrace` runs all `assets-local/uae/trace_*.json` (recorded with `py tools/uae/capture_script.py <direction> <seconds> [movement]`). The test starts the port from the same state, runs as long as the recording and compares the sequence of changes in (script position, picture +0, picture +$18, dx, x). Repeated steps in the original trace are merged, because the recording sometimes reads memory between the actor loop and the scrolling. All steps agree: right 134, left 95 (turn, op37-39, op41), down 20, fire 150 (op68, op78), up from the start 1, stairs up 201 (`right:2.5,left:0.15` first; op80, `$8888`).

Note: the Go names `Upper`/`Lower` for the actor's two bobs are misleading. The record `+0` (`Upper`) is drawn at the actor's y (the legs), and the record `+$18` (`Lower`) 16 pixels higher up (the upper body).

## Drawing (`$7D7E`, `$803A`)

**Code + verified against a screenshot.** The bob list starts at `$8128` (the lift cabin), followed by the player's two records `$8140/$8158`. Each record is 24 bytes: x, y (world pixels), picture number (a direct index into `NSIBobs`), status (`+6`: 0 = empty, 2 = hide, otherwise draw). Screen position = (x - `$8F56` - 16, y - `$8F58`). The visible play area is 320 x 128 (measured in the emulator's screenshot: the map is visible up to and including line 128, the HUD starts on line 129). The blitter draws 4 screen planes: plane *p* gets the next stored plane if bit *p* in the BHDR record's first tag byte is set, otherwise it is cleared under the mask. The reading continues past the colour planes into the mask plane, so a 3-plane bob with tag `$4F` gets the mask as plane 3 (colours 8-15). `amiga.Bank.Bob` decodes it so.
