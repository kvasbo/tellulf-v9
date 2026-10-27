# Moduser: strøm, kalender og vær i full detalj

Dato: 2026-10-27. Status: utkast til godkjenning.

## Hva og hvorfor

Skjermen viser i dag én side, oversikten («Natt»). Den skal kunne bytte
mellom flere sider, moduser, styrt fra MQTT, for eksempel fra en automasjon
eller en knapp. Første utgave har fire: oversikten som i dag, **Strøm** med
alt Tibber vet om hjemme og hytta, **Kalender** med de neste to ukene i
full detalj, og **Vær** med de neste 48 timene og dagene framover for Oslo
og hytta. Hver modus har sin egen URL, og skjermen bytter mellom dem med en
rolig page transition.

Skjermbildene ved siden av dette notatet er tatt av implementasjonen med
testdata (`2026-10-27-strom.webp`, `2026-10-27-kalender.webp`,
`2026-10-27-vaer.webp`, 1280x720, skaleres 1,5x til kiosken). Natt-notatet
(`2026-10-26-natt-redesign-design.md`) gjelder fortsatt for alt som ikke
står her: farger, skrift, typografisk skala, rutenett og tallformat.

## Mål

- Én MQTT-melding bytter side på alle åpne skjermer innen et par sekunder.
- Detaljsidene ser ut som Natt: samme farger, skrift, skala, etiketter og
  marger, og samme CSS-klasser der det er samme ting. Ingen nye farger og
  ingen nye skriftstørrelser.
- Byttet er rolig: siden toner over, og klokka glir fra plassen sin på én
  side til plassen sin på neste.
- Hver side kan åpnes direkte på sin URL, også uten MQTT.

## Ikke-mål

- Ingen berøring eller klikk for å bytte side.
- Ingen automatisk retur til oversikten. Det er avsenderens jobb.
- Ingen nye datakilder utover det Tibber og Google Calendar allerede gir.
- Ingen beskrivelser fra kalenderen: de er lange, ofte HTML og møtelenker.
- Ingen fasestrøm eller spenning fra Tibber: mange målere sender dem bare
  hvert tiende sekund, og de endrer ingen beslutning.

## Moduser og URL-er

| Modus | URL | Innhold |
|---|---|---|
| `oversikt` | `/` | Natt, uendret |
| `strom` | `/strom` | Tibber for hjemme og hytta |
| `kalender` | `/kalender` | i dag og 13 dager fram, med alle detaljer |
| `vaer` | `/vaer` | været nå, de neste 48 timene og de neste 12 dagene |

## MQTT

- Emne `tellulf/mode`. Payload er modusens navn eller URL: `oversikt`,
  `strom` (også `strøm`), `kalender`, `vaer` (også `vær`), eller `/`,
  `/strom`, `/kalender`, `/vaer`.
  Store og små bokstaver og mellomrom rundt spiller ingen rolle. Tom payload
  betyr oversikten.
- Ukjent payload logges som advarsel og endrer ingenting.
- **Tilbake til oversikten etter en fast tid.** Tiden settes med
  `TELLULF_MODE_TIMEOUT` (Go-varighet som `90s` eller `5m`, eller sekunder;
  `0` betyr aldri) og er to minutter når den ikke er satt
  (`mode.DefaultTimeout`). En annen side enn oversikten står så lenge fra
  siste melding; en ny melding, også om samme side, starter tiden på nytt.
  Deretter går skjermene tilbake til oversikten, med samme sideskift.
- Send uten retain (`mosquitto_pub -t tellulf/mode -m strom`). En retained
  melding ignoreres: brokeren spiller den av på nytt ved hver
  gjentilkobling, og da ville en gammel side kommet tilbake for en ny
  periode.
- **Serveren eier tilstanden.** Med MQTT konfigurert starter den på
  oversikten og sender modusen til åpne sider over SSE (se under). Den
  redirecter aldri: hver URL åpnes som den blir bedt om, også fra en annen
  maskin. Uten MQTT (utvikling) finnes ingen modus.
