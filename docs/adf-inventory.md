# Inventering av originaldisketten (ADF)

Steg 1 för att undersöka en ren port från originalbinärerna. Underlaget är `assets-local/raw/*.adf`, uppackat med `go run ./cmd/extract` till `assets-local/adf/`. Underlaget är originalets, så det som läses ur det hålls lokalt i `assets-local/`.

Status per fil: **Verifierat** betyder att formatet är tolkat med header eller chunk-struktur. **Hypotes** betyder att det är en gissning från byte-mönster och inte kontrollerat mot kod.

## Disketten

- Volymnamn `PGI`, OFS (bootblock `DOS\0`, flagga 0), 880 KB, 901 120 byte.
- Enda startsteget är `s/startup-sequence`, som innehåller bara `ns`. Allt startas alltså från en enda binär.
- Disketten har 33 filer, ca 565 KB data. Banor och grafik är uppdelade i separata filer som laddas från disk, och koden ligger i `ns`.

## Sammanfattning

| Kategori | Filer | Format |
|---|---|---|
| Körbar | `ns` | AmigaOS hunk-binär, 68000 |
| Helskärmsbilder | 11 st med tvåbokstavsnamn, `NSILoader`, `NSIMenu`, 10 st `NSIRoomN` | IFF ILBM, 320x200, 5 bitplan (32 färger) |
| Spritebanker | `NSIBobs`, `NSIIcons` | IFF-liknande `FORM JBOB` (eget format) |
| Ljud | `DAS`, `DBY`, `IAZ`, `NSISound`, `NSIMusicSound` | Rå 8-bitars-ljud med 6 byte header (hypotes) |
| Speldata | `RoomData`, `NSIA` | Okänt eget format |
| Font | `NSIAscii` | 256 tecken x 8 byte, 1 bitplan (hypotes, stark) |
| System | `devs/system-configuration` | AmigaOS Preferences-struct (232 byte) |

## Körbar fil

### `ns` (55 980 byte) — verifierat

- Hunk-fil (`0x3F3`) med 2 hunkar:
  - Hunk 0: CODE 47 816 byte, med 2 024 RELOC32-poster.
  - Hunk 1: BSS 4 byte.
- Ingen symboltabell och ingen debuginfo. Disassemblering blir alltså anonym, utan funktionsnamn.
- Inga läsbara strängar i koden. Filnamn (`NSIRoom1` m.fl.) byggs troligen upp dynamiskt (t.ex. `NSIRoom` + nummer), så de syns inte som hela strängar. Filnamnen i diskettens rot är förmodligen det enda "API" som koden använder mot data.
- Den är liten (47 KB kod) för ett helt spel, så den är hanterbar att disassemblera och porta.
- Spelet nämner inte AmigaDOS-bibliotek i klartext. Det talar för att det skriver direkt mot hårdvaran (blitter, copper, Paula) och använder egen trackloader. Det måste bekräftas med disassemblering. Det påverkar porten starkt, eftersom hårdvarurelaterad kod måste ersättas.

## Helskärmsbilder, IFF ILBM — verifierat

Alla är 320x200, 5 bitplan, mask=2 (transparent färg), komprimering 1 (ByteRun1), 32 färger i CMAP. De innehåller också chunkarna `DPPV` (Deluxe Paint-perspektiv, 104 byte) och 4 st `CRNG` (färgcykling). Färgcyklingen finns alltså i data och bör kunna återskapas i porten.

| Fil | Storlek | Transparent färg | Tolkning (hypotes) |
|---|---|---|---|
| `NSILoader` | 25 910 | 0 | Laddnings-/titelbild |
| `NSIMenu` | 6 736 | 0 | Menybild |
| `NSIRoom1` | 14 848 | 0 | Bakgrund/rum 1 |
| `NSIRoom5` | 10 594 | 0 | Rum 5 |
| `NSIRoom9` | 12 932 | 13 | Rum 9 |
| `NSIRoom10` | 12 932 | 0 | Rum 10 |
| `NSIRoom11` | 13 264 | 13 | Rum 11 |
| `NSIRoom12` | 12 924 | 0 | Rum 12 |
| `NSIRoom13` | 11 986 | 0 | Rum 13 |
| `NSIRoom14` | 14 946 | 0 | Rum 14 |
| `NSIRoom15` | 11 026 | 0 | Rum 15 |
| `BB` | 16 024 | 0 | Helskärmsbild |
| `DO` | 4 280 | 2 | Helskärmsbild |
| `EN` | 19 762 | 8 | Helskärmsbild |
| `ET` | 4 560 | 0 | Helskärmsbild |
| `EX` | 27 904 | 0 | Helskärmsbild |
| `HC` | 12 876 | 0 | Helskärmsbild |
| `IT` | 5 618 | 0 | Helskärmsbild |
| `PF` | 2 884 | 0 | Helskärmsbild |
| `ST` | 6 858 | 0 | Helskärmsbild |
| `TA` | 8 338 | 0 | Helskärmsbild |
| `WT` | 7 784 | 0 | Helskärmsbild |

