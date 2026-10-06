# Plan: port till Go (Windows först)

Mål: en trogen port av originalspelet till Go, som körs på Windows och använder originalets datafiler oförändrade under utveckling. Spellogiken återskapas från disassemblyn av `ns`. Ingen emulering av 68000-koden.

Underlag: [adf-inventory.md](adf-inventory.md).

## Principer

- **Originalfilerna läses direkt.** Spelet laddar `assets-local/adf/*` i originalformat (ILBM, JBOB, RoomData, ljud). Ingen konvertering i förväg, så det blir en enda sanningskälla. Kartmatrisen och andra tabeller som ligger i `ns` extraheras av ett verktyg till en egen datafil (`assets-local/extracted/`), så att spelet aldrig behöver läsa binären.
- **En asset-loader med utbytbar källa.** All läsning går via ett `content`-paket som pekar på en katalog. Byte till egna assets ska vara en konfigurationsändring.
- **Spelstate är ren data.** Simuleringen är deterministisk, med fast tidssteg på **50 Hz** (PAL, samma takt som originalets VBlank-loop). Rendering och ljud läser state men ändrar den aldrig. Det gör nätspel och repris möjliga senare.
- **Beteende dokumenteras före implementation.** Varje rutin vi kartlägger i `ns` beskrivs i `docs/re/` (vad den gör, variabler, konstanter). Go-koden skrivs från beskrivningen, inte genom att översätta assembler rad för rad. Det ger läsbar kod och håller isär "förstå" och "bygga".
- **Facit är originalet i emulator.** WinUAE med samma ADF används för att jämföra beteende (hastigheter, timer, skador) och skärmbilder.

## Teknik

- **Go + Ebitengine** (`github.com/hajimehoshi/ebiten/v2`). Det ger fönster, skalning, input (tangentbord och handkontroll) och ljud på Windows utan cgo, och fungerar även på macOS, Linux och webben (WASM) om det blir aktuellt senare.
- Intern upplösning **320 x 256** (PAL-skärm: 200 rader spelvy + HUD), skalad heltalsvis till fönstret.
- Bilder hålls som palettindex (8 bitar) i minnet och färgläggs vid ritning. Det behövs för färgcykling (`CRNG`) och palettbyten, som originalet använder.
- Ljud: originalets 8-bitars sampel spelas upp via en egen liten mixer med 4 kanaler (som Paula) ovanpå Ebitengines ljudström.

## Paketstruktur

```
cmd/
  game/          spelet
  viewer/        verktyg: bläddra bland bilder, sprites, block och karta
  extract/       verktyg: läs ut tabeller ur ns till assets-local/extracted/
pkg/
  amiga/         filformat: ilbm, jbob, sample, font, hunk
  content/       asset-loader (källkatalog via config), cache
  game/          spelstate och simulering (ren Go, inga Ebitengine-beroenden)
    world/       karta, block, tiles, kollision
    actor/       spelare, fiender, gisslan
    item/        bomber, kort, sprängladdningar, vapen
    flow/        speltillstånd: intro, spel, död, vinst, game over
  input/         abstraktion: Action-flaggor per tick (tangentbord, kontroll, repris)
  render/        ritar state: karta, sprites, HUD, skärmar, färgcykling
  audio/         mixer, effekter, musik
tools/           python-verktygen för reverse engineering (finns redan)
docs/re/         beskrivningar av kartlagda rutiner
```

`pkg/game` får inte importera `render`, `audio` eller Ebitengine. Det är det som håller state ren och testbar.

## Faser

Varje fas avslutas med något som går att köra och visa.

### Fas 0: grund (liten)

- `git init`, Go-modul, Ebitengine, tomt fönster med fast tidssteg.
- Lägg till `assets-local/extracted/` och ev. `cmd/*/`-binärer i `.gitignore`.

**Klart när:** ett tomt fönster öppnas och loopen tickar 50 gånger per sekund.

### Fas 1: filformat i Go (liten–medel)

- Porta de verifierade Python-tolkarna till `pkg/amiga`: ILBM (inkl. `CRNG`), JBOB, ljudformatet, 8x8-fonten.
- `cmd/extract`: läs hunk-filen, plocka ut kartmatrisen (`$A333`, 29 x 208), blockkonstanterna och fler tabeller efterhand.
- `cmd/viewer`: visa skärmbilder (med färgcykling), spritebankerna med index, block, och hela kartan med scrollning.
- Enhetstester för varje format (storlekar, kända värden, t.ex. att alla 649 JBOB-poster validerar).

**Klart när:** viewern visar hela banan med rätt färger och kan scrolla i den.

**Status (2026-10-05): klar.** `pkg/amiga` (IFF, ILBM med CRNG, JBOB med tile-planmask, ljud, font, hunk), `pkg/content` (loader med konfigurerbar rot), `pkg/game/world` (bana), `cmd/extract` (kartmatris, 7 paletter, startvy) och `cmd/viewer` (bilder med färgcykling, spritebanker, hela banan). Tilefärgerna är verifierade mot emulatorns skärm (98,5 % av pixlarna; resten är sprites). Bobs ritas ännu med den overifierade tolkningen.

### Fas 2: kartläggning av spelets kärna (stor, pågår parallellt med fas 3–6)

