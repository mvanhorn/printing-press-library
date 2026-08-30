// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// venues get — single listing: local store first, optional live GET /v1/listings/{id}.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newVenuesGetCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagForce bool

	cmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Lookup a listing from local store, or live-fetch full detail (GET /v1/listings/{id}) and cache it.",
		Long: `Get one listing by id.

Default (--data-source auto): use SQLite if present; if missing or --force, live-fetch
GET /v1/listings/{id} (full page blocks: description, rules, parking, amenities) and
write through to the local store.

Use --data-source local for offline-only. Use --data-source live to always hit the API.`,
		Example:     "  peerspace-pp-cli venues get demo-listing --dry-run --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:endpoint": "venues.detail", "pp:method": "GET", "pp:path": "/v1/listings/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) > 0 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			wantLive := flags.dataSource == "live" || flagForce
			wantLocal := flags.dataSource != "live"

			if wantLocal && !wantLive {
				s, err := openNovelStoreRO(ctx, flagDB)
				if err != nil {
					return err
				}
				if s != nil {
					defer s.Close()
					if l, raw, ok, err := findListingByID(ctx, s, id); err != nil {
						return err
					} else if ok {
						if len(raw) > 0 {
							var asAny any
							if json.Unmarshal(raw, &asAny) == nil {
								if wantsHumanTable(cmd.OutOrStdout(), flags) {
									fmt.Fprintf(cmd.OutOrStdout(), "id=%s title=%s city=%s price=%.0f guests=%d fit=%s hydrated=%v\n",
										l.ID, l.Title, l.City, l.PriceHourly, l.Guests, l.FormatFit, l.Hydrated)
								}
								return printJSONFiltered(cmd.OutOrStdout(), asAny, flags)
							}
						}
						return printJSONFiltered(cmd.OutOrStdout(), listingToRow(l), flags)
					}
				}
				if flags.dataSource == "local" {
					return notFoundErr(fmt.Errorf("listing %q not found in local store", id))
				}
				// auto: fall through to live
			}

			// Live fetch full detail
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := fetchListingDetail(ctx, c, id)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			l, err := parseAndUpsertListingDetail(ctx, id, data)
			if err != nil {
				// still return raw if parse soft-fails
				var asAny any
				_ = json.Unmarshal(data, &asAny)
				return printJSONFiltered(cmd.OutOrStdout(), asAny, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && !wantsMachineOutput(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "id=%s title=%s city=%s price=%.0f guests=%d fit=%s (live)\n",
					l.ID, l.Title, l.City, l.PriceHourly, l.Guests, l.FormatFit)
			}
			var asAny any
			if json.Unmarshal(data, &asAny) == nil {
				return printJSONFiltered(cmd.OutOrStdout(), asAny, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), listingToRow(l), flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Always live-fetch detail even if present locally")
	return cmd
}
