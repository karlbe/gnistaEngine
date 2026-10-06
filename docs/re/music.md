# The music (`$96DE`, `$94EA`, `$95EA`)

Status: ported in `pkg/audio/music.go`. The player's logic is read from the code and tested against the real files (number of patterns, note starts, the end). The sound has **not been compared with the original by ear**.

## What runs what

The game has two interrupts on the same vector (`$6C`, routine `$7566`): the copper interrupt runs the game's frame, and the vertical blank interrupt (`$75C8`) runs the music's tick (`$96DE`) 50 times a second, unless bit 0 of `$9A88` is set. That bit means "the song is over" or "stopped by the game".

## The files

| File | Contents |
|---|---|
| `NSIA` (10,072 bytes) | the song: 6-byte records ("events"), 100 patterns, four order lists |
| `NSIMusicSound` | the title song's instruments: ten samples |
| `IAZ`, `DAS`, `DBY` | the three short samples that the background track in the game uses |

## NSIA

The events come first (6 bytes each, up to `$2328`): flag byte, instrument, period (u16), length in rows, volume. A pattern is a run of events and ends with the first one whose flag byte is exactly 2 (`$9B5C`); that event is played too. Then come four order lists of 156 bytes each (one per voice, from `$24B8`), where each byte is a pattern number and `$FF` ends the list. All four are 16 entries long.

## The player

- Each voice has a countdown (`$9A82`-`$9A85`, start value 1), a pointer to the next event and a position in its order list.
- The player takes a step (a "row") every fifth tick. The counter `$9A87` goes 4, 3, 2, 1, 0.
- A voice that is not switched off (`$9A89`) counts down its note, or starts the next one when the countdown is 1: DMA for the channel is switched off, pointer, length and period are set from the instrument table and the event, the volume is set (the value 4 means 0), the countdown is loaded with the note's length and the voice is marked in `$9A86`.
- At the next tick `$96DE` switches on DMA for the marked voices and sets the length to 1 word, so each note is played **once** and goes quiet. (The same two steps as the sound effects.)
- An event with flag 2 moves the voice to the next pattern in the order list. Voices 0 and 3 check for `$FF`: then the song is over (`$97F0`). If it is the title song, `$9A88` bit 0 is set. If it is the background track, it starts over.

## The title song (`$94EA`)

Starts when the logo is done (`$66FC`) and runs for 96 seconds. The title waits for it to end (`$67C4`) before "press fire" is shown. The voices all start at the first pattern in their lists. The instruments are the ten in `NSIMusicSound`, and the table is built by `$9C52` (an offset and a length in words each; the last sample's length is 2 bytes longer than the file, which the original also reads past).

## The background track in the game (`$95EA`)

Called when a game begins (`$2A`). Voices 0 and 1 are switched off (`$9A89` = 3). Voices 2 and 3 start at position 18 in their order lists, with the instrument table replaced by IAZ, DAS and DBY (lengths 2500, 2000 and 1000 words). When it ends it starts over (`$9A8A`).

The enemy check (`$ACA`) clears `$9A88` at the start of every frame and sets it again as long as an enemy is visible on the screen, so the background track only plays when no enemy is visible. The game stops it altogether when the game is over (`$3A`). In rooms and during pause the interrupts do not run, and so neither does the music.

## The port

`audio.Tracker` is a pure state machine that drives four voices (`Voices`): `LoadEntry` corresponds to the register writes with DMA off and `Start` switches DMA on. `Mixer` implements `Voices`. `cmd/game` calls `Tick` once per frame (the title and the game). Without a sound device `audio.Silent` is used, so the title's wait for the end of the song works anyway.
