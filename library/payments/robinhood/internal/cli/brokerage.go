// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood/internal/brokeragemap"
	"github.com/spf13/cobra"
)

func newBrokerageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brokerage",
		Short: "Inspect authenticated Robinhood brokerage/account route maps",
		Long: `Inspect authenticated Robinhood route maps. Read-only map commands include
Robinhood's official Crypto API alongside brokerage/account routes captured from
logged-in ticker, options, crypto, account, settings, transfer, document, and
market pages.

Reads and plans do not call Robinhood. The execute command can send live
brokerage/account requests with caller-owned ROBINHOOD_BROKERAGE_TOKEN or
ROBINHOOD_COOKIE; write routes default to dry-run and require
ROBINHOOD_PP_ALLOW_WRITES=1 for live execution.`,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
	}
	cmd.AddCommand(newBrokerageSummaryCmd(flags))
	cmd.AddCommand(newBrokerageRoutesCmd(flags, false, true))
	cmd.AddCommand(newBrokerageRoutesCmd(flags, false))
	cmd.AddCommand(newBrokerageRoutesCmd(flags, true))
	cmd.AddCommand(newBrokeragePlanCmd(flags))
	cmd.AddCommand(newBrokerageExecuteCmd(flags))
	// Typed brokerage commands (hand-added from the captured live API surface).
	// See brokerage_read.go for rationale and the auth/host-reuse contract.
	cmd.AddCommand(newBrokerageAccountsCmd(flags))
	cmd.AddCommand(newBrokerageCeresAccountsCmd(flags))
	cmd.AddCommand(newBrokerageAccountCmd(flags))
	cmd.AddCommand(newBrokerageAccountSwitcherCmd(flags))
	cmd.AddCommand(newBrokeragePositionsCmd(flags))
	cmd.AddCommand(newBrokeragePortfoliosCmd(flags))
	cmd.AddCommand(newBrokerageInstrumentCmd(flags))
	cmd.AddCommand(newBrokerageQuoteCmd(flags))
	cmd.AddCommand(newBrokerageOrdersCmd(flags))
	cmd.AddCommand(newBrokerageOptionsCmd(flags))
	cmd.AddCommand(newBrokeragePerformanceCmd(flags))
	cmd.AddCommand(newBrokerageTransfersCmd(flags))
	cmd.AddCommand(newBrokerageDividendsCmd(flags))
	cmd.AddCommand(newBrokerageHistoryCmd(flags))
	cmd.AddCommand(newBrokerageWatchlistCmd(flags))
	return cmd
}

func newBrokerageSummaryCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "summary",
		Short:       "Summarize bundled Robinhood brokerage/account route maps",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			summary, err := brokeragemap.Summarize()
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, summary)
			}
			return flags.printTable(cmd, []string{"metric", "value"}, [][]string{
				{"routes", fmt.Sprintf("%d", summary.RouteCount)},
				{"unified_routes", fmt.Sprintf("%d", summary.UnifiedRouteCount)},
				{"official_crypto_routes", fmt.Sprintf("%d", summary.OfficialCryptoCount)},
				{"browser_routes", fmt.Sprintf("%d", summary.BrowserRouteCount)},
				{"hosts", fmt.Sprintf("%d", len(summary.Hosts))},
				{"risk_classes", fmt.Sprintf("%d", len(summary.Risks))},
			})
		},
	}
}

func newBrokerageRoutesCmd(flags *rootFlags, browser bool, unifiedOpt ...bool) *cobra.Command {
	var filters brokeragemap.Filters
	unified := len(unifiedOpt) > 0 && unifiedOpt[0]
	name := "routes"
	short := "List bundled brokerage/account route templates"
	if unified {
		name = "all-routes"
		short = "List unified official Crypto plus brokerage/account route templates"
	}
	if browser {
		name = "browser-routes"
		short = "List latest sanitized authenticated CDP route templates"
	}
	cmd := &cobra.Command{
		Use:         name,
		Short:       short,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var routes []brokeragemap.Route
			var err error
			if unified {
				routes, err = brokeragemap.UnifiedRoutes()
			} else if browser {
				routes, err = brokeragemap.BrowserRoutes()
			} else {
				routes, err = brokeragemap.Routes()
			}
			if err != nil {
				return err
			}
			if filters.Limit == 0 {
				filters.Limit = 80
			}
			filtered := brokeragemap.Filter(routes, filters)
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"count": len(filtered), "routes": filtered})
			}
			rows := make([][]string, 0, len(filtered))
			for _, route := range filtered {
				rows = append(rows, []string{
					route.Risk,
					route.Host,
					strings.Join(route.Categories, ","),
					strings.Join(route.Methods, ","),
					route.URL,
				})
			}
			return flags.printTable(cmd, []string{"risk", "host", "categories", "methods", "url"}, rows)
		},
	}
	cmd.Flags().StringVar(&filters.Risk, "risk", "", "filter by risk")
	cmd.Flags().StringVar(&filters.Category, "category", "", "filter by category")
	cmd.Flags().StringVar(&filters.Host, "host", "", "filter by host")
	cmd.Flags().StringVar(&filters.Query, "query", "", "substring filter against URL")
	cmd.Flags().IntVar(&filters.Limit, "limit", 80, "maximum routes to print")
	return cmd
}

