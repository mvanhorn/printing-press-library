// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// psx_tables.go implements the PSX data-portal surfaces that are served as HTML
// table fragments rather than JSON. Every row is keyed by its own <th> text via
// internal/psx, never by column position, because PSX reorders columns without
// notice and position-indexed parsing fails silently with plausible-looking
// numbers.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/psx"
)

// psxClient builds the sibling portal client. Root --timeout bounds it via
// boundCtx at each call site; the client itself carries the portal's required
// headers and the self-imposed politeness limiter.
func psxClient(flags *rootFlags) *psx.Client {
	timeout := 30 * time.Second
	rate := 0.0
	if flags != nil {
		if flags.timeout > 0 {
			timeout = flags.timeout
		}
		rate = flags.rateLimit
	}
	return psx.NewWithRate(timeout, rate)
}

// tableView is the machine-facing envelope for every table-backed command.
type tableView struct {
	Source  string              `json:"source"`
	Table   string              `json:"table,omitempty"`
	Headers []string            `json:"headers"`
	Count   int                 `json:"count"`
	Rows    []map[string]string `json:"rows"`
	Note    string              `json:"note,omitempty"`
}

// emitTable renders a parsed table through the standard output helpers so
// --json, --agent, --select, --csv and --quiet all behave identically to
// generated commands. Empty results stay valid JSON rather than becoming null.
func emitTable(cmd *cobra.Command, flags *rootFlags, source string, t psx.Table, limit int, note string) error {
	rows := t.Rows
	if rows == nil {
		rows = make([]map[string]string, 0)
	}
	headers := t.Headers
	if headers == nil {
		headers = []string{}
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	view := tableView{
		Source:  source,
		Table:   t.ID,
		Headers: headers,
		Count:   len(rows),
		Rows:    rows,
		Note:    note,
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No rows returned.")
		if note != "" {
			fmt.Fprintln(cmd.OutOrStdout(), note)
		}
		return nil
	}
	generic := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m := make(map[string]any, len(r))
		for k, v := range r {
			m[k] = v
		}
		generic = append(generic, m)
	}
	return printAutoTable(cmd.OutOrStdout(), generic)
}

// fetchTable GETs a portal path and returns the first table matching the
// selector. A path that returns no usable table is an explicit error rather
// than an empty success, so a portal redesign cannot masquerade as "no data".
func fetchTable(ctx context.Context, c *psx.Client, path, tableID string, mustHave ...string) (psx.Table, error) {
	tables, err := c.GetTables(ctx, path)
	if err != nil {
		return psx.Table{}, err
	}
	t, ok := psx.FindTable(tables, tableID, mustHave...)
	if !ok {
		return psx.Table{}, fmt.Errorf("no table with the expected columns at %s; the portal layout may have changed (run 'psx-pp-cli doctor')", path)
	}
	return t, nil
}

// tableCmd describes one GET-and-parse command.
type tableCmd struct {
	use         string
	short       string
	long        string
	example     string
	path        string   // may contain %s placeholders for positionals
	positionals []string // names, in order, substituted into path
	tableID     string
	mustHave    []string
	defaultLim  int
}

func newTableCmd(flags *rootFlags, spec tableCmd) *cobra.Command {
	var limit int
	use := spec.use
	for _, p := range spec.positionals {
		use += " <" + p + ">"
	}
	cmd := &cobra.Command{
		Use:         use,
		Short:       spec.short,
		Long:        spec.long,
		Example:     spec.example,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && len(spec.positionals) > 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, spec.use)
			}
			if len(args) < len(spec.positionals) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required argument: %s", strings.Join(spec.positionals[len(args):], ", ")))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			path := spec.path
			for _, a := range args[:len(spec.positionals)] {
				path = strings.Replace(path, "%s", url.PathEscape(strings.TrimSpace(a)), 1)
			}
			t, err := fetchTable(ctx, psxClient(flags), path, spec.tableID, spec.mustHave...)
			if err != nil {
				return err
			}
			return emitTable(cmd, flags, path, t, limit, "")
		},
	}
	def := spec.defaultLim
	cmd.Flags().IntVar(&limit, "limit", def, "maximum rows to return (0 = all)")
	return cmd
}

