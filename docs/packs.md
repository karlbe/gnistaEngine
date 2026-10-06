# Spelpaket: egen bana, egen grafik och egna ljud

Den här filen beskriver hur man gör en egen version av spelet med ett **spelpaket** (pack): en katalog med egna filer i öppna format (PNG, JSON, WAV, Tiled) som ersätter originalets data bit för bit. Det som paketet inte har hämtas från originalet, så ett paket kan börja litet (några skärmar) och växa tills inget original återstår.

Status och vad som återstår för ett helt publicerbart spel står längst ned ("Vad som inte är fritt än").

## Snabbstart

```
go run ./cmd/packtool new mygame      # skapar ett startpaket (egen palett, platshållartiles, en liten bana)
go run ./cmd/packtool check mygame    # läser paketet som spelet gör och rapporterar fel
go run ./cmd/game -pack mygame        # kör spelet med paketet
```

`packtool new` skapar ett startpaket. **Lägg aldrig original-grafik eller något som är spårat eller målat över originalet i ett paket.**

`go run ./cmd/packtool inventory` listar vad originalet har (storlekar, antal) och vad ett paket kan ersätta. Den behöver `assets-local/` och skriver bara mått och antal.

## Paketets innehåll

Allt är valfritt. Det som saknas tas från originalet.

| Fil | Vad |
|---|---|
| `pack.json` | namn, palett, texter, volymer (se nedan) |
| `tiles.png` | kartans tiles: rutnät av 16x16-rutor, tile-id = index rad för rad (högst 256) |
| `tiles_fg.png` | samma rutnät; ritade pixlar hamnar **framför** figurerna (räcken, golvkanter) |
| `sprites/N.png` | figurbild nummer N, valfri storlek, genomskinlig där figuren inte är |
| `sprites/map.json` | `{"335": {"same": 333, "flip": true}}`: bild 335 är bild 333 spegelvänd |
| `screens/NAMN.png` | helskärmsbilder: `NSILoader` (logotyp), `IT`, `TA`, `PF`, `ST` (titel och intro), `ET`, `EX`, `WT`, `HC`, `EN` (slut), `hud` (panelen) |
| `rooms/N.png` | bilderna bakom dörrar, bildnummer N (se "Rum") |
| `font.png` | 256x64: 128 tecken à 16x8, 16 per rad. Pixlar som är ritade och inte svarta räknas |
| `level.tmj` | banan, gjord i Tiled (se "Banan") |
| `scripts/*.gs`, `program.json` | egna skript och deras ingångar; utan dem körs originalets |
| `sounds/N.wav` | ljudeffekt N, mono eller stereo, 8 eller 16 bit, valfri frekvens |
| `music/title.wav`, `music/game.wav` | inspelad musik: titeln spelas en gång, spelmusiken loopar |

`pack.json`:

```json
{
  "name": "Mygame",
  "palette": ["#0b1020", "..."],        // spelvyns färger, upp till 256 (tiles, sprites, rumsbilder)
  "flash": ["#...", "..."],              // explosionens blixt, 32 färger
  "title_palettes": {"7316": ["..."]},   // färgbyten för silhuettbilden (TA), annars bildens egen palett
  "credits": ["RAD 1", "RAD 2"],         // texten som rullar över titeln
  "messages": ["", "TEXT\nNY RAD"],      // texter i rum, index = meddelandenummer
  "clock": {"top": [..10..], "bottom": [..10..]},  // teckenkoder för klockans siffror
  "sound_volumes": {"3": 40},            // volym 0-64 per effekt (standard 64)
  "music_volume": 0.6,                   // 0-1
  "empty_tile": 115                      // tile för tomma rutor i kartan (standard 0x73)
}
```

### Färger

- Har paketet en `palette` matchas varje pixel i `tiles.png` och `sprites/` mot den närmaste färgen. Har det ingen används originalets färger.
- Originalets tiles och sprites som paketet inte ersatt färgas om till närmaste färg i paketets palett, så blandade paket ser sammanhängande ut.
- Helskärmsbilder och rumsbilder har sin egen palett: ett indexerat PNG behåller sina index, ett vanligt PNG får en palett av sina färger. Har bilden fler än 256 färger minskas de automatiskt (median cut). Spara gärna som indexerat PNG om du vill styra färgerna.
- Panelen (`hud`) visar bara index 0-15. Index 13-15 är vapenrutornas ramar (markeras vid byte) och 4-9 vapenikonerna som ändrar färg när man plockar upp ett vapen. Behåll de rollerna.
- Titelns rullande text ritas som "udda index" ovanpå silhuettbilden (`TA`): bilden får bara använda jämna färgindex i textområdet nederst.

