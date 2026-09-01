package icaroclient

import (
	"fmt"
	"sort"
	"strings"
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
		v := strings.TrimSpace(params[k])
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
// crescita» diventa `coesione adj2 crescita`, non `coesione adj crescita`.
// Misurato sul portale: ISIS indicizza la congiunzione come posizione, quindi
// l'adiacenza stretta fra le due parole superstiti non aggancia la locuzione
// vera (sul ddl 969, «prevenzione e contrasto», `adj` torna 3 risultati e non
// lo comprende, `adj2` ne torna 41 e lo comprende, l'AND 144).
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
			distanza++
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
