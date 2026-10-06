# The playthrough (the end-to-end test)

`pkg/solve` plays the game with a bot, and `cmd/solve` records the inputs that win it. `TestPlaythrough` (`pkg/solve/playthrough_test.go`) replays the recording in a new game and checks that the game ends in a win.

## What the test proves

- The whole game can be played from start to end: cards, weapons, doors (including the one that has to be blown open), lifts, stairs, the bomb, the wire cutting and the ending screens.
- The simulation is deterministic. The replay only holds if every tick behaves exactly as it did when the recording was made (72,642 ticks).
- No unported opcodes are reached along the way.

The test does not compare against the original. It proves that the port is complete and stable, not that it is identical.

## The recording

`pkg/solve/testdata/playthrough.txt` contains only what the player pressed (a run-length coded list of `value:count`). It contains no game data and can be checked in. It is kept in step with the game by running `go run ./cmd/solve` again whenever the rules change (the test fails otherwise).

The recording uses the invulnerability cheat as its first input (F5), so it needs `-improvements` on. The cheat key is an ordinary input, so the replay is still just inputs.

## The bot

- **Macros.** The bot does one thing at a time from a standing position: take one or more steps right or left, up or down (stairs, ladder, door), ride the lift a number of floors, or lay a charge on a locked door and back away. Then it waits until the player stands still.
- **Fighting.** The bot shoots enemies as a reflex: it turns towards the nearest, picks the best weapon that has ammunition (the shotgun kills with one shot) and fires. The walking macros ignore enemies, because the bot is invulnerable and enemies only block stairs, lifts and doors. Those macros clear them first.
- **Search.** The search works on copies of the game (`State.Clone`) and is breadth first or A* with a map of the level as a hint. The map (`BuildGraph`, 6,000 states) is built once by a fully equipped player and saved in `assets-local/solve/graph.json`.
- **Plan.** First the cards and weapons are collected, and ammunition until the supply is enough (the last corridors send waves of enemies). Then the bot goes to the bomb, blows the door, cuts the right wire and sees the ending.

## Known limitations

- Pressing a direction in some waiting states (after shooting) only turns the player, so the result of a macro depends on the script state. The bot treats the script state as part of the state, and the search always runs on real states.
- The recording visits 28 of 63 rooms. Not all rooms fit in the game's 25 minutes with this route planning: the way to the bomb takes about 12,000 ticks and a room costs about 1,600 on average. The command `go run ./cmd/solve -reserve 0` records the shortest win instead (14 rooms, about 42,000 ticks).
- The recording ends with about 17 seconds left on the clock, so every change to the game's pace or rules means it has to be recorded again.
