# Natt: redesign av hjemmeskjermen

Dato: 2026-10-26. Status: utkast til godkjenning.

## Hva og hvorfor

Tellulf viser i dag klokke, vær, kalender, strøm og t-bane oppå en animert
WebGL-himmel, med glasskort, animerte værikoner, regndråper, bølger og en
timegraf med 18 kolonner. Det oppleves rotete og for mye animert, og mange av
tallene endrer aldri en beslutning.

Den nye skjermen heter «Natt»: nesten svart bakgrunn, varm hvit tekst, én
aksentfarge i rav, én skrift, ingen gradienter, ingen dekorative ikoner, og
ingenting som beveger seg utenom at klokka skifter minutt. Hierarkiet er
klokke, dato, dagens middag, og så alt annet i grått. Mockupen ligger ved
siden av dette notatet som `2026-10-26-natt-mockup.webp` (1280x720,
skaleres 1,5x til kiosken) og er referansen for utseendet.

Datakildene, Go-serveren, templ, SSE-huben og kiosken på 1920x1080 er
uendret. Dette er et nytt utseende på de samme dataene.

## Mål

- Lesbart tvers over rommet. Klokka og middagen skal kunne leses fra døra.
- Ro: bare klokka endrer seg synlig i løpet av et minutt.
- Lite lys om kvelden.
- Mindre kode enn i dag.

## Ikke-mål

- Ingen nye datakilder (ikke nowcast, tømmekalender, luftkvalitet, nyheter).
- Ingen AI-generert tekst. Værsetningene er regelstyrte. (Endret
  2026-10-28: Claude skriver nå en setning i I DAG og korte titler, se
  `2026-10-28-claude-design.md`. Værsetningene er fortsatt regelstyrte.)
- Ingen lys variant. Skjermen er alltid mørk, døgnet rundt.
- Ingen berøring eller input.

## Anti-mål

Det ville være en fiasko om skjermen igjen fikk animasjoner «fordi det er
pent», om den fikk flere tall enn mockupen viser, eller om det kom inn ikoner
utover barneprikken og regnstrekene.

## Rammer

- Go + templ + SSE som i dag. Klient-JS uten byggesteg, lintet av Biome.
- Skrift: Inter Tight fra Google Fonts, vektene 200, 300 og 400, med
  `font-variant-numeric: tabular-nums` på alt som inneholder tall. Lastes
  som Lato lastes i dag.
- Alle tall på norsk: desimalkomma, «−» (U+2212) for negative tall, «–»
  (en dash) i intervaller som «5–12°», «·» som skilletegn.
- Beholder dagens mekanismer: SSE-fragmenter med `data-sse`, versjonsstyrt
  reload, midnattsreload, kalenderoverflyt i `client.js`, klientlogging til
  `/log` for sidelasting, synlighet og JS-feil.

## Layout

To kolonner på 1920x1080 med 96 px marg. Venstre kolonne er omtrent 55 % av
bredden, høyre omtrent 37 %, med luft mellom. Målene i mockupen ganget med
1,5 er utgangspunktet, utbyggeren justerer til det ser likt ut.

### Venstre kolonne, ovenfra

1. **Klokke**: tynne, store sifre (omtrent 280 px). Under: datoen som
   «Mandag 26. oktober» i grått, og etter den, svakere, «· uke 44»
   (min avgjørelse: ukenummeret bor på datolinja). Klokke og dato settes
   fortsatt av klienten som i dag.
2. **Vær nå**: temperaturen stor (omtrent 120 px) til venstre, i hele
   grader («11°»). Til høyre to
   linjer: en hovedsetning i primærfarge og en detaljlinje i grått (reglene
   står under «Værsetninger»). Temperaturen er utesensoren fra MQTT som i
   dag; mangler den, brukes prognosens temperatur for inneværende time
   (min avgjørelse, så skjermen aldri viser «–» i 120 px).