func newBrokeragePlanCmd(flags *rootFlags) *cobra.Command {
	var method string
	var params []string
	var bodyJSON string
	cmd := &cobra.Command{
		Use:         "plan <query>",
		Short:       "Build a dry-run request plan for a mapped route",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			routes, err := brokeragemap.Routes()
			if err != nil {
				return err
			}
			route, err := brokeragemap.Find(routes, args[0])
			if err != nil {
				return notFoundErr(err)
			}
			parsedParams, err := brokeragemap.ParseParams(params)
			if err != nil {
				return usageErr(err)
			}
			body, err := parseBrokerageBody(bodyJSON)
			if err != nil {
				return usageErr(err)
			}
			plan := brokeragemap.BuildPlan(route, method, parsedParams, body, true)
			if flags.asJSON {
				return flags.printJSON(cmd, plan)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n%s\n", plan.Method, plan.URL, plan.Command)
			for _, warning := range plan.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
			}
			if len(plan.MissingParams) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "missing params: %s\n", strings.Join(plan.MissingParams, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "override inferred HTTP method")
	cmd.Flags().StringArrayVar(&params, "param", nil, "replace a route placeholder; repeatable name=value")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "JSON request body")
	return cmd
}

func newBrokerageExecuteCmd(flags *rootFlags) *cobra.Command {
	var method string
	var params []string
	var bodyJSON string
	var full bool
	cmd := &cobra.Command{
		Use:   "execute <query>",
		Short: "Execute a mapped brokerage/account request with PP write gates",
		Long: `Execute a mapped brokerage/account request with caller-owned auth.

[WRITES TO LIVE ROBINHOOD] Write routes default to --dry-run. Live execution
requires --live-write and ROBINHOOD_PP_ALLOW_WRITES=1. Read routes may execute
without the write gate when ROBINHOOD_BROKERAGE_TOKEN or ROBINHOOD_COOKIE is set.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only":   "false",
			"mcp:risk":        "write-mutate",
			"pp:dynamic-risk": "true",
			"pp:barrier":      "requires_ROBINHOOD_PP_ALLOW_WRITES",
			"pp:auth-surface": "ROBINHOOD_BROKERAGE_TOKEN|ROBINHOOD_COOKIE",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			routes, err := brokeragemap.Routes()
			if err != nil {
				return err
			}
			route, err := brokeragemap.Find(routes, args[0])
			if err != nil {
				return notFoundErr(err)
			}
			parsedParams, err := brokeragemap.ParseParams(params)
			if err != nil {
				return usageErr(err)
			}
			body, err := parseBrokerageBody(bodyJSON)
			if err != nil {
				return usageErr(err)
			}
			dryRun := flags.dryRun
			if brokeragemap.Mutates(route.Risk) && !flags.dryRun && !flags.liveWrite {
				fmt.Fprintf(cmd.ErrOrStderr(), "[WRITES TO LIVE ROBINHOOD] %s defaults to --dry-run. Pass --live-write and set ROBINHOOD_PP_ALLOW_WRITES=1 only after explicit approval.\n", cmd.CommandPath())
				dryRun = true
			}
			plan := brokeragemap.BuildPlan(route, method, parsedParams, body, dryRun)
			// Refuse to execute a route whose path params are unresolved: the
			// URL would still contain literal {placeholder} segments and hit a
			// wrong/garbage endpoint. `brokerage plan` surfaces these the same
			// way; execute must not silently send the request.
			if len(plan.MissingParams) > 0 {
				return usageErr(fmt.Errorf("cannot execute: unresolved path params %s — supply them with --param name=value (use 'brokerage plan' to inspect)", strings.Join(plan.MissingParams, ", ")))
			}
			bodyBytes, _ := json.Marshal(body)
			if body == nil {
				bodyBytes = nil
			}
			result, err := brokeragemap.Execute(cmd.Context(), plan, brokeragemap.ExecuteOptions{
				DryRun:    dryRun,
				Body:      bodyBytes,
				FullBody:  full,
				RateLimit: flags.rateLimit,
				Timeout:   flags.timeout,
			})
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				if err := flags.printJSON(cmd, result); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%d %s %s %s\n", result.Status, result.StatusText, result.Method, result.URL)
				if result.Body != "" {
					fmt.Fprintln(cmd.OutOrStdout(), result.Body)
				}
			}
			// Propagate HTTP failures as a non-zero exit, mirroring how the
			// crypto commands surface errors via classifyAPIError. Without this,
			// a 401/403/404/500 from the brokerage API printed but exited 0,
			// so scripts could not detect a failed live request. Dry runs set
			// OK=true, so previews are unaffected.
			if !result.OK {
				return apiErr(fmt.Errorf("brokerage request failed: %d %s", result.Status, result.StatusText))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "override inferred HTTP method")
	cmd.Flags().StringArrayVar(&params, "param", nil, "replace a route placeholder; repeatable name=value")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "JSON request body")
	cmd.Flags().BoolVar(&full, "full", false, "print full response body instead of bounded preview")
	return cmd
}

func parseBrokerageBody(raw string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var body any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return nil, fmt.Errorf("invalid --body-json: %w", err)
	}
	return body, nil
}
