// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/riverside-fm/internal/store"
)

// newSearchCmd queries the resources_fts virtual table for full-text matches
// across every synced resource, with optional filters on resource_type and a
// speaker substring (transcripts only).
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var resourceType string
	var speaker string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across synced resources (recordings, projects, transcripts).",
		Long: `Runs a FTS5 query against the resources_fts virtual table built by ` + "`sync`" + `.
Without a query, prints usage. Use --type to scope to a resource type (transcriptions,
projects, recordings); use --speaker to substring-match speaker names inside transcript
bodies; use --limit to cap result count.`,
		Example: `  riverside-fm-pp-cli search "compounding loop" --json
  riverside-fm-pp-cli search "network effects" --type transcriptions --speaker "Damien" --limit 50`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("search query required"))
			}
			if dbPath == "" {
				dbPath = defaultRiversideDB()
			}
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w (run `sync` first)", err)
			}
			defer s.Close()

			rawDB := s.DB()
			sql := `SELECT id, resource_type, snippet(resources_fts, 2, '[', ']', '...', 32) AS hit
FROM resources_fts WHERE resources_fts MATCH ?`
			params := []any{query}
			if resourceType != "" {
				sql += ` AND resource_type = ?`
				params = append(params, resourceType)
			}
			if speaker != "" {
				sql += ` AND content LIKE ?`
				params = append(params, "%"+speaker+"%")
			}
			sql += ` LIMIT ?`
			params = append(params, limit)

			rows, err := rawDB.QueryContext(cmd.Context(), sql, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type row struct {
				ID           string `json:"id"`
				ResourceType string `json:"resource_type"`
				Hit          string `json:"hit"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.ID, &r.ResourceType, &r.Hit); err != nil {
					return err
				}
				out = append(out, r)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(j))
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matches for %q in the local store. Run `sync` if you haven't yet.\n", query)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d matches:\n", len(out))
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n    %s\n", r.ResourceType, r.ID, r.Hit)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourceType, "type", "", "Scope to one resource type (e.g. transcriptions, projects)")
	cmd.Flags().StringVar(&speaker, "speaker", "", "Speaker substring filter (transcript-only matches)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/riverside-fm-pp-cli/store.db)")
	return cmd
}

func defaultRiversideDB() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "riverside-fm-pp-cli", "store.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "riverside-fm-pp-cli", "store.db")
}
