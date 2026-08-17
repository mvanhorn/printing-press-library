// pp:client-call
// Novel feature — fin dove arriva la FONTE, archivio per archivio.
//
// `sync stale` dice quanto è vecchia la copia locale. Non dice l'altra metà, che
// è quella che serve quando una ricerca risponde `[]`: se il portale quel
// documento non l'ha ancora indicizzato, l'assenza è latenza della fonte, non
// assenza dell'atto. Misurato a mano il 13/08/2026: l'archivio ddl era fermo al
// 28/07, e due notizie di agosto risultavano «inesistenti» solo per questo.
//
// Perché non basta chiedere un record solo: l'ordinamento della fonte NON è
// uniforme. Verificato dal vivo: `ddl cerca --anno 2026` esce dal più recente
// (28.07.26 in cima), `leggi cerca --anno 2026` esce dal più vecchio (L.R. 1 del
// 5.01.2026 in cima). Un probe `--limit 1` darebbe quindi la data giusta su un
// archivio e quella sbagliata sull'altro, e sarebbe un dato falso proprio nel
// comando dove si va a decidere «la fonte è indietro» o «l'atto non esiste».
// Qui l'ordine non si assume: si legge la prima pagina e si guarda com'è fatta.
// Se le date scendono, il massimo è già in cima e il giro finisce lì; altrimenti
// si scarica l'anno intero e il massimo si calcola sulle righe lette.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/store"
	"github.com/spf13/cobra"
)

// annoIndietroMax è quanti anni si torna indietro quando l'anno corrente è
// vuoto: un archivio poco movimentato (pareri, biblioteca) può non avere nulla
// nell'anno in corso senza per questo essere fermo.
const annoIndietroMax = 2

// coverageLimitAnno è il tetto di righe per la scansione dell'anno intero. Serve
// solo agli archivi in ordine crescente: 142 ddl e 50 resoconti nel 2026, quindi
// è largo abbastanza da non tagliare, e comunque il taglio viene dichiarato.
const coverageLimitAnno = 1000

func newNovelSyncCoverageCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB        string
		flagResources []string
	)
	cmd := &cobra.Command{
		Use:   "coverage",
		Args:  rejectPositionalArgs,
		Short: "Data del record più recente sul PORTALE, archivio per archivio: distingue «la fonte è indietro» da «l'atto non esiste».",
		Example: "  ars-sicilia-pp-cli sync coverage --resources ddl --json\n" +
			"  ars-sicilia-pp-cli sync coverage --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliIsVerify() {
				return nil
			}
			return runSyncCoverage(cmd, flags, flagDB, flagResources)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Percorso del database SQLite, per confrontare con l'ultima sync locale (default: ~/.local/share/ars-sicilia-pp-cli/data.db).")
	cmd.Flags().StringSliceVar(&flagResources, "resources", nil, "Archivi da misurare (default: tutti i 12). Es: --resources ddl,resoconti")
	return cmd
}

type coverageEntry struct {
	Archivio string `json:"archivio"`
	// UltimoRecord è la data (YYYY-MM-DD) del documento più recente che la fonte
	// espone per questo archivio.
	UltimoRecord string `json:"ultimo_record_fonte,omitempty"`
	// RitardoGiorni è quanti giorni separano quel documento da oggi. È la misura
	// che risponde alla domanda: una notizia più recente di così non può ancora
	// essere nell'archivio.
	//
	// È un puntatore perché zero e "non misurato" sono cose diverse: con un int
	// e omitempty un archivio perfettamente aggiornato spariva dal JSON insieme a
	// quelli che non avevamo saputo misurare, e senza omitempty un archivio in
	// errore avrebbe dichiarato "0 giorni di ritardo".
	RitardoGiorni *int `json:"ritardo_fonte_giorni,omitempty"`
	// AnnoLetto è la finestra effettivamente interrogata.
	AnnoLetto int `json:"anno_letto,omitempty"`
	// Righe è quante righe sono state lette per arrivare alla risposta.
	Righe int `json:"righe_lette"`
	// Parziale segnala che la finestra è stata tagliata (limite o pagine): il
	// massimo letto è allora un minimo garantito, non necessariamente il massimo
	// dell'anno. Si dichiara invece di spacciare per esatta una stima.
	Parziale bool `json:"parziale,omitempty"`
	// UltimaSyncLocale è il timestamp dell'ultima sync della copia locale, per
	// leggere le due latenze — quella della fonte e quella della copia — insieme.
	UltimaSyncLocale string `json:"ultima_sync_locale,omitempty"`
	Nota             string `json:"nota,omitempty"`
	Errore           string `json:"errore,omitempty"`
}