Anmärkningar:

- Rummen som finns är 1, 5 och 9–15. Rum 2–4 och 6–8 saknas på den här disketten. Antingen genereras de (se `RoomData`), eller så finns de på en annan diskett, eller så delas bilder mellan rum. `NSIRoom9` och `NSIRoom10` har samma storlek (12 932 byte) men olika `trans`, vilket tyder på nära släktskap.
- De tvåbokstavsfilerna (`BB`, `DO`, `EN`, `ET`, `EX`, `HC`, `IT`, `PF`, `ST`, `TA`, `WT`) har oklar roll. De har samma ILBM-layout som rummen. Det är en öppen fråga om de är fasta skärmar, rumsbakgrunder eller något annat. Renderar vi dem till PNG avgörs det snabbt.

## Spritebanker, `FORM JBOB` — delvis verifierat

- `NSIBobs` (109 428 byte) och `NSIIcons` (40 204 byte).
- IFF-liknande container med chunkarna `BHDR` och `BODY`. Det är ett eget format, troligen från en Amiga-sprite/BOB-verktyg (Blitter Objects).
- `BHDR` verkar vara en tabell med poster om 24 byte, med 4 tecken namn/tagg (`OTRL`, `WTRL`, `CTRL`). Det ger ca 649 poster i `NSIBobs` (15 576 / 24) och 362 i `NSIIcons` (8 688 / 24). Taggarna syns också i strängar som `Adr.Mask`.
- Den första posten pekar på offset 0, och nästa post börjar vid 0x80. Det stämmer med 4 bitplan à 32 byte (16 rader x 16 pixlar). **Hypotes:** 16x16-sprites med planar data och en maskplan. Det måste bekräftas när vi skriver en konverterare.
- Detta är spelets huvudgrafik, alltså spelare, fiender, bomber, gisslan och tiles.

## Ljud — hypotes

Alla fem filerna börjar med samma mönster: en 32-bitars längd (big endian), ett 16-bitars ord, sedan data som ser ut som 8-bitars PCM (signerad, Paula-stil).

| Fil | Filstorlek | Längdfält | Ord efter längd | Notering |
|---|---|---|---|---|
| `DAS` | 4 006 | 4 000 | `0x3E1C` | Kort effekt |
| `DBY` | 2 006 | 2 000 | `0x396C` | Kort effekt |
| `IAZ` | 5 006 | 5 000 | `0x3DB8` | Kort effekt |
| `NSISound` | 82 806 | 82 800 | `0x2710` | Troligen en bank med flera effekter |
| `NSIMusicSound` | 140 258 | 140 252 | `0x2710` | Troligen musik eller instrumentsamplingar |

- Längdfältet + 6 = filstorleken i samtliga fall. Det är verifierat.
- Ordet (`0x2710` = 10 000, de andra runt 14 700–15 900) är troligen samplingsfrekvens eller periodvärde. Det är inte bekräftat.
- `NSISound` och `NSIMusicSound` börjar med nästan identiska bytes, men det kan vara en slump. Om de är banker med flera ljud behövs en indextabell som troligen ligger i `ns`.

## Speldata

### `RoomData` (20 480 byte) — okänt

- Exakt 20 KB = 20 x 1 024. Börjar med många upprepade `0x73`, som troligen är ett "tomt" tile/värde.
- Låg entropi (4,45) och 31 % nollor. Det stämmer med tilemaps eller banlayout.
- Det är den troligaste källan till banlayout, objektplaceringar (bomber, gisslan) och rumsövergångar. Hela filen ska tolkas, och det är kärnan i en port.

### `NSIA` (10 072 byte) — okänt

- Börjar med `02 05 00 7f 40 04 00 04 01 e0 10 40 ...` följt av upprepade poster som `00 00 01 e0 08 36`. Mönstret med fasta 6 byte tyder på en lista med poster (t.ex. animations- eller objektdefinitioner).
- Låg entropi (3,42). Rollen är oklar. Kan vara en animationsdefinition, en objektdefinition eller en tabell till `NSIBobs`.

