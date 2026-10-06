# Skriptmotorn (aktörer, animation, rörelse)

Översikt och bakgrund finns i `README.md`. Den här filen samlar de tekniska detaljerna. Adresser är hunk-relativa i `ns`.

Status per påstående: **verifierat** (bekräftat i emulatorn), **kod** (utläst ur disassemblyn men inte provat), **hypotes**.

## Verktyg

- `py tools/scriptdis.py ops` listar opkoder, hanteraradresser och antal operandbyte.
- `py tools/scriptdis.py scripts assets-local/scripts.txt` disassemblerar alla skript i skripttabellen. Utdata ligger lokalt eftersom det är originaldata.

Operandräkningen är automatisk: hanteraren följs linjärt, och både läsningar via `(a5)+` och överhopp med `adda/addq #n,a5` räknas. Det senare behövs eftersom de villkorade hoppen läser sina operander via `a0` och hoppar över dem. Alla 170 skript avkodas utan någon ogiltig opkod. De villkorade hoppen till `$18BE` och op37:s loop (via `$10F8`) behandlas som "fortsätt om villkoret är falskt". Bara op40 och op41 (stora hanterare med egna slutdelar) stoppar fortfarande disassemblern.

Användning i skripten (antal förekomster): op00 5 591, op22 895, op27 542, op02 320, op16 310, op23 223, op24/25 120 vardera, op03 116, op01 111, op32/33 96 vardera.

## Aktörsloopen (`$C36`)

**Kod + verifierat.**

- 5 aktörsplatser à 30 byte från `$2B28` (räknare i `$C7C`, startvärde 4).
- Plats `+$0C`: skriptpekare (absolut adress) som laddas i `a5`. Plats `+$10`: pekare till aktörens data, som laddas i `a4`. En plats där `+$10` är 0 hoppas över.
- Efter tolken skrivs `a5` tillbaka till `+$0C`. Nästa gång fortsätter skriptet där det slutade.
- I spelläge från savestate används bara plats 0 (spelaren, aktördata på `$8140`). Övriga platser är tomma.

## Tolken (`$C7E`)

**Kod.**

Tolken hämtar opkodsbyten, slår upp hanterarens adress i en tabell med 4 byte per opkod (på `$21EC`) och hoppar dit. Hanteraren läser sina operander ur skriptströmmen och hoppar tillbaka till tolken.

91 opkoder (0–90). Tabellen slutar där värdena inte längre är kodadresser.

## Skripttabellen (`$2BBE`)

**Kod.** 170 långord med absoluta pekare till skript. Skripten ligger i kodhunken från `$2E6A`. Opkod 34 (`goto n`) hoppar till skript *n* via tabellen.

## Kända opkoder

