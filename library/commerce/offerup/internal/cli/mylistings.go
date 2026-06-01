// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

// pp:data-source live
func newMyListingsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "my-listings",
		Short:       "List your own active OfferUp listings (requires login)",
		Example:     "  offerup-pp-cli my-listings --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthRead(cmd, flags, []any{}, func() (any, error) {
				return newOfferupClient(flags).MyListings(cmd.Context(), limit)
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum listings to return")
	cmd.AddCommand(newMyListingsArchivedCmd(flags), newMyListingsMarkSoldCmd(flags), newMyListingsArchiveCmd(flags))
	return cmd
}

// pp:data-source live
func newMyListingsArchivedCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "archived",
		Short:       "List your archived/sold OfferUp listings",
		Example:     "  offerup-pp-cli my-listings archived --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthRead(cmd, flags, []any{}, func() (any, error) {
				return newOfferupClient(flags).ArchivedListings(cmd.Context())
			})
		},
	}
}

func newMyListingsMarkSoldCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:     "mark-sold <listing-id>",
		Short:   "Mark one of your listings as sold",
		Example: "  offerup-pp-cli my-listings mark-sold 1838048755 --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyListingMutation(cmd, flags, args, confirm, "mark-sold",
				func(c *offerup.Client, ctx context.Context, id int64) (any, error) { return c.MarkSold(ctx, id) })
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually apply the change (omit to preview without mutating)")
	return cmd
}

func newMyListingsArchiveCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:     "archive <listing-id>",
		Short:   "Archive one of your listings",
		Example: "  offerup-pp-cli my-listings archive 1838048755 --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyListingMutation(cmd, flags, args, confirm, "archive",
				func(c *offerup.Client, ctx context.Context, id int64) (any, error) { return c.Archive(ctx, id) })
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually apply the change (omit to preview without mutating)")
	return cmd
}

// applyListingMutation is the shared RunE body for the listing-write commands:
// parse the numeric id, preview by default, and apply only with confirm. It
// short-circuits under verify-mode so the verifier never mutates a real listing.
func applyListingMutation(cmd *cobra.Command, flags *rootFlags, args []string, confirm bool, action string, apply func(*offerup.Client, context.Context, int64) (any, error)) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	if dryRunOK(flags) {
		return nil
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return usageErr(fmt.Errorf("listing id must be numeric, got %q", args[0]))
	}
	if cliutil.IsVerifyEnv() || !confirm {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"action": action, "listingId": id, "applied": false,
			"hint": "re-run with --confirm to apply",
		}, flags)
	}
	v, err := apply(newOfferupClient(flags), cmd.Context(), id)
	if err != nil {
		return classifyOfferupError(err)
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": action, "listingId": id, "applied": true, "result": v}, flags)
}
