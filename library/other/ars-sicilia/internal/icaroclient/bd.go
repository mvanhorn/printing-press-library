package icaroclient

// Client per il backend nuovo /bd/ del portale ARS. A differenza del motore
// Icaro (GET default.jsp + shortList.jsp), i 3 archivi delle sedute
// (sommari 230, resoconti 217, convocazioni 229) sono stati migrati a un
// backend che risponde a POST /bd/<archivio> con HTML paginato. L'indice Icaro
// di questi 3 è congelato (sommari a giu 2025, resoconti a feb 2026), mentre
// /bd/ è corrente: per questo Search instrada qui gli archivi migrati.
//
// Vedi docs/bd-migration/API_DOCUMENTATION.md per il reverse-engineering.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// bdSpec descrive come parlare a un archivio /bd/: il path, la mappa dei filtri
// friendly (chiavi Params) verso i nomi campo POST, e i campi statici sempre
// impostati (i selettori di modalità `$S$...`, che valgono "all" = tutte le parole).
type bdSpec struct {
	path   string
	fields map[string]string // friendly key -> POST field name
	static map[string]string // campi sempre inviati (selettori modalità)
	// speakerField, se valorizzato, è il campo <select multiple> degli oratori
	// (es. "$Ispeakers"): il filtro --oratore viene risolto da nome a ID leggendo
	// le <option> del form, poi inviato su questo campo con modalità "$S..."="or".
	speakerField string
	// commissioneField, se valorizzato, è il <select> delle commissioni (es.
	// "$Icommissione_id" sommari, "$Iidcomm" convocazioni): --commissione/--codcom
	// vengono risolti in id (per-legislatura). commissioneMode, se non vuoto, è il
	// valore del selettore "$S<field>" (es. "or" per i select multipli).
	commissioneField string
	commissioneMode  string
}

// bdArchives elenca gli archivi serviti dal backend /bd/. Gli altri restano su
// Icaro. Si aggiunge un archivio alla volta man mano che è verificato end-to-end.
var bdArchives = map[string]bdSpec{
	"sommari": {
		path: "sommari",
		fields: map[string]string{
			"legisl": "$Ilegislatura",
			"anno":   "anno",
			"numero": "$Iseduta_numero",
			"testo":  "$TTEXT",
		},
		static:           map[string]string{"$S$TTEXT": "all", "$S$Todg": "all"},
		commissioneField: "$Icommissione_id",
		// Il <select> è renderizzato senza attributo `multiple`, ma il backend
		// accetta lo stesso una lista di id se glielo si dichiara con "$S…=or".
		// Senza, `commissioni sommari --commissione QUARTA` risponde 500: le
		// commissioni hanno un id per legislatura — la IV ne ha nove, dalla
		// "IV - Ambiente e Territorio" alla "IV - Ambiente, territorio e
		// mobilità" — e senza --legisl si risolvono tutti, cioè si manda una
		// lista a un campo che il form dichiara singolo.
		commissioneMode: "or",
	},
	"resoconti": {
		path: "resoconti",
		fields: map[string]string{
			"legisl": "$Ilegislatura",
			"anno":   "anno",
			"numero": "$Inrosed",
			"testo":  "$TTEXT",
		},
		static:       map[string]string{"$S$TTEXT": "all"},
		speakerField: "$Ispeakers", // --oratore risolto nome->ID dalle <option> del form
	},
	"convocazioni": {
		path: "convocazioni",
		fields: map[string]string{
			"legisl": "$Ilegislatura",
			"anno":   "anno",
			"testo":  "$TTEXT",
		},
		static:           map[string]string{"$S$TTEXT": "all"},
		commissioneField: "$Iidcomm", // multi-select
		commissioneMode:  "or",
	},
}

// IsBDArchive segnala se lo slug è servito dal backend /bd/.
func IsBDArchive(slug string) bool {
	_, ok := bdArchives[slug]
	return ok
}

// BDEndpoint torna l'URL a cui il backend /bd/ riceve la POST di ricerca per
// quell'archivio, e false se l'archivio non è servito da /bd/. Serve alle
// anteprime --dry-run: senza, annunciano l'URL Icaro anche dove la richiesta
// parte davvero verso /bd/, cioè dicono con sicurezza un endpoint che non
// verrà interrogato — su un comando che esiste apposta per diagnosticare.
func BDEndpoint(baseURL, slug string) (string, bool) {
	spec, ok := bdArchives[slug]
	if !ok {
		return "", false
	}
	return strings.TrimSuffix(baseURL, "/") + "/bd/" + spec.path, true
}

// BDPreview descrive la POST che searchBD manderebbe, senza mandarla.
//
// Mostrare i filtri della riga di comando come se fossero i campi della POST
// sarebbe una seconda bugia dell'anteprima, sorella di quella che il campo
// `backend` ha appena chiuso: searchBD non li spedisce come li riceve. I nomi
// passano per spec.fields (`legisl` → `$Ilegislatura`), i selettori di modalità
// in spec.static viaggiano sempre, e tre filtri non sono affatto campi —
// `--data` diventa un ciclo sul campo `anno` più un filtro client-side sulle
// righe, mentre `--oratore` e `--commissione`/`--codcom` si risolvono da nome a
// id leggendo le <option> del form, che è una richiesta e un dry run non la fa.
//
// Quindi si separano le due cose: PostFields sono i campi che partirebbero
// esattamente così, Deferred nomina i filtri che si risolvono al momento della
// richiesta e dice in che cosa si trasformano. Un'anteprima che tace la
// differenza manda a cercare il guasto su parametri che nessuno spedisce.
type BDPreviewResult struct {
	Endpoint   string
	PostFields map[string]string
	Deferred   map[string]string
	// Invalid non e' nil quando un filtro non si parsa: searchBD in quel caso
	// esce con InvalidParamError PRIMA di mandare qualunque cosa, e l'anteprima
	// deve fare lo stesso. Ignorare l'errore e stampare comunque una richiesta
	// plausibile e' il peggiore dei casi: `--data 2025-01-01:garbage --dry-run`
	// diceva «ecco cosa parte», mentre senza --dry-run lo stesso comando
	// fallisce e non parte nulla.
	Invalid error
	// Anni sono i valori che il campo `anno` prende, uno per giro: con --data
	// searchBD non manda UNA richiesta, ne manda una per anno dell'intervallo
	// (e dentro ciascuna una per pagina). Dire solo «l'intervallo si risolve al
	// momento della richiesta» lasciava credere a una richiesta sola, e da un
	// dry run su un intervallo di piu' anni non si capiva quante ne partono ne'
	// come rifarle a mano. Gli anni si ricavano senza rete — bdDateFilter parsa
	// e basta — quindi tacerli era una scelta, non un limite.
	Anni []string
}

