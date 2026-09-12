package icaroclient

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// BuildQuery turns a friendly param map (CLI flag values) into the ISIS query
// expression the Icaro engine accepts. The shape mirrors what the JSP form
// produces server-side after a POST: `(<v>.<FIELD> E <v>.<FIELD>) E (<free>)`.
//
// Empty params produce the universal selector `all`, matching every record
// in the archive. Unknown flag names (not in arc.FieldMap) are passed through
// as free-text search terms.
//
// When isisRaw is non-empty it overrides everything: the caller has its own
// fully-formed expression and we ship it verbatim.
func BuildQuery(arc Archive, params map[string]string, isisRaw string) string {
	if isisRaw = strings.TrimSpace(isisRaw); isisRaw != "" {
		return isisRaw
	}
	var fielded []string
	var freeText []string
	var exclude string

	// Stable key order so identical inputs produce identical queries (helps
	// session caching, dogfood reproducibility).
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v, _ := ValoreRipulito(k, params[k])
		if v == "" {
			continue
		}
		// `escludi` is an exclusion term (ISIS NOT), not a positive criterion;
		// it qualifies the whole expression below.
		if k == "escludi" || k == "exclude" {
			exclude = v
			continue
		}
		if k == "testo" || k == "free" || k == "terms" || k == "q" {
			freeText = append(freeText, andJoinWords(v))
			continue
		}
		// `frase` cerca le parole ADIACENTI, nell'ordine dato. È l'accesso
		// esplicito al comportamento nativo di ISIS che andJoinWords converte in
		// AND: l'AND vale sull'intero testo del documento, quindi "aree idonee"
		// aggancia anche un ddl che ha "aree" in un articolo e "idonee" in un
		// altro. Con adj restano solo i documenti che contengono la locuzione.
		if k == "frase" || k == "phrase" {
			freeText = append(freeText, adjJoinWords(v))
			continue
		}
		field, ok := arc.FieldMap[k]
		if !ok {
			// Unmapped flag: drop into free-text as fallback.
			freeText = append(freeText, v)
			continue
		}
		if CampoIdentificativo(k) {
			fielded = append(fielded, fmt.Sprintf("%s.%s", EspressioneIdentificativo(v), field))
			continue
		}
		fielded = append(fielded, fmt.Sprintf("%s.%s", quoteValue(v), field))
	}

	var parts []string
	if len(fielded) > 0 {
		parts = append(parts, "("+strings.Join(fielded, " E ")+")")
	}
	if len(freeText) > 0 {
		// Free text uses AND ("E") between terms unless caller already wrote
		// boolean operators (we don't try to detect this — keep it simple).
		parts = append(parts, "("+strings.Join(freeText, " E ")+")")
	}

	var expr string
	switch len(parts) {
	case 0:
		expr = "all"
	case 1:
		expr = parts[0]
	default:
		expr = strings.Join(parts, " E ")
	}

	// Apply the exclusion (ISIS NOT). We wrap the excluded value in parentheses
	// so a multi-word term stays a single operand. Field-qualified exclusions
	// (e.g. "ospedale.titol") are unreliable upstream, so the CLI flag documents
	// plain-term exclusion; power users can still field-qualify via --isis-query.
	if exclude != "" {
		expr = fmt.Sprintf("(%s) NOT (%s)", expr, exclude)
	}
	return expr
}

// andJoinWords makes a free-text value match documents containing ALL its
// words. ISIS treats space-separated terms as ADJ (adjacency/phrase), so
// "obiezione di coscienza" would only match that exact phrase — surprising for
// a search flag. We join the words with the AND operator (E) instead. If the
// value already uses a boolean operator or parentheses, the caller is writing
// their own expression, so we pass it through verbatim.
func andJoinWords(v string) string {
	if strings.ContainsAny(v, "()") {
		return v
	}
	fields := strings.Fields(v)
	if len(fields) < 2 {
		return v
	}
	for _, f := range fields {
		if isISISOperator(f) {
			return v
		}
	}
	return strings.Join(fields, " E ")
}

// adjJoinWords fa combaciare le parole come locuzione, unendole con
// l'operatore di adiacenza ISIS. Vedi adjExpr per il dettaglio; qui si tiene
// solo l'espressione, perché la maggior parte dei chiamanti non ha modo di
// avvisare l'utente.
func adjJoinWords(v string) string {
	expr, _, _ := adjExpr(v)
	return expr
}

