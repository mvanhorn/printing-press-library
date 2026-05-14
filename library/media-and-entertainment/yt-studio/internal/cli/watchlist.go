package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Manage competitor channel watchlist (suggest, add, remove, list)",
		Long:  "Watchlist groups competitor channels for the vs-watchlist and idea-gap commands.",
	}
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	cmd.AddCommand(newWatchlistSuggestCmd(flags))
	return cmd
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List channels currently on the watchlist",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()
			entries, err := ytstore.ListWatchlist(ctx, db)
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{"entries": entries, "count": len(entries)})
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(empty watchlist; run `yt-studio-pp-cli watchlist suggest` or `watchlist add`)")
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s\n", e.ChannelID, e.Handle, e.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var handle, title, niche, dbPath string
	cmd := &cobra.Command{
		Use:     "add [channel_id]",
		Short:   "Add a channel to the watchlist",
		Example: "  yt-studio-pp-cli watchlist add UC_xxxxxxxxxxxxxxxxxxxxxx --handle @creator --niche poe2",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()
			channelID := args[0]
			if err := ytstore.AddToWatchlist(ctx, db, channelID, handle, title, niche); err != nil {
				return err
			}
			res := map[string]any{"channel_id": channelID, "handle": handle, "title": title, "niche": niche, "added": true}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s to watchlist\n", channelID)
			return nil
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "Channel handle (@... )")
	cmd.Flags().StringVar(&title, "title", "", "Channel display title")
	cmd.Flags().StringVar(&niche, "niche", "", "Niche tag (e.g. poe2, last-epoch)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "remove [channel_id]",
		Short:   "Remove a channel from the watchlist",
		Example: "  yt-studio-pp-cli watchlist remove UC_xxxxxxxxxxxxxxxxxxxxxx",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()
			channelID := args[0]
			if err := ytstore.RemoveFromWatchlist(ctx, db, channelID); err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{"channel_id": channelID, "removed": true})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from watchlist\n", channelID)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func newWatchlistSuggestCmd(flags *rootFlags) *cobra.Command {
	var (
		niche   string
		top     int
		live    bool
		dbPath  string
		minSubs int
	)
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Suggest watchlist candidates via the YouTube Search API (uses 100 quota units per call)",
		Long: strings.TrimSpace(`
Auto-discovers competitor channels by querying search.list with the provided
niche keywords. By default this is a DRY suggestion based on the local store;
pass --live to actually call search.list (100 quota units per request).

Returns the top-N candidates ranked by recency. Use 'watchlist add' to
materialize a selection.`),
		Example:     "  yt-studio-pp-cli watchlist suggest --niche \"poe2,last-epoch\" --top 10 --live",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if niche == "" {
				return usageErr(errors.New("--niche is required (comma-separated keywords)"))
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			if !live {
				// Suggest from already-synced search idea-gap entries
				suggestions, err := suggestFromStore(ctx, db, niche, top)
				if err != nil {
					return err
				}
				return emitSuggestions(cmd, flags, suggestions, niche, "store")
			}

			token, err := loadOAuthToken(flags)
			if err != nil {
				return err
			}
			suggestions, err := suggestLive(ctx, token, niche, top, minSubs)
			if err != nil {
				return err
			}
			_ = ytstore.LogQuota(ctx, db, "search.list", 100)
			return emitSuggestions(cmd, flags, suggestions, niche, "live")
		},
	}
	cmd.Flags().StringVar(&niche, "niche", "", "Comma-separated keywords (e.g. \"poe2,last-epoch\")")
	cmd.Flags().IntVar(&top, "top", 20, "Top-N candidates")
	cmd.Flags().BoolVar(&live, "live", false, "Call the live Search API (100 quota units)")
	cmd.Flags().IntVar(&minSubs, "min-subs", 0, "Minimum subscriber count filter (live mode)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

// WatchlistSuggestion is a single competitor channel candidate.
type WatchlistSuggestion struct {
	ChannelID string `json:"channel_id"`
	Handle    string `json:"handle,omitempty"`
	Title     string `json:"title,omitempty"`
	Reason    string `json:"reason"`
}

func suggestFromStore(ctx context.Context, db *sql.DB, niche string, top int) ([]WatchlistSuggestion, error) {
	// Pure local suggestion: return distinct competitor channels from the
	// idea-gap table that aren't already on the watchlist, capped at top.
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT i.competitor_channel_id, COALESCE(c.handle,''), COALESCE(c.title,'')
		FROM yt_search_idea_gap i
		LEFT JOIN yt_channels c ON c.channel_id = i.competitor_channel_id
		WHERE i.competitor_channel_id NOT IN (SELECT channel_id FROM yt_watchlist)
		LIMIT ?
	`, top)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchlistSuggestion
	for rows.Next() {
		var s WatchlistSuggestion
		if err := rows.Scan(&s.ChannelID, &s.Handle, &s.Title); err != nil {
			return nil, err
		}
		s.Reason = "seen in idea-gap snapshots"
		out = append(out, s)
	}
	return out, rows.Err()
}

func suggestLive(ctx context.Context, token, niche string, top, minSubs int) ([]WatchlistSuggestion, error) {
	keywords := strings.Split(niche, ",")
	for i := range keywords {
		keywords[i] = strings.TrimSpace(keywords[i])
	}
	q := strings.Join(keywords, " ")
	if q == "" {
		return nil, errors.New("empty niche")
	}
	v := url.Values{}
	v.Set("part", "snippet")
	v.Set("type", "channel")
	v.Set("q", q)
	v.Set("order", "relevance")
	v.Set("maxResults", "50")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/youtube/v3/search?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, authErr(fmt.Errorf("search.list auth failed: %s", string(body)))
	}
	if resp.StatusCode == 429 {
		return nil, apiErr(fmt.Errorf("search.list rate-limited (exit 7); back off"))
	}
	if resp.StatusCode >= 400 {
		return nil, apiErr(fmt.Errorf("search.list http %d: %s", resp.StatusCode, string(body)))
	}
	type item struct {
		ID struct {
			ChannelID string `json:"channelId"`
		} `json:"id"`
		Snippet struct {
			ChannelTitle string `json:"channelTitle"`
			Title        string `json:"title"`
		} `json:"snippet"`
	}
	var payload struct {
		Items []item `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}
	suggestions := make([]WatchlistSuggestion, 0, top)
	seen := map[string]bool{}
	for _, it := range payload.Items {
		if it.ID.ChannelID == "" || seen[it.ID.ChannelID] {
			continue
		}
		seen[it.ID.ChannelID] = true
		suggestions = append(suggestions, WatchlistSuggestion{
			ChannelID: it.ID.ChannelID,
			Title:     it.Snippet.ChannelTitle,
			Reason:    "matched niche keywords",
		})
		if len(suggestions) >= top {
			break
		}
	}
	return suggestions, nil
}

func emitSuggestions(cmd *cobra.Command, flags *rootFlags, suggestions []WatchlistSuggestion, niche, source string) error {
	res := map[string]any{
		"niche":       niche,
		"source":      source,
		"count":       len(suggestions),
		"suggestions": suggestions,
	}
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, res)
	}
	if len(suggestions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no suggestions; try --live or refine --niche)")
		return nil
	}
	for _, s := range suggestions {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  (%s)\n", s.ChannelID, s.Title, s.Reason)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nAdd one with: yt-studio-pp-cli watchlist add <channel_id>\n")
	return nil
}