// BDPreview torna false sugli archivi che /bd/ non serve.
func BDPreview(baseURL, slug string, params map[string]string) (BDPreviewResult, bool) {
	spec, ok := bdArchives[slug]
	if !ok {
		return BDPreviewResult{}, false
	}
	out := BDPreviewResult{
		Endpoint:   strings.TrimSuffix(baseURL, "/") + "/bd/" + spec.path,
		PostFields: map[string]string{},
		Deferred:   map[string]string{},
	}
	// sessionHTML vuoto = modo anteprima. La form e' quella che searchBD
	// manderebbe, costruita dalla stessa funzione: non c'e' una seconda
	// implementazione che possa divergere.
	req, err := bdBuildForm(slug, spec, params, "", true)
	if err != nil {
		out.Invalid = err
		return out, true
	}
	for k, vs := range req.Form {
		if len(vs) > 0 {
			out.PostFields[k] = vs[0]
		}
	}
	out.Anni = req.Anni
	out.Deferred = req.Deferred
	return out, true
}

// bdOption è una <option> di un <select> del form (oratori o commissioni): id,
// nome e le legislature associate (data-leg/data-legs). Per commissioni gli id
// sono PER-LEGISLATURA (es. "I - Affari Istituzionali" ha id 116 in leg 18, 1 in
// leg 13), quindi il filtro per legislatura è essenziale per prendere l'id giusto.
type bdOption struct {
	ID   string
	Name string
	Legs string
}

// reBDOption matcha le <option> di un select (due spazi dopo <option nel markup).
// Il secondo gruppo cattura gli attributi residui, da cui si estrae data-leg(s).
var reBDOption = regexp.MustCompile(`<option\s+value="([^"]*)"([^>]*)>([^<]+)</option>`)
var reDataLegs = regexp.MustCompile(`data-legs?="([^"]*)"`)
var reBDCount = regexp.MustCompile(`Trovati\s+(\d+)\s+risultati`)
var reOpenRisultati = regexp.MustCompile(`openRisultati\(([^)]*)\)`)

// bdSchedaURL costruisce l'URL della scheda per-record dagli argomenti della
// funzione JS openRisultati(...) presente nel link della riga. I pattern (dalla
// definizione inline di openRisultati nelle pagine /bd/):
//   - resoconti:    openRisultati(legis, nro)        -> /bd/resoconti/scheda/<legis>/<nro>
//   - sommari:      openRisultati(legis, comm, nro)  -> /bd/sommari/scheda/<legis>/<comm>/<nro>
//   - convocazioni: openRisultati(id)                -> /bd/convocazioni/results/<id>
//
// Ritorna "" se l'href non contiene una chiamata riconoscibile.
func bdSchedaURL(baseURL, slug, href string) string {
	m := reOpenRisultati.FindStringSubmatch(href)
	if m == nil {
		return ""
	}
	var args []string
	for _, a := range strings.Split(m[1], ",") {
		if a = strings.Trim(strings.TrimSpace(a), "'\""); a != "" {
			args = append(args, a)
		}
	}
	switch slug {
	case "resoconti":
		if len(args) >= 2 {
			return baseURL + "/bd/resoconti/scheda/" + args[0] + "/" + args[1]
		}
	case "sommari":
		if len(args) >= 3 {
			return baseURL + "/bd/sommari/scheda/" + args[0] + "/" + args[1] + "/" + args[2]
		}
	case "convocazioni":
		if len(args) >= 1 {
			return baseURL + "/bd/convocazioni/results/" + args[0]
		}
	}
	return ""
}

// parseSelectOptions estrae le <option> del solo <select id="selectID">. Lo scoping
// al select giusto evita di mescolare select diversi presenti nello stesso form
// (legislatura, anno, oratori, commissioni).
func parseSelectOptions(body, selectID string) []bdOption {
	start := strings.Index(body, `<select id="`+selectID+`"`)
	if start < 0 {
		return nil
	}
	rest := body[start:]
	end := strings.Index(rest, "</select>")
	if end < 0 {
		return nil
	}
	var out []bdOption
	for _, m := range reBDOption.FindAllStringSubmatch(rest[:end], -1) {
		if strings.TrimSpace(m[1]) == "" { // opzione "Tutte" (value vuoto)
			continue
		}
		legs := ""
		if lm := reDataLegs.FindStringSubmatch(m[2]); lm != nil {
			legs = lm[1]
		}
		out = append(out, bdOption{ID: m[1], Name: unescapeMini(strings.TrimSpace(m[3])), Legs: legs})
	}
	return out
}

// resolveOptionIDs ritorna gli id delle option il cui nome contiene la query
// (case-insensitive) e, se legisl è dato, associate a quella legislatura.
func resolveOptionIDs(opts []bdOption, query, legisl string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var ids []string
	for _, o := range opts {
		if !strings.Contains(strings.ToLower(o.Name), q) {
			continue
		}
		if legisl != "" && o.Legs != "" && !legsContains(o.Legs, legisl) {
			continue
		}
		ids = append(ids, o.ID)
	}
	return ids
}

