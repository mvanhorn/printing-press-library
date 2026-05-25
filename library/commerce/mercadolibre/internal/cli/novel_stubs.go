// Copyright 2026 LeaCast. Licensed under MIT. See LICENSE.
// Novel-feature stubs — wired into the command tree as hidden + experimental.
// Each ships as an honest "coming next" message in v0.1.x. The infrastructure
// (HTTP client, OAuth, SQLite store) all exists; what's missing is the
// per-command implementation logic. These stubs ship hidden by default so they
// don't pollute --help for end-users yet, but they're discoverable via
// `mercadolibre-pp-cli help <stub-name>` and via the `api` browser. Promoted
// out of stub status as each one is built; see roadmap in README "Novel
// features (planned)" section.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "watch <site-id> <keyword>",
		Short:  "[experimental] Poll a catalog search at interval; emit JSON on new products (cron-friendly)",
		Hidden: true,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"novel:status":  "stub",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return novelStubMessage(flags, "watch", "Poll-and-diff a saved catalog search; emit JSON on new product appearances. Cron-friendly resale-opportunity radar.", "watch MLA \"centro de mesa\" --interval 1h --since 2026-01-01")
		},
	}
	cmd.Flags().Duration("interval", 0, "Polling interval (e.g. 30m, 1h, 24h)")
	cmd.Flags().String("since", "", "Only emit products created after this ISO date")
	cmd.Flags().Int("limit", 50, "Max products to fetch per poll")
	return cmd
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "compare <site-id> <keyword>",
		Short:  "[experimental] Dedup near-duplicate catalog results via token-bag fingerprinting",
		Hidden: true,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"novel:status":  "stub",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return novelStubMessage(flags, "compare", "Token-bag fingerprint dedup of catalog search results. Collapses near-duplicate listings to one exemplar per fingerprint.", "compare MLA \"iphone\" --dedupe-variants")
		},
	}
	cmd.Flags().Bool("dedupe-variants", true, "Enable variant collapsing")
	cmd.Flags().Int("limit", 50, "Max results to compare")
	return cmd
}

func newAnalyticsStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "ml-analytics <category-id>",
		Short:  "[experimental] Local SQLite analytics: price percentiles, outlier-trim, top sellers (requires sync)",
		Hidden: true,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"novel:status":  "stub",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return novelStubMessage(flags, "ml-analytics", "Analytics over locally-synced catalog data: price percentiles, IQR outlier trim, top sellers by category, std-dev distribution.", "ml-analytics MLA1055 --metric price --window 30d")
		},
	}
	cmd.Flags().String("metric", "price", "Metric to compute (price, listings, sellers)")
	cmd.Flags().String("window", "30d", "Time window for metric")
	return cmd
}

func novelStubMessage(flags *rootFlags, name, description, exampleUsage string) error {
	out := map[string]any{
		"status":        "stub",
		"command":       name,
		"description":   description,
		"example_usage": exampleUsage,
		"message":       fmt.Sprintf("The '%s' command is a wired stub in v0.1.x. The infrastructure exists; the per-command implementation is on the roadmap. See https://github.com/LeaCast/mercadolibre-pp-cli for roadmap status.", name),
	}
	if flags != nil && flags.asJSON {
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	fmt.Fprintf(cmd().OutOrStderr(), "[stub] %s — %s\n  Example: %s\n  Status: coming in v0.2.x\n", name, description, exampleUsage)
	return nil
}

// cmd returns a no-op cobra.Command for stderr access in non-json mode.
func cmd() *cobra.Command { return &cobra.Command{} }