// ---- market ----------------------------------------------------------------

func newPsxMarketCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "market",
		Short:       "Whole-market snapshots, movers and session status",
		Example:     "  psx-pp-cli market watch --limit 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTableCmd(flags, tableCmd{
		use:      "watch",
		short:    "Whole-market snapshot: LDCP, open, high, low, current, change and volume",
		example:  "  psx-pp-cli market watch --limit 20 --json",
		path:     "/market-watch",
		mustHave: []string{"symbol", "current"},
	}))
	cmd.AddCommand(newPerformersCmd(flags))
	cmd.AddCommand(newDebtPerformersCmd(flags))
	cmd.AddCommand(newMarketStatusCmd(flags))
	return cmd
}

// newDebtPerformersCmd ranks movers on the debt market. It is a distinct
// endpoint upstream, so it stays a distinct command rather than a flag on the
// equity movers table.
func newDebtPerformersCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "debt-performers",
		Short: "Top movers on the debt market",
		Long: "Use this command for movers on the debt market.\n" +
			"Do NOT use it for equity movers; use 'market performers' instead.",
		Example:     "  psx-pp-cli market debt-performers --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market debt-performers")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			tables, err := psxClient(flags).GetTables(ctx, "/debt-performers")
			if err != nil {
				return err
			}
			for _, t := range tables {
				if len(t.Rows) > 0 {
					return emitTable(cmd, flags, "/debt-performers", t, limit, "")
				}
			}
			return emitTable(cmd, flags, "/debt-performers", psx.Table{Rows: make([]map[string]string, 0)}, 0,
				"the debt-market movers table is empty right now")
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to return (0 = all)")
	return cmd
}

// newPerformersCmd exposes the portal's three mover tables (active, gainers,
// losers) behind one --kind flag. The portal returns all three in one response,
// so selecting a kind costs no extra request.
func newPerformersCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var market string
	var limit int
	cmd := &cobra.Command{
		Use:   "performers",
		Short: "Top active, gaining and losing instruments",
		Long: "Use this command for the plain top movers ranking straight from the portal.\n" +
			"Do NOT use it to find names abnormal versus their own history; use 'unusual' instead.",
		Example:     "  psx-pp-cli market performers --kind gainers --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market performers")
			}
			idx, err := performerIndex(kind)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			path := "/performers"
			if strings.EqualFold(market, "debt") {
				path = "/debt-performers"
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			tables, err := psxClient(flags).GetTables(ctx, path)
			if err != nil {
				return err
			}
			// Index into the UNFILTERED list. Dropping empty tables first
			// would shift the ordinals, so an empty "active" table would make
			// --kind gainers silently return losers, labelled as gainers.
			if len(tables) == 0 {
				return fmt.Errorf("no mover tables at %s; the portal layout may have changed", path)
			}
			if idx >= len(tables) {
				return fmt.Errorf("%s does not publish a %q table (found %d mover tables)", path, kind, len(tables))
			}
			selected := tables[idx]
			note := ""
			if len(selected.Rows) == 0 {
				note = fmt.Sprintf("the %q table is empty in this session", kind)
			}
			return emitTable(cmd, flags, path+"#"+kind, selected, limit, note)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "active", "which mover table: active, gainers or losers")
	cmd.Flags().StringVar(&market, "market", "equity", "market to rank: equity or debt")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to return (0 = all)")
	return cmd
}

// performerIndex maps a mover kind to its position among the portal's mover
// tables. The portal emits them in a fixed order: active, gainers, losers.
func performerIndex(kind string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "active", "":
		return 0, nil
	case "gainers", "gainer", "up":
		return 1, nil
	case "losers", "loser", "down":
		return 2, nil
	default:
		return 0, fmt.Errorf("--kind must be one of active, gainers, losers (got %q)", kind)
	}
}

