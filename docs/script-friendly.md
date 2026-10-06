# Skriptspråket: skriv beteende med vanliga ord

Skript beskriver hur en gestalt beter sig: vilken bild den visar, hur den rör sig och vad den väntar på. Du skriver dem som text i `scripts/*.gs` i ett spelpaket. Språket har vanliga ord, namn på tal, bilder och ljud, och mallar för det som upprepas. Motorns egna instruktioner (opkoder) behöver du inte känna till; de finns i `docs/script-reference.md` om du vill.

Exempel, en gång i trappan upp åt höger:

```
script stairs_up_r
    place legs 0 0
    place body 0 -16
    camera follow up-right every 3 ticks
    repeat 8 times
        stairstep
        repeat 4 times as i
            hold legs=legs_walk_r+i body=body_aim_r for 6 ticks moving up-right every 3 ticks
        end
    end
    goto level_out_r
```

Allt efter `;` på en rad är kommentar. Mellanslag och tomma rader spelar ingen roll. Ord skrivs med små bokstäver.

## Tid och rörelse

Spelet går 50 bildrutor (ticks) i sekunden. Ett skript körs tills det väntar; nästa tick fortsätter det därifrån.

| Skriv | Betydelse |
|---|---|
| `wait` / `wait 5` / `wait 5 ticks` | vänta en eller flera ticks |
| `show POSE` / `show legs=A body=B` | visa en bild (ben och överkropp) |
| `hold POSE for 6 ticks` | visa och vänta |
| `hold legs=A body=B for 4 ticks moving right` | och flytta figuren en pixel per tick åt höger (`left`, `up`, `down`, eller `up-right`, `down-left` och så vidare) |
| `... moving up-right every 3 ticks` | ett steg var tredje tick i stället för varje |
| `place legs X Y` / `place body X Y` | var benen och överkroppen sitter, i pixlar från figurens läge (överkroppen brukar sitta 16 uppåt: `place body 0 -16`) |
| `move right 4` | fyra pixlar åt höger, en per tick (`left`, `up`, `down`) |
| `move up 4 camera` | och vyn följer med |
| `step right` | ett steg på en gång utan att vänta |
| `camera follow right` | vyn följer med, en pixel per tick (`every N ticks`, `speed N`; riktningar som ovan) |
| `camera stop` / `camera nudge up` | sluta följa / ett steg för vyn |
| `face right` / `face left` | vänd figuren (det styr också vad den känner av framför sig; spelarens vänteläge gör det varje tick) |

## Bilder, ljud och namn

```
const walk_ticks = 4              ; ett namn på ett tal (tal, eller summor och produkter: walk_ticks*2+1)
sound pistol = 0                  ; ett namn på en ljudeffekt (filen sounds/0.wav)
pose stand legs=0 body=64         ; ett namn på ett par bilder
```

Ett namn kan användas överallt där ett tal väntas. Bildnummer är numren på filerna i `sprites/`. Vapnet lägger sedan till sin bas på varje bild: `weapon sprites legs=0 body=64` betyder att överkroppen visas som bild 64 med pistol, 128 med hagelbössa och 192 med gevär.

| Skriv | Betydelse |
|---|---|
| `play pistol` | spela ljudet (kanal 0). `play lift_motor channel 1 looping` låter det gå om och om igen |
| `footstep`, `stairstep` | nästa fotsteg eller trappsteg ur ljudlistan (`program.json`) |
| `start sound` | starta ljudet som `fire weapon` har laddat |

## Upprepning och mallar

```
repeat 8 times as i               ; i räknar 0 till 7 och kan användas som ett tal
    hold legs=1+i body=64 for 4 ticks moving right
end
```

Det som skiljer tre vapens skript åt är ett ljud och vilket skript som kommer sedan, så de skrivs en gång som en mall och används tre gånger:

```
template fire_right effect
    show legs=legs_stand_r body=body_fire_r
    fire weapon sound=effect empty=dry_r
    ...

script fire_r_0 from fire_right effect=pistol
script fire_r_1 from fire_right effect=shotgun
```

Mallens parametrar byts ut överallt där de står som ett helt ord (i `key=värde` byts bara värdet). Ge dem därför namn som inte också är ord i språket. `self` är skriptets eget namn.

## Hopp

