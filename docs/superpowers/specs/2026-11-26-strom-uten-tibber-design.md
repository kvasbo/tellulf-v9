# Strøm uten Tibber hjemme

Dato: 2026-11-26. Status: implementert (alternativ B). Sanntid hjemme venter på HAN-leseren.

## Hva og hvorfor

Hjemme slutter vi med Tibber som strømleverandør. Da forsvinner Tibbers
live-feed (Pulse), timeforbruket og prisene for hjemme. Hytta beholder
Tibber foreløpig.

- **Forbruk hjemme** kommer fra Elvia (nettselskapet), med et personlig
  token fra Min side (`ELVIA_TOKEN`).
- **Spotpris** kommer fra hvakosterstrommen.no, både hjemme og på hytta
  («erstatt det for Tibber overalt»). Tibbers prisliste brukes ikke lenger.
- **Sanntidseffekt hjemme** venter: en HAN-leser er bestilt og skal sende på
  MQTT om noen dager. Den kommer som en egen endring, men denne må gjøre plass
  til den.

## Mål

- Hjemme vises det som kan vises uten sanntid: dagens forbruk og kostnad
  fram til siste hele time, månedens forbruk mot Norgespris-kvoten,
  månedens kostnad, timegrafen for i dag, dagsgrafen for måneden og
  spotprisen i dag og i morgen.
- Hytta ser ut som før, men med pris fra hvakosterstrommen.
- Kostnaden regnes likt begge steder: time for time, Norgespris opp til
  kvoten, spotpris over.
- HAN-leseren kan senere kobles inn som live-kilde for hjemme uten at
  noe annet må bygges om.

## Ikke-mål

- Sanntidseffekt hjemme (HAN-leseren, egen endring).
- Makstimer og kapasitetsledd fra Elvia (`maxhours` virker, men vises ikke nå).
- Nettleiepriser (Elvias Gridtariff-API krever egen nøkkel).
- Tibbers påslag og månedsavgift. Prisen er ren spotpris med mva.

## Anti-mål

- Hjemme blir blankt fordi det ikke finnes live-data. Uten feed skal timene
  fra Elvia vises.
- Gamle tall som ser ut som nå. Dagens tall uten live-feed sier hvor langt
  de rekker («til kl. 13»).
- Kall som bryter vilkår eller hamrer på et gratis API: prisdager som er
  hentet, hentes ikke igjen.

## Det vi vet om kildene (prøvd 2026-11-26)

**Elvia, måleverdier.**
`GET https://elvia.azure-api.net/customer/metervalues/api/v1/metervalues?startTime=…&endTime=…&includeProductionSteps=false`
med `Authorization: Bearer <token>`. Tidene er ISO 8601 med offset (`+` må
URL-kodes). v2 gir 404 på denne stien. Svar:

```json
{"meteringpoints":[{"meteringPointId":"7070…","customerContract":{…},
  "metervalue":{"fromHour":"…","toHour":"…","resolutionMinutes":60,
    "timeSeries":[{"startTime":"2026-11-26T12:00:00+01:00","endTime":"2026-11-26T13:00:00+01:00",
      "value":11.53,"uom":"kWh","production":false,"verified":false}]}}]}
```

Bare timer (å be om 15 minutter gir fortsatt 60). Klokka 13:40 var timen
12–13 med, så forsinkelsen er under en time. Dagens timer er `verified:
false` og kan justeres litt; de brukes som de er og erstattes ved neste
henting. Tokenet varer til 2031 og dekker bare måleren hjemme. Ett
målepunkt; har svaret flere, summeres de.

**hvakosterstrommen.no.**
`GET https://www.hvakosterstrommen.no/api/v1/prices/2026/11-26_NO1.json`,
ingen nøkkel. Liste av
`{"NOK_per_kWh":2.13283,"EUR_per_kWh":…,"EXR":…,"time_start":"…+01:00","time_end":"…+01:00"}`.
Prisen er uten mva; i NO1 ganges med 1,25. 24 oppføringer per dag i dag.
Kommer det kvarterspriser, slår `spotpris` dem sammen til timer (snittet),
så resten av koden bare ser timepriser, som Elvias timer.
Morgendagen var ute 13:40 (404 før den er publisert). Fortid ligger der
også, så månedens priser kan hentes dag for dag.

**Prisområde.** Hjemme (Oslo) og hytta (59,13° N 10,32° Ø, Vestfold) er
begge NO1. Ett område for begge, som en konstant.

## Alternativer

- **A. `tibber` setter fortsatt sammen tallene**, og kaller nye klienter for
  Elvia og priser. Minst endring, men pakken som heter Tibber ville
  inneholde Elvia, hvakosterstrommen og snart HAN-leseren.
- **B. Ny pakke `internal/power` setter sammen tallene** (valgt). `tibber`
  krymper til en klient for hytta, og `elvia` og `spotpris` er nye
  klienter. HAN-leseren blir en ny live-kilde i `power`.
- Et felles grensesnitt for alle kilder ble forkastet: to kilder med ulike
  data (live og timer mot bare timer) gir mer abstraksjon enn nytte.

## Design

### Pakker

- **`internal/spotpris`**: `New(area string)`, `Day(ctx, day) ([]Price,
  error)`. `Price{Start time.Time; Total float64}`, én per time, kr/kWh med
  mva (kvarterspriser snittes til timer). 404 er `ErrNotPublished`. Ingen
  tilstand.
- **`internal/elvia`**: `New(token)`, `Hours(ctx, from, to) ([]Hour, error)`.
  `Hour{Start time.Time; KWh float64}`. Ingen tilstand. Et avvist token
  logges og prøves igjen med backoff som andre feil: Elvia har ingen regel
  mot det, og tokenet varer til 2031.
