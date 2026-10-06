# Slutet (`$30`–`$130`)

Status: portat i `pkg/game/ending.go` och verifierat på skärmbilder av båda sluten. Tiderna är uppmätta i emulatorn.

När spelet är slut (`$21EA` ≠ 0) stänger huvudloopen av ljudet (`$3A`: `$9A88` = 1 och DMA av) och visar en sekvens. Utfallet `$21EA` är 2 (explosion: tiden tog slut, spelaren dog eller fel tråd klipptes) eller 3 (rätt tråd). Utfall 1, som skulle visa `DO`, sätts aldrig.

## Explosion

1. Spelvyn i blixtpaletten `$73D6` i 95 bildrutor (väntesilja på 500 000 varv).
2. Svart i 22 bildrutor.
3. Bilden `ET` (en textsida om blixten och chockvågen) i 380 bildrutor.
4. Bilden `EX` (ett svampmoln med en spelet-är-slut-text) i 380 bildrutor.

## Vinst

1. Svart i 22 bildrutor.
2. `WT` (berättelsen om att bomben desarmerats), tills fire trycks.
3. `HC` (flygplansbilden), tills fire trycks.
4. `EN` (sista sidan), tills fire trycks.

Sidorna som väntar på fire går vidare så fort fire är nere, eftersom originalet läser knappen varje bildruta. Håller man fire nere hoppar man alltså över flera sidor.

Därefter börjar spelet om från titeln. Diskettladdningstiderna mellan bilderna (1–3 s) återskapas inte.
