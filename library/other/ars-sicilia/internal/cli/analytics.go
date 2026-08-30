// pp:data-source local
// Novel feature — analytics su campi strutturati ARS: cofirmatari, oratori
// più attivi, distribuzioni per archivio. Tutto in locale via SQLite.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/store"
	"github.com/spf13/cobra"
)

func newNovelAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagType    string
		flagGroupBy string
		flagLimit   int
		flagLegisl  int
		flagDB      string
	)
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Aggregazioni locali sui dati ARS: cofirmatari di DDL, oratori più attivi in aula, ecc.",
		Long: `Esegue analisi sul database SQLite sincronizzato.

Esempi:
  # Le 50 coppie di deputati che firmano più DDL insieme
  ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 50

  # I 30 oratori più attivi in aula nella XVIII legislatura (in diretta, richiede --legisl)
  ars-sicilia-pp-cli analytics --type resoconti --group-by oratore --legisl 18 --limit 30

  # Classifica DDL per deputato proponente / per gruppo (1 richiesta, legislatura corrente)
  ars-sicilia-pp-cli analytics --type ddl --group-by proponente --limit 20
  ars-sicilia-pp-cli analytics --type ddl --group-by gruppo`,
		Example: "  ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 50 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runAnalytics(cmd, flags, flagType, flagGroupBy, flagLimit, flagLegisl, flagDB)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Archivio sorgente (ddl, interrogazioni, mozioni, resoconti).")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "", "Campo di aggregazione (cofirme, cofirmatari, oratore, proponente, gruppo, anno). cofirme = quante volte ciascun deputato ha cofirmato (in diretta, richiede --legisl); cofirmatari = coppie che firmano insieme (richiede sync --deep).")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max righe in output.")
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Filtra per legislatura (0 = tutte).")
	cmd.Flags().StringVar(&flagDB, "db", "", "Percorso del database SQLite.")
	return cmd
}

type analyticsRow struct {
	Chiave    string `json:"chiave"`
	Conteggio int    `json:"conteggio"`
	Note      string `json:"note,omitempty"`
}

