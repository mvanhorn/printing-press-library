// `listing get`, `listing get-by-pid`, `listing images` — typed read access
// against rapi.craigslist.org. The PID lookup is necessarily a search-then-fetch
// round-trip because rapi has no by-PID endpoint; the user's --site is the
// search scope, with a hint to widen if the PID is not in the default city.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"

	"github.com/spf13/cobra"
)

func newListingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "listing",
		Short:       "Fetch a single listing by UUID or posting ID",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newListingGetCmd(flags))
	cmd.AddCommand(newListingGetByPIDCmd(flags))
	cmd.AddCommand(newListingImagesCmd(flags))
	return cmd
}

func newListingGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get [uuid]",
		Short:       "Fetch a listing detail by UUID",
		Long:        "Fetch the full listing payload (title, body, images, attributes) from rapi.craigslist.org for a known UUID.",
		Example:     "  craigslist-pp-cli listing get xks8RmxNUYD2vVYqkMJ9C6 --json\n  craigslist-pp-cli listing get xks8RmxNUYD2vVYqkMJ9C6 --json --select name,price,images",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			uuid := strings.TrimSpace(args[0])
			c := craigslist.New(1.0)
			d, err := c.GetListing(cmd.Context(), uuid)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), d, flags)
		},
	}
	return cmd
}

func newListingGetByPIDCmd(flags *rootFlags) *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:         "get-by-pid [pid]",
		Short:       "Fetch a listing detail by posting ID (slower)",
		Long:        "Resolve a posting ID to its UUID via a sapi search (scoped to --site), then fetch the detail. There is no native rapi by-PID endpoint, so this round-trips.",
		Example:     "  craigslist-pp-cli listing get-by-pid 7915891289 --json\n  craigslist-pp-cli listing get-by-pid 7915891289 --site nyc",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pid, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pid %q: %w", args[0], err)
			}
			c := craigslist.New(1.0)
			ctx := cmd.Context()
			sr, err := c.Search(ctx, site, craigslist.SearchQuery{Query: strconv.FormatInt(pid, 10)})
			if err != nil {
				return err
			}
			var matchUUID string
			for _, item := range sr.Items {
				if item.PostingID == pid {
					matchUUID = item.UUID
					break
				}
			}
			if matchUUID == "" {
				return fmt.Errorf("pid %d not found on site %q — try --site=<otherSite>", pid, site)
			}
			d, err := c.GetListing(ctx, matchUUID)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), d, flags)
		},
	}
	cmd.Flags().StringVar(&site, "site", "sfbay", "Site to search when resolving the PID")
	return cmd
}

func newListingImagesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "images [uuid]",
		Short:       "List the image IDs for a listing",
		Long:        "Return just the images array from a listing's detail payload.",
		Example:     "  craigslist-pp-cli listing images xks8RmxNUYD2vVYqkMJ9C6 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			uuid := strings.TrimSpace(args[0])
			c := craigslist.New(1.0)
			d, err := c.GetListing(cmd.Context(), uuid)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), d.Images, flags)
		},
	}
	return cmd
}
