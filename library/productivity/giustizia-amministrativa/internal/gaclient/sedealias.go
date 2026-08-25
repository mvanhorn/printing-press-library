package gaclient

import "strings"

// Naming a sede is where a query is most easily lost. The portal's own filter
// wants a city ("Roma"), but the same sede is called three other ways in
// material a caller has at hand: the code inside the ECLI this very CLI emits
// (ECLI:IT:TARLAZ:2026:14259SENT), the name of the region ("TAR Lazio"), and
// the codes the pre-2026 portal used, which survive in older documents and in
// what a language model recalls. Rejecting those spellings answers "sede non
// riconosciuta" to someone who named the sede correctly, in one of the
// spellings we ourselves print.
//
// sedeAliases maps every such spelling to a key of sedeMap. The ECLI codes are
// the ones actually observed in the store (31 sedi, one per portal seat), not
// a guessed list.
var sedeAliases = map[string]string{
	// Codes carried by the ECLI of each sede.
	"cds": "consiglio-di-stato", "cgars": "cgars",
	"tarlaz": "roma", "tarlt": "latina", "tarna": "napoli", "tarsa": "salerno",
	"tarmi": "milano", "tarbs": "brescia", "tarpa": "palermo", "tarct": "catania",
	"tarven": "venezia", "tarpie": "torino", "tarbo": "bologna", "tarpr": "parma",
	"tartos": "firenze", "tarba": "bari", "tarle": "lecce", "tarcz": "catanzaro",
	"tarrc": "reggio-calabria", "tarlig": "genova", "tarsar": "cagliari",
	"tarfvg": "trieste", "tarmar": "ancona", "taraq": "laquila", "tarpe": "pescara",
	"tarumb": "perugia", "tarmol": "campobasso", "tarbas": "potenza",
	"trgatn": "trento", "trgabz": "bolzano", "tarvda": "aosta",

	// Codes used by the pre-2026 portal, still quoted in older material.
	// TARBOL (Bolzano) and TARABR (Pescara) are deliberately absent: read as
	// abbreviations they point at Bologna and at L'Aquila, and a filter that
	// silently answers for the wrong sede is worse than one that refuses.
	"tarlom": "milano", "tarcam": "napoli", "tarcamsal": "salerno",
	"tarsic": "palermo", "tarsiccat": "catania", "taremi": "bologna",
	"tarpug": "bari", "tarpuglec": "lecce", "tarcal": "catanzaro",
	"tarcalreg": "reggio-calabria", "tarfri": "trieste", "tarabrlaq": "laquila",
	"tartretn": "trento",

	// Regions, resolving to the seat of the TAR. Where a region has a second
	// seat, that one is addressable by name (see sediStaccate).
	"lazio": "roma", "lombardia": "milano", "campania": "napoli",
	"sicilia": "palermo", "veneto": "venezia", "piemonte": "torino",
	"emilia-romagna": "bologna", "emilia": "bologna", "toscana": "firenze",
	"puglia": "bari", "calabria": "catanzaro", "liguria": "genova",
	"sardegna": "cagliari", "friuli": "trieste", "friuli-venezia-giulia": "trieste",
	"marche": "ancona", "abruzzo": "laquila", "umbria": "perugia",
	"molise": "campobasso", "basilicata": "potenza", "trentino": "trento",
	"trentino-alto-adige": "trento", "valle-daosta": "aosta", "valdaosta": "aosta",

	// Regions plus the seat, for the sedi staccate.
	"lazio-latina": "latina", "lombardia-brescia": "brescia",
	"campania-salerno": "salerno", "sicilia-catania": "catania",
	"puglia-lecce": "lecce", "calabria-reggio-calabria": "reggio-calabria",
	"calabria-reggio": "reggio-calabria", "emilia-romagna-parma": "parma",
	"abruzzo-pescara": "pescara", "abruzzo-laquila": "laquila",
	"trentino-bolzano": "bolzano", "trentino-trento": "trento",

	"consiglio-stato": "consiglio-di-stato",
	"cga":             "cgars",
}

// sediStaccate names, for a region whose case law is split across two seats,
// the seat an alias for the region alone leaves out. Filtering by "sicilia"
// searches Palermo and returns nothing from Catania: true of the portal's own
// filter too, but invisible unless it is said.
var sediStaccate = map[string]string{
	"roma":      "Latina",
	"milano":    "Brescia",
	"napoli":    "Salerno",
	"palermo":   "Catania",
	"bari":      "Lecce",
	"catanzaro": "Reggio Calabria",
	"bologna":   "Parma",
	"laquila":   "Pescara",
	"trento":    "Bolzano",
}

// regioniDoppiaSede lists the aliases that name a whole region, so the warning
// above is raised only for those and not for a caller who asked for the city.
var regioniDoppiaSede = map[string]bool{
	"lazio": true, "lombardia": true, "campania": true, "sicilia": true,
	"puglia": true, "calabria": true, "emilia-romagna": true, "emilia": true,
	"abruzzo": true, "trentino": true, "trentino-alto-adige": true,
}

// sedeKey normalises a sede as written by the caller: case, separators,
// apostrophes and dots all vary, and a leading "tar"/"trga" is noise once the
// rest names the sede ("TAR Lazio" and "lazio" are the same filter).
func sedeKey(s string) string {
	key := strings.ToLower(strings.TrimSpace(s))
	key = strings.NewReplacer(" ", "-", "_", "-", "'", "", ".", "").Replace(key)
	for strings.HasPrefix(key, "tar-") || strings.HasPrefix(key, "trga-") {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(key, "trga-"), "tar-")
		if trimmed == "" {
			break
		}
		key = trimmed
	}
	return strings.Trim(key, "-")
}

// resolveSede returns the sedeMap key a spelling refers to, and whether it
// names a sede at all.
func resolveSede(s string) (string, bool) {
	key := sedeKey(s)
	if key == "" {
		return "", false
	}
	if _, ok := sedeMap[key]; ok {
		return key, true
	}
	if alias, ok := sedeAliases[key]; ok {
		return alias, true
	}
	return "", false
}

// sedeAliasWarning returns the notice to raise when the caller named a whole
// region and the search therefore covered only its main seat.
func sedeAliasWarning(input string) string {
	key := sedeKey(input)
	if !regioniDoppiaSede[key] {
		return ""
	}
	target, ok := sedeAliases[key]
	if !ok {
		return ""
	}
	staccata, ok := sediStaccate[target]
	if !ok {
		return ""
	}
	return "\"" + strings.TrimSpace(input) + "\" e' stata interpretata come la sede di " + sedeMap[target] +
		": quella regione ha anche la sede di " + staccata +
		", che questa ricerca non copre. Interrogala con --sede " + strings.ToLower(staccata) +
		", oppure usa --sede-sweep per tutte le sedi"
}