// resolveCommissioneIDs risolve --codcom (1-6, per numero ordinale romano) oppure
// --commissione (nome, substring) negli id del <select> commissioni, filtrando per
// legislatura (gli id sono per-legislatura). Ritorna [] (non nil) se il filtro è
// richiesto ma non corrisponde nulla; nil se nessun filtro è richiesto.
func resolveCommissioneIDs(opts []bdOption, codcom, commissione, legisl string) []string {
	if cod := strings.TrimSpace(codcom); cod != "" {
		roman := romanOrdinal(cod)
		if roman == "" {
			return []string{}
		}
		ids := []string{}
		for _, o := range opts {
			if !strings.HasPrefix(o.Name, roman+" ") { // "I - Affari…" inizia con "I "
				continue
			}
			if legisl != "" && o.Legs != "" && !legsContains(o.Legs, legisl) {
				continue
			}
			ids = append(ids, o.ID)
		}
		return ids
	}
	if com := strings.TrimSpace(commissione); com != "" {
		// L'ordinale a lettere ("SESTA") non è un frammento della denominazione
		// d'archivio ("VI - Salute, Servizi Sociali e Sanitari"): senza questa
		// traduzione --commissione SESTA non matcha nulla su /bd/, e il comando
		// restituisce zero record come se la commissione non avesse lavori.
		if roman := romanFromOrdinalName(com); roman != "" {
			ids := []string{}
			for _, o := range opts {
				if !strings.HasPrefix(o.Name, roman+" ") {
					continue
				}
				if legisl != "" && o.Legs != "" && !legsContains(o.Legs, legisl) {
					continue
				}
				ids = append(ids, o.ID)
			}
			return ids
		}
		ids := resolveOptionIDs(opts, com, legisl)
		if ids == nil {
			return []string{}
		}
		return ids
	}
	return nil
}

// romanFromOrdinalName traduce l'ordinale a lettere nel numero romano con cui il
// <select> di /bd/ prefissa la denominazione: "SESTA" -> "VI".
func romanFromOrdinalName(name string) string {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "PRIMA":
		return "I"
	case "SECONDA":
		return "II"
	case "TERZA":
		return "III"
	case "QUARTA":
		return "IV"
	case "QUINTA":
		return "V"
	case "SESTA":
		return "VI"
	}
	return ""
}

// romanOrdinal converte il codice commissione 1-6 nel numero romano I..VI ("" altrimenti).
func romanOrdinal(n string) string {
	switch strings.TrimSpace(n) {
	case "1":
		return "I"
	case "2":
		return "II"
	case "3":
		return "III"
	case "4":
		return "IV"
	case "5":
		return "V"
	case "6":
		return "VI"
	}
	return ""
}

// legsContains riporta se legs ("18,17,16") include la legislatura leg.
func legsContains(legs, leg string) bool {
	for _, l := range strings.Split(legs, ",") {
		if strings.TrimSpace(l) == leg {
			return true
		}
	}
	return false
}

// unescapeMini decodifica le poche entità che compaiono nei nomi (apostrofo, &).
func unescapeMini(s string) string {
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}

// searchBD esegue una ricerca sul backend /bd/: GET di sessione, poi POST
// paginati, parsando l'HTML in Record. Onora Limit/MaxPages/Truncated come
// Search. Il filtro --data non ha un campo server (il portale filtra per
// `anno`): si deriva l'anno per il server e si filtra client-side sulla data.
// bdRequest e' la richiesta che il backend /bd/ riceverebbe: la form gia'
// compilata, i giri sugli anni, il filtro client-side sulle date, e i filtri
// che senza la sessione non si possono risolvere.
//
// Esiste perche' searchBD e l'anteprima --dry-run devono descrivere la stessa
// cosa, e per un po' non l'hanno fatto: l'anteprima reimplementava a mano cio'
// che searchBD costruisce, e sette rilievi di review hanno trovato altrettante
// divergenze — endpoint sbagliato, nomi di campo non tradotti, gli anni taciuti,
// `page` e `anno` assenti, `--codcom` riscritto, una data malformata accettata
// in anteprima e rifiutata dal vivo, un filtro non supportato differito invece
// che rifiutato. Due implementazioni parallele restano d'accordo solo per
// ispezione, ed e' cosi' che le divergenze si accumulano senza che nulla lo
// segnali. Con un costruttore solo, divergere non e' piu' possibile.
type bdRequest struct {
	Form     url.Values
	Anni     []string
	KeepDate func(rowDate string) bool
	// Deferred e' popolato solo in modo anteprima: nomina i filtri che
	// richiedono la sessione e dice in che cosa si trasformano.
	Deferred map[string]string
	// VuotoPerAnno segnala l'unico caso in cui il percorso vivo non manda
	// nulla e non e' un errore: --anno fuori dall'intervallo di --data, dove
	// l'intersezione dei filtri e' vuota e zero risultati e' la risposta giusta.
	VuotoPerAnno bool
}

