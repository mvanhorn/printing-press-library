// pp:client-call
// Hand-written sync: the generic spec-driven sync template does not fit a
// stateful HTML search portal. Here `sync` (a) seeds the local store with the
// most recent provvedimenti and (b) refreshes every saved watch.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var full bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Popola lo store locale con i provvedimenti recenti e aggiorna le ricerche salvate (watch).",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli sync
  giustizia-amministrativa-pp-cli sync --limit 100
  giustizia-amministrativa-pp-cli sync --full`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if gaSkip(flags) {
				out := map[string]any{"dry_run": true, "would_sync": "provvedimenti recenti + ricerche salvate (watch)"}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if full {
				limit = 200
			}
			if limit <= 0 {
				limit = 50
			}
			st, err := openGAStore(cmd.Context())
			if err != nil {
				return err
			}
			defer st.Close()
			c := gaclient.New()

			// Seed: most recent provvedimenti (empty query = newest first).
			// "fetched" and "added" are reported separately on purpose: they
			// are equal only on a first run, and a single number invites the
			// reader to call a re-fetch of records already in the store "N new
			// provvedimenti" when nothing changed at all.
			fetched, added := 0, 0
			if res, serr := c.Search(cmd.Context(), gaclient.SearchOptions{Limit: limit}); serr == nil {
				for _, p := range res.Items {
					if id := provID(p); id != "" {
						if existing, gerr := st.Get("provvedimenti", id); gerr != nil || len(existing) == 0 {
							added++
						}
					}
				}
				persistProvvedimenti(st, res.Items)
				fetched = len(res.Items)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "seed recenti: %v\n", serr)
			}

			// Refresh saved watches.
			watches, _ := st.List("watches", 100000)
			type result struct {
				Name string `json:"name"`
				New  int    `json:"new"`
			}
			results := []result{}
			for _, raw := range watches {
				var ws watchState
				if json.Unmarshal(raw, &ws) != nil || ws.Name == "" {
					continue
				}
				res, serr := c.Search(cmd.Context(), ws.Opts)
				if serr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch %q: %v\n", ws.Name, serr)
					continue
				}
				persistProvvedimenti(st, res.Items)
				seen := map[string]bool{}
				for _, id := range ws.Seen {
					seen[id] = true
				}
				newCount := 0
				for _, p := range res.Items {
					id := provID(p)
					if !seen[id] {
						seen[id] = true
						ws.Seen = append(ws.Seen, id)
						newCount++
					}
				}
				const maxSeen = 5000
				if len(ws.Seen) > maxSeen {
					ws.Seen = ws.Seen[len(ws.Seen)-maxSeen:]
				}
				ws.LastNew = newCount
				ws.RunCount++
				if data, merr := json.Marshal(ws); merr == nil {
					_ = st.Upsert("watches", ws.Name, data)
				}
				results = append(results, result{Name: ws.Name, New: newCount})
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Sync: %d provvedimenti recenti scaricati (%d nuovi nello store), %d watch aggiornate.\n", fetched, added, len(results))
			}
			data, _ := json.Marshal(map[string]any{"fetched": fetched, "added": added, "watches_synced": len(results), "watches": results})
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Numero di provvedimenti recenti da scaricare.")
	cmd.Flags().BoolVar(&full, "full", false, "Scarica più provvedimenti recenti (equivale a --limit 200).")
	return cmd
}
