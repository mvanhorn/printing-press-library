// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel replay command — replay agent session calls from SQLite cache.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/tavily/internal/store"
)

func newReplayCmd(flags *rootFlags) *cobra.Command {
	var session string
	var listSessions bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay all API calls from a named agent session using cached results",
		Long: `Replay every search and extract call recorded under a session label,
substituting locally cached results. No API credits are consumed.

Useful for:
  - Reproducing agent behavior for debugging
  - Running CI checks with deterministic cached data
  - Verifying prompt changes without spending credits

Use --list to see available sessions.`,
		Example: `  tavily-pp-cli replay --session my-agent
  tavily-pp-cli replay --list
  tavily-pp-cli replay --session my-agent --dry-run`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			if listSessions {
				sessions, err := st.AllSessions()
				if err != nil {
					return fmt.Errorf("listing sessions: %w", err)
				}
				if len(sessions) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No sessions recorded yet.")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Available sessions (%d):\n", len(sessions))
				for _, s := range sessions {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", s)
				}
				return nil
			}

			if session == "" && len(args) > 0 {
				session = args[0]
			}
			if session == "" {
				return fmt.Errorf("required: --session <name> or --list")
			}

			searches, err := st.SearchesBySession(session)
			if err != nil {
				return fmt.Errorf("reading searches: %w", err)
			}
			extracts, err := st.ExtractsBySession(session)
			if err != nil {
				return fmt.Errorf("reading extracts: %w", err)
			}

			if len(searches)+len(extracts) == 0 {
				return fmt.Errorf("session %q not found or empty", session)
			}

			type ReplayResult struct {
				Type     string          `json:"type"`
				Query    string          `json:"query,omitempty"`
				URLs     []string        `json:"urls,omitempty"`
				Response json.RawMessage `json:"response"`
				CachedAt string          `json:"cached_at"`
			}
			var results []ReplayResult

			for _, s := range searches {
				if dryRun {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would replay search: %q\n", s.Query)
					continue
				}
				results = append(results, ReplayResult{
					Type:     "search",
					Query:    s.Query,
					Response: json.RawMessage(s.Response),
					CachedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z"),
				})
			}

			for _, e := range extracts {
				var urls []string
				json.Unmarshal([]byte(e.URLs), &urls)
				if dryRun {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would replay extract: %v\n", urls)
					continue
				}
				results = append(results, ReplayResult{
					Type:     "extract",
					URLs:     urls,
					Response: json.RawMessage(e.Response),
					CachedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z"),
				})
			}

			if dryRun {
				return nil
			}

			if flags.asJSON {
				data, _ := json.MarshalIndent(results, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Replaying session %q (%d calls from cache)\n\n",
				session, len(results))
			for i, r := range results {
				switch r.Type {
				case "search":
					fmt.Fprintf(cmd.OutOrStdout(), "[%d] search %q  (cached %s)\n", i+1, r.Query, r.CachedAt)
					var resp struct {
						Results []struct {
							URL   string `json:"url"`
							Title string `json:"title"`
						} `json:"results"`
					}
					if json.Unmarshal(r.Response, &resp) == nil {
						for _, res := range resp.Results {
							fmt.Fprintf(cmd.OutOrStdout(), "     %s\n", res.URL)
						}
					}
				case "extract":
					fmt.Fprintf(cmd.OutOrStdout(), "[%d] extract %v  (cached %s)\n", i+1, r.URLs, r.CachedAt)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&session, "session", "", "Session label to replay")
	cmd.Flags().BoolVar(&listSessions, "list", false, "List all available sessions")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be replayed without outputting results")
	return cmd
}