func runSyncCoverage(cmd *cobra.Command, flags *rootFlags, dbPath string, resources []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	archivi, err := coverageArchivi(resources)
	if err != nil {
		return usageErr(err)
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	probe := &sonda{c: c}
	syncLocale := ultimeSyncLocali(ctx, dbPath)

	oggi := time.Now()
	entries := make([]coverageEntry, 0, len(archivi))
	for _, arc := range archivi {
		e := misuraCopertura(ctx, probe, arc, oggi)
		e.UltimaSyncLocale = syncLocale[arc.Slug]
		entries = append(entries, e)
	}
	// In testa gli archivi più indietro, in coda quelli non misurati: un archivio
	// che non abbiamo saputo leggere non è il più aggiornato di tutti.
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].RitardoGiorni, entries[j].RitardoGiorni
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		return *a > *b
	})
	return emitCoverage(cmd, flags, entries)
}

// coverageArchivi risolve --resources in archivi, rifiutando gli slug ignoti:
// uno slug scritto male che venisse ignorato darebbe un rapporto silenziosamente
// più corto di quello chiesto.
func coverageArchivi(resources []string) ([]icaro.Archive, error) {
	if len(resources) == 0 {
		return icaro.All, nil
	}
	var out []icaro.Archive
	for _, r := range resources {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		arc := icaro.BySlug(r)
		if arc == nil {
			noti := make([]string, 0, len(icaro.All))
			for _, a := range icaro.All {
				noti = append(noti, a.Slug)
			}
			return nil, fmt.Errorf("archivio sconosciuto %q (noti: %s)", r, strings.Join(noti, ", "))
		}
		out = append(out, *arc)
	}
	return out, nil
}

// misuraCopertura trova la data del record più recente di un archivio senza
// assumerne l'ordinamento (vedi la nota in testa al file).
func misuraCopertura(ctx context.Context, s *sonda, arc icaro.Archive, oggi time.Time) coverageEntry {
	e := coverageEntry{Archivio: arc.Slug}
	if finestraAnno(arc, 2000) == nil {
		// Nessun filtro temporale su questo archivio: resta la lettura non
		// filtrata della prima pagina, che vale solo se l'ordine scende davvero
		// (è la misura manuale `cerca --limit 1`, ma con l'ordine verificato
		// invece che sperato). `pareri` sta qui. `biblioteca` no: non ha proprio
		// una colonna data, quindi non c'è niente da misurare.
		return coperturaSenzaFinestra(ctx, s, arc, oggi, e)
	}
	for indietro := 0; indietro <= annoIndietroMax; indietro++ {
		anno := oggi.Year() - indietro
		max, righe, parziale, err := massimoDellAnno(ctx, s, arc, anno)
		e.AnnoLetto = anno
		e.Righe = righe
		e.Parziale = parziale
		if err != nil {
			e.Errore = erroreLeggibile(arc, err)
			return e
		}
		if max == "" {
			if righe > 0 {
				// Righe lette ma nessuna data leggibile: non è un anno vuoto, è un
				// anno che non sappiamo datare. Proseguire fino in fondo al ciclo
				// farebbe dire «nessun documento negli ultimi 3 anni» a un archivio
				// che i documenti ce li ha — l'esatta affermazione che questo
				// comando esiste per non fare.
				e.Errore = "le date di questo archivio non sono in una forma interpretabile: la copertura non è misurabile"
				return e
			}
			continue // anno senza documenti: si guarda quello prima
		}
		datare(&e, max, oggi)
		return e
	}
	e.Errore = fmt.Sprintf("nessun documento negli ultimi %d anni", annoIndietroMax+1)
	return e
}

