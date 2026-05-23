// Hand-written novel feature. Not generated.
package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/halopsa/internal/store"
)

func newAssetHistoryCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "history [<tag-or-id>]",
		Short: "Every ticket that touched this asset, chronological, with agent + time",
		Long: `Resolves the asset by inventory tag (e.g. LAP-0042) or numeric id.
Then lists tickets matched on asset_id, including agent + first action date.`,
		Example: strings.Trim(`
  # By inventory tag
  halopsa-pp-cli asset history LAP-0042 --json

  # By id
  halopsa-pp-cli asset history 1234 --limit 50
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()
			byID := false
			if _, err := strconv.Atoi(query); err == nil {
				byID = true
			}
			var assetID, assetTag, assetName string
			if byID {
				if err := db.DB().QueryRowContext(cmd.Context(), `SELECT id, COALESCE(json_extract(data,'$.inventory_number'),''), COALESCE(json_extract(data,'$.name'),'') FROM asset WHERE id = ?`, query).Scan(&assetID, &assetTag, &assetName); err != nil {
					return fmt.Errorf("asset %q not in local store: %w", query, err)
				}
			} else {
				if err := db.DB().QueryRowContext(cmd.Context(), `SELECT id, COALESCE(json_extract(data,'$.inventory_number'),''), COALESCE(json_extract(data,'$.name'),'') FROM asset
                    WHERE LOWER(COALESCE(json_extract(data,'$.inventory_number'),'')) = LOWER(?)
                       OR LOWER(COALESCE(json_extract(data,'$.name'),'')) LIKE LOWER(?)
                    LIMIT 1`, query, "%"+query+"%").Scan(&assetID, &assetTag, &assetName); err != nil {
					return fmt.Errorf("asset matching %q not in local store: %w", query, err)
				}
			}
			// Find tickets that reference this asset_id. Halo stores it under tickets.assets[*].id sometimes.
			q := `SELECT t.id, COALESCE(t.agent_name,'?'), COALESCE(json_extract(t.data,'$.status_name'),'?'),
                COALESCE(NULLIF(json_extract(t.data,'$.lastactiondate'),''), t.datecreated),
                COALESCE(t.summary,'')
                FROM tickets t
                WHERE EXISTS (
                    SELECT 1 FROM json_each(COALESCE(json_extract(t.data,'$.assets'),'[]'))
                    WHERE json_extract(json_each.value,'$.id') = ?
                ) OR json_extract(t.data,'$.asset_id') = ?
                ORDER BY COALESCE(NULLIF(json_extract(t.data,'$.lastactiondate'),''), t.datecreated) DESC
                LIMIT ?`
			rows, err := db.DB().QueryContext(cmd.Context(), q, assetID, assetID, limit)
			if err != nil {
				return fmt.Errorf("asset history query: %w", err)
			}
			defer rows.Close()
			type row struct {
				ID         string `json:"id"`
				Agent      string `json:"agent"`
				Status     string `json:"status"`
				LastAction string `json:"last_action"`
				Summary    string `json:"summary"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var lastAction sql.NullString
				if err := rows.Scan(&r.ID, &r.Agent, &r.Status, &lastAction, &r.Summary); err != nil {
					continue
				}
				r.LastAction = lastAction.String
				out = append(out, r)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"asset_id":   assetID,
					"asset_tag":  assetTag,
					"asset_name": assetName,
					"tickets":    out,
					"count":      len(out),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Asset %s (%s) — %s\n\n", assetID, assetTag, assetName)
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tickets reference this asset in the local store.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-20s %-12s %-20s %s\n", "TICKET", "AGENT", "STATUS", "LAST_ACTION", "SUMMARY")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 100))
			for _, r := range out {
				summary := r.Summary
				if len(summary) > 40 {
					summary = summary[:40] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-20s %-12s %-20s %s\n", r.ID, r.Agent, r.Status, truncTime(r.LastAction), summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max ticket rows to return")
	return cmd
}