- **Alle sider følger endringer i modusen; ingen ting huskes i
  nettleseren.** Den første verdien en side får (øyeblikksbildet), er
  modusen slik den står: oversikten går dit med en gang, og en side som
  allerede er modusens, blir der til serveren sier noe annet. Enhver annen
  side er åpnet direkte (eller ble stående etter en omstart): den blir
  stående og går til oversikten etter samme tid (aldri med `0`), med mindre
  modusen endres før det. Slik kan hver side åpnes direkte, kiosken følger
  MQTT, og i utvikling holder `TELLULF_MODE_TIMEOUT=0` en side oppe så
  lenge man vil. Tiden følger med til klienten som `data-mode-timeout` på
  `<body>`.

## Sideskift

- Klienten får modusen som SSE-hendelsen `mode`, med URL-en som data, og
  navigerer etter reglene over (`location.assign`).
- Sidene bruker cross-document view transitions
  (`@view-transition { navigation: auto }`, Chrome 126+). Den gamle siden
  toner ut (0,25 s), og så toner den nye inn (0,3 s), gjennom den mørke
  bakgrunnen, så to fulle sider aldri ligger oppå hverandre. Klokka og
  datoen blir stående hele tiden og glir og skalerer fra plassen sin på én
  side til plassen sin på neste (`view-transition-name`, 0,6 s). På
  oversikten er klokka 280 px, på detaljsidene 40 px oppe til høyre.
  Temperaturen nå glir på samme måte mellom oversikten og værsiden; finnes
  den bare på én av sidene, toner den med siden.
- **Unntak fra «Ingen bevegelse»:** dette er den eneste animasjonen, og den
  skjer bare når modusen skifter. Reload (ny versjon, midnatt) animeres
  ikke, og fragmentbytter er fortsatt uten transition.
  `prefers-reduced-motion: reduce` slår sideskiftet av.
- Serveren rendrer klokka, datoen og uka inn i HTML-en (samme format som
  `client.js`), så den nye siden er komplett i første bilde og klokka har
  noe å gli til. Klienten tar over derfra som før.

## Felles ramme for detaljsidene

- Samme marger som oversikten: 96 px på sidene, 80 px over, 40 px under.
- Øverst en topplinje: sidens navn til venstre i 40 px primærfarge
  («Strøm», «Kalender»); til høyre datoen i 40 px grått og klokka i 40 px
  primærfarge, med 24 px mellom. 56 px luft under. Ingen hårlinje.
- Hjørnelinja med stemplene (b, s, l, d) ligger i samme hjørne som på
  oversikten. Bhg-telleren står bare på oversikten.

## Strøm

To like kolonner, 808 px brede med 112 px mellom: HJEMME til venstre,
HYTTA til høyre. Kolonnene er bygd likt, ovenfra:

1. **Etikett**: «HJEMME», «HYTTA».
2. **Effekt nå**, stort som temperaturen på oversikten: «2,4» i 120 px
   vekt 200, og «kW» i 40 px grått etter. Negativ effekt (hytta
   produserer) med «−». Til høyre to linjer som værlinjene: dagens snitt og
   topp i primærfarge, «Snitt 1,1 kW · topp 6,2 kW», og dagens laveste i
   grått, «Lavest 0,3 kW i dag». Har stedet produsert i dag, kommer
   «· sol topp 3,4 kW» etter.
3. **Hårlinje**, så tre rader. Hver rad er en tekstblokk til venstre
   (248 px, samme stil som cellene i bunnraden: etikett, verdi 35 px,
   detaljlinjer 22 px grått) og en graf til høyre (520 x 144 px):
   - **I DAG**: forbruket i dag (netto, som på oversikten), kostnaden, og
     «sol 3,1 kWh» når stedet har produsert. Grafen er forbruk per time,
     én plass per time fra midnatt til midnatt (23 eller 25 på dager med
     skifte til eller fra sommertid).
   - **Månedens navn** («OKTOBER»): månedens forbruk, andelen av
     Norgespris-kvoten («47 % av kvoten»), tempoet («+20 % mot tempo»:
     forbruket mot rettlinjet bruk av kvoten, andel av måneden gått ganger
     kvoten, som i den gamle cellen) og månedens kostnad. Grafen er
     forbruket akkumulert dag for dag, med en tynn diagonal for rettlinjet
     bruk av kvoten (0 ved månedens start, kvoten ved slutten) og en
     hårlinje for selve kvoten. En søyle over diagonalen betyr at kvoten
     tar slutt før måneden. Skalaen går til kvoten, eller lenger når
     forbruket har passert den. Uten kvote: bare forbruk og kostnad, og
     ingen linjer.
   - **SPOTPRIS**: prisen nå (med mva, som Tibber oppgir den), dagens
     laveste og høyeste («0,19–0,57 kr i dag») og Norgespris nå
     («Norgespris 0,50 kr»). Grafen er pris per time i dag og i morgen, 48
     plasser; morgendagen fylles inn når Tibber publiserer, rundt kl. 13.
     Norgespris er en tynn linje. Negative priser går under en nullinje.