// newMarketStatusCmd reports whether the tape is moving by measuring the age of
// the newest KSE100 tick. The portal exposes no explicit open/closed flag, so
// freshness is derived rather than claimed.
func newMarketStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Session status derived from the age of the latest index tick",
		Example:     "  psx-pp-cli market status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market status")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ticks, err := fetchTuples(ctx, psxClient(flags), "/timeseries/int/KSE100")
			if err != nil {
				return err
			}
			type statusView struct {
				Index      string  `json:"index"`
				LastTickAt string  `json:"last_tick_at"`
				AgeSeconds int64   `json:"age_seconds"`
				Level      float64 `json:"level"`
				Fresh      bool    `json:"fresh"`
				Note       string  `json:"note"`
			}
			if len(ticks) == 0 {
				return fmt.Errorf("no KSE100 ticks returned; the portal may be unavailable")
			}
			newest := ticks[0]
			for _, t := range ticks {
				if t.Epoch > newest.Epoch {
					newest = t
				}
			}
			age := time.Since(time.Unix(newest.Epoch, 0))
			view := statusView{
				Index:      "KSE100",
				LastTickAt: time.Unix(newest.Epoch, 0).UTC().Format(time.RFC3339),
				AgeSeconds: int64(age.Seconds()),
				Level:      newest.Value,
				Fresh:      age < 15*time.Minute,
				Note:       "PSX publishes no explicit session flag; freshness is derived from the newest KSE100 tick.",
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			state := "stale (market likely closed)"
			if view.Fresh {
				state = "fresh (market likely open)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "KSE100 %.2f  last tick %s  age %s  %s\n",
				view.Level, view.LastTickAt, age.Round(time.Second), state)
			return nil
		},
	}
	return cmd
}

// ---- simple single-table surfaces -----------------------------------------

func newPsxScreenerCmd(flags *rootFlags) *cobra.Command {
	return newTableCmd(flags, tableCmd{
		use:   "screener",
		short: "Fundamental metrics for the listed universe",
		long: "Use this command to filter the universe by current valuation metrics.\n" +
			"Do NOT use it to trace one symbol's metric over time; use 'drift' instead.",
		example:  "  psx-pp-cli screener --limit 25 --json",
		path:     "/screener",
		tableID:  "screenerTable",
		mustHave: []string{"symbol"},
	})
}

func newPsxDebtCmd(flags *rootFlags) *cobra.Command {
	return newTableCmd(flags, tableCmd{
		use:     "debt",
		short:   "Debt market instruments: TFCs, Sukuks and government paper",
		example: "  psx-pp-cli debt --limit 20 --json",
		path:    "/debt-market",
	})
}

func newPsxEligibleScripsCmd(flags *rootFlags) *cobra.Command {
	return newTableCmd(flags, tableCmd{
		use:     "eligible-scrips",
		short:   "Securities eligible for margin trading and margin financing",
		example: "  psx-pp-cli eligible-scrips --limit 20 --json",
		path:    "/eligible-scrips",
	})
}

func newPsxIndicesCmd(flags *rootFlags) *cobra.Command {
	return newTableCmd(flags, tableCmd{
		use:     "indices",
		short:   "All PSX indices with current level and change",
		example: "  psx-pp-cli indices --json",
		path:    "/indices",
	})
}

func newPsxCircuitBreakersCmd(flags *rootFlags) *cobra.Command {
	return newTableCmd(flags, tableCmd{
		use:     "circuit-breakers",
		short:   "Instruments that hit a price circuit breaker",
		example: "  psx-pp-cli circuit-breakers --json",
		path:    "/circuit-breakers",
	})
}

func newPsxListingsCmd(flags *rootFlags) *cobra.Command {
	return newTableCmd(flags, tableCmd{
		use:         "listings",
		short:       "Listed companies filtered by board and listing status",
		example:     "  psx-pp-cli listings main nc --json",
		path:        "/listings-table/%s/%s",
		positionals: []string{"board", "status"},
	})
}

