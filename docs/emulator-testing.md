# Automatiska tester mot originalet i WinUAE

Originalspelet i WinUAE är facit för port 1. `tools/uae/uaectl.py` startar och styr emulatorn utan GUI och utan att någon sitter vid datorn.

## Förutsättningar

- WinUAE 5.3: `E:\Spel\Amiga\WinUAE\winuae64.exe` (kan ändras med miljövariabeln `WINUAE`).
- Kickstart 1.3 (34.5, okrypterad, 256K): `E:\Spel\Amiga\Kickstart v1.3 r34.5 (1987)(Commodore)(A500-A1000-A2000-CDTV)[!].rom`. Filen `Kickstart 1.3.rom` (512K) ska **inte** användas, eftersom den ger en ROM-nyckelvarning.
- Konfiguration: `assets-local/uae/pgi.uae`. Det är en A500 med OCS, 68000, cykelexakt, 512K chip + 512K slow, spelets ADF i df0, inget ljud, `gfx_api=gdi`, joystick i port 2 på piltangenter + höger Ctrl (`kbd2`).
- Allt under `assets-local/uae/` (konfiguration, savestate, skärmbilder) är lokalt och committas inte.

## Kommandon

```
py tools/uae/uaectl.py boot              kallstart, tryck fire genom intro, spara assets-local/uae/ingame.uss
py tools/uae/uaectl.py launch [--fresh]  starta från ingame.uss (eller kallstart) och låt den köra
py tools/uae/uaectl.py watch [ADDR...]   koppla upp mot en körande WinUAE och skriv ut variabler när de ändras
py tools/uae/uaectl.py shot FIL.png      koppla upp och ta en skärmbild
```

`watch` utan adresser visar `$21EA` (utfall), `$8F56/$8F58` (vyposition) och `$8F72/$8F74` (blockkoordinater). Den fungerar medan man spelar själv, så att man kan se vad som händer i minnet när man t.ex. tar en hiss.

## Hur det fungerar

**Start.** WinUAE startas med `-f pgi.uae` och `-s nyckel=värde` för överstyrningar. Ett savestate laddas med `-s statefile=...`, eftersom flaggan `-statefile` ignoreras tillsammans med `-f`. Boot använder turbo-diskladdning (`floppy_speed=0`), vilket bara påverkar laddning, inte spelet. Varningsdialoger klickas bort automatiskt. Fokus lämnas tillbaka till det fönster användaren hade.

**Minne.** WinUAE mappar Amiga-adressrymden linjärt i sitt processminne (`host = bas + Amiga-adress`). Verktyget letar upp den körande, relokerade kodhunken i `ns` (filnamnstabellen plus samstämmiga relokeringar, så att orelokerade kopior i DOS-buffertar avvisas). Därefter läses och skrivs spelets variabler med samma hunk-relativa adresser som i `docs/re/`. Hunken laddas på olika adresser i olika körningar (t.ex. `$C08708`, `$C08738`), men sökningen hanterar det. Chip-RAM nås via samma bas, vilket är verifierat med ExecBase och `ChkBase`.

**Input utan fokus.** Tangenttryck via Windows går till det fönster som har fokus, och det fungerar därför inte. I stället pekar `take_control()` om spelets alla läsningar av joystick och eldknapp till två brevlådor i oanvänd chip-RAM (68000-användarvektorerna):

| Instruktion i `ns` | Ställen | Brevlåda |
|---|---|---|
| `move.w $DFF00C.l,Dn` (JOY1DAT) | `$235E`, `$2618` | `$3F0` (word, JOY1DAT-kodning) |
| `btst #7,$BFE001.l` (eldknapp port 2) | `$140`, `$15C`, `$178`, `$238E`, `$282C`, `$67C4`, `$681E`, `$68A2`, `$760E` | `$3F4` (bit 7, 0 = nedtryckt) |

Instruktionerna behåller opkod och längd, och bara adressfältet byts. Spelets logik påverkas alltså inte. `release_control()` återställer originalbytes. Görs alltid före ett savestate, så att det sparade spelet går att spela med vanlig joystick. `set_input(dirs, fire)` och `hold(dirs, fire, seconds)` skriver brevlådorna.

**I spelläge.** Inputrutinen `$235C` skriver varje bildruta en nollskild riktningsmask till `$23CA` (bit 0 ingen riktning, 1 upp, 2 höger, 3 ned, 4 vänster, 5 fire). Filens värde är 0, så `$23CA != 0` betyder att spelet har startat.

**Skärmbilder.** `PrintWindow` på emulatorfönstret. Med Direct3D blir bilden svart när fönstret saknar fokus, och därför används `gfx_api=gdi`.

## Exempel

```python
import sys; sys.path.insert(0, "tools/uae")
import uaectl, time
u = uaectl.UAE.launch(state=uaectl.STATE)
u.wait_hunk(30)
u.take_control()
x0 = u.word(0x8F56)
u.hold(["left"], seconds=1.5)
print("vyn flyttades", x0 - u.word(0x8F56), "pixlar")
u.screenshot("assets-local/uae/shots/test.png")
u.release_control()
u.kill()
```

## Skriptspår

`py tools/uae/capture_script.py <riktning> <sek> [förflyttning] > assets-local/uae/trace_<namn>.json` startar från savestatet och kör först förflyttningen (t.ex. `right:2.5,left:0.15`). När gubben har stått still i 1,5 s sparas startläget (plats 0 och motorns variabler), och därefter hålls riktningen medan varje ändring av skriptposition, bilder, dx och vy spelas in. `pkg/game/script/trace_test.go` kör alla `trace_*.json` mot Go-motorn.

Fiender skjuter: på våningen ovanför trappan vid bron blir gubben skjuten om han går vänster en stund (skript `$4F2E`). Undvik det i förflyttningar tills fienderna är portade.

En walkthrough (rutt genom hela banan, tredjepartstext) ligger lokalt i `assets-local/walkthrough/`.

## Begränsningar och nästa steg

- Tidsupplösningen för input är Python-sömn (millisekunder), inte bildrutor. För bildruteexakta tester behövs synk mot spelets VBlank, t.ex. genom att läsa en räknare som spelet ökar varje bildruta. Den är inte hittad ännu.
- Tangentbordskommandon i spelet (om sådana finns) går inte via brevlådorna.
- Ljudet är avstängt i konfigurationen.
