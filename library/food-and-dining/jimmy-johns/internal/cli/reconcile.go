// PATCH: hand-authored workflow `reconcile` — diffs locally synced stores
// against fresh API state. Reports new / removed / unchanged stores. Uses
// both the local store and a live API call. See .printing-press-patches.json
// patch id "workflow-reconcile".

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"jimmy-johns-pp-cli/internal/store"
)

type reconcileDiff struct {
	Resource     string   `json:"resource"`
	LocalCount   int      `json:"local_count"`
	RemoteCount  int      `json:"remote_count"`
	NewIDs       []string `json:"new_ids,omitempty"`
	RemovedIDs   []string `json:"removed_ids,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var address string
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Diff locally synced stores against the live API",
		Long: `Fetch the current store list from the live API, compare against the local
SQLite store, and report new / removed IDs. Pure read-only.

Requires both: a populated local store (run 'sync' first) AND a live API
session that can respond (run 'auth import-cookies' if PerimeterX is gating
the endpoint).`,
		Example: `  jimmy-johns-pp-cli reconcile --address 98112 --json
  jimmy-johns-pp-cli reconcile --address "Seattle, WA"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("jimmy-johns-pp-cli")
			}
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			diff := reconcileDiff{Resource: "stores"}

			// Local set.
			localIDs := map[string]bool{}
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT id FROM stores`)
			if err != nil {
				return fmt.Errorf("local store query: %w", err)
			}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					localIDs[id] = true
				}
			}
			rows.Close()
			diff.LocalCount = len(localIDs)

			// Live set (one API call).
			c, err := flags.newClient()
			if err != nil {
				diff.Notes = append(diff.Notes, fmt.Sprintf("client init failed: %v", err))
				return emitReconcile(cmd, flags, diff)
			}
			params := map[string]string{}
			if address != "" {
				params["addressSearch"] = address
			}
			respBody, err := c.Get("/stores", params)
			if err != nil {
				diff.Notes = append(diff.Notes,
					fmt.Sprintf("live /stores call failed (likely PerimeterX): %v", err))
				return emitReconcile(cmd, flags, diff)
			}

			remoteIDs := map[string]bool{}
			var arr []map[string]any
			if err := json.Unmarshal(respBody, &arr); err == nil {
				for _, item := range arr {
					if id, ok := item["id"]; ok {
						remoteIDs[fmt.Sprint(id)] = true
					} else if id, ok := item["storeId"]; ok {
						remoteIDs[fmt.Sprint(id)] = true
					}
				}
			}
			diff.RemoteCount = len(remoteIDs)

			for id := range remoteIDs {
				if !localIDs[id] {
					diff.NewIDs = append(diff.NewIDs, id)
				}
			}
			for id := range localIDs {
				if !remoteIDs[id] {
					diff.RemovedIDs = append(diff.RemovedIDs, id)
				}
			}

			return emitReconcile(cmd, flags, diff)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (defaults to the user cache dir)")
	cmd.Flags().StringVar(&address, "address", "", "Address to scope the remote query (defaults to none, which the API may reject)")
	return cmd
}

func emitReconcile(cmd *cobra.Command, flags *rootFlags, diff reconcileDiff) error {
	if flags.asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(diff)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Reconcile %s:\n", diff.Resource)
	fmt.Fprintf(w, "  local:    %d\n", diff.LocalCount)
	fmt.Fprintf(w, "  remote:   %d\n", diff.RemoteCount)
	fmt.Fprintf(w, "  new:      %d\n", len(diff.NewIDs))
	fmt.Fprintf(w, "  removed:  %d\n", len(diff.RemovedIDs))
	for _, n := range diff.Notes {
		fmt.Fprintf(w, "Note: %s\n", n)
	}
	return nil
}
