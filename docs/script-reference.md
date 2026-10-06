# Language reference for the game's scripts

**Write scripts with the friendly language in `docs/script-friendly.md`.** This file describes the engine's own instructions underneath it, for anyone who wants to know exactly what each word does; the friendly language is translated into them. The file describes the script language that the game engine (`pkg/game/script`) runs, **in its own words and from the engine's own Go code**, not from the original's assembler. It is meant as a basis for writing your own scripts. The opcode numbers are the original's, because the engine still reads the original's bytecode; an assembler of our own gives them names.

## The model

- The game runs up to five **slots** each frame (50 per second). Slot 0 is the player, 1–3 are enemies, 4 is the helper slot (lift cabin or explosive charge).
- A slot has a **script** (a sequence of opcodes, one byte each, with operand bytes after), an **actor** (two image parts: upper body and legs, each with a position and an image number), a position in the world (pixels) and six **sensors**.
- Each frame the script of each slot runs until it meets a wait opcode (`wait`). The next frame it continues from there. After all slots the view is scrolled.
- The scripts are numbered 0–169. Opcodes that jump give a script number. The engine also starts scripts directly at an address.

### Images

An actor consists of two parts. An image is set as `n + offset + base × weapon`: `n` is the operand, the offset depends on the opcode, and `base` is the current base for the part (set with `setbase`) multiplied by the weapon (0–2). That is why the player's poses for the pistol, shotgun and rifle lie at a fixed distance in the image bank.

The parts' positions are set with `part_upper`/`part_lower` relative to the slot's position. The slot's position is moved with `stepr/stepl/stepu/stepd`.

### Sensors

Six tile ids around the actor are re-read each time it has passed a tile boundary: `here` (the tile it stands on), `below`, `above2` (two rows up), `side` (the tile in the direction it is going), `belowside` and `side2`. They are used by the conditional opcodes. Tile ids below 11 block walking (see `docs/packs.md`).

## Opcodes

`n` is a script number, `b` a byte, `s` a signed byte.

### Time

| Op | Name | Meaning |
|---|---|---|
| 0, 4, 5, 14, 15 | `wait` | end the frame; continue here next time |
| 77 | `deactivate` | the actor is removed from the slot (dead enemy, finished helper) |
| 90 | `explode_end` | the game ends with the explosion outcome |

### Images and positions

| Op | Operands | Meaning |
|---|---|---|
| 1 / 2 / 3 | b b | the image of both parts: offset 0 / 116 / 254, plus base × weapon |
| 6–13, 17–20 | b | the image of one part: see the table in `script.go` (`singleFrameOps`): the upper part 6, 7, 8, 9, 10, 17, 18 and the lower part 11, 12, 13, 19, 20, with different offsets; some add base × weapon |
| 16 | b b | the image of both parts = n + 394 |
| 21 | b b | set the image bases (upper, lower) |
| 26 / 27 | s s | the upper / lower part's position = the slot's position + (dx, dy) |
| 52 / 53 | – | show / hide the actor's images |

### Movement

| Op | Operands | Meaning |
|---|---|---|
| 22 / 23 / 24 / 25 | – | move the actor one pixel right / left / up / down. If a tile boundary is passed the sensors are re-read |
| 28 | s s b | set a lasting scroll (dx, dy) that is taken every (b+1)th frame |
| 29 | – | stop the scrolling |
| 30–33 | – | one scroll step right / left / up / down for this frame only |
| 38 / 39 | – | turn right / left and re-read the sensors |

The player stands fixed on the screen (tile 10, 6 in the view); it is the view that is scrolled when the player walks. Enemies are moved with 22–25.

### Flow

| Op | Operands | Meaning |
|---|---|---|
| 34 | n | jump to script n |
| 35 / 36 | n / – | call script n (one level) / return |
| 37 | b n | jump to n a total of b times in a row, then continue (loop) |
| 42 / 43 | – | back to the player's latest control, right / left |

