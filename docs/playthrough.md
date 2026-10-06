# Genomspelningen (end-to-end-testet)

`pkg/solve` spelar spelet med en bot och `cmd/solve` spelar in de inputs som vinner det. `TestPlaythrough` (`pkg/solve/playthrough_test.go`) spelar upp inspelningen i ett nytt spel och kontrollerar att spelet tar slut med vinst.

## Vad testet bevisar

- Hela spelet går att spela klart från start till slut: kort, vapen, dörrar (även den som måste sprängas), hissar, trappor, bomben, trådklippningen och slutskärmarna.
- Simuleringen är deterministisk. Uppspelningen håller bara om varje tick beter sig exakt som när inspelningen gjordes (42 000 ticks).
- Inga ej portade opkoder nås längs vägen.

Testet jämför inte mot originalet. Det bevisar att porten är komplett och stabil, inte att den är identisk.

## Inspelningen

`pkg/solve/testdata/playthrough.txt` innehåller bara vad spelaren tryckte (en run-length-kodad lista `värde:antal`). Den innehåller ingen speldata och kan checkas in. Den hålls i synk med spelet genom att köra om `go run ./cmd/solve` när spelreglerna ändras (testet failar annars).

Inspelningen använder osårbarhetsfusket som första input (F5), så den kräver `-improvements` på. Fusktangenten är en vanlig input, så uppspelningen är fortfarande bara inputs.

## Boten

- **Makron.** Botten gör en sak i taget från ett stillastående läge: ta ett steg eller flera åt höger eller vänster, upp eller ned (trappa, stege, dörr), åka hissen ett visst antal våningar, eller placera en sprängladdning på en låst dörr och backa undan. Därefter väntar den tills gubben står still.
- **Strid.** Boten skjuter ned fiender som en reflex: den vänder sig mot närmaste, väljer bästa vapnet med ammunition (hagelbössan dödar med ett skott) och skjuter. Gå-makron bryr sig inte om fiender, eftersom boten är osårbar och fiender bara blockerar trappor, hissar och dörrar. De makrona rensar först.
- **Sökning.** Sökningen jobbar på kopior av spelet (`State.Clone`) och är bredden först eller A* med en nivåkarta som ledtråd. Nivåkartan (`BuildGraph`, 6 000 lägen) byggs en gång av en fullt utrustad spelare och sparas i `assets-local/solve/graph.json`.
- **Plan.** Först hämtas korten och vapnen, och ammunition tills förrådet räcker (de sista korridorerna skickar vågor av fiender). Därefter går boten till bomben, spränger dörren, klipper rätt tråd och ser slutet.

## Kända begränsningar

- Tryck på en riktningsknapp i vissa vänteställen (efter att ha skjutit) vänder bara gubben, så makrons resultat beror på skriptläget. Botten behandlar skriptläget som en del av läget, och sökningen körs alltid på riktiga tillstånd.
- Inspelningen besöker 28 av 63 rum. Alla rum hinns inte med på spelets 25 minuter med den här ruttplaneringen: vägen till bomben tar ca 12 000 ticks och ett rum kostar i snitt ca 1 600. Kommandot `go run ./cmd/solve -reserve 0` spelar i stället in den kortaste vinsten (14 rum, ca 42 000 ticks).
- Inspelningen slutar med ca 17 sekunder kvar på klockan, så varje ändring av spelets takt eller regler gör att den måste spelas in på nytt.
