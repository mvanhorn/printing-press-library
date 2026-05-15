// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

// newVoicesFindCmd registers `voices find "<description>"`. Uses voices_fts
// when available (FTS5 prefix MATCH); otherwise falls back to LIKE on
// name/description.
func newVoicesFindCmd(flags *rootFlags) *cobra.Command {
	var lang string
	var gender string
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "find [description]",
		Short:       "Search the local voices catalog by free-text description and filters",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  cartesia-pp-cli voices find "warm female" --lang en --gender f --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			desc := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			useFTS := !looksLikeRegex(desc) && hasFTSTable(db, "voices_fts")
			var q string
			var argv []any
			if useFTS {
				q = `SELECT v.id, COALESCE(v.name,''), COALESCE(v.description,''), COALESCE(v.language,''), COALESCE(v.gender,''), v.data
                     FROM voices_fts f JOIN voices v ON v.rowid = f.rowid
                     WHERE voices_fts MATCH ?`
				// FTS5 MATCH treats bare apostrophes and multi-word
				// patterns as syntax tokens; wrap in double quotes (and
				// escape embedded ones) so the description is treated
				// as a literal phrase.
				ftsPattern := `"` + strings.ReplaceAll(desc, `"`, `""`) + `"`
				argv = append(argv, ftsPattern)
			} else {
				// PATCH(greptile #560): same LIKE-escape contract as
				// calls_grep — bare % and _ in user input must not act as
				// wildcards. Escape backslash, %, _, then wrap with % for
				// substring match, and add ESCAPE '\\' to both predicates.
				q = `SELECT id, COALESCE(name,''), COALESCE(description,''), COALESCE(language,''), COALESCE(gender,''), data
                     FROM voices
                     WHERE (LOWER(COALESCE(name,'')) LIKE ? ESCAPE '\' OR LOWER(COALESCE(description,'')) LIKE ? ESCAPE '\')`
				escaped := strings.NewReplacer(
					`\`, `\\`,
					`%`, `\%`,
					`_`, `\_`,
				).Replace(strings.ToLower(desc))
				p := "%" + escaped + "%"
				argv = append(argv, p, p)
			}
			if lang != "" {
				if useFTS {
					q += " AND v.language = ?"
				} else {
					q += " AND language = ?"
				}
				argv = append(argv, lang)
			}
			if gender != "" {
				if useFTS {
					q += " AND v.gender = ?"
				} else {
					q += " AND gender = ?"
				}
				argv = append(argv, gender)
			}
			if limit <= 0 {
				limit = 25
			}
			q += " LIMIT ?"
			argv = append(argv, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query voices: %w", err)
			}
			defer rows.Close()

			type hit struct {
				ID          string          `json:"id"`
				Name        string          `json:"name,omitempty"`
				Description string          `json:"description,omitempty"`
				Language    string          `json:"language,omitempty"`
				Gender      string          `json:"gender,omitempty"`
				Data        json.RawMessage `json:"data,omitempty"`
			}
			results := []hit{}
			for rows.Next() {
				var h hit
				var data string
				if err := rows.Scan(&h.ID, &h.Name, &h.Description, &h.Language, &h.Gender, &data); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				if data != "" {
					h.Data = json.RawMessage(data)
				}
				results = append(results, h)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("rows: %w", err)
			}
			return flags.printJSON(cmd, results)
		},
	}

	cmd.Flags().StringVar(&lang, "lang", "", "Filter by language code")
	cmd.Flags().StringVar(&gender, "gender", "", "Filter by gender (m, f, or whatever the catalog uses)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum voices to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// hasFTSTable reports whether the named FTS table exists in the store.
func hasFTSTable(db *store.Store, name string) bool {
	var n string
	err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type IN ('table','virtual') AND name = ?`, name).Scan(&n)
	return err == nil
}