### The player's control

`40` (turn right) and `41` (turn left) are followed by **twelve script numbers** and read the joystick and keys. They are the player's state machine: it chooses a script for walking, turning, climbing, rolling, shooting, changing weapon or using a door. The operands:

`0` walk/turn right · `1` walk/turn left · `2`, `3` up variants (stairs, ladder) · `4`, `5` lift · `6` lay a charge · `7`, `8`, `9` down variants · `10` after a roll · `11` shoot.

The control tests the tile at the slot's position to choose a variant (door 0x26–0x28 opens the room, stairs/ladder/lift by id) and switches weapon on F1–F3 to the weapon's idle script. It also sets the walking direction. Cards and weapons control whether the lift may be called (see `docs/re/lift.md`).

### Conditional jumps

| Op | Operands | Meaning |
|---|---|---|
| 44 | n n | up and a lift tile four rows above: jump to the first n; down and a lift tile four below: the second n; otherwise continue |
| 45 / 46 | n | right / left held and a clear way ahead: jump to n |
| 47 | n | the roll: down held and a clear way: jump to n and set the "down flag" |
| 64 | n n | fire: the first n; otherwise if F1–F3 continue; otherwise if no direction: the second n |
| 65 | n n | fire: the first n; down: the second n; otherwise continue |
| 61, 62, 63 | – | the lift's exit: 61 reads fire (with `-improvements` also left/right), 62/63 start the cabin at a lift tile if fire is not stored |

### Hits

| Op | Operands | Meaning |
|---|---|---|
| 66 | damage a b c d | if an enemy has fired: take the damage (hits decrease, at zero a life is lost). a, b: hit from the right (life lost, dead); c, d: from the left. 67 returns after 66 |
| 68 / 69 | – | shoot right / left: the nearest living enemy on that side, regardless of height, is hit; the shotgun kills at once, otherwise the hit points decrease and the enemy goes to its hurt script, at zero to its death script |
| 70 / 71 | – | the same, but the enemy gets the second death script |
| 75 | – | the enemy lives again: start the main script over |
| 76 | – | the enemy shoots at the player (if no one else is already aiming) |
| 78 | n b | the player's shot: take a shot, reload from the reserve (otherwise jump to n), play sound b |

### Lift and explosive charge (the helper slot)

| Op | Meaning |
|---|---|
| 48–51 | start the lift cabin with one of four scripts |
| 54, 55, 56, 57 | hide the cabin; the cabin one floor (64 px) up / down and show; show |
| 58, 59, 60 | the lift: called / busy / the cabin disappeared (ends the frame) |
| 72, 73, 74 | lay a charge at the actor; show it; it explodes (kills whoever stands on it, otherwise opens the door) |

### Sound

| Op | Operands | Meaning |
|---|---|---|
| 79 / 80 | – | the next effect from the cyclic footstep list / stair step list |
| 81, 82, 83, 84 | b | play effect b on channel 0, 1, 2 (through different routines), 3 |
| 85, 86, 87, 88 | – | channel 0–3: let the sound finish playing and go quiet |
| 89 | – | start channel 1 (looping until it is loaded again) |

## What the language cannot do

There are no variables, no arithmetic and no expressions. All you can do is change image, move a pixel, wait, jump and ask the engine about some fixed conditions. Behaviour that is not one of the opcodes above (the spawner, the rules of the lift and the doors, the contact collision, the clock) lives in Go and is a change to the engine, not to the scripts.

## The assembler

Own scripts are written as text (`*.gs`) and assembled by `pkg/game/script/asm`, which `packtool check` and the game call when a pack has `scripts/`. The source format:

```
; comment
script walk_right        ; a script begins; scripts are numbered in the order they appear
    frames 1 64          ; instruction and operands
    stepr
    wait
    goto walk_right      ; scripts refer to each other by name
mark here                ; inside a script: names the address of the next byte
data step_sound 11 12 13 0 ; raw bytes (for example a zero-terminated sound list)
```

