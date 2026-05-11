package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newWatchCmd adds the watchlist commands. The watchlist is the explicit-ID
// cursor sync needs since the Unify Data API has no list-records endpoint.
func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage the watchlist of records sync should refresh",
		Long: `The Unify Data API has no LIST or SEARCH endpoint for records, so 'sync'
needs an explicit list of records to refresh. The watchlist holds (object,
match-key, match-value) tuples that sync uses to call find-unique for each.

Add entries with 'watch add', remove with 'watch remove', list with 'watch list'.`,
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var matchPairs []string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "add <object>",
		Short: "Add a (object, match-key=value) entry to the watchlist",
		Example: strings.Trim(`
  unify-pp-cli watch add company --match domain=gladly.com
  unify-pp-cli watch add salesforce_account --match domain=stripe.com --match domain=plaid.com
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			object := args[0]
			if len(matchPairs) == 0 {
				return usageErr(fmt.Errorf("at least one --match key=value pair is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			added := 0
			for _, p := range matchPairs {
				idx := strings.IndexByte(p, '=')
				if idx <= 0 {
					return usageErr(fmt.Errorf("--match must be key=value, got %q", p))
				}
				key := strings.TrimSpace(p[:idx])
				val := strings.TrimSpace(p[idx+1:])
				if key == "" || val == "" {
					return usageErr(fmt.Errorf("--match key and value must both be non-empty, got %q", p))
				}
				if err := s.AddWatch(ctx, store.WatchEntry{ObjectName: object, MatchKey: key, MatchValue: val}); err != nil {
					return apiErr(err)
				}
				added++
			}
			out := map[string]any{"added": added, "object": object}
			blob, _ := json.MarshalIndent(out, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringArrayVar(&matchPairs, "match", nil, "Match expression (key=value); repeat for multiple")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var object string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List watchlist entries (object, match key, match value, age) optionally filtered by object",
		Aliases:     []string{"ls"},
		Example:     "  unify-pp-cli watch list --object company --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			entries, err := s.ListWatch(ctx, object)
			if err != nil {
				return apiErr(err)
			}
			view := make([]map[string]any, 0, len(entries))
			for _, e := range entries {
				view = append(view, map[string]any{
					"object_name": e.ObjectName,
					"match_key":   e.MatchKey,
					"match_value": e.MatchValue,
					"added":       secondsAgo(e.AddedAt),
				})
			}
			blob, _ := json.MarshalIndent(view, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().StringVar(&object, "object", "", "Filter by object name")
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var matchPair string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "remove <object>",
		Aliases:     []string{"rm"},
		Short:       "Remove a watchlist entry",
		Example:     "  unify-pp-cli watch remove company --match domain=gladly.com",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			object := args[0]
			if matchPair == "" {
				return usageErr(fmt.Errorf("--match key=value required"))
			}
			idx := strings.IndexByte(matchPair, '=')
			if idx <= 0 {
				return usageErr(fmt.Errorf("--match must be key=value, got %q", matchPair))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			removed, err := s.RemoveWatch(ctx, object, strings.TrimSpace(matchPair[:idx]), strings.TrimSpace(matchPair[idx+1:]))
			if err != nil {
				return apiErr(err)
			}
			out := map[string]any{"removed": removed, "object": object, "match": matchPair}
			blob, _ := json.MarshalIndent(out, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&matchPair, "match", "", "Match expression (key=value)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	return cmd
}
