# Titelsekvensen (`$6690`, `$6852`)

Status: portad i `pkg/title` och `pkg/render/title.go`. Ordningen och bilderna är **verifierade** mot originalet (cold boot i WinUAE med skärmbilder var halv sekund); siluettens slutpose uppmättes mot emulatorn (`$7396`). Tiderna är **hämtade ur väntesilingorna i koden**, inte uppmätta.

## Bilderna (alla 320 x 200, 5 bitplan)

| Fil | Innehåll |
|---|---|
| `NSILoader` | titelbilden med logotypen |
| `IT` | inledningstexten (en textsida som sätter scenen) |
| `TA` | siluetten av en man med gevär. Bilden använder bara jämna färgindex (0, 2, ... 14) |
| `PF` | uppmaningen att trycka fire för att börja |
| `ST` | berättelsen om hur spelaren hamnar på oljeriggen, som visas medan spelet laddas |

## Ordningen

1. Logotypen (`NSILoader`), svart och sedan bilden med sin egen palett. Väntesilja på `$10C8E0` varv.
2. Svart, sedan `IT` med sin egen palett. Väntesilja på `$186A00` varv.
3. Svart, sedan `TA`. Dess egen palett är bara platshållare. Färgerna kommer från fem palettabeller i koden som byts med en kort väntan emellan (`$C350` varv): `$7316`, `$7356`, `$7396`, `$7356`, `$7396`. Tabellerna ger siluetten tre olika poser (armen och geväret höjs), mot röd bakgrund. Slutposen är `$7396`.
4. Rullande krediter (`$6852`) över siluetten tills texten är slut eller fire trycks.
5. "Press fire" (`PF`), väntar på fire. Fire under kreditrullningen hoppar direkt till steg 6.
6. Berättelsen (`ST`) medan spelet laddas (`$682A`). Porten väntar på fire, eftersom laddningen inte återskapas.

## Kreditrullningen

- Texten ligger på `$9D82`: rader som slutar med 0, och en rad som börjar med `$01` avslutar. 89 rader. Den innehåller bland annat krediterna för den som knäckte spelet, så den läses ur koden vid körning och bäddas inte in.
- Varje rad skrivs med 6 pixlars teckenavstånd på x = `$1E`, y = `$C9` (strax under bildens 200 rader). Efter varje rad skrivs inget nytt förrän textlagret har rullat 15 steg (`$68A2`-`$68C4`), ett steg per pass av en silja på `$300C` varv (`$9D6C` flyttar lagret en pixel uppåt). Det blir ungefär 2,34 bildrutor per pixel, alltså drygt en minut för hela texten.
- Texten är ett eget lager i bitplanet som siluetten lämnar ledigt: en textpixel gör bildens (jämna) index till nästa udda index, som palettabellerna färgar vitt. Därför ligger texten ovanpå siluetten utan att dölja den.

## Väntetider

Väntesiljorna är `subi.l #1,d0 / bne` och går ungefär 5263 varv per bildruta (samma värde som slutet använder): logotypen ca 209 bildrutor, inledningstexten ca 304, varje pose 10, mellan sidorna 19.

## Avvikelser

- Fire hoppar till nästa sida från alla skärmar (originalet läser bara fire under krediterna och på "press fire"). Esc hoppar över hela titeln och `-title=false` stänger av den (körningar med `-ticks`, `-shot`, `-load`, `-inputs` eller `-hold` startar i spelet om inte `-title` anges).
- Diskettladdningstiderna återskapas inte, och musiken finns inte än. Originalet väntar också på en flagga som musiken sätter (`$9A88`) innan "press fire" visas, vilket porten ersätter med att visa den när krediterna är slut.
