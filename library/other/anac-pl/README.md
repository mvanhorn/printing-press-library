# ANAC Pubblicità a Valore Legale CLI

**Cerca bandi, esiti e avvisi di gara della piattaforma ANAC dalla riga di comando, con dettaglio JSON, cronologia, store locale e ricerca offline.**

anac-pl espone l'API pubblica della Piattaforma di Pubblicità a Valore Legale di ANAC come CLI agent-native: ricerca full-text con filtri (data, importo, CPV, tipologia), dettaglio JSON completo degli esiti, cronologia delle rettifiche, e un database SQLite locale per ricerca offline ed export CSV/JSON. Nessuna autenticazione richiesta.

Printed by [@aborruso](https://github.com/aborruso) (aborruso).

Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

La CLI è costruita con [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), che a partire da un'API genera una CLI in Go pronta sia per le persone sia per gli agenti, con server MCP, skill e store locale già inclusi, e la pubblica in un catalogo comune. È un prodotto molto comodo e lo consiglio a chi vuole fare lo stesso con altre API.

## Install

### Dal catalogo Printing Press

anac-pl è nel [catalogo Printing Press](https://github.com/mvanhorn/printing-press-library). L'installer fa tutto in un comando, binario più skill per gli agent:

```bash
npx -y @mvanhorn/printing-press-library install anac-pl
```

Solo il binario, senza skill:

```bash
npx -y @mvanhorn/printing-press-library install anac-pl --cli-only
```

Per aggiornarla all'ultima release del catalogo:

```bash
npx -y @mvanhorn/printing-press-library update anac-pl
```

Senza Node, con Go 1.26.6 o superiore:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/anac-pl/cmd/anac-pl-pp-cli@latest
```

### Binario precompilato dal repo dell'autore

Scarica l'archivio per la tua piattaforma dall'[ultima release](https://github.com/aborruso/anac-pl-pp-cli/releases/latest), scompattalo e metti il binario in una cartella del `PATH`.

```bash
# Linux x86-64
curl -sL https://github.com/aborruso/anac-pl-pp-cli/releases/latest/download/anac-pl-pp-cli_linux_amd64.tar.gz | tar xz
chmod +x anac-pl-pp-cli
./anac-pl-pp-cli doctor
```

Su macOS, la prima volta va tolta la quarantena di Gatekeeper: `xattr -d com.apple.quarantine anac-pl-pp-cli`.

### Dai sorgenti (richiede Go 1.26.6 o superiore)

```bash
git clone https://github.com/aborruso/anac-pl-pp-cli.git
cd anac-pl-pp-cli
go build -o anac-pl-pp-cli ./cmd/anac-pl-pp-cli
```

Nessuna configurazione, nessuna chiave: l'API di ANAC è pubblica e di sola lettura.

## Uso con Claude Desktop e altri client MCP

Il progetto contiene anche un server MCP, `anac-pl-pp-mcp`, che mette la ricerca di avvisi, affidamenti e CPV a disposizione degli assistenti che parlano quel protocollo. Con Claude Desktop basta chiedere in linguaggio naturale:

![Claude Desktop usa anac-pl per cercare gli avvisi con CPV 30213000-5](https://raw.githubusercontent.com/aborruso/anac-pl-pp-cli/main/docs/img/claude-desktop-cpv.png)

Il server non chiama ANAC da solo: passa ogni richiesta alla CLI `anac-pl-pp-cli`, che cerca nella sua stessa cartella (poi nella variabile `ANAC_PL_CLI_PATH`, poi nel `PATH`). Per questo i due programmi vanno tenuti insieme. Nessuna chiave da configurare.

### Tutto su Windows (o tutto su macOS)

La via più semplice è l'estensione `.mcpb`, un pacchetto che contiene il server e la CLI. Scaricala dalla [release del catalogo](https://github.com/mvanhorn/printing-press-library/releases/tag/anac-pl-current):

- Windows x86-64: [`anac-pl-pp-mcp-windows-amd64.mcpb`](https://github.com/mvanhorn/printing-press-library/releases/download/anac-pl-current/anac-pl-pp-mcp-windows-amd64.mcpb)
- Windows ARM: [`anac-pl-pp-mcp-windows-arm64.mcpb`](https://github.com/mvanhorn/printing-press-library/releases/download/anac-pl-current/anac-pl-pp-mcp-windows-arm64.mcpb)
- macOS Apple Silicon: [`anac-pl-pp-mcp-darwin-arm64.mcpb`](https://github.com/mvanhorn/printing-press-library/releases/download/anac-pl-current/anac-pl-pp-mcp-darwin-arm64.mcpb)

Poi aprilo con Claude Desktop (doppio clic sul file, oppure Impostazioni > Estensioni) e conferma l'installazione. La release si chiama `anac-pl-current` e viene rigenerata a ogni nuova versione: per aggiornare si scarica di nuovo il file e lo si reinstalla.

Senza estensione, a mano: scarica dall'[ultima release di questo repo](https://github.com/aborruso/anac-pl-pp-cli/releases/latest) l'archivio della tua piattaforma (per Windows `anac-pl-pp-cli_windows_amd64.zip`), che contiene `anac-pl-pp-mcp.exe` e `anac-pl-pp-cli.exe`, scompattalo in una cartella e aggiungi il server a `%APPDATA%\Claude\claude_desktop_config.json` (su macOS `~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "anac-pl": {
      "command": "C:\\Users\\<utente>\\bin\\anac-pl-pp-mcp.exe"
    }
  }
}
```

### Claude Desktop su Windows, CLI dentro WSL

Se la CLI la usi in WSL e vuoi che Claude Desktop per Windows usi quella stessa copia, installa i due programmi Linux dentro WSL, nella stessa cartella:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/anac-pl/cmd/anac-pl-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/other/anac-pl/cmd/anac-pl-pp-mcp@latest
```

