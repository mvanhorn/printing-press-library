package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
)

// newSyncStatusChangesCmd walks every synced estimate and pulls its
// status_changes feed into the local store under resource_type
// `estimate-status-changes` keyed by `<estimateId>-<changedAt>`. This
// per-estimate sweep is required for the audit/reports commands that join
// across status transitions; the ST API has no tenant-wide status_changes
// list endpoint.
func newSyncStatusChangesCmd(flags *rootFlags) *cobra.Command {
	var (
		tenant     string
		dbPath     string
		estimateID int64
		since      string
	)
	cmd := &cobra.Command{
		Use:   "sync-status-changes",
		Short: "Pull per-estimate status change history into the local store",
		Long: "Walks every synced estimate and pulls its status_changes feed under\n" +
			"resource_type `estimate-status-changes`, keyed by `<estimateId>-\n" +
			"<changedAt>` so reruns dedupe deterministically. --estimate limits\n" +
			"the sweep to one id (use after a known change); --since limits to\n" +
			"estimates modified in the window so the full-store walk doesn't\n" +
			"refetch unchanging older estimates. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli sync-status-changes
  servicetitan-salestech-pp-cli sync-status-changes --estimate 78421
  servicetitan-salestech-pp-cli sync-status-changes --since 7d --json
`, "\n"),
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			t := resolveTenant(tenant)
			if t == "" {
				return fmt.Errorf("tenant is required (pass --tenant or set ST_TENANT_ID)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openStoreLenient(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			var ids []int64
			if estimateID > 0 {
				ids = []int64{estimateID}
			} else {
				estimates, err := salestech.LoadEstimates(db)
				if err != nil {
					return err
				}
				var cutoff time.Time
				if since != "" {
					d, err := parseAgeDuration(since)
					if err != nil {
						return err
					}
					cutoff = time.Now().UTC().Add(-d)
				}
				for _, e := range estimates {
					if !cutoff.IsZero() {
						mod, ok := parseTimestampLocal(e.ModifiedOn)
						if !ok || mod.Before(cutoff) {
							continue
						}
					}
					ids = append(ids, e.ID)
				}
			}

			totalChanges := 0
			var fetchErr error
			for i, id := range ids {
				path := fmt.Sprintf("/tenant/%s/status/estimates/%d/changes", t, id)
				body, err := c.Get(path, nil)
				if err != nil {
					fetchErr = fmt.Errorf("estimate %d (%d/%d): %w", id, i+1, len(ids), err)
					break
				}
				// Response is either an array of changes or { data: [...] }.
				var arr []map[string]any
				if err := json.Unmarshal(body, &arr); err != nil {
					var env struct {
						Data []map[string]any `json:"data"`
					}
					if err := json.Unmarshal(body, &env); err != nil {
						continue
					}
					arr = env.Data
				}
				for _, ch := range arr {
					if _, has := ch["estimateId"]; !has {
						ch["estimateId"] = id
					}
					at, _ := ch["changedAt"].(string)
					if at == "" {
						if v, ok := ch["timestampUtc"].(string); ok {
							at = v
							ch["changedAt"] = v
						}
					}
					key := fmt.Sprintf("%d-%s", id, at)
					data, err := json.Marshal(ch)
					if err != nil {
						continue
					}
					if err := db.Upsert(salestech.ResStatusChanges, key, data); err != nil {
						return fmt.Errorf("upsert change %s: %w", key, err)
					}
					totalChanges++
				}
			}
			out := map[string]any{
				"resource":         salestech.ResStatusChanges,
				"estimates_walked": len(ids),
				"changes_synced":   totalChanges,
				"complete":         fetchErr == nil,
			}
			if fetchErr != nil {
				out["error"] = fetchErr.Error()
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "Tenant id (defaults to ST_TENANT_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().Int64Var(&estimateID, "estimate", 0, "Limit to one estimate id (0 = all synced estimates)")
	cmd.Flags().StringVar(&since, "since", "", "Only estimates modified in this window (e.g. 24h, 7d)")
	return cmd
}

// parseTimestampLocal mirrors salestech.parseTimestamp without importing
// time-zone juggling into this file.
func parseTimestampLocal(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.999999",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

var _ = strconv.Itoa
