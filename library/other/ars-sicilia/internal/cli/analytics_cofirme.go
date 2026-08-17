// pp:client-call
// Novel feature — quante volte ciascun deputato ha COFIRMATO, in diretta.
//
// Da non confondere con `--group-by cofirmatari`, che è un'altra domanda: quello
// conta le COPPIE (quante volte due deputati firmano insieme) e per saperlo deve
// leggere l'elenco dei firmatari dentro ogni documento, cioè la deep sync. Qui la
// domanda è «quanto cofirma ciascuno», e non serve aprire niente.
//
// Il portale sa già rispondere, se glielo si chiede in ISIS. L'espressione l'ha
// insegnata il sito istituzionale (www.ars.sicilia.it, vedi
// docs/anagrafiche-www-ars.md), che dietro i propri contatori mette:
//
//	(18.LEGISL E ((Cracolici Antonino.FIRMAT) NOT (1 ADJ Cracolici Antonino).FIRMAT))
//
// cioè: compare fra i firmatari, ma non in prima posizione. Verificato contro i
// contatori pubblicati dal sito — Cracolici 302 cofirme e 7 da primo firmatario,
// Schifani 2 — e il conteggio della CLI dà gli stessi numeri.
//
// Costa una richiesta per deputato (icaro.Count legge il totale dalla pagina di
// apertura sessione), quindi ~66 per una legislatura: meno di `--group-by
// oratore`, che ne fa una per oratore su un backend più fragile.

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/store"
	"github.com/spf13/cobra"
)

// cofirmaExpr costruisce l'espressione ISIS «firma ma non è primo firmatario»
// per un deputato in una legislatura.
//
// `1 ADJ Nome.FIRMAT` è il primo firmatario (posizione 1); toglierlo dall'insieme
// di chi compare fra i firmatari lascia esattamente le cofirme.
func cofirmaExpr(legisl int, nome string) string {
	return fmt.Sprintf("(%d.LEGISL E ((%s.FIRMAT) NOT (1 ADJ %s).FIRMAT))", legisl, nome, nome)
}

// cofirmaNome tiene separate le due forme del nome: quella da mostrare in
// classifica e quella con cui il portale lo indicizza davvero.
type cofirmaNome struct {
	Display string // «D'Acquisto Mario», come lo scrive l'anagrafica
	Query   string // «D Acquisto Mario», come lo indicizza ISIS
}

// isisNome estrae dalla colonna isis_expr del seed la forma normalizzata del
// nome: «1 ADJ2   D Acquisto Mario.firmat» -> «D Acquisto Mario». Restituisce
// stringa vuota se l'espressione non ha quella forma.
var isisNomeRe = regexp.MustCompile(`(?i)^\s*1\s+ADJ\d*\s+(.*)\.firmat\s*$`)

func isisNome(expr string) string {
	m := isisNomeRe.FindStringSubmatch(expr)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// firmatariDiLegislatura legge dal seed locale i nomi dei firmatari nella forma
// esatta con cui il portale li indicizza per QUELLA legislatura.
//
// La forma conta: la scheda del sito istituzionale intesta «Cracolici Antonino
// detto Antonello», e il portale documentale indicizza *Antonino* — cercare il
// nome con cui lo chiama la stampa non trova niente. Il seed ha già le due cose
// che servono, nome e legislatura, per 1110 voci dalla X alla XVIII.
func firmatariDiLegislatura(ctx context.Context, dbPath string, legisl int) ([]cofirmaNome, error) {
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("apertura database (%s): %w", dbPath, err)
	}
	defer db.Close()
	if _, err := db.DB().ExecContext(ctx, createFirmatariTable); err != nil {
		return nil, fmt.Errorf("creazione tabella firmatari: %w", err)
	}
	if err := seedFirmatari(ctx, db.DB()); err != nil {
		return nil, fmt.Errorf("seed firmatari: %w", err)
	}
	rows, err := db.DB().QueryContext(ctx,
		`SELECT nome, isis_expr FROM firmatari WHERE legisl = ? ORDER BY nome`, itoa(legisl))
	if err != nil {
		return nil, fmt.Errorf("query firmatari: %w", err)
	}
	defer rows.Close()
	var out []cofirmaNome
	for rows.Next() {
		var n, expr string
		if err := rows.Scan(&n, &expr); err != nil {
			return nil, err
		}
		if n = strings.TrimSpace(n); n == "" {
			continue
		}
		// Il display non è cercabile: il portale indicizza senza accenti e con
		// la punteggiatura sciolta in spazi (Andò -> Ando, D'Acquisto ->
		// D Acquisto, F.sco -> F sco). Sono 49 nomi su 864: interrogarli nella
		// forma con cui li scrive l'anagrafica li farebbe risultare a zero
		// cofirme o non misurati. La forma buona è già nel seed.
		q := isisNome(expr)
		if q == "" {
			q = n
		}
		out = append(out, cofirmaNome{Display: n, Query: q})
	}
	return out, rows.Err()
}

