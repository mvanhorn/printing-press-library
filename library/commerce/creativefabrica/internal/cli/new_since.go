// Hand-authored transcendence: show catalog items added since the last run for
// a tracked query or designer, by diffing against a local snapshot.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/snapshot"
	"github.com/spf13/cobra"
)

func newNovelNewSinceCmd(flags *rootFlags) *cobra.Command {
	var query, designer, itemType string
	var limit int
	var reset bool
	cmd := &cobra.Command{
		Use:   "new-since [query]",
		Short: "Show catalog items added since your last run for a tracked query or designer",
		Long: `Track the public catalog: re-run a saved query (or a designer's catalog) sorted
newest-first, diff the object ids against your last local snapshot, and print
only what is new. The first run seeds the snapshot and reports nothing new.

This tracks the public catalog, not a personal library (which is not in scope).`,
		Example:     strings.Trim("\n  creativefabrica-pp-cli new-since \"watercolor flowers\" --agent\n  creativefabrica-pp-cli new-since --designer 2880714", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) > 0 {
				query = args[0]
			}
			if query == "" && designer == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a query or --designer to track"))
			}
			if dryRunOK(flags) {
				return nil
			}
			key := "q:" + query + "|d:" + designer + "|t:" + itemType
			store := snapshot.Open("")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newAlgoliaClient(flags)
			q := catalogQuery{query: query, designer: designer, itemType: itemType, sortBy: "newest", limit: limit}
			req := q.request()
			req.HitsPerPage = clampInt(limit, 20, 100)
			results, err := c.Search(ctx, req)
			if err != nil {
				return apiErr(err)
			}
			var hits []algolia.Hit
			if len(results) > 0 {
				hits = results[0].Hits
			}
			ids := make([]string, 0, len(hits))
			byID := make(map[string]algolia.Hit, len(hits))
			for _, h := range hits {
				ids = append(ids, h.ObjectID)
				byID[h.ObjectID] = h
			}

			prior, seeded := store.Get(key)
			// Fail loud: a tracker that cannot persist its baseline has no
			// product. Silently dropping this error makes the first run claim
			// "seeded" forever, and later runs re-report the same stale set as
			// new on every invocation.
			if perr := store.Put(key, ids); perr != nil {
				return fmt.Errorf("saving snapshot for this tracker: %w", perr)
			}

			// --reset re-seeds the baseline rather than diffing against the old
			// (or an empty) one, so it never reports every current item as "new".
			if reset || !seeded {
				verb := "seeded"
				if reset {
					verb = "reset"
				}
				msg := fmt.Sprintf("%s snapshot for this tracker (%d items); re-run later to see what's new", verb, len(ids))
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, map[string]any{"seeded": true, "reset": reset, "tracked": len(ids), "new": []productView{}, "note": msg})
				}
				fmt.Fprintln(cmd.OutOrStdout(), msg)
				return nil
			}

			added := snapshot.Diff(prior.ObjectIDs, ids)
			newHits := make([]algolia.Hit, 0, len(added))
			for _, id := range added {
				if h, ok := byID[id]; ok {
					newHits = append(newHits, h)
				}
			}
			sortHitsByDate(newHits)
			return printProducts(cmd, flags, toViews(newHits))
		},
	}
	cmd.Flags().StringVar(&designer, "designer", "", "Track a designer's catalog (id or name) instead of a query")
	cmd.Flags().StringVar(&itemType, "type", "", "Constrain the tracked set to a product type")
	cmd.Flags().IntVar(&limit, "limit", 50, "Newest items to compare (20-100)")
	cmd.Flags().BoolVar(&reset, "reset", false, "Reset this tracker's snapshot before comparing")
	return cmd
}