Finiscono in `~/go/bin` (oppure compilali dai sorgenti in una cartella a scelta). Poi fai lanciare il server a Windows attraverso `wsl.exe`, indicando il percorso Linux assoluto, in `%APPDATA%\Claude\claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "anac-pl": {
      "command": "wsl.exe",
      "args": ["-e", "/home/<utente>/go/bin/anac-pl-pp-mcp"]
    }
  }
}
```

`wsl.exe -e` usa la distribuzione predefinita: se i programmi stanno in un'altra, aggiungi `"-d", "<nome-distro>"` prima di `"-e"` (l'elenco lo dà `wsl.exe -l -v`). Per aggiornare basta rilanciare i due `go install`.

### Dopo l'installazione

Chiudi del tutto Claude Desktop, anche dall'icona nell'area di notifica, e riaprilo: tra i connettori compare `anac-pl`. Per una prima prova che non dipende dalla rete chiedi "Il CIG B7E26B1DC8 è valido?"; per una ricerca vera, "Cerca gli avvisi con CPV 30213000-5 pubblicati da giugno 2026". Se il server non compare, su Windows i log sono in `%APPDATA%\Claude\logs\mcp.log`.

In Claude Code: `claude mcp add anac-pl -s user -- /percorso/assoluto/anac-pl-pp-mcp`.

Per gli agenti che leggono le skill (Claude Code, Codex, Cursor, ...) il repo contiene `SKILL.md`: copialo nella cartella delle skill del tuo agente.

## Per iniziare

```bash
# Verifica raggiungibilità dell'API
anac-pl-pp-cli doctor

# Ultimi bandi pubblicati, dal più recente (il servizio ordina solo senza --query: col testo libero vince la rilevanza)
anac-pl-pp-cli cerca --tipologia bandi --size 10 --sort-field dataPubblicazione --sort-dir DESC

# Con testo libero l'ordine è per rilevanza e --sort-field viene ignorato: per i più recenti si filtra per data
anac-pl-pp-cli avvisi search --query "corso intelligenza artificiale" --published-from 01/06/2026

# Trova il codice CPV partendo dalle parole
anac-pl-pp-cli cpv search "posta elettronica"

# Filtro CPV che seleziona davvero per codice (ricerca avanzata del portale)
anac-pl-pp-cli cerca-avanzata --cpv 30213000

# Il codice come compare negli atti, con la cifra di controllo, va bene lo stesso
anac-pl-pp-cli cerca-avanzata --cpv 30213000-5

# Tabella committente -> aggiudicatario -> importo -> CIG -> CPV -> giurisdizione
anac-pl-pp-cli affidamenti --cpv-code 72412000 -t "" --pages 3 --from-search --csv

# Gli affidamenti a una società, dal più recente (ordinamento su una colonna della tabella)
anac-pl-pp-cli affidamenti -q "google workspace" --pages 3 --from-search --sort-field data --sort-dir desc --csv

# Oltre la prima pagina: --pages unisce N pagine da --size (il servizio pagina a token, --page non esiste)
anac-pl-pp-cli cerca -q "ufficio stampa" --size 50 --pages 4 --json

# Tipologie di avviso e valore da usare come filtro
anac-pl-pp-cli tipologie list
```

