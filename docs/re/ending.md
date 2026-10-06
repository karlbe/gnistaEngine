# The ending (`$30`-`$130`)

Status: ported in `pkg/game/ending.go` and verified on screenshots of both endings. The timings were measured in the emulator.

When the game is over (`$21EA` is non-zero) the main loop switches off the sound (`$3A`: `$9A88` = 1 and DMA off) and shows a sequence. The outcome `$21EA` is 2 (explosion: time ran out, the player died or the wrong wire was cut) or 3 (the right wire). Outcome 1, which would show `DO`, is never set.

## Explosion

1. The game view in the flash palette `$73D6` for 95 frames (a delay loop of 500,000 iterations).
2. Black for 22 frames.
3. The picture `ET` (a page of text about the flash and the shock wave) for 380 frames.
4. The picture `EX` (a mushroom cloud with a game-over text) for 380 frames.

## Win

1. Black for 22 frames.
2. `WT` (the story of the bomb being defused), until fire is pressed.
3. `HC` (the aeroplane picture), until fire is pressed.
4. `EN` (the last page), until fire is pressed.

The pages that wait for fire go on as soon as fire is down, because the original reads the button every frame. Holding fire down therefore skips several pages.

After that the game starts again from the title. The disk loading times between the pictures (1 to 3 s) are not reproduced.
