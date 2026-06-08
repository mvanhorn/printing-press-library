package cli

import (
	"fmt"
	"sort"
	"strings"

	"continente-pp-cli/internal/acquisition/storefront"
	"github.com/spf13/cobra"
)

type alternativesResponse struct {
	Query        string                 `json:"query,omitempty"`
	Seed         productResponse        `json:"seed"`
	Count        int                    `json:"count"`
	Alternatives []alternativeCandidate `json:"alternatives"`
}

type alternativeCandidate struct {
	storefrontItem
	SimilarityScore     float64  `json:"similarity_score"`
	SamePack            bool     `json:"same_pack,omitempty"`
	PriceDelta          float64  `json:"price_delta,omitempty"`
	UnitPriceDelta      float64  `json:"unit_price_delta,omitempty"`
	SavingsPercentDelta float64  `json:"savings_percent_delta,omitempty"`
	BetterDeal          bool     `json:"better_deal,omitempty"`
	MatchReasons        []string `json:"match_reasons,omitempty"`
	ComparisonSummary   []string `json:"comparison_summary,omitempty"`
}

func newAlternativesCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagDealSort string

	cmd := &cobra.Command{
		Use:         "alternatives <slugAndPid>",
		Aliases:     nil,
		Short:       "Find comparable alternatives for a product",
		Long:        "Fetch a product detail page, search the storefront for comparable products, and rank alternatives by similarity plus pricing signals.",
		Example:     "  continente-pp-cli alternatives leite-uht-meio-gordo-mimosa-mimosa-7745833 --limit 5 --deal-sort unit-price",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("slugAndPid is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			productData, err := storefront.FetchProduct(cmd.Context(), c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				return printOutputWithFlags(cmd.OutOrStdout(), productData, flags)
			}
			seed, err := parseProductHTML(args[0], productData)
			if err != nil {
				return err
			}
			query := alternativesQuery(seed)
			searchData, err := storefront.FetchSearchFragment(cmd.Context(), c, storefront.SearchParams{
				Query:    query,
				PageSize: maxInt(flagLimit*3, 12),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			searchPayload, err := parseSearchHTML(query, 0, searchData)
			if err != nil {
				return err
			}
			candidates := buildAlternatives(seed, searchPayload.Items, flagDealSort, flagLimit)
			payload := alternativesResponse{
				Query:        query,
				Seed:         seed,
				Count:        len(candidates),
				Alternatives: candidates,
			}
			rows := make([]map[string]any, 0, len(candidates))
			for _, candidate := range candidates {
				row := storefrontItemHumanRow(candidate.storefrontItem)
				row["similarity_score"] = candidate.SimilarityScore
				if candidate.PriceDelta != 0 {
					row["price_delta"] = candidate.PriceDelta
				}
				if candidate.SamePack {
					row["same_pack"] = true
				}
				if candidate.UnitPriceDelta != 0 {
					row["unit_price_delta"] = candidate.UnitPriceDelta
				}
				if candidate.SavingsPercentDelta != 0 {
					row["savings_percent_delta"] = candidate.SavingsPercentDelta
				}
				if candidate.BetterDeal {
					row["better_deal"] = true
				}
				if len(candidate.MatchReasons) > 0 {
					row["match_reasons"] = strings.Join(candidate.MatchReasons, " | ")
				}
				if len(candidate.ComparisonSummary) > 0 {
					row["comparison_summary"] = strings.Join(candidate.ComparisonSummary, " | ")
				}
				rows = append(rows, row)
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live", ResourceType: "alternatives"}, len(candidates), rows)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 8, "Maximum alternatives to return")
	cmd.Flags().StringVar(&flagDealSort, "deal-sort", "", "Deal tiebreaker: unit-price, savings-percent, discount-amount")
	return cmd
}

func buildAlternatives(seed productResponse, items []storefrontItem, dealSort string, limit int) []alternativeCandidate {
	if limit <= 0 {
		limit = 8
	}
	out := make([]alternativeCandidate, 0, len(items))
	for _, item := range items {
		if item.ID == seed.ID {
			continue
		}
		score, reasons := alternativeScore(seed, item)
		if score <= 0 {
			continue
		}
		out = append(out, alternativeCandidate{
			storefrontItem:      item,
			SimilarityScore:     score,
			SamePack:            samePackLabel(seed.PackLabel, item.PackLabel),
			PriceDelta:          roundMoney(item.Price - seed.Price),
			UnitPriceDelta:      comparisonUnitPriceDelta(seed, item),
			SavingsPercentDelta: roundMoney(item.SavingsPercent - seed.SavingsPercent),
			BetterDeal:          isBetterDealThanSeed(seed, item),
			MatchReasons:        reasons,
			ComparisonSummary:   buildComparisonSummary(seed, item),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SimilarityScore != out[j].SimilarityScore {
			return out[i].SimilarityScore > out[j].SimilarityScore
		}
		return compareAlternativeDeal(out[i].storefrontItem, out[j].storefrontItem, dealSort)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func alternativeScore(seed productResponse, item storefrontItem) (float64, []string) {
	var score float64
	var reasons []string

	if seed.Category != "" && item.Category == seed.Category {
		score += 4
		reasons = append(reasons, "same category")
	} else if sameTopCategory(seed.Categories, item.Categories) {
		score += 2
		reasons = append(reasons, "same family")
	}
	if seed.UnitLabel != "" && item.UnitLabel != "" && strings.EqualFold(seed.UnitLabel, item.UnitLabel) {
		score += 2
		reasons = append(reasons, "same unit")
	}
	if seed.Brand != "" && item.Brand != "" && strings.EqualFold(seed.Brand, item.Brand) {
		score += 1
		reasons = append(reasons, "same brand")
	}
	overlap := overlapCount(alternativeTokens(seed.Name, seed.Brand), alternativeTokens(item.Name, item.Brand))
	if overlap > 0 {
		score += float64(overlap)
		reasons = append(reasons, fmt.Sprintf("name overlap x%d", overlap))
	}
	if item.HasDiscount {
		score += 0.2
		reasons = append(reasons, "active discount")
	}
	return score, reasons
}

func compareAlternativeDeal(left, right storefrontItem, mode string) bool {
	switch mode {
	case "unit-price":
		return compareFloatAsc(left.UnitPrice, right.UnitPrice, left.Price, right.Price)
	case "savings-percent":
		return compareFloatDesc(left.SavingsPercent, right.SavingsPercent, left.Price, right.Price)
	case "discount-amount":
		return compareFloatDesc(left.DiscountAmount, right.DiscountAmount, left.Price, right.Price)
	default:
		if left.UnitPrice != 0 || right.UnitPrice != 0 {
			return compareFloatAsc(left.UnitPrice, right.UnitPrice, left.Price, right.Price)
		}
		return left.Price < right.Price
	}
}

func comparisonUnitPriceDelta(seed productResponse, item storefrontItem) float64 {
	if seed.UnitPrice == 0 || item.UnitPrice == 0 {
		return 0
	}
	return roundMoney(item.UnitPrice - seed.UnitPrice)
}

func isBetterDealThanSeed(seed productResponse, item storefrontItem) bool {
	switch {
	case seed.UnitPrice != 0 && item.UnitPrice != 0 && item.UnitLabel != "" && strings.EqualFold(seed.UnitLabel, item.UnitLabel):
		if item.UnitPrice != seed.UnitPrice {
			return item.UnitPrice < seed.UnitPrice
		}
		return item.SavingsPercent > seed.SavingsPercent
	case item.Price != 0 && seed.Price != 0:
		if item.Price != seed.Price {
			return item.Price < seed.Price
		}
		return item.SavingsPercent > seed.SavingsPercent
	default:
		return item.SavingsPercent > seed.SavingsPercent
	}
}

func buildComparisonSummary(seed productResponse, item storefrontItem) []string {
	var out []string
	samePack := samePackLabel(seed.PackLabel, item.PackLabel)
	if !samePack && seed.PackLabel != "" && item.PackLabel != "" {
		out = append(out, "different pack size")
	}
	if seed.UnitPrice != 0 && item.UnitPrice != 0 && item.UnitLabel != "" && strings.EqualFold(seed.UnitLabel, item.UnitLabel) {
		diff := roundMoney(item.UnitPrice - seed.UnitPrice)
		switch {
		case diff < 0:
			out = append(out, fmt.Sprintf("%.2f EUR/%s cheaper than seed", -diff, item.UnitLabel))
		case diff > 0:
			out = append(out, fmt.Sprintf("%.2f EUR/%s more expensive than seed", diff, item.UnitLabel))
		}
	}
	if samePack && seed.Price != 0 && item.Price != 0 {
		diff := roundMoney(item.Price - seed.Price)
		switch {
		case diff < 0:
			out = append(out, fmt.Sprintf("%.2f EUR lower upfront price", -diff))
		case diff > 0:
			out = append(out, fmt.Sprintf("%.2f EUR higher upfront price", diff))
		}
	}
	if item.SavingsPercent != 0 || seed.SavingsPercent != 0 {
		diff := roundMoney(item.SavingsPercent - seed.SavingsPercent)
		switch {
		case diff > 0:
			out = append(out, fmt.Sprintf("%.1f pp higher savings", diff))
		case diff < 0:
			out = append(out, fmt.Sprintf("%.1f pp lower savings", -diff))
		}
	}
	if len(out) == 0 && item.HasDiscount && !seed.HasDiscount {
		out = append(out, "candidate has active discount")
	}
	return out
}

func samePackLabel(left, right string) bool {
	left = strings.TrimSpace(strings.ToLower(left))
	right = strings.TrimSpace(strings.ToLower(right))
	if left == "" || right == "" {
		return left == right
	}
	return left == right
}

func alternativesQuery(seed productResponse) string {
	query := strings.TrimSpace(seed.Name)
	if query != "" {
		return query
	}
	return strings.TrimSpace(seed.Category)
}

func sameTopCategory(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	return strings.EqualFold(left[0], right[0])
}

func alternativeTokens(values ...string) []string {
	seen := map[string]bool{}
	var out []string
	stopwords := map[string]bool{
		"de": true, "da": true, "do": true, "e": true, "com": true,
		"produto": true, "pack": true, "emb": true,
	}
	for _, value := range values {
		value = strings.ToLower(value)
		value = strings.NewReplacer("-", " ", "/", " ", ",", " ", ".", " ").Replace(value)
		for _, token := range strings.Fields(value) {
			if len(token) < 3 || stopwords[token] || seen[token] {
				continue
			}
			seen[token] = true
			out = append(out, token)
		}
	}
	return out
}

func overlapCount(left, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, token := range left {
		seen[token] = true
	}
	count := 0
	for _, token := range right {
		if seen[token] {
			count++
		}
	}
	if count > 4 {
		return 4
	}
	return count
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