| Skriv | Betydelse |
|---|---|
| `goto NAMN` | fortsätt i ett annat skript |
| `call NAMN` / `return` | gör ett annat skript och kom tillbaka (en nivå) |
| `if holding right and path_clear goto NAMN` | om man håller höger och vägen är fri (`left`) |
| `if rolling goto NAMN` | om man håller ned medan man går: rulla |
| `branch fire=A idle=B` | skjuter man: A; gör man ingenting: B (direkt, utan att vänta: vänta själv först); annars vidare |
| `branch fire=A down=B` | skjuter man: A; håller man ned: B; annars vidare |
| `controls facing right right=A left=B ... default=C` | spelarens knappar, se nedan |
| `resume controls right` | tillbaka till spelarens knappar (`left`). Det går bara åt ett håll som spelaren redan har stått vänd åt |

### Spelarens knappar

Spelarens vänteläge är ett skript som börjar med `controls`. Varje `nyckel=skript` säger vilket skript en inmatning startar; det som inte anges får `default=`. Nycklarna är:

`right`, `left` (gå eller vända), `up_stairs` (upp framför trappa), `up_ladder`, `lift_call` (första trycket vid hissen), `lift_ready` (när hissen är där), `charge` (mellanslag), `down_stairs`, `down_ladder`, `duck` (ned), `after_roll`, `fire`.

Om ingenting trycks fortsätter skriptet under raden: visa stående bild, kontrollera träffar, `wait` och `goto self`.

## Strid

| Skriv | Betydelse |
|---|---|
| `check hits damage=1 right_hurt=A right_dead=B left_hurt=C left_dead=D` | har en fiende skjutit: ta skadan; den som tar skadan går till A (träffad från höger) eller B (död), C och D från vänster |
| `back from hit` | tillbaka till där träffkontrollen stod |
| `fire weapon sound=S empty=SKRIPT` | ta ett skott ur magasinet (ladda om ur reserven; är allt slut gå till SKRIPT) |
| `shoot right` / `shoot left` | skottet går: närmaste fiende åt det hållet träffas. `second_death` ger fienden dess andra dödsskript |
| `game over explosion` | spelet är slut (explosionsslutet) |

Fiender:

| Skriv | Betydelse |
|---|---|
| `enemy shoots` | fienden skjuter mot spelaren |
| `enemy revives` | börja om från huvudskriptet |
| `remove actor` | fienden är borta (och blir kvar som en bild på marken) |

En fiendemall i `program.json` anger skripten en fiende startar med: `start` (gå in), och `scripts`: huvudskript (siktar), efter skott, två dödsskript, skadad.

## Laddningar, hiss och dörrar

| Skriv | Betydelse |
|---|---|
| `place charge` | lägg en sprängladdning där figuren står |
| `charge show` / `charge explodes` | visa den / den går av (dödar den som står på den, annars öppnas dörren) |
| `cabin picture N` | hisskorgens (och laddningens) bild: frame 622 + N |
| `lift cabin show` / `hide` / `up` / `down` | visa, göm, flytta korgen en våning |
| `lift cabin 0` ... `3` | starta ett av hissens fyra korgskript (`program.json`) |
| `lift called` / `busy` / `gone` | hissen är kallad / upptagen / borta |
| `lift choose up=A down=B` | upp eller ned om det finns en hissruta en våning över eller under |
| `lift exit_check` | känn efter om man trycker fire (stanna vid nästa våning) |
| `lift board_up` / `board_down` | i slutet av en våning: åk vidare om det finns mer hiss (hissruta 0x29), annars stanna (0x2A) |
| `show actor` / `hide actor` | visa eller göm figuren (i hissen) |



## Programmet: program.json

Förutom skripten behöver motorn veta vilka som är dess ingångar. `program.json` pekar ut dem med namn: `player` (första skriptet), `idle` (vänteläget per håll och vapen), `lift_call`, `lift_recall`, `cabin` (fyra), `lift_helper`, `charge`, `blown_up`, `step_sounds` och `stair_sounds` (ljudlistor: `data namn 11 12 13 0`), `contact_right`, `contact_left`, `player_frames`, `mag_size`, fiendemallarna (`first_right`, `first_left`, `wave_right`, `wave_left`) och `wave_order`.

## Kontroll och fel

`go run ./cmd/scriptasm asm <katalog>` sätter ihop skripten och visar fel med fil och rad. `go run ./cmd/packtool check <paket>` gör det som spelet gör.