| Opkod | Operander | Betydelse | Status |
|---|---|---|---|
| 0 | – | Vänta en bildruta: lämna tolken, fortsätt här nästa bildruta | **verifierat** (≈50 per sekund när spelaren går; mätt till 44/s plus 5 hopp som inte räknades, på 3 s) |
| 1, 2, 3 | 2 | Sätt aktörens två bildnummer (`$4(a4)`, `$1C(a4)`), räknat från en bas (`$EA0`/`$EA2`) och en riktningsfaktor (`$4F2`). Op2 och op3 lägger till 116 (`$74`) respektive ett annat tillägg, troligen andra riktningar eller varianter | kod; innebörden är hypotes |
| 7 | 1 | Övre bild = *n* + 116 + bas `$EA0` x riktning `$4F2` | kod |
| 10 | 1 | Övre bild = *n* + 622 (`$26E`) | kod |
| 13 | 1 | Undre bild = *n* + 254 (`$FE`) + bas `$EA2` x riktning | kod |
| 16 | 2 | Övre och undre bild = *n* + 394 (`$18A`) | kod |
| 17 | 1 | Övre bild = *n* + 116 | kod |
| 20 | 1 | Undre bild = *n* + 394 | kod |
| 21 | 2 | Sätt bildbaserna `$EA0` och `$EA2` | kod |
| 26 | 2 (signerade) | Övre spritedelens position = aktörens position (plats `+0/+2`) + (dx, dy) → aktörsdata `+0/+2` | kod |
| 27 | 2 (signerade) | Samma för undre delen → aktörsdata `+$18/+$1A`. Förekommer före varje bildbyte i gångskripten (t.ex. −4, −6, −16) | kod |
| 22–25 | – | Flytta aktören 1 pixel höger/vänster/upp/ned (båda spritedelarna och platsens position). Vid passerad tile-gräns läses sensorerna om (se nedan) | kod |
| 35 | 1 | Anropa skript *n* (gosub). Återhoppsadressen sparas i `$1130` (en nivå) | kod |
| 37 | 2 (*antal*, *n*) | Räknad loop: goto *n* sammanlagt *antal* gånger i följd, fortsätt sedan. Räknaren ligger i `$117E` (`$FF` = ingen loop pågår) | kod; verifierat (trace, gång vänster) |
| 38 / 39 | – | Vänd höger/vänster: läs sensorerna `$16–$1A` runt aktörens egen ruta åt det hållet (inte `$1B`), nollställ/sätt bit 1 i `$506` | kod; verifierat (trace) |
| 43 | – | Återgå till sparad skriptposition (`$17E0`) | kod |
| 61 | – | Om fire: sätt spärrflaggan `$1A24` | kod |
| 62 / 63 | – | Om spärrflaggan är satt: nollställ den. Annars, om aktören står på tile `$29` (hissen): starta hisskorgen i plats 4 (`$2BA0`) med eget skript och byt själv skript | kod |
| 64 | 2 (*a*, *b*) | Fire: goto *a*. Om senast tryckta tangent (`$69AD`) är F1–F3 (`$50–$52`): fortsätt. Om ingen riktning: goto *b* | kod |
| 65 | 2 (*a*, *b*) | Fire: goto *a*. Nedåt: goto *b* | kod |
| 66 | 5 (skada, *a*, *b*, *c*, *d*) | Om aktören träffats (`$1BFC` ≠ 0): dra skadan från en räknare (`$1C05`, startar på 3) och välj ett av fyra skript beroende på från vilken sida träffen kom och om räknaren tog slut | kod; innebörd delvis hypotes |
| 75 | – | Nollställ platsens byte `$1C` och starta om aktörens första skript | kod |
| 78 | 2 (*n*, ljud) | Skott: per vapen finns 4 byte på `$4F6 + vapen*4` (skott i magasinet, magasin, magasinsstorlek, HUD-flagga). Tomt magasin: ladda om från reserv (sätt HUD-flagga), eller goto *n* om inga finns. Dra ett skott och spela ljudet (fortsätter in i op81) | kod |
| 79 | – | Spela nästa ljud ur en nollterminerad cyklisk lista (`$20C2`, position i `$20CE`). Används för fotsteg. Efter ett varv spelas första ljudet två gånger, precis som i originalet | kod |
| 81 | 1 | Spela ljud *n* via `$64FE` | kod |
| 90 | – | Sätt utfallet `$21EA`=2 (explosionsslutet) och avsluta bildrutan | kod |
| 82 / 83 | 1 | Ljudeffekt *n* från tabellen på `$6432` (10 byte per post) via `$652C` respektive `$655A`, troligen starta/stoppa | kod; innebörd hypotes |
| 85 / 86 / 87 | – | Ljudkanal 0/1/2: slå på DMA och sätt längden till 1 ord, alltså tysta kanalen efter aktuellt sampel | kod; innebörd hypotes |
| 28 | 3 | `dx` → `$8F5A`, `dy` → `$8F5C`, tredje byten → `$8F5F`. Markerar rörelse (`$8F60`=1) | kod |
| 29 | – | Nollställ `dx`, `dy`, `$8F5E/$8F5F` och rörelseflaggan | kod |
| 30 / 31 | – | `dx` = +1 / −1, `dy` = 0, rörelseflaggan nollställs | kod |
| 32 / 33 | – | `dx` = 0, `dy` = −1 / +1, rörelseflaggan nollställs | kod |
| 34 | 1 | `goto` skript *n* (via `$2BBE`) | kod; används för att kedja och loopa gångcykeln (sett i emulatorn) |
| 40 | 12 (skriptnummer) | **Spelarens styrning i stillastående** (`$11DC/$1384`). F1–F3 byter vapen (`$4F2`=0–2, kräver bit i `$504` för F2/F3), markerar vapnet i HUD-paletten (`$7296`) och startar om vapnets viloskript (`$2E6A`/`$30F8`/`$33FC`), varefter bildrutan slutar. Annars, i tur och ordning: nedflagga (`$18BC`) → operand 10. Mellanslag (`$40`) på tile `$26–$29` med laddningar kvar (`$502`) → räkna ned, operand 6 (placera sprängladdning). Höger och sidosensor ≥ 11 → operand 0. Vänster → operand 1. Upp (om inte `$C1C`): tile under fötterna `$26–$28` → rutin `$23CC`; sidosensor `$0B–$0F` → operand 2; två rader upp `$21–$25` → operand 3; annars rutin `$16A8` (hissen). Ned: `$C1C` → operand 9; snett nedanför `$1B–$1D` → 7; under `$1E–$20` → 8; annars 9. Fire → operand 11. Sätter även `$506` (bit 0/1 = påbörjad gång höger/vänster), `$C34` (upp/ned) och `$17DC` (egen adress för op42) | kod; gång höger + väggstopp **verifierat** (trace) |
| 41 | 12 | **Spelarens styrning vänd vänster** (`$1442/$15EA`), spegelbild av op40: sätter bit 1 i `$506`, sparar sin adress i `$17E0`, går framåt (vänster) bara om sidosensorn ≥ 11, höger vänder (operand 0). Upp: sidosensor `$16–$1A` (i stället för `$0B–$0F`). Ned: `$1A` i `$13–$15` (i stället för `$1B–$1D`). Viloskript efter F1–F3: `$2EE2/$3170/$3474` | kod; verifierat (trace vänster, trappa upp) |
| 42 | – | Tillbaka till senaste op40 (`$17DC`) | kod; verifierat (trace) |
| 43 | – | Tillbaka till senaste op41 (`$17E0`) | kod; verifierat (trace) |
| 44 | 2 (*a*, *b*) | Våningsbyte: anropar `$FFC` (aktörens ruta i tilebufferten). Styrspak upp och tilen 4 rader ovanför (−240) är `$29` eller `$2A`: goto *a*. Styrspak ned och tilen 4 rader nedanför (+240) är `$29`/`$2A`: goto *b*. Annars fortsätt. `$29/$2A` är hissen, en tile per våning (se "Hissen" nedan) | kod |
| 45 | 1 (*n*) | Om styrspak höger och sensor `$19` ≥ 11: goto *n* | kod; stopp vid vägg **verifierat** |
| 46 | 1 (*n*) | Om styrspak vänster och sensor `$19` ≥ 11: goto *n* | kod; stopp vid vägg **verifierat** |
| 47 | 1 (*n*) | Om styrspak ned och sensor `$1B` ≥ 11: sätt `$18BC`=1 och goto *n* | kod |
| 48–51 | – | Hisskorgen (plats 4, aktördata `$8128`): starta skript `$4943`/`$49B2`/`$4A21`/`$4A92` | kod |
| 52 / 53 | – | Sätt aktörens båda bobs status (`+6`, `+$1E`) till 1 resp. 2 (2 = dölj) | kod |
| 54–57 | – | Hisskorgen: status 2; y −64 och status 1; y +64 och status 1; status 1 | kod; innebörd hypotes (en våning = 64 px) |
| 58 / 59 | – | `$17C6`: nollställ bit 0 och sätt bit 1 / sätt bit 0 | kod |
| 60 | – | Nollställ `$17C6` och töm plats 4 (hisskorgen borta), avsluta bildrutan | kod |
| 68 / 69 | – | Skott höger/vänster (hitscan): närmaste levande aktör (`+$10` ≠ 0, `+$1C` = 0) i plats 1–3 på den sidan, oavsett höjd. Träff: vapen 1 dödar direkt, annars minskas träffpunkterna i fiendeposten (`+8` → `+$14`) och fienden byter till skadeskriptet (`+$10`); vid 0 sätts `+$1C`=1 och dödsskriptet (`+8`) startas. `$1F44` nollställer `$1BFC` om det var den träffade | kod; utan träff verifierat (trace fire); träff ej portad |
| 80 | – | Som op79 men med listan på `$2114` (position i `$2120`), troligen fotsteg i trappor | kod; verifierat (trace trappa) |

