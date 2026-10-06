# Språkreferens för spelets skript

**Skriv skript med det vänliga språket i `docs/script-friendly.md`.** Den här filen beskriver motorns egna instruktioner under det, för den som vill veta exakt vad varje ord gör; det vänliga språket översätts till dem. Filen beskriver skriptspråket som spelmotorn (`pkg/game/script`) kör, **med egna ord och utifrån motorns egen Go-kod**, inte utifrån originalets assembler. Den är tänkt som underlag för att skriva egna skript. Opkodsnumren är originalets, eftersom motorn ännu läser originalets bytekod; en egen assembler ger dem namn.

## Modellen

- Spelet kör upp till fem **platser** (slots) varje bildruta (50 per sekund). Plats 0 är spelaren, 1–3 fiender, 4 hjälpplatsen (hisskorg eller sprängladdning).
- En plats har ett **skript** (en följd av opkoder, ett byte var, med operandbyte efter), en **aktör** (två bilddelar: överkropp och ben, var och en med läge och bildnummer), ett läge i världen (pixlar) och sex **sensorer**.
- Varje bildruta körs varje platsens skript tills det möter en vänteopkod (`wait`). Nästa bildruta fortsätter det därifrån. Efter alla platser scrollas vyn.
- Skripten numreras 0–169. Opkoder som hoppar anger skriptnummer. Motorn startar också skript direkt på adress.

### Bilder

En aktör består av två delar. En bild sätts som `n + förskjutning + bas × vapen`: `n` är operanden, förskjutningen beror på opkoden, och `bas` är den aktuella basen för delen (sätts med `setbase`) multiplicerad med vapnet (0–2). Därför ligger spelarens poser för pistol, hagelbössa och gevär med fast avstånd i bildbanken.

Delarnas lägen sätts med `part_upper`/`part_lower` relativt platsens läge. Platsens läge flyttas med `stepr/stepl/stepu/stepd`.

### Sensorer

Sex tile-id runt aktören läses om varje gång den passerat en tile-gräns: `here` (tilen den står på), `below`, `above2` (två rader upp), `side` (tilen åt det håll den går), `belowside` och `side2`. De används av de villkorliga opkoderna. Tile-id under 11 blockerar gång (se `docs/packs.md`).

## Opkoder

`n` är ett skriptnummer, `b` ett byte, `s` ett signerat byte.

### Tid

| Op | Namn | Betydelse |
|---|---|---|
| 0, 4, 5, 14, 15 | `wait` | avsluta bildrutan; fortsätt här nästa |
| 77 | `deactivate` | aktören tas bort från platsen (död fiende, avslutad hjälpare) |
| 90 | `explode_end` | spelet slutar med explosionsutfall |

### Bilder och lägen

| Op | Operander | Betydelse |
|---|---|---|
| 1 / 2 / 3 | b b | båda delarnas bild: förskjutning 0 / 116 / 254, plus bas × vapen |
| 6–13, 17–20 | b | en dels bild: se tabellen i `script.go` (`singleFrameOps`): övre delen 6, 7, 8, 9, 10, 17, 18 och nedre delen 11, 12, 13, 19, 20, med olika förskjutningar; vissa lägger till bas × vapen |
| 16 | b b | båda delarnas bild = n + 394 |
| 21 | b b | sätt bildbaserna (övre, nedre) |
| 26 / 27 | s s | övre / nedre delens läge = platsens läge + (dx, dy) |
| 52 / 53 | – | visa / dölj aktörens bilder |

### Rörelse

| Op | Operander | Betydelse |
|---|---|---|
| 22 / 23 / 24 / 25 | – | flytta aktören en pixel höger / vänster / upp / ned. Passeras en tile-gräns läses sensorerna om |
| 28 | s s b | sätt en bestående scrollning (dx, dy) som tas var (b+1):e bildruta |
| 29 | – | stoppa scrollningen |
| 30–33 | – | ett scrollsteg höger / vänster / upp / ned för just den här bildrutan |
| 38 / 39 | – | vänd åt höger / vänster och läs om sensorerna |

Spelaren står fast på skärmen (tile 10, 6 i vyn); det är vyn som scrollas när spelaren går. Fiender flyttas med 22–25.

### Flöde

