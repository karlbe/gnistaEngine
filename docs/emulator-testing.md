# Automatic tests against the original in WinUAE

The original game in WinUAE is the reference for the port. `tools/uae/uaectl.py` starts and controls the emulator without a GUI and without anyone sitting at the computer. It needs your own copy of the game disk and a Kickstart ROM, neither of which is part of this repository.

## What you need

- WinUAE 5.3 (the path can be set with the environment variable `WINUAE`).
- A Kickstart 1.3 ROM (34.5, unencrypted, 256K). A 512K `Kickstart 1.3.rom` should **not** be used, because it gives a ROM key warning.
- A configuration: `assets-local/uae/pgi.uae`. It is an A500 with OCS, 68000, cycle exact, 512K chip + 512K slow, the game's ADF in df0, no sound, `gfx_api=gdi`, and a joystick in port 2 on the arrow keys + right Ctrl (`kbd2`).
- Everything under `assets-local/uae/` (configuration, savestate, screenshots) is local and is not committed.

## Commands

```
py tools/uae/uaectl.py boot              cold start, press fire through the intro, save assets-local/uae/ingame.uss
py tools/uae/uaectl.py launch [--fresh]  start from ingame.uss (or cold start) and let it run
py tools/uae/uaectl.py watch [ADDR...]   attach to a running WinUAE and print variables when they change
py tools/uae/uaectl.py shot FILE.png     attach and take a screenshot
```

`watch` without addresses shows `$21EA` (outcome), `$8F56/$8F58` (view position) and `$8F72/$8F74` (block coordinates). It works while you play yourself, so you can see what happens in memory when, say, you take a lift.

## How it works

**Starting.** WinUAE is started with `-f pgi.uae` and `-s key=value` for overrides. A savestate is loaded with `-s statefile=...`, because the flag `-statefile` is ignored together with `-f`. Boot uses turbo disk loading (`floppy_speed=0`), which only affects loading, not the game. Warning dialogs are clicked away automatically. Focus is handed back to the window the user had.

**Memory.** WinUAE maps the Amiga address space linearly in its process memory (`host = base + Amiga address`). The tool finds the running, relocated code hunk of `ns` (the file name table plus matching relocations, so that unrelocated copies in DOS buffers are rejected). It then reads and writes the game's variables with the same hunk-relative addresses as in `docs/re/`. The hunk is loaded at different addresses in different runs (for example `$C08708`, `$C08738`), but the search handles that. Chip RAM is reached through the same base, which is verified with ExecBase and `ChkBase`.

**Input without focus.** Key presses through Windows go to the window that has focus, so they do not work. Instead `take_control()` redirects all of the game's reads of the joystick and the fire button to two mailboxes in unused chip RAM (the 68000 user vectors):

| Instruction in `ns` | Places | Mailbox |
|---|---|---|
| `move.w $DFF00C.l,Dn` (JOY1DAT) | `$235E`, `$2618` | `$3F0` (word, JOY1DAT encoding) |
| `btst #7,$BFE001.l` (fire button, port 2) | `$140`, `$15C`, `$178`, `$238E`, `$282C`, `$67C4`, `$681E`, `$68A2`, `$760E` | `$3F4` (bit 7, 0 = pressed) |

The instructions keep their opcode and length, and only the address field is changed. The game's logic is therefore not affected. `release_control()` restores the original bytes. This is always done before a savestate, so that the saved game can be played with a normal joystick. `set_input(dirs, fire)` and `hold(dirs, fire, seconds)` write the mailboxes.

**In game.** The input routine `$235C` writes a non-zero direction mask to `$23CA` every frame (bit 0 no direction, 1 up, 2 right, 3 down, 4 left, 5 fire). The value in the file is 0, so `$23CA != 0` means that the game has started.

**Screenshots.** `PrintWindow` on the emulator window. With Direct3D the picture goes black when the window has no focus, and that is why `gfx_api=gdi` is used.

## Example

```python
import sys; sys.path.insert(0, "tools/uae")
import uaectl, time
u = uaectl.UAE.launch(state=uaectl.STATE)
u.wait_hunk(30)
u.take_control()
x0 = u.word(0x8F56)
u.hold(["left"], seconds=1.5)
print("the view moved", x0 - u.word(0x8F56), "pixels")
u.screenshot("assets-local/uae/shots/test.png")
u.release_control()
u.kill()
```

## Script traces

`py tools/uae/capture_script.py <direction> <seconds> [movement] > assets-local/uae/trace_<name>.json` starts from the savestate and first runs the movement (for example `right:2.5,left:0.15`). When the player has stood still for 1.5 s the starting state is saved (slot 0 and the engine's variables), and then the direction is held while every change of script position, pictures, dx and view y is recorded. `pkg/game/script/trace_test.go` runs all `trace_*.json` against the Go engine.

Enemies shoot: on the floor above the stairs by the bridge the player is shot if he walks left for a while (script `$4F2E`). Avoid that in movements until the enemies are ported.

A walkthrough (a route through the whole level, third-party text) can be kept locally in `assets-local/walkthrough/`.

## Limitations and next steps

- The time resolution of input is Python sleep (milliseconds), not frames. For frame-exact tests the game's VBlank has to be synchronised, for example by reading a counter that the game increments every frame. That has not been found yet.
- Keyboard commands in the game (if there are any) do not go through the mailboxes.
- The sound is switched off in the configuration.
