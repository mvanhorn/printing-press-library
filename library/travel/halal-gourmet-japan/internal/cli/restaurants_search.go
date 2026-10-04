// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/hgj"
	"github.com/spf13/cobra"
)

func newRestaurantsSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "search"}
	options := &hgj.SearchOptions{}
	cmd.Flags().StringVar(&options.Query, "query", "", "Keyword from the source search field, at most 200 characters")
	cmd.Flags().StringVar(&options.Prefecture, "prefecture", "", "Japanese prefecture name, such as Tokyo, Kyoto or Osaka")
	cmd.Flags().IntVar(&options.Limit, "limit", 10, "Maximum source cards returned, from 1 to 50")
	cmd.Flags().StringVar(&options.Genre, "genre", "", "Exact source cuisine label, such as Ramen or Japanese")
	cmd.Flags().StringSliceVar(&options.Features, "feature", nil, "Source food conditions, repeated or comma-separated; use plan match for every requirement")
	return configureHGJSearchCmd(cmd, flags, hgj.Restaurant, options)
}