| Op | Operander | Betydelse |
|---|---|---|
| 34 | n | hoppa till skript n |
| 35 / 36 | n / – | anropa skript n (en nivå) / återvänd |
| 37 | b n | hoppa till n sammanlagt b gånger i följd, gå sedan vidare (loop) |
| 42 / 43 | – | tillbaka till spelarens senaste styrning, höger / vänster |

### Spelarens styrning

`40` (vänd höger) och `41` (vänd vänster) följs av **tolv skriptnummer** och läser styrspak och tangenter. De är spelarens tillståndsmaskin: den väljer skript för att gå, vända, klättra, rulla, skjuta, byta vapen eller använda en dörr. Operanderna:

`0` gå/vänd höger · `1` gå/vänd vänster · `2`, `3` upp-varianter (trappa, stege) · `4`, `5` hiss · `6` lägg laddning · `7`, `8`, `9` ned-varianter · `10` efter rullning · `11` skjut.

Styrningen testar tilen på platsen för att välja variant (dörr 0x26–0x28 öppnar rummet, trappa/stege/hiss efter id) och byter vapen vid F1–F3 till viloskriptet för vapnet. Den sätter också gångriktningen. Kort och vapen styr om hissen får kallas (se `docs/re/lift.md`).

### Villkorliga hopp

| Op | Operander | Betydelse |
|---|---|---|
| 44 | n n | uppåt och hisstile fyra rader över: hoppa till första n; nedåt och hisstile fyra under: andra n; annars vidare |
| 45 / 46 | n | höger / vänster hållen och fri väg framåt: hoppa till n |
| 47 | n | rullningen: nedåt hållen och fri väg: hoppa till n och sätt "ned-flaggan" |
| 64 | n n | fire: första n; annars om F1–F3 vidare; annars om ingen riktning: andra n |
| 65 | n n | fire: första n; nedåt: andra n; annars vidare |
| 61, 62, 63 | – | hissens utgång: 61 läser fire (med `-improvements` även vänster/höger), 62/63 startar korgen vid hisstile om fire inte är lagrad |

### Träffar

| Op | Operander | Betydelse |
|---|---|---|
| 66 | skada a b c d | om en fiende har skjutit: ta skadan (träffar minskar, vid noll förlorar man ett liv). a, b: träffad från höger (liv förlorat, död); c, d: från vänster. 67 återvänder efter 66 |
| 68 / 69 | – | skjut höger / vänster: närmaste levande fiende på den sidan, oavsett höjd, träffas; hagelbössan dödar direkt, annars minskar träffpunkterna och fienden går till sitt skadeskript, vid noll till sitt dödsskript |
| 70 / 71 | – | samma, men fienden får det andra dödsskriptet |
| 75 | – | fienden lever igen: börja om huvudskriptet |
| 76 | – | fienden skjuter mot spelaren (om ingen annan redan siktar) |
| 78 | n b | spelarens skott: dra ett skott, ladda om från reserv (annars hoppa till n), spela ljud b |

### Hiss och sprängladdning (hjälparplatsen)

| Op | Betydelse |
|---|---|
| 48–51 | starta hisskorgen med ett av fyra skript |
| 54, 55, 56, 57 | göm korgen; korgen en våning (64 px) upp / ned och visa; visa |
| 58, 59, 60 | hissen: kallad / upptagen / korgen försvunnen (avslutar bildrutan) |
| 72, 73, 74 | lägg en laddning vid aktören; visa den; den exploderar (dödar den som står på den, annars öppnar dörren) |

### Ljud

| Op | Operander | Betydelse |
|---|---|---|
| 79 / 80 | – | nästa effekt ur den cykliska fotstegslistan / trappstegslistan |
| 81, 82, 83, 84 | b | spela effekt b på kanal 0, 1, 2 (via olika rutiner), 3 |
| 85, 86, 87, 88 | – | kanal 0–3: låt ljudet spelas färdigt och tystna |
| 89 | – | starta kanal 1 (loopande tills den laddas om) |

## Vad språket inte kan

Det finns inga variabler, ingen aritmetik och inga uttryck. Allt man kan göra är att byta bild, flytta en pixel, vänta, hoppa och fråga motorn om några fasta villkor. Beteende som inte är en av opkoderna ovan (spawnern, hissens och dörrarnas regler, kontaktkollisionen, klockan) ligger i Go och är ändringar i motorn, inte i skripten.

## Assemblern

Egna skript skrivs som text (`*.gs`) och sätts ihop av `pkg/game/script/asm`, som `packtool check` och spelet anropar när ett paket har `scripts/`. Källformatet:

