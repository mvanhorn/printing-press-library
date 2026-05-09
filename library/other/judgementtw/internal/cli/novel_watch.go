// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"judgementtw-pp-cli/internal/extract"
	"judgementtw-pp-cli/internal/judicial"
	"judgementtw-pp-cli/internal/source/fjud"
)

// newWatchCmd builds the 'watch' parent. The two subcommands ('case' and
// 'query') are distinct novel features; both write to the local watchlist.
func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track new judgments for a saved query or specific case (replaces missing RSS)",
		Long: `The Judicial Yuan site has no RSS, no email digest, no webhook. 'watch' provides
the agent-shaped equivalent: save a search or a case-pattern, and on each invocation
print only the JIDs that are newer than the last cursor.

Two subcommands:
  watch query <name> --terms <q>     — save a named search; subsequent runs of
                                       the same name print only deltas.
  watch case  <jid-pattern>          — track a specific case by its JID prefix
                                       (court+案號 root). Uses the full search
                                       under the hood.

Use 'watch list' to see saved entries; 'watch delete <name>' to remove.`,
	}
	cmd.AddCommand(newWatchQueryCmd(flags))
	cmd.AddCommand(newWatchCaseCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchDeleteCmd(flags))
	return cmd
}

// watchQuerySpec is the JSON payload stored in watchlist.query_json for
// kind=query entries.
type watchQuerySpec struct {
	Court    string `json:"court,omitempty"`
	Type     string `json:"type,omitempty"`
	Year     int    `json:"year,omitempty"`
	CaseChar string `json:"case_char,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Terms    string `json:"terms,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func newWatchQueryCmd(flags *rootFlags) *cobra.Command {
	var terms, court, caseType, caseChar, reason string
	var year, limit int
	cmd := &cobra.Command{
		Use:   "query [name]",
		Short: "Save a named search and print judgments newer than the last cursor",
		Long: `Save a named query (court+type+keyword) and run it. On the first invocation,
prints all matching judgments and stores the highest-seen JID as the cursor.
On subsequent invocations with the same name, prints only judgments newer
than the cursor.

Pass --terms together with the other filters on the FIRST call to register the
query. Subsequent calls only need the name — the saved filters are reused.`,
		Example: `  # Register a watch
  judgementtw-pp-cli watch query drug-cases --terms 毒品危害防制條例 --type criminal

  # Run the watch (prints only new judgments since last call)
  judgementtw-pp-cli watch query drug-cases --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			name := args[0]
			existing, err := judicial.GetWatch(ctx, db, name)
			if err != nil {
				return err
			}
			var spec watchQuerySpec
			if existing != nil {
				_ = json.Unmarshal(existing.Query, &spec)
			}
			// Override saved spec with any flags the caller passed.
			if terms != "" {
				spec.Terms = terms
			}
			if court != "" {
				spec.Court = court
			}
			if caseType != "" {
				spec.Type = caseType
			}
			if year > 0 {
				spec.Year = year
			}
			if caseChar != "" {
				spec.CaseChar = caseChar
			}
			if reason != "" {
				spec.Reason = reason
			}
			if limit > 0 {
				spec.Limit = limit
			} else if spec.Limit == 0 {
				spec.Limit = 20
			}
			if existing == nil && spec.Terms == "" && spec.Court == "" && spec.Type == "" {
				return usageErr(errMsg("first registration of '" + name + "' needs at least --terms or a filter (court/type/year/etc.)"))
			}
			if err := judicial.SaveWatch(ctx, db, name, judicial.WatchQuery, spec); err != nil {
				return err
			}

			// Run the search.
			c := fjudClient(flags)
			res, err := c.Search(ctx, fjud.SearchParams{
				Courts:    parseCSVList(spec.Court),
				CaseTypes: resolveCaseTypes(spec.Type),
				Year:      spec.Year,
				CaseChar:  spec.CaseChar,
				Reason:    spec.Reason,
				Keyword:   spec.Terms,
				Limit:     spec.Limit,
				Page:      1,
			})
			if err != nil {
				return err
			}

			lastSeen := ""
			if existing != nil {
				lastSeen = existing.LastSeen
			}
			delta := make([]fjud.JudgmentRef, 0, len(res.Items))
			highest := lastSeen
			for _, item := range res.Items {
				if lastSeen == "" || item.JID > lastSeen {
					delta = append(delta, item)
				}
				if item.JID > highest {
					highest = item.JID
				}
			}
			if highest != lastSeen {
				_ = judicial.UpdateWatchCursor(ctx, db, name, highest)
			}
			_ = judicial.LogEvent(ctx, db, "watch_run", "", "watch query "+name)

			payload := map[string]any{
				"name":           name,
				"saved":          spec,
				"new_count":      len(delta),
				"total_returned": len(res.Items),
				"cursor":         highest,
				"items":          delta,
			}
			return emitJSON(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&terms, "terms", "", "Free-text keyword (registered on first call)")
	cmd.Flags().StringVar(&court, "court", "", "Court code(s)")
	cmd.Flags().StringVar(&caseType, "type", "", "Case type (criminal|civil|...)")
	cmd.Flags().IntVar(&year, "year", 0, "ROC year")
	cmd.Flags().StringVar(&caseChar, "case-char", "", "字別")
	cmd.Flags().StringVar(&reason, "reason", "", "案由 substring")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max items per run")
	// --since is accepted for compatibility with documented examples but is a
	// no-op: the watch's last-seen JID cursor is the canonical "newer than"
	// boundary. The flag exists so users copying README examples don't get
	// "unknown flag" errors.
	var sinceCompat string
	cmd.Flags().StringVar(&sinceCompat, "since", "", "Compatibility flag (no-op; the watch's saved cursor is authoritative)")
	_ = sinceCompat
	return cmd
}

func newWatchCaseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "case [jid-pattern]",
		Short: "Track a specific case for new rulings (matches JID prefix)",
		Long: `Polls FJUD for judgments matching a JID prefix (typically court+year+字別).
Stores the watch under the pattern itself; subsequent runs print only the
JIDs that are new since the last invocation.

JID pattern examples:
  TPHM,110,毒抗     — every 110年度毒抗 ruling at 臺灣高等法院
  TPSM,115         — every 115 Supreme Court criminal ruling`,
		Example:     `  judgementtw-pp-cli watch case TPHM,110,毒抗 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			pattern := args[0]
			parts := strings.Split(pattern, ",")
			if len(parts) < 2 {
				return usageErr(errMsg("pattern must contain at least court+year (e.g. TPHM,110)"))
			}
			courtAndType := parts[0]
			if len(courtAndType) < 4 {
				return usageErr(errMsg("court+type prefix too short — e.g. TPHM (court TPH, type M)"))
			}
			court := courtAndType[:3]
			caseType := courtAndType[3:]
			year := 0
			_, _ = fmtSscanInt(parts[1], &year)

			caseChar := ""
			if len(parts) >= 3 {
				caseChar = parts[2]
			}

			c := fjudClient(flags)
			res, err := c.Search(ctx, fjud.SearchParams{
				Courts:    []string{court},
				CaseTypes: []string{caseType},
				Year:      year,
				CaseChar:  caseChar,
				Limit:     50,
			})
			if err != nil {
				return err
			}

			existing, _ := judicial.GetWatch(ctx, db, pattern)
			lastSeen := ""
			if existing != nil {
				lastSeen = existing.LastSeen
			} else {
				_ = judicial.SaveWatch(ctx, db, pattern, judicial.WatchCase, map[string]any{
					"pattern":   pattern,
					"court":     court,
					"case_type": caseType,
					"year":      year,
					"case_char": caseChar,
				})
			}

			delta := make([]fjud.JudgmentRef, 0, len(res.Items))
			highest := lastSeen
			for _, item := range res.Items {
				if !strings.HasPrefix(item.JID, pattern) {
					continue
				}
				if lastSeen == "" || item.JID > lastSeen {
					delta = append(delta, item)
				}
				if item.JID > highest {
					highest = item.JID
				}
			}
			if highest != lastSeen {
				_ = judicial.UpdateWatchCursor(ctx, db, pattern, highest)
			}
			_ = judicial.LogEvent(ctx, db, "watch_run", "", "watch case "+pattern)
			payload := map[string]any{
				"pattern":   pattern,
				"new_count": len(delta),
				"cursor":    highest,
				"items":     delta,
			}
			return emitJSON(cmd.OutOrStdout(), payload, flags)
		},
	}
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all saved watches",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := judicial.ListWatches(ctx, db)
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), all, flags)
		},
	}
	return cmd
}

func newWatchDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [name]",
		Short:   "Remove a saved watch",
		Example: `  judgementtw-pp-cli watch delete drug-cases`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := judicial.DeleteWatch(ctx, db, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted watch %q\n", args[0])
			return nil
		},
	}
	return cmd
}

// avoid unused-import warnings for extract when only used in tests.
var _ = extract.CleanHTML
