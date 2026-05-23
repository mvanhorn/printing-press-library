// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// orders list — L2-authenticated listing of the EOA's open CLOB orders.
//
// The generator-default version of this file sent default Bearer auth and
// returned HTTP 401 from /data/orders. /data/* endpoints require L2 HMAC
// headers (POLY_API_KEY + POLY_SIGNATURE built from the L2 secret), which
// this hand-written version builds via internal/clob.BuildL2Headers — the
// same path orders create + orders cancel + ctf redeem use.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/clob"
	"polymarket-pp-cli/internal/config"
)

func newOrdersListCmd(flags *rootFlags) *cobra.Command {
	var flagMarket, flagNextCursor string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the authenticated user's open CLOB orders (L2 HMAC auth required).",
		Example: "  polymarket-pp-cli orders list\n  polymarket-pp-cli orders list --market 0xCONDITION_ID",
		Annotations: map[string]string{
			"pp:endpoint":   "orders.list",
			"pp:method":     "GET",
			"pp:path":       "https://clob.polymarket.com/data/orders",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			signer, err := loadSigner(flags)
			if err != nil {
				return authErr(err)
			}
			cfg, _ := config.Load(flags.configPath)
			if cfg == nil {
				return authErr(fmt.Errorf("no config loaded"))
			}
			creds := clob.L2Creds{
				APIKey:     cfg.PolymarketApiKey,
				Secret:     cfg.PolymarketApiSecret,
				Passphrase: cfg.PolymarketApiPassphrase,
			}
			if creds.APIKey == "" {
				return authErr(fmt.Errorf("L2 credentials missing — run `polymarket-pp-cli auth derive` first"))
			}

			// Build path-with-query string used both for the HTTP URL and
			// for HMAC signing. Polymarket's signature path includes the
			// raw query string, so we build it once and pass it through.
			path := "/data/orders"
			if flagMarket != "" || flagNextCursor != "" {
				v := url.Values{}
				if flagMarket != "" {
					v.Set("market", flagMarket)
				}
				if flagNextCursor != "" {
					v.Set("next_cursor", flagNextCursor)
				}
				path += "?" + v.Encode()
			}

			headers, err := clob.BuildL2Headers(creds, signer.AddressHex(), "GET", path, nil, time.Now().Unix())
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, gerr := c.GetWithHeaders(cmd.Context(), "https://clob.polymarket.com"+path, nil, headers)
			if gerr != nil {
				return classifyAPIError(gerr, flags)
			}

			out := map[string]any{
				"meta":    map[string]any{"source": "live", "owner": signer.AddressHex()},
				"results": json.RawMessage(raw),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagMarket, "market", "", "Filter by condition_id (0x-prefixed)")
	cmd.Flags().StringVar(&flagNextCursor, "next-cursor", "", "Pagination cursor from previous response")
	return cmd
}
