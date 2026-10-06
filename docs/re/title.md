# The title sequence (`$6690`, `$6852`)

Status: ported in `pkg/title` and `pkg/render/title.go`. The order and the pictures are **verified** against the original (a cold boot in WinUAE with a screenshot every half second); the silhouette's final pose was measured against the emulator (`$7396`). The timings are **taken from the delay loops in the code**, not measured.

## The pictures (all 320 x 200, 5 bitplanes)

| File | Contents |
|---|---|
| `NSILoader` | the title picture with the logo |
| `IT` | the introduction text (a page of text that sets the scene) |
| `TA` | the silhouette of a man with a rifle. The picture only uses even colour indices (0, 2, ... 14) |
| `PF` | the prompt to press fire to begin |
| `ST` | the story of how the player ends up on the oil rig, shown while the game loads |

## The order

1. The logo (`NSILoader`), black and then the picture with its own palette. A delay loop of `$10C8E0` iterations.
2. Black, then `IT` with its own palette. A delay loop of `$186A00` iterations.
3. Black, then `TA`. Its own palette is only a placeholder. The colours come from five palette tables in the code, which are switched with a short wait between them (`$C350` iterations): `$7316`, `$7356`, `$7396`, `$7356`, `$7396`. The tables give the silhouette three different poses (the arm and the rifle are raised), against a red background. The final pose is `$7396`.
4. Rolling credits (`$6852`) over the silhouette until the text is over or fire is pressed.
5. "Press fire" (`PF`), waits for fire. Fire during the credits jumps straight to step 6.
6. The story (`ST`) while the game loads (`$682A`). The port waits for fire, because the loading is not reproduced.

## The credits roll

- The text is at `$9D82`: lines that end with 0, and a line that begins with `$01` ends it. 89 lines. It is credits text, so it is read from the code at run time and is not embedded.
- Each line is written with a character spacing of 6 pixels at x = `$1E`, y = `$C9` (just below the picture's 200 lines). After each line nothing new is written until the text layer has rolled 15 steps (`$68A2`-`$68C4`), one step per pass of a delay of `$300C` iterations (`$9D6C` moves the layer one pixel up). That is roughly 2.34 frames per pixel, a little more than a minute for the whole text.
- The text is a layer of its own in the bitplane that the silhouette leaves free: a text pixel turns the picture's (even) index into the next odd index, which the palette tables colour white. The text therefore lies on top of the silhouette without hiding it.

## Waiting times

The delay loops are `subi.l #1,d0 / bne` and run about 5263 iterations per frame (the same value that the ending uses): the logo about 209 frames, the introduction text about 304, each pose 10, between the pages 19.

## Deviations

- Fire goes to the next page from all screens (the original only reads fire during the credits and on "press fire"). Esc skips the whole title and `-title=false` switches it off (runs with `-ticks`, `-shot`, `-load`, `-inputs` or `-hold` start in the game unless `-title` is given).
- The disk loading times are not reproduced. The original also waits for a flag that the music sets (`$9A88`) before "press fire" is shown, which the port replaces by showing it when the credits are over.