### `NSIAscii` (2 048 byte) — hypotes, stark

- Exakt 256 x 8 byte. Första glyfen (`ff 81 bd a5 a5 bd 81 ff`) är en ruta med mönster, alltså en 8x8-bitmapsfont. Antagligen en teckentabell som även innehåller grafiska tecken.

## System

### `devs/system-configuration` (232 byte) — verifierat

- Storleken 232 byte är AmigaOS `struct Preferences`. Den innehåller bl.a. strängen `generic` (skrivarnamn) och inställningar för tangentbord och skärm. Den har ingen spelfunktion och behöver inte portas.

### `s/startup-sequence` (3 byte)

- Innehåller `ns`. Det enda som startar spelet.

## Slutsatser för portbarheten

1. **Gynnsamt:** bara en binär (`ns`, 47 KB kod). Allt annat är data. Grafiken ligger som standardformat (IFF ILBM) som är enkelt att konvertera, och färgcyklingen finns i datat.
2. **Medelsvårt:** spritebankerna (`JBOB`) och ljuden är egna format. Båda ser dock enkla ut, med planardata och rå PCM.
3. **Största arbetet:** `RoomData` och `NSIA` samt hela spellogiken i `ns`. Banformat och objektdata måste tolkas, och logiken kräver en disassembly. Det saknas symboler, så det går via spårning av dataflöde mot filnamnen och sprite-index.
4. **Risk:** om `ns` skriver direkt mot Amiga-hårdvaran (troligt) måste all ritning, ljud och input ersättas. Det är OK för en omskrivning i TypeScript där vi bara återskapar logiken, men det är inte en emulering av binären.
5. **Alternativ till portning:** emulera hela binären i webbläsaren (WebAssembly-baserad Amiga-emulator med ADF). Det är betydligt enklare tekniskt men ger ingen "ren port" och löser inte rättighetsfrågan. Det passar inte kravet på egen kod.

## Öppna frågor / nästa steg

- Konvertera alla ILBM-filer till PNG och se vad de tvåbokstavsfilerna är.
- Disassemblera `ns` (t.ex. med Ghidra, som har 68000-stöd och kan läsa Amiga-hunkar) och kartlägg de funktioner som läser in datafilerna.
- Avkoda `JBOB` fullt ut och dumpa spritesheets till PNG.
- Avkoda `RoomData` och `NSIA`.
- Avgör om `NSISound` och `NSIMusicSound` är banker och hur de indexeras.
- Gränsen för "ren port": en omskrivning som återskapar logiken, inte att köra originalbinären.

## Tillägg: vad bilderna visar (konverterade med `tools/ilbm2png.py`)

Konverteringen skrevs till `assets-local/png/` (gitignored). De tvåbokstavsfilerna är **berättelse- och spelslutsskärmar**, inte rumsbakgrunder:

| Fil | Innehåll |
|---|---|
| `NSILoader` | Titelbild med huvudkaraktärer och logotyp |
| `PF` | "Tryck fire för att starta" |
| `IT` | Intro: förutsättningen, ett terrorhot mot världens största oljereserv |
| `EN` | Intro: transportplan och uppställd trupp |
| `HC` | Intro: helikopter i silhuett |
| `ST` | Berättelse efter kraschen, ankomst till riggen |
| `WT` | Vinstskärm med epilog |
| `DO` | Dödsskärm (text) |
| `ET` | Slutskärm: explosion (text) |
| `EX` | Game over: svampmoln |
| `BB` | Bomb-avvecklingsskärm (diagram över bomben med ledningar, plus tänger) |
| `TA` | Okänd grafik, färggrann siluett. Troligen en bild som bygger på färgcykling eller transparens |
| `NSIMenu` | Spelets HUD (vapen, magasin, sprängladdningar, kort, våning, träffar, klocka) |
| `NSIRoomN` | Rumsbilder i spelvyn, med en ram i nederkant för HUD. Spelet är alltså en **sidovy per rum**, inte isometriskt |

## Tillägg: sprite- och kartformat (verifierat med `tools/jbob2png.py` och `tools/levelmap2png.py`)