// contatore è il solo pezzo di client che serve alla classifica: una richiesta
// per nome, e il totale letto dalla pagina di apertura sessione. Sta qui come
// interfaccia perché il ciclo qui sotto decide se il comando dice il vero o no
// — e una regola del genere va difesa da un test, non dalla buona volontà di
// chi la rilegge.
type contatore interface {
	Count(ctx context.Context, arc icaro.Archive, opts icaro.SearchOptions) (int, error)
}

// classificaCofirme interroga il portale una volta per nome e restituisce la
// classifica insieme all'elenco di chi non è stato misurato. Chi non risponde
// non vale zero: vale "non lo sappiamo", ed è il chiamante a doverlo dire.
func classificaCofirme(ctx context.Context, c contatore, arc icaro.Archive, legisl int, nomi []cofirmaNome, avanzamento func(fatti, totale int)) ([]analyticsRow, []string, error) {
	rows := make([]analyticsRow, 0, len(nomi))
	var persi []string
	for i, nome := range nomi {
		if avanzamento != nil {
			avanzamento(i+1, len(nomi))
		}
		n, err := c.Count(ctx, arc, icaro.SearchOptions{ISISRaw: cofirmaExpr(legisl, nome.Query)})
		if err != nil {
			if ctx.Err() != nil {
				return nil, persi, ctx.Err()
			}
			// Il 429 non è una richiesta persa fra le altre: è il portale che
			// chiede tregua. Archiviarlo in `persi` e proseguire spara le decine
			// di richieste rimaste contro un backend che ha già detto basta, e
			// perde il codice di uscita 7 su cui si regolano gli script.
			if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
				return nil, persi, rateLimitErr(fmt.Errorf("classifica cofirme: %w", err))
			}
			persi = append(persi, nome.Display)
			continue
		}
		if n > 0 {
			rows = append(rows, analyticsRow{Chiave: nome.Display, Conteggio: n})
		}
	}
	// Zero misurati non è una classifica vuota, è un comando fallito. Senza
	// questo ramo un consumatore JSON leggerebbe `[]` con exit 0 come «nessun
	// deputato ha cofirmato nulla»: la nota che avvisa va su stderr, e chi legge
	// solo stdout non la vede. Stessa regola di SpeakerSessionCounts.
	if len(rows) == 0 && len(persi) > 0 {
		return nil, persi, fmt.Errorf("classifica cofirme: nessuno dei %d deputati misurato, il portale non ha risposto", len(persi))
	}
	return rows, persi, nil
}

// runCofirmeAnalyticsLive costruisce la classifica delle cofirme interrogando il
// portale una volta per deputato.
func runCofirmeAnalyticsLive(cmd *cobra.Command, flags *rootFlags, typ string, legisl, limit int, dbPath string) error {
	arc := icaro.BySlug(typ)
	if arc == nil {
		return usageErr(fmt.Errorf("archivio sconosciuto %q", typ))
	}
	if _, ok := arc.FieldMap["firmatario"]; !ok {
		return usageErr(fmt.Errorf("--group-by cofirme vale solo sugli archivi che hanno un campo firmatario (ddl, interrogazioni, interpellanze, mozioni, odg, risoluzioni); ricevuto --type %q", typ))
	}
	if legisl <= 0 {
		return usageErr(fmt.Errorf("--group-by cofirme richiede --legisl: i nomi dei firmatari valgono per una legislatura, e senza si interrogherebbero tutti i deputati di sempre. Es: analytics --type ddl --group-by cofirme --legisl 18"))
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	nomi, err := firmatariDiLegislatura(ctx, dbPath, legisl)
	if err != nil {
		return err
	}
	if len(nomi) == 0 {
		return fmt.Errorf("nessun firmatario noto per la legislatura %d: l'anagrafica interna copre la X-XVIII (vedi `ars-sicilia-pp-cli ddl firmatari --legisl %d`)", legisl, legisl)
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}

	var avanzamento func(fatti, totale int)
	if !flags.asJSON {
		avanzamento = func(fatti, totale int) {
			fmt.Fprintf(os.Stderr, "\rclassifica cofirme %s (leg %d): %d/%d   ", typ, legisl, fatti, totale)
		}
	}
	rows, persi, err := classificaCofirme(ctx, c, *arc, legisl, nomi, avanzamento)
	if !flags.asJSON {
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return err
	}
	// Chi non è stato misurato va nominato, non contato zero: una classifica
	// muta su un buco si legge come «questo deputato cofirma poco», che è una
	// affermazione che non abbiamo fatto. Stessa regola di --group-by oratore.
	if len(persi) > 0 {
		fmt.Fprintf(os.Stderr, "nota: classifica parziale — %d deputati su %d non misurati (il portale non ha risposto): %s. Ripeti il comando per completarla.\n",
			len(persi), len(nomi), elencoTroncato(persi, 5))
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Conteggio > rows[j].Conteggio })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return emitAnalytics(cmd.OutOrStdout(), flags, rows)
}