3. **Seks kolonner fram i tid** under en hårlinje: time i smått grått
   øverst, temperatur i hele grader med «°» under i større, og en tynn
   regnstrek under
   temperaturen når kolonnen har nedbør. Temperaturen blir blå i kolonner
   med nedbør. Regler under «Tretimerskolonnene».
4. **Bunnrad** under en hårlinje, tre celler (like brede som to
   timekolonner, så de flukter med dem) med liten etikett i versaler
   og hovedverdi under:
   - **T-BANE**: neste avgang som hovedverdi i rav, avgangen etter på
     linja under i grått, som detaljlinjen i de andre cellene. Ingen
     retning: alle avganger går samme vei. Avganger som er under to
     minutter unna vises ikke. Sanntidsprikken beholdes: en liten prikk
     etter klokkeslettet, rød ved forsinkelse, grå uten sanntidsdata,
     ellers ingen.
   - **STRØM HJEMME**: effekt nå i kW med én desimal («0,0 kW»), alene på
     linja. Negativ effekt vises med «−». Dagens avrunding til 0,1 kW
     beholdes så fragmentet ikke endres hvert sekund uten grunn. Under, i
     grått: månedens forbruk og hvor mye av Norgespris-kvoten på 5000 kWh
     det har brukt, «2345 kWh · 47 %» (hele kWh fra 10 og opp, én desimal
     under). Prisen vises ikke så lenge forbruket er under kvoten, siden
     hver kWh da koster den samme faste Norgespris-prisen. Når kvoten er
     brukt opp (fra 100 %), kommer prisen per kWh med to desimaler på en
     tredje linje i grått, «1,23 kr»; den er da `EffectivePrice`, altså spot.
     Uten kvote vises bare kWh på andre linje, og prisen alltid.
   - **HYTTA**: samme som hjemme, med hyttas Norgespris-kvote på
     1000 kWh: «1034 kWh · 103 %» og «1,23 kr» under.
   Mangler data for en celle, vises etiketten og ingenting under.

### Høyre kolonne, ovenfra

1. **I DAG**: liten etikett i rav. Helt til høyre på samme linje:
   soloppgang og solnedgang i grått, «SOL 07:23 · 16:37» (utelatt de dagene
   sola ikke står opp eller går ned), og dagens barneprikk. Så dagens middag stort (omtrent 48 px). Så merknader i
   grått, én per linje: bursdager som «Kai Lützenkirchen fyller 45», og
   heldagshendelser som ikke er middag eller bursdag. Så avtaler med
   klokkeslett: starttid i grått med lik bredde, tittel i primærfarge.
   Hendelser som inneholder «hytta» får hyttas vær bak tittelen i grått,
   «(5°, 20 % regn)», som i dag. Dagen uten middag viser ingen middagslinje.
2. **Hårlinje.**
3. **Kommende dager**, to linjer hver:
   - Linje 1: barneprikk, ukedag i grått, middag i primærfarge, og helt til
     høyre «5–12° · regn» i grått. Uten middag står plassen tom. «regn»
     står når dagens sannsynlighet for lett regn
     (`Daily.LightRainProbability`) er minst 50 %, ellers bare
     temperaturintervallet (min avgjørelse: mockupen sier «regn» ved
     100 % og ikke ved 20 eller 30 %).
   - Linje 2: hendelser i grått på én linje, skilt med «·»: heldags først
     uten klokkeslett, så «11:00 PT (Lena)». Bursdager som «Kai 45 år».
     Linja brytes som vanlig tekst, også inne i en avtale, men aldri
     mellom klokkeslettet og første ord i tittelen. Den får maks tre
     linjer; det som ikke får plass, erstattes av «…». Hver avtale bærer
     sin egen «·», så ingen linje starter med en prikk. Titler er kuttet
     til 50 tegn, så én avtale tar aldri mer enn to linjer. Dimmede hendelser (per
     hendelse, `EnrichedEvent.Faded` fra regelen i
     `internal/calendar/enrich.go`) vises enda svakere enn resten av linja.
     Dagen selv dimmes ikke.
   - Ukedagen står uten dato («Tirsdag») til og med seks dager fram. Fra
     dag sju og utover, når navnet ville gjentatt seg, står den med dato:
     «Mandag 2.».
   - Serveren rendrer like mange dager som i dag (`calendarDays`), og
     `client.js` skjuler hele dager nedenfra når de ikke får plass. I dag
     viser alltid alt den har og skjules aldri av overflytsjekken.
