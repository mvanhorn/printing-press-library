// search: offline full-text-ish search over synced listings in the local store.
// Fulfils absorbed feature "offline search of synced listings".
// pp:data-source local
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search synced listings offline (title, location, agency, city)",
		Long: "Search the local store of synced listings by matching the query against title, " +
			"location/area, agency, and city. Run 'zameen-pp-cli pull ...' first to populate it.",
		Example:     "  zameen-pp-cli search \"corner house\" --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local store")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			q := strings.ToLower(query)
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return emitEmptyMirrorHint(cmd, flags, dbPath)
			}

			// Fast path: the resources FTS index, scoped to listings.
			matched := make([]types.Listing, 0)
			if db, oErr := store.OpenReadOnlyContext(cmd.Context(), dbPath); oErr == nil {
				if raws, sErr := db.SearchListings(query, limit); sErr == nil {
					for _, r := range raws {
						var l types.Listing
						if json.Unmarshal(r, &l) == nil {
							matched = append(matched, l)
						}
					}
				}
				db.Close()
			}

			// Fallback: substring scan of the full store (FTS can miss partial
			// tokens; this guarantees results for simple queries).
			if len(matched) == 0 {
				listings, err := loadStoredListings(cmd.Context(), dbPath)
				if err != nil {
					if errors.Is(err, errNoMirror) {
						return emitEmptyMirrorHint(cmd, flags, dbPath)
					}
					return err
				}
				for _, l := range listings {
					hay := strings.ToLower(l.Title + " " + l.Location + " " + l.Agency + " " + l.City + " " + l.PropertyType)
					if q == "" || strings.Contains(hay, q) {
						matched = append(matched, l)
					}
				}
			}
			sort.SliceStable(matched, func(i, j int) bool { return matched[i].CreatedAt > matched[j].CreatedAt })
			if limit > 0 && len(matched) > limit {
				matched = matched[:limit]
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "matched %d listings\n", len(matched))
			return emitListings(cmd, flags, matched)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum listings to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