Gemensamt slut för de villkorade hoppen: `$18BE` läser operandbyten och gör `goto` via `$2BBE`. Spelarens skript fungerar alltså som en tillståndsmaskin, ungefär "stå still; om höger och fritt, byt till gå-höger".

## Hissen (`$16A8`)

**Kod; att `$29/$2A` är hissen är en stark hypotes** (kartan har dem en våning isär i kolumn 489 med samma ram, och walkthroughen nämner hissar).

Op40/op41 anropar `$16A8` när man trycker upp och inget annat upp-fall gäller. Först `$16D8`: om vyns blockkolumn (`$8F72`) är 0–2, 3–5 eller 6–8 krävs bit 0, 1 resp. 2 i `$505` (korten, som sätts när man plockar upp föremål och visas under "ELEVATOR/DOOR CARDS" i HUD:en). Saknas kortet händer ingenting. Längre åt höger krävs inget kort. Därefter, om spelaren står på `$29/$2A` och `$17C6` bit 0 inte är satt:

- `$17C6` bit 1 satt: hisskorgen (plats 4) startar om på `$46A6`, och spelaren fortsätter med op40:s operand 5.
- Annars: sätt bit 0, placera korgen i plats 4 på spelarens position − (8, 22) med aktördata `$8128` och skript `$455E`. Spelaren fortsätter med operand 4.

