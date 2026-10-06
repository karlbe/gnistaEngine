# Sound effects

Status: the effects are ported (`pkg/audio`). The music is described in `docs/re/music.md`.

## The effect table (`$6432`): verified

20 records of 10 bytes: data pointer (4), length in **words** (2), Paula period (2), volume (1), padding. The table is zeroed in the file and built at start-up by `$61B0-$6430`. All data pointers point into the data of the `NSISound` file (after the 6-byte header, `$64FA` + 6) plus an offset. The offsets chain in the file (record 1 starts where record 0 ends), which confirms that the length is in words. The port interprets the routine at run time (`audio.ParseTable`) instead of copying the table.

Sample rate = 3,546,895 / period (PAL). Volume 0 to 64.

| Id | Used by | Remark |
|---|---|---|
| 0, 4 | Shots (op78 on channel 0) | the same data, different period |
| 11-13 | Footsteps (the op79 list `$20C2`) | channel 0 |
| 16-18 | Stair steps (the op80 list `$2114`) | channel 0 |
| 5-10, 19 | Op82/83/84 (channels 1 to 3) | shouts, death, explosion and so on; exactly which is which is a **hypothesis** |
| 14, 15 | The lift (op82 + op89 / op86) | 14 loops, 15 ends |

## Channels and playback: verified from the code, the playback behaviour is interpreted

The scripts drive Paula directly in two steps:

1. `$64FE` / `$652C` / `$655A` / `$6588` (op78/81, 82, 83, 84) switch off the channel's DMA and load the registers: pointer, length, period, volume. Nothing is started.
2. A later op switches DMA on: op85-88 (`$2184-$21C0`) switch DMA on and set the length to 1 word, so the sample plays **once** and the channel goes quiet in a one-word loop. Op89 (`$21D4`) only switches DMA on, so the sample **loops** until the channel is loaded again (the lift motor).

Channels 0 and 3 go to the left, 1 and 2 to the right (the Amiga standard).

## The port

- `game` only records commands (`AudioCmd`: Load, Start, StartOnce) in order, per tick (`State.Sounds`). The game state is not touched.
- `pkg/audio.Mixer` has four channels, nearest-neighbour resampling (like Paula, without a filter) and 16-bit stereo. `cmd/game` sends the commands after every tick. `-mute` switches the sound off.
- Deviations: the commands are applied at the pace of the tick, not on the scan line. The word loop after a one-shot sample counts as silence. The Amiga's low-pass filter is missing.
- Corrected at the same time: the start position of the op79/op80 lists is set by the init code `$3B0`, not by a record in the file.