// bdBuildForm compila la richiesta per il backend /bd/.
//
// `anteprima` e' un parametro esplicito e non si deduce da sessionHTML vuoto.
// Dedurlo sarebbe un guasto silenzioso: se la GET di sessione tornasse 200 con
// corpo vuoto — portale che sbanda, risposta troncata — il percorso VIVO
// scivolerebbe in modo anteprima, non risolverebbe --oratore e
// --commissione/--codcom, e manderebbe la POST senza quel filtro. Il risultato
// sarebbe piu' largo di quello chiesto, presentato come buono: esattamente il
// filtro che sparisce in silenzio contro cui e' costruito il resto di questo
// file. In anteprima --oratore e --commissione/--codcom non si risolvono perche'
// leggono le <option> del form, cioe' una richiesta che un dry run non fa, e
// finiscono in Deferred; dal vivo la risoluzione avviene e un valore che non
// aggancia nulla produce UnresolvedFilterError come prima.
func bdBuildForm(slug string, spec bdSpec, params map[string]string, sessionHTML string, anteprima bool) (bdRequest, error) {
	out := bdRequest{Form: url.Values{}, Deferred: map[string]string{}}

	// Un filtro che questo archivio non sa applicare fallisce, non viene
	// ignorato: silenziosamente restituirebbe un set piu' largo di quello che
	// la riga di comando chiede. Vale anche in anteprima, dove prima usciva
	// come "differito" mentre il comando vero sarebbe fallito.
	if err := bdUnsupportedParams(slug, spec, params); err != nil {
		return out, err
	}

	for k, v := range spec.static {
		out.Form.Set(k, v)
	}
	// `page` viaggia su ogni richiesta e la prima di ogni giro e' sempre 1. Il
	// ciclo di searchBD lo riscrive a ogni pagina con lo stesso valore di
	// partenza: metterlo qui non cambia nulla al vivo e rende l'anteprima
	// riproducibile alla lettera.
	out.Form.Set("page", "1")

	for k, v := range params {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch k {
		case "data":
			// Il campo server `anno` accetta un anno solo: l'intervallo diventa
			// un giro per anno piu' un filtro sulle righe ricevute.
			anni, keep := bdDateFilter(v)
			if anni == nil || keep == nil {
				// Un valore che non si parsa lasciava entrambi a nil e la
				// ricerca proseguiva senza vincolo d'anno sul form e senza
				// filtro client-side: `--data 2025-01-01:garbage` non
				// restituiva «niente in quell'intervallo», restituiva
				// l'archivio intero dall'inizio — resoconti fino al 12/04/1951
				// — presentandolo come esito buono. Il filtro che sparisce in
				// silenzio e' peggio del filtro che fallisce.
				return out, &InvalidParamError{Filtro: "--data", Valore: v,
					Rimedio: "usa YYYY-MM-DD, AAMMGG, o un intervallo YYYY-MM-DD:YYYY-MM-DD (AAMMGG/AAMMGG)"}
			}
			out.Anni, out.KeepDate = anni, keep
		case "oratore", "codcom", "commissione":
			// Risolti sotto: servono le <option> della sessione.
		default:
			if field, ok := spec.fields[k]; ok {
				out.Form.Set(field, v)
			}
		}
	}

	// --anno e --data scrivono lo stesso campo server `anno`: si intersecano in
	// modo esplicito (l'anno deve cadere nell'intervallo della data), invece di
	// lasciare che vinca l'ordine — casuale — di iterazione della mappa params.
	if anno := strings.TrimSpace(params["anno"]); anno != "" && len(out.Anni) > 0 {
		keep := out.Anni[:0]
		for _, y := range out.Anni {
			if y == anno {
				keep = append(keep, y)
			}
		}
		out.Anni = keep
		if len(out.Anni) == 0 {
			// Nessuna richiesta partirebbe: in anteprima si toglie anche il
			// campo, o si annuncerebbe una richiesta plausibile che non parte.
			out.Form.Del("anno")
			out.Deferred["anno"] = "fuori dall'intervallo di --data: nessun anno da interrogare, la ricerca non restituirebbe nulla e nessuna richiesta partirebbe"
			out.VuotoPerAnno = true
			return out, nil
		}
	}
	// Il primo giro e' l'unico valore dicibile senza indovinare; il ciclo di
	// searchBD riscrive il campo a ogni anno.
	if len(out.Anni) > 0 {
		out.Form.Set("anno", out.Anni[0])
		out.Deferred["data"] = "il backend non ha un campo data: l'intervallo diventa una richiesta per ciascun anno in `anni`, che sono i valori che il campo `anno` prende uno per giro — nei campi c'e' il primo, cioe' quello della prima richiesta — piu' un filtro sulle righe ricevute per tagliare i giorni fuori intervallo. Dentro ogni anno `page` parte da 1 e cresce di uno fino al numero di pagine che la risposta dichiara, o finche' --limit e' pieno: quel numero sta nella risposta, quindi le pagine oltre la prima non sono anteprimabili"
	}

	legisl := strings.TrimSpace(params["legisl"])

	// --oratore: risolve il nome negli ID del <select> oratori del form (con le
	// legislature in cui l'oratore e' attivo) e li invia in modalita' "or".
	if spec.speakerField != "" {
		if orat := strings.TrimSpace(params["oratore"]); orat != "" {
			if anteprima {
				out.Deferred["oratore"] = "risolto da nome a id leggendo le <option> di " + spec.speakerField + " nel form, che richiede una richiesta"
			} else {
				sel := parseSelectOptions(sessionHTML, spec.speakerField)
				ids := resolveOptionIDs(sel, orat, legisl)
				if len(ids) == 0 {
					// Un risultato vuoto direbbe "non e' mai intervenuto", che
					// e' un'altra affermazione: qui il nome non esiste in
					// anagrafica.
					return out, &UnresolvedFilterError{Filtro: "--oratore", Valore: orat, Legisl: legisl,
						Rimedio:     "Prova con il solo cognome, o con una porzione del nome.",
						Disponibili: suggestOptionNames(sel, orat, legisl)}
				}
				out.Form[spec.speakerField] = ids
				out.Form.Set("$S"+spec.speakerField, "or")
			}
		}
	}

	// --commissione / --codcom: risolve in id (per-legislatura) dal <select>.
	if spec.commissioneField != "" {
		cod := strings.TrimSpace(params["codcom"])
		com := strings.TrimSpace(params["commissione"])
		if cod != "" || com != "" {
			if anteprima {
				chiave := "commissione"
				if com == "" {
					chiave = "codcom"
				}
				out.Deferred[chiave] = "risolto in id per-legislatura leggendo le <option> di " + spec.commissioneField + " nel form, che richiede una richiesta"
			} else {
				selOpts := parseSelectOptions(sessionHTML, spec.commissioneField)
				ids := resolveCommissioneIDs(selOpts, cod, com, legisl)
				if len(ids) == 0 {
					// Come per --oratore: zero record direbbe "questa
					// commissione non ha lavori", che e' un'altra affermazione.
					val, filtro := com, "--commissione"
					rimedio := "Usa il nome ordinale della commissione: PRIMA, SECONDA, TERZA, QUARTA, QUINTA, SESTA."
					if val == "" {
						val, filtro = cod, "--codcom"
						rimedio = "Il codice commissione va da 1 a 6 (1=PRIMA, 2=SECONDA, ... 6=SESTA); per cercarla per nome usa --commissione."
					}
					return out, &UnresolvedFilterError{Filtro: filtro, Valore: val, Legisl: legisl,
						Disponibili: suggestOptionNames(selOpts, val, legisl), Rimedio: rimedio}
				}
				out.Form[spec.commissioneField] = ids
				if spec.commissioneMode != "" {
					out.Form.Set("$S"+spec.commissioneField, spec.commissioneMode)
				}
			}
		}
	}

	return out, nil
}

