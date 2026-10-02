// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Provider adapter for generated endpoint en.list-routes.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-bus-online/internal/jbo"
	"github.com/spf13/cobra"
)

func newRoutesCmd(flags *rootFlags) *cobra.Command {
	var query, language string
	var limit, offset int
	cmd := &cobra.Command{Use: "routes", Short: "Show route-discovery command help; routes list fetches bounded course IDs, names and source URLs", Example: "  japan-bus-online-pp-cli routes list --query Hamamatsu --limit 5 --agent", RunE: parentNoSubcommandRunE(flags)}
	list := &cobra.Command{Use: "list", Short: "Search the provider route catalog with bounded output", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--query=Hamamatsu;--limit=5"}, Example: "  japan-bus-online-pp-cli routes list --query Hamamatsu --limit 5 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "routes list")
		}
		if limit < 1 || limit > 100 || offset < 0 {
			return usageErr(fmt.Errorf("limit must be 1-100; offset must be nonnegative"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		if flags.dataSource == "local" {
			return usageErr(fmt.Errorf("route catalog requires live source data"))
		}
		c, e := jbo.New(language, flags.rateLimit)
		if e != nil {
			return usageErr(e)
		}
		out, e := c.Routes(ctx, query, offset, limit)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		return busEmit(cmd, flags, out)
	}}
	list.Flags().StringVar(&query, "query", "", "Substring of provider route name or course ID")
	list.Flags().StringVar(&language, "language", "en", "Source language: en (verified public surface)")
	list.Flags().IntVar(&limit, "limit", 20, "Maximum route rows to return, 1-100")
	list.Flags().IntVar(&offset, "offset", 0, "Filtered catalog offset for deterministic pagination")
	cmd.AddCommand(list)
	return cmd
}

func newEnPromotedCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "en", Hidden: true, Short: "Legacy bounded public catalog endpoint", Example: "  japan-bus-online-pp-cli en --agent", Args: cobra.NoArgs, Annotations: map[string]string{"pp:endpoint": "en.list-routes", "pp:method": "GET", "pp:path": "/en/AllRouteList", "mcp:read-only": "true", "mcp:hidden": "true", "pp:data-source": "live"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "en")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		if flags.dataSource == "local" {
			return usageErr(fmt.Errorf("route catalog requires live source data"))
		}
		c, e := jbo.New("en", flags.rateLimit)
		if e != nil {
			return e
		}
		out, e := c.Routes(ctx, "", 0, 20)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		return busEmit(cmd, flags, out)
	}}
}
