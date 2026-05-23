// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity/internal/store"
)

// newRestoreQueueCmd is a parent for the restore-queue oversight commands.
// `list` composes /companies/{id}/restore-queues/ across the local fleet
// snapshot; `--watch` repolls every N seconds and prints diffs.
//
// Note: the generator already emits `restore-queue-web-login` (a single
// endpoint typed command), but no `restore-queue list` parent. This command
// provides the cross-company composer view.
func newRestoreQueueCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore-queue",
		Short: "Cross-company restore-queue oversight (DRaaS-in-flight visibility)",
		Long: `Watch every restore queue across the fleet from one terminal. Composes
GET /companies/{id}/restore-queues/ for each company the local store knows.
With --watch, repolls and prints only the diff per cycle.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newRestoreQueueListCmd(flags))
	return cmd
}

func newRestoreQueueListCmd(flags *rootFlags) *cobra.Command {
	var companyFilter string
	var watch bool
	var interval time.Duration
	var dbPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List per-company restore queues across the fleet",
		Example: `  # One-shot
  servosity-pp-cli restore-queue list --json

  # One company only
  servosity-pp-cli restore-queue list --company 4421 --json

  # Watch every 30 seconds (Ctrl-C to exit)
  servosity-pp-cli restore-queue list --watch --interval 30s`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run"},"results":[]}` + "\n"))
				}
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("servosity-pp-cli")
			}

			companyIDs, err := resolveCompanyIDs(ctx, dbPath, companyFilter)
			if err != nil {
				return err
			}
			if len(companyIDs) == 0 {
				return notFoundErr(fmt.Errorf("no companies in local store — run 'sync' first"))
			}

			pull := func() (map[string]any, error) {
				rows := []map[string]any{}
				for _, cid := range companyIDs {
					path := "/companies/" + cid + "/restore-queues/"
					data, perr := c.Get(path, nil)
					if perr != nil {
						continue
					}
					var items []map[string]json.RawMessage
					if uerr := unmarshalAnyList(data, &items); uerr != nil {
						continue
					}
					for _, it := range items {
						rb, _ := json.Marshal(it)
						row := map[string]any{
							"company_id": cid,
						}
						_ = json.Unmarshal(rb, &row)
						rows = append(rows, row)
					}
				}
				return map[string]any{
					"meta": map[string]any{
						"source":     "live",
						"companies":  len(companyIDs),
						"queue_rows": len(rows),
						"polled_at":  time.Now().UTC().Format(time.RFC3339),
					},
					"results": rows,
				}, nil
			}

			if !watch {
				out, err := pull()
				if err != nil {
					return err
				}
				payload, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
			}

			// --watch path: print first cycle in full, then deltas
			if interval <= 0 {
				interval = 30 * time.Second
			}
			prev, err := pull()
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(prev)
			_ = printOutputWithFlags(cmd.OutOrStdout(), payload, flags)

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
				cur, err := pull()
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch: poll failed: %v\n", err)
					continue
				}
				delta := diffRestoreRows(prev["results"].([]map[string]any), cur["results"].([]map[string]any))
				delta["polled_at"] = time.Now().UTC().Format(time.RFC3339)
				dpayload, _ := json.Marshal(delta)
				_ = printOutputWithFlags(cmd.OutOrStdout(), dpayload, flags)
				prev = cur
			}
		},
	}
	cmd.Flags().StringVar(&companyFilter, "company", "", "Restrict to one company ID")
	cmd.Flags().BoolVar(&watch, "watch", false, "Repoll on an interval and print diffs (Ctrl-C to exit)")
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "Poll interval for --watch")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/servosity-pp-cli/data.db)")
	return cmd
}

func resolveCompanyIDs(ctx context.Context, dbPath, companyFilter string) ([]string, error) {
	if companyFilter != "" {
		return []string{companyFilter}, nil
	}
	st, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, configErr(fmt.Errorf("open store: %w", err))
	}
	defer st.Close()
	rows, err := st.DB().QueryContext(ctx, `SELECT id FROM companies LIMIT 50000`)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil && id != "" {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

func diffRestoreRows(prev, cur []map[string]any) map[string]any {
	keyFn := func(r map[string]any) string {
		// Prefer "id" then "queue_id" then a (company_id, name) hash
		for _, k := range []string{"id", "queue_id", "uuid"} {
			if v := anyToString(r[k]); v != "" {
				return v
			}
		}
		return anyToString(r["company_id"]) + "|" + anyToString(r["name"])
	}
	pi := map[string]map[string]any{}
	for _, r := range prev {
		pi[keyFn(r)] = r
	}
	added, removed, changed := []map[string]any{}, []map[string]any{}, []map[string]any{}
	curIdx := map[string]map[string]any{}
	for _, r := range cur {
		k := keyFn(r)
		curIdx[k] = r
		if pv, ok := pi[k]; !ok {
			added = append(added, r)
		} else if !sameRow(pv, r) {
			row := map[string]any{"key": k, "from": pv, "to": r}
			changed = append(changed, row)
		}
	}
	for k, pv := range pi {
		if _, ok := curIdx[k]; !ok {
			removed = append(removed, pv)
		}
	}
	sort.SliceStable(added, func(i, j int) bool { return anyToString(added[i]["company_id"]) < anyToString(added[j]["company_id"]) })
	sort.SliceStable(removed, func(i, j int) bool {
		return anyToString(removed[i]["company_id"]) < anyToString(removed[j]["company_id"])
	})
	return map[string]any{
		"added":   added,
		"removed": removed,
		"changed": changed,
	}
}

func sameRow(a, b map[string]any) bool {
	// Compare on a small set of status-y keys to avoid noisy timestamp diffs.
	for _, k := range []string{"status", "state", "progress", "current_step"} {
		if anyToString(a[k]) != anyToString(b[k]) {
			return false
		}
	}
	return true
}

// keep strings/time import used
var _ = strings.TrimSpace
var _ = time.Now