func (c *Client) searchBD(ctx context.Context, arc Archive, opts SearchOptions) ([]Record, error) {
	spec := bdArchives[arc.Slug]
	bdURL := c.BaseURL + "/bd/" + spec.path

	// Sessione (cookie JSESSIONID nel jar del Client). La risposta contiene anche
	// il form, inclusi i <select> di oratori e commissioni: la teniamo per
	// risolvere --oratore e --commissione/--codcom.
	sessionHTML, err := c.get(ctx, bdURL)
	if err != nil {
		return nil, fmt.Errorf("bd session (%s): %w", arc.Slug, err)
	}

	// Un filtro che il backend non sa applicare deve fallire, non essere
	// ignorato (vedi bdUnsupported).
	if err := bdUnsupported(arc.Slug, spec, opts); err != nil {
		return nil, err
	}

	// La form la costruisce bdBuildForm, la stessa che l'anteprima --dry-run
	// stampa: e' l'unico modo perche' le due non possano descrivere richieste
	// diverse (vedi bdRequest).
	req, err := bdBuildForm(arc.Slug, spec, opts.Params, sessionHTML, false)
	if err != nil {
		return nil, err
	}
	if req.VuotoPerAnno {
		return nil, nil // --anno fuori dall'intervallo di --data: nessun risultato
	}
	form, years, keepDate := req.Form, req.Anni, req.KeepDate

	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 1
	}

	// Il campo server `anno` accetta un solo anno: un --data a cavallo di più
	// anni si serve interrogandoli uno per uno. Ordine discendente (bdDateFilter
	// li produce così) perché con --limit piccolo l'utente si aspetta i record
	// più recenti dell'intervallo, come nel resto della CLI.
	// years vuoto = nessun filtro data: un solo giro, con l'eventuale `anno`
	// già impostato sopra dal param omonimo.
	loops := years
	if len(loops) == 0 {
		loops = []string{""}
	}

	var all []Record
	truncated := false
years:
	for yi, y := range loops {
		if y != "" {
			form.Set("anno", y)
		}
		for page := 1; ; page++ {
			form.Set("page", strconv.Itoa(page))
			body, err := c.post(ctx, bdURL, form)
			if err != nil {
				return all, err
			}
			rows, total, err := parseBDList(body, arc, c.BaseURL)
			if err != nil {
				return all, err
			}
			// parseBDRow imposta l'URL della scheda per-record da openRisultati(...);
			// se non ricavabile, fallback alla pagina della banca dati /bd/.
			for i := range rows {
				if rows[i].URL == "" {
					rows[i].URL = bdURL
				}
			}
			if keepDate != nil {
				kept := rows[:0]
				for _, r := range rows {
					if keepDate(r.Fields["Data"]) {
						kept = append(kept, r)
					}
				}
				rows = kept
			}
			all = append(all, rows...)

			if opts.Limit > 0 && len(all) >= opts.Limit {
				// Resta fuori qualcosa se il taglio scarta righe già lette, se
				// l'anno corrente ha altre pagine o se ci sono anni non ancora
				// interrogati: in tutti e tre i casi il set è incompleto.
				truncated = len(all) > opts.Limit || page < total || yi < len(loops)-1
				all = all[:opts.Limit]
				break years
			}
			if page >= total {
				break
			}
			// Con un filtro data attivo scorriamo tutte le pagine dell'anno (bounded),
			// così il filtro client-side vede l'intero anno; altrimenti rispettiamo
			// MaxPages come nel flusso Icaro.
			if keepDate == nil && page >= maxPages {
				truncated = true
				break
			}
		}
	}
	if opts.Truncated != nil {
		*opts.Truncated = truncated
	}
	return all, nil
}

// bdUnsupported riporta un errore se opts porta filtri che il backend /bd/ non
// sa applicare per questo archivio. Il motore Icaro li traduce in espressione
// ISIS; su /bd/ non hanno equivalente e vanno rifiutati invece che ignorati,
// perché un filtro caduto restituisce più record di quanti la riga di comando
// ne chieda — un errore silenzioso, e quindi peggiore di un comando che fallisce.
func bdUnsupported(slug string, spec bdSpec, opts SearchOptions) error {
	// --isis-query non e' un parametro della form, quindi non passa dal
	// costruttore: resta l'unico controllo che vive qui.
	if strings.TrimSpace(opts.ISISRaw) != "" {
		return fmt.Errorf("l'archivio %s è servito dal backend /bd/ del portale, che non supporta --isis-query: rimuovi il filtro (gli altri filtri restano validi)", slug)
	}
	return bdUnsupportedParams(slug, spec, opts.Params)
}

// bdUnsupportedParams e' la meta' che guarda i soli parametri della form, ed e'
// chiamata anche da bdBuildForm: cosi' l'anteprima rifiuta gli stessi filtri
// che il percorso vivo rifiuta, invece di presentarli come "differiti".
func bdUnsupportedParams(slug string, spec bdSpec, params map[string]string) error {
	unsupported := func(flag string) error {
		return fmt.Errorf("l'archivio %s è servito dal backend /bd/ del portale, che non supporta --%s: rimuovi il filtro (gli altri filtri restano validi)", slug, flag)
	}
	// Ordine stabile: le mappe Go iterano a caso e il messaggio d'errore non
	// deve dipendere dal giro.
	keys := make([]string, 0, len(params))
	for k := range params {
		if strings.TrimSpace(params[k]) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch k {
		case "data": // tradotto in anno + filtro client-side
		case "oratore":
			if spec.speakerField == "" {
				return unsupported(k)
			}
		case "codcom", "commissione":
			if spec.commissioneField == "" {
				return unsupported(k)
			}
		default:
			if _, ok := spec.fields[k]; !ok {
				return unsupported(k)
			}
		}
	}
	return nil
}

