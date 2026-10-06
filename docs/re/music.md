# Musiken (`$96DE`, `$94EA`, `$95EA`)

Status: portad i `pkg/audio/music.go`. Spelarens logik är läst ur koden och testad mot de riktiga filerna (antal mönster, notstarter, slut). Ljudet är **inte jämfört med originalet på gehör**.

## Vad som kör vad

Spelet har två avbrott på samma vektor (`$6C`, rutinen `$7566`): copper-avbrottet kör spelets bildruta, och vertikal-blank-avbrottet (`$75C8`) kör musikens tick (`$96DE`) 50 gånger i sekunden, om inte bit 0 i `$9A88` är satt. Den biten betyder "låten är slut" eller "stoppad av spelet".

## Filerna

| Fil | Innehåll |
|---|---|
| `NSIA` (10 072 byte) | låten: 6-bytesposter ("händelser"), 100 mönster, fyra ordningslistor |
| `NSIMusicSound` | titellåtens instrument: tio samplingar |
| `IAZ`, `DAS`, `DBY` | de tre korta samplingar som bakgrundsspåret i spelet använder |

## NSIA

Från början ligger händelserna (6 byte vardera, upp till `$2328`): flaggbyte, instrument, period (u16), längd i rader, volym. Ett mönster är en följd av händelser och slutar med den första vars flaggbyte är exakt 2 (`$9B5C`); den händelsen spelas också. Därefter kommer fyra ordningslistor på 156 byte vardera (en per stämma, från `$24B8`), där varje byte är ett mönsternummer och `$FF` avslutar. Alla fyra är 16 poster långa.

## Spelaren

- Varje stämma har en nedräknare (`$9A82`–`$9A85`, startvärde 1), en pekare till nästa händelse och en position i sin ordningslista.
- Spelaren går ett steg (en "rad") var femte tick. Räknaren `$9A87` går 4, 3, 2, 1, 0.
- En stämma som inte är avstängd (`$9A89`) räknar ned sin not, eller startar nästa när nedräknaren är 1: DMA för kanalen stängs av, pekare, längd och period sätts från instrumenttabellen och händelsen, volymen sätts (värdet 4 betyder 0), nedräknaren laddas med notens längd och stämman markeras i `$9A86`.
- Vid nästa tick slår `$96DE` på DMA för de markerade stämmorna och sätter längden till 1 ord, så varje not spelas **en gång** och tystnar. (Samma tvåsteg som ljudeffekterna.)
- En händelse med flagga 2 flyttar stämman till nästa mönster i ordningslistan. Stämma 0 och 3 kontrollerar `$FF`: då är låten slut (`$97F0`). Slutar den titellåten sätts `$9A88` bit 0. Slutar bakgrundsspåret startar det om.

## Titellåten (`$94EA`)

Startar när logotypen är färdig (`$66FC`) och går i 96 sekunder. Titeln väntar på att den tar slut (`$67C4`) innan "press fire" visas. Stämmorna börjar alla på första mönstret i sina listor. Instrumenten är de tio i `NSIMusicSound`, och tabellen byggs av `$9C52` (ett förskjutningsvärde och en längd i ord var; den sista sampling-längden är 2 byte längre än filen, vilket originalet också läser förbi).

## Bakgrundsspåret i spelet (`$95EA`)

Anropas när ett spel börjar (`$2A`). Stämma 0 och 1 stängs av (`$9A89` = 3). Stämma 2 och 3 börjar på position 18 i sina ordningslistor, med instrumenttabellen utbytt mot IAZ, DAS och DBY (längderna 2500, 2000 och 1000 ord). När det tar slut startar det om (`$9A8A`).

Fiendekontrollen (`$ACA`) nollställer `$9A88` i början av varje bildruta och sätter den igen så länge en fiende syns på skärmen, så bakgrundsspåret spelar bara när ingen fiende syns. Spelet stoppar det helt när spelet är slut (`$3A`). I rum och under paus kör inte avbrotten och därmed inte musiken.

## Porten

`audio.Tracker` är en ren tillståndsmaskin som styr fyra röster (`Voices`): `LoadEntry` motsvarar registerskrivningarna med DMA av och `Start` slår på DMA. `Mixer` implementerar `Voices`. `cmd/game` anropar `Tick` en gång per bildruta (titeln och spelet). Utan ljudenhet används `audio.Silent`, så titelns väntan på låtens slut fungerar ändå.
