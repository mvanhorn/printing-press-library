// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

// Typed brokerage commands built on the captured Robinhood live API surface
// (library/.../.manuscripts/.../research/robinhood-live-api-surface.md).
//
// These are hand-added extensions to the generated CLI (recorded in
// .printing-press-patches.json). The crypto-focused generated client
// (internal/client) speaks the official Crypto API's x-api-key + ed25519
// signed-request scheme and cannot reach the brokerage hosts, which use an
// OAuth `Authorization: Bearer <token>` credential across multiple hosts
// (api.robinhood.com, bonfire.robinhood.com, nummus.robinhood.com, ...).
//
// Rather than invent a second auth path, every typed command below builds a
// brokeragemap.Plan for a specific captured endpoint and runs it through
// brokeragemap.Execute — the same Bearer-token, multi-host, rate-limited,
// dry-run/write-gated transport the generic `brokerage execute` command uses.
// Auth comes from ROBINHOOD_BROKERAGE_TOKEN (or ROBINHOOD_COOKIE +
// ROBINHOOD_CSRF_TOKEN), shared with the route-map executor.
//
// Read commands execute live by default (when a token is present). Write
// commands (orders place/cancel, watchlist add/remove) default to --dry-run
// and require both --live-write and ROBINHOOD_PP_ALLOW_WRITES=1 for a live
// mutation, mirroring the generated write-gate contract.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood/internal/brokeragemap"
	"github.com/spf13/cobra"
)

// brokerage host constants — the captured surface is multi-host.
const (
	hostAPI     = "api.robinhood.com"
	hostBonfire = "bonfire.robinhood.com"
	hostNummus  = "nummus.robinhood.com"
	hostMinerva = "minerva.robinhood.com"
)

// runBrokerageRead executes a typed read request against a captured endpoint
// and prints the JSON body through the standard flag pipeline. It is the
// read-side analogue of newBrokerageExecuteCmd, specialized for a fixed
// host+path so callers do not pass route queries.
func runBrokerageRead(cmd *cobra.Command, flags *rootFlags, host, path, risk string, params, query map[string]string) error {
	plan := brokeragemap.BuildDirectPlan(host, path, "GET", risk, params, query)
	if len(plan.MissingParams) > 0 {
		return usageErr(fmt.Errorf("missing required path params %s — supply them with the documented flags", strings.Join(plan.MissingParams, ", ")))
	}
	return execBrokeragePlan(cmd, flags, plan, nil)
}

// runBrokerageWrite executes a typed write request. The write defaults to
// dry-run unless the caller passes --live-write; brokeragemap.Execute then
// still enforces the ROBINHOOD_PP_ALLOW_WRITES=1 env gate before any live
// mutation. Order placement and cancellation route through here.
func runBrokerageWrite(cmd *cobra.Command, flags *rootFlags, host, path, method, risk string, params map[string]string, body any) error {
	plan := brokeragemap.BuildDirectPlan(host, path, method, risk, params, nil)
	if len(plan.MissingParams) > 0 {
		return usageErr(fmt.Errorf("missing required path params %s — supply them with the documented flags", strings.Join(plan.MissingParams, ", ")))
	}
	dryRun := flags.dryRun
	if brokeragemap.Mutates(plan.Risk) && !flags.dryRun && !flags.liveWrite {
		fmt.Fprintf(cmd.ErrOrStderr(), "[WRITES TO LIVE ROBINHOOD] %s defaults to --dry-run. Pass --live-write and set ROBINHOOD_PP_ALLOW_WRITES=1 only after explicit approval.\n", cmd.CommandPath())
		dryRun = true
	}
	plan.Body = body
	if dryRun {
		plan.Mode = "dry_run"
	}
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	return execBrokeragePlanWithOpts(cmd, flags, plan, brokeragemap.ExecuteOptions{
		DryRun:    dryRun,
		Body:      bodyBytes,
		RateLimit: flags.rateLimit,
		Timeout:   flags.timeout,
	})
}

func execBrokeragePlan(cmd *cobra.Command, flags *rootFlags, plan brokeragemap.Plan, body []byte) error {
	dryRun := flags.dryRun
	if dryRun {
		plan.Mode = "dry_run"
	}
	return execBrokeragePlanWithOpts(cmd, flags, plan, brokeragemap.ExecuteOptions{
		DryRun:    dryRun,
		Body:      body,
		RateLimit: flags.rateLimit,
		Timeout:   flags.timeout,
	})
}