// bdDateFilter traduce un valore --data (già normalizzato da normalizeParams in
// AAMMGG, oppure ancora in YYYY-MM-DD) in: gli anni per il filtro server e una
// funzione che tiene solo le righe la cui data (dd/mm/yyyy) cade nell'intervallo.
// Ritorna years=nil, keep=nil se il valore non è interpretabile (nessun filtro).
//
// Il campo `anno` del form /bd/ vale per un anno solo: un intervallo a cavallo
// di più anni produce più anni da interrogare in sequenza, dal più recente al
// più vecchio (chi passa --limit si aspetta i record più recenti).
func bdDateFilter(v string) (years []string, keep func(rowDate string) bool) {
	lo, hi, ok := parseDateBounds(v)
	if !ok {
		return nil, nil
	}
	loY, _ := strconv.Atoi(lo[:4])
	hiY, _ := strconv.Atoi(hi[:4])
	for y := hiY; y >= loY; y-- {
		years = append(years, strconv.Itoa(y))
	}
	return years, func(rowDate string) bool {
		d := ddmmyyyyToISO(rowDate)
		return d != "" && d >= lo && d <= hi
	}
}

// parseDateBounds normalizza un valore data (o range) in due estremi yyyymmdd.
// Accetta: YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD, AAMMGG, AAMMGG/AAMMGG.
func parseDateBounds(v string) (lo, hi string, ok bool) {
	split := func(s, sep string) (string, string, bool) {
		if a, b, found := strings.Cut(s, sep); found {
			return a, b, true
		}
		return s, s, false
	}
	// Una data va anche esistita: la sola forma non basta. "2025-13-45" ha la
	// forma giusta e otto cifre, ma come estremo di intervallo non seleziona
	// nulla — `--data 2025-13-45` rispondeva `[]`, cioè «in quel giorno non c'è
	// niente», che di un giorno inesistente è un'affermazione senza senso.
	// time.Parse chiude il buco: se non torna indietro identica, non è una data.
	valida := func(iso string) string {
		if t, err := time.Parse("20060102", iso); err == nil && t.Format("20060102") == iso {
			return iso
		}
		return ""
	}
	toISO := func(s string) string {
		s = strings.TrimSpace(s)
		// YYYY-MM-DD
		if len(s) == 10 && s[4] == '-' && s[7] == '-' {
			iso := s[:4] + s[5:7] + s[8:10]
			if isDigits(iso) {
				return valida(iso)
			}
		}
		// AAMMGG
		if len(s) == 6 && isDigits(s) {
			// Il secolo qui non c'è e va scelto. La finestra è fondata
			// sull'archivio, non su una convenzione generica: il documento più
			// antico servito da /bd/ è il resoconto della seduta inaugurale del
			// 25/05/1947 — nel 1946 non c'è nulla, l'ARS nasce lì — quindi da
			// 47 in su è Novecento e sotto è Duemila. Col prefisso "20" fisso
			// `--data 510412` andava a cercare il 2051 e rispondeva `[]` su una
			// seduta che esiste, quella del 12/04/1951, e con essa su tutte le
			// date fra il 1947 e il 1999 scritte in questa forma.
			//
			// Il prezzo è che AAMMGG non arriva al 2047: lì si scrive la data
			// per esteso, che non è ambigua.
			secolo := "20"
			if s[:2] >= "47" {
				secolo = "19"
			}
			return valida(secolo + s)
		}
		return ""
	}
	// range con ':' (ISO) o '/' (AAMMGG)
	a, b, isRange := split(v, ":")
	if !isRange {
		a, b, isRange = split(v, "/")
	}
	loISO, hiISO := toISO(a), toISO(b)
	if loISO == "" || hiISO == "" {
		return "", "", false
	}
	if loISO > hiISO {
		loISO, hiISO = hiISO, loISO
	}
	return loISO, hiISO, true
}

// isDigits riporta se s è non vuoto e composto solo da cifre.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ddmmyyyyToISO converte "14/07/2026" in "20260714". "" se non riconosciuto.
func ddmmyyyyToISO(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 10 && s[2] == '/' && s[5] == '/' {
		iso := s[6:10] + s[3:5] + s[0:2]
		if isDigits(iso) {
			return iso
		}
	}
	return ""
}

// parseBDList estrae le righe da una risposta HTML /bd/ e il numero di pagine.
// Struttura: <ul class="tabella"> con <li class="intestazione"> (header, saltato)
// e <li> per riga; ogni colonna è <div class="intesta"> con <span class="simobile">
// etichetta + <p> valore; l'ultima colonna ha <h3><a>denominazione</a></h3><p>testo</p>.
func parseBDList(body string, arc Archive, baseURL string) ([]Record, int, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("parsing /bd/ HTML: %w", err)
	}
	totalPages := extractTotalPages(root)

	var ul *html.Node
	walk(root, func(n *html.Node) {
		if ul == nil && n.Type == html.ElementNode && n.Data == "ul" && hasClass(n, "tabella") {
			ul = n
		}
	})
	if ul == nil {
		return nil, totalPages, nil
	}
	var rows []Record
	for li := ul.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		if hasClass(li, "intestazione") {
			continue
		}
		rec := parseBDRow(li, arc.Slug, baseURL)
		if rec.Title != "" || len(rec.Fields) > 0 {
			rows = append(rows, rec)
		}
	}
	return rows, totalPages, nil
}

// parseBDRow legge una singola <li> di risultato in un Record. Le etichette
// "N. Seduta"/"N. Foglio" vengono normalizzate anche su "Numero" così il resto
// della CLI (flatten/emit/sync) trova la chiave attesa.
func parseBDRow(li *html.Node, slug, baseURL string) Record {
	rec := Record{Fields: map[string]string{}}
	for div := li.FirstChild; div != nil; div = div.NextSibling {
		if div.Type != html.ElementNode || div.Data != "div" || !hasClass(div, "intesta") {
			continue
		}
		label := strings.TrimSpace(findSimobileLabel(div))
		// La denominazione (commissione) sta in <h3><a>...; il testo/OdG nel <p>.
		if h3 := firstTextOfTag(div, "h3"); strings.TrimSpace(h3) != "" {
			rec.Title = collapseSpaces(h3)
			if p := nthPText(div, 0); strings.TrimSpace(p) != "" {
				rec.Excerpt = collapseSpaces(p)
			}
			// L'<a> della denominazione richiama openRisultati(...): da lì si
			// costruisce l'URL della scheda per-record.
			var href string
			walk(div, func(n *html.Node) {
				if href == "" && n.Type == html.ElementNode && n.Data == "a" {
					href = attr(n, "href")
				}
			})
			if u := bdSchedaURL(baseURL, slug, href); u != "" {
				rec.URL = u
			}
			continue
		}
		val := collapseSpaces(stripSimobileLabel(textContent(div), label))
		if val == "" || label == "" {
			continue
		}
		switch label {
		case "Legisl.":
			// Il /bd/ mostra la legislatura in numero romano (XVIII); la
			// normalizziamo in arabo ("18") per coerenza con il flusso Icaro e
			// perché lo store/le query locali filtrano su "$.legisl" == "18".
			val = romanToArabic(val)
		case "N. Seduta", "N. Foglio":
			rec.Fields["Numero"] = val
		}
		rec.Fields[label] = val
	}
	return rec
}

