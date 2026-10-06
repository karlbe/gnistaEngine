# Förbättringar och tillägg (avvikelser från originalet)

Port 1 ska vara identisk med originalet. Allt som inte finns i originalet ligger bakom **en** flagga som är **av** som standard i porten: `-improvements` på kommandoraden slår på allt, eller raden "Improvements" i Esc-menyn (gäller direkt i pågående spel). Med flaggan av, som standard, är spelet en ren kopia av originalet. Ett bygge kan ha den på som standard; standardvärdet sätts när spelet byggs (`-ldflags "-X main.defaultImprovements=true"`). Flaggan är en inställning och sparas inte i sparade tillstånd. Testerna kör med den av om inget annat anges.

## Det flaggan styr

| Tillägg | Originalbeteende (flaggan av) | Var i koden |
|---|---|---|
| Lämna hissen med vänster, höger eller fire | bara fire, och hissen tittar bara efter det en gång per några bildrutor och inte alls de första 36 per våning | `script.VM.liftExit`, `game.env.LiftExitRequest` |
| Lämna hissen vänd åt höger om man valde höger | utgången slutar alltid med en vändning åt vänster (skript 89) | `op43` vid `$455D` i `script.go` |
| Trappor upp och ned oavsett håll gubben tittar åt | styrningen letar bara efter trappor åt det håll man tittar | `script.VM.oppositeStairs` |
| Rullen startas med en buffrad nedtryckning | nedtryckningen måste träffa den enda bildruta per steg (32 bildrutor) som skriptet tittar efter | `game.env.TakeRollRequest`, `Joystick` |
| Nedtryckningen som lämnar ett rum ignoreras tills den släpps | gubben duckar om knappen hålls kvar | `State.IgnoreDown` |
| Fusktangenter: F4 (alla vapen, kort, full ammo och hälsa, hela kartan), F5 osårbar, F6 dubbel fart | finns inte | `State.cheat`, `input.Cheat` m.fl. |
| P växlar paus av och på | P pausar och fire släpper pausen (`$7602`) | `State.Step` |
| WASD som riktningstangenter, F4–F6 | bara piltangenter, Ctrl, mellanslag och F1–F3 | `cmd/game/keyboard.go` |
| Minikarta med dimma | finns inte | `render/minimap.go` |
| Ammo-, hälso- och fuskindikatorer och "PAUSED" i HUD:en | HUD:en visar bara det originalet visar | `render/hud.go` (`Renderer.Extras`) |
| Titeln: fire går till nästa sida, Esc hoppar över allt, fire som avslutar titeln spärras i spelet tills den släpps | fire läses bara under krediterna och på "press fire" | `title.Title.Improve`, `cmd/game/main.go` |
| Mjukare stereobild (0,5 i stället för Amigans hårda vänster/höger) | kanal 0 och 3 vänster, 1 och 2 höger | `Mixer.Separation`, `app.stereo` |

## Utvecklingsverktyg som inte styrs av flaggan

Esc-menyn (som flaggan själv ligger i), sparade tillstånd (F9/F10) och kommandoradsflaggorna (`-ticks`, `-shot`, `-inputs`, `-hold`, `-load`, `-save`) är verktyg runt spelet och påverkar inte spelreglerna.

## Avvikelser som inte är tillägg

Diskettladdningstider återskapas inte (rum, bilder i titel och slut). Trappstegens volym kan höjas med `stairGain` i `cmd/game/main.go`; den står på 1,0, alltså originalnivån.