**`JBOB` (NSIBobs, NSIIcons).** `BHDR` är en tabell med poster om 24 byte, big endian: `u32` bodyoffset, `u32` maskplansoffset, `u32` planstorlek, `u16` flaggor, `u16` bytebredd+2, `u16` höjd, `u16` bredd i ord, 4 tecken tagg. Färgplanen ligger i följd i `BODY` och maskplanet sist. Antalet färgplan är `(maskoffset - offset) / planstorlek`. Formatet är bekräftat eftersom alla 649 poster i `NSIBobs` uppfyller alla konsistenskontroller.

- `NSIBobs`: 649 sprites, mest 16x16 med 3 plan (574 st), plus 27 st 32x38. Innehåller figurernas animationsbilder i många poser (gång, ligga, skjuta m.m.), samt hissar och dörrar.
- `NSIIcons`: 240 giltiga poster, mest 16x16 med 3–4 plan. Det är kartans **tileset** (väggar, golv, trappor, rör, dörrar, möbler, föremål). Posterna 240–361 är identiska tomposter (`Adr.Mask`, tagg `CTRL`) och ska ignoreras.
- Taggarna `OTRL`, `WTRL`, `CTRL` och `_TRL` är okända. Eventuellt anger de objekttyp.
- Färgdjupet (3 plan = 8 färger) betyder att spritebankerna delar en palett med spelvyn, och att paletten inte ligger i filen. Den måste hämtas från rumsbilden eller koden.

**`RoomData`.** *Rättelse:* första tolkningen (en tile-karta på 80 x 256) var fel. Filen är **256 block om 20 x 4 tiles** (80 byte per block, radvis, 1 byte per tile, index in i `NSIIcons`), alltså 320 x 64 pixlar per block. Blockens ordning i banan bestäms inte av filen utan av en separat kartmatris i `ns` (se nästa avsnitt).

## Tillägg: första disassembly av `ns` (`tools/disasm_hunk.py`, Capstone 5, 68000)

Utdata skrivs till `assets-local/ns.asm` (gitignored, härledd från originalet).

- 47 816 byte kod gav 13 036 rader, varav 1 284 kunde inte avkodas. 3 380 `ori.b` är nästan säkert nollifylld **data inbäddad i kodhunken**, som linjär disassembly tolkar som kod. Hunken blandar kod och data, så en vanlig linjär disassembly räcker inte. Den behöver rekursiv traversering från kända ingångspunkter.
- **Hårdvara direkt.** Koden skriver till custom-chipsen (`$DFF096` DMACON 22 gånger, `$DFF09A/9C` interrupts, ljudkanalerna `$DFF0A0–$DFF0D8`, Copper och bitplan-pekare) och till CIA (`$BFE001` joystick/eldknapp, `$BFD100` diskenhetsstyrning). Det är alltså en **trackloader med egen hårdvarukod**. Ritning, ljud och input måste skrivas om från grunden, och OS-anropen är få.
- Ingångspunkten är en huvudloop på adress 0 som anropar sju initieringsrutiner och sedan växlar på ett tillståndsord på `$21EA` (1, 2, 3 = olika lägen, troligen intro, spel och slut). Spelstate ligger i absolut adresserad minnesdata (`$21EA`, `$7234`, `$7C5C`, `$9A88`), och `ns` har 2 024 relokeringar som pekar på dem.
- Väntlooparna är räknarloopar (`subi.l #1,d0 / bne`), alltså **CPU-tidsberoende** och ska ersättas med riktiga timers.

**Hur väl går det att dekompilera?** Disassemblyn är ren och läsbar. Spellogiken ser ut att vara handskriven assembler utan kompilatorgenererad struktur, så en C-liknande dekompilering från Ghidra blir hjälpsam men ytlig. Den är ofta bra för kontrollflöde och datatillgång, men variabelnamn och typer måste vi sätta själva. Med 47 KB kod är det hanterbart. Rekommendation: rekursiv disassembly i Ghidra (kräver JDK 17+), sedan manuell kartläggning av tillståndsmaskinen, inputhanteringen och kartläsningen.

## Tillägg: rekursiv disassembler i Python (`tools/recdis.py`), ingen Ghidra behövs

Skriptet följer språng och anrop från adress 0 och använder 2 024 relokeringar som extra ingångspunkter. Resultatet skrivs till `assets-local/ns_rec.asm` och täcker 64 % av koden (resten är data eller oanalyserad kod). Kod och data är nu åtskilda. 17 indirekta `jsr -n(a6)` är OS-bibliotekanrop som skriptet inte följer.

Fynd:

