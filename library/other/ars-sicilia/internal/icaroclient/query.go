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
// l'operatore di adiacenza ISIS. Una parola sola non ha adiacenza da esprimere
// e passa così com'è; un valore che contiene già parentesi o operatori è
// un'espressione scritta dal chiamante e non va toccata (stesse guardie di
// andJoinWords).
func adjJoinWords(v string) string {
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
	return strings.Join(fields, " adj ")
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
