// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type citeView struct {
	FdcID       int    `json:"fdc_id"`
	Description string `json:"description"`
	Style       string `json:"style"`
	Citation    string `json:"citation"`
	URL         string `json:"url"`
}

func newNovelCiteCmd(flags *rootFlags) *cobra.Command {
	var flagStyle string

	cmd := &cobra.Command{
		Use:   "cite <fdcId>",
		Short: "Emit an APA or MLA citation for a USDA food record",
		Long: "Emit an APA or MLA citation for a USDA FoodData Central food record so agent " +
			"output is verifiable and traceable to a real FDC id.",
		Example:     "  nutrition-pp-cli cite 173414 --style apa",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would fetch a USDA food and format a citation")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an FDC id is required, e.g. cite 173414"))
			}
			style := flagStyle
			if style == "" {
				style = "apa"
			}
			if style != "apa" && style != "mla" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--style must be 'apa' or 'mla'"))
			}
			fdcID := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			food, _, err := fetchUSDAFood(cmd.Context(), c, fdcID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			year := citationYear(food.PublishedAt)
			retrieved := time.Now().Format("January 2, 2006")
			url := fmt.Sprintf("https://fdc.nal.usda.gov/fdc-app.html#/food-details/%d/nutrients", food.FdcID)
			var citation string
			switch style {
			case "mla":
				citation = fmt.Sprintf("U.S. Department of Agriculture, Agricultural Research Service. %q FoodData Central, %s, %s. Accessed %s.",
					food.Description+".", year, url, retrieved)
			default: // apa
				citation = fmt.Sprintf("U.S. Department of Agriculture, Agricultural Research Service. (%s). %s (FDC ID: %d) [Data set]. FoodData Central. %s. Retrieved %s.",
					year, food.Description, food.FdcID, url, retrieved)
			}

			view := citeView{
				FdcID:       food.FdcID,
				Description: food.Description,
				Style:       style,
				Citation:    citation,
				URL:         url,
			}
			// Human output is just the citation line; JSON/agent gets the envelope.
			if !flags.asJSON && !flags.agent && !flags.compact && flags.selectFields == "" && isTerminal(cmd.OutOrStdout()) {
				fmt.Fprintln(cmd.OutOrStdout(), citation)
				return nil
			}
			return emitNutritionJSON(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagStyle, "style", "apa", "Citation style: apa or mla")
	return cmd
}

// citationYear extracts a 4-digit year from USDA's date fields, which appear in
// both "M/D/YYYY" (SR Legacy publicationDate) and "YYYY-MM-DD" forms.
func citationYear(published string) string {
	published = strings.TrimSpace(published)
	if published == "" {
		return "n.d."
	}
	// ISO: YYYY-MM-DD -> leading 4 digits.
	if len(published) >= 4 && isAllDigits(published[:4]) {
		return published[:4]
	}
	// Slash form: M/D/YYYY -> last segment.
	if idx := strings.LastIndex(published, "/"); idx >= 0 {
		tail := published[idx+1:]
		if len(tail) == 4 && isAllDigits(tail) {
			return tail
		}
	}
	return "n.d."
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