Det mest osäkra arbetet. Görs bit för bit i takt med att funktionerna behövs.

- Förbättra `tools/recdis.py`: en symbolfil (`adress → namn, kommentar`) som disassemblern läser in, så att listningen blir läsbar ju mer vi förstår. Följ hopptabeller för att öka täckningen över 64 %.
- Kartlägg i ungefär denna ordning:
  1. Huvudloop, VBlank-takt, speltillstånd (`$21EA`).
  2. Inläsning av joystick och tangentbord (`$BFE001`, `$DFF00C`).
  3. Spelarens rörelse, animationstabeller (troligen `NSIA`), kollision mot tiles.
  4. Dörrar, hissar, trappor, kort.
  5. Fiender: placering, beteende, skott, träffar.
  6. Bomber, gisslan, timer, sprängladdningar, bomb-avvecklingsskärmen (`BB`).
  7. Paletter, färgcykling, ljud- och musikuppspelning.
- Varje område dokumenteras i `docs/re/<område>.md` innan det byggs.

**Status (2026-10-05): pågår.** Skriptmotorn är kartlagd i stora delar och portad till Go (`pkg/game/script`). Den körs mot originalets bytekod och stämmer steg för steg med ett inspelat spår (gång höger + väggstopp). Se `docs/re/script-engine.md` och avsnittet "Nästa steg" i `README.md`. Insikten som förändrar planen: **beteende ligger till stor del i skript (data)**, så porten kör originalskripten i stället för att skriva om varje rörelse.

### Fas 3: spelaren i världen (medel)

- Inputabstraktion (`input.Actions` per tick) med tangentbord och handkontroll.
- Kamera och scrollning som i originalet (startposition från `$8F56`/`$8F58`).
- Spelarens rörelse, animationer och kollision enligt fas 2.
- HUD (`NSIMenu`): vapen, magasin, våning, träffar, klocka.

**Klart när:** man kan gå runt i byggnaden och HUD:en uppdateras.

### Fas 4: interaktion (medel)

- Dörrar och kort, hissar och trappor, våningsbyte.
- Föremål: plocka upp magasin, sprängladdningar, kort.

### Fas 5: fiender och strid (medel–stor)

- Fiendernas placering, rörelse och AI.
- Skott, träffar, död (`DO`-skärmen).

### Fas 6: målen (medel)

- Bomber och nedräkningstimer, gisslan.
- Bomb-avvecklingssekvensen (`BB`).
- Vinst (`WT`), explosion (`ET`), game over (`EX`).

### Fas 7: flöde och presentation (liten–medel)

- Titel (`NSILoader`), "tryck fire" (`PF`), intro (`IT`, `EN`, `HC`, `ST`), slutskärmar.
- Färgcykling, övergångar.

### Fas 8: ljud och musik (medel)

- Effekter från `NSISound`, `DAS`, `DBY`, `IAZ` (samplingsfrekvens och uppdelning bekräftas i fas 2).
- Musik från `NSIMusicSound`. Formatet är okänt. Det kan vara en egen sequencer, och då är det en större uppgift.

### Fas 9: trohet och finslipning (medel)

- Jämför mot WinUAE: hastigheter, timer, svårighet, skärmbilder.
- Repristest: spela in input-sekvenser och verifiera att simuleringen ger samma state varje gång.
- Inställningar: fönsterstorlek, tangenter, helskärm.

### Fas 10: egna assets (senare, eget projekt)

- Ersätt grafik, ljud, texter och bana med eget material (se spelpaket).
- Porten innehåller inga original-assets och inga data utlästa ur `ns`; de läses ur användarens egen diskett.

## Tester

- `pkg/amiga`: formattester mot de riktiga filerna. Hoppa över testet om `assets-local/` saknas, så att repot fungerar utan originalfilerna.
- `pkg/game`: enhetstester för kollision, timer, bomblogik och skador. Använd små syntetiska kartor, inte originaldata.
- Determinism: samma input-sekvens ska ge samma state-hash efter N tick.

## Risker

| Risk | Påverkan | Åtgärd |
|---|---|---|
| Spellogiken är svårare att kartlägga än väntat (handskriven assembler, inga symboler) | Fas 2 drar ut på tiden | Symbolfil och iterativ kartläggning; jämför mot emulator i stället för att förstå allt i detalj |
| Musikformatet är en egen sequencer | Fas 8 växer | Gör musiken sist; spelet fungerar utan |
| Saknad fil `HK` refereras i koden | Någon skärm kan saknas | Kontrollera i fas 7 när den laddas; ev. annan diskettversion |
| Beteende som beror på Amiga-timing (räknarloopar, blitter-väntan) | Fel takt | Allt kopplas till 50 Hz-tick; mät mot emulator |
| Rättigheterna klaras inte | Porten med originaldata kan inte släppas | Håll allt lokalt; fas 10 ger en publicerbar version |

## Beslut (2026-10-05)

- Porten är en identisk Go-port. Motorn kan senare användas för egna varianter.
- Ebitengine används.
- Port 1 ska vara **identisk** med originalet så långt det går. Avvikelser dokumenteras.
- En emulator (WinUAE) är facit för testerna.
- Mekaniken i `pkg/game` ska gå att återanvända i den egna varianten, så den hålls fri från plattformsberoenden.
