// PATCH(novel-feature): events price-bands — bucket events by priceRanges.min
// into bands and report count + sample events per band, grouped by
// classification. Hand-authored.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsPriceBandsCmd(flags *rootFlags) *cobra.Command {
	var window int
	var dmaID string

	cmd := &cobra.Command{
		Use:   "price-bands",
		Short: "Bucket events by priceRanges.min into bands (<$50 / $50-100 / $100-200 / $200+)",
		Long: strings.TrimSpace(`
Distribution of event prices across the synced local store, grouped by
classification segment + band. Missing price data is bucketed as
"unknown" (Discovery omits priceRanges on resale or dynamic-priced events).

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events price-bands --dma 383 --window 30 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			conds := []string{`json_extract(data, '$.dates.start.localDate') IS NOT NULL`}
			argv := []any{}
			if window > 0 {
				conds = append(conds, `date(json_extract(data, '$.dates.start.localDate')) BETWEEN date('now') AND date('now', ?)`)
				argv = append(argv, fmt.Sprintf("+%d days", window))
			}
			if dmaID != "" {
				conds = append(conds,
					`EXISTS (SELECT 1 FROM json_each(json_extract(data, '$._embedded.venues[0].dmas')) AS d
					         WHERE CAST(json_extract(d.value, '$.id') AS TEXT) = ?)`)
				argv = append(argv, dmaID)
			}
			where := strings.Join(conds, " AND ")

			q := `SELECT data FROM events WHERE ` + where
			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type segmentBuckets struct {
				Segment string         `json:"segment"`
				Bands   map[string]int `json:"bands"`
				Total   int            `json:"total"`
			}
			bySeg := map[string]*segmentBuckets{}

			for rows.Next() {
				var s string
				if err := rows.Scan(&s); err != nil {
					return err
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(s), &obj); err != nil {
					continue
				}
				seg := extractFirstClassification(obj, "segment")
				if seg == "" {
					seg = "Unknown"
				}
				if _, ok := bySeg[seg]; !ok {
					bySeg[seg] = &segmentBuckets{
						Segment: seg,
						Bands: map[string]int{
							"unknown":  0,
							"<$50":     0,
							"$50-100":  0,
							"$100-200": 0,
							"$200+":    0,
						},
					}
				}
				bySeg[seg].Total++
				band := classifyPriceBand(obj)
				bySeg[seg].Bands[band]++
			}

			out := make([]segmentBuckets, 0, len(bySeg))
			for _, v := range bySeg {
				out = append(out, *v)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Total != out[j].Total {
					return out[i].Total > out[j].Total
				}
				return out[i].Segment < out[j].Segment
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No events in window.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SEGMENT\tTOTAL\t<$50\t$50-100\t$100-200\t$200+\tUNKNOWN")
			for _, b := range out {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
					b.Segment, b.Total,
					b.Bands["<$50"], b.Bands["$50-100"], b.Bands["$100-200"], b.Bands["$200+"], b.Bands["unknown"])
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&window, "window", 30, "Window in days from today (0 = no window)")
	cmd.Flags().StringVar(&dmaID, "dma", "", "Restrict to a single DMA ID")
	return cmd
}

func classifyPriceBand(obj map[string]any) string {
	arr, ok := obj["priceRanges"].([]any)
	if !ok || len(arr) == 0 {
		return "unknown"
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		return "unknown"
	}
	min, ok := first["min"].(float64)
	if !ok || min <= 0 {
		return "unknown"
	}
	switch {
	case min < 50:
		return "<$50"
	case min < 100:
		return "$50-100"
	case min < 200:
		return "$100-200"
	default:
		return "$200+"
	}
}
