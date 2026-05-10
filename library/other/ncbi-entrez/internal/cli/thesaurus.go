package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// thesaurusEntry represents a stored spelling correction mapping.
type thesaurusEntry struct {
	Original  string `json:"original"`
	Corrected string `json:"corrected"`
	Status    string `json:"status"`
	DB        string `json:"db"`
	DecidedAt string `json:"decided_at"`
}

func ensureThesaurusTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS thesaurus_entries (
			original TEXT NOT NULL,
			corrected TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			db TEXT NOT NULL DEFAULT 'pubmed',
			decided_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (original, corrected)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_thesaurus_status ON thesaurus_entries(status)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating thesaurus tables: %w", err)
		}
	}
	return nil
}

func newThesaurusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thesaurus",
		Short: "ESpell thesaurus with audit trail for query corrections",
		Long: strings.TrimSpace(`
ESpell Thesaurus with Audit Trail -- check spelling of queries via ESpell,
accept or reject suggestions, and apply stored mappings to transform queries.

Use 'thesaurus check' to spell-check a query, 'thesaurus accept/reject'
to record decisions, and 'thesaurus apply' to transform queries.`),
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli thesaurus check "brest cancr" --db pubmed
  ncbi-entrez-pp-cli thesaurus accept "brest cancr" "breast cancer"
  ncbi-entrez-pp-cli thesaurus reject "brest cancr" "breast cancer"
  ncbi-entrez-pp-cli thesaurus list
  ncbi-entrez-pp-cli thesaurus apply "my brest cancr study"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newThesaurusCheckCmd(flags))
	cmd.AddCommand(newThesaurusAcceptCmd(flags))
	cmd.AddCommand(newThesaurusRejectCmd(flags))
	cmd.AddCommand(newThesaurusListCmd(flags))
	cmd.AddCommand(newThesaurusApplyCmd(flags))

	return cmd
}

func newThesaurusCheckCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "check <query>",
		Short: "Check query spelling via ESpell",
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli thesaurus check "brest cancr" --db pubmed
  ncbi-entrez-pp-cli thesaurus check "astma treatment"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("query required as argument")
			}
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Call ESpell
			params := map[string]string{
				"db":      flagDB,
				"term":    query,
				"retmode": "json",
			}
			data, err := c.Get("/espell.fcgi", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse ESpell response
			corrected := parseESpellSuggestion(data)

			result := map[string]any{
				"original": query,
				"db":       flagDB,
			}

			if corrected != "" && corrected != query {
				result["corrected"] = corrected
				result["has_suggestion"] = true

				// Store as pending in the thesaurus
				dbPath := defaultDBPath("ncbi-entrez-pp-cli")
				db, dbErr := store.Open(dbPath)
				if dbErr == nil {
					defer db.Close()
					if ensureThesaurusTables(db) == nil {
						db.DB().Exec(
							`INSERT OR IGNORE INTO thesaurus_entries (original, corrected, status, db, decided_at) VALUES (?, ?, 'pending', ?, CURRENT_TIMESTAMP)`,
							query, corrected, flagDB,
						)
					}
				}
			} else {
				result["has_suggestion"] = false
				result["message"] = "No spelling corrections suggested"
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagDB, "db", "pubmed", "Target database for ESpell")

	return cmd
}

// parseESpellSuggestion extracts the corrected query from an ESpell JSON response.
func parseESpellSuggestion(data json.RawMessage) string {
	var resp struct {
		ESpellResult struct {
			CorrectedQuery string `json:"CorrectedQuery"`
		} `json:"espellresult"`
	}
	if json.Unmarshal(data, &resp) == nil && resp.ESpellResult.CorrectedQuery != "" {
		return resp.ESpellResult.CorrectedQuery
	}

	// Fallback
	var alt struct {
		CorrectedQuery string `json:"corrected_query"`
	}
	if json.Unmarshal(data, &alt) == nil && alt.CorrectedQuery != "" {
		return alt.CorrectedQuery
	}

	return ""
}

func newThesaurusAcceptCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "accept <original> <corrected>",
		Short:   "Accept a spelling correction mapping",
		Example: `  ncbi-entrez-pp-cli thesaurus accept "brest cancr" "breast cancer"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 && !flags.dryRun {
				return fmt.Errorf("usage: thesaurus accept <original> <corrected>")
			}
			if dryRunOK(flags) {
				return nil
			}

			original := args[0]
			corrected := args[1]

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureThesaurusTables(db); err != nil {
				return err
			}

			_, err = db.DB().Exec(
				`INSERT INTO thesaurus_entries (original, corrected, status, db, decided_at) VALUES (?, ?, 'accepted', '', CURRENT_TIMESTAMP)
				 ON CONFLICT(original, corrected) DO UPDATE SET status = 'accepted', decided_at = CURRENT_TIMESTAMP`,
				original, corrected,
			)
			if err != nil {
				return fmt.Errorf("accepting mapping: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":    "accepted",
				"original":  original,
				"corrected": corrected,
			}, flags)
		},
	}
}

func newThesaurusRejectCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "reject <original> <corrected>",
		Short:   "Reject a spelling correction mapping",
		Example: `  ncbi-entrez-pp-cli thesaurus reject "brest cancr" "breast cancer"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 && !flags.dryRun {
				return fmt.Errorf("usage: thesaurus reject <original> <corrected>")
			}
			if dryRunOK(flags) {
				return nil
			}

			original := args[0]
			corrected := args[1]

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureThesaurusTables(db); err != nil {
				return err
			}

			_, err = db.DB().Exec(
				`INSERT INTO thesaurus_entries (original, corrected, status, db, decided_at) VALUES (?, ?, 'rejected', '', CURRENT_TIMESTAMP)
				 ON CONFLICT(original, corrected) DO UPDATE SET status = 'rejected', decided_at = CURRENT_TIMESTAMP`,
				original, corrected,
			)
			if err != nil {
				return fmt.Errorf("rejecting mapping: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":    "rejected",
				"original":  original,
				"corrected": corrected,
			}, flags)
		},
	}
}

func newThesaurusListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List all stored spelling correction mappings with their accept/reject status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # List all thesaurus entries
  ncbi-entrez-pp-cli thesaurus list --json

  # Agent-friendly list
  ncbi-entrez-pp-cli thesaurus list --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureThesaurusTables(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT original, corrected, status, db, decided_at FROM thesaurus_entries ORDER BY decided_at DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var entries []thesaurusEntry
			for rows.Next() {
				var e thesaurusEntry
				if err := rows.Scan(&e.Original, &e.Corrected, &e.Status, &e.DB, &e.DecidedAt); err != nil {
					return err
				}
				entries = append(entries, e)
			}
			if entries == nil {
				entries = []thesaurusEntry{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}
}

func newThesaurusApplyCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "apply <query>",
		Short:       "Apply accepted corrections to a query",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  ncbi-entrez-pp-cli thesaurus apply "my brest cancr study"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("query required as argument")
			}
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureThesaurusTables(db); err != nil {
				return err
			}

			// Load accepted mappings
			rows, err := db.DB().Query(
				`SELECT original, corrected FROM thesaurus_entries WHERE status = 'accepted' ORDER BY LENGTH(original) DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			transformed := query
			var applied []map[string]any
			for rows.Next() {
				var original, corrected string
				if err := rows.Scan(&original, &corrected); err != nil {
					continue
				}
				if strings.Contains(strings.ToLower(transformed), strings.ToLower(original)) {
					transformed = strings.ReplaceAll(
						strings.ToLower(transformed),
						strings.ToLower(original),
						corrected,
					)
					applied = append(applied, map[string]any{
						"original":  original,
						"corrected": corrected,
					})
				}
			}

			if applied == nil {
				applied = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"original":    query,
				"transformed": transformed,
				"applied":     applied,
				"changed":     query != transformed,
			}, flags)
		},
	}
}
