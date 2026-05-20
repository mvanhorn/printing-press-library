// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/spf13/cobra"
)

type reviewBucket struct {
	Count       int     `json:"count"`
	MedianScore float64 `json:"median_score"`
}

func newReviewsStatsCmd(flags *rootFlags) *cobra.Command {
	var slug, country, by string
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Group hotel reviews by score band, language, or traveler type",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]reviewBucket{})
			}
			if slug == "" || country == "" || by == "" {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("reviews stats: %w", err)
			}
			groups := map[string][]float64{}
			for page := 1; page <= 5; page++ {
				data, err := c.Get("/reviewlist.html", map[string]string{"pagename": slug, "cc1": country, "page": strconv.Itoa(page), "rows": "25"})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: review page %d failed: %v\n", page, err)
					continue
				}
				parsed, err := booking.ParseReviewList(data)
				if err != nil {
					return fmt.Errorf("reviews stats: %w", err)
				}
				reviews := make([]booking.Review, 0)
				if err := json.Unmarshal(parsed, &reviews); err != nil {
					return fmt.Errorf("reviews stats: %w", err)
				}
				for _, r := range reviews {
					groups[reviewGroupKey(r, by)] = append(groups[reviewGroupKey(r, by)], r.Score)
				}
			}
			out := map[string]reviewBucket{}
			for key, vals := range groups {
				out[key] = reviewBucket{Count: len(vals), MedianScore: medianFloat(vals)}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "Hotel slug")
	cmd.Flags().StringVar(&country, "country", "", "Hotel country code")
	cmd.Flags().StringVar(&by, "by", "score-band", "Group by score-band, language, or traveler-type")
	return cmd
}

func reviewGroupKey(r booking.Review, by string) string {
	switch by {
	case "language":
		return firstNonEmptyString(r.Language, "unknown")
	case "traveler-type":
		return firstNonEmptyString(r.TravelerType, "unknown")
	default:
		switch {
		case r.Score >= 9:
			return "9-10"
		case r.Score >= 8:
			return "8-8.9"
		case r.Score >= 6:
			return "6-7.9"
		case r.Score > 0:
			return "0-5.9"
		default:
			return "unknown"
		}
	}
}