// coperturaSenzaFinestra misura gli archivi che non hanno né `anno` né `data`
// fra i campi interrogabili. Senza finestra non si può leggere «tutto l'anno»
// per prendere il massimo: o l'ordine di consegna scende — e allora la prima
// riga è il record più recente — oppure la misura non si può dare, e si dice.
func coperturaSenzaFinestra(ctx context.Context, s *sonda, arc icaro.Archive, oggi time.Time, e coverageEntry) coverageEntry {
	if !haColonnaData(arc) {
		e.Errore = "archivio senza colonna data: la copertura temporale non è misurabile"
		return e
	}
	recs, _, err := s.cerca(ctx, arc, nil, 20, 1)
	if err != nil {
		e.Errore = erroreLeggibile(arc, err)
		return e
	}
	e.Righe = len(recs)
	date := dateInOrdine(recs)
	if len(date) == 0 {
		// È il caso di `pareri`: il portale scrive lì le date a parole e spesso
		// tagliate («17 luglio 2»), quindi non c'è niente di confrontabile.
		e.Errore = "le date di questo archivio non sono in una forma interpretabile: la copertura non è misurabile"
		return e
	}
	if !decrescente(date) {
		e.Errore = "archivio senza filtro per anno e senza ordine decrescente verificabile: la copertura non è misurabile in modo affidabile"
		return e
	}
	datare(&e, date[0], oggi)
	return e
}

func datare(e *coverageEntry, max string, oggi time.Time) {
	e.UltimoRecord = max
	t, err := time.Parse("2006-01-02", max)
	if err != nil {
		return
	}
	// Giorni di calendario, non ore diviso 24: `t` è mezzanotte UTC mentre `oggi`
	// è l'ora locale, e la differenza fra i due fusi bastava a far sparire il
	// segno. Una seduta annunciata per domani dava −15h → int(−0,625) → 0, cioè
	// «archivio aggiornato a oggi», e la nota sulle date future non usciva mai
	// sotto le 24 ore di anticipo.
	oggiData := time.Date(oggi.Year(), oggi.Month(), oggi.Day(), 0, 0, 0, 0, time.UTC)
	giorni := int(oggiData.Sub(t).Hours() / 24)
	e.RitardoGiorni = &giorni
	if giorni < 0 {
		// Non è un errore di misura: `convocazioni` annuncia sedute future, e il
		// suo record più recente è normalmente in avanti nel tempo. Senza la nota
		// un ritardo negativo si legge come un bug.
		e.Nota = "il record più recente è una data futura: questo archivio annuncia sedute ancora da tenere"
	}
}

// haColonnaData dice se il listato dell'archivio espone una colonna data: senza
// quella, nessuna riga porta una data da confrontare.
func haColonnaData(arc icaro.Archive) bool {
	for _, col := range arc.Columns {
		if col == "Data" {
			return true
		}
	}
	return false
}

// erroreLeggibile riscrive il guasto del backend /bd/ in quello che significa
// per chi legge: quel backend tronca le risposte grandi e la CLI ritenta già da
// sé, quindi qui non è «archivio vuoto», è «non ha risposto, riprova».
func erroreLeggibile(arc icaro.Archive, err error) string {
	if icaro.IsBDArchive(arc.Slug) {
		return "il backend /bd/ non ha consegnato la risposta (tronca le pagine grandi): non è assenza di dato, riprova — " + err.Error()
	}
	return err.Error()
}

// massimoDellAnno legge la prima pagina dell'anno; se le date scendono (ordine
// decrescente, verificato sulle righe lette e non dato per scontato) il massimo
// è la prima ed è finita, altrimenti scarica l'anno e prende il massimo.
func massimoDellAnno(ctx context.Context, s *sonda, arc icaro.Archive, anno int) (max string, righe int, parziale bool, err error) {
	params := finestraAnno(arc, anno)
	prime, _, err := s.cerca(ctx, arc, params, 20, 1)
	if err != nil {
		return "", 0, false, err
	}
	date := dateInOrdine(prime)
	if len(date) == 0 {
		return "", len(prime), false, nil
	}
	if decrescente(date) {
		return date[0], len(prime), false, nil
	}
	// L'ordine non scende: il record più recente può stare in fondo, quindi
	// l'anno va letto tutto. È il caso di `leggi`, che esce dal più vecchio.
	// Sessione nuova: è la seconda volta che si fa questa identica ricerca, e
	// ripeterla sulla stessa sessione la fa ricominciare più in basso (vedi
	// sondaNuova). Qui il danno sarebbe silenzioso — un massimo calcolato su
	// righe che non sono quelle che si crede di avere letto.
	tutte, troncato, err := sondaNuova().cerca(ctx, arc, params, coverageLimitAnno, 0)
	if err != nil {
		return "", len(prime), false, err
	}
	date = dateInOrdine(tutte)
	if len(date) == 0 {
		return "", len(tutte), troncato, nil
	}
	return massimo(date), len(tutte), troncato, nil
}

