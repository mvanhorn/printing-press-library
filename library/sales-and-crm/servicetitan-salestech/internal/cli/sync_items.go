package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// newSyncItemsCmd walks the tenant-wide /estimates/items list endpoint and
// stores each line item under resource_type `estimate-items` keyed by its
// own id, with `estimateId` preserved so audit/reports can join. The
// framework `sync` doesn't cover this because the items get-list command
// uses the parent `estimates` resourceType, which would clobber the parent
// rows.
func newSyncItemsCmd(flags *rootFlags) *cobra.Command {
	var (
		tenant     string
		dbPath     string
		pageSize   int
		all        bool
		estimateID string
		modifiedOn string
		activeOnly bool
	)
	cmd := &cobra.Command{
		Use:   "sync-items",
		Short: "Pull estimate line items into the local store (resource_type: estimate-items)",
		Long: "Walks /tenant/{tenant}/estimates/items and stores every line item\n" +
			"under resource_type `estimate-items` keyed by item id, with\n" +
			"`estimateId` preserved so audit/reports can join back to the parent\n" +
			"estimate. Required for the audit / sku-frequency commands.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli sync-items
  servicetitan-salestech-pp-cli sync-items --estimate-id 78421
  servicetitan-salestech-pp-cli sync-items --modified-after 2026-05-01T00:00:00Z --json
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
			path := fmt.Sprintf("/tenant/%s/estimates/items", t)
			params := map[string]string{
				"pageSize":     strconv.Itoa(pageSize),
				"includeTotal": "true",
			}
			if estimateID != "" {
				params["estimateId"] = estimateID
			}
			if modifiedOn != "" {
				params["modifiedOnOrAfter"] = modifiedOn
			}
			if activeOnly {
				params["active"] = "True"
			}
			total := 0
			page := 1
			for {
				params["page"] = strconv.Itoa(page)
				body, err := c.Get(path, params)
				if err != nil {
					return fmt.Errorf("list page %d: %w", page, err)
				}
				var env struct {
					Data    []json.RawMessage `json:"data"`
					HasMore bool              `json:"hasMore"`
				}
				if err := json.Unmarshal(body, &env); err != nil {
					return fmt.Errorf("parse page %d: %w", page, err)
				}
				for _, raw := range env.Data {
					var it struct {
						ID         int64 `json:"id"`
						EstimateID int64 `json:"estimateId"`
					}
					_ = json.Unmarshal(raw, &it)
					if it.ID == 0 {
						continue
					}
					if it.EstimateID == 0 && estimateID != "" {
						var m map[string]any
						if json.Unmarshal(raw, &m) == nil {
							if eid, err := strconv.ParseInt(estimateID, 10, 64); err == nil {
								m["estimateId"] = eid
							}
							raw, _ = json.Marshal(m)
						}
					}
					if err := db.Upsert(salestech.ResEstimateItems, strconv.FormatInt(it.ID, 10), raw); err != nil {
						return fmt.Errorf("upsert item %d: %w", it.ID, err)
					}
					total++
				}
				if !env.HasMore || !all {
					break
				}
				page++
			}
			out := map[string]any{
				"resource":     salestech.ResEstimateItems,
				"items_synced": total,
				"pages":        page,
				"complete":     true,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "Tenant id (defaults to ST_TENANT_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&pageSize, "page-size", 100, "Page size for the items list endpoint")
	cmd.Flags().BoolVar(&all, "all", false, "Page through every record. Default false (single page = up to --page-size items) so the bare command stays fast; pass --all to walk the whole feed.")
	cmd.Flags().StringVar(&estimateID, "estimate-id", "", "Limit sync to one estimate's items")
	cmd.Flags().StringVar(&modifiedOn, "modified-after", "", "Only items modified on or after this ISO 8601 timestamp")
	cmd.Flags().BoolVar(&activeOnly, "active", true, "Only active items (default true)")
	return cmd
}

// openStoreLenient opens the local store without enforcing estimates-non-
// empty. Sync commands legitimately run on an empty store during initial
// bootstrap; the audit commands enforce non-empty via openSalestechStore.
func openStoreLenient(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("servicetitan-salestech-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	return db, nil
}
