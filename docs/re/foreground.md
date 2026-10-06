# Foreground: why the player walks behind railings

Status: **verified** against the screenshot from the original (`assets-local/uae/ingame.png`) and the code, ported in `pkg/render`.

## The mechanism

The game screen has **five bitplanes**, but the colours only use the first four (the tile pictures only have indices 0 to 15). The fifth plane is a **foreground flag**.

- The tile drawer (`$8D66`) copies a stored plane to screen plane *p* if bit *p* is set in the tile's plane mask (the first tag byte in the JBOB record). Tiles with bit 4 set (the tags `$57` and `$5F`, 88 tiles) store a fifth plane and thus get foreground pixels. Floor edges, railings and stair railings are such tiles.
- The bob drawer (`$803A`) first builds a mask in `$83B0` (`$80EA`, minterm `$B50`): **the bob's mask AND NOT the screen's fifth plane**. Then the usual cookie-cut blit is done with that mask. Bobs are therefore never drawn over pixels with the foreground flag, and that goes for all bobs (the player, enemies, the lift cabin and the corpses left behind).
- Colour registers 16 to 31 in the extracted palette are black, but the original picture shows the foreground pixels in their usual colours. The port therefore draws them with indices 0 to 15 and uses the fifth plane only as a mask.

## The port

`amiga.Bank.TileForeground(i)` reads the fifth plane. `render.drawMap` fills a foreground mask for the view and `drawBobPal` skips those pixels.

## Sound on stairs (deviation)

The stair steps (effects 16 to 18, op80) are about four times weaker than the footsteps (effects 11 to 13) in the original data, so they are practically inaudible. The port has a gain (`stairGain`) for them, now 1.0, which is the original level (a factor of 4 matched the footsteps but was too loud, and 2 was too loud as well) (`stairGain` in `cmd/game/main.go`, `Entry.Gain`). It is **not** original behaviour. Ladders (the scripts from `$3BAA`) have no sound op at all in the original and are silent in the port too.
