// Copyright 2026 abhirup-dev and contributors. Licensed under Apache-2.0.
// Hand-authored novel command: since.
// pp:data-source local
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/moneycontrol/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/moneycontrol/internal/store"
)

// newNovelSinceCmd builds `since <duration>`: articles in the local SQLite
// mirror synced within the last N hours, optionally filtered by a stock symbol
// (matched against the article URL and title). This reads the local store only;
// run `sync --resources articles` first.
//
// Note on time semantics: moneycontrol news listing pages do not expose reliable
// publish timestamps in the extracted link payload, so "since" is keyed on the
// store's updated_at (when the article was last synced), not on upstream
// publish time. This answers "what did I just pull in" rather than "what was
// published at exactly this hour" — which is the honest, useful framing for a
// portfolio-CLI news ingestion loop.
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var symbol string
	var limit int
	var durationFlag string

	cmd := &cobra.Command{
		Use:   "since",
		Short: "Articles synced locally within the last N hours, optional symbol filter.",
		Long: "Articles synced locally within the last N hours, optional symbol filter.\n\n" +
			"Duration accepts Go syntax plus d/w day/week shorthand (e.g. 6h, 2d, 1w).\n" +
			"Reads the local SQLite mirror; run `sync --resources articles` first.\n" +
			"--symbol matches the article URL or title (e.g. RI, RELIANCE, reliance).",
		Example: `  moneycontrol-pp-cli since --duration 2h
  moneycontrol-pp-cli since --duration 6h --symbol RI --json
  moneycontrol-pp-cli since --duration 1d --db ~/mc.db`,
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--duration=6h",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "since")
			}
			durSrc := durationFlag
			if durSrc == "" && len(args) >= 1 {
				durSrc = args[0]
			}
			if durSrc == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("duration is required (pass --duration or a positional, e.g. 6h, 2d, 1w)"))
			}
			dur, err := cliutil.ParseDurationLoose(durSrc)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("moneycontrol-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: moneycontrol-pp-cli sync --resources articles --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]articleLink, 0), flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `
				SELECT id, data, updated_at FROM resources
				WHERE resource_type = 'articles'
				  AND updated_at >= datetime('now', ?)
				ORDER BY updated_at DESC`, fmt.Sprintf("-%d seconds", int(dur.Seconds())))
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			type storedRow struct {
				ID     string
				Title  string
				URL    string
				Data   json.RawMessage
			}
			rawRows := make([]storedRow, 0)
			for rows.Next() {
				var id sql.NullString
				var data json.RawMessage
				var updatedAt sql.NullString
				if err := rows.Scan(&id, &data, &updatedAt); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan row: %w", err)
				}
				r := storedRow{ID: id.String, Data: data}
				// The extracted-link payload is {href,text}; fall back to id.
				var pl struct {
					Href string `json:"href"`
					Text string `json:"text"`
				}
				_ = json.Unmarshal(data, &pl)
				r.URL = pl.Href
				if r.URL == "" {
					r.URL = id.String
				}
				r.Title = pl.Text
				rawRows = append(rawRows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate rows: %w", err)
			}
			_ = rows.Close()

			out := make([]articleLink, 0, len(rawRows))
			needle := ""
			if symbol != "" {
				needle = strings.ToUpper(strings.TrimSpace(symbol))
			}
			for _, r := range rawRows {
				if needle != "" {
					hay := strings.ToUpper(r.URL + " " + r.Title)
					if !strings.Contains(hay, needle) {
						continue
					}
				}
				out = append(out, articleLink{URL: r.URL, Title: r.Title})
				if limit > 0 && len(out) >= limit {
					break
				}
			}

			view := struct {
				Duration string        `json:"duration"`
				Symbol   string        `json:"symbol,omitempty"`
				Count    int           `json:"count"`
				Articles []articleLink `json:"articles"`
			}{Duration: dur.String(), Symbol: symbol, Count: len(out), Articles: out}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out2 := cmd.OutOrStdout()
			label := dur.String()
			if symbol != "" {
				label += " symbol=" + symbol
			}
			fmt.Fprintf(out2, "SINCE %s (%d)\n", label, len(out))
			for i, h := range out {
				fmt.Fprintf(out2, "  %d. %s\n     %s\n", i+1, h.Title, h.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&durationFlag, "duration", "", "time window (e.g. 6h, 2d, 1w); alternatively pass as positional")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local store)")
	cmd.Flags().StringVar(&symbol, "symbol", "", "stock symbol/name to filter on (matched against URL and title)")
	cmd.Flags().IntVar(&limit, "limit", 50, "max articles to return")
	return cmd
}