4. **Hjørnelinje** helt nede i skjermens høyre hjørne, veldig svak og
   liten (12 px): bhg-telleren og stemplene med én bokstav som etikett,
   «b» bygd, «s» start, «l» lastet, «d» data. Den ligger under kolonnens
   bunnmarg, utenfor flyten, så overflytberegningen ikke teller den med
   og den aldri møter siste dag.

### Barneprikken

Fire tilstander fra `calendar.KidsStatus`, tegnet med CSS (ikke et
tegn fra skriften), i rav:

| KidsStatus | Prikk |
|---|---|
| `KidsFull` | fylt |
| `KidsAway` | tom ring |
| `KidsArriving` | høyre halvdel fylt |
| `KidsLeaving` | venstre halvdel fylt |

## Værsetninger

Begge linjene bygges av regler fra timeprognosen for Oslo, i
`internal/view`, med tester. Ordliste fra MET-symbolet (uten `_day` /
`_night` / `_polartwilight`):

| Symbol | Ord |
|---|---|
| clearsky | «Sol» om dagen, «Klart» om natta |
| fair | «Lettskyet» |
| partlycloudy | «Delvis skyet» |
| cloudy | «Skyet» |
| fog | «Tåke» |
| lightrain, rain | «Regn» |
| heavyrain | «Kraftig regn» |
| *showers* (alle byge-symboler) | «Byger» |
| sleet* | «Sludd» |
| snow*, lightsnow, heavysnow | «Snø» |
| *thunder* | «Torden» |
| ukjent | «Skyet» |

- **Linje 1**: ordet for inneværende time, «nå», og det første
  symbolskiftet (nytt ord) innen tolv timer: «Sol nå, skyet fra kl. 14».
  Ingen skifte innen tolv timer: «Sol nå og de neste timene». Den eksakte
  formuleringen er utbyggerens, men linja skal være én setning uten
  komma-rekker og under 45 tegn.
- **Linje 2**: før kl. 15: «Opp mot 12° i dag», høyeste temperatur resten
  av dagen. Fra kl. 15: «Ned mot 4° i natt», laveste temperatur fram til
  kl. 08 i morgen. Så, etter « · », nedbør: regner det ikke nå, «regn fra
  kl. 02» for første time med minst 0,1 mm innen 18 timer; regner det nå,
  «regn til kl. 15» for første tørre time. Ingen nedbør i sikte: ingenting
  etter temperaturen. «Regn» byttes med «snø» eller «sludd» når symbolet
  den timen sier det.

## Tretimerskolonnene

- Første kolonne er første hele time som er delelig med tre og strengt
  etter inneværende time (kl. 11:56 gir 12, kl. 12:10 gir 15). Deretter
  hver tredje time, seks kolonner i alt, altså 18 timer fram.
- Kolonnen merket «15» dekker timene 15, 16 og 17. Temperaturen er
  prognosen for time 15.
- Regnstreken viser **summen** av nedbør over kolonnens tre timer, så en
  skur mellom to merker ikke forsvinner. Høyden er lineær og full ved
  9 mm (3 mm per time i snitt). Fargen har tre trinn og bestemmes av den
  våteste enkelttimen i vinduet, så en kraftig skur ikke vannes ut:
  under 0,5 mm dempet, 0,5 til under 2 mm mellomblå, 2 mm og over lysest
  (min avgjørelse om sum for høyde og maks for farge). Ingen strek når
  summen er under 0,1 mm.
- Temperaturen i en kolonne med strek settes i samme blå som det lyseste
  trinnet.
- Prognosen fra `HourlyForecasts` må rekke 21 timer fram; sjekk at
  `internal/weather` beholder nok timer.

