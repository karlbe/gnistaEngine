# Enemies (`$508`, `$ACA`, op66-77)

Status: ported in `pkg/game/enemies.go` and the script engine. The addresses are hunk-relative. The trace tests against the original (`trace_enemy_contact`, `trace_get_shot`, `trace_shoot_enemy`) agree step by step.

## Slots and actors

The player is slot 0. The enemies get slots 1 to 3 (actor data `$8170 + $30*k`), and the lift cabin and the explosive charge share slot 4. An enemy is a slot with a script, a position and a record (`+8`): five script numbers (main script, script after a shot or contact, two death scripts, hurt script) and hit points (2).

## The spawner (`$508`)

Runs before the actor loop every frame, and stands still while the player is on stairs (`$C34`).

- **Triggers.** The table at `$A738` has one byte per cell in the level matrix (indexed with `$4EE`). Bit 0 marks a trigger, bit 1 "armed" (set at the start of `$496`, cleared when it fires). Bits 2 to 6 choose what follows the first enemy.
- **The first enemy** (`$522`): when the player stands on an armed cell, an enemy is started in the first free slot from the left or right edge of the view, depending on which way the player faces. The templates (24 bytes: script, then the enemy record) are at `$5EF8`/`$5EC8` (first enemy right/left) and `$5E80`/`$5E38` (waves), and the template alternates for every enemy (`$C17`).
- **Waves.** Bits 2 to 5 choose one of four waiting lists (3, 6, 8 or 15 more enemies, with delays of 20 to 80 frames); bit 6 starts a single enemy. The next wave enemy (`$74E`) appears when the delay has run out, if the block to the right of the player's cell allows it (bit 0 in `$A628`) and a slot is free. A player who stands still gets a random side (`$C2C`).

## Contact (`$ACA`)

An enemy outside the view is removed (`$B74`). An enemy on the screen sets `$C1C` (blocks stairs, lift and doors, because op40 ignores "up" then) and `$9A88` (stops the background music). If an enemy comes closer than 32 pixels it catches the player: the player's script is replaced by `$4E5A`/`$4EC4` and the controls stop.

## Shots

- **The player shoots** (op68-71): the nearest living actor in slots 1 to 3 on that side, at any height, is hit. The shotgun (weapon 1) kills at once, otherwise the hit points are reduced and the enemy changes to its hurt script. At zero it is killed and its death script starts. The shot costs a round (op78: from the magazine, reload from the reserve or go to the script's empty branch).
- **The enemy shoots** (op76): if no other enemy is already aiming, `$1BFC` is set. The player's script (op66) takes the damage: the health counter `$1C05` goes down and becomes 3 again when a hit is lost (`$1C02`). If the hits have run out the player dies. The counter starts at 0 (the original never sets it), so the first hit always costs a hit.
- **Dead enemies** (op77) stay in the frame: the bob is drawn one last time without being saved and becomes part of the background, see `docs/re/foreground.md`.
