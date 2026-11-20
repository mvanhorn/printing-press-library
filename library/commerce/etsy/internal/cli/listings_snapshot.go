// Hand-authored (Slice 2): listings snapshot.
// pp:data-source auto

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if p := s2FindChild(root, "listings"); p != nil {
			addNovelCommandIfAbsent(p, newS2ListingsSnapshotCmd(flags))
		}
	})
}

type s2SnapshotResult struct {
	Mode       string `json:"mode"`
	SnapshotID int64  `json:"snapshot_id"`
	ListingID  int64  `json:"listing_id"`
	Source     string `json:"source"`
	HasInv     bool   `json:"has_inventory"`
	HasPers    bool   `json:"has_personalization"`
}

func newS2ListingsSnapshotCmd(flags *rootFlags) *cobra.Command {
	var list, fromStore, bodies bool
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "snapshot [listing_id]",
		Short: "Save a listing's data, inventory and personalization as a history row before you change it.",
		Long: strings.Trim(`
Stores the listing, its inventory and its personalization settings as JSON in
the local database (table listing_snapshots), appending a new row each time so
you keep a history. The default reads live (three GET requests); --from-store
copies the already-synced rows with no API calls. --list shows the saved
history (all listings, or one with listing_id); --bodies adds the stored JSON.
'listings bulk-edit --apply' snapshots every listing automatically first.
`, "\n"),
		Example: strings.Trim(`
  etsy-pp-cli listings snapshot 1000000001
  etsy-pp-cli listings snapshot 1000000001 --from-store
  etsy-pp-cli listings snapshot --list --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings snapshot")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("etsy-pp-cli")
			}
			var id int64
			if len(args) > 0 {
				n, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
				if err != nil || n <= 0 {
					return usageErr(fmt.Errorf("listing_id %q must be a positive integer", args[0]))
				}
				id = n
			}
			if list {
				rows := make([]store.ListingSnapshotRow, 0)
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local database at %s; no snapshots yet\n", dbPath)
					return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
				}
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				defer db.Close()
				if rows, err = db.ListListingSnapshots(id, limit, bodies); err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if id == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("listing_id is required (or use --list)"))
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			idStr := strconv.FormatInt(id, 10)
			var listingJSON, invJSON, persJSON string
			source := "live"
			if fromStore {
				source = "store"
				var data []byte
				if err := db.DB().QueryRow(`SELECT data FROM listings WHERE listing_id = ? OR id = ? LIMIT 1`, id, idStr).Scan(&data); err != nil {
					return notFoundErr(fmt.Errorf("listing %d is not in the local store; run `etsy-pp-cli sync` or drop --from-store", id))
				}
				listingJSON = string(data)
				invJSON = s2StoredInventory(db, id)
			} else {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				// read-before-write / snapshot / refresh reads must see live data, never the 5-minute response cache
				c.NoCache = true
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				thr := &bulkThrottle{interval: bulkMinInterval}
				get := func(path string) ([]byte, error) {
					if err := thr.wait(ctx); err != nil {
						return nil, err
					}
					return c.Get(ctx, path, nil)
				}
				l, err := get("/listings/" + idStr)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				inv, err := get("/listings/" + idStr + "/inventory")
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				pers, err := get("/listings/" + idStr + "/personalization")
				if err != nil && bulkAPIStatus(err) != 404 {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				listingJSON, invJSON, persJSON = string(l), string(inv), string(pers)
			}
			snap, err := db.SaveListingSnapshot(id, source, listingJSON, invJSON, persJSON)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), s2SnapshotResult{Mode: "saved", SnapshotID: snap, ListingID: id, Source: source, HasInv: invJSON != "", HasPers: persJSON != ""}, flags)
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "Show saved snapshots instead of taking one")
	cmd.Flags().BoolVar(&fromStore, "from-store", false, "Copy the synced local rows instead of calling the API")
	cmd.Flags().BoolVar(&bodies, "bodies", false, "With --list, include the stored JSON")
	cmd.Flags().IntVar(&limit, "limit", 50, "With --list, maximum snapshots to show")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