func execBrokeragePlanWithOpts(cmd *cobra.Command, flags *rootFlags, plan brokeragemap.Plan, opts brokeragemap.ExecuteOptions) error {
	result, err := brokeragemap.Execute(cmd.Context(), plan, opts)
	if err != nil {
		return apiErr(err)
	}
	// Dry-run and successful live reads both print the response body. The
	// body for a live read is the raw API JSON; for a dry run it is the
	// pretty-printed plan envelope brokeragemap.Execute synthesizes.
	if flags.asJSON {
		if result.OK && plan.Mode != "dry_run" && json.Valid([]byte(result.Body)) {
			// Pass the raw API JSON through the flag pipeline so --select /
			// --compact behave like the generated read commands.
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(result.Body), flags)
		}
		if err := flags.printJSON(cmd, result); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%d %s %s %s\n", result.Status, result.StatusText, result.Method, result.URL)
		if result.Body != "" {
			fmt.Fprintln(cmd.OutOrStdout(), result.Body)
		}
	}
	if !result.OK {
		return apiErr(fmt.Errorf("brokerage request failed: %d %s", result.Status, result.StatusText))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

func newBrokerageAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "List all brokerage accounts (individual + retirement)",
		Long: `List every Robinhood brokerage account for the authenticated user.

Maps GET https://api.robinhood.com/accounts/. Robinhood returns one entry per
account (multiple individual investing accounts plus any retirement account);
each carries its account_number, type, and a portfolio/positions URL. Use
'brokerage portfolios' for per-account dollar balances and 'brokerage account'
for the unified balance view of a single account.`,
		Example:     "  robinhood-pp-cli brokerage accounts --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/accounts/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

func newBrokerageCeresAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ceres-accounts",
		Short: "List accounts via the ceres gateway (richer account metadata)",
		Long: `List accounts through the ceres aggregation gateway.

Maps GET https://api.robinhood.com/ceres/v1/accounts. ceres returns the
account set the modern web/app dashboards use, including the account UUIDs
required by 'brokerage ceres-positions', 'brokerage ceres-orders', and
'brokerage pnl'.`,
		Example:     "  robinhood-pp-cli brokerage ceres-accounts --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/ceres/v1/accounts", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

func newBrokerageAccountCmd(flags *rootFlags) *cobra.Command {
	var accountID string
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Show the unified balance view for one account",
		Long: `Show the unified (balances + buying power) view for a single account.

Maps GET https://bonfire.robinhood.com/accounts/{account_id}/unified/. The
account id is the account_number from 'brokerage accounts'. This is the
endpoint the web dashboard uses to render the "main" account balance.`,
		Example:     "  robinhood-pp-cli brokerage account --account-id 1AB23456 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostBonfire, "/accounts/{account_id}/unified/", "sensitive-read",
				map[string]string{"account_id": accountID}, nil)
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "Account number (from 'brokerage accounts'); required")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func newBrokerageAccountSwitcherCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account-switcher",
		Short: "List accounts as shown in the app account switcher",
		Long: `List accounts in the account-switcher shape the mobile app uses.

Maps GET https://bonfire.robinhood.com/home/account_switcher/v2. Handy for a
quick "which accounts do I have" view across individual + retirement without
the full balance payload.`,
		Example:     "  robinhood-pp-cli brokerage account-switcher --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostBonfire, "/home/account_switcher/v2", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

// ---------------------------------------------------------------------------
// Positions / portfolios
// ---------------------------------------------------------------------------

func newBrokeragePositionsCmd(flags *rootFlags) *cobra.Command {
	var nonzero bool
	cmd := &cobra.Command{
		Use:   "positions",
		Short: "List equity positions",
		Long: `List the authenticated user's equity (stock) positions.

Maps GET https://api.robinhood.com/positions/. Pass --nonzero to request only
open positions (quantity > 0) via the nonzero=true query param Robinhood
supports.`,
		Example:     "  robinhood-pp-cli brokerage positions --nonzero --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := map[string]string{}
			if nonzero {
				query["nonzero"] = "true"
			}
			return runBrokerageRead(cmd, flags, hostAPI, "/positions/", "sensitive-read", nil, query)
		},
	}
	cmd.Flags().BoolVar(&nonzero, "nonzero", false, "Only return open positions (quantity > 0)")
	return cmd
}

func newBrokeragePortfoliosCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolios",
		Short: "List portfolios (equity, market value, withdrawable amount per account)",
		Long: `List portfolio summaries — one per account.

Maps GET https://api.robinhood.com/portfolios/. Each entry carries equity,
market_value, extended-hours equity, and withdrawable_amount for its account.
This is the dollar-balance companion to 'brokerage accounts'.`,
		Example:     "  robinhood-pp-cli brokerage portfolios --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/portfolios/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

func newBrokerageInstrumentCmd(flags *rootFlags) *cobra.Command {
	var symbol string
	cmd := &cobra.Command{
		Use:   "instrument",
		Short: "Look up an instrument (tradable security) by symbol",
		Long: `Look up the instrument record for a ticker symbol.

Maps GET https://api.robinhood.com/instruments/ with a symbol query filter.
Returns the instrument UUID, tradability, and market — the UUID feeds
'brokerage options-chain' and order placement.`,
		Example:     "  robinhood-pp-cli brokerage instrument --symbol AAPL --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := map[string]string{}
			if symbol != "" {
				query["symbol"] = strings.ToUpper(symbol)
			}
			return runBrokerageRead(cmd, flags, hostAPI, "/instruments/", "read", nil, query)
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Ticker symbol to look up (e.g. AAPL)")
	return cmd
}

func newBrokerageQuoteCmd(flags *rootFlags) *cobra.Command {
	var symbols string
	cmd := &cobra.Command{
		Use:   "quote",
		Short: "Fetch real-time quotes for one or more symbols",
		Long: `Fetch market quotes for a comma-separated list of symbols.

Maps GET https://api.robinhood.com/marketdata/quotes/ with a symbols query
filter. Returns last trade price, bid/ask, and previous close.`,
		Example:     "  robinhood-pp-cli brokerage quote --symbols AAPL,TSLA --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := map[string]string{}
			if symbols != "" {
				query["symbols"] = strings.ToUpper(symbols)
			}
			return runBrokerageRead(cmd, flags, hostAPI, "/marketdata/quotes/", "read", nil, query)
		},
	}
	cmd.Flags().StringVar(&symbols, "symbols", "", "Comma-separated ticker symbols (e.g. AAPL,TSLA)")
	return cmd
}

// ---------------------------------------------------------------------------
// Orders (equity) — read + write-gated place/cancel
// ---------------------------------------------------------------------------

func newBrokerageOrdersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "List equity orders (open + historical)",
		Long: `List the authenticated user's equity orders.

Maps GET https://api.robinhood.com/orders/. Returns order state, side,
quantity, price, and timestamps.`,
		Example:     "  robinhood-pp-cli brokerage orders --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/orders/", "sensitive-read", nil, nil)
		},
	}
	cmd.AddCommand(newBrokerageOrdersPlaceCmd(flags))
	cmd.AddCommand(newBrokerageOrdersCancelCmd(flags))
	return cmd
}