- **Filnamn finns som klartext** i datadelen, med `df0:`-prefix: `NSIascii`, `RoomData`, `NSISound`, `IAZ`, `DAS`, `DBY`, `NSIBobs`, `NSIIcons`, `NSIMenu`, `NSIRoom1/5/9–15`, `BB`, `HK`, `NSILoader`, `ST`, `WT`, `DO`, `ET`, `EX`, `EN`, `HC`, `IT`, `TA`, `PF`, `NSIA`, `NSIMusicSound`. Alla finns på disketten **utom `HK`**, så den filen refereras men saknas här. Rum 2–4 och 6–8 refereras aldrig, så spelet har bara de 11 rumsbakgrunderna.
- **Bibliotek:** `graphics.library`, `dos.library` och `intuition.library`. Filerna läses via DOS (`jsr -$1e/-$24/-$2a(a6)` = Open/Close/Read). Ritning och ljud går ändå direkt mot hårdvaran.
- **Spelets utfall** ligger i ordet `$21EA`. Det är 0 medan spelet pågår. 1, 2 och 3 laddar `DO` (död), `ET` (slut: explosion) respektive `WT` (vinst). Det motsvarar skärmarna jag såg i bilderna.
- Kodens datastrukturer ligger i absolut adresserad data, så porten får bygga ett eget spelstate i stället för att återskapa minneslayouten.

## Tillägg: kartmatrisen i koden (verifierat, `tools/levelmap2png.py`)

Banan ligger **inte** i någon datafil. Den är en statisk bytematris inbyggd i kodhunken i `ns`, och `RoomData` är bara ett bibliotek av block som matrisen pekar ut. Adresserna nedan är hunk-relativa (filoffset = adress + 36).

**Matrisen**

| Egenskap | Värde |
|---|---|
| Adress | `$A333` (filoffset 41 815) |
| Format | 1 byte per cell, radvis. Cellen är ett **block-ID** (0–255) |
| Radlängd (stride) | **29** celler. Värdet ligger i ordet `$910E` |
| Antal rader | **26**. Värdet ligger i ordet `$9110`. *Rättelse:* först antogs 208 rader (till hunkens slut), men efter rad 26 (`$A5A5`) följer andra tabeller |
| Efter matrisen | Från `$A5A5` en tabell med samma bredd och nästan bara 0/1. **Hypotes:** en flagga per block (t.ex. tänt/besökt). Därefter fler tabeller med annan struktur |
| ID 0 | Tomt block, som ritas som bakgrund |

**Block.** Ett ID `n` pekar på `RoomData[n * 80 .. n * 80 + 79]`. Det är 20 x 4 tiles, radvis (20 byte per rad, 4 rader), och varje tile är ett index i `NSIIcons` (16x16 pixlar). Ett block är alltså 320 x 64 pixlar. Det är en skärmbredd av en våning, vilket stämmer med att våningarna ligger 64 pixlar isär i spelet. Konstanterna ligger i data: `$9112`=20 (bredd), `$9114`=4 (höjd), `$9116`=80 (byte per block).

**Hela banan** är 29 x 26 block, alltså 9 280 x 1 664 pixlar. I tvärsnitt består den av tre byggnader av olika höjd som förbinds av en marknivå, med våningar, trappor, hissar och rum. Visa den med `go run ./cmd/viewer` (zooma ut med mushjulet).

**Hur koden ritar** (rutinen `$8DE0`, med hjälprutinen `$8E50`):

1. Kartpositionen räknas om från pixlar till block i `$8EDA`. `$8F56` (x) och `$8F58` (y) är vyns pixelposition. Den delas med blockstorleken (320 respektive 64) och ger blockkolumn `$8F72`, blockrad `$8F74` och en återstående tile-offset `$8F76`.
2. `$8DE0` beräknar adressen i matrisen: `$A333 + $8F72 + $8F74 * $910E`.
3. Den läser block-ID:n för ett fönster på 3 x 4 block (`$94D8`=3, `$94DA`=4) och anropar `$8E50` för varje.
4. `$8E50` kopierar blockets 4 rader a 20 tiles från `RoomData` (pekaren `$94E6`, 20 480 byte) till en **tilebuffer** som är 60 tiles bred (`$94DC`=60, alltså 3 block) och 16 tiles hög. Bufferten används för scrollning. Skärmen visar 20 tiles åt gången.
5. Tilebufferten ritas sedan till skärmen med blittern (se rutinen vid `$8D9C`, som skriver till `$DFF040–$DFF058`).

