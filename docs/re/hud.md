# HUD-panelen (`NSIMenu`, `$7864`, `$761C`)

Status: portat i `pkg/render/hud.go`. Utseendet är jämfört med originalets skärmbild (`assets-local/uae/ingame.png`).

Panelen är bilden `NSIMenu` (320 x 200, men bara de översta 72 raderna används) och visas från skärmrad 128 under spelytan, med en egen palett som spelet ändrar (`$7296`). Text skrivs med 8x8-fonten `NSIAscii` via `$6A2C`, med glyfavståndet 6 pixlar.

| Del | Rutin | Innehåll |
|---|---|---|
| Sprängladdningar | `$78B6` | en siffra, `$502` |
| Magasin | `$7974` | en siffra per vapen (`$4F6 + vapen*4 + 1`) |
| Vapen | `$2690`, `$26BA`, `$13AC` | ikoner för vapen 2 och 3 när de plockats upp, och den valda har röd ram (F1–F3) |
| Kort | `$26E4`–`$27CC` | en ruta per kort i färgerna 1, 2, 2, 3 och `$B` |
| Våning | `$78F8` | `21 - floor(viewY/16/4)` |
| Träffar | `$7936` | `9 - $1C02` |
| Klocka | `$761C`, `$7706`–`$77FC` | 23:MM:SS i sifferglyfer ur kodens tabeller (`$7846`, `$7850`) |

Klockan räknas av `$761C` efter varje bildruta: 50 bildrutor per sekund, 25 minuter från 23:35:00 till 24:00:00, då utfallet sätts till explosion. Klockan står stilla i rum och under paus.

Ammo-, hälso-, fusk- och pausindikatorerna och minikartan som porten också ritar finns inte i originalet och ligger bakom `-improvements`.