// FraseDegradata torna l'espressione che `--frase` produce per v, i token
// scartati perché sono congiunzioni che collidono col vocabolario ISIS, e i
// token che collidono ma NON si possono scartare.
//
// Servono al livello CLI per avvisare, e dicono due cose diverse: con
// `scartati` il comando ha cercato una prossimità più larga della locuzione
// promessa; con `collisioni` non ha riscritto nulla e la frase è partita
// com'era, che il portale legge come un'espressione booleana.
func FraseDegradata(v string) (string, []string, []string) {
	return adjExpr(v)
}

// congiunzioneCollidente riporta se il token e' una delle due congiunzioni
// italiane che ISIS legge come operatore.
//
// Sono le uniche due parole che si possono togliere da una locuzione senza
// cambiarla: reggono la sintassi e non portano significato proprio. Il
// vocabolario ISIS contiene pero' anche `seguito`, `vicino`, `meno`, `no` ed
// `escluso`, che in italiano sono parole piene: toglierle non attenua la
// ricerca, la falsifica — «aree meno idonee» diventerebbe «aree idonee», cioe'
// il contrario. Quelle fanno uscire la frase intatta, com'era prima.
func congiunzioneCollidente(tok string) bool {
	// Il confronto ignora le maiuscole: qui ci arriva anche il titolo in
	// stampatello, dove la maiuscola non distingue piu' l'operatore dalla
	// parola (vedi adjExpr).
	switch strings.ToLower(tok) {
	case "e", "o":
		return true
	}
	return false
}

// adjExpr costruisce l'espressione di adiacenza per una locuzione e riporta i
// token scartati.
//
// Una parola sola non ha adiacenza da esprimere e passa così com'è; un valore
// che contiene parentesi, o un operatore scritto in maiuscolo, è
// un'espressione voluta da chi chiama e non va toccata.
//
// Il caso che questa funzione esiste per risolvere è la congiunzione italiana
// dentro il titolo di un atto: «coesione e crescita». Il token «e» è anche
// l'operatore AND di ISIS, e finché bastava vederlo per uscire verbatim il
// flag prometteva una locuzione e consegnava un AND — senza dirlo. Ma «e» in
// minuscolo dentro una frase è una parola, non un operatore: la maiuscola è
// l'unico segnale che distingue chi sta scrivendo un'espressione booleana da
// chi sta citando un titolo.
//
// Le stopword si scartano e la distanza le tiene in conto: «coesione e
// crescita» diventa `coesione adj3 crescita`, non `coesione adj crescita`.
// Misurato sul portale: ISIS indicizza la congiunzione come posizione, quindi
// l'adiacenza stretta fra le due parole superstiti non aggancia la locuzione
// vera (sul ddl 969, «prevenzione e contrasto», `adj` torna 3 risultati e non
// lo comprende, `adj2` ne torna 41 e lo comprende, l'AND 144).
//
// La congiunzione vale DUE posizioni, non una: in un titolo italiano si porta
// dietro l'articolo che chi cerca non scrive. Il ddl 1200 si intitola
// «Disposizioni per la coesione e la crescita», e fra le due parole cercate ce
// ne sono tre: con `adj2` la ricerca del nome con cui i giornali chiamano la
// manovra tornava vuota. Misurato sul portale il 2026-09-05: `coesione adj2
// crescita` → 0 risultati, `adj3` → il ddl 1200 in prima posizione. Il costo
// in precisione è quello: sul caso documentato qui sopra, «prevenzione e
// contrasto» passa da 130 a 170 risultati, e il ddl 969 resta dentro in
// entrambi.
func adjExpr(v string) (string, []string, []string) {
	if strings.ContainsAny(v, "()") {
		return v, nil, nil
	}
	fields := strings.Fields(v)
	if len(fields) < 2 {
		return v, nil, nil
	}
	// La maiuscola distingue l'operatore dalla parola solo se nella frase c'è
	// anche qualcosa di minuscolo. Un titolo copiato in stampatello dal
	// portale — «SVILUPPO E COESIONE» — non porta quel segnale: leggere «E»
	// come operatore lì rimetterebbe in piedi l'AND muto, e senza nemmeno
	// l'avviso.
	if strings.ToUpper(v) != v {
		for _, f := range fields {
			// Operatore scritto in maiuscolo: espressione deliberata, intatta.
			if isISISOperator(f) && f == strings.ToUpper(f) {
				return v, nil, nil
			}
		}
	}
	// Una collisione che non e' una congiunzione non si puo' risolvere: la
	// frase esce com'era, come faceva prima, e l'avviso dice perche'.
	var collisioni []string
	for _, f := range fields {
		if isISISOperator(f) && !congiunzioneCollidente(f) {
			collisioni = append(collisioni, f)
		}
	}
	if len(collisioni) > 0 {
		return v, nil, collisioni
	}
	var parti, scartati []string
	distanza := 0
	for _, f := range fields {
		if congiunzioneCollidente(f) {
			scartati = append(scartati, f)
			// Due posizioni: la congiunzione e l'articolo che la accompagna.
			distanza += 2
			continue
		}
		if len(parti) > 0 {
			op := "adj"
			if distanza > 0 {
				op = fmt.Sprintf("adj%d", distanza+1)
			}
			parti = append(parti, op)
		}
		parti = append(parti, f)
		distanza = 0
	}
	// Niente da unire: la frase era fatta di sole congiunzioni. Passa com'era,
	// senza dichiarare una degradazione che non ha prodotto nulla di diverso.
	if len(parti) == 0 {
		return v, nil, nil
	}
	return strings.Join(parti, " "), scartati, nil
}

