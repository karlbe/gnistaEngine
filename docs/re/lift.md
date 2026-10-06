# Hissen (`$16A8`, op44, op48–63)

Status: portat i `pkg/game/script` och verifierat i emulatorn (`trace_lift_up`) och med genomspelningen (`docs/playthrough.md`: åk upp och ned flera våningar). Kortlogiken är tolkad ur koden.

## Hur man åker

1. **Kalla.** Upp på en hisstile (`$29`/`$2A`, en per våning) anropar `$16A8` (`vm.callLift`). Kräver kort: banans tredjedelar (blockkolumn 0–2, 3–5, 6–8) kräver kort 0, 1 respektive 2 (bitar i `$505`); längre åt höger krävs inget. Korgen läggs i plats 4 med skript `$455E` och kommer till spelarens våning (ca 160 bildrutor). Står den redan på rätt ställe startas den om på `$46A6`.
2. **Gå in.** Ett nytt "upp" när korgen kommit startar inträdesskripten (85–87). Där läser op44 (`$427B`, `$4420`) styrspaken: **upp** startar färden uppåt, **ned** nedåt, men bara om det finns en hisstile fyra rader över respektive under. Annars väntar skriptet.
3. **Åk.** Färdskripten (93 uppåt, 94 neråt) flyttar spelaren och korgen en våning (64 pixlar) i taget. Vid varje våningsslut tittar op62/op63: är fire latchad (op61) går man ut, annars fortsätter färden så länge det finns hisstile vidare.
4. **Gå ut.** Utgångsskriptet (89) animerar utstigningen (ca 100 bildrutor) och slutar med att vända åt vänster (`op39`, `op43`), så spelaren kommer alltid ut vänd åt vänster. Med `-improvements` slutar det i stället åt höger om man valde höger (se `docs/improvements.md`).

## Detaljer att veta

- Op61 tittar efter fire en gång per några bildrutor och inte alls de första ca 36 bildrutorna av varje våning, så en kort tryckning missas ofta i originalet. Fire måste hållas nere i en våning för att färden ska stanna vid dess slut. Med `-improvements` buffras tryckningen och vänster och höger gäller också.
- Enemies på skärmen blockerar hissen (`$C1C`) som alla uppåtrörelser.
- Korgen är en enda bob (`$8128`), och `LiftBusy` (`$17C6` bit 0) är satt från anropet tills skriptet 60 tömmer plats 4, men inte medan spelaren åker.