**Startposition.** Vid `$03A0` sätts `$8F56`=7984 och `$8F58`=1360. Det motsvarar blockkolumn 24 och blockrad 21 (7984 / 320 = 24,95 och 1360 / 64 = 21,25). Det är troligen spelarens startvy, men det är ännu inte kontrollerat mot screenshoten.

**Ännu okänt**
- Vad ordet `$9110` (=26) betyder. Det ligger bredvid stride-värdet och kan vara synlig bredd eller en kartgräns.
- Vilka ID som bara är dekor och vilka som är hissar, dörrar eller trappor. Kollision och interaktion kräver förmodligen en egen tabell, eftersom block bara innehåller tile-ID.
- Var bomber, gisslan och fiender placeras. De ligger inte i matrisen, och `NSIA` är ett av de möjliga ställena.
- Paletten. Screenshoten har en 16-färgs gameplaypalett (plus 2 extra) där ordningen skiljer sig från rumsbildernas CMAP. Jag använder screenshotens palett för tilesen, och färgerna i kartan ser rimliga ut. Var spelet själv sätter paletten är inte kartlagt.

Rumsbilderna är sidovyer med bakgrund och karaktär, i ett spel med rum, dörrar och hissar. Spelet är alltså en sidovy, inte isometriskt.

## Tillägg: skärm, palett och hur tiles ritas (verifierat mot emulatorn)

Verifierat genom att läsa copperlistan och skärmens bitplan ur chip-RAM i WinUAE (`tools/uae/uaectl.py`), med start från savestate i spelläge.

**Skärmen i spelläge** (copperlistan ligger på `$FB6C` i chip-RAM i den körningen):

- `BPLCON0=$4000`: **4 bitplan**, ett playfield, 16 färger. Copperlistan sätter 6 planpekare, men bara 4 används.
- Planen är 46 byte breda per rad (`DDFSTRT=$30`, `DDFSTOP=$D0` ger 42 byte, plus modulo 4), alltså 368 pixlar. Den extra bredden används för mjukscrollning (`BPLCON1`).
- Spelvyn har en palett med 16 färger (de övriga 16 registren är 0). På rad 172 byter copperlistan planpekare och palett för HUD:en.

**Paletter.** Spelet har 7 palettabeller om 32 färger (12-bitars `$0RGB`) i kodhunken: `$7256`, `$72D6`, `$7316`, `$7356`, `$7396`, `$73D6`, `$7416`. Den aktiva sätts med `move.l #tabell,$7234.l`, och rutinen `$723C` bygger copperlistans `COLOR00–31`. Spelläget använder `$7256` (sätts på `$6950`). Ordningen är densamma som i rumsbildernas CMAP. WinUAE:s egna PNG-skärmbilder sorterar om paletten, så de går inte att använda som källa för index.

**Tiles ritas med en planmask** (rutinen `$8D66`, anropad per tile från `$8D32`):

1. Tile-ID x 24 ger tilens `BHDR`-post i `NSIIcons`. Tile-ID:t är alltså direkt ett postnummer.
2. Datapekaren är postens bodyoffset plus `BODY`-basen.
3. `d7` = **postens byte 20, alltså första tecknet i "taggen"**. Det är en planmask, inte ett namn.
4. För skärmplan 0–3: om bit *p* i masken är satt kopieras tilens nästa lagrade 32-byteplan (16 rader x 1 ord) till skärmplan *p*. Annars töms planet.
5. Blittern skriver hela rutan utan mask, så map-tiles är **ogenomskinliga**.

Taggarna som förekommer: `'W'`=`$57` (plan 0–2), `'_'`=`$5F` (plan 0–3), `'O'`=`$4F` (plan 0–3), `'C'`=`$43` (plan 0–1), `'D'`=`$44` (bara plan 2), `'@'`=`$40` (inga plan, alltså färg 0). Samma lagrade data kan alltså ge olika färger beroende på masken. Det förklarar varför samma motiv ser grått ut i korridorer och blått i rum.

Med den här modellen stämmer 98,5 % av pixlarna i spelvyn mot emulatorns skärm. Resten är sprites (figuren m.m.), som ritas ovanpå. Spelvyns vänsterkant ligger 16 pixlar till höger om vypositionen i `$8F56` (den extra scrollkolumnen).

**Hypotes:** spritebankens första taggbyte (t.ex. `'O'` i `NSIBobs`) fungerar troligen på samma sätt för figurer, men de ritas med en annan rutin och mask. Det är inte verifierat. `pkg/amiga` avkodar därför bobs med den gamla tolkningen (färgplan + maskplan), och tiles via `Bank.Tile`.