```
; kommentar
script gå_höger          ; ett skript börjar; skripten numreras i den ordning de står
    frames 1 64          ; instruktion och operander
    stepr
    wait
    goto gå_höger        ; skript hänvisar till varandra med namn
mark här                 ; i ett skript: namnger adressen till nästa byte
data stegljud 11 12 13 0 ; råa byte (t.ex. en nollavslutad ljudlista)
```

Operander är heltal (`0x10` går bra, signerade operander kan vara negativa) eller skriptnamn. Filerna i `scripts/` sätts ihop i filnamnsordning. `program.json` pekar ut de skript och tabeller som motorn startar själv (spelaren, fienderna, kontaktskript, ljudlistor, bildnummer, magasinstorlekar, fiendemallar); alla namn slås upp i den hopsatta avbilden. Språket beskrivs i `docs/script-friendly.md`.

Namn och operandtyper (b = byte, s = signerat byte, n = skript). Operandräkningen är provad mot alla originalets skript: de avkodas exakt (`TestDecodeOriginal`).

| Op | Namn | Operander |
|---|---|---|
| 0 | `wait` | |
| 1 | `frames` | b b |
| 2 | `frames116` | b b |
| 3 | `frames254` | b b |
| 6 | `legs_w` | b |
| 7 | `legs_w116` | b |
| 8 | `legs_w254` | b |
| 9 | `legs_w394` | b |
| 10 | `legs_622` | b |
| 11 | `body_w` | b |
| 12 | `body_w116` | b |
| 13 | `body_w254` | b |
| 16 | `both_394` | b b |
| 17 | `legs_116` | b |
| 18 | `legs_394` | b |
| 19 | `body_254` | b |
| 20 | `body_394` | b |
| 21 | `setbase` | b b |
| 22 | `stepr` | |
| 23 | `stepl` | |
| 24 | `stepu` | |
| 25 | `stepd` | |
| 26 | `legs_at` | s s |
| 27 | `body_at` | s s |
| 28 | `scroll` | s s b |
| 29 | `scroll_stop` | |
| 30 | `scroll_r` | |
| 31 | `scroll_l` | |
| 32 | `scroll_u` | |
| 33 | `scroll_d` | |
| 34 | `goto` | n |
| 35 | `call` | n |
| 36 | `return` | |
| 37 | `repeat` | b n |
| 38 | `face_right` | |
| 39 | `face_left` | |
| 40 | `control_right` | n n n n n n n n n n n n |
| 41 | `control_left` | n n n n n n n n n n n n |
| 42 | `resume_right` | |
| 43 | `resume_left` | |
| 44 | `lift_choose` | n n |
| 45 | `if_right_free` | n |
| 46 | `if_left_free` | n |
| 47 | `if_roll` | n |
| 48 | `cabin0` | |
| 49 | `cabin1` | |
| 50 | `cabin2` | |
| 51 | `cabin3` | |
| 52 | `show` | |
| 53 | `hide` | |
| 54 | `cabin_hide` | |
| 55 | `cabin_up` | |
| 56 | `cabin_down` | |
| 57 | `cabin_show` | |
| 58 | `lift_called` | |
| 59 | `lift_busy` | |
| 60 | `lift_gone` | |
| 61 | `lift_exit_check` | |
| 62 | `lift_enter_up` | |
| 63 | `lift_enter_down` | |
| 64 | `if_fire` | n n |
| 65 | `if_fire_down` | n n |
| 66 | `take_hit` | b n n n n |
| 67 | `hit_return` | |
| 68 | `shoot_right` | |
| 69 | `shoot_left` | |
| 70 | `shoot2_right` | |
| 71 | `shoot2_left` | |
| 72 | `charge_place` | |
| 73 | `charge_show` | |
| 74 | `charge_blow` | |
| 75 | `revive` | |
| 76 | `enemy_fire` | |
| 77 | `deactivate` | |
| 78 | `fire_weapon` | n b |
| 79 | `step_sound` | |
| 80 | `stair_sound` | |
| 81 | `sound0` | b |
| 82 | `sound1` | b |
| 83 | `sound2` | b |
| 84 | `sound3` | b |
| 85 | `silence0` | |
| 86 | `silence1` | |
| 87 | `silence2` | |
| 88 | `silence3` | |
| 89 | `start1` | |
| 90 | `explode_end` | |
