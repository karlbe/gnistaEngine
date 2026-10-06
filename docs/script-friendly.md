# The script language: write behaviour in plain words

A script describes how a figure behaves: which image it shows, how it moves and what it waits for. You write scripts as text in `scripts/*.gs` in a content pack. The language has plain words, names for numbers, images and sounds, and templates for whatever repeats. You do not need to know the engine's own instructions (opcodes); they are in `docs/script-reference.md` if you want them.

Example, one go up the stairs to the right:

```
script stairs_up_r
    place legs 0 0
    place body 0 -16
    camera follow up-right every 3 ticks
    repeat 8 times
        stairstep
        repeat 4 times as i
            hold legs=legs_walk_r+i body=body_aim_r for 6 ticks moving up-right every 3 ticks
        end
    end
    goto level_out_r
```

Everything after `;` on a line is a comment. Spaces and blank lines do not matter. Words are written in lower case.

## Time and movement

The game runs 50 frames (ticks) per second. A script runs until it waits; on the next tick it continues from there.

| Write | Meaning |
|---|---|
| `wait` / `wait 5` / `wait 5 ticks` | wait one or more ticks |
| `show POSE` / `show legs=A body=B` | show an image (legs and upper body) |
| `hold POSE for 6 ticks` | show and wait |
| `hold legs=A body=B for 4 ticks moving right` | and move the figure one pixel per tick to the right (`left`, `up`, `down`, or `up-right`, `down-left` and so on) |
| `... moving up-right every 3 ticks` | one step every third tick instead of every tick |
| `place legs X Y` / `place body X Y` | where the legs and the upper body sit, in pixels from the figure's position (the upper body usually sits 16 up: `place body 0 -16`) |
| `move right 4` | four pixels to the right, one per tick (`left`, `up`, `down`) |
| `move up 4 camera` | and the view follows |
| `step right` | one step at once without waiting |
| `camera follow right` | the view follows, one pixel per tick (`every N ticks`, `speed N`; directions as above) |
| `camera stop` / `camera nudge up` | stop following / one step for the view |
| `face right` / `face left` | turn the figure (this also controls what it senses in front of it; the player's idle state does it every tick) |

## Images, sounds and names

```
const walk_ticks = 4              ; a name for a number (numbers, or sums and products: walk_ticks*2+1)
sound pistol = 0                  ; a name for a sound effect (the file sounds/0.wav)
pose stand legs=0 body=64         ; a name for a pair of images
```

A name can be used anywhere a number is expected. Image numbers are the numbers of the files in `sprites/`. The weapon then adds its base to each image: `weapon sprites legs=0 body=64` means that the upper body is shown as image 64 with the pistol, 128 with the shotgun and 192 with the rifle.

| Write | Meaning |
|---|---|
| `play pistol` | play the sound (channel 0). `play lift_motor channel 1 looping` makes it go round and round |
| `footstep`, `stairstep` | the next footstep or stair step from the sound list (`program.json`) |
| `start sound` | start the sound that `fire weapon` has loaded |

## Repetition and templates

```
repeat 8 times as i               ; i counts 0 to 7 and can be used as a number
    hold legs=1+i body=64 for 4 ticks moving right
end
```

What separates the scripts of three weapons is a sound and which script comes next, so they are written once as a template and used three times:

```
template fire_right effect
    show legs=legs_stand_r body=body_fire_r
    fire weapon sound=effect empty=dry_r
    ...

script fire_r_0 from fire_right effect=pistol
script fire_r_1 from fire_right effect=shotgun
```

A template's parameters are replaced everywhere they appear as a whole word (in `key=value` only the value is replaced). Give them names that are not also words in the language. `self` is the script's own name.

## Jumps

| Write | Meaning |
|---|---|
| `goto NAME` | continue in another script |
| `call NAME` / `return` | run another script and come back (one level) |
| `if holding right and path_clear goto NAME` | if right is held and the way is clear (`left`) |
| `if rolling goto NAME` | if down is held while walking: roll |
| `branch fire=A idle=B` | if shooting: A; if doing nothing: B (at once, without waiting: wait first yourself); otherwise continue |
| `branch fire=A down=B` | if shooting: A; if holding down: B; otherwise continue |
| `controls facing right right=A left=B ... default=C` | the player's buttons, see below |
| `resume controls right` | back to the player's buttons (`left`). This only works in a direction the player has already stood facing |

### The player's buttons

The player's idle state is a script that begins with `controls`. Each `key=script` says which script an input starts; whatever is not given gets `default=`. The keys are:

`right`, `left` (walk or turn), `up_stairs` (up in front of stairs), `up_ladder`, `lift_call` (the first press at the lift), `lift_ready` (when the lift is there), `charge` (space), `down_stairs`, `down_ladder`, `duck` (down), `after_roll`, `fire`.

If nothing is pressed the script continues on the line below: show the standing image, check hits, `wait` and `goto self`.

## Combat

| Write | Meaning |
|---|---|
| `check hits damage=1 right_hurt=A right_dead=B left_hurt=C left_dead=D` | if an enemy has fired: take the damage; the one who takes the damage goes to A (hit from the right) or B (dead), C and D from the left |
| `back from hit` | back to where the hit check stood |
| `fire weapon sound=S empty=SCRIPT` | take a shot from the magazine (reload from the reserve; if everything is gone go to SCRIPT) |
| `shoot right` / `shoot left` | the shot goes: the nearest enemy in that direction is hit. `second_death` gives the enemy its second death script |
| `game over explosion` | the game is over (the explosion ending) |

Enemies:

| Write | Meaning |
|---|---|
| `enemy shoots` | the enemy shoots at the player |
| `enemy revives` | start over from the main script |
| `remove actor` | the enemy is gone (and stays behind as an image on the ground) |

An enemy template in `program.json` gives the scripts an enemy starts with: `start` (walk in), and `scripts`: main script (aiming), after a shot, two death scripts, hurt.

## Charges, lift and doors

| Write | Meaning |
|---|---|
| `place charge` | lay an explosive charge where the figure stands |
| `charge show` / `charge explodes` | show it / it goes off (kills whoever stands on it, otherwise the door opens) |
| `cabin picture N` | the image of the lift cabin (and the charge): frame 622 + N |
| `lift cabin show` / `hide` / `up` / `down` | show, hide, move the cabin one floor |
| `lift cabin 0` ... `3` | start one of the lift's four cabin scripts (`program.json`) |
| `lift called` / `busy` / `gone` | the lift is called / busy / gone |
| `lift choose up=A down=B` | up or down depending on whether there is a lift tile one floor above or below |
| `lift exit_check` | check whether fire is pressed (stop at the next floor) |
| `lift board_up` / `board_down` | at the end of a floor: continue if there is more lift (lift tile 0x29), otherwise stop (0x2A) |
| `show actor` / `hide actor` | show or hide the figure (in the lift) |

## The program: program.json

Besides the scripts, the engine needs to know which ones are its entry points. `program.json` points them out by name: `player` (the first script), `idle` (the idle state per direction and weapon), `lift_call`, `lift_recall`, `cabin` (four), `lift_helper`, `charge`, `blown_up`, `step_sounds` and `stair_sounds` (sound lists: `data name 11 12 13 0`), `contact_right`, `contact_left`, `player_frames`, `mag_size`, the enemy templates (`first_right`, `first_left`, `wave_right`, `wave_left`) and `wave_order`.

## Checking and errors

`go run ./cmd/scriptasm asm <directory>` assembles the scripts and shows errors with file and line. `go run ./cmd/packtool check <pack>` does what the game does.
