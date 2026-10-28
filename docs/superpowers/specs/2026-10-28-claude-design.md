# Claude: dagens setning, korte titler og uka

Dato: 2026-10-28. Status: implementert, med Haiku som standardmodell.

## Hva og hvorfor

Skjermen viser vær, kalender, barneuker og hytta hver for seg, etter faste
regler. Det reglene ikke kan, er å se to ting i sammenheng («regn akkurat
når barna kommer»), å rydde i rotete kalendertitler, eller å si hva som er
verdt å tenke på i uka som kommer. Det klarer en språkmodell. Claude skriver
nå tre ting:

1. **Dagens setning**: én linje i grått under middagen i I DAG.
2. **Korte titler og merknader**: lange titler blir korte på oversikten, og
   kalendersiden får én nyttig ting fra beskrivelsen under avtalen.
3. **Uka**: en ny modus, `uke` (`/uke`), med noen få linjer om dagene som
   kommer, ved siden av dagene slik oversikten lister dem.

Natt-notatet hadde «Ingen AI-generert tekst» som ikke-mål. Det er endret
med dette notatet. Værsetningene er fortsatt regelstyrte, og alt annet på
skjermen er som før.

## Mål

- Tekstene sier noe skjermen ikke allerede sier, eller ingenting.
- Ingen tall fra lufta: hvert tall og klokkeslett i en tekst må stå i
  dataene den ble skrevet fra, ellers forkastes teksten.
- Skjermen ser ut som før når Claude mangler, feiler eller ikke har svart
  ennå. Fravær, aldri «–» eller en feilmelding.
- Billig: noen få kall i timen.

## Ikke-mål

- Ingen nye datakilder. Claude får bare det skjermen allerede vet.
- Ingen tekst skrives mens en side rendres. Alt skrives i bakgrunnen.
- Tellulf bytter ikke selv til `uke` søndag kveld. Det er automasjonens jobb,
  som for de andre modusene.
- Ingen lagring mellom omstarter: titlene spørres om igjen etter en omstart.

## Dagens setning

Under middagen, før bursdagene, i samme stil som merknadene (22 px grått).
Uten middag rett under etiketten. Tom setning betyr ingen linje.

Claude får timen nå, i dag og i morgen fra kalenderen (barna, middag,
bursdager og avtaler; ferdige avtaler og dimmede utelatt), timevarselet for
Oslo 24 timer fram, og hyttas dagsvarsel for dagene noen er der. Systemprompten
forklarer hva skjermen allerede viser, og ber om en sammenheng mellom to ting,
eller tom setning. Etter kl. 18 kan den handle om i morgen.

- Skrives på nytt når inndataene endres, høyst én gang i timen. Inndataene
  nevner timen, så i praksis én gang i timen, og med en gang når dagen skifter.
- Vises bare på dagen den ble skrevet for, og bare så lenge inndataene er
  sett uendret de siste 90 minuttene. Feiler Claude i timevis, forsvinner
  setningen heller enn å bli stående om noe som har skjedd.
- Forkastes når den er lengre enn 100 tegn eller har et tall som ikke står
  i inndataene.

## Korte titler og merknader

Avtaler med tittel over 30 tegn, eller med beskrivelse, sendes til Claude
i bunter på høyst 25. Beskrivelsen gjøres om fra HTML til ren tekst og
kuttes ved 1500 tegn. Svaret er en kort tittel og en merknad (hva man skal
ha med, når man møter opp), begge kanskje tomme.

- Nøkkelen er tittel og beskrivelse, så alle avtalene i en serie deler ett
  svar, og en endret avtale spørres om på nytt.
- Hver avtale spørres om én gang. Hopper Claude over en, får den et tomt
  svar og spørres ikke igjen. Avtaler som er borte fra kalenderen, glemmes.
- Den korte tittelen brukes bare når originalen er over 30 tegn, er under
  40 tegn selv, og ikke har tall som ikke står i originalen. Merknaden må
  være under 60 tegn, ha tallene sine fra tittel eller beskrivelse, og ikke
  bare gjenta tittelen.
- **Oversikten**: den korte tittelen erstatter tittelen i I DAG og på de
  kommende dagene. Uten en kort tittel kuttes tittelen ved 50 tegn som før.