## Farger og størrelser

Utgangspunkt, utbyggeren stiller inn mot mockupen:

| Token | Verdi | Bruk |
|---|---|---|
| `--bg` | `#121212` | bakgrunn |
| `--fg` | `#ede7dc` | primærtekst |
| `--muted` | `#9a948b` | sekundærtekst, etiketter |
| `--faint` | `#5c5852` | dimmede hendelser, hjørnelinja, hårlinjer |
| `--accent` | `#e3a53a` | «I DAG», neste avgang, barneprikk |
| `--rain-1` / `--rain-2` / `--rain-3` | `#3d4f73` / `#4c6fae` / `#6a9cf0` | regnstrek lett / moderat / kraftig, og blå temperatur |

Tekststørrelser ved 1080p: klokke 280, temperatur nå 120, middag i dag 48,
dato 40, verdiene i bunnraden 35, primærtekst 28, sekundærtekst 22,
etiketter 18 i versaler med litt sperring, hjørnelinja 12. Ingen andre. Klokka og store tall i vekt 200, brødtekst i
300, titler i 400.

## Grafiske prinsipper

Dette er krav, ikke smak. Mockupen følger dem, og implementasjonen skal
også gjøre det der mockupen ikke sier noe.

- **Ett rutenett.** Alt ligger på et 8 px-rutenett ved 1080p. Venstre
  kanter i venstre kolonne er én loddrett linje: klokka, temperaturen,
  hårlinjene og etikettene i bunnraden starter på samme x. Høyre kolonne
  har én venstrekant for etikett, middag og klokkeslett, og én høyrekant
  for temperaturintervallene. Store, tynne sifre har egne sidemarger i
  skriften, så klokka justeres optisk mot teksten under, ikke etter
  boksen.
- **Én typografisk skala.** Størrelsene i «Farger og størrelser» ligger
  på en skala med fast forhold (omtrent 1,25 mellom nabosteg under 48 px);
  utbyggeren justerer, men legger ikke til flere steg. Maks tre vekter.
  Etiketter i versaler får sperring på omtrent 0,08 em. Store sifre
  (klokke og temperatur) får litt negativ sperring, siden Inter Tight
  i vekt 200 ellers blir luftig.
- **Hierarki med størrelse, vekt og lyshet, ikke farge.** Primærtekst,
  sekundærtekst og dimmet tekst er tre tydelige lyshetstrinn. Rav brukes
  bare på det som betyr «nå» eller «neste»: etiketten «I DAG», neste
  avgang og barneprikken. Blått brukes bare på nedbør. Ingen annen farge.
- **Kontrast for lesing på avstand.** Primærtekst mot bakgrunn minst
  7:1, sekundærtekst minst 4,5:1. Dimmet tekst og hjørnelinja slipper
  kravet, men må være lesbare på nært hold. Ikke ren hvit på ren svart:
  varm hvit på nesten svart, som i mockupen, gir mindre gjenskinn om
  kvelden.
- **Nærhet før linjer.** Det som hører sammen står tett, grupper skilles
  med luft, og hårlinjer brukes bare der luft alene ikke holder (under
  været, over bunnraden, under «I DAG»). Luften mellom grupper er alltid
  større enn luften inni dem.
- **Konsekvente tall.** Tabulære sifre overalt, unntatt i løpende tekst
  som avtalelinjen, der en tabulær «1» lager et hull etter «·». Klokkeslett alltid
  «HH:MM». Grader alltid hele tall med «°» rett på tallet. kW med én
  desimal, kr med to. Enheten skilles fra tallet med et smalt mellomrom
  som ikke bryter linja.
- **Én linje er én linje.** Sekundærtekst brytes ikke; den kuttes med
  «…». Unntaket er avtalelinjen for kommende dager, som brytes opp til
  tre linjer, fordi en skjult avtale er verre enn en dag færre. Primærtekst (middag, avtaletittel) kan bryte én gang, aldri to.