func newBrokerageOrdersPlaceCmd(flags *rootFlags) *cobra.Command {
	var bodyJSON string
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place an equity order (DRY-RUN BY DEFAULT — never auto-executes)",
		Long: `Scaffold/place an equity order against POST https://api.robinhood.com/orders/.

[WRITES TO LIVE ROBINHOOD] This command DEFAULTS TO --dry-run and never places
a real trade unless you pass --live-write AND export ROBINHOOD_PP_ALLOW_WRITES=1.
The order payload is supplied verbatim via --body-json (Robinhood expects fields
like account, instrument, symbol, type, time_in_force, side, quantity, price);
no field validation is performed here on purpose so the agent cannot
accidentally reshape an order. Real trades are intended to be executed by the
account owner; this command exists to preview the exact request.`,
		Example:     `  robinhood-pp-cli brokerage orders place --body-json '{"symbol":"AAPL","side":"buy","type":"market","quantity":"1","time_in_force":"gfd"}' --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:risk": "write-mutate", "pp:barrier": "requires_ROBINHOOD_PP_ALLOW_WRITES"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := parseBrokerageBody(bodyJSON)
			if err != nil {
				return usageErr(err)
			}
			return runBrokerageWrite(cmd, flags, hostAPI, "/orders/", "POST", "write-mutate", nil, body)
		},
	}
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Full order payload as JSON (required for a live place)")
	return cmd
}

func newBrokerageOrdersCancelCmd(flags *rootFlags) *cobra.Command {
	var orderID string
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an equity order (DRY-RUN BY DEFAULT — never auto-executes)",
		Long: `Cancel an open equity order via POST https://api.robinhood.com/orders/{order_id}/cancel/.

[WRITES TO LIVE ROBINHOOD] Defaults to --dry-run; a live cancel requires
--live-write AND ROBINHOOD_PP_ALLOW_WRITES=1.`,
		Example:     "  robinhood-pp-cli brokerage orders cancel --order-id <uuid> --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:risk": "write-mutate", "pp:barrier": "requires_ROBINHOOD_PP_ALLOW_WRITES"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageWrite(cmd, flags, hostAPI, "/orders/{order_id}/cancel/", "POST", "write-mutate",
				map[string]string{"order_id": orderID}, nil)
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order UUID to cancel; required")
	_ = cmd.MarkFlagRequired("order-id")
	return cmd
}

// ---------------------------------------------------------------------------
// Options — positions, orders, chain, market data, place/cancel
// ---------------------------------------------------------------------------

func newBrokerageOptionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Read and analyze options: positions, orders, chain, quotes",
		Long: `Options subcommands. Read options positions and orders, pull an options
chain for a symbol, and quote a specific option contract. Place/cancel are
write-gated (dry-run by default).`,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
	}
	cmd.AddCommand(newBrokerageOptionsPositionsCmd(flags))
	cmd.AddCommand(newBrokerageOptionsOrdersCmd(flags))
	cmd.AddCommand(newBrokerageOptionsChainCmd(flags))
	cmd.AddCommand(newBrokerageOptionsInstrumentsCmd(flags))
	cmd.AddCommand(newBrokerageOptionsMarketdataCmd(flags))
	cmd.AddCommand(newBrokerageOptionsPlaceCmd(flags))
	cmd.AddCommand(newBrokerageOptionsCancelCmd(flags))
	return cmd
}

func newBrokerageOptionsPositionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "positions",
		Short: "List aggregate options positions",
		Long: `List aggregate options positions.

Maps GET https://api.robinhood.com/options/aggregate_positions/. Returns the
user's open options exposure grouped by underlying + strategy.`,
		Example:     "  robinhood-pp-cli brokerage options positions --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/options/aggregate_positions/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

func newBrokerageOptionsOrdersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "List options orders (open + historical)",
		Long: `List options orders.

Maps GET https://api.robinhood.com/options/orders/.`,
		Example:     "  robinhood-pp-cli brokerage options orders --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/options/orders/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

func newBrokerageOptionsChainCmd(flags *rootFlags) *cobra.Command {
	var chainID string
	cmd := &cobra.Command{
		Use:   "chain",
		Short: "List option chains, or fetch one chain by id",
		Long: `List option chains (GET https://api.robinhood.com/options/chains/), or fetch a
single chain by id (GET https://api.robinhood.com/options/chains/{chain_id}/)
when --chain-id is supplied. A chain id is discoverable from an instrument's
tradable_chain_id field via 'brokerage instrument'.`,
		Example:     "  robinhood-pp-cli brokerage options chain --chain-id <uuid> --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if chainID != "" {
				return runBrokerageRead(cmd, flags, hostAPI, "/options/chains/{chain_id}/", "read",
					map[string]string{"chain_id": chainID}, nil)
			}
			return runBrokerageRead(cmd, flags, hostAPI, "/options/chains/", "read", nil, nil)
		},
	}
	cmd.Flags().StringVar(&chainID, "chain-id", "", "Option chain UUID; omit to list all chains")
	return cmd
}