Il testo libero di `--query` va al motore di ricerca di ANAC. Nella ricerca estesa, quella predefinita, i termini sono in OR e ordinati per rilevanza: su query di più parole i primi risultati sono di norma pertinenti, ma più in basso compaiono avvisi che contengono solo uno dei termini. Virgolette e `+` vengono ignorati senza errore, e `AND` viene cercato come parola, aggiungendo risultati estranei.

Per una frase esatta c'è la corrispondenza esatta del portale: `cerca --mode esatta` oppure `avvisi search --fuzzy=false`. Le parole devono comparire adiacenti e nell'ordine dato, e serve una tipologia, perché senza il servizio risponde 500 (la CLI lo blocca prima):

```bash
anac-pl-pp-cli cerca -q "data visualization" --mode esatta -t affidamenti-diretti
```

## Elenchi di riferimento di ANAC

ANAC pubblica le tipologiche della Piattaforma Contratti Pubblici nel repository [anticorruzione/npa](https://github.com/anticorruzione/npa). Due servono direttamente alla ricerca avanzata:

- [codici CPV](https://github.com/anticorruzione/npa/blob/main/docs/modello-dati/tipologiche/CPV.json): 9.454 codici, con descrizione in italiano e in inglese. È l'elenco a cui rimanda la finestra informativa del campo CPV della ricerca avanzata, e il vocabolario incluso nella CLI (`cpv search`, `cpv get`) coincide con questo.
- [categorie](https://github.com/anticorruzione/npa/blob/main/docs/modello-dati/tipologiche/categoria.json): le categorie di qualificazione dei lavori (OG, OS), le forniture di beni (FB) e di servizi (FS), e le stazioni appaltanti con sistema di qualificazione proprio (AA). Sono i valori di `cerca-avanzata --categorie`. Nel file compaiono anche `999` (categoria non definita), che la ricerca non propone, e `OS 29` due volte: il servizio ne espone 55, senza duplicati.

Dallo stesso repository la CLI prende altre due cose, che funzionano senza rete:

- [codici scheda](https://github.com/anticorruzione/npa/blob/main/docs/modello-dati/tipologiche/codiceScheda.json): ogni avviso porta un `codiceScheda` (AD3, A1_29, P1_16...) che indica il modello di pubblicazione e la norma di riferimento. `tipologie schede AD3` lo descrive, `tipologie schede` li elenca tutti e 150. Non sono i valori da passare a `--scheda`, che vuole il numero template di `tipologie list`.
- [algoritmo di validazione del CIG](https://github.com/anticorruzione/npa/blob/main/docs/Algoritmo%20validazione%20CIG/algoritmoValidazioneCIG.md): `cig check B7E26B1DC7` verifica struttura e cifra di controllo per le tre famiglie (Simog, Simog seconda versione e PCP, SmartCIG). Un CIG trascritto male non dà errore nella ricerca, restituisce avvisi estranei: per questo `cerca --query` e `avvisi search --keywords` avvisano su stderr quando il testo ha la forma di un CIG ma non supera il controllo.

## Doppi invii: righe uguali con `id_avviso` diverso

La piattaforma pubblica ciò che riceve, compresi gli avvisi che una stazione appaltante manda due volte a pochi secondi di distanza: due `idAvviso` distinti, stesso `idAppalto`, stessa scheda, contenuto identico. In `affidamenti` compaiono come righe uguali con `id_avviso` diverso. Non vengono fuse, perché sullo stesso CIG esistono anche avvisi diversi e legittimi (esito, rettifica, ripubblicazione, due notice TED per lo stesso accordo quadro). La chiave per riconoscere i doppi invii è `id_appalto` insieme a `cig`, `cf_aggiudicatario`, `importo` e `data`:

```bash
anac-pl-pp-cli affidamenti -q 03289010542 -t "" --pages 3 --csv > a.csv
duckdb -c "select * from 'a.csv' qualify row_number() over (partition by id_appalto, cig, cf_aggiudicatario, importo, data order by id_avviso) = 1"
```

## Un limite messo apposta: una chiamata al secondo, una istanza per volta

La piattaforma di ANAC è un servizio pubblico che non dichiara alcuna quota, e non c'è un modo per chiedere più banda. Il tetto quindi è scritto nel programma e non è configurabile: al massimo una richiesta al secondo.

Il ritmo è condiviso fra processi, non solo dentro il processo: l'istante dell'ultima chiamata sta in `~/.cache/anac-pl-pp-cli/pace.lock`, protetto da un lock esclusivo di sistema. Anche facendo lavorare insieme la CLI e il server MCP, la somma resta una chiamata al secondo.

Due copie della CLI non girano in parallelo: la seconda aspetta il proprio turno, dicendolo su stderr, fino a cinque minuti; scaduti quelli esce con codice 7. In modalità non interattiva (`--no-input`, e quindi `--agent`) non attende affatto ed esce 7 subito, perché uno script preferisce un errore immediato a un comando che tace. Il lock è `~/.cache/anac-pl-pp-cli/instance.lock`, rilasciato dal sistema operativo alla fine del processo, quindi non resta mai appeso.

Il lock non è però ciò che garantisce il tetto - quello lo tiene `pace.lock`, che vale per un numero qualunque di processi. Serve come rete per il caso in cui quel file non sia utilizzabile e il ritmo torni a valere per il solo processo. Per questo `doctor` lo salta: un controllo di salute deve rispondere anche mentre un `sync` lavora.

Ogni richiesta si presenta con un `User-Agent` che dichiara nome, versione e questo repository, così che chi amministra la piattaforma possa riconoscere il traffico e, se desse fastidio, scrivere invece di bloccare.

`--rate-limit` esiste ancora, ma serve solo a rallentare ulteriormente: `--rate-limit 0.2` scende a una chiamata ogni cinque secondi, `--rate-limit 100` non alza nulla.

La conseguenza pratica è che le scansioni lunghe sono lente per costruzione: `sync` di molte pagine va lanciato e lasciato lavorare. Per le analisi ripetute conviene sincronizzare una volta e poi interrogare lo store locale con `search-local` ed `export`, che non toccano la rete.

## Quattro avvertenze sui dati

Il servizio pagina a token, non per numero di pagina: `page` veniva accettato e ignorato, restituendo sempre la prima pagina. `cerca --pages N` e `affidamenti --pages N` scorrono le pagine seguendo il token e uniscono i risultati, deduplicati per `idAvviso`. Su `avvisi search`, `--all` segue i token di continuazione; il limite di sicurezza è 100 pagine e si può aumentare con `--max-pages N`. Se il limite viene raggiunto, il comando restituisce un errore senza presentare risultati parziali come completi. Senza `--all`, `--size` regola la prima pagina.

Il campo CPV di `cerca` non è un filtro sul codice ma un match testuale: restituisce anche avvisi con CPV estranei. Per selezionare davvero per codice serve `cerca-avanzata`, che usa l'endpoint della ricerca avanzata rilasciata in beta a luglio 2026. La CLI lo segnala su stderr quando usi `cerca --cpv`.

Il numero di risultati dichiarato dal servizio, sugli aggregati, è una stima progressiva: cambia mentre sfogli le pagine e fra chiamate identiche. Va usato come ordine di grandezza, non come totale. I codici CPV completi a 8 cifre sono invece stabili.

Il campo `titolo` dei metadati di un avviso è spesso `null`: su due campioni di 200 avvisi lo era in 122 e 118 casi, soprattutto sulle schede AD3 e A2_*. `descrizione` è invece sempre valorizzata, e quando ci sono entrambi i due testi possono essere diversi. Per l'oggetto dell'avviso conviene leggere `descrizione`.

Ogni risultato della ricerca ha i campi `oscurato` e `noteOscuramento`. Secondo la [guida di ANAC sull'oscuramento](https://github.com/anticorruzione/npa/blob/main/docs/Guide_Utente/Oscuramento_Avviso_PVL_e_Adeguamento_PCP.md), la piattaforma può oscurare un avviso che non supera i suoi controlli ed escludere gli avvisi oscurati dai risultati di ricerca; la stessa guida dice che i controlli sono per ora disattivati. Nei campioni presi a settembre 2026 `oscurato` era sempre `false`. Se un avviso noto non compare nella ricerca, l'oscuramento è una delle spiegazioni possibili.

## Funzioni esclusive

Capacità che nessun altro strumento per questa API mette a disposizione.

### Stato locale che si accumula
- **`sync`** - Scarica e conserva gli avvisi in un database SQLite locale per analisi offline.

  _Permette analisi ripetute e aggregazioni che l'API paginata non offre._

  ```bash
  anac-pl-pp-cli sync --resources avvisi --param keywords=microsoft
  ```
- **`search-local`** - Cerca tra gli avvisi già sincronizzati in locale, senza rete.

  _Risposte istantanee e componibili con jq/SQL._

  ```bash
  anac-pl-pp-cli search-local microsoft
  ```
- **`export`** - Esporta gli avvisi sincronizzati in CSV o JSON per fogli di calcolo e pipeline dati.

  _Porta i dati ANAC direttamente in strumenti di analisi._

  ```bash
  anac-pl-pp-cli export avvisi
  ```

## Ricette


### Esiti recenti per parola chiave

```bash
anac-pl-pp-cli cerca -q 'servizi informatici' -t esiti --published-from 01/06/2026 --size 20
```

Filtra gli esiti di gara per oggetto. Con testo libero l'ordine è per rilevanza, quindi a limitarli ai più recenti è il filtro sulla data.

### Dettaglio JSON di un esito

```bash
anac-pl-pp-cli avvisi get c5bfcc8d-ebed-4b6b-ab5f-661d78fa88e2 --json
```

Recupera il JSON completo della pagina di dettaglio.

### Estrai solo campi chiave

```bash
anac-pl-pp-cli avvisi search --query microsoft --agent --select idAvviso,codiceScheda,dataPubblicazione,score
```

Output compatto per agenti su risposte voluminose.

## Uso

Esegui `anac-pl-pp-cli --help` per l'elenco completo dei comandi e delle opzioni.

## Comandi

### avvisi

Ricerca e consultazione di bandi, esiti e avvisi pubblicati sulla Piattaforma di Pubblicità a Valore Legale ANAC

- **`anac-pl-pp-cli avvisi cronologia`** - Cronologia delle versioni/rettifiche di un avviso nel tempo
- **`anac-pl-pp-cli avvisi get`** - Dettaglio completo di un avviso/esito in formato JSON, incluse sezioni e committente
- **`anac-pl-pp-cli avvisi search`** - Ricerca full-text di avvisi (bandi, esiti, altri avvisi) con ranking di rilevanza e filtri

### news

Avvisi e comunicazioni della piattaforma

- **`anac-pl-pp-cli news`** - Ultime news pubblicate sulla piattaforma

### tipologie

Tassonomia delle tipologie di avviso (categorie, tipologie e codici scheda)

- **`anac-pl-pp-cli tipologie`** - Mappa di categorie, tipologie e codici scheda usati per filtrare la ricerca
- **`anac-pl-pp-cli tipologie schede`** - Descrive i codici scheda degli avvisi (AD3, A1_29, P1_16), offline

### cig

- **`anac-pl-pp-cli cig check`** - Verifica struttura e cifra di controllo di uno o più CIG, offline


## Formati di output

```bash
# Tabella leggibile (predefinita nel terminale, JSON quando l'output è in pipe)
anac-pl-pp-cli avvisi get mock-value

# JSON per script e agent
anac-pl-pp-cli avvisi get mock-value --json

# Filtra solo alcuni campi
anac-pl-pp-cli avvisi get mock-value --json --select id,name,status

# Prova a vuoto - mostra la richiesta senza inviarla
anac-pl-pp-cli avvisi get mock-value --dry-run

# Modalità agent - JSON, output compatto e nessuna domanda, con una sola opzione
anac-pl-pp-cli avvisi get mock-value --agent
```

## Uso da parte degli agent

Questa CLI è pensata per essere usata da agent AI:

- **Non interattiva** - non fa mai domande, ogni input è un'opzione
- **Componibile in pipe** - con `--json` l'output va su stdout, gli errori su stderr
- **Filtrabile** - `--select id,name` restituisce solo i campi che servono
- **Ispezionabile** - `--dry-run` mostra la richiesta senza inviarla
- **Di sola lettura** - questa CLI non crea, aggiorna, cancella, pubblica, invia né modifica risorse remote
- **Utilizzabile offline** - i comandi di sync e ricerca possono usare lo store SQLite locale, quando c'è
- **Sicura per gli agent** - nessun colore né formattazione, a meno di `--human-friendly`

Codici di uscita: `0` successo, `2` errore d'uso, `3` non trovato, `5` errore dell'API, `7` limite di chiamate, `10` errore di configurazione.

## Controllo di salute

```bash
anac-pl-pp-cli doctor
```

Verifica la configurazione e la raggiungibilità dell'API.

## Configurazione

File di configurazione: `~/.config/anac-pl-pp-cli/config.toml`

Gli header fissi delle richieste si impostano sotto `headers`; quelli indicati sul singolo comando hanno la precedenza.

## Risoluzione dei problemi
**Errori "non trovato" (codice di uscita 3)**
- Controlla che l'identificativo della risorsa sia corretto
- Esegui il comando `list` per vedere gli elementi disponibili

- **`search-local` esce 3 quando la ricerca non trova nulla**: è la risposta, non un guasto. Lo stdout resta quello normale (`[]` in JSON), cambia solo il codice di uscita, così uno script distingue «nessun avviso» da «trovati».

### Specifici di questa API
- **HTTP 500 sui filtri data** - Usa il formato GG/MM/AAAA per --published-from e --published-to
- **ricerca archivio vuota** - Con --archive specifica un intervallo date inferiore a 6 mesi