// finestraAnno costruisce i filtri che restringono un archivio a un anno solo,
// e ritorna nil se l'archivio non ha alcuna dimensione temporale.
//
// La scelta non è cosmetica: `anno` è un campo vero solo su leggi (LEGANN),
// resoconti (ANNSED), ddl (mappato sul range DATPRE) e sui tre archivi /bd/,
// dove è nativo del form. Sugli altri — interrogazioni, mozioni, odg,
// risoluzioni, pareri, interpellanze — `anno` non sta nella FieldMap, e
// BuildQuery degrada gli sconosciuti in ricerca libera: si finirebbe a cercare
// «2026» come testo dentro i documenti e a spacciare il risultato per una
// misura di copertura. Lì la finestra si esprime sul campo data, che esiste.
// `biblioteca` non ha nessuno dei due (né colonna data nel listato): non è
// misurabile e lo si dice, invece di rispondere con un numero inventato.
func finestraAnno(arc icaro.Archive, anno int) map[string]string {
	if icaro.IsBDArchive(arc.Slug) {
		return map[string]string{"anno": itoa(anno)}
	}
	if _, ok := arc.FieldMap["anno"]; ok {
		return map[string]string{"anno": itoa(anno)}
	}
	if _, ok := arc.FieldMap["data"]; ok {
		y := itoa(anno)
		return map[string]string{"data": y + "-01-01:" + y + "-12-31"}
	}
	return nil
}

