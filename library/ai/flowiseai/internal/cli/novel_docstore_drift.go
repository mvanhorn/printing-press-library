// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newDocstoreDriftCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Show which document stores got new content recently and which chatflows reference each store",
		Long: `Joins the locally synced document_store table with upsert_history to show
which stores received new upserts in the time window (default: 7 days). Then
parses cached chatflow flowData to surface which chatflows reference each
drifting store, so you can plan re-evaluation of downstream flows.

Run ` + "`flowiseai-pp-cli sync`" + ` first to ensure the local cache is fresh.`,
		Example: "  flowiseai-pp-cli docstore drift --since 7d --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoffTS, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := cutoffTS.UTC().Format(time.RFC3339)

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Aggregate upsert_history rows by chatflowid in the window. upsert_history
			// is keyed by chatflowid (not docstoreId) in the Flowise schema — it tracks
			// which chatflow triggered which upsert. We still need to also surface
			// document_store updated_date drift for direct store updates.
			historyQuery := `SELECT COALESCE(chatflowid, '') AS chatflowid,
				COUNT(*) AS upsert_count,
				MIN(date) AS first_seen,
				MAX(date) AS last_seen
				FROM upsert_history
				WHERE date IS NOT NULL AND date >= ?
				GROUP BY chatflowid
				ORDER BY upsert_count DESC`
			hrows, err := db.DB().QueryContext(cmd.Context(), historyQuery, cutoff)
			if err != nil {
				return fmt.Errorf("upsert_history query: %w", err)
			}
			type chatflowDrift struct {
				ChatflowID   string `json:"chatflowId"`
				ChatflowName string `json:"chatflowName"`
				UpsertCount  int    `json:"upsertCount"`
				FirstSeen    string `json:"firstSeen"`
				LastSeen     string `json:"lastSeen"`
			}
			var chatflows []chatflowDrift
			for hrows.Next() {
				var cd chatflowDrift
				if err := hrows.Scan(&cd.ChatflowID, &cd.UpsertCount, &cd.FirstSeen, &cd.LastSeen); err != nil {
					hrows.Close()
					return fmt.Errorf("scan: %w", err)
				}
				chatflows = append(chatflows, cd)
			}
			hrows.Close()

			// Resolve chatflow names
			for i, cd := range chatflows {
				if cd.ChatflowID == "" {
					continue
				}
				var name string
				if scanErr := db.DB().QueryRowContext(cmd.Context(), `SELECT COALESCE(name,'') FROM chatflows WHERE id = ?`, cd.ChatflowID).Scan(&name); scanErr == nil {
					chatflows[i].ChatflowName = name
				}
			}

			// Document stores updated in the window
			storeQuery := `SELECT id, COALESCE(name,'') AS name,
				COALESCE(status,'') AS status,
				COALESCE(updated_date,'') AS updated_date
				FROM document_store
				WHERE updated_date IS NOT NULL AND updated_date >= ?
				ORDER BY updated_date DESC`
			srows, err := db.DB().QueryContext(cmd.Context(), storeQuery, cutoff)
			if err != nil {
				return fmt.Errorf("document_store query: %w", err)
			}
			type storeDrift struct {
				ID                 string   `json:"id"`
				Name               string   `json:"name"`
				Status             string   `json:"status"`
				UpdatedDate        string   `json:"updatedDate"`
				ReferencingFlows   []string `json:"referencingChatflows,omitempty"`
			}
			var stores []storeDrift
			for srows.Next() {
				var s storeDrift
				if err := srows.Scan(&s.ID, &s.Name, &s.Status, &s.UpdatedDate); err != nil {
					srows.Close()
					return fmt.Errorf("scan: %w", err)
				}
				stores = append(stores, s)
			}
			srows.Close()

			// For each store, scan every chatflow's flow_data looking for the store id
			// or name as a reference. This is best-effort string match — flowData is
			// big and varied, but a literal id/name match is reliable for "this flow
			// references this store" reports.
			if len(stores) > 0 {
				cfRows, qErr := db.DB().QueryContext(cmd.Context(), `SELECT id, COALESCE(name,''), COALESCE(flow_data,'') FROM chatflows`)
				if qErr == nil {
					type cfRow struct{ id, name, fd string }
					var allFlows []cfRow
					for cfRows.Next() {
						var r cfRow
						_ = cfRows.Scan(&r.id, &r.name, &r.fd)
						allFlows = append(allFlows, r)
					}
					cfRows.Close()
					for si, s := range stores {
						for _, fr := range allFlows {
							if fr.fd == "" {
								continue
							}
							if strings.Contains(fr.fd, s.ID) || (s.Name != "" && strings.Contains(fr.fd, s.Name)) {
								label := fr.name
								if label == "" {
									label = fr.id
								}
								stores[si].ReferencingFlows = append(stores[si].ReferencingFlows, label)
							}
						}
					}
				}
			}

			result := struct {
				Since               string         `json:"since"`
				Cutoff              string         `json:"cutoff"`
				DocumentStoresDrift []storeDrift   `json:"documentStoresDrift"`
				ChatflowsByUpserts  []chatflowDrift `json:"chatflowsByUpserts"`
			}{
				Since:               since,
				Cutoff:              cutoff,
				DocumentStoresDrift: stores,
				ChatflowsByUpserts:  chatflows,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Drift report since %s (cutoff %s)\n\n", since, cutoff)
			if len(stores) == 0 && len(chatflows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No drift detected — local cache shows no document store updates or upsert history in the window. Run `sync` to refresh.")
				return nil
			}
			if len(stores) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Document stores with updates (%d):\n", len(stores))
				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "ID\tNAME\tSTATUS\tUPDATED\tREFERENCING FLOWS")
				for _, s := range stores {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, truncate(s.Name, 30), s.Status, s.UpdatedDate, truncate(strings.Join(s.ReferencingFlows, ", "), 50))
				}
				w.Flush()
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if len(chatflows) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Chatflows by upsert volume (%d):\n", len(chatflows))
				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "CHATFLOW\tUPSERTS\tFIRST\tLAST")
				for _, cd := range chatflows {
					label := cd.ChatflowName
					if label == "" {
						label = cd.ChatflowID
					}
					fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", truncate(label, 40), cd.UpsertCount, cd.FirstSeen, cd.LastSeen)
				}
				w.Flush()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "7d", "Time window for drift detection (e.g. 24h, 7d, 30d)")
	return cmd
}

// avoid unused import warnings on conditional code paths
var _ = json.Marshal