// isISISOperator reports whether a token is an ISIS boolean/proximity operator
// (any language variant). A trailing digit on proximity operators (NEAR3, ADJ2)
// is tolerated.
func isISISOperator(tok string) bool {
	u := strings.ToUpper(strings.TrimRight(tok, "0123456789"))
	switch u {
	case "E", "AND", "ET", "UND",
		"O", "OR", "OU", "ODER", "XOR",
		"NOT", "NO", "ESCLUSO", "MENO", "EXCLU", "OHNE", "SANS",
		"SAME", "SPARA", "MPARA", "GPARA", "NSAME",
		"WITH", "SFRASE", "NWITH",
		"LINE", "SRIGA", "NLINE",
		"NEAR", "VICINO", "VOISINE", "NAHE",
		"ADJ", "SEGUITO", "SUIVI", "GEFOLGT":
		return true
	}
	return false
}

// quoteValue returns the value as-is for purely alphanumeric/whitespace
// content, and parenthesizes/quotes anything that looks structurally complex
// to keep the ISIS parser happy. The portal's own JSP form just emits the
// value verbatim for typical inputs, so we don't escape over-aggressively.
func quoteValue(v string) string {
	if needsQuoting(v) {
		return "(" + v + ")"
	}
	return v
}

func needsQuoting(v string) bool {
	for _, r := range v {
		if r == ' ' || r == '\t' || r == '(' || r == ')' || r == '.' {
			return true
		}
	}
	return false
}

// ValoreRipulito rende un valore utente digeribile dal parser del portale e
// dice quali caratteri ha tolto.
//
// Il motore ISIS non accetta la punteggiatura dentro il valore di una ricerca:
// non la ignora, RIFIUTA la query. Misurato sull'archivio ddl il 2026-09-06, un
// carattere per volta su ITERST, la pagina d'errore dice «Impossibile creare la
// Query»: ' " , - / . ; : + * ? & ! ( = # . Passano lettere, cifre, spazio, il
// troncamento `$` (`Assembl$` torna righe) e `%`.
//
// Non e' un caso di nicchia: rifiutavano `--iter "Approvato dall'Assemblea"`
// (il nome dello stato scritto dal portale stesso), `--testo "dell'ambiente"`,
// `--materia "sanita'"` e `--firmatario "D'Agostino"`, che e' un cognome
// siciliano. E il rifiuto arrivava all'utente come un problema di soglia, con
// il consiglio di restringere il periodo: una strada che li' non porta da
// nessuna parte.
//
// La riscrittura e' fedele all'indice, non una resa: il portale indicizza la
// punteggiatura come separatore di parole, e in un valore di campo lo spazio e'
// adiacenza. Misurato: `--iter "Approvato dall Assemblea"` torna esattamente le
// 16 righe di `--iter "Assemblea"` sul 2026, mentre `--iter "Assegnato
// Assemblea"` — due stati veri ma non adiacenti — torna vuoto; `--firmatario "D
// Agostino"` torna il ddl 8030, come "Agostino".
//
// Due valori restano intatti. Quelli con parentesi, perche' chi le scrive sta
// scrivendo la propria espressione (stessa convenzione di andJoinWords e
// adjExpr). E quelli di sola struttura — cifre e `/` — che sono i range di data
// costruiti dalla CLI (`260101/261231`), non testo dell'utente.
func ValoreRipulito(param, v string) (string, []string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	if strings.ContainsAny(v, "()") {
		return v, nil
	}
	if campoData(param) && formaData(v) {
		return v, nil
	}
	var rimossi []string
	visto := map[rune]bool{}
	pulito := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '$' || r == '%' {
			return r
		}
		if !visto[r] {
			visto[r] = true
			rimossi = append(rimossi, string(r))
		}
		return ' '
	}, v)
	if len(rimossi) == 0 {
		return v, nil
	}
	return strings.Join(strings.Fields(pulito), " "), rimossi
}