Korgens skript använder op48–60 och fler okartlagda opkoder (bl.a. op89), så hissen fungerar inte i porten än.

## Rörelse och scrollning per bildruta (`$8888`)

**Kod + verifierat (trace trappa).** `$8F5E` är en nedräkning: när den inte är 0 minskas den och inget annat händer. Annars laddas den om från `$8F5F`, vyn flyttas med dx/dy (`$892C`), och om bit 0 i `$8F60` inte är satt nollställs dx/dy. Op28 sätter alltså en bestående rörelse med fördröjning (gång: `op28 1 0 0`, trappa: `op28 255 255 2` = ett steg var tredje bildruta), medan op30–33 bara gäller en bildruta.

## Aktörens position och sensorer

**Kod + verifierat.**

- Platsens byte `$14`/`$15` är aktörens tile-position i vyn. `$FFC` räknar ut aktörens ruta i **tilebufferten på `$9118`** (60 tiles bred, 16 rader): `$9118 + ($14 + $8F76) + ($15 + $8F78) * 60`.
- Platsens byte `$18–$1B` är **sensorer**: tile-ID från rutorna runt aktören, som fylls i av hanterarna runt `$F2A–$FF4` och `$1190–$11CA`. Vänd höger: två rader upp (`−$78`), rutan till höger (`+1`), raden under och två steg åt sidan (`+$3E`), två rutor till höger (`+2`). Vänd vänster: samma sak med `−1`, `−2` och `+$3A`.
- **Kollisionsregel: tile-ID < 11 blockerar.** Ingen separat kollisionstabell behövs för väggar. **Verifierat:** när man går höger från start stannar figuren vid x=8112 med `$19`=0, och åt vänster vid x=7920 med `$19`=6. Gångbron är 12 tiles lång.

Övriga opkoder är okartlagda. Deras hanteradresser och operandantal finns i `scriptdis.py ops`.

## Rörelse per bildruta (`$892C`)

**Kod + verifierat.**

- `$8F56 += dx` (vyns och spelarens x-position i världen) och `$8F7A += dx` (pixel inom tilen, 0–15).
- När `$8F7A` slår runt justeras tile-kolumnen inom blocket (`$8F76`), blockkolumnen (`$8F72`) och **`$4EE`** (spelarens positionsindex, se nedan). Samma sak gäller y via `$8F58`, `$8F7C`, `$8F78`, `$8F74`.
- Bitar i `$8F6C` markerar att en tile-gräns passerats åt vänster/höger/upp/ned. Kartritaren använder dem för att rita in nya kolumner och rader.
- Uppmätt: när man går höger ökar `$8F56` med 1 per bildruta. När man släpper styrspaken fortsätter figuren till nästa jämna 16-pixelsgräns. Vid byte av riktning dröjer det ca 0,6 s innan figuren börjar gå (vändning). **Verifierat** med `tools/uae/uaectl.py`.

## Attribut per position

**Kod, hypotes om innebörd.** På `$23D0` används `$4EE` som index i en bytetabell på `$A9F4`. Värdet x 12 pekar ut en post på 12 byte i en tabell på `$ACAC`, och postens flaggor styr vad som händer (t.ex. `btst #5,1(a0)`). Tabellerna ligger direkt efter kartmatrisen (`$A333`, 29 x 26 byte) och 0/1-tabellen på `$A5A5`. Troligen är det spelets kollisions- och interaktionsdata (golv, trappor, dörrar, hissar).