### Banan (`level.tmj`)

Gör banan i [Tiled](https://www.mapeditor.org/) med `tiles.png` som tileset (16x16, firstgid 1). Spara som JSON (`.tmj`) och ange tile layer format CSV. Packtool `new` skapar en färdig karta att utgå från.

- **Tile layer** `tiles`: en tile per ruta. Tomma rutor (gid 0) blir `empty_tile`. Kartan fylls ut till hela block: **block är 20 x 4 tiles** och kartan kan ha högst **256 olika block** (identiska block delas automatiskt). Det är den verkliga begränsningen: upprepa golv, väggar och trapphus.
- **Object layer** `objects`, med objekt av dessa klasser (punkter eller rektanglar, läget tas som övre vänstra hörnet):

| Klass | Egenskaper | Betydelse |
|---|---|---|
| `start` | – | där spelaren börjar. Det måste finnas minst 10 tiles åt vänster och 6 uppåt (vyn är 20 x 8 tiles och spelaren står 10 in och 6 ned) |
| `door` | `lock`, `card`, `weapon`, `mags1`–`mags3`, `charges`, `picture`, `message` | rummet bakom en dörrtile (id 0x26–0x28) på samma ruta. `lock`: kort som öppnar ("1,3"), `card`: kort som rummet ger (1-5), `weapon`: 2 eller 3, `mags*`: magasin 0-9, `charges`: sprängladdningar, `picture`: nummer på rumsbilden, `message`: nummer på texten |
| `spawn` | `wave` 0-4 | fiendeutlösare. 0 = en fiende, 1-4 = en fiende och sedan 3, 6, 8 eller 15 till |
| `wave_zone` | – (rektangel) | block i zonen tillåter att fiendevågor dyker upp |

- Egenskap på kartan: `lift_cards`, kommalista med kortbitar per blockkolumn ("1,1,1,2,2,2,4,4,4"), kort som krävs för att kalla hissen där. Tom = inga kort.
- **Celler.** Fiendeutlösare och dörrar hör till en *cell*: en region på 20 x 4 tiles räknad från spelarens position (vyn börjar 10 tiles till vänster och 6 ovanför spelaren). **En cell har högst en dörr.** En fiendeutlösare löser ut så fort spelaren kommer in i cellen, så lägg inte en dörr i samma cell som en utlösare om spelaren ska hinna använda dörren: fienderna spärrar upp-knappen så länge de syns.
- **Stopplägen.** Spelaren stannar bara på **varannan tile** räknat från starttilen (en gångcykel är två tiles). Dörrar, hissar och trappfötter ska därför stå på tiles med samma paritet som starttilen, annars går de inte att nå.

### Tile-id: vad spelet gör med varje id

Skripten och sensorerna tittar på tile-id:n. Ett eget tileset måste lägga rätt sorts tile på rätt id (guiden `guides/tiles_guide.png`, som `packtool new` skriver, visar alla id:n färgkodade):

| Id | Betydelse |
|---|---|
| 0x00–0x0A | solid: blockerar gång (väggar, golv man inte kan gå igenom) |
| 0x0B–0x0F, 0x13–0x15 | trappa som stiger åt höger ("/"), sedd från vänster |
| 0x16–0x1D | trappa som stiger åt vänster ("\") |
| 0x1E–0x20 | trappsteg nedåt (nedgång) |
| 0x21–0x25 | stege |
| 0x26–0x28 | dörr (en tile, spelaren står på den). 0x29 går också att lägga en sprängladdning på |
| 0x29, 0x2A | hiss, en tile per våning, **fyra rader (64 px) mellan våningarna** |
| övriga id | fria: bakgrund, scenerier, golvdekor. Alla id ≥ 0x0B som inte är ovan går att gå genom |

Alla id upp till 255 går att använda. Id:n som inte ritas i `tiles.png` hämtas från originalet. `packtool check` varnar för det.

### Figurer (sprites)

Spelet ritar varje figur som **två delar**, överkropp och ben, 16 x 16 px var, på bildnummer som skripten räknar ut: `n + förskjutning + bas × vapen`. Det betyder att de **bildnummer som originalets skript använder styr vilken pose som visas**. Originalet har 649 bilder: 622 stycken 16 x 16 (spelare, fiender) och 27 stycken 32 x 38 (hisskorg, tråd-markör m.m.).

Vad man gör nu:

- Rita de poser som behövs och peka ut alla bildnummer som ska se likadana ut med `sprites/map.json` (samma bild, eventuellt spegelvänd). Bilder som paketet inte har ritas av originalet (omfärgat till paketets palett).
- För ett helt eget spel skrivs egna skript med egna bildnummer. Då behövs bara de bilder skripten använder.

Hotspot är övre vänstra hörnet, precis som i originalet. Bildens mått får vara vilka som helst.

### Rum

Bilden bakom en dörr väljs av `picture`. Några nummer har en egen betydelse i reglerna och ska behållas: **0** = tomt rum (visas för redan besökta rum), **5** = första hjälpen (fyller träffarna, besöks aldrig "klart"), **8** = bombrummet (fire lämnar, mellanslag startar trådklippningen), **9** = närbilden av bombens trådar. Trådklippningen använder fyra lägen i bilden (x ≈ 0x92, 0xA5, 0xC0, 0xD7 från vänster) och den fjärde tråden desarmerar. Övriga nummer är fria. Rumsbilder är 320 x 200, men bara de 128 översta raderna syns i spelvyn.

### Ljud och musik

`sounds/N.wav` ersätter effekt N. Effekterna är numrerade 0–19 i originalet (`packtool inventory` listar längd och frekvens); vilken som spelas när styrs av skripten (`docs/re/sound.md`). Spelet spelar dem på fyra kanaler (0 och 3 vänster, 1 och 2 höger, med mjuk stereo). Frekvensen avrundas till en period (3546895 / frekvens), så välj gärna 22050 Hz eller 44100 Hz.

`music/title.wav` och `music/game.wav` ersätter originalets spelade låtar. Titeln väntar på att musiken tagit slut; spelmusiken loopar, tystnar när en fiende syns och fortsätter sedan. Normalisera till omkring −6 dB, spelet multiplicerar med `music_volume`.

### Texter

Texterna är versaler i den teckenuppsättning som `font.png` har (ASCII). Radbrytning i rummen: `\n`.

## Arbetsgång för en egen bana

1. `packtool new mygame`, öppna `level.tmj` i Tiled.
2. Rita tiles i `tiles.png` (en ruta i taget; rutor du lämnar helt genomskinliga hämtas från originalet), följ id-tabellen ovan.
3. Bygg banan, placera `start`, `door`, `spawn`, `wave_zone`. Håll koll på paritet och celler.
4. `packtool check mygame`, sedan `go run ./cmd/game -pack mygame`. F4 (fusk) visar hela kartan på minikartan och F5 gör dig osårbar, vilket underlättar test.
5. Rita sprites och skärmar, lägg ljud och musik. Börja med det som syns mest: titel, panel, tiles.
6. Bomben och målet: bombrummet är ett rum med `picture` 8 och dörrlås som inget kort öppnar (`lock` 32), så dörren måste sprängas. Att desarmera bomben är vinsten. Det finns ingen annan vinstvillkor i reglerna än så och klockan; ett eget mål kräver ändrad kod.

## Vad som är fixerat i spelets regler (inte i paketet än)

- Klockan räknar ned och när den når noll går bomben av. Spelaren har 9 liv och 3 träffar per liv, och börjar med pistol (7 skott i magasinet, 5 magasin) och 2 sprängladdningar.
- Fienderna: tre platser åt gången, deras beteende, träffpunkter (2) och skott. Spawnern använder fyra mallar (första fiende höger/vänster, våg höger/vänster).
- Spelarens hastighet, hopp-, rull- och trappregler. Allt detta ligger i originalets skript.
- Det finns fem kort (hiss- och dörrkort), tre vapen (F1–F3).

## Utan originalet

Har paketet palett, bana och skript startar spelet utan några originalfiler alls: `go run ./cmd/game -assets /finns/inte -pack mygame`. Det som paketet inte har hämtas annars från originalet, så utelämnar man något (t.ex. panelen eller teckensnittet) ritas det inte. `packtool leakcheck <paket>` jämför ett paket med originalet och rapporterar kopior.

## Vad som inte är fritt än

Ett paket ger egen grafik, bana, ljud, musik, texter **och egna skript**. Ett paket med egna skript körs utan originalet, men då är det paketets ansvar att allt i det är eget. Språket för skripten beskrivs i `docs/script-friendly.md` och `docs/script-reference.md`.

Kvar att göra för ett publicerbart spel, i ordning:

1. Egen grafik, bana, ljud, musik och texter (paketet, klart att använda nu).
2. Egna skript och egna tabeller (`own-scripts.md`, steg 2–4).
3. Egen titel, namn och logotyper; originalets namn får inte användas.
4. Gå igenom att varje fil har ett ursprung och en licens, och ta bort allt som härrör från originalet.