func runAnalytics(cmd *cobra.Command, flags *rootFlags, typ, groupBy string, limit, legisl int, dbPath string) error {
	if typ == "" || groupBy == "" {
		return fmt.Errorf("--type e --group-by sono richiesti (es. --type ddl --group-by cofirmatari)")
	}
	if dbPath == "" {
		dbPath = defaultDBPath("ars-sicilia-pp-cli")
	}
	if limit <= 0 {
		limit = 30
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// group-by oratore: percorso LIVE su /bd/resoconti. L'anagrafica oratori non è
	// nello store locale (né lo era su Icaro); la classifica si costruisce contando
	// le sedute per ciascun oratore della legislatura direttamente dal portale.
	if groupBy == "oratore" || groupBy == "oratori" {
		if typ != "resoconti" {
			return fmt.Errorf("--group-by oratore vale solo con --type resoconti (la classifica è sugli oratori delle sedute d'Aula); ricevuto --type %q", typ)
		}
		return runOratoreAnalyticsLive(cmd, flags, legisl, limit)
	}

	// group-by cofirme: percorso LIVE, una richiesta per deputato. Non è un
	// doppione di `cofirmatari`: quello conta le coppie e per farlo deve leggere
	// i firmatari dentro ogni documento (deep sync); questo conta quante volte
	// ciascuno ha cofirmato, e il portale lo sa già dire (vedi analytics_cofirme.go).
	if groupBy == "cofirme" {
		return runCofirmeAnalyticsLive(cmd, flags, typ, legisl, limit, dbPath)
	}

	// group-by proponente/gruppo: percorso LIVE sulle viste pre-aggregate /edem/
	// (una sola richiesta). Sono classifiche dei DDL già calcolate dal portale
	// (proponente = primo firmatario; gruppo = gruppo parlamentare).
	if groupBy == "proponente" || groupBy == "gruppo" {
		if typ != "ddl" {
			return fmt.Errorf("--group-by %s vale solo con --type ddl (classifica dei disegni di legge); ricevuto --type %q", groupBy, typ)
		}
		return runDDLRankingLive(cmd, flags, groupBy, legisl, limit)
	}

	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("apertura database (%s): %w. Esegui prima `ars-sicilia-pp-cli sync --resources %s`.", dbPath, err, typ)
	}
	defer db.Close()

	out := cmd.OutOrStdout()

	var rows []analyticsRow
	switch groupBy {
	case "cofirmatari":
		rows, err = pairCofirmatari(ctx, db.DB(), typ, legisl, limit)
	case "anno":
		rows, err = groupByAnno(ctx, db.DB(), typ, legisl, limit)
	default:
		return fmt.Errorf("group-by %q non supportato. Disponibili: cofirme, cofirmatari, oratore, proponente, gruppo, anno", groupBy)
	}
	if err != nil {
		return err
	}
	// Empty result: hint on stderr (keeps JSON/CSV on stdout clean). Be honest
	// about *why* it is empty e se un sync può aiutare.
	if len(rows) == 0 {
		switch groupBy {
		case "cofirmatari":
			// The firmatari are not in the short-list; only the deep sync
			// extracts them from each ddl's detail page.
			fmt.Fprintf(os.Stderr,
				"hint: --group-by cofirmatari conta le COPPIE di cofirmatari, e le coppie stanno solo dentro le schede di dettaglio: esegui `ars-sicilia-pp-cli sync --resources ddl --deep` (più lento) e riprova, una sync normale non le popola. Se invece ti basta quante volte ciascuno ha cofirmato, `--group-by cofirme --legisl N` lo chiede al portale in diretta, senza sync.\n")
		default:
			// Distinguere "store vuoto" da "store popolato ma senza record per
			// questa legislatura": la sync di default scarica le prime pagine
			// (ordinate per recenza), quindi su una legislatura passata trova
			// zero record pur essendo fresca. Suggerire un semplice `sync` in
			// quel caso manda l'utente a rifare esattamente la sync che ha già.
			if n := countResources(ctx, db.DB(), typ); n > 0 && legisl > 0 {
				fmt.Fprintf(os.Stderr,
					"hint: nessun dato per --type %s --group-by %s con --legisl %d. Lo store ha %d record di tipo %s, ma nessuno di quella legislatura: la sync di default scarica solo le prime pagine (le più recenti). Esegui `ars-sicilia-pp-cli sync --resources %s --legisl %d --full` e riprova.\n",
					typ, groupBy, legisl, n, typ, typ, legisl)
			} else {
				fmt.Fprintf(os.Stderr,
					"hint: nessun dato per --type %s --group-by %s. Lo store locale potrebbe non essere sincronizzato: esegui `ars-sicilia-pp-cli sync --resources %s` e riprova.\n",
					typ, groupBy, typ)
			}
		}
	}
	return emitAnalytics(out, flags, rows)
}