func newBrokerageOptionsInstrumentsCmd(flags *rootFlags) *cobra.Command {
	var chainID, expiration, optionType string
	cmd := &cobra.Command{
		Use:   "instruments",
		Short: "List option instruments (contracts) for a chain",
		Long: `List option instruments (individual contracts).

Maps GET https://api.robinhood.com/options/instruments/. Filter with
--chain-id, --expiration (YYYY-MM-DD), and --type (call|put) to narrow a
chain to a specific expiry/side — this is the per-contract detail backing an
options chain view.`,
		Example:     "  robinhood-pp-cli brokerage options instruments --chain-id <uuid> --expiration 2026-06-19 --type call --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := map[string]string{}
			if chainID != "" {
				query["chain_id"] = chainID
			}
			if expiration != "" {
				query["expiration_dates"] = expiration
			}
			if optionType != "" {
				query["type"] = strings.ToLower(optionType)
			}
			return runBrokerageRead(cmd, flags, hostAPI, "/options/instruments/", "read", nil, query)
		},
	}
	cmd.Flags().StringVar(&chainID, "chain-id", "", "Restrict to a chain UUID")
	cmd.Flags().StringVar(&expiration, "expiration", "", "Expiration date filter (YYYY-MM-DD)")
	cmd.Flags().StringVar(&optionType, "type", "", "Contract type: call or put")
	return cmd
}

func newBrokerageOptionsMarketdataCmd(flags *rootFlags) *cobra.Command {
	var instruments string
	cmd := &cobra.Command{
		Use:   "marketdata",
		Short: "Fetch market data (greeks, IV, bid/ask) for option contracts",
		Long: `Fetch options market data — price, bid/ask, implied volatility, and greeks.

Maps GET https://api.robinhood.com/marketdata/options/ with an instruments
query filter (comma-separated option-instrument URLs or UUIDs from
'brokerage options instruments').`,
		Example:     "  robinhood-pp-cli brokerage options marketdata --instruments <uuid> --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := map[string]string{}
			if instruments != "" {
				query["instruments"] = instruments
			}
			return runBrokerageRead(cmd, flags, hostAPI, "/marketdata/options/", "read", nil, query)
		},
	}
	cmd.Flags().StringVar(&instruments, "instruments", "", "Comma-separated option-instrument UUIDs or URLs")
	return cmd
}

