# The lift (`$16A8`, op44, op48-63)

Status: ported in `pkg/game/script` and verified in the emulator (`trace_lift_up`) and with the playthrough (`docs/playthrough.md`: ride up and down several floors). The card logic is read from the code.

## How to ride

1. **Call.** Up on a lift tile (`$29`/`$2A`, one per floor) calls `$16A8` (`vm.callLift`). Cards are required: the thirds of the level (block columns 0-2, 3-5, 6-8) need card 0, 1 and 2 respectively (bits in `$505`); further to the right none is needed. The cabin is put in slot 4 with script `$455E` and comes to the player's floor (about 160 frames). If it is already in the right place it is restarted at `$46A6`.
2. **Get in.** A new "up" when the cabin has come starts the entry scripts (85-87). There op44 (`$427B`, `$4420`) reads the stick: **up** starts the ride upwards, **down** downwards, but only if there is a lift tile four rows above or below. Otherwise the script waits.
3. **Ride.** The ride scripts (93 up, 94 down) move the player and the cabin one floor (64 pixels) at a time. At the end of each floor op62/op63 look: if fire is latched (op61) you get out, otherwise the ride goes on as long as there are more lift tiles.
4. **Get out.** The exit script (89) animates the exit (about 100 frames) and ends by turning left (`op39`, `op43`), so the player always comes out facing left. With `-improvements` it ends facing right instead if you chose right (see `docs/improvements.md`).

## Details to know

- Op61 looks for fire once every few frames and not at all in the first 36 or so frames of each floor, so a short press is often missed in the original. Fire has to be held down during a floor for the ride to stop at its end. With `-improvements` the press is buffered and left and right count as well.
- Enemies on the screen block the lift (`$C1C`), like all upward movements.
- The cabin is a single bob (`$8128`), and `LiftBusy` (`$17C6` bit 0) is set from the call until script 60 empties slot 4, but not while the player is riding.
