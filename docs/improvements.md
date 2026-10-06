# Improvements and additions (deviations from the original)

The port is meant to be identical to the original. Everything that is not in the original is behind **one** flag, which is **off** by default in the engine: `-improvements` on the command line turns everything on, or the "Improvements" line in the Esc menu (it takes effect at once in a running game). With the flag off, which is the default, the game is a plain copy of the original. A build can have it on by default; the default is set when the game is built (`-ldflags "-X main.defaultImprovements=true"`). The flag is a setting and is not saved in saved states. The tests run with it off unless stated otherwise.

## What the flag controls

| Addition | Original behaviour (flag off) | Where in the code |
|---|---|---|
| Leave the lift with left, right or fire | only fire, and the lift only looks for it once every few frames, and not at all in the first 36 of each floor | `script.VM.liftExit`, `game.env.LiftExitRequest` |
| Leave the lift facing right if you chose right | the exit always ends with a turn to the left (script 89) | `op43` at `$455D` in `script.go` |
| Stairs up and down whichever way the player faces | the controls only look for stairs in the direction the player faces | `script.VM.oppositeStairs` |
| The roll starts with a buffered press of down | the press has to hit the one frame per step (32 frames) that the script looks at | `game.env.TakeRollRequest`, `Joystick` |
| The press of down that leaves a room is ignored until it is released | the player ducks if the key is still held | `State.IgnoreDown` |
| Cheat keys: F4 (all weapons, cards, full ammunition and health, the whole map), F5 invulnerable, F6 double speed | do not exist | `State.cheat`, `input.Cheat` and others |
| P toggles pause | P pauses and fire releases the pause (`$7602`) | `State.Step` |
| WASD as direction keys, F4 to F6 | only the arrow keys, Ctrl, space and F1 to F3 | `cmd/game/keyboard.go` |
| A minimap with fog | does not exist | `render/minimap.go` |
| Ammunition, health and cheat indicators and "PAUSED" in the HUD | the HUD only shows what the original shows | `render/hud.go` (`Renderer.Extras`) |
| The title: fire goes to the next page, Esc skips everything, the fire that ends the title is blocked in the game until it is released | fire is only read during the credits and on "press fire" | `title.Title.Improve`, `cmd/game/main.go` |
| A softer stereo image (0.5 instead of the Amiga's hard left and right) | channels 0 and 3 left, 1 and 2 right | `Mixer.Separation`, `app.stereo` |

## Development tools that the flag does not control

The Esc menu (which the flag itself is in), saved states (F9/F10) and the command line flags (`-ticks`, `-shot`, `-inputs`, `-hold`, `-load`, `-save`) are tools around the game and do not affect the rules.

## Deviations that are not additions

Disk loading times are not reproduced (rooms, the pictures in the title and the ending). The volume of the stair steps can be raised with `stairGain` in `cmd/game/main.go`; it stands at 1.0, which is the original level.