func newBrokerageOptionsPlaceCmd(flags *rootFlags) *cobra.Command {
	var bodyJSON string
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place an options order (DRY-RUN BY DEFAULT — never auto-executes)",
		Long: `Place an options order against POST https://api.robinhood.com/options/orders/.

[WRITES TO LIVE ROBINHOOD] Defaults to --dry-run; a live order requires
--live-write AND ROBINHOOD_PP_ALLOW_WRITES=1. The full options-order payload
(account, direction, legs, price, type, time_in_force, quantity, trigger) is
supplied verbatim via --body-json. Real trades are intended to be executed by
the account owner.`,
		Example:     `  robinhood-pp-cli brokerage options place --body-json '{"direction":"debit","legs":[],"type":"limit"}' --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:risk": "write-mutate", "pp:barrier": "requires_ROBINHOOD_PP_ALLOW_WRITES"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := parseBrokerageBody(bodyJSON)
			if err != nil {
				return usageErr(err)
			}
			return runBrokerageWrite(cmd, flags, hostAPI, "/options/orders/", "POST", "write-mutate", nil, body)
		},
	}
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Full options-order payload as JSON (required for a live place)")
	return cmd
}

func newBrokerageOptionsCancelCmd(flags *rootFlags) *cobra.Command {
	var orderID string
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an options order (DRY-RUN BY DEFAULT — never auto-executes)",
		Long: `Cancel an options order via POST https://api.robinhood.com/options/orders/{order_id}/cancel/.

[WRITES TO LIVE ROBINHOOD] Defaults to --dry-run; a live cancel requires
--live-write AND ROBINHOOD_PP_ALLOW_WRITES=1.`,
		Example:     "  robinhood-pp-cli brokerage options cancel --order-id <uuid> --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:risk": "write-mutate", "pp:barrier": "requires_ROBINHOOD_PP_ALLOW_WRITES"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageWrite(cmd, flags, hostAPI, "/options/orders/{order_id}/cancel/", "POST", "write-mutate",
				map[string]string{"order_id": orderID}, nil)
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Options order UUID to cancel; required")
	_ = cmd.MarkFlagRequired("order-id")
	return cmd
}

// ---------------------------------------------------------------------------
// Performance windows
// ---------------------------------------------------------------------------

func newBrokeragePerformanceCmd(flags *rootFlags) *cobra.Command {
	var accountID, span, interval string
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Portfolio value over a window (YTD, 1week, 1month, year, 5year, all)",
		Long: `Portfolio value history for an account over a time window.

Maps GET https://api.robinhood.com/portfolios/historicals/{account_id}/ with
span + interval query params. --span accepts: day, week, month, 3month, year,
5year, all (Robinhood's native spans). 'ytd' is accepted as a convenience alias
that maps to span=year (Robinhood has no native YTD span; year is the closest
trailing window). The interval is auto-derived from the span when --interval is
omitted (day→5minute, week/month→day, longer→week), matching the web client.`,
		Example:     "  robinhood-pp-cli brokerage performance --account-id 1AB23456 --span year --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s := strings.ToLower(strings.TrimSpace(span))
			if s == "ytd" {
				s = "year"
			}
			switch s {
			case "", "day", "week", "month", "3month", "year", "5year", "all":
			default:
				return usageErr(fmt.Errorf("invalid --span %q: must be one of day, week, month, 3month, year, 5year, all (or ytd)", span))
			}
			iv := strings.ToLower(strings.TrimSpace(interval))
			if iv == "" {
				iv = defaultIntervalForSpan(s)
			}
			query := map[string]string{}
			if s != "" {
				query["span"] = s
			}
			if iv != "" {
				query["interval"] = iv
			}
			return runBrokerageRead(cmd, flags, hostAPI, "/portfolios/historicals/{account_id}/", "sensitive-read",
				map[string]string{"account_id": accountID}, query)
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "Account number (from 'brokerage accounts'); required")
	cmd.Flags().StringVar(&span, "span", "year", "Time window: day, week, month, 3month, year, 5year, all (ytd aliases year)")
	cmd.Flags().StringVar(&interval, "interval", "", "Sample interval: 5minute, 10minute, hour, day, week (auto if omitted)")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func defaultIntervalForSpan(span string) string {
	switch span {
	case "day":
		return "5minute"
	case "week", "month":
		return "day"
	case "3month", "year", "5year", "all":
		return "week"
	default:
		return "day"
	}
}

// ---------------------------------------------------------------------------
// Transfers / deposits / withdrawals
// ---------------------------------------------------------------------------

func newBrokerageTransfersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfers",
		Short: "List ACH transfers (deposits + withdrawals)",
		Long: `List ACH transfers — both deposits and withdrawals.

Maps GET https://api.robinhood.com/ach/transfers/. Each entry carries
direction (deposit|withdraw), amount, state, and the linked bank
relationship.`,
		Example:     "  robinhood-pp-cli brokerage transfers --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/ach/transfers/", "sensitive-read", nil, nil)
		},
	}
	cmd.AddCommand(newBrokerageTransfersRelationshipsCmd(flags))
	cmd.AddCommand(newBrokerageTransfersUnifiedCmd(flags))
	return cmd
}

func newBrokerageTransfersRelationshipsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relationships",
		Short: "List linked bank (ACH) relationships",
		Long: `List linked bank accounts (ACH relationships).

Maps GET https://api.robinhood.com/ach/relationships/. Returns the bank
relationship UUIDs and masked account numbers used as the source/destination
of transfers.`,
		Example:     "  robinhood-pp-cli brokerage transfers relationships --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/ach/relationships/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

func newBrokerageTransfersUnifiedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unified",
		Short: "List unified transfers across rails (ACH + instant + crypto)",
		Long: `List unified transfers across payment rails.

Maps GET https://bonfire.robinhood.com/paymenthub/unified_transfers/. This is
the consolidated money-movement feed the modern app uses, broader than
ach/transfers alone.`,
		Example:     "  robinhood-pp-cli brokerage transfers unified --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostBonfire, "/paymenthub/unified_transfers/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

// ---------------------------------------------------------------------------
// Dividends
// ---------------------------------------------------------------------------

func newBrokerageDividendsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dividends",
		Short: "List dividends (paid + pending)",
		Long: `List dividend records.

Maps GET https://api.robinhood.com/dividends/. Each entry carries the
instrument, amount, rate, position, state (pending|paid|reinvested), and
pay/record dates.`,
		Example:     "  robinhood-pp-cli brokerage dividends --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/dividends/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

// ---------------------------------------------------------------------------
// Account history / transactions
// ---------------------------------------------------------------------------

func newBrokerageHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List account transaction history (unified activity feed)",
		Long: `List the unified account transaction history.

Maps GET https://minerva.robinhood.com/history/transactions/ — the
consolidated activity feed (trades, transfers, dividends, fees) the app's
account-history tab renders.`,
		Example:     "  robinhood-pp-cli brokerage history --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "sensitive-read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostMinerva, "/history/transactions/", "sensitive-read", nil, nil)
		},
	}
	return cmd
}

// ---------------------------------------------------------------------------
// Watchlists — read + safe add/remove writes
// ---------------------------------------------------------------------------

func newBrokerageWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "List watchlists, or add/remove items",
		Long: `Watchlist subcommands.

List the default watchlist (GET https://api.robinhood.com/discovery/lists/default/)
and its items, or add/remove instruments. add/remove are reversible writes and
still honor the PP write gate (dry-run by default).`,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/discovery/lists/default/", "read", nil, nil)
		},
	}
	cmd.AddCommand(newBrokerageWatchlistItemsCmd(flags))
	cmd.AddCommand(newBrokerageWatchlistAddCmd(flags))
	cmd.AddCommand(newBrokerageWatchlistRemoveCmd(flags))
	return cmd
}

func newBrokerageWatchlistItemsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "items",
		Short: "List watchlist items",
		Long: `List the items on the user's watchlists.

Maps GET https://api.robinhood.com/discovery/lists/user_items/.`,
		Example:     "  robinhood-pp-cli brokerage watchlist items --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:risk": "read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrokerageRead(cmd, flags, hostAPI, "/discovery/lists/user_items/", "read", nil, nil)
		},
	}
	return cmd
}

func newBrokerageWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var listID, bodyJSON string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an item to a watchlist (safe, reversible write)",
		Long: `Add an instrument to a watchlist via POST
https://api.robinhood.com/discovery/lists/{list_id}/item_updates/.

This is a safe, reversible write but still honors the PP write gate: it
defaults to --dry-run and a live add requires --live-write AND
ROBINHOOD_PP_ALLOW_WRITES=1. Supply the item payload via --body-json.`,
		Example:     `  robinhood-pp-cli brokerage watchlist add --list-id <uuid> --body-json '{"object_id":"<instrument-uuid>","object_type":"instrument","operation":"create"}' --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:risk": "write-mutate", "pp:barrier": "requires_ROBINHOOD_PP_ALLOW_WRITES"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := parseBrokerageBody(bodyJSON)
			if err != nil {
				return usageErr(err)
			}
			return runBrokerageWrite(cmd, flags, hostAPI, "/discovery/lists/{list_id}/item_updates/", "POST", "write-mutate",
				map[string]string{"list_id": listID}, body)
		},
	}
	cmd.Flags().StringVar(&listID, "list-id", "", "Watchlist UUID; required")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Item-update payload as JSON")
	_ = cmd.MarkFlagRequired("list-id")
	return cmd
}

func newBrokerageWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var listID, bodyJSON string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove an item from a watchlist (safe, reversible write)",
		Long: `Remove an instrument from a watchlist via POST
https://api.robinhood.com/discovery/lists/{list_id}/item_updates/ with a delete
operation.

Defaults to --dry-run; a live remove requires --live-write AND
ROBINHOOD_PP_ALLOW_WRITES=1. Supply the item payload via --body-json.`,
		Example:     `  robinhood-pp-cli brokerage watchlist remove --list-id <uuid> --body-json '{"object_id":"<instrument-uuid>","object_type":"instrument","operation":"delete"}' --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:risk": "write-mutate", "pp:barrier": "requires_ROBINHOOD_PP_ALLOW_WRITES"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := parseBrokerageBody(bodyJSON)
			if err != nil {
				return usageErr(err)
			}
			return runBrokerageWrite(cmd, flags, hostAPI, "/discovery/lists/{list_id}/item_updates/", "POST", "write-mutate",
				map[string]string{"list_id": listID}, body)
		},
	}
	cmd.Flags().StringVar(&listID, "list-id", "", "Watchlist UUID; required")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Item-update payload as JSON")
	_ = cmd.MarkFlagRequired("list-id")
	return cmd
}