// romanToArabic converte un numero romano nella sua forma arabica come stringa.
// Se s non è un numero romano valido lo restituisce invariato.
func romanToArabic(s string) string {
	vals := map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	u := strings.ToUpper(strings.TrimSpace(s))
	total, prev := 0, 0
	for i := len(u) - 1; i >= 0; i-- {
		v, ok := vals[u[i]]
		if !ok {
			return s // non romano
		}
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	if total <= 0 {
		return s
	}
	return strconv.Itoa(total)
}

// post esegue una POST x-www-form-urlencoded usando il jar/limiter del Client.
// Come la GET, viene ritentata se il portale tronca la risposta: è una ricerca,
// quindi rigiocarla non ha effetti collaterali.
func (c *Client) post(ctx context.Context, rawURL string, form url.Values) (string, error) {
	return c.read(ctx, rawURL, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if c.UserAgent != "" {
			req.Header.Set("User-Agent", c.UserAgent)
		}
		req.Header.Set("Origin", c.BaseURL)
		req.Header.Set("Referer", rawURL)
		req.Header.Set("Accept-Language", "it-IT,it;q=0.9")
		return req, nil
	})
}

// SpeakerCount è il numero di sedute d'Aula in cui un oratore è intervenuto.
type SpeakerCount struct {
	Name  string `json:"nome"`
	ID    string `json:"id"`
	Count int    `json:"sedute"`
}

// UnresolvedFilterError segnala che un filtro risolto su un <select> di /bd/
// (--oratore, --commissione) non corrisponde a nessuna voce. È un errore e non
// una lista vuota perché le due cose dicono cose diverse: "non è mai
// intervenuto" contro "questo nome non esiste in anagrafica". Confonderle ha già
// prodotto gap analysis sbagliate.
// InvalidParamError dice che un valore passato dall'utente non è scrivibile in
// quel filtro: non che la ricerca non abbia trovato nulla.
//
// È un tipo a sé perché i comandi che aggregano più archivi — deputato profilo,
// commissione dossier — scartano gli errori di Search per non far cadere l'intero
// report quando un archivio non risponde. Con un errore anonimo, `deputato
// profilo --data <malformata>` finiva in «nessun atto trovato per il deputato
// "Cracolici" (verifica il nome)»: il nome era giusto, sbagliata era la data, e
// l'avviso mandava a cercare dalla parte opposta. Riconoscendo il tipo, quei
// comandi possono propagarlo invece di ingoiarlo.
type InvalidParamError struct {
	Filtro  string
	Valore  string
	Rimedio string
}

func (e *InvalidParamError) Error() string {
	msg := fmt.Sprintf("%s %q non è un valore valido", e.Filtro, e.Valore)
	if e.Rimedio != "" {
		msg += ": " + e.Rimedio
	}
	return msg
}

type UnresolvedFilterError struct {
	Filtro      string
	Valore      string
	Legisl      string
	Disponibili []string // candidati vicini, non l'anagrafica intera
	// Rimedio dice come si scrive un valore valido per QUESTO filtro. Serve
	// perché il rimedio giusto dipende dal filtro: un cognome parziale aiuta su
	// --oratore, ma su --codcom (codice 1-6) manda fuori strada.
	Rimedio string
}

func (e *UnresolvedFilterError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %q non corrisponde a nessuna voce", e.Filtro, e.Valore)
	if e.Legisl != "" {
		fmt.Fprintf(&b, " nella legislatura %s", e.Legisl)
	}
	if len(e.Disponibili) > 0 {
		b.WriteString(". Forse cercavi:")
		for _, n := range e.Disponibili {
			b.WriteString("\n  - " + n)
		}
		return b.String()
	}
	if e.Rimedio != "" {
		b.WriteString(". " + e.Rimedio)
	}
	return b.String()
}

// suggestOptionNames propone le voci che condividono almeno una parola con il
// termine cercato: con "Gallo Afflitto" fa emergere il "Gallo" dell'anagrafica.
// Restituisce al massimo 10 nomi, e nessuno se non c'è affinità (elencare
// centinaia di deputati non aiuterebbe).
func suggestOptionNames(opts []bdOption, term, legisl string) []string {
	words := strings.Fields(strings.ToLower(term))
	seen := map[string]bool{}
	var out []string
	for _, o := range opts {
		if legisl != "" && o.Legs != "" && !legsContains(o.Legs, legisl) {
			continue
		}
		low := strings.ToLower(o.Name)
		for _, w := range words {
			if len(w) < 3 || !strings.Contains(low, w) {
				continue
			}
			if name := strings.TrimSpace(o.Name); name != "" && !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
			break
		}
		if len(out) >= 10 {
			break
		}
	}
	return out
}

// reBDFileHref cattura l'href del documento allegato a una scheda /bd/. Il token
// `?id=<uuid>` è obbligatorio — senza, il server risponde 500 — ma è stabile:
// identifica il documento, non la sessione, quindi l'URL che ne esce si può
// citare e riusare.
var reBDFileHref = regexp.MustCompile(`href="(/bd/[a-z]+/file/[^"]+\.pdf\?id=[^"]+)"`)