// runOratoreAnalyticsLive costruisce la classifica degli oratori per numero di
// sedute d'Aula, interrogando /bd/resoconti in diretta (l'anagrafica oratori non
// è nello store). Richiede --legisl per limitare gli oratori a quelli attivi nella
// legislatura (~90), altrimenti sarebbero ~1000 = troppe richieste.
func runOratoreAnalyticsLive(cmd *cobra.Command, flags *rootFlags, legisl, limit int) error {
	if legisl <= 0 {
		return fmt.Errorf("--group-by oratore richiede --legisl (senza, gli oratori sarebbero circa mille: troppe richieste). Es: analytics --type resoconti --group-by oratore --legisl 18")
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	// Progress su stderr (una richiesta per oratore): tenuto fuori dallo stdout
	// così JSON/CSV restano puliti.
	progress := func(done, total int) {
		if flags.asJSON {
			return
		}
		fmt.Fprintf(os.Stderr, "\rclassifica oratori (leg %d): %d/%d   ", legisl, done, total)
		if done == total {
			fmt.Fprintln(os.Stderr)
		}
	}
	counts, persi, err := c.SpeakerSessionCounts(ctx, itoa(legisl), "", progress)
	if err != nil {
		// Il rate limit ha un codice suo: chi ha uno script deve poter capire che
		// si tratta di aspettare e riprovare, non di un guasto.
		if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
			return rateLimitErr(err)
		}
		return err
	}
	// Sempre su stderr, come il caveat su --legisl qui sopra: lo stdout resta
	// JSON/CSV puro. Una classifica a cui manca qualcuno non può però uscire
	// muta: chi la legge la userebbe per dire «è intervenuto meno degli altri»,
	// quando in realtà non l'abbiamo misurato.
	if len(persi) > 0 {
		soggetto := fmt.Sprintf("%d oratori su %d non misurati", len(persi), len(persi)+len(counts))
		if len(persi) == 1 {
			soggetto = fmt.Sprintf("1 oratore su %d non misurato", len(counts)+1)
		}
		fmt.Fprintf(os.Stderr, "nota: classifica parziale — %s (il backend /bd/ non ha risposto): %s. Ripeti il comando per completarla.\n",
			soggetto, elencoTroncato(persi, 5))
	}
	rows := make([]analyticsRow, 0, len(counts))
	for _, sc := range counts {
		if sc.Count == 0 {
			continue // oratore senza sedute nella legislatura
		}
		rows = append(rows, analyticsRow{Chiave: sc.Name, Conteggio: sc.Count})
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return emitAnalytics(cmd.OutOrStdout(), flags, rows)
}

// runDDLRankingLive costruisce la classifica dei DDL per proponente (canale 6) o
// per gruppo (canale 7) leggendo le viste pre-aggregate /edem/ in diretta: una
// sola richiesta, il portale ha già calcolato i conteggi (primo firmatario).
//
// Le viste /edem/ NON sono parametrizzabili per legislatura: aggregano solo la
// legislatura corrente. Per questo --legisl non viene validato contro un numero
// (sarebbe una bomba a orologeria alla prossima legislatura); se passato, si
// avvisa su stderr che il flag non filtra questa classifica.
func runDDLRankingLive(cmd *cobra.Command, flags *rootFlags, groupBy string, legisl, limit int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	channel := icaro.EdemChannelProponente
	if groupBy == "gruppo" {
		channel = icaro.EdemChannelGruppo
	}
	// Caveat di correttezza (non rumore di progress): sempre su stderr, così non
	// inquina lo stdout JSON/CSV ma avvisa che --legisl è stato ignorato.
	if legisl > 0 {
		fmt.Fprintf(os.Stderr,
			"nota: --group-by %s copre solo la legislatura corrente (le classifiche /edem/ non sono filtrabili per legislatura); --legisl %d ignorato.\n",
			groupBy, legisl)
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	items, err := c.DDLRanking(ctx, channel)
	if err != nil {
		return err
	}
	rows := make([]analyticsRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, analyticsRow{Chiave: it.Name, Conteggio: it.Count})
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return emitAnalytics(cmd.OutOrStdout(), flags, rows)
}

// pairCofirmatari estrae le coppie di firmatari in un archivio (default: ddl)
// raggruppando per pair e contando.
func pairCofirmatari(ctx context.Context, db *sql.DB, typ string, legisl, limit int) ([]analyticsRow, error) {
	whereLegisl := ""
	args := []any{typ}
	if legisl > 0 {
		whereLegisl = "AND json_extract(data, '$.legisl') = ? "
		args = append(args, fmt.Sprintf("%d", legisl))
	}
	q := `SELECT json_extract(data, '$.firmatari') AS firmat
		FROM resources
		WHERE resource_type = ?
		` + whereLegisl + `
		  AND json_extract(data, '$.firmatari') IS NOT NULL
		  AND json_extract(data, '$.firmatari') != ''`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query cofirmatari: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		names := splitFirmatari(raw.String)
		// coppie ordinate
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				a, b := names[i], names[j]
				if a > b {
					a, b = b, a
				}
				if a == "" || b == "" {
					continue
				}
				counts[a+" ↔ "+b]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lettura righe: %w", err)
	}
	result := make([]analyticsRow, 0, len(counts))
	for k, v := range counts {
		result = append(result, analyticsRow{Chiave: k, Conteggio: v})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Conteggio > result[j].Conteggio })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// (--group-by oratore ora è gestito da runOratoreAnalyticsLive via /bd/resoconti;
// la vecchia groupOratori che leggeva `$.oratori` dallo store — campo mai popolato
// — è stata rimossa.)

// groupByAnno conta documenti per anno in un archivio. L'anno va estratto
// con iterDateKey (non un substr SQL a larghezza fissa): le date nello store
// hanno larghezza variabile — "D.M.YY" per ddl, "DD.MM.YYYY" per leggi — e un
// substr(-4) mescola mese e anno sulle date a una cifra (es. "5.3.26" ->
// "3.26", non l'anno).
// countResources conta i record di un tipo nello store, senza filtro di
// legislatura. Serve solo a scegliere il messaggio di hint su risultato vuoto:
// un errore qui non deve rompere il comando, quindi torna 0 e tace.
func countResources(ctx context.Context, db *sql.DB, typ string) int {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, typ).Scan(&n); err != nil {
		return 0
	}
	return n
}

