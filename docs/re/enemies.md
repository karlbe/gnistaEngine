# Fiender (`$508`, `$ACA`, op66–77)

Status: portat i `pkg/game/enemies.go` och skriptmotorn. Adresserna är hunk-relativa. Spårtesterna mot originalet (`trace_enemy_contact`, `trace_get_shot`, `trace_shoot_enemy`) stämmer steg för steg.

## Platser och aktörer

Spelaren är plats 0. Fienderna får plats 1–3 (aktörsdata `$8170 + $30*k`), hisskorgen och sprängladdningen delar plats 4. En fiende är en plats med ett skript, en position och en post (`+8`): fem skriptnummer (huvudskript, skript efter skott eller kontakt, två dödsskript, skadeskript) och träffpunkter (2).

## Spawnern (`$508`)

Körs före aktörsloopen varje bildruta och står stilla medan spelaren är på trappor (`$C34`).

- **Utlösare.** Tabellen på `$A738` har en byte per ruta i nivåmatrisen (indexerad med `$4EE`). Bit 0 markerar en utlösare, bit 1 "laddad" (sätts vid start av `$496`, nollställs när den löser ut). Bitarna 2–6 väljer vad som följer efter första fienden.
- **Första fienden** (`$522`): när spelaren står på en laddad ruta startas en fiende i första lediga plats från vänster eller höger kant av vyn, beroende på åt vilket håll spelaren tittar. Mallarna (24 byte: skript, sedan fiendeposten) ligger på `$5EF8`/`$5EC8` (första fienden höger/vänster) och `$5E80`/`$5E38` (vågor), och mallen växlar för varje fiende (`$C17`).
- **Vågor.** Bitarna 2–5 väljer en av fyra väntelistor (3, 6, 8 eller 15 fiender till, med fördröjningar på 20–80 bildrutor); bit 6 startar en enda fiende. Nästa vågfiende (`$74E`) dyker upp när fördröjningen gått ut om blocket till höger om spelarens ruta tillåter det (bit 0 i `$A628`) och en plats är ledig. Spelaren som står stilla får en slumpad sida (`$C2C`).

## Kontakt (`$ACA`)

En fiende utanför vyn tas bort (`$B74`). En fiende på skärmen sätter `$C1C` (blockerar trappor, hiss och dörrar, eftersom op40 struntar i "upp" då) och `$9A88` (stoppar bakgrundsmusiken). Kommer en fiende närmare än 32 pixlar fångar den spelaren: spelarens skript byts mot `$4E5A`/`$4EC4` och kontrollerna slutar.

## Skott

- **Spelaren skjuter** (op68–71): närmaste levande aktör i plats 1–3 på den sidan, oavsett höjd, träffas. Hagelbössan (vapen 1) dödar direkt, annars minskas träffpunkterna och fienden byter till skadeskriptet. Vid noll dödas den och dödsskriptet startar. Skottet kostar en patron (op78: ur magasinet, ladda om från reserven eller gå till skriptets tomma gren).
- **Fienden skjuter** (op76): om ingen annan fiende redan siktar sätts `$1BFC`. Spelarens skript (op66) tar skadan: hälsoräknaren `$1C05` minskar och blir 3 igen när en träff förloras (`$1C02`). Är träffarna slut dör spelaren. Räknaren startar på 0 (originalet sätter den aldrig), så första träffen kostar alltid en träff.
- **Döda fiender** (op77) blir kvar i bildrutan: bobben ritas en sista gång utan att sparas och blir en del av bakgrunden, se `docs/re/foreground.md`.
