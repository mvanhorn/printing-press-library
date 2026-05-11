// Copyright 2026 quoxientzero. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newPostsTopCmd ranks recent posts by a chosen signal (upvotes, comments,
// or engagement = upvotes + 3*comments) inside a time window and returns
// full post content. This is the "signal in the noise" command for a
// community: "what's worth reading from the last 7 days."
func newPostsTopCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string
	var flagSince string
	var flagBy string
	var flagTop int
	var flagFull bool

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Top posts in a community within a time window (full content)",
		Long: `Rank recent posts by signal and return them with full content.

The window is anchored to post createdAt timestamps and supports relative
durations (24h, 7d, 30d) or absolute dates (2026-05-01, RFC3339).

Available --by signals:
  upvotes     pure upvote count
  comments    pure comment count
  engagement  upvotes + 3*comments (comments weigh more — they're harder-won)
  newest      ignore signal, return newest first
`,
		Example: `  skool-pp-cli posts top --community earlyaidopters --since 7d --top 5 --by engagement --json
  skool-pp-cli posts top --community bewarethedefault --since 24h --by upvotes --top 3 --json
  skool-pp-cli posts top --community earlyaidopters --since 30d --top 10 --by comments --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community := strings.TrimSpace(flagCommunity)
			if community == "" && c.Config != nil {
				community = c.Config.TemplateVars["community"]
			}
			if community == "" {
				return usageErr(fmt.Errorf("--community is required (or set template_vars.community in config)"))
			}
			cutoff, err := parseSinceArg(flagSince)
			if err != nil {
				return usageErr(err)
			}

			// Walk pages of the community feed until we hit the cutoff or run
			// out. Skool returns ~10 posts per page; cap at 10 pages so a
			// busy community with --since 90d doesn't blow up the call count.
			type rawPost struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
				UserID    string `json:"userId"`
				PostType  string `json:"postType"`
				Metadata  struct {
					Content   string `json:"content"`
					Comments  int    `json:"comments"`
					Upvotes   int    `json:"upvotes"`
					Upvoted   bool   `json:"upvoted"`
					Attachments string `json:"attachments"`
					Medias    json.RawMessage `json:"medias"`
				} `json:"metadata"`
				User struct {
					Name      string `json:"name"`
					FirstName string `json:"firstName"`
					LastName  string `json:"lastName"`
				} `json:"user"`
			}
			type envelope struct {
				PageProps struct {
					Total     int `json:"total"`
					PostTrees []struct {
						Post rawPost `json:"post"`
					} `json:"postTrees"`
				} `json:"pageProps"`
			}

			path := "/_next/data/{buildId}/" + community + ".json"
			var collected []rawPost
			seen := make(map[string]struct{})
			const maxPages = 10
			for page := 1; page <= maxPages; page++ {
				params := map[string]string{"g": community}
				if page > 1 {
					params["p"] = fmt.Sprintf("%d", page)
				}
				raw, err := c.Get(path, params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var env envelope
				if err := json.Unmarshal(raw, &env); err != nil {
					return fmt.Errorf("parsing community feed page %d: %w", page, err)
				}
				if len(env.PageProps.PostTrees) == 0 {
					break
				}
				oldestInPage := time.Time{}
				for _, pt := range env.PageProps.PostTrees {
					if _, dup := seen[pt.Post.ID]; dup {
						continue
					}
					seen[pt.Post.ID] = struct{}{}
					collected = append(collected, pt.Post)
					if ts, perr := time.Parse(time.RFC3339, pt.Post.CreatedAt); perr == nil {
						if oldestInPage.IsZero() || ts.Before(oldestInPage) {
							oldestInPage = ts
						}
					}
				}
				// Stop once the oldest post on this page is older than the cutoff.
				// Skool's default sort is newest-first, so once we've passed the
				// window there's nothing useful on later pages.
				if !oldestInPage.IsZero() && oldestInPage.Before(cutoff) {
					break
				}
			}

			// Filter to window
			filtered := make([]rawPost, 0, len(collected))
			for _, p := range collected {
				ts, perr := time.Parse(time.RFC3339, p.CreatedAt)
				if perr != nil {
					continue
				}
				if ts.Before(cutoff) {
					continue
				}
				filtered = append(filtered, p)
			}

			// Sort by chosen signal
			signal := strings.ToLower(strings.TrimSpace(flagBy))
			scoreOf := func(p rawPost) float64 {
				switch signal {
				case "comments":
					return float64(p.Metadata.Comments)
				case "upvotes", "likes":
					return float64(p.Metadata.Upvotes)
				case "newest":
					if ts, err := time.Parse(time.RFC3339, p.CreatedAt); err == nil {
						return float64(ts.Unix())
					}
					return 0
				case "engagement", "":
					return float64(p.Metadata.Upvotes) + 3*float64(p.Metadata.Comments)
				default:
					return float64(p.Metadata.Upvotes) + 3*float64(p.Metadata.Comments)
				}
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				return scoreOf(filtered[i]) > scoreOf(filtered[j])
			})
			if flagTop > 0 && flagTop < len(filtered) {
				filtered = filtered[:flagTop]
			}

			// Project to a clean, full-content shape
			type outRow struct {
				ID          string  `json:"id"`
				Name        string  `json:"name"`
				URL         string  `json:"url"`
				By          string  `json:"by"`
				FullName    string  `json:"full_name,omitempty"`
				CreatedAt   string  `json:"created_at"`
				UpdatedAt   string  `json:"updated_at,omitempty"`
				PostType    string  `json:"post_type,omitempty"`
				Upvotes     int     `json:"upvotes"`
				Comments    int     `json:"comments"`
				Score       float64 `json:"score"`
				ScoredBy    string  `json:"scored_by"`
				Content     string  `json:"content,omitempty"`
				Attachments string  `json:"attachments,omitempty"`
			}
			rows := make([]outRow, 0, len(filtered))
			for _, p := range filtered {
				row := outRow{
					ID:        p.ID,
					Name:      p.Name,
					URL:       "https://www.skool.com/" + community + "/" + p.Name,
					By:        p.User.Name,
					FullName:  strings.TrimSpace(p.User.FirstName + " " + p.User.LastName),
					CreatedAt: p.CreatedAt,
					UpdatedAt: p.UpdatedAt,
					PostType:  p.PostType,
					Upvotes:   p.Metadata.Upvotes,
					Comments:  p.Metadata.Comments,
					Score:     scoreOf(p),
					ScoredBy:  signalLabel(signal),
				}
				if flagFull {
					row.Content = p.Metadata.Content
					row.Attachments = p.Metadata.Attachments
				}
				// --full=false omits Content / Attachments so callers can
				// fetch a tight list of ranked post handles without paying
				// the content tax. URL + id are always present so the user
				// can fetch a specific body via `posts get` afterwards.
				rows = append(rows, row)
			}

			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to template_vars.community)")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Time window: relative (24h, 7d, 30d) or absolute date (2026-05-01)")
	cmd.Flags().StringVar(&flagBy, "by", "engagement", "Signal: upvotes | comments | engagement | newest")
	cmd.Flags().IntVar(&flagTop, "top", 5, "Cap to top N (0 = all)")
	cmd.Flags().BoolVar(&flagFull, "full", true, "Include full post body (default true — set false for shape-only)")
	return cmd
}

func signalLabel(s string) string {
	switch strings.ToLower(s) {
	case "", "engagement":
		return "engagement (upvotes + 3*comments)"
	case "upvotes", "likes":
		return "upvotes"
	case "comments":
		return "comments"
	case "newest":
		return "newest"
	default:
		return s
	}
}
