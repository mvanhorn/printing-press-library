// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newUnusedInCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		variant string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "unused-in [album_name]",
		Short: "List album members that haven't been downloaded locally yet",
		Long: `Show every nasa_id in a mirrored album that does NOT yet appear in the
local downloads ledger. Use this to plan the next batch of downloads
("which Apollo-at-50 images haven't I grabbed yet?") without keeping a
manual list of used nasa_ids.

Run 'mirror album <name>' first to populate album_members; run
'download album <name>' to record what's been downloaded.`,
		Example:     "  nasa-images-pp-cli unused-in Apollo-at-50 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			album := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would list unused entries in album %q\n", album)
				return nil
			}
			ctx := cmd.Context()
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			query := `
				SELECT am.nasa_id, COALESCE(json_extract(r.data, '$.title'), '') AS title
				FROM album_members am
				LEFT JOIN downloads d ON d.nasa_id = am.nasa_id AND (? = '' OR d.variant = ?) AND d.status = 'completed'
				LEFT JOIN resources r ON r.resource_type = 'asset' AND r.id = am.nasa_id
				WHERE am.album_name = ?
				  AND d.nasa_id IS NULL
				ORDER BY am.position
				LIMIT ?
			`
			if limit <= 0 {
				limit = 100
			}
			rows, err := s.DB().QueryContext(ctx, query, variant, variant, album, limit)
			if err != nil {
				return fmt.Errorf("querying album_members: %w", err)
			}
			defer rows.Close()

			type entry struct {
				NasaID string `json:"nasa_id"`
				Title  string `json:"title,omitempty"`
			}
			var entries []entry
			for rows.Next() {
				var e entry
				var title sql.NullString
				if err := rows.Scan(&e.NasaID, &title); err == nil {
					e.Title = title.String
					entries = append(entries, e)
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			// Distinguish unknown album (no album_members rows) from a
			// fully-downloaded one. Unknown-album invocations are usage
			// errors so dogfood/error_path tests see a non-zero exit.
			var seen int
			_ = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM album_members WHERE album_name = ?`, album).Scan(&seen)
			if seen == 0 {
				return fmt.Errorf("no album_members rows for album %q; run 'mirror album %s' first", album, album)
			}
			result := map[string]any{
				"album":   album,
				"variant": variant,
				"unused":  entries,
				"count":   len(entries),
			}
			if len(entries) == 0 {
				result["note"] = "every member of this album has been downloaded"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&variant, "variant", "", "Only count downloads of this variant as 'used' (e.g. orig). Empty = any variant.")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum unused entries to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}