func newPsxBoardCmd(flags *rootFlags) *cobra.Command {
	cmd := newTableCmd(flags, tableCmd{
		use:   "board",
		short: "Trading board with bid/offer depth for one market and board",
		long: "Use this command to inspect depth on a single market and board.\n" +
			"Do NOT use it to compare futures against spot; use 'basis' instead.\n" +
			"Markets: REG regular, ODL odd lot, DFC deliverable futures, SQR square up, CSF cash settled futures.\n" +
			"Boards: main, gem, bnb.",
		example:     "  psx-pp-cli board REG main --limit 20 --json",
		path:        "/trading-board/%s/%s",
		positionals: []string{"market", "board"},
	})
	cmd.Annotations["pp:happy-args"] = "market=REG;board=main"
	return cmd
}

func newPsxSectorsExtraCmds(flags *rootFlags) []*cobra.Command {
	summary := newTableCmd(flags, tableCmd{
		use:   "summary",
		short: "Per-sector aggregates: advances, declines, turnover and market cap",
		long: "Use this command for the current per-sector aggregate table.\n" +
			"Do NOT use it to rank sectors by movement over a window; use 'rotation' instead.",
		example: "  psx-pp-cli sectors summary --limit 20 --json",
		path:    "/sector-summary/sectorwise",
	})
	return []*cobra.Command{summary}
}

func newPsxCompanyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "company",
		Short:       "Per-company profile and financial report index",
		Example:     "  psx-pp-cli company reports OGDC --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	profile := newTableCmd(flags, tableCmd{
		use:         "profile",
		short:       "Company profile and quote page for one instrument",
		example:     "  psx-pp-cli company profile OGDC --json",
		path:        "/company/%s",
		positionals: []string{"symbol"},
	})
	profile.Annotations["pp:happy-args"] = "symbol=OGDC"
	cmd.AddCommand(profile)

	reports := newTableCmd(flags, tableCmd{
		use:         "reports",
		short:       "Financial report index for one instrument",
		example:     "  psx-pp-cli company reports OGDC --json",
		path:        "/company/reports/%s",
		positionals: []string{"symbol"},
	})
	reports.Annotations["pp:happy-args"] = "symbol=OGDC"
	cmd.AddCommand(reports)
	return cmd
}

// ---- tuple (JSON array-of-arrays) helpers ---------------------------------

// Tuple is one point from the portal's timeseries endpoints. Intraday tuples
// are [epoch, price, volume]; EOD tuples are [epoch, close, volume, open]
// (the fourth element was confirmed as open by cross-checking a session
// against the /historical table, not assumed).
type Tuple struct {
	Epoch   int64
	Value   float64
	Volume  float64
	Open    float64
	HasOpen bool
}

func fetchTuples(ctx context.Context, c *psx.Client, path string) ([]Tuple, error) {
	raw, err := c.GetEnvelope(ctx, path)
	if err != nil {
		return nil, err
	}
	var rows [][]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	out := make([]Tuple, 0, len(rows))
	for _, r := range rows {
		if len(r) < 2 {
			continue
		}
		epoch, ok := tupleNumber(r[0])
		if !ok {
			continue
		}
		t := Tuple{Epoch: int64(epoch)}
		if v, ok := tupleNumber(r[1]); ok {
			t.Value = v
		}
		if len(r) > 2 {
			if v, ok := tupleNumber(r[2]); ok {
				t.Volume = v
			}
		}
		if len(r) > 3 {
			if v, ok := tupleNumber(r[3]); ok {
				t.Open = v
				t.HasOpen = true
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// tupleNumber accepts both JSON numbers and JSON-encoded numeric strings.
// Feeds that switch to string encoding are a known silent-zeroing bug class:
// unmarshalling "1.91" into a float64 field succeeds with a zero value and no
// error, which would quietly empty every aggregate downstream.
func tupleNumber(raw json.RawMessage) (float64, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
		if s == "" {
			return 0, false
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}