func (s *sonda) cerca(ctx context.Context, arc icaro.Archive, params map[string]string, limit, maxPages int) ([]icaro.Record, bool, error) {
	searchParams := params
	if !icaro.IsBDArchive(arc.Slug) {
		searchParams = normalizeParams(arc, params)
	}
	if maxPages == 0 {
		maxPages = (limit + 9) / 10
	}
	// Sugli archivi /bd/ il tentativo si ripete, con una sessione nuova a ogni
	// giro. Il motivo del ritentativo è accertato: quel backend consegna la
	// pagina tagliata a intermittenza (20 GET su /bd/resoconti → 14 integre, 6
	// tagliate, misurate il 12/08). Il motivo della sessione nuova no: qui è
	// sembrata passare più spesso, ma su una manciata di prove — e questo
	// progetto ha già scambiato per segnale un campione così (vedi LOG.md,
	// 12/08, l'ipotesi sulle connessioni riusate, smentita a 40 prove). Costa
	// zero e non fa danno, quindi resta; non la si dia per dimostrata.
	// Sugli altri archivi non si ritenta: lì un errore è un errore.
	tentativi := 1
	if icaro.IsBDArchive(arc.Slug) {
		tentativi = 3
	}
	var troncato bool
	var err error
	for i := 0; i < tentativi; i++ {
		if i > 0 && !s.rinnova() {
			break
		}
		var recs []icaro.Record
		recs, err = s.c.Search(ctx, arc, icaro.SearchOptions{
			Params:    searchParams,
			Limit:     limit,
			MaxPages:  maxPages,
			Truncated: &troncato,
		})
		if err == nil {
			return recs, troncato, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil, false, err
}

// sonda tiene il client con cui si interroga il portale, e sa sostituirlo:
// vedi la nota in `cerca` sul perché una sessione nuova riesce dove il
// ritentativo sulla stessa sessione no.
type sonda struct{ c *icaro.Client }

// sondaNuova apre una sessione pulita.
//
// Serve perché una sessione riusata non ripete la stessa ricerca: la continua.
// Misurato sulle interrogazioni del 2026, tre chiamate identiche a 30 righe:
// con lo stesso client la prima parte dal 3 agosto e le successive dal 28
// luglio; con un client nuovo ogni volta partono tutte dal 3 agosto. I
// documenti più recenti sparivano senza che niente lo segnalasse, ed è il caso
// peggiore per un comando che misura fin dove arriva la fonte.
func sondaNuova() *sonda {
	c, err := icaro.New(nil)
	if err != nil {
		return &sonda{}
	}
	return &sonda{c: c}
}

func (s *sonda) rinnova() bool {
	c, err := icaro.New(nil)
	if err != nil {
		return false
	}
	s.c = c
	return true
}

// dateInOrdine estrae le date delle righe in forma ordinabile (YYYY-MM-DD)
// MANTENENDO l'ordine di consegna della fonte: è quell'ordine che nonCrescente
// deve poter guardare. Le righe senza data leggibile sono scartate, non
// trasformate in stringhe vuote che romperebbero il confronto.
func dateInOrdine(recs []icaro.Record) []string {
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		if d := dataOrdinabile(r.Fields["Data"]); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// dataOrdinabile normalizza le due forme in cui le date arrivano dai due motori:
// `28.07.26` dal flusso Icaro, `05/08/2026` dalle pagine /bd/.
func dataOrdinabile(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "/", "."))
	if s == "" {
		return ""
	}
	iso := parseICaroDate(s)
	if len(iso) != 10 || iso[4] != '-' {
		return ""
	}
	return iso
}

// decrescente dice se le date, nell'ordine in cui la fonte le ha consegnate,
// non salgono mai E scendono almeno una volta: solo allora la prima riga è
// davvero il massimo e una pagina basta.
//
// Il «almeno una volta» non è pedanteria, è il caso `leggi`: quell'archivio è
// indicizzato per articolo, e le prime dieci righe del 2026 sono i dieci
// articoli della L.R. 1, tutti datati 5.01.2026. Una sequenza costante soddisfa
// «non sale mai» senza dire niente sull'ordinamento: fermarsi lì rispondeva
// 05.01.2026 su un archivio che nel 2026 arriva almeno al 13.03 — cioè
// esattamente il dato falso che questo comando esiste per non produrre.
func decrescente(date []string) bool {
	if len(date) < 2 {
		return false
	}
	scende := false
	for i := 1; i < len(date); i++ {
		if date[i] > date[i-1] {
			return false
		}
		if date[i] < date[i-1] {
			scende = true
		}
	}
	return scende
}

// massimo ritorna la data più recente (le stringhe YYYY-MM-DD si confrontano
// come date).
func massimo(date []string) string {
	m := date[0]
	for _, d := range date[1:] {
		if d > m {
			m = d
		}
	}
	return m
}

func ultimeSyncLocali(ctx context.Context, dbPath string) map[string]string {
	out := map[string]string{}
	if dbPath == "" {
		dbPath = defaultDBPath("ars-sicilia-pp-cli")
	}
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return out // nessuna copia locale: la colonna resta vuota
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx, `SELECT resource_type, last_synced_at FROM sync_state`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var slug, ts string
		if err := rows.Scan(&slug, &ts); err == nil {
			out[slug] = ts
		}
	}
	return out
}

func emitCoverage(cmd *cobra.Command, flags *rootFlags, entries []coverageEntry) error {
	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		return printJSONFiltered(out, entries, flags)
	}
	fmt.Fprintf(out, "%-15s %-14s %-9s %-7s %s\n", "ARCHIVIO", "ULTIMO RECORD", "RITARDO", "RIGHE", "NOTE")
	for _, e := range entries {
		nota := e.Errore
		if nota == "" && e.Parziale {
			nota = "finestra tagliata: la data è un minimo, non il massimo dell'anno"
		}
		if nota == "" {
			nota = e.Nota
		}
		ritardo := "—"
		if e.RitardoGiorni != nil {
			ritardo = fmt.Sprintf("%dg", *e.RitardoGiorni)
		}
		fmt.Fprintf(out, "%-15s %-14s %-9s %-7d %s\n",
			e.Archivio, valueOr(e.UltimoRecord, "—"), ritardo, e.Righe, nota)
	}
	return nil
}
