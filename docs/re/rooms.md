# Rooms, doors and the bomb (`$23CC`, `$2864`)

Status: ported in `pkg/game/rooms.go`. The room records and the sequence itself are verified in the emulator (`TestFirstDoor`, the player goes through the first door). The room pictures and the messages are the original's and are read at run time.

## Doors

Doors are tiles `$26`-`$28` (one tile, one door; 63 of them in the level). Up on a door tile calls `$23CC` (`State.enterRoom`). The room is looked up through the level matrix cell of the **view's** block position (`$4EE`, not the player's): `$A9F4 + $4EE` gives a room number, which points out a 12-byte record in `$ACAC`. The cell for the view and for the player coincide in practice because the player stands 10 tiles into the view, but it is the view's cell that counts.

The room record: `+0` lock (bits from the card byte `$505`; any one of these cards opens it), `+1` flags (bits 0-4 a card, bit 5 visited, bit 6 weapon 2, bit 7 weapon 3), `+2..4` magazines per weapon, `+5` explosive charges, `+6` picture, `+7` message, `+8` blown.

- **A visited room** is shown as empty (picture 0, message 0).
- **A locked door** does nothing if the player lacks the cards and the door is not blown. 36 of the rooms have card locks; the bomb room has the lock `%100000`, which no card can open, so the door has to be blown.
- The room gives its items (card, weapon, magazines up to 9, charges up to 9), is marked as visited and shows the picture and the message. First aid (picture 5) refills the hits and is never marked as visited. Down leaves the room.
- The clock, the enemies and everything else stand still during the visit, because the original runs the whole visit inside the player's control opcode.

## Blowing

Space on a door tile (`$26`-`$29`) with charges left counts the charges down and starts script 6: the charge is laid out (it takes about 120 frames) and goes off shortly after. If the player is still standing on the charge's cell when it goes off he dies (op74); otherwise the door's "blown" byte is set and the door picture is stamped in as blown. The player must therefore move away as soon as the laying is done.

## The bomb

The bomb room (picture 8) has a sequence of its own: fire leaves the room, space starts the wire cutting. Four wires, a marker (`WireX`) and one reading of the stick every 50 frames: right and left move the marker, fire cuts. The fourth wire (`wireGood` = 3, the green one) defuses the bomb and gives outcome 3 (win); the others give outcome 2 (explosion).