// campoData nomina i parametri il cui valore lo costruisce la CLI: `--data` lo
// compila in `260101/261231` e `--anno`, sui ddl, nello stesso range. Sono i
// soli due posti dove la barra e il trattino sono sintassi.
//
// L'esenzione sta su QUESTI DUE NOMI, non sulla forma del valore, ed e' la
// seconda volta che il perimetro si stringe. Guardando solo i caratteri
// passava un ISBN-13 (`978-88-7524-166-7`, cifre e trattini); guardando la
// forma di data passava una data cercata come TESTO — `ddl cerca --testo
// "2026-07-01"`, cioe' un documento che cita quella data, che e' una domanda
// legittima e usciva col rifiuto del portale invece che ripulita (misurato il
// 2026-09-06: la query partiva come `... E (2026-07-01)`). Un valore non dice
// da se' se e' una data: lo dice il campo in cui sta.
func campoData(param string) bool {
	return param == "data" || param == "anno"
}

// reFormaData riconosce le forme che quei due parametri portano davvero: un
// numero puro, il range ISIS `260101/261231`, e la forma ISO `2026-07-01` o
// `2026-07-01:2026-07-31` che normalizeParams non fosse riuscito a convertire.
//
// La forma ISO e' una difesa, non un percorso vivo: oggi ogni chiamante passa
// da normalizeParams. Ma se un domani una data ISO arrivasse qui intatta,
// ripulirla la trasformerebbe in `2026 07 01 2026 07 31`, cioe' in una query
// che risponde qualcosa di plausibile invece di essere rifiutata — un errore
// silenzioso al posto di uno rumoroso, che e' lo scambio peggiore. Il controllo
// di forma resta quindi accanto a quello sul nome: un `--data` scritto in un
// modo che nessuno riconosce non viene ne' ripulito ne' spedito di nascosto.
var reFormaData = regexp.MustCompile(`^(?:\d+|\d{6}/\d{6}|\d{4}-\d{2}-\d{2}(?::\d{4}-\d{2}-\d{2})?)$`)

func formaData(v string) bool {
	return reFormaData.MatchString(v)
}

// CampoIdentificativo dice se il valore di quel parametro e' un identificativo
// scritto a gruppi, dove la punteggiatura e' formattazione e non un confine di
// parola.
//
// E' l'eccezione alla regola generale, e non si deduce dalla forma del valore:
// due campi numerici dello stesso archivio si comportano al contrario. Misurato
// il 2026-09-06 sull'archivio 205:
//
//	--dewey "340.5"  -> `(340 5).DEWEY` trova 1 record: li' il punto separa
//	                    davvero due token, e la riscrittura in spazio e' giusta.
//	--isbn  "978-88-7524-166-7" -> `(978 88 7524 166 7).ISBN` trova 0, mentre
//	                    `9788875241667.ISBN` trova il record: quell'ISBN sta
//	                    nell'indice come un token solo.
//
// L'archivio pero' non e' coerente con se stesso — un altro record porta
// `978 88 98231-25-6`, spazi e trattini insieme, e li' vale l'opposto: la forma
// unita trova 0 e quella a spazi trova il record. Nessuna delle due riscritture
// da sola copre il catalogo, quindi si spediscono entrambe in OR (vedi
// EspressioneIdentificativo). Verificato sui due record: l'OR li trova tutti e
// due, ciascuna forma da sola ne perde uno.
func CampoIdentificativo(param string) bool {
	return param == "isbn"
}