## Sensorerna läser via vyn, med en bildrutas fördröjning

**Verifierat (trace).** `$FFC` räknar ut aktörens ruta som *vyns* tile-position (`floor(view_x/16)`, där `view_x` = `$8F56`) plus platsens `$14/$15`. Vyn scrollas av `$892C` **efter** aktörsloopen i varje bildruta. Sensorer som läses i en bildruta ser därför vyn från förra bildrutan. För spelaren (fast på skärmruta (10, 6), världs-x = vy-x + 160) ger det basen `floor((x − dx)/16)`, inte `floor(x/16)`. Med `floor(x/16)` gick porten en tile för långt innan den stannade vid väggen.

## Go-porten av motorn (`pkg/game/script`)

- `Program` (kodhunken + skripttabellen), `VM` med 5 `Slot`, `Globals` och ett `Env`-gränssnitt för styrspak, tangent, tiles (världskoordinater), hjälparaktör, träff och ljud.
- `VM.Frame()` kör alla aktiva platser till nästa väntan och scrollar sedan vyn som `$8888`. En plats som når en okartlagd opkod blir stående på den, och resten av bildrutan körs ändå.
- Okartlagda opkoder ger `ErrUnimplemented`, och okartlagda originalrutiner (`$23CC`) ger `ErrUnmapped`. Inget gissas.
- Implementerade opkoder: 0–3, 4/5/14/15 (vänta), 6–13, 16–47 (utom 41:s okartlagda `$23CC`-fall), 61–66 (66 bara utan träff), 68/69 (bara utan träff), 77–83, 85–87, 90. Dessutom `$16A8` (hissanrop) och `$8888` (scrollning).
- **Jämförelse mot originalet:** `TestAgainstOriginalTrace` kör alla `assets-local/uae/trace_*.json` (inspelade med `py tools/uae/capture_script.py <riktning> <sek> [förflyttning]`). Testet startar porten från samma läge, kör lika länge som inspelningen och jämför följden av ändringar i (skriptposition, bild +0, bild +$18, dx, x). Upprepade steg i originalspåret slås ihop, eftersom inspelningen ibland läser minnet mellan aktörsloopen och scrollningen. Alla steg stämmer: höger 134, vänster 95 (vändning, op37–39, op41), ned 20, fire 150 (op68, op78), upp från start 1, trappa upp 201 (`right:2.5,left:0.15` först; op80, `$8888`).

Obs: Go-namnen `Upper`/`Lower` på aktörens två bobs är missvisande. Posten `+0` (`Upper`) ritas på aktörens y (benen), och posten `+$18` (`Lower`) 16 pixlar högre upp (överkroppen).

## Ritning (`$7D7E`, `$803A`)

**Kod + verifierat mot skärmbild.** Bob-listan börjar på `$8128` (hisskorgen), följd av spelarens två poster `$8140/$8158`. Varje post är 24 byte: x, y (världspixlar), bildnummer (direkt index i `NSIBobs`), status (`+6`: 0 = tom, 2 = dölj, annars rita). Skärmposition = (x − `$8F56` − 16, y − `$8F58`). Den synliga spelytan är 320 × 128 (mätt i emulatorns skärmbild: kartan syns till och med rad 128, HUD:en börjar på rad 129). Blitten ritar 4 skärmplan: plan *p* får nästa lagrade plan om bit *p* i BHDR-postens första taggbyte är satt, annars rensas det under masken. Läsningen fortsätter förbi färgplanen in i maskplanet, så en 3-plansbob med tagg `$4F` får masken som plan 3 (färg 8–15). `amiga.Bank.Bob` avkodar så.

## Nästa steg

1. Hissen: korgens skript (`$455E`, `$46A6`, `$4943`…) och opkoderna 48–60, 84, 88, 89. Spela in när spelaren har kört hissen.
2. Kartlägg `$23CC` (upp på tile `$26–$28`, troligen dörr) och resten av opkoderna, särskilt de som fiender och gisslan använder (op66 med träff, 67, 70–76, 84, 88, 89).
3. Fiender i plats 1–3: var de startas, deras poster (`+8`: skript och träffpunkter) och träffdelen av op68/69.