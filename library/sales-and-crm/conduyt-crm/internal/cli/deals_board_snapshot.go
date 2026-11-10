package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Hand-written (board #116 for app #111, contract frozen at app 5729bc7b4): `deals list --board-snapshot --pipeline <id>
// --board-columns <stageId>[:<limit>],...` reads every named column of one pipeline from ONE database snapshot. The
// generator only adds the two flags to the paginated list; this path refuses the mode's conflicts before any request, never
// pages (--all) or serves a cached answer (a snapshot is a fresh read), never writes the object answer into the local store,
// checks the answer's shape and counts the deals across the columns for the provenance line. A reprint must keep this.

type dealsBoardSnapshotArgs struct {
	columns    string
	pipeline   string
	status     string
	assignedTo string
	sort       string
	order      string
	perPage    int
}

const dealsListBoardSnapshotLong = "List deals (GET /deals). BOARD SNAPSHOT: --board-snapshot with --pipeline <id> and --board-columns <stageId>[:<limit>],... reads every named column of that one pipeline from ONE database snapshot, so a deal moved while the read runs is in exactly one column. It prints { columns: [{ stageId, limit, deals }], meta: { board_snapshot: true } } (under data): columns in the order named, each column's deals exactly what --stage <stageId> --per-page <limit> rows-only answers (same filters, order and projection), no totals; deals.length equal to limit means the column may hold more. 1-50 live stages of the pipeline, no stage twice, limit a whole number 1-1000 (default --per-page, itself default 50), at most 5000 rows in total. Refused before any request (usage error): --page (even 1: a snapshot is always page one), --stage, --all, every other deals list filter (the snapshot forwards only --pipeline, --status, --assigned-to, --sort, --order and --per-page), --board-columns without --board-snapshot, a missing --pipeline or --board-columns, --data-source local, and a --board-columns value the server would refuse. A snapshot is always read live (never the response cache, never written into the local store). 503 (nothing was changed; retried by the client, then the error carries the code and retryAfter): board_snapshot_busy (--board-snapshot only: the one snapshot could not finish in its budget; read the columns one by one with --stage, or retry) and gate_busy (any deals list call, snapshot or not: the access check could not finish in time; retry after Retry-After)."

func runDealsBoardSnapshot(cmd *cobra.Command, flags *rootFlags, snapshot bool, a dealsBoardSnapshotArgs) error {
	if !snapshot {
		return usageErr(fmt.Errorf("--board-columns needs --board-snapshot"))
	}
	var conflicts []string
	for _, name := range []string{"page", "stage", "all"} {
		if cmd.Flags().Changed(name) {
			conflicts = append(conflicts, "--"+name)
		}
	}
	// reprint637: the full public GET /deals adds stage aliases, health_*, rows_only, summary, search and other filters that
	// this snapshot request does not forward; refuse every other local flag instead of silently dropping it.
	snapshotForwards := map[string]bool{"board-snapshot": true, "board-columns": true, "pipeline": true, "per-page": true,
		"status": true, "assigned-to": true, "sort": true, "order": true, "page": true, "stage": true, "all": true}
	local := cmd.LocalNonPersistentFlags()
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if local.Lookup(f.Name) != nil && !snapshotForwards[f.Name] {
			conflicts = append(conflicts, "--"+f.Name)
		}
	})
	if len(conflicts) > 0 {
		return usageErr(fmt.Errorf("--board-snapshot cannot be combined with %s: it answers every named column's first rows, rows only, from one snapshot", strings.Join(conflicts, ", ")))
	}
	if flags.dataSource == "local" {
		return usageErr(fmt.Errorf("--board-snapshot reads the live API (one database snapshot); --data-source local cannot answer it"))
	}
	if strings.TrimSpace(a.pipeline) == "" {
		return usageErr(fmt.Errorf("--board-snapshot needs --pipeline <id>"))
	}
	if !cmd.Flags().Changed("board-columns") {
		return usageErr(fmt.Errorf("--board-snapshot needs --board-columns <stageId>[:<limit>],..."))
	}
	if err := cliutil.CheckPageSize("per-page", a.perPage, cliutil.BoardSnapshotMaxColumnRows); err != nil {
		return usageErr(err)
	}
	columns, err := cliutil.ParseBoardColumns(a.columns, a.perPage)
	if err != nil {
		return usageErr(err)
	}
	params := map[string]string{
		"board_snapshot": "true",
		"board_columns":  columns,
		"pipeline":       a.pipeline,
		"per_page":       strconv.Itoa(a.perPage),
	}
	for k, v := range map[string]string{"status": a.status, "assigned_to": a.assignedTo, "sort": a.sort, "order": a.order} {
		if v != "" {
			params[k] = v
		}
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, err := c.GetWithHeadersNoCache(cmd.Context(), "/deals", params, nil)
	if err != nil {
		return classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	if c.DryRun {
		return nil
	}
	count, ok := boardSnapshotDealCount(data)
	if !ok {
		return apiErr(fmt.Errorf("--board-snapshot: the API answered without { columns, meta: { board_snapshot: true } }; refusing to print another shape"))
	}
	prov := attachFreshness(DataProvenance{Source: "live"}, flags)
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		printProvenance(cmd, count, prov)
	}
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		filtered := data
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		} else if flags.compact {
			filtered = compactFields(filtered)
		}
		wrapped, wrapErr := wrapWithProvenance(filtered, prov)
		if wrapErr != nil {
			return wrapErr
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
}

// boardSnapshotDealCount checks the snapshot answer ({ data: { columns: [{ deals: [...] }], meta: { board_snapshot: true } } or
// the same object unwrapped) and counts the deals across its columns.
func boardSnapshotDealCount(data json.RawMessage) (int, bool) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return 0, false
	}
	if inner, ok := obj["data"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(inner, &nested) != nil {
			return 0, false
		}
		obj = nested
	}
	var meta struct {
		BoardSnapshot bool `json:"board_snapshot"`
	}
	var columns []struct {
		Deals []json.RawMessage `json:"deals"`
	}
	if json.Unmarshal(obj["meta"], &meta) != nil || !meta.BoardSnapshot || json.Unmarshal(obj["columns"], &columns) != nil || columns == nil {
		return 0, false
	}
	n := 0
	for _, col := range columns {
		n += len(col.Deals)
	}
	return n, true
}
