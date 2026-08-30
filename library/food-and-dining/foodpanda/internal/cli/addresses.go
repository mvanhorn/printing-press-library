// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored command: list your saved foodpanda delivery addresses.
//
// This is deliberately NOT a spec resource. Modelling it as one made `sync`
// treat it as syncable, and sync resolves resource paths against the ROOT
// base_url rather than the resource's base_url override — so it requested
// disco.deliveryhero.io/api/v5/customers/addresses and got a Cloudflare 530.
// Addresses are auth-only, tiny, and pointless to mirror offline, so they ship
// as a direct command instead.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type fpAddressRow struct {
	ID        float64 `json:"id"`
	Label     string  `json:"label,omitempty"`
	City      string  `json:"city"`
	Line1     string  `json:"address_line1,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	IsDefault bool    `json:"is_default"`
}

type fpAddressView struct {
	Addresses []fpAddressRow `json:"addresses"`
	Count     int            `json:"count"`
	Note      string         `json:"note,omitempty"`
}

func newNovelAddressesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addresses",
		Short: "List your saved foodpanda delivery addresses with coordinates.",
		Long: "List the delivery addresses saved on your foodpanda account.\n\n" +
			"Requires a session: run 'foodpanda-pp-cli auth login --chrome' first.\n" +
			"The coordinates here are what 'home' uses to scope its vendor sweep.",
		Example:     "  foodpanda-pp-cli addresses --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "session"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "addresses")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			addrs, err := fpFetchAddresses(ctx, c)
			if err != nil {
				return authErr(fmt.Errorf("could not read saved addresses (run 'foodpanda-pp-cli auth login --chrome'): %w", err))
			}
			rows := make([]fpAddressRow, 0, len(addrs))
			for _, a := range addrs {
				rows = append(rows, fpAddressRow{
					ID: a.ID, Label: a.Label, City: a.City, Line1: a.AddressLine1,
					Latitude: a.Latitude, Longitude: a.Longitude, IsDefault: a.IsDefault,
				})
			}
			view := fpAddressView{Addresses: rows, Count: len(rows)}
			if len(rows) == 0 {
				view.Note = "no saved addresses on this account"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				def := "-"
				if r.IsDefault {
					def = "yes"
				}
				out = append(out, []string{
					truncate(orDefault(r.Label, r.Line1), 28), truncate(r.City, 16),
					fmt.Sprintf("%.5f", r.Latitude), fmt.Sprintf("%.5f", r.Longitude), def,
				})
			}
			return flags.printTable(cmd, []string{"ADDRESS", "CITY", "LAT", "LNG", "DEFAULT"}, out)
		},
	}
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelAddressesCmd(flags))
	})
}