Grafene: søyler i `--faint`, timen eller dagen nå i `--accent` (rav betyr
«nå», som ellers), referanselinjene (Norgespris, diagonalen) 1 px
`--muted`, kvotelinja og nullinja 1 px `--faint`. Ingen y-akse og ingen
tall inni grafen; tallene står i tekstblokka. Under grafen en akse i 18 px
grått: «00 06 12 18» for timer, «1. 8. 15. 22. 29.» for dager, og for
prisene ukedagen («ons») der morgendagen begynner. Timer og dager uten data
har ingen søyle.

Uten data fra live-feeden vises bare etiketten, som cellene på oversikten.
Mangler en rad data (ingen priser ennå, ingen historikk), vises etiketten og
ingenting under.

### Data

- **Forbruk per time i dag.** Tibbers forbrukshistorikk har dagens timer
  med en til to timers forsinkelse. Timene etter det fylles fra
  live-feeden: serveren noterer akkumulert forbruk hver gang en måling
  krysser en hel time, men bare når forrige måling var under ett minutt før
  timeskiftet (ellers ville et hull i feeden blitt regnet inn i én time).
  Timen nå er akkumulert forbruk nå minus merket ved timens start.
  Historikken vinner der begge har en time. Etter en omstart er timene
  mellom siste historikktime og omstarten tomme til historikken tar dem
  igjen.
- **Månedens dager**: summert fra den samme timehistorikken som månedens
  forbruk regnes ut fra i dag; dagen i dag er live-forbruket.
- **Priser**: i dag og i morgen fra samme spørring som i dag. I tillegg
  hentes listen på nytt fra kl. 13 når den mangler morgendagen, høyst hvert
  kvarter.

## Kalender

I dag og 13 dager fram, i tre spalter (528 px brede, 72 px mellom) som
fylles ovenfra og ned, venstre spalte først. En dag deles aldri mellom to
spalter; dager som ikke får plass i de tre, vises ikke.

Hver dag er bygd som «I DAG»-blokka på oversikten, med middagen i 28 px i
stedet for 48:

1. **Dagslinje**: ukedag og dato som etikett, «ONSDAG 28. OKTOBER»; i dag
   «I DAG» i rav. Til høyre været i samme etikettstil,
   «5–12° · 60 % REGN» (sannsynligheten står når den er over null), og
   barneprikken.
2. **Middag** i 28 px vekt 400. Flere skilles med « · ».
3. **Merknader** i grått, én per linje: bursdager med fullt navn og alder
   («Kai Lützenkirchen fyller 45»), så heldagshendelser. En heldagshendelse
   over flere dager får «til og med fredag». Hytta-vær bak heldagshendelser
   som nevner hytta, som på oversikten.
4. **Avtaler**: starttid i grått i fast kolonne (96 px) og hele tittelen
   (ikke kuttet til 50 tegn) i primærfarge, høyst to linjer. Under
   tittelen en detaljlinje i grått, kuttet med «…»:
   - slutten: «til 13:30», over flere dager «til fredag 14:00»;
   - stedet: første del av adressen, før første komma
     («Storgata 1, 0155 Oslo» blir «Storgata 1»);
   - hyttas vær når tittelen nevner hytta, «5°, 20 % regn».

   En avtale som startet en tidligere dag har tom tidskolonne. Dimmede
   avtaler (barna borte) er svakere, som på oversikten.

Ukedagen i «til fredag» får dato når den er sju dager eller mer unna,
«til mandag 9. november». Avtaler som er ferdige, vises ikke (Google
filtrerer dem bort, som i dag). Kalenderhendelsene har fått `Location` fra
Google.

## Vær

Samme rutenett som oversikten (912 og 702 px, 114 px mellom).

**Venstre kolonne**, ovenfra:

1. Etiketten «OSLO».
2. **Været nå**, bygd som på oversikten: temperaturen i 120 px (utesensoren,
   ellers prognosen) og de to setningene. Under dem, i grått, fuktighet og
   lufttrykk fra sensorene når de finnes: «Fuktighet 81 % · lufttrykk
   1013 hPa».
3. **Hårlinje**, så tre rader som på strømsiden, med grafen 624 px bred og
   48 timer fram fra timen nå. Tekstblokkene oppsummerer de neste 24
   timene, så «kl. 15» aldri kan bety to dager:
   - **TEMPERATUR**: «2–11°», «Varmest kl. 15», «Kaldest kl. 03» («nå» når
     det er timen nå). Grafen er en kurve i primærfarge, skalert med 2° luft
     over og under, med nullinja når den er innenfor.
   - **NEDBØR**: summen, «3,8 mm», eller «Opphold»; når det starter eller
     slutter («Regn fra kl. 00», samme regel som linje 2 på oversikten); og
     den våteste timen, «Mest 2,6 mm kl. 01». Grafen er nedbør per time i de
     tre regnfargene fra oversikten (under 0,5 mm, under 2 mm, over), med
     skala til minst 2 mm, så yr blir liten. Timer under 0,1 mm har ingen
     søyle.
   - **VIND**: vinden nå, «3 m/s», kastene («Kast 6 m/s»), retningen den
     kommer fra («Fra sørvest», åtte retninger), og den sterkeste timen
     når det ikke er nå. Grafen er vind per time som søyler og kastene som
     en tynn grå kurve over.

   Aksen viser «06 12 18» og ukedagen der et nytt døgn starter («ons»), og
   en svak loddrett linje skiller døgnene.

**Høyre kolonne**, ovenfra:

1. **SOL** som en tekstblokk: «07:26–16:34», dagslengden «9 t 9 min
   dagslys», og endringen fra i går, «5 min kortere enn i går».
2. **Hårlinje** på samme høyde som den til venstre, så en tabell over de
   neste 12 dagene fra i morgen: ukedag i grått (med dato fra dag sju, som
   på oversikten), og for OSLO og HYTTA temperaturintervallet i
   primærfarge og sannsynligheten for regn i grått, i regnblått fra 50 %.

Hyttas timeprognose hentes ikke; hytta har dagene. Uten prognose vises
bare været nå (eller ingenting) og tabellen uten tall.

## Fragmenter og takt

| Fragment | Innhold | Takt |
|---|---|---|
| `strom-home`, `strom-cabin` | hver sin kolonne på strømsiden | 1 s |
| `kalender` | alle dagene på kalendersiden | 15 s |
| `vaer` | hele innholdet på værsiden | 15 s |
| `mode` | modusens URL; sendt når den endres, og med i øyeblikksbildet | 1 s |

Hver side får bare sine egne fragmenter: `live.js` sender navnene den lytter
på i URL-en (`/sse?events=weather,calendar,…,version,mode`), og serveren
hopper over resten. Uten parameteren sendes alt, som før. Ellers ville
oversikten fått strømsidens fragmenter hvert sekund døgnet rundt.

## Verifisering

- `go test ./...`, `go vet ./...`, `gofmt -l .`, Biome på `public/`.
- Tester for: modusparsing, MQTT-emnet, timesporingen fra live-feeden
  (timeskifte, hull i feeden, midnatt), sammenslåing med historikken (også
  på tvers av tidssoner), dagssummene, prisgrafens akse og nå-time,
  månedsgrafens diagonal og skala, detaljlinjene i kalenderen (slutt over
  flere dager, sted, hytta), værradene (oppsummering, regnfarger, døgnskille,
  kast), sola, rutene (som aldri redirecter), SSE-filteret og at modusen er med i
  øyeblikksbildet.
- Skjermbilder i ekte 1920x1080 av alle fire sider med testdata, og av
  strømsiden med Tibbers demohjem.
- Sideskift prøvd ende til ende med en lokal MQTT-broker: melding inn,
  skjermen bytter med overtoning, reload beholder siden.

## Overlatt til utbyggeren

Nøyaktige piksler innenfor rutenettet, søylebredde og mellomrom, hvordan
grafene tegnes (SVG), varigheten og kurven på toningen, og ordlyden i
detaljlinjene så lenge den følger tallformatet fra Natt.
