// pp:data-source local
// Novel feature — drift dell'iter dei DDL: confronta lo stato corrente con
// la sync precedente e segnala i DDL che si sono mossi.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/store"
	"github.com/spf13/cobra"
)

func newNovelDdlDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince string
		flagDB    string
	)
	cmd := &cobra.Command{
		Use:     "drift",
		Short:   "Confronta lo stato dell'iter dei DDL nella sync corrente con quella precedente: segnala i DDL che si sono mossi.",
		Example: "  ars-sicilia-pp-cli ddl drift --since 7d --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runDdlDrift(cmd, flags, flagSince, flagDB)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Finestra temporale del confronto (es. 24h, 7d, 30d).")
	cmd.Flags().StringVar(&flagDB, "db", "", "Percorso del database SQLite (default: ~/.local/share/ars-sicilia-pp-cli/store.db).")
	return cmd
}

type driftItem struct {
	Numero string `json:"numero"`
	Legisl string `json:"legisl,omitempty"`
	Titolo string `json:"titolo,omitempty"`
	IterDa string `json:"iter_da,omitempty"`
	IterA  string `json:"iter_a,omitempty"`
	DataDa string `json:"data_da,omitempty"`
	DataA  string `json:"data_a,omitempty"`
	URL    string `json:"url,omitempty"`
}

type driftReport struct {
	Window      string      `json:"window"`
	GeneratedAt string      `json:"generated_at"`
	Mossi       []driftItem `json:"mossi"`
	Nuovi       []driftItem `json:"nuovi"`
	Note        string      `json:"note,omitempty"`
}

func runDdlDrift(cmd *cobra.Command, flags *rootFlags, since, dbPath string) error {
	if dbPath == "" {
		dbPath = defaultDBPath("ars-sicilia-pp-cli")
	}
	window := parseDurationLoose(since, 7*24*time.Hour)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	report := driftReport{
		Window:      since,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		report.Note = "Nessun database locale. Esegui `ars-sicilia-pp-cli sync --resources ddl` due volte (a distanza di tempo) per generare snapshot da confrontare."
		return emitDriftReport(cmd, flags, report)
	}
	defer db.Close()

	// Strategy: confronta i record "ddl" attualmente in `resources` con la
	// loro precedente versione conservata in `resources_history` (se presente).
	// Se la tabella di history non c'è, ripiega su differenze rispetto
	// all'ultima sync_state.
	cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339)
	// Tentativo con tabella resources_history (potrebbe non esistere).
	rows, err := db.DB().QueryContext(ctx, `
		SELECT cur.id, json_extract(cur.data, '$.legisl'),
		       json_extract(cur.data, '$.title'),
		       json_extract(cur.data, '$.iter'),
		       json_extract(prev.data, '$.iter'),
		       json_extract(cur.data, '$.data'),
		       json_extract(prev.data, '$.data'),
		       json_extract(cur.data, '$.url')
		FROM resources cur
		LEFT JOIN resources_history prev
		   ON prev.id = (
		       SELECT id FROM resources_history rh
		       WHERE rh.resource_id   = cur.id
		         AND rh.resource_type = cur.resource_type
		         AND rh.captured_at   < cur.synced_at
		       ORDER BY rh.captured_at DESC
		       LIMIT 1
		   )
		WHERE cur.resource_type = 'ddl'
		  AND cur.synced_at >= ?
		  AND json_extract(cur.data, '$.iter') IS NOT NULL
		  AND (prev.data IS NULL OR json_extract(prev.data, '$.iter') IS NOT json_extract(cur.data, '$.iter'))
	`, cutoff)
	if err != nil {
		report.Note = "Tabelle di history non disponibili in questo store (richiede schema esteso). Verifica `ars-sicilia-pp-cli doctor` o esegui due sync ravvicinate."
		return emitDriftReport(cmd, flags, report)
	}
	defer rows.Close()

	for rows.Next() {
		var numero, legisl, titolo, iter, iterPrev, data, dataPrev, url sql.NullString
		if err := rows.Scan(&numero, &legisl, &titolo, &iter, &iterPrev, &data, &dataPrev, &url); err != nil {
			continue
		}
		it := driftItem{
			Numero: numero.String,
			Legisl: legisl.String,
			Titolo: titolo.String,
			IterA:  iter.String,
			IterDa: iterPrev.String,
			DataA:  data.String,
			DataDa: dataPrev.String,
			URL:    url.String,
		}
		if iterPrev.Valid {
			report.Mossi = append(report.Mossi, it)
		} else {
			report.Nuovi = append(report.Nuovi, it)
		}
	}
	sort.SliceStable(report.Mossi, func(i, j int) bool {
		return parseICaroDate(report.Mossi[i].DataA) > parseICaroDate(report.Mossi[j].DataA)
	})
	return emitDriftReport(cmd, flags, report)
}

func emitDriftReport(cmd *cobra.Command, flags *rootFlags, r driftReport) error {
	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	fmt.Fprintf(out, "Window: %s   Snapshot: %s\n\n", r.Window, r.GeneratedAt)
	if r.Note != "" {
		fmt.Fprintf(out, "%s\n", r.Note)
		return nil
	}
	fmt.Fprintf(out, "Mossi (%d):\n", len(r.Mossi))
	for _, it := range r.Mossi {
		fmt.Fprintf(out, "  DDL %s  %s -> %s  (%s)\n", it.Numero, it.IterDa, it.IterA, it.Titolo)
	}
	fmt.Fprintf(out, "\nNuovi (%d):\n", len(r.Nuovi))
	for _, it := range r.Nuovi {
		fmt.Fprintf(out, "  DDL %s  (%s)\n", it.Numero, it.Titolo)
	}
	return nil
}
