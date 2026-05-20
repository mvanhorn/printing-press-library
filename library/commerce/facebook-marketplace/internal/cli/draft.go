package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/spf13/cobra"
)

func newDraftCmd(flags *rootFlags) *cobra.Command {
	var photos []string
	var notes string
	var targetPrice float64
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Draft Marketplace listing copy from photos and notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if notes == "" && len(photos) == 0 {
				return cmd.Help()
			}
			comparableCount := 0
			if db, err := openLocalDB(); err == nil {
				_ = db.QueryRow(`SELECT COUNT(*) FROM listings WHERE title LIKE ?`, "%"+draftComparableNeedle(notes)+"%").Scan(&comparableCount)
				_ = db.Close()
			}
			title := draftTitle(notes, photos)
			price := "research comparable listings before pricing"
			if targetPrice > 0 {
				price = fmt.Sprintf("$%.2f", targetPrice)
			}
			out := map[string]any{
				"title":             title,
				"description":       draftDescription(notes, photos),
				"price_suggestion":  price,
				"local_comparables": comparableCount,
				"photo_count":       len(photos),
				"write_required":    false,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\n\nDescription:\n%s\n\nPrice suggestion: %s\n", out["title"], out["description"], out["price_suggestion"])
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&photos, "photos", nil, "Photo paths to consider")
	cmd.Flags().StringVar(&notes, "notes", "", "Seller notes about the item")
	cmd.Flags().Float64Var(&targetPrice, "target-price", 0, "Optional target price in dollars")
	return cmd
}

func draftComparableNeedle(notes string) string {
	words := strings.Fields(notes)
	if len(words) == 0 {
		return ""
	}
	return words[0]
}

func draftTitle(notes string, photos []string) string {
	words := strings.Fields(notes)
	if len(words) > 7 {
		words = words[:7]
	}
	if len(words) > 0 {
		return toTitleCase(strings.Join(words, " "))
	}
	if len(photos) > 0 {
		base := strings.TrimSuffix(filepath.Base(photos[0]), filepath.Ext(photos[0]))
		return toTitleCase(strings.ReplaceAll(base, "-", " "))
	}
	return "Marketplace Listing"
}

func toTitleCase(value string) string {
	return cases.Title(language.Und).String(value)
}

func draftDescription(notes string, photos []string) string {
	parts := []string{}
	if notes != "" {
		parts = append(parts, notes)
	}
	if len(photos) > 0 {
		parts = append(parts, fmt.Sprintf("Includes %d photo(s) for review.", len(photos)))
	}
	parts = append(parts, "Pickup details, measurements, condition notes, and any defects should be confirmed before posting.")
	return strings.Join(parts, "\n\n")
}