- **`internal/schedule`**: får `Backoff(attempt)` (dagens `backoff` fra
  `tibber`, 10 s til 30 min med jitter), som feeden, prisene og timene
  bruker.
- **`internal/tibber`**: krymper til en klient for ett hjem (hytta):
  `New(token, homeID)`, `RunFeed(ctx, func(Measurement))` (dagens feed, med
  reconnect-reglene uendret) og `Hours(ctx, last int) ([]Hour, error)`
  (dagens timeforbruk, uten kostnad). Prisspørringen fjernes.
  `Measurement` blir eksportert (dagens `liveMeasurement`). `ErrUnauthorized`
  blir eksportert.
- **`internal/power`**: overtar `Place` (`Home`, `Cabin`), `PowerData`,
  `Details`, `Usage`, `PricePoint`, Norgespris og `liveHours`/`mergeHours`/
  `dayTotals` fra `tibber`. Importerer de tre klientene; ingen av dem
  importerer `power`. `view` og `web` bytter fra `tibber.` til `power.`.

### Tilstand og løkker i `power`

`power.Power` har en `Run(ctx)` og tilstand bak en mutex, som andre kilder.
Den lagrer rådata, ikke ferdige summer:

- `prices`: prislister per dag (Oslo-dato), hentet én gang hver.
- `hours[place]`: månedens timer fra historikken (Elvia hjemme, Tibber på
  hytta), byttet ut hele ved hver henting.
- `live[place]`: siste live-måling og `liveHours`-merkene (bare hytta nå).

Løkker:

- **Priser**, hvert minutt: hent dager som mangler, fra den 1. i måneden til
  i dag, og i morgen fra kl. 13 (høyst hvert 15. minutt til den finnes).
  Feil gir backoff som i dag. Dager som er hentet, hentes ikke igjen. Når
  måneden skifter, kastes dager før den 1.
- **Timer hjemme** (Elvia), hvert 15. minutt: timene fra midnatt den 1. til nå.
- **Timer på hytta** (Tibber), én gang i timen med dagens `monthlyPace`
  (flyttet til `power`). Avvist Tibber-token venter til neste time, som i dag.
- **Feed på hytta**: som i dag, målingene går til `power`.

En kilde som mangler (ingen token), gir ingen løkke, og den plassen er
blank. Prisene trenger ingen nøkkel og går alltid når `power` går.

### Tallene regnes ut når de leses

`Data(p, now)` og `Details(p, now)` regner ut alt fra rådataene (noen hundre
timer, billig selv hvert sekund). Det fjerner dagens mellomlagrede
`monthBeforeToday`/`monthCostBeforeToday` og problemet med å telle gårsdagen
to ganger.

- **Live er gjeldende** når siste måling er under 10 minutter gammel og fra i
  dag (dagens `powerIsCurrent`, flyttet til `PowerData.IsLive(now)`).
- **I dag**: med live gjeldende, feedens `Accumulated*` som i dag. Ellers
  summen av dagens timer fra historikken, og `TodayUntil` = slutten av siste
  kjente time (null når live er gjeldende).
- **Måneden**: timene før i dag + i dag.
- **Kostnad**: time for time gjennom måneden, Norgespris til kvoten er brukt
  og spotpris for den timen over. Med live gjeldende regnes dagens del som i
  dag (dagens totale forbruk delt ved kvoten, prisen nå på markedsdelen).
  En time over kvoten uten pris teller som 0 kr (kort tid etter oppstart).
- **Pris nå**: den siste timeprisen i listene som startet før nå. Normalt er
  det timen nå; mangler den, blir det siste kjente, som i dag, uten egen
  tilstand.
- **Hytta uten live**: Tibbers timer er brutto forbruk, feeden er netto (minus
  sol). Uten live viser «i dag» brutto fra timene. Det er slik måneden
  allerede regnes (brutto dager før i dag), og om vinteren er forskjellen
  liten.
- `MonthKnown`: timene for måneden er hentet minst én gang.

### Visning

- `HasData` (strøm-cellen og strømsiden) er sann når live er sett eller
  `MonthKnown`. Hjemme blir dermed ikke blankt uten feed.
- Effekt (kW, snitt, topp, lavest, sol) vises bare når live er gjeldende,
  som i dag.
- **I dag-raden uten live**: forbruk og kostnad fra timene, med detalj
  «til kl. 13» (slutten av siste kjente time). Ingen kjente timer i dag gir
  ingen verdi. Dette gjelder også hytta når feeden er borte; i dag skjules
  verdien da.
- Ellers uendret: månedsraden, prisraden og strøm-cellen.

### Konfigurasjon

- `ELVIA_TOKEN` slår på hjemme.
- `TIBBER_KEY` og `TIBBER_ID_CABIN` slår på hytta. `TIBBER_ID_HOME` fjernes.
- Med ingen av dem: «power disabled» som i dag. Med én: den andre plassen er
  blank, med en advarsel i loggen.
- `docker-compose.yml`, README og CLAUDE.md oppdateres. Produksjonsoppsettet
  i Jeeves-dockerfiler trenger også `ELVIA_TOKEN`; det pushes ikke herfra.

### HAN-leseren senere

`power` får da en metode for live-målinger hjemme (samme form som Tibbers
`Measurement`, eller en egen), og alt over virker uendret: `IsLive`, i dag fra
live, `liveHours` som fyller timene Elvia ikke har ennå.

## Overlatt til den som bygger

- Navn på interne felt og hjelpefunksjoner.
- Hvordan testene for `tibber` fordeles mellom `tibber` og `power`. Logikken
  som flytter, tar testene med seg.
- Formuleringen av loggmeldinger.
