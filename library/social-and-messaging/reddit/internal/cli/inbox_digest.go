// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

type inboxItem struct {
	ThingID     string  `json:"thing_id"`
	Kind        string  `json:"kind"`
	Sub         string  `json:"subreddit"`
	Subject     string  `json:"subject,omitempty"`
	Body        string  `json:"body"`
	Author      string  `json:"author"`
	LinkTitle   string  `json:"link_title,omitempty"`
	LinkID      string  `json:"link_id,omitempty"`
	ParentScore int     `json:"parent_score,omitempty"`
	CreatedUTC  float64 `json:"created_utc"`
	New         bool    `json:"new"`
}

type inboxBySub struct {
	Sub   string      `json:"subreddit"`
	Count int         `json:"count"`
	Items []inboxItem `json:"items"`
}

// newInboxDigestCmd groups inbox items by source-sub and enriches each item
// with the current thread score (via /api/info). Replaces Reddit's flat
// inbox UI with a sub-grouped triage view.
func newInboxDigestCmd(flags *rootFlags) *cobra.Command {
	var (
		window string
		filter string
	)
	cmd := &cobra.Command{
		Use:   "inbox-digest",
		Short: "Group inbox items by source-sub and enrich with parent-thread score",
		Long: `Triage your Reddit inbox grouped by source subreddit, with parent thread
score enrichment per item.

Default window is 24h. Use --filter to restrict to a single inbox view:
  all | unread | mentions | messages | sent`,
		Example: `  reddit-pp-cli inbox digest --window 24h
  reddit-pp-cli inbox digest --filter unread --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			windowHours, err := parseDurationHours(window)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window: %w", err))
			}
			cutoff := time.Now().Add(-time.Duration(windowHours) * time.Hour).Unix()

			path := "/message/inbox"
			switch strings.ToLower(strings.TrimSpace(filter)) {
			case "unread":
				path = "/message/unread"
			case "mentions":
				path = "/message/mentions"
			case "messages":
				path = "/message/messages"
			case "sent":
				path = "/message/sent"
			}
			body, err := c.Get(cmd.Context(), path, map[string]string{"limit": "100"})
			if err != nil {
				return apiErr(fmt.Errorf("fetching inbox: %w", err))
			}
			items := parseInboxItems(body, cutoff)

			// Enrich with parent scores when link_id present
			linkIDs := []string{}
			seen := map[string]bool{}
			for _, it := range items {
				if it.LinkID == "" || seen[it.LinkID] {
					continue
				}
				seen[it.LinkID] = true
				linkIDs = append(linkIDs, it.LinkID)
			}
			scoreByLink := map[string]int{}
			if len(linkIDs) > 0 {
				// Reddit's /api/info accepts up to 100 IDs comma-separated
				infoBody, err := c.Get(cmd.Context(), "/api/info", map[string]string{
					"id": strings.Join(linkIDs, ","),
				})
				if err == nil {
					var env struct {
						Data struct {
							Children []struct {
								Data struct {
									Name  string `json:"name"`
									Score int    `json:"score"`
								} `json:"data"`
							} `json:"children"`
						} `json:"data"`
					}
					if err := json.Unmarshal(infoBody, &env); err == nil {
						for _, ch := range env.Data.Children {
							scoreByLink[ch.Data.Name] = ch.Data.Score
						}
					}
				}
			}
			for i := range items {
				if s, ok := scoreByLink[items[i].LinkID]; ok {
					items[i].ParentScore = s
				}
			}

			// Group by sub
			bySub := map[string]*inboxBySub{}
			for _, it := range items {
				sub := it.Sub
				if sub == "" {
					sub = "(message)"
				}
				g, ok := bySub[sub]
				if !ok {
					g = &inboxBySub{Sub: sub}
					bySub[sub] = g
				}
				g.Count++
				g.Items = append(g.Items, it)
			}
			groups := []inboxBySub{}
			for _, g := range bySub {
				groups = append(groups, *g)
			}
			sort.Slice(groups, func(i, j int) bool {
				return groups[i].Count > groups[j].Count
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), groups, flags)
			}
			renderInboxDigest(cmd.OutOrStdout(), groups)
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "24h", "Look-back window (e.g. 1h, 24h, 7d)")
	cmd.Flags().StringVar(&filter, "filter", "all", "Inbox view: all|unread|mentions|messages|sent")
	return cmd
}

func parseInboxItems(body []byte, cutoff int64) []inboxItem {
	var env struct {
		Data struct {
			Children []struct {
				Kind string `json:"kind"`
				Data struct {
					Name        string  `json:"name"`
					Subreddit   string  `json:"subreddit"`
					Subject     string  `json:"subject"`
					Body        string  `json:"body"`
					Author      string  `json:"author"`
					LinkTitle   string  `json:"link_title"`
					ContextLink string  `json:"context"`
					ParentID    string  `json:"parent_id"`
					LinkID      string  `json:"link_id"`
					CreatedUTC  float64 `json:"created_utc"`
					New         bool    `json:"new"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}
	out := []inboxItem{}
	for _, ch := range env.Data.Children {
		if int64(ch.Data.CreatedUTC) < cutoff {
			continue
		}
		out = append(out, inboxItem{
			ThingID:    ch.Data.Name,
			Kind:       ch.Kind,
			Sub:        ch.Data.Subreddit,
			Subject:    ch.Data.Subject,
			Body:       truncate(ch.Data.Body, 200),
			Author:     ch.Data.Author,
			LinkTitle:  ch.Data.LinkTitle,
			LinkID:     ch.Data.LinkID,
			CreatedUTC: ch.Data.CreatedUTC,
			New:        ch.Data.New,
		})
	}
	return out
}

func renderInboxDigest(w io.Writer, groups []inboxBySub) {
	if len(groups) == 0 {
		fmt.Fprintln(w, "Inbox empty (or window too short).")
		return
	}
	total := 0
	for _, g := range groups {
		total += g.Count
	}
	fmt.Fprintf(w, "Inbox digest — %d item(s) across %d source(s)\n\n", total, len(groups))
	for _, g := range groups {
		fmt.Fprintf(w, "r/%s — %d item(s)\n", g.Sub, g.Count)
		for _, it := range g.Items {
			marker := " "
			if it.New {
				marker = "*"
			}
			fmt.Fprintf(w, "  %s [%s] by u/%s • %s\n", marker, it.Kind, it.Author, it.Body)
			if it.LinkTitle != "" {
				fmt.Fprintf(w, "    thread: %s (score=%d)\n", it.LinkTitle, it.ParentScore)
			}
		}
		fmt.Fprintln(w, "")
	}
}

var _ = context.Background
