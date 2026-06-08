package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/acquisition/storefront"
	"github.com/spf13/cobra"
)

func newBrowseCmd(flags *rootFlags) *cobra.Command {
	var flagQ string
	var flagCgid string
	var flagStart int
	var flagSz int
	var flagSrule string
	var flagPmin float64
	var flagPrefn1 string
	var flagPrefv1 string
	var flagPrefn2 string
	var flagPrefv2 string
	var flagDealSort string

	cmd := &cobra.Command{
		Use:         "browse",
		Short:       "Browse category or filtered storefront results",
		Long:        "Browse search or category result fragments with explicit pagination and filter metadata.",
		Example:     "  continente-pp-cli browse --cgid leite --sz 24\n  continente-pp-cli browse --q leite --prefn1=brand --prefv1=Mimosa",
		Annotations: map[string]string{"pp:endpoint": "on.get-search-fragment", "pp:method": "GET", "pp:path": "/on/demandware.store/Sites-continente-Site/default/Search-ShowAjax", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flags.dryRun && flagQ == "" && flagCgid == "" {
				return fmt.Errorf("one of %q or %q must be set", "q", "cgid")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := storefront.SearchParams{
				Query:      flagQ,
				CategoryID: flagCgid,
				Start:      flagStart,
				PageSize:   flagSz,
				SortRule:   flagSrule,
				MinPrice:   flagPmin,
				Prefn1:     flagPrefn1,
				Prefv1:     flagPrefv1,
				Prefn2:     flagPrefn2,
				Prefv2:     flagPrefv2,
			}
			data, err := storefront.FetchSearchFragment(cmd.Context(), c, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			payload, err := parseSearchHTML(flagQ, flagStart, data)
			if err != nil {
				return err
			}
			applyDealSort(payload.Items, flagDealSort)
			enrichSearchResponse(&payload, params)
			rows := make([]map[string]any, 0, len(payload.Items))
			for _, item := range payload.Items {
				rows = append(rows, storefrontItemHumanRow(item))
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live", ResourceType: "search"}, len(payload.Items), rows)
		},
	}
	cmd.Flags().StringVar(&flagQ, "q", "", "Search query")
	cmd.Flags().StringVar(&flagCgid, "cgid", "", "Category identifier")
	cmd.Flags().IntVar(&flagStart, "start", 0, "Result offset")
	cmd.Flags().IntVar(&flagSz, "sz", 0, "Requested page size")
	cmd.Flags().StringVar(&flagSrule, "srule", "", "Storefront sort rule")
	cmd.Flags().Float64Var(&flagPmin, "pmin", 0.0, "Minimum price filter")
	cmd.Flags().StringVar(&flagPrefn1, "prefn1", "", "First storefront refinement name")
	cmd.Flags().StringVar(&flagPrefv1, "prefv1", "", "First storefront refinement value")
	cmd.Flags().StringVar(&flagPrefn2, "prefn2", "", "Second storefront refinement name")
	cmd.Flags().StringVar(&flagPrefv2, "prefv2", "", "Second storefront refinement value")
	cmd.Flags().StringVar(&flagDealSort, "deal-sort", "", "Local comparison sort: unit-price, savings-percent, discount-amount")
	return cmd
}

func enrichSearchResponse(payload *searchResponse, params storefront.SearchParams) {
	if payload == nil {
		return
	}
	payload.PageSize = params.PageSize
	if payload.PageSize == 0 {
		payload.PageSize = payload.Count
	}
	payload.SortRule = params.SortRule
	filters := map[string]string{}
	if params.MinPrice != 0 {
		filters["pmin"] = fmt.Sprintf("%v", params.MinPrice)
	}
	if params.Prefn1 != "" && params.Prefv1 != "" {
		filters[params.Prefn1] = params.Prefv1
	}
	if params.Prefn2 != "" && params.Prefv2 != "" {
		filters[params.Prefn2] = params.Prefv2
	}
	if len(filters) > 0 {
		payload.ActiveFilters = filters
	}
	if params.PageSize > 0 && payload.TotalCount > 0 && payload.Start+payload.Count < payload.TotalCount {
		next := payload.Start + payload.Count
		payload.NextStart = &next
		return
	}
	if params.PageSize > 0 && payload.TotalCount == 0 && payload.Count >= params.PageSize {
		next := payload.Start + payload.Count
		payload.NextStart = &next
	}
}

func applyDealSort(items []storefrontItem, mode string) {
	mode = strings.TrimSpace(mode)
	if len(items) < 2 || mode == "" {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch mode {
		case "unit-price":
			return compareFloatAsc(left.UnitPrice, right.UnitPrice, left.Price, right.Price)
		case "savings-percent":
			return compareFloatDesc(left.SavingsPercent, right.SavingsPercent, left.Price, right.Price)
		case "discount-amount":
			return compareFloatDesc(left.DiscountAmount, right.DiscountAmount, left.Price, right.Price)
		default:
			return false
		}
	})
}

func compareFloatAsc(left, right, leftFallback, rightFallback float64) bool {
	switch {
	case left == 0 && right == 0:
		return leftFallback < rightFallback
	case left == 0:
		return false
	case right == 0:
		return true
	case left == right:
		return leftFallback < rightFallback
	default:
		return left < right
	}
}

func compareFloatDesc(left, right, leftFallback, rightFallback float64) bool {
	switch {
	case left == 0 && right == 0:
		return leftFallback < rightFallback
	case left == 0:
		return false
	case right == 0:
		return true
	case left == right:
		return leftFallback < rightFallback
	default:
		return left > right
	}
}
