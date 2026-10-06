# The HUD panel (`NSIMenu`, `$7864`, `$761C`)

Status: ported in `pkg/render/hud.go`. The look has been compared with a screenshot from the original (`assets-local/uae/ingame.png`).

The panel is the picture `NSIMenu` (320 x 200, but only the top 72 lines are used) and is shown from screen line 128 below the play area, with a palette of its own that the game changes (`$7296`). Text is written with the 8x8 font `NSIAscii` through `$6A2C`, with a glyph spacing of 6 pixels.

| Part | Routine | Contents |
|---|---|---|
| Explosive charges | `$78B6` | one digit, `$502` |
| Magazines | `$7974` | one digit per weapon (`$4F6 + weapon*4 + 1`) |
| Weapons | `$2690`, `$26BA`, `$13AC` | icons for weapons 2 and 3 once they are picked up, and the selected one has a red frame (F1 to F3) |
| Cards | `$26E4`-`$27CC` | one box per card in the colours 1, 2, 2, 3 and `$B` |
| Floor | `$78F8` | `21 - floor(viewY/16/4)` |
| Hits | `$7936` | `9 - $1C02` |
| Clock | `$761C`, `$7706`-`$77FC` | 23:MM:SS in digit glyphs from the code's tables (`$7846`, `$7850`) |

The clock is counted down by `$761C` after every frame: 50 frames per second, 25 minutes from 23:35:00 to 24:00:00, when the outcome is set to explosion. The clock stands still in rooms and during pause.

The ammunition, health, cheat and pause indicators and the minimap that the port also draws are not in the original and are behind `-improvements`.