// EspressioneIdentificativo costruisce l'espressione che aggancia un
// identificativo in tutte le grafie con cui l'archivio lo tiene.
//
// La prima versione univa due sole grafie — quella unita e quella coi
// separatori resi spazio — e bastava finche' l'utente scriveva i separatori.
// Chi incolla le tredici cifre nude non ne ha, quindi partiva la sola grafia
// unita: **l'8% dell'archivio**, con `troncato: false` sul restante 92%. Una
// lista vuota che si presenta come una risposta, che e' il difetto peggiore
// che questa CLI conosca.
//
// Dalle cifre nude il raggruppamento non si deduce, perche' dipende dal
// registrante. Si enumera: per un ISBN-13 il prefisso e' fisso (tre cifre) e
// il carattere di controllo e' l'ultimo, quindi restano nove cifre da spezzare
// in gruppo (1-5 cifre), registrante e pubblicazione — **25 combinazioni**, un
// numero costante, non una ricerca. Vanno tutte in OR con la grafia unita, e
// costano UNA richiesta.
//
// Misurato il 2026-09-06 sull'archivio 205, coi due controlli in entrambi i
// versi: `9788898231256` (record tenuto in grafia separata) e `9788875241667`
// (record tenuto in grafia unita) tornano ciascuno il proprio documento, e due
// ISBN inesistenti — un carattere di controllo sbagliato e tredici cifre a
// caso — tornano zero. L'espressione e' lunga circa 950 caratteri e il portale
// la accetta.
//
// Fuori da questa forma non si inventa nulla: un valore che non e' tredici
// cifre con prefisso 978/979 (un ISBN-10, un identificativo di un altro
// genere) ricade sul comportamento precedente, che copre chi i separatori li
// scrive.
func EspressioneIdentificativo(pulito string) string {
	if grafie := grafieISBN13(pulito); grafie != "" {
		return grafie
	}
	unito := strings.ReplaceAll(pulito, " ", "")
	if unito == pulito {
		return pulito // nessun separatore: una grafia sola, niente da unire
	}
	return fmt.Sprintf("(%s O (%s))", unito, pulito)
}

// grafieISBN13 enumera le grafie di un ISBN-13 e le unisce in OR, oppure torna
// "" se il valore non e' un ISBN-13.
//
// La catena di ogni candidato e' `prefisso adj gruppo adj registrante adj
// pubblicazione adj controllo`: `adj` e' l'adiacenza ISIS, e ricostruisce
// esattamente un ISBN tenuto a token separati. Discrimina: misurato, una cifra
// di controllo sbagliata o due gruppi scambiati tornano zero.
func grafieISBN13(pulito string) string {
	cifre := strings.ReplaceAll(pulito, " ", "")
	if len(cifre) != 13 || !soloCifre(cifre) {
		return ""
	}

	if p := cifre[:3]; p != "978" && p != "979" {
		return ""
	}
	prefisso, mezzo, controllo := cifre[:3], cifre[3:12], cifre[12:]
	grafie := []string{cifre} // la grafia unita, un token solo
	// Il gruppo linguistico va da una a cinque cifre (ISO 2108); registrante e
	// pubblicazione si spartiscono il resto, almeno una cifra ciascuno.
	for g := 1; g <= 5 && g < len(mezzo)-1; g++ {
		gruppo, resto := mezzo[:g], mezzo[g:]
		for r := 1; r < len(resto); r++ {
			grafie = append(grafie, fmt.Sprintf("(%s adj %s adj %s adj %s adj %s)",
				prefisso, gruppo, resto[:r], resto[r:], controllo))
		}
	}
	return "(" + strings.Join(grafie, " O ") + ")"
}

// ValoriRipuliti riporta, per una mappa di parametri, i caratteri che
// ValoreRipulito toglierebbe. Serve al livello CLI per avvisare: la
// riscrittura avviene dentro BuildQuery, dove non c'e' modo di parlare
// all'utente (stessa divisione di FraseDegradata).
func ValoriRipuliti(params map[string]string) (map[string][]string, bool) {
	out := map[string][]string{}
	for k, v := range params {
		if _, rimossi := ValoreRipulito(k, v); len(rimossi) > 0 {
			out[k] = rimossi
		}
	}
	return out, len(out) > 0
}

// NumeroGrafie dice quante grafie l'espressione mette in OR, per gli avvisi:
// il testo dell'espressione e' lungo quasi mille caratteri e in un hint non si
// legge, il conteggio si'.
func NumeroGrafie(pulito string) int {
	if g := grafieISBN13(pulito); g != "" {
		return strings.Count(g, " O ") + 1
	}
	if strings.Contains(strings.TrimSpace(pulito), " ") {
		return 2
	}
	return 1
}
