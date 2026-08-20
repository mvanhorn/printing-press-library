// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/parser"
)

func newHotelParentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "hotel",
		Short:       "Single-property commands (show, reviews)",
		RunE:        parentNoSubcommandRunE(flags),
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newHotelShowCmd(flags))
	cmd.AddCommand(newHotelReviewsCmd(flags))
	return cmd
}

func newHotelShowCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout, currency, locale string
	cmd := &cobra.Command{
		Use:         "show <property-token>",
		Short:       "Full property detail (description, amenities, prices, images)",
		Example:     strings.Trim("\n  hotel-goat-pp-cli hotel show ChcIyIDJ4cf-7J3eARoKL20vMDJycDRobBAB --checkin 2026-08-15 --checkout 2026-08-17\n", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			token := args[0]
			extra := map[string]string{}
			if currency != "" {
				extra["currency"] = currency
			}
			if locale != "" {
				extra["hl"] = locale
			}
			if cliutil.IsVerifyEnv() {
				wouldURL := fmt.Sprintf("would query: https://www.google.com/travel/hotels/entity/%s", token)
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"meta": map[string]string{"source": wouldURL}, "result": nil}, flags)
			}
			client := &http.Client{Timeout: 45 * time.Second}
			html, err := parser.FetchPropertyDetailHTML(cmd.Context(), client, token, checkin, checkout, extra)
			if err != nil {
				return apiErr(err)
			}
			hotel, err := parser.ParseDetailPage(html)
			if err != nil {
				return apiErr(err)
			}
			env := map[string]any{
				"meta": map[string]any{
					"source":         "https://www.google.com/travel/hotels/entity/" + token,
					"fetched_at":     time.Now().UTC().Format(time.RFC3339),
					"parser_version": parser.ParserVersion,
				},
				"result": hotel,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&currency, "currency", "", "ISO 4217 currency code")
	cmd.Flags().StringVar(&locale, "locale", "en", "Locale")
	return cmd
}

func newHotelReviewsCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout string
	cmd := &cobra.Command{
		Use:         "reviews <property-token>",
		Short:       "Review breakdown for a property (overall rating + per-category)",
		Example:     strings.Trim("\n  hotel-goat-pp-cli hotel reviews ChcIyIDJ4cf-7J3eARoKL20vMDJycDRobBAB\n", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			token := args[0]
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"meta": map[string]string{"source": "would query: " + token}, "result": nil}, flags)
			}
			client := &http.Client{Timeout: 45 * time.Second}
			html, err := parser.FetchPropertyDetailHTML(cmd.Context(), client, token, checkin, checkout, nil)
			if err != nil {
				return apiErr(err)
			}
			hotel, err := parser.ParseDetailPage(html)
			if err != nil {
				return apiErr(err)
			}
			env := map[string]any{
				"meta": map[string]any{
					"source":         "https://www.google.com/travel/hotels/entity/" + token,
					"parser_version": parser.ParserVersion,
				},
				"result": map[string]any{
					"property_token": hotel.PropertyToken,
					"name":           hotel.Name,
					"rating":         hotel.Rating,
					"reviews":        hotel.Reviews,
				},
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date (YYYY-MM-DD)")
	return cmd
}
