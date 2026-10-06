# Ljudeffekter

Status: effekterna är portade (`pkg/audio`). Musiken (`NSIMusicSound`, rutinerna runt `$96F4–$9A32`) är inte kartlagd eller portad.

## Effekttabellen (`$6432`) — verifierat

20 poster à 10 byte: datapekare (4), längd i **ord** (2), Paula-period (2), volym (1), fyllnad. Tabellen är nollad i filen och byggs vid start av `$61B0–$6430`. Alla datapekare pekar i `NSISound`-filens data (efter 6-byteshuvudet, `$64FA` + 6) plus en offset. Offseterna kedjar sig i filen (post 1 börjar där post 0 slutar), vilket bekräftar att längden är i ord. Porten tolkar rutinen vid körning (`audio.ParseTable`) i stället för att kopiera tabellen.

Samplingstakt = 3 546 895 / period (PAL). Volym 0–64.

| Id | Används av | Anmärkning |
|---|---|---|
| 0, 4 | Skott (op78 på kanal 0) | samma data, olika period |
| 11–13 | Fotsteg (op79-listan `$20C2`) | kanal 0 |
| 16–18 | Trappsteg (op80-listan `$2114`) | kanal 0 |
| 5–10, 19 | Op82/83/84 (kanal 1–3) | rop, död, explosion m.m.; exakt vem som är vem är **hypotes** |
| 14, 15 | Hissen (op82 + op89 / op86) | 14 loopar, 15 avslutar |

## Kanaler och uppspelning — verifierat ur koden, uppspelningsbeteendet är tolkat

Skripten styr Paula direkt i två steg:

1. `$64FE` / `$652C` / `$655A` / `$6588` (op78/81, 82, 83, 84) stänger kanalens DMA och laddar register: pekare, längd, period, volym. Inget startas.
2. En senare op slår på DMA: op85–88 (`$2184–$21C0`) slår på DMA och sätter längden till 1 ord, så samplet spelas **en gång** och kanalen tystnar i en ordslinga. Op89 (`$21D4`) slår bara på DMA, så samplet **loopar** tills kanalen laddas om (hissmotorn).

Kanalerna 0 och 3 går till vänster, 1 och 2 till höger (Amiga-standard).

## Porten

- `game` registrerar bara kommandon (`AudioCmd`: Load, Start, StartOnce) i ordning, per tick (`State.Sounds`). Spelstate rörs inte.
- `pkg/audio.Mixer` har fyra kanaler, närmaste-granne-resampling (som Paula, utan filter) och 16-bitars stereo. `cmd/game` skickar kommandona efter varje tick. `-mute` stänger av ljudet.
- Avvikelser: kommandona tillämpas i tickens takt, inte på skanlinjen. Ordslingan efter ett engångssampel räknas som tystnad. Amigans lågpassfilter saknas.
- Rättat samtidigt: op79/op80-listornas startposition sätts av initkoden `$3B0`, inte av en post i filen.