func groupByAnno(ctx context.Context, db *sql.DB, typ string, legisl, limit int) ([]analyticsRow, error) {
	whereLegisl := ""
	args := []any{typ}
	if legisl > 0 {
		whereLegisl = "AND json_extract(data, '$.legisl') = ? "
		args = append(args, fmt.Sprintf("%d", legisl))
	}
	q := `SELECT json_extract(data, '$.data') AS data
		FROM resources
		WHERE resource_type = ?
		` + whereLegisl + `
		  AND json_extract(data, '$.data') IS NOT NULL
		  AND json_extract(data, '$.data') != ''`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query anno: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if y := yearOf(raw.String); y != "" {
			counts[y]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lettura righe anno: %w", err)
	}
	result := make([]analyticsRow, 0, len(counts))
	for k, v := range counts {
		result = append(result, analyticsRow{Chiave: k, Conteggio: v})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Conteggio > result[j].Conteggio })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// yearOf extracts the 4-digit year from an ICaro date via the shared date
// parsers (iterDateKey handles both the "D.M.YY"/"DD.MM.YYYY" short-list
// form and the "DD mese YYYY" document-body form), returning "" when the
// date doesn't parse to a sortable "YYYY-MM-DD" key.
func yearOf(dateStr string) string {
	s := strings.TrimSpace(dateStr)
	// Formato del backend /bd/: DD/MM/YYYY (es. 21/07/2026).
	if len(s) == 10 && s[2] == '/' && s[5] == '/' && isDigits(s[6:10]) {
		return s[6:10]
	}
	key := iterDateKey(dateStr)
	if len(key) >= 5 && key[4] == '-' {
		return key[:4]
	}
	return ""
}

// splitFirmatari divide una stringa di firmatari sui separatori comuni
// usati dal portale (virgola, ";", " e ").
func splitFirmatari(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, sep := range []string{";", " - ", " - ", " E ", " e ", " ed "} {
		s = strings.ReplaceAll(s, sep, ",")
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func emitAnalytics(w interface{ Write(p []byte) (int, error) }, flags *rootFlags, rows []analyticsRow) error {
	// JSON default for non-TTY and explicit --json. --csv wins over the
	// piped-JSON fallback, same precedence as emitRecords for */cerca.
	asJSON := flags.asJSON
	if !asJSON {
		// Best-effort: emit table by default, JSON when stdout looks piped.
		asJSON = !isTerminal(w) && !flags.csv
	}
	if asJSON {
		return printJSONFiltered(w, rows, flags)
	}
	if flags.csv {
		return writeAnalyticsCSV(w, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "Nessun dato. Esegui prima `ars-sicilia-pp-cli sync`.")
		return nil
	}
	for _, r := range rows {
		fmt.Fprintf(w, "%6d   %s\n", r.Conteggio, r.Chiave)
	}
	return nil
}

func writeAnalyticsCSV(w interface{ Write(p []byte) (int, error) }, rows []analyticsRow) error {
	fmt.Fprintln(w, "chiave,conteggio,note")
	for _, r := range rows {
		fmt.Fprintf(w, "%s,%d,%s\n", csvEscape(r.Chiave), r.Conteggio, csvEscape(r.Note))
	}
	return nil
}

// elencoTroncato rende leggibile una lista di nomi in un avviso di una riga:
// oltre `max` nomi si dice quanti ne restano invece di riversarli tutti nel
// terminale. Novantuno nomi in un `nota:` non li legge nessuno.
func elencoTroncato(nomi []string, max int) string {
	if len(nomi) <= max {
		return strings.Join(nomi, ", ")
	}
	return strings.Join(nomi[:max], ", ") + fmt.Sprintf(" e altri %d", len(nomi)-max)
}
