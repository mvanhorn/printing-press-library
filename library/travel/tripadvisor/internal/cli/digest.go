// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Preserved across regen.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

type digestView struct {
	LocationID    string            `json:"location_id"`
	Details       taDetail          `json:"details"`
	ReviewsNote   string            `json:"reviews_note"`
	Reviews       []json.RawMessage `json:"reviews"`
	Photos        []json.RawMessage `json:"photos"`
	FetchWarnings []string          `json:"fetch_warnings,omitempty"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var (
		reviews  int
		photos   int
		language string
		currency string
	)

	cmd := &cobra.Command{
		Use:   "digest <locationId>",
		Short: "One location ID to a single payload: details + recent reviews + photo URLs",
		Long: "Combine a location's details, its most recent traveler reviews (user-generated content), and " +
			"its photo URLs into one response, so an agent makes a single call instead of three. " +
			"Review and photo counts are hard-capped at 5 by the free Content API.",
		Example: "  tripadvisor-pp-cli digest 93450 --reviews 3 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<locationId>=89575",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("locationId is required"))
			}
			id := args[0]
			esc := url.PathEscape(id)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			view := digestView{
				LocationID:  id,
				ReviewsNote: "User-generated content: review text is written by Tripadvisor travelers, not editorial.",
				Reviews:     []json.RawMessage{},
				Photos:      []json.RawMessage{},
			}

			det, derr := taFetchDetail(cmd.Context(), c, id, language, currency)
			if derr != nil {
				return classifyAPIError(derr, flags)
			}
			view.Details = det

			revParams := map[string]string{"limit": strconv.Itoa(reviews)}
			if language != "" {
				revParams["language"] = language
			}
			if raw, rerr := c.Get(cmd.Context(), "/location/"+esc+"/reviews", revParams); rerr != nil {
				view.FetchWarnings = append(view.FetchWarnings, fmt.Sprintf("reviews: %v", rerr))
			} else {
				view.Reviews = taExtractDataArray(raw)
			}

			photoParams := map[string]string{"limit": strconv.Itoa(photos)}
			if language != "" {
				photoParams["language"] = language
			}
			if raw, perr := c.Get(cmd.Context(), "/location/"+esc+"/photos", photoParams); perr != nil {
				view.FetchWarnings = append(view.FetchWarnings, fmt.Sprintf("photos: %v", perr))
			} else {
				view.Photos = taExtractDataArray(raw)
			}

			// Digest is inherently a structured payload; always emit JSON.
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}

	cmd.Flags().IntVar(&reviews, "reviews", 3, "Recent reviews to include (max 5)")
	cmd.Flags().IntVar(&photos, "photos", 3, "Photos to include (max 5)")
	cmd.Flags().StringVar(&language, "language", "en", "Language code")
	cmd.Flags().StringVar(&currency, "currency", "USD", "ISO 4217 currency for price fields")
	return cmd
}