Operands are integers (`0x10` works, signed operands can be negative) or script names. The files in `scripts/` are assembled in file-name order. `program.json` points out the scripts and tables that the engine starts by itself (the player, the enemies, contact scripts, sound lists, image numbers, magazine sizes, enemy templates); all names are looked up in the assembled image. The language is described in `docs/script-friendly.md`.

Names and operand types (b = byte, s = signed byte, n = script). The operand count has been tested against all the original's scripts: they decode exactly (`TestDecodeOriginal`).

| Op | Name | Operands |
|---|---|---|
| 0 | `wait` | |
| 1 | `frames` | b b |
| 2 | `frames116` | b b |
| 3 | `frames254` | b b |
| 6 | `legs_w` | b |
| 7 | `legs_w116` | b |
| 8 | `legs_w254` | b |
| 9 | `legs_w394` | b |
| 10 | `legs_622` | b |
| 11 | `body_w` | b |
| 12 | `body_w116` | b |
| 13 | `body_w254` | b |
| 16 | `both_394` | b b |
| 17 | `legs_116` | b |
| 18 | `legs_394` | b |
| 19 | `body_254` | b |
| 20 | `body_394` | b |
| 21 | `setbase` | b b |
| 22 | `stepr` | |
| 23 | `stepl` | |
| 24 | `stepu` | |
| 25 | `stepd` | |
| 26 | `legs_at` | s s |
| 27 | `body_at` | s s |
| 28 | `scroll` | s s b |
| 29 | `scroll_stop` | |
| 30 | `scroll_r` | |
| 31 | `scroll_l` | |
| 32 | `scroll_u` | |
| 33 | `scroll_d` | |
| 34 | `goto` | n |
| 35 | `call` | n |
| 36 | `return` | |
| 37 | `repeat` | b n |
| 38 | `face_right` | |
| 39 | `face_left` | |
| 40 | `control_right` | n n n n n n n n n n n n |
| 41 | `control_left` | n n n n n n n n n n n n |
| 42 | `resume_right` | |
| 43 | `resume_left` | |
| 44 | `lift_choose` | n n |
| 45 | `if_right_free` | n |
| 46 | `if_left_free` | n |
| 47 | `if_roll` | n |
| 48 | `cabin0` | |
| 49 | `cabin1` | |
| 50 | `cabin2` | |
| 51 | `cabin3` | |
| 52 | `show` | |
| 53 | `hide` | |
| 54 | `cabin_hide` | |
| 55 | `cabin_up` | |
| 56 | `cabin_down` | |
| 57 | `cabin_show` | |
| 58 | `lift_called` | |
| 59 | `lift_busy` | |
| 60 | `lift_gone` | |
| 61 | `lift_exit_check` | |
| 62 | `lift_enter_up` | |
| 63 | `lift_enter_down` | |
| 64 | `if_fire` | n n |
| 65 | `if_fire_down` | n n |
| 66 | `take_hit` | b n n n n |
| 67 | `hit_return` | |
| 68 | `shoot_right` | |
| 69 | `shoot_left` | |
| 70 | `shoot2_right` | |
| 71 | `shoot2_left` | |
| 72 | `charge_place` | |
| 73 | `charge_show` | |
| 74 | `charge_blow` | |
| 75 | `revive` | |
| 76 | `enemy_fire` | |
| 77 | `deactivate` | |
| 78 | `fire_weapon` | n b |
| 79 | `step_sound` | |
| 80 | `stair_sound` | |
| 81 | `sound0` | b |
| 82 | `sound1` | b |
| 83 | `sound2` | b |
| 84 | `sound3` | b |
| 85 | `silence0` | |
| 86 | `silence1` | |
| 87 | `silence2` | |
| 88 | `silence3` | |
| 89 | `start1` | |
| 90 | `explode_end` | |
