// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored: `watch` command group — saved-query event monitoring.
// Each `watch run` re-runs saved queries against the local store, diffs
// against a per-watch seen-set (so only genuinely-new items surface),
// prints them, and with --notify POSTs to configured webhooks.
// One-shot (cron-friendly), not a daemon.
// `watch remove` hard-deletes the search + its seen-set (R12).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "watch",
		Example: strings.Trim(`
  readwise-pp-cli watch list --json
  readwise-pp-cli watch add --name inbox --kind docs --location new
  readwise-pp-cli watch run --json
`, "\n"),
		Short: "Save queries and get alerted to new docs/highlights (via 'watch run --notify')",
		// off the agent MCP surface; subcommands mutate the saved-search store + fire outbound webhooks
		// watch group itself only prints help: read-only so live dogfood runs its first (list) Example for real
		Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchDiffCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath, name, kind, location, category, tag, query string
	var includeExisting bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Save a query to monitor for new docs or highlights",
		Example: `  readwise-pp-cli watch add --name inbox --kind docs --location new
  readwise-pp-cli watch add --name stoicism --kind highlights --query stoicism`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch add")
			}
			if name == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("watch add requires --name"))
			}
			switch kind {
			case "docs", "highlights":
				// valid
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--kind must be docs or highlights (got %q)", kind))
			}
			// Build filters JSON
			filters := map[string]any{}
			if kind == "docs" {
				if location != "" {
					filters["location"] = splitWatchCSV(location)
				}
				if category != "" {
					filters["category"] = splitWatchCSV(category)
				}
				if tag != "" {
					filters["tag"] = splitWatchCSV(tag)
				}
			} else {
				if query != "" {
					filters["query"] = query
				}
			}
			filtersJSON, err := json.Marshal(filters)
			if err != nil {
				return fmt.Errorf("marshal filters: %w", err)
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			id, err := db.AddWatch(cmd.Context(), name, kind, string(filtersJSON))
			if err != nil {
				return err
			}
			baselineSeeded := 0
			if !includeExisting {
				resource := "highlights"
				if kind == "docs" {
					resource = "documents"
				}
				_, at, _, err := db.GetSyncState(resource)
				if err != nil {
					return err
				}
				if at.IsZero() {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning: watched resource has never completed a sync; baseline may be empty; sync before notifying")
				}
				w := store.Watch{ID: id, Name: name, Kind: kind, FiltersJSON: string(filtersJSON)}
				for {
					baseline, err := runWatchDiff(cmd.Context(), db, w, false, false)
					if err != nil {
						_, _ = db.RemoveWatch(cmd.Context(), id)
						return fmt.Errorf("seed watch baseline: %w", err)
					}
					baselineSeeded += baseline.NewCount
					if baseline.NewCount == 0 {
						break
					}
					if err := db.MarkSeen(cmd.Context(), id, kind, baseline.NewIDs); err != nil {
						_, _ = db.RemoveWatch(cmd.Context(), id)
						return fmt.Errorf("seed watch baseline: %w", err)
					}
				}
			}
			if flags.asJSON {
				created, err := db.GetWatch(cmd.Context(), strconv.FormatInt(id, 10))
				if err != nil {
					return err
				}
				return printWatchJSON(cmd, struct {
					store.Watch
					BaselineSeeded int `json:"baseline_seeded"`
				}{created, baselineSeeded}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added watch #%d (%s)\n", id, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&name, "name", "", "A label for this watch (required)")
	cmd.Flags().StringVar(&kind, "kind", "", "Watch kind: docs or highlights (required)")
	cmd.Flags().StringVar(&location, "location", "", "Docs: filter by location csv (new,later,shortlist,archive,feed)")
	cmd.Flags().StringVar(&category, "category", "", "Docs: filter by category csv")
	cmd.Flags().StringVar(&tag, "tag", "", "Docs: filter by tag csv (membership in document tags)")
	cmd.Flags().BoolVar(&includeExisting, "include-existing", false, "Treat existing matches as new (default: seed the seen baseline)")
	cmd.Flags().StringVar(&query, "query", "", "Highlights: FTS query")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	// pp:data-source local
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved watches with their id, name, kind, and stored filters",
		Example:     "  readwise-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch list")
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			watches, err := db.ListWatches(cmd.Context())
			if err != nil {
				return err
			}
			if flags.asJSON {
				if watches == nil {
					watches = []store.Watch{}
				}
				return printWatchJSON(cmd, watches, flags)
			}
			if len(watches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no watches saved")
				return nil
			}
			for _, w := range watches {
				fmt.Fprintf(cmd.OutOrStdout(), "#%d\t%s\tkind=%s\tfilters=%s\n",
					w.ID, w.Name, w.Kind, w.FiltersJSON)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use: "remove <id>",
		Example: strings.Trim(`
  readwise-pp-cli watch remove 1 --dry-run --json
  readwise-pp-cli watch remove 1
`, "\n"),
		Aliases:     []string{"rm"},
		Short:       "Hard-delete a saved watch and its seen-history (cascades watch_seen rows)",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch remove")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("watch remove requires an <id>"))
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid watch id %q", args[0]))
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			n, err := db.RemoveWatch(cmd.Context(), id)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printWatchJSON(cmd, map[string]any{"removed": id}, flags)
			}
			if n == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no watch #%d\n", id)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "removed watch #%d (and its seen-history)\n", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "diff <name|id>",
		Short:       "Show new-since-last items for a saved watch (no state change, no notify)",
		Example:     "  readwise-pp-cli watch diff inbox",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch diff")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("watch diff requires a <name|id>"))
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			w, err := db.GetWatch(cmd.Context(), args[0])
			if err != nil {
				return usageErr(err)
			}
			result, err := runWatchDiff(cmd.Context(), db, w, false, false)
			if err != nil {
				return err
			}
			return emitWatchDiffResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var dbPath, name string
	var notify, includeText, strict bool
	cmd := &cobra.Command{
		Use:     "run [--name <name>] [--notify] [--include-text]",
		Short:   "Check saved watches for new items; with --notify, POST them to webhooks",
		Example: "  readwise-pp-cli watch run --notify",
		// --notify makes outbound POSTs; off MCP surface
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch run")
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			var watches []store.Watch
			if name != "" {
				w, err := db.GetWatch(cmd.Context(), name)
				if err != nil {
					return usageErr(err)
				}
				watches = []store.Watch{w}
			} else {
				watches, err = db.ListWatches(cmd.Context())
				if err != nil {
					return err
				}
			}
			if len(watches) == 0 && !flags.asJSON {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no watches saved — add one with 'watch add --name N --kind K'"))
			}

			// Load delivery targets once for notices and empty-recipient checks.
			var allHooks []store.Webhook
			if notify {
				allHooks, err = db.ListWebhooks(cmd.Context())
				if err != nil {
					return err
				}
			}

			// Only stored recipient consent enables text delivery.
			if notify {
				var textEndpoints []string
				for _, h := range allHooks {
					if h.IncludeText {
						textEndpoints = append(textEndpoints, redactWebhookURL(h.URL))
					}
				}
				anyHookText := len(textEndpoints) > 0
				if anyHookText {
					if len(textEndpoints) > 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "notice: highlight text / document summaries will be sent to: %s\n",
							strings.Join(textEndpoints, ", "))
					}
				}
			}

			type watchRunItem struct {
				ID          int64   `json:"id"`
				Name        string  `json:"name"`
				NewCount    int     `json:"new_count"`
				Notified    bool    `json:"notified"`
				FailedHooks []int64 `json:"failed_hooks"`
			}
			rows := make([]watchRunItem, 0, len(watches))
			totalNew := 0
			anyFailed := false
			allFailed := true
			for _, w := range watches {
				rows = append(rows, watchRunItem{ID: w.ID, Name: w.Name, FailedHooks: []int64{}})
				row := &rows[len(rows)-1]
				result, err := runWatchDiff(cmd.Context(), db, w, true, includeText)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch #%d (%s): %v\n", w.ID, w.Name, err)
					anyFailed = true
					continue
				}
				allFailed = false
				newCount := result.NewCount
				row.NewCount = newCount
				totalNew += newCount
				if notify {
					matching := false
					for _, h := range allHooks {
						if webhookWantsKind(h.Events, w.Kind) {
							matching = true
							break
						}
					}
					if !matching {
						fmt.Fprintf(cmd.ErrOrStderr(), "watch #%d notify: no matching webhook succeeded (no matching hooks)\n", w.ID)
						anyFailed = true
						continue
					}
				}
				if newCount == 0 {
					continue
				}
				if !flags.asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "watch #%d (%s): %d new item(s)\n", w.ID, w.Name, newCount)
				}

				if notify {
					body, buildErr := fitWatchPayload(w, &result)
					newCount = result.NewCount
					if buildErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "watch #%d build payload: %v\n", w.ID, buildErr)
						anyFailed = true
						continue
					} else {
						sent, err := fireWatchWebhooks(cmd.Context(), cmd.ErrOrStderr(), db, w.Kind, body, newCount, &row.FailedHooks)
						row.Notified = sent > 0
						if err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "watch #%d notify: %v\n", w.ID, err)
							anyFailed = true
							continue
						}
					}
				}

				if result.DeferredCount > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch #%d: batch capped at %d items; %d deferred to later runs\n", w.ID, watchBatchLimit, result.DeferredCount)
				}
				// Mark new items as seen
				newIDs := result.NewIDs
				if err := db.MarkSeen(cmd.Context(), w.ID, w.Kind, newIDs); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch #%d mark-seen: %v\n", w.ID, err)
					anyFailed = true
				}
			}
			if totalNew == 0 && !allFailed && !flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), "no new items across all watches")
			}
			if flags.asJSON {
				if err := printWatchJSON(cmd, map[string]any{"watches": rows, "total_new": totalNew}, flags); err != nil {
					return err
				}
			}
			// R8: exit codes
			if allFailed && len(watches) > 0 {
				return apiErr(fmt.Errorf("all watches failed"))
			}
			if strict && anyFailed {
				return apiErr(fmt.Errorf("one or more watches failed (--strict)"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&name, "name", "", "Run only this named watch (default: all)")
	cmd.Flags().BoolVar(&notify, "notify", false, "Fire webhooks for new items (default: print only)")
	cmd.Flags().BoolVar(&includeText, "include-text", false, "Fetch text for opted-in hooks only; never overrides recipient consent")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero if any watch fails (default: continue)")
	return cmd
}

const watchBatchLimit = 200

// --- shared helpers ---

// WatchDiffResult is the exported alias of watchDiffResult for use by external
// packages (e.g. internal/mcp) that need the diff result type.
type WatchDiffResult = watchDiffResult

// WatchDiff runs a watch diff for external callers (e.g. MCP tools).
// Returns new item ids/titles/urls and store_last_synced_at.
// Does not mark items as seen and does not fire webhooks.
func WatchDiff(ctx context.Context, db *store.Store, w store.Watch) (WatchDiffResult, error) {
	return runWatchDiff(ctx, db, w, false, false)
}

// watchDiffResult holds the output of a single watch diff pass.
type watchDiffResult struct {
	WatchID                int64
	WatchName              string
	Kind                   string
	NewCount               int
	NewIDs                 []string
	DocItems               []store.DocItem
	HLItems                []store.HLItem
	LastSyncedAt           string
	AnyWebhookIncludesText bool
	DeferredCount          int
}

// runWatchDiff runs the watch's query, diffs against seen-set, returns new items.
// If checkWebhookText is false (diff command), does NOT update seen-set.
// includeText controls whether highlight text is fetched from the store; it is OR'd
// with AnyWebhookIncludesText so callers only fetch text when delivery actually needs it.
func runWatchDiff(ctx context.Context, db *store.Store, w store.Watch, checkWebhookText bool, includeText bool) (watchDiffResult, error) {
	result := watchDiffResult{WatchID: w.ID, WatchName: w.Name, Kind: w.Kind}

	// Get last-synced-at for the relevant resource type
	resource := "documents"
	if w.Kind == "highlights" {
		resource = "highlights"
	}
	_, lastSynced, _, _ := db.GetSyncState(resource)
	if !lastSynced.IsZero() {
		result.LastSyncedAt = lastSynced.UTC().Format(time.RFC3339)
	}

	// Check if any webhook has include_text enabled
	if checkWebhookText {
		hooks, _ := db.ListWebhooks(ctx)
		for _, h := range hooks {
			if h.IncludeText && webhookWantsKind(h.Events, w.Kind) {
				result.AnyWebhookIncludesText = true
				break
			}
		}
	}

	// Parse filters
	var filters map[string]any
	if w.FiltersJSON != "" && w.FiltersJSON != "{}" {
		_ = json.Unmarshal([]byte(w.FiltersJSON), &filters)
	}

	seen, err := db.SeenItemIDs(ctx, w.ID, w.Kind)
	if err != nil {
		return result, fmt.Errorf("seen ids: %w", err)
	}

	switch w.Kind {
	case "docs":
		locations := toStringSlice(filters["location"])
		categories := toStringSlice(filters["category"])
		tags := toStringSlice(filters["tag"])
		docs, err := db.ListDocItems(ctx, locations, categories, tags)
		if err != nil {
			return result, err
		}
		var newDocs []store.DocItem
		var newIDs []string
		for _, d := range docs {
			if !seen[d.ID] {
				newDocs = append(newDocs, d)
				newIDs = append(newIDs, d.ID)
			}
		}
		if len(newDocs) > watchBatchLimit {
			result.DeferredCount = len(newDocs) - watchBatchLimit
			newDocs = newDocs[:watchBatchLimit]
			newIDs = newIDs[:watchBatchLimit]
		}
		result.DocItems = newDocs
		result.NewIDs = newIDs
		result.NewCount = len(newDocs)

	case "highlights":
		query := ""
		if q, ok := filters["query"].(string); ok {
			query = q
		}
		// Fetch text only when the caller or a webhook actually needs it.
		fetchText := includeText || result.AnyWebhookIncludesText
		hls, err := db.ListHighlightItems(ctx, query, fetchText, w.ID)
		if err != nil {
			return result, err
		}
		var newHLs []store.HLItem
		var newIDs []string
		for _, h := range hls {
			if !seen[h.ID] {
				newHLs = append(newHLs, h)
				newIDs = append(newIDs, h.ID)
			}
		}
		result.HLItems = newHLs
		result.NewIDs = newIDs
		result.NewCount = len(newHLs)
	}

	return result, nil
}

// watchDiffJSON is the JSON output shape for watch diff (§11b).
type watchDiffJSON struct {
	Watch             string `json:"watch"`
	Kind              string `json:"kind"`
	NewCount          int    `json:"new_count"`
	StoreLastSyncedAt string `json:"store_last_synced_at"`
	Items             any    `json:"items"`
}

func emitWatchDiffResult(cmd *cobra.Command, flags *rootFlags, result watchDiffResult) error {
	if flags.asJSON {
		var items any
		switch result.Kind {
		case "docs":
			minItems := make([]map[string]string, 0, len(result.DocItems))
			for _, d := range result.DocItems {
				minItems = append(minItems, map[string]string{"id": d.ID, "title": d.Title, "author": d.Author, "url": d.URL})
			}
			items = minItems
		case "highlights":
			minItems := make([]map[string]string, 0, len(result.HLItems))
			for _, h := range result.HLItems {
				minItems = append(minItems, map[string]string{"id": h.ID, "title": h.Title, "author": h.Author, "url": h.URL})
			}
			items = minItems
		}
		out := watchDiffJSON{
			Watch:             result.WatchName,
			Kind:              result.Kind,
			NewCount:          result.NewCount,
			StoreLastSyncedAt: result.LastSyncedAt,
			Items:             items,
		}
		return printWatchJSON(cmd, out, flags)
	}
	if result.NewCount == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "watch %q: no new items\n", result.WatchName)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "watch %q: %d new item(s)\n", result.WatchName, result.NewCount)
	switch result.Kind {
	case "docs":
		for _, d := range result.DocItems {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\t%s\n", d.ID[:min8(len(d.ID))], d.Title, d.URL)
		}
	case "highlights":
		for _, h := range result.HLItems {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\t%s\n", h.ID, h.Title, h.URL)
		}
	}
	return nil
}

