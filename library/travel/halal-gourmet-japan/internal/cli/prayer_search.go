// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/hgj"
	"github.com/spf13/cobra"
)

func newPrayerSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "search"}
	options := &hgj.SearchOptions{}
	cmd.Flags().StringVar(&options.Query, "query", "", "Keyword from the source search field, at most 200 characters")
	cmd.Flags().StringVar(&options.Prefecture, "prefecture", "", "Japanese prefecture name, such as Tokyo, Kyoto or Osaka")
	cmd.Flags().IntVar(&options.Limit, "limit", 10, "Maximum source cards returned, from 1 to 50")
	cmd.Flags().StringVar(&options.PlaceType, "place-type", "", "Prayer-place type: spaces or mosques; omit for both")
	cmd.Flags().StringSliceVar(&options.PrayerFeatures, "prayer-feature", nil, "Source facility keys wudu, wifi, hotWater or qibla, repeated or comma-separated")
	return configureHGJSearchCmd(cmd, flags, hgj.Prayer, options)
}
