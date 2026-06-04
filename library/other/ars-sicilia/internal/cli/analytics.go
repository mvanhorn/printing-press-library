// pp:data-source local
// Novel feature — analytics su campi strutturati ARS: cofirmatari, oratori
// più attivi, distribuzioni per archivio. Tutto in locale via SQLite.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

  # I 30 oratori più attivi in aula
  ars-sicilia-pp-cli analytics --type resoconti --group-by oratore --limit 30`,
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
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "", "Campo di aggregazione (cofirmatari, oratore, anno).")
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
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("apertura database (%s): %w. Esegui prima `ars-sicilia-pp-cli sync --resources %s`.", dbPath, err, typ)
	}
	defer db.Close()

	out := cmd.OutOrStdout()

	switch groupBy {
	case "cofirmatari":
		rows, err := pairCofirmatari(ctx, db.DB(), typ, legisl, limit)
		if err != nil {
			return err
		}
		return emitAnalytics(out, flags, rows)
	case "oratore", "oratori":
		rows, err := groupOratori(ctx, db.DB(), legisl, limit)
		if err != nil {
			return err
		}
		return emitAnalytics(out, flags, rows)
	case "anno":
		rows, err := groupByAnno(ctx, db.DB(), typ, legisl, limit)
		if err != nil {
			return err
		}
		return emitAnalytics(out, flags, rows)
	default:
		return fmt.Errorf("group-by %q non supportato. Disponibili: cofirmatari, oratore, anno", groupBy)
	}
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

// groupOratori conta interventi per nome oratore nei resoconti d'aula.
// Per ora usa il campo libero `data->>'$.oratori'` (presente nei record sync).
func groupOratori(ctx context.Context, db *sql.DB, legisl, limit int) ([]analyticsRow, error) {
	whereLegisl := ""
	args := []any{}
	if legisl > 0 {
		whereLegisl = "AND json_extract(data, '$.legisl') = ? "
		args = append(args, fmt.Sprintf("%d", legisl))
	}
	q := `SELECT json_extract(data, '$.oratori') AS oratori
		FROM resources
		WHERE resource_type = 'resoconti'
		` + whereLegisl + `
		  AND json_extract(data, '$.oratori') IS NOT NULL
		  AND json_extract(data, '$.oratori') != ''`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query oratori: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		for _, n := range splitFirmatari(raw.String) {
			if n == "" {
				continue
			}
			counts[n]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lettura righe oratori: %w", err)
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

// groupByAnno conta documenti per anno in un archivio.
func groupByAnno(ctx context.Context, db *sql.DB, typ string, legisl, limit int) ([]analyticsRow, error) {
	whereLegisl := ""
	args := []any{typ}
	if legisl > 0 {
		whereLegisl = "AND json_extract(data, '$.legisl') = ? "
		args = append(args, fmt.Sprintf("%d", legisl))
	}
	q := `SELECT substr(json_extract(data, '$.data'), -4) AS anno, COUNT(*) AS n
		FROM resources
		WHERE resource_type = ?
		` + whereLegisl + `
		  AND substr(json_extract(data, '$.data'), -4) != ''
		GROUP BY anno ORDER BY n DESC LIMIT ` + fmt.Sprintf("%d", limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query anno: %w", err)
	}
	defer rows.Close()
	var result []analyticsRow
	for rows.Next() {
		var anno sql.NullString
		var n int
		if err := rows.Scan(&anno, &n); err != nil {
			continue
		}
		result = append(result, analyticsRow{Chiave: anno.String, Conteggio: n})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lettura righe anno: %w", err)
	}
	return result, nil
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
	// JSON default for non-TTY and explicit --json.
	asJSON := flags.asJSON
	if !asJSON {
		// Best-effort: emit table by default, JSON when stdout looks piped.
		asJSON = !isTerminal(w)
	}
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
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