func min8(n int) int {
	if n < 8 {
		return n
	}
	return 8
}

// openWatchStore opens the local store at the canonical path.
func openWatchStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("readwise-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return db, nil
}

func splitWatchCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch tv := v.(type) {
	case []any:
		out := make([]string, 0, len(tv))
		for _, item := range tv {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if tv == "" {
			return nil
		}
		return []string{tv}
	}
	return nil
}

// fitWatchPayload halves over-cap batches while retaining deferred item ids.
func fitWatchPayload(w store.Watch, result *watchDiffResult) ([]byte, error) {
	for {
		var body []byte
		var err error
		if w.Kind == "docs" {
			body, err = buildDocPayload(w.Name, result.DocItems, result.AnyWebhookIncludesText)
		} else {
			body, err = buildHLPayload(w.Name, result.HLItems, result.AnyWebhookIncludesText)
		}
		if err == nil || !strings.Contains(err.Error(), "256 KiB cap") || result.NewCount <= 1 {
			return body, err
		}
		size := result.NewCount / 2
		result.DeferredCount += result.NewCount - size
		result.NewCount = size
		result.NewIDs = result.NewIDs[:size]
		if w.Kind == "docs" {
			result.DocItems = result.DocItems[:size]
		} else {
			result.HLItems = result.HLItems[:size]
		}
	}
}

// Keep watch result shapes stable in both JSON and agent mode.
func printWatchJSON(cmd *cobra.Command, value any, flags *rootFlags) error {
	outputFlags := *flags
	outputFlags.agent = false
	outputFlags.asJSON = true
	outputFlags.csv = false
	outputFlags.plain = false
	outputFlags.quiet = false
	return printJSONFiltered(cmd.OutOrStdout(), value, &outputFlags)
}