- **Kalendersiden**: hele tittelen som før, unntatt titler over 50 tegn
  (de som oversikten kutter, og som uansett ikke får plass på to linjer).
  Der står den korte. Merknaden får sin egen linje under detaljlinja, i
  samme stil. Heldagshendelser får den bak « · ».

## Uka

Ny modus `uke`, URL `/uke`, med samme ramme som de andre detaljsidene og
topplinja «Uka». Samme rutenett som oversikten:

- **Venstre** (912 px): Claudes linjer, én setning hver, i 35 px
  primærfarge med 32 px mellom.
- **Høyre** (702 px): de sju dagene som linjene handler om, bygd akkurat som
  de kommende dagene på oversikten (samme komponent og CSS-klasser). Første
  dag heter «I dag» når den er i dag. Dager som ikke får plass, skjules
  nedenfra av den samme sjekken i `client.js`.

Uka begynner i dag, og fra kl. 18 i morgen. Claude får hver dag med barna,
middag (også «ingen planlagt» når middagskalenderen er i bruk), bursdager,
avtaler, Oslo-været og hyttas vær de dagene noen er der. Systemprompten ber
om én til fem linjer i prioritert rekkefølge: barnebytter, bursdager, dager
uten middag, travle dager, værskifter og hyttevær. Ikke en opplisting av
avtalene, som står ved siden av.

- Skrives på nytt når inndataene endres, høyst hver tredje time, og med en
  gang når uka begynner på en ny dag. Inndataene nevner dagen, ikke timen.
- Vises bare for uka den ble skrevet for, og så lenge inndataene er sett
  uendret de siste seks timene.
- Hver linje over 140 tegn eller med et tall som ikke står i inndataene
  fjernes for seg. Er ingen linjer igjen, regnes det som en feil.
- Uten linjer (ingen Claude, eller ikke klart) viser siden bare dagene.

Fragmentet `uke` rendres hvert 15. sekund som de andre datafragmentene.

## Modell, kall og feil

- Den offisielle Go-SDK-en, `github.com/anthropics/anthropic-sdk-go/v2`,
  med strukturert output (JSON-skjema) og adaptiv tenkning. Innsats
  `medium` for setningen og uka, `low` for titlene.
- Modellen er `claude-haiku-5-5` til vi vet om den holder; den kan byttes
  med `TELLULF_CLAUDE_MODEL`, for eksempel til `claude-opus-5-5` hvis
  Haiku skriver for klønete.
- Uten `ANTHROPIC_API_KEY` er Claude av, og loggen sier hvorfor.
- Første runde starter et minutt etter oppstart, så kalenderen og varselet
  har kommet. Deretter sjekkes alt hvert minutt.
- Et kall som feiler (nettverk, avslag, kuttet svar, forkastet tekst) prøves
  igjen etter ett minutt, så dobbelt så lenge for hver feil på rad, høyst
  30 minutter. Hver av de tre tekstene har sin egen takt.
- Nye tekster logges på info-nivå («claude: today», «claude: week»,
  «claude: title»), så det er lett å se hva Claude skriver. Tokens og
  svartid per kall logges med `TELLULF_DEBUG`.

## Kostnad

Målt med Haiku på en oppdiktet uke: setningen bruker rundt 1500 tokens inn
og 800 ut, uka 1500 inn og 2500 ut (mest tenkning), en bunt titler 1000 inn
og 150 ut. Setningen én gang i timen, uka opptil åtte ganger om dagen og
titlene når kalenderen endres blir under 10 kr i måneden. Opus koster 40
ganger så mye per token, altså et par hundre kroner i måneden.

## Personvern

Kalenderens titler, beskrivelser, steder og barneukene sendes til Anthropics
API. Beskrivelser fra invitasjoner kan inneholde møtelenker og passord; de
sendes som de er, og prompten ber Claude la dem ligge.

## Prøve promptene

`go test -tags live -run Live -v ./internal/claude` skriver alle tre fra en
oppdiktet uke mot det ekte API-et (med `ANTHROPIC_API_KEY`;
`TELLULF_CLAUDE_MODEL` og `LIVE_AT=2026-10-29T19:00` kan settes). Det
koster et par øre, og er måten å sammenligne modeller og justere promptene
på.
