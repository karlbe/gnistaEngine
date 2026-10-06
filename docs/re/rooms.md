# Rum, dörrar och bomben (`$23CC`, `$2864`)

Status: portat i `pkg/game/rooms.go`. Rumsposterna och själva sekvensen är verifierade i emulatorn (`TestFirstDoor`, spelaren går första dörren). Rumsbilderna och meddelandena är original och läses vid körning.

## Dörrar

Dörrar är tiles `$26`–`$28` (en tile, en dörr; 63 stycken i banan). Upp på en dörrtile anropar `$23CC` (`State.enterRoom`). Rummet slås upp via nivåmatrisens ruta för **vyns** blockposition (`$4EE`, inte spelarens): `$A9F4 + $4EE` ger ett rumsnummer, som pekar ut en post på 12 byte i `$ACAC`. Ruta för vyn och för spelaren sammanfaller i praktiken eftersom spelaren står 10 tiles in i vyn, men det är vyns ruta som gäller.

Rumsposten: `+0` lås (bitar ur kortbyten `$505`; vilket av dessa kort som helst öppnar), `+1` flaggor (bit 0–4 ett kort, bit 5 besökt, bit 6 vapen 2, bit 7 vapen 3), `+2..4` magasin per vapen, `+5` sprängladdningar, `+6` bild, `+7` meddelande, `+8` sprängd.

- **Besökt rum** visas som tomt (bild 0, meddelande 0).
- **Låst dörr** gör ingenting om spelaren saknar korten och dörren inte är sprängd. 36 av rummen har kortlås; bombrummet har låset `%100000`, som inget kort kan öppna, så dörren måste sprängas.
- Rummet ger sina föremål (kort, vapen, magasin upp till 9, laddningar upp till 9), markeras som besökt och visar bilden och meddelandet. Första hjälpen (bild 5) fyller träffarna och markeras aldrig som besökt. Ned lämnar rummet.
- Klockan, fiender och allt annat står stilla under besöket, eftersom originalet kör hela besöket inuti spelarens kontrollopkod.

## Sprängning

Mellanslag på en dörrtile (`$26`–`$29`) med laddningar kvar nedräknar laddningarna och startar skript 6: laddningen läggs ut (det tar ca 120 bildrutor) och sprängs kort därefter. Står spelaren kvar på laddningens ruta när den går av dör han (op74); annars sätts dörrens "sprängd"-byte och dörrbilden stämplas in som sprängd. Spelaren måste alltså gå undan direkt när lägganden är klar.

## Bomben

Bombrummet (bild 8) har en egen sekvens: fire lämnar rummet, mellanslag startar trådklippningen. Fyra trådar, en markör (`WireX`) och en inläsning av styrspaken var 50:e bildruta: höger och vänster flyttar markören, fire klipper. Den fjärde tråden (`wireGood` = 3, den gröna) desarmerar bomben och ger utfallet 3 (vinst); de andra ger utfallet 2 (explosion).
