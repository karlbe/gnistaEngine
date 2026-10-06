# Förgrund: varför gubben går bakom räcken

Status: **verifierat** mot skärmbilden från originalet (`assets-local/uae/ingame.png`) och koden, portat i `pkg/render`.

## Mekanismen

Spelskärmen har **fem bitplan**, men färgerna använder bara de fyra första (tile-bilderna har bara index 0–15). Det femte planet är en **förgrundsflagga**.

- Tile-ritaren (`$8D66`) kopierar ett lagrat plan till skärmplan *p* om bit *p* är satt i tilens planmask (första tagbyten i JBOB-posten). Tiles med bit 4 satt (taggarna `$57` och `$5F`, 88 tiles) lagrar ett femte plan och får därmed förgrundspixlar. Golvkanter, räcken och trappräcken är sådana.
- Bob-ritaren (`$803A`) bygger först en mask i `$83B0` (`$80EA`, minterm `$B50`): **bobbens mask OCH INTE skärmens femte plan**. Därefter görs den vanliga cookie-cut-blittningen med den masken. Bobbar ritas alltså aldrig över pixlar med förgrundsflaggan, och det gäller alla bobbar (spelaren, fiender, hisskorgen och de kvarlämnade liken).
- Färgregistren 16–31 i den extraherade paletten är svarta, men originalbilden visar förgrundspixlarna i sina vanliga färger. Porten ritar dem därför med index 0–15 och använder femte planet bara som mask.

## Porten

`amiga.Bank.TileForeground(i)` läser femte planet. `render.drawMap` fyller en förgrundsmask för vyn och `drawBobPal` hoppar över de pixlarna.

## Ljud på trappor (avvikelse)

Trappstegen (effekt 16–18, op80) är ungefär fyra gånger svagare än fotstegen (effekt 11–13) i originaldata, så de är praktiskt taget ohörbara. Porten har en förstärkning (`stairGain`) för dem, nu 1,0 alltså originalnivån (faktor 4 motsvarade fotstegen men var för högt, och 2 också) (`stairGain` i `cmd/game/main.go`, `Entry.Gain`). Det är **inte** originalbeteende. Stegar (skripten från `$3BAA`) har ingen ljudop alls i originalet och är tysta även i porten.
