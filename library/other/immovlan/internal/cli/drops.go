// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newDropsCmd(flags))
	})
}

type dropRow struct {
	immovlan.Listing
	FirstPrice  float64 `json:"first_price"`
	LatestPrice float64 `json:"latest_price"`
	CutEUR      float64 `json:"cut_eur"`
	CutPct      float64 `json:"cut_pct"`
	Cuts        int     `json:"cuts"`
	FirstSeen   string  `json:"first_seen"`
	LastCutAt   string  `json:"last_cut_at"`
	DaysListed  *int    `json:"days_listed,omitempty"`
	Gone        bool    `json:"gone"`
}

// dropsView wraps the rows so an empty store still returns an envelope with a note.
type dropsView struct {
	Results []dropRow `json:"results"`
	Note    string    `json:"note,omitempty"`
}

func newDropsCmd(flags *rootFlags) *cobra.Command {
	var deal, postcodes, since, dbPath string
	var limit int
	var includeGone bool
	cmd := &cobra.Command{
		Use:   "drops",
		Short: "Stored listings whose asking price fell (from prices recorded by find and watch)",
		Long: `Immovlan shows no price history. Every find/watch/enrich run records the asking
price it saw; drops lists the listings whose recorded price went down, with the
first and latest price, number of cuts, total cut and days on market.`,
		Example: strings.Trim(`
  immovlan-pp-cli drops --postcode 1030,1210 --agent
  immovlan-pp-cli drops --deal sale --since 30d --limit 20`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list price cuts from the local store")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cutoff, err := sinceCutoff(since)
			if err != nil {
				return err
			}
			f := store.ListingFilter{IncludeGone: includeGone}
			if f.Deal, err = parseDealFlag(deal); err != nil {
				return err
			}
			if f.PostalCodes, err = parsePostcodes(postcodes); err != nil {
				return err
			}
			out := []dropRow{}
			note := ""
			if path, ok := localStoreExists(dbPath); ok {
				db, err := openVlanStore(ctx, path)
				if err != nil {
					return err
				}
				defer db.Close()
				hists, err := db.AllVlanPriceHistories(ctx)
				if err != nil {
					return err
				}
				if len(hists) > 0 {
					ids := make([]string, 0, len(hists))
					for id := range hists {
						ids = append(ids, id)
					}
					f.IDs = ids
					rows, err := db.QueryVlanListings(ctx, f)
					if err != nil {
						return err
					}
					now := time.Now()
					for _, r := range rows {
						h := hists[r.ID]
						if len(h) < 2 || h[len(h)-1].Price >= h[0].Price {
							continue
						}
						cuts, lastCut := 0, ""
						for i := 1; i < len(h); i++ {
							if h[i].Price < h[i-1].Price {
								cuts++
								lastCut = h[i].ObservedAt
							}
						}
						if cuts == 0 || (cutoff != "" && lastCut < cutoff) {
							continue
						}
						row := dropRow{Listing: r.Listing, FirstPrice: h[0].Price, LatestPrice: h[len(h)-1].Price, Cuts: cuts, FirstSeen: r.FirstSeen, LastCutAt: lastCut, Gone: r.GoneAt != ""}
						row.CutEUR = row.FirstPrice - row.LatestPrice
						row.CutPct = pct1(row.CutEUR / row.FirstPrice)
						if d, ok := immovlan.DaysListed(r.CreatedAt, now); ok {
							row.DaysListed = &d
						}
						out = append(out, row)
					}
				}
			} else {
				note = noStoreNote(path, "find or watch")
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].CutPct > out[j].CutPct })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), dropsView{Results: out, Note: note}, flags)
			}
			if note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), note)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No price cuts recorded yet. Cuts appear once find or watch has seen a listing at two prices.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tLOCALITY\tFIRST\tNOW\tCUT\tCUTS\tLAST CUT\tDAYS\tURL")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t-%.1f%%\t%d\t%s\t%s\t%s\n", r.ID, locLabel(r.Listing), fmtEUR(r.FirstPrice), fmtEUR(r.LatestPrice), r.CutPct, r.Cuts, shortDate(r.LastCutAt), intStr(r.DaysListed), termSafe(r.URL))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&deal, "deal", "", "Only this deal (sale, rent...)")
	cmd.Flags().StringVar(&postcodes, "postcode", "", "Only these postal codes, comma-separated")
	cmd.Flags().StringVar(&since, "since", "", "Only cuts recorded in the last window (e.g. 30d)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows")
	cmd.Flags().BoolVar(&includeGone, "include-gone", false, "Include listings that left Immovlan")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}
