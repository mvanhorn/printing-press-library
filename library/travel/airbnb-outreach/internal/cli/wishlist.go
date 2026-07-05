// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newWishlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wishlist",
		Short: "List your wishlists and their saved listings",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWishlistListCmd(flags))
	cmd.AddCommand(newWishlistItemsCmd(flags))
	return cmd
}

func newWishlistListCmd(flags *rootFlags) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List your wishlists",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := newAirbnbClient(flags)
			data, err := c.Wishlists(limit, offset)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return flags.printJSON(cmd, data)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 12, "Number of wishlists to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	return cmd
}

func newWishlistItemsCmd(flags *rootFlags) *cobra.Command {
	var listingType string
	cmd := &cobra.Command{
		Use:     "items [listing-id...]",
		Short:   "Fetch saved-listing details for one or more listing IDs",
		Example: "  airbnb-outreach-pp-cli wishlist items 400704 6913171 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			c := newAirbnbClient(flags)
			data, err := c.WishlistItems(args, listingType)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return flags.printJSON(cmd, data)
		},
	}
	cmd.Flags().StringVar(&listingType, "type", "HOME", "Listing type: HOME or EXPERIENCE")
	return cmd
}