- **Ingen bevegelse.** Innhold byttes uten transition. Ingenting blinker,
  glir eller toner.
- **Tomt er tomt.** Manglende data gir ingen plassholdere som «–» eller
  «ingen», bare fravær, så skjermen er roligere i uker uten barna.

## Det som fjernes

Alt som bare tjente det gamle utseendet slettes, ikke kommenteres ut:

- `internal/sky`, `internal/views/sky.templ`, `public/sky.js`, `Sky` i
  `views.Page`, `BuildSky`, `sky`-fragmentet, tema-variablene fra serveren,
  `--page-bg`, canvasen, og sky/WebGL-linjene i `report.js`.
- Timegrafen (`hourly_forecast.templ`, `BuildHourlyForecast` og regn-,
  bølge- og nullinje-logikken), `current_weather.templ` slik den er.
- `static/weather-icons-animated`, `static/weather-icons-static`,
  `wave1-3.svg`, `high/low-temperature.png`, `sunrise/sunset.png`,
  `kids.svg`, `dinner.svg`, `birthday.svg`.
- idiomorph i `static/vendor` og `data-morph` i `live.js`: ingen søyler
  animerer lenger, så vanlig innholdsbytte holder.
- Glasskort, blur, skygger, gradienter, alle `@keyframes` og `transition`
  i `styles.css`. Lato.
- Klikk på klokka for reload, og Megajouleens dag.
- Min/gjennomsnitt/maks-søylene for strøm, «Forbruk»/«Kost»-tabellen,
  fargekodet «Norgespris»/«Spotpris»-status. Prosenten mot rettlinjet
  forbruk av kvoten beholdes, og flytter til detaljlinjen.

`internal/sun` beholdes (soloppgang og solnedgang vises fortsatt).
Dokumentasjonen (`CLAUDE.md`, `README.md`) oppdateres der den beskriver
himmelen, animasjonene, idiomorph og WebGL-gotchas. Gotchaen om `Temporal`
gjelder fortsatt (ukenummer).

## Fragmenter og takt

| Fragment | Innhold | Takt |
|---|---|---|
| `weather` | temperatur nå, to setninger, seks kolonner | 15 s |
| `calendar` | hele høyre kolonne inkludert sola i «I DAG» og hjørnelinja | 15 s |
| `trains` | t-banecellen | 30 s |
| `power-home`, `power-cabin` | hver sin celle | 1 s |
| `version` | som i dag | etter snapshot og hvert minutt |

Navnene er utbyggerens, men hvert fragment må ha et `data-sse`-element,
ellers dropper `live.js` det stille.

## Verifisering

- `go test ./...`, `go vet ./...`, `gofmt -l .`, Biome på `public/`.
- Tester i `internal/view` for: ordlista og begge setningene (tabelldrevet,
  med «nå» morgen og kveld, med og uten regn, regn nå), kolonnevalget ved
  ulike klokkeslett, sum/maks-regelen for streken, t-banefilteret på to
  minutter, kvoteprosenten, prikk-tilstandene, ukedag med og uten dato.
- Skjermbilde i ekte 1920x1080 med headless Chrome (se memory-notatet om
  skjermbilder) sammenlignet side om side med mockupen skalert 1,5x, både
  med og uten MQTT- og Tibber-data.
- Sjekk at hjørnelinja aldri overlapper siste dag, og at en dag med åtte
  avtaler i dag gir færre kommende dager, ikke overflyt.

## Overlatt til utbyggeren

Nøyaktige piksler, grid eller flex, hvordan halvprikken tegnes, de
eksakte fargeverdiene innenfor kontrastkravene, formuleringen av «ingen
skifte»-varianten i linje 1, fragmentnavnene, og rekkefølgen arbeidet
gjøres i. Små avgjørelser som gjennomleseren pekte på og som utbyggeren
tar selv: «Kai 45 år» bruker fornavnet, flere middager samme dag skilles
med « · », flerdagers avtaler på kommende dager viser bare starttiden, og
hytta-været vises bare under «I DAG».
