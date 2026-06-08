package cli

import (
	"fmt"
	"strings"

	"continente-pp-cli/internal/acquisition/storefront"
	"github.com/spf13/cobra"
)

type compareResponse struct {
	Left    productResponse `json:"left"`
	Right   productResponse `json:"right"`
	Summary []string        `json:"summary,omitempty"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare <leftSlugAndPid> <rightSlugAndPid>",
		Short:       "Compare two products side by side",
		Long:        "Fetch two product detail pages and return side-by-side pricing and comparison deltas.",
		Example:     "  continente-pp-cli compare leite-uht-meio-gordo-mimosa-mimosa-7745833 leite-uht-meio-gordo-bem-essencial-mimosa-mimosa-4421406",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return usageErr(fmt.Errorf("leftSlugAndPid and rightSlugAndPid are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			leftData, err := storefront.FetchProduct(cmd.Context(), c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rightData, err := storefront.FetchProduct(cmd.Context(), c, args[1])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"left":  string(leftData),
					"right": string(rightData),
				}, flags)
			}
			left, err := parseProductHTML(args[0], leftData)
			if err != nil {
				return err
			}
			if err := fetchAndAttachNutrition(cmd.Context(), c, &left); err != nil {
				return classifyAPIError(err, flags)
			}
			right, err := parseProductHTML(args[1], rightData)
			if err != nil {
				return err
			}
			if err := fetchAndAttachNutrition(cmd.Context(), c, &right); err != nil {
				return classifyAPIError(err, flags)
			}
			payload := compareResponse{
				Left:    left,
				Right:   right,
				Summary: compareProductsSummary(left, right),
			}
			rows := []map[string]any{
				productCompareRow("left", left),
				productCompareRow("right", right),
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live", ResourceType: "comparison"}, 2, rows)
		},
	}
	return cmd
}

func compareProductsSummary(left, right productResponse) []string {
	var out []string
	if left.UnitPrice != 0 && right.UnitPrice != 0 && left.UnitLabel != "" && strings.EqualFold(left.UnitLabel, right.UnitLabel) {
		diff := roundMoney(right.UnitPrice - left.UnitPrice)
		switch {
		case diff < 0:
			out = append(out, fmt.Sprintf("right is %.2f EUR/%s cheaper", -diff, right.UnitLabel))
		case diff > 0:
			out = append(out, fmt.Sprintf("right is %.2f EUR/%s more expensive", diff, right.UnitLabel))
		}
	}
	if samePackLabel(left.PackLabel, right.PackLabel) && left.Price != 0 && right.Price != 0 {
		diff := roundMoney(right.Price - left.Price)
		switch {
		case diff < 0:
			out = append(out, fmt.Sprintf("right has %.2f EUR lower upfront price", -diff))
		case diff > 0:
			out = append(out, fmt.Sprintf("right has %.2f EUR higher upfront price", diff))
		}
	} else if left.PackLabel != "" && right.PackLabel != "" && !samePackLabel(left.PackLabel, right.PackLabel) {
		out = append(out, "different pack sizes")
	}
	if left.SavingsPercent != 0 || right.SavingsPercent != 0 {
		diff := roundMoney(right.SavingsPercent - left.SavingsPercent)
		switch {
		case diff > 0:
			out = append(out, fmt.Sprintf("right has %.1f pp higher savings", diff))
		case diff < 0:
			out = append(out, fmt.Sprintf("right has %.1f pp lower savings", -diff))
		}
	}
	out = append(out, nutritionSummary(nutritionFactsForCompare(left), nutritionFactsForCompare(right))...)
	if len(out) == 0 {
		out = append(out, "no strong pricing delta detected")
	}
	return out
}

func nutritionFactsForCompare(product productResponse) *nutritionFacts {
	if product.Nutrition == nil {
		return nil
	}
	if product.Nutrition.Per100g != nil {
		return product.Nutrition.Per100g
	}
	return product.Nutrition.PerServing
}

func productCompareRow(side string, product productResponse) map[string]any {
	row := productResponseHumanRow(product)
	row["side"] = side
	return row
}