// SchedaAllegatoURL risolve la scheda /bd/ di un record e ne estrae l'URL del PDF
// (il resoconto stenografico integrale, per l'archivio resoconti). Ritorna ""
// senza errore se la scheda non espone un allegato.
//
// Serve perché il testo integrale non è nell'indice: l'archivio Icaro ne tiene
// solo frammenti per punto dell'ordine del giorno, e per le sedute recenti non
// ha nulla.
func (c *Client) SchedaAllegatoURL(ctx context.Context, schedaURL string) (string, error) {
	schedaURL = strings.TrimSpace(schedaURL)
	if schedaURL == "" {
		return "", nil
	}
	body, err := c.get(ctx, schedaURL)
	if err != nil {
		return "", fmt.Errorf("scheda %s: %w", schedaURL, err)
	}
	m := reBDFileHref.FindStringSubmatch(body)
	if m == nil {
		return "", nil
	}
	return c.BaseURL + m[1], nil
}

// CommissioniDisponibili elenca le denominazioni delle commissioni indicizzate dal
// backend /bd/, per la legislatura data (vuoto = tutte, comprese quelle storiche).
// Le denominazioni sono la forma che i filtri /bd/ riconoscono per nome ("VI -
// Salute, Servizi Sociali e Sanitari"), includono le commissioni speciali e
// **cambiano da una legislatura all'altra**: vanno enumerate al volo, mai
// memorizzate in una tabella statica.
func (c *Client) CommissioniDisponibili(ctx context.Context, legisl string) ([]string, error) {
	spec, ok := bdArchives["convocazioni"]
	if !ok {
		return nil, fmt.Errorf("archivio convocazioni non configurato per /bd/")
	}
	bdURL := c.BaseURL + "/bd/" + spec.path
	sessionHTML, err := c.get(ctx, bdURL)
	if err != nil {
		return nil, fmt.Errorf("bd session (convocazioni): %w", err)
	}
	legisl = strings.TrimSpace(legisl)
	seen := map[string]bool{}
	var out []string
	for _, o := range parseSelectOptions(sessionHTML, spec.commissioneField) {
		name := strings.TrimSpace(o.Name)
		if name == "" || seen[name] {
			continue
		}
		if legisl != "" && o.Legs != "" && !legsContains(o.Legs, legisl) {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out, nil
}

// SpeakerSessionCounts costruisce la classifica degli oratori per numero di sedute
// (archivio /bd/resoconti). Enumera gli oratori del <select> attivi nella
// legislatura data e, per ciascuno, conta le sedute (una POST per oratore). anno
// opzionale restringe l'anno. progress, se non nil, è chiamato ad ogni oratore.
// Richiede legisl (senza, gli oratori sarebbero ~1000 = troppe richieste).
//
// Il secondo valore elenca gli oratori che il backend non ha misurato: sono 91
// richieste in fila per la legislatura XVIII, e su un portale che ne tronca una
// ogni tanto pretenderle tutte significa non avere mai la classifica. Chi chiama
// deve dichiarare quei nomi, non nasconderli in una classifica che sembra piena.
func (c *Client) SpeakerSessionCounts(ctx context.Context, legisl, anno string, progress func(done, total int)) ([]SpeakerCount, []string, error) {
	if strings.TrimSpace(legisl) == "" {
		return nil, nil, fmt.Errorf("legislatura richiesta per la classifica oratori")
	}
	spec, ok := bdArchives["resoconti"]
	if !ok {
		return nil, nil, fmt.Errorf("archivio resoconti non configurato per /bd/")
	}
	bdURL := c.BaseURL + "/bd/" + spec.path
	sessionHTML, err := c.get(ctx, bdURL)
	if err != nil {
		return nil, nil, fmt.Errorf("bd session (resoconti): %w", err)
	}
	var sel []bdOption
	for _, s := range parseSelectOptions(sessionHTML, spec.speakerField) {
		if s.Legs == "" || legsContains(s.Legs, legisl) {
			sel = append(sel, s)
		}
	}
	out := make([]SpeakerCount, 0, len(sel))
	var persi []string
	for i, s := range sel {
		if ctx.Err() != nil {
			return out, persi, ctx.Err()
		}
		form := url.Values{}
		form.Set("$Ilegislatura", legisl)
		if strings.TrimSpace(anno) != "" {
			form.Set("anno", anno)
		}
		form.Set(spec.speakerField, s.ID)
		form.Set("$S"+spec.speakerField, "or")
		form.Set("page", "1")
		body, err := c.post(ctx, bdURL, form)
		if err != nil {
			// Il 429 fa eccezione: non è una richiesta persa fra le altre, è il
			// backend che chiede tregua. Proseguire gliene sparerebbe altre
			// novanta e perderebbe l'errore su cui il chiamante regola il codice
			// di uscita dedicato.
			if rl := new(HTTPRateLimitError); errors.As(err, &rl) {
				return out, persi, fmt.Errorf("classifica oratori: %w", err)
			}
			// Una richiesta persa non deve portarsi via le novanta riuscite.
			// Con un oratore per richiesta e un backend che tronca a
			// intermittenza, arrendersi al primo errore significava non vedere
			// mai la classifica: si annota chi manca e si prosegue. Chi legge
			// deve saperlo — una classifica parziale spacciata per completa
			// sarebbe la stessa bugia del not-found sui documenti.
			persi = append(persi, s.Name)
			if progress != nil {
				progress(i+1, len(sel))
			}
			continue
		}
		n := 0
		if m := reBDCount.FindStringSubmatch(body); m != nil {
			n, _ = strconv.Atoi(m[1])
		}
		out = append(out, SpeakerCount{Name: s.Name, ID: s.ID, Count: n})
		if progress != nil {
			progress(i+1, len(sel))
		}
	}
	// Zero misurati non è una classifica parziale, è un comando fallito: dirlo
	// come errore invece di restituire una lista vuota, che si leggerebbe come
	// «nessuno è mai intervenuto».
	if len(out) == 0 && len(persi) > 0 {
		return nil, persi, fmt.Errorf("classifica oratori: nessuno dei %d oratori misurato, il backend /bd/ non ha risposto", len(persi))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, persi, nil
}
