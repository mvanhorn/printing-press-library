// Hand-authored transcendence: aggregate a designer's catalog locally.
// pp:data-source live
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/cliutil"
	"github.com/spf13/cobra"
)

type designerProfile struct {
	Designer    string       `json:"designer"`
	DesignerID  int          `json:"designer_id"`
	Total       int          `json:"total_products"`
	Scanned     int          `json:"scanned"`
	FreeCount   int          `json:"free_count"`
	PodCount    int          `json:"pod_count"`
	OnSaleCount int          `json:"on_sale_count"`
	MinPrice    float64      `json:"min_price"`
	MaxPrice    float64      `json:"max_price"`
	MedianPrice float64      `json:"median_price"`
	TypeMix     []facetCount `json:"type_mix"`
	NewestName  string       `json:"newest_product,omitempty"`
	NewestDate  int64        `json:"newest_date,omitempty"`
}

// profileDesigner fetches and aggregates a designer's catalog.
func profileDesigner(cmd *cobra.Command, flags *rootFlags, designer string, maxScanPages int) (designerProfile, error) {
	// Curtail pagination under live-dogfood so the matrix doesn't exhaust the
	// shared public search key's rate limit (one page is enough to prove the path).
	if cliutil.IsDogfoodEnv() && maxScanPages > 1 {
		maxScanPages = 1
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c := newAlgoliaClient(flags)
	hits, nbHits, err := fetchAllForDesigner(ctx, c, designer, maxScanPages)
	if err != nil {
		return designerProfile{}, apiErr(err)
	}
	p := designerProfile{Designer: designer, Total: nbHits, Scanned: len(hits)}
	if len(hits) == 0 {
		return p, nil
	}
	typeCounts := map[string]int{}
	var prices []float64
	p.MinPrice = -1
	for _, h := range hits {
		typeCounts[h.Type]++
		if h.IsFree {
			p.FreeCount++
		}
		if h.HasPod {
			p.PodCount++
		}
		if h.HasPromotions {
			p.OnSaleCount++
		}
		if h.Designer.DesignerID != 0 {
			p.DesignerID = h.Designer.DesignerID
		}
		if h.Designer.DesignerName != "" {
			p.Designer = h.Designer.DesignerName
		}
		if !h.IsFree {
			price := h.Price.Float()
			prices = append(prices, price)
			if p.MinPrice < 0 || price < p.MinPrice {
				p.MinPrice = price
			}
			if price > p.MaxPrice {
				p.MaxPrice = price
			}
		}
		if h.Date > p.NewestDate {
			p.NewestDate = h.Date
			p.NewestName = h.NameEN
		}
	}
	if p.MinPrice < 0 {
		p.MinPrice = 0
	}
	p.MedianPrice = median(prices)
	for t, n := range typeCounts {
		p.TypeMix = append(p.TypeMix, facetCount{Value: t, Count: n})
	}
	sort.Slice(p.TypeMix, func(i, j int) bool { return p.TypeMix[i].Count > p.TypeMix[j].Count })
	return p, nil
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return round2(s[n/2])
	}
	return round2((s[n/2-1] + s[n/2]) / 2)
}

func newNovelDesignerStatsCmd(flags *rootFlags) *cobra.Command {
	var maxScanPages int
	cmd := &cobra.Command{
		Use:   "designer-stats <id|name>",
		Short: "Profile a designer's catalog: type mix, price band, free/POD counts, newest drop",
		Long: `Aggregate a single designer's catalog into a one-shot profile.

For the raw product list use 'designer'; to compare two designers use
'designer-compare'.`,
		Example:     strings.Trim("\n  creativefabrica-pp-cli designer-stats \"DigiArt\" --agent\n  creativefabrica-pp-cli designer-stats 2880714", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("designer id or name is required"))
			}
			p, err := profileDesigner(cmd, flags, args[0], maxScanPages)
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, p)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Designer: %s (id %d)\n", p.Designer, p.DesignerID)
			fmt.Fprintf(out, "Catalog:  %d total, %d scanned\n", p.Total, p.Scanned)
			fmt.Fprintf(out, "Free:     %d   POD: %d   On sale: %d\n", p.FreeCount, p.PodCount, p.OnSaleCount)
			fmt.Fprintf(out, "Price:    $%.2f–$%.2f (median $%.2f)\n", p.MinPrice, p.MaxPrice, p.MedianPrice)
			var mix []string
			for _, t := range p.TypeMix {
				mix = append(mix, fmt.Sprintf("%s %d", t.Value, t.Count))
			}
			fmt.Fprintf(out, "Types:    %s\n", strings.Join(mix, ", "))
			if p.NewestName != "" {
				fmt.Fprintf(out, "Newest:   %s\n", p.NewestName)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Max catalog pages to scan (100/page)")
	return cmd
}
