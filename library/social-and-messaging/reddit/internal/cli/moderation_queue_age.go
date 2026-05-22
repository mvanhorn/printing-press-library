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

type queueItem struct {
	ThingID    string  `json:"thing_id"`
	Kind       string  `json:"kind"`
	Sub        string  `json:"subreddit"`
	Author     string  `json:"author"`
	Title      string  `json:"title,omitempty"`
	Body       string  `json:"body,omitempty"`
	Permalink  string  `json:"permalink"`
	CreatedUTC float64 `json:"created_utc"`
	AgeHours   float64 `json:"age_hours"`
	NumReports int     `json:"num_reports"`
}

// newModQueueAgeCmd is a focused convenience over the spec-derived
// `moderation modqueue` command. The spec command returns the raw modqueue
// payload; this one fetches the same data and adds:
//
//	--sort age           — order by oldest first (modqueue UI can't do this)
//	--older-than 24h     — filter to items past a threshold age
//
// The signature insight: Reddit's modqueue UI surfaces items in arrival order
// without exposing their age, so moderators triage the >24h backlog only by
// clicking each item to inspect timestamps.
func newModQueueAgeCmd(flags *rootFlags) *cobra.Command {
	var (
		sortBy    string
		olderThan string
		limit     int
		only      string
	)
	cmd := &cobra.Command{
		Use:   "queue <subreddit>",
		Short: "Modqueue with age sort and --older-than filter (UI can't sort by age)",
		Long: `Get a subreddit's modqueue and sort/filter by item age.

The Reddit web UI displays modqueue items in arrival order without surfacing
their age. This command fetches the same /r/<sub>/about/modqueue endpoint and
applies local age sorting + filtering so moderators can find the >24h backlog.`,
		Example: `  reddit-pp-cli mod queue programming --sort age --older-than 24h
  reddit-pp-cli mod queue mysub --sort age --agent
  reddit-pp-cli mod queue mysub --only comments --older-than 1h`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			sub := strings.TrimPrefix(strings.TrimPrefix(args[0], "r/"), "/r/")
			if sub == "" {
				return usageErr(fmt.Errorf("subreddit required"))
			}
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			items, err := fetchModQueueItems(cmd.Context(), c, sub, only, limit)
			if err != nil {
				return apiErr(err)
			}

			now := time.Now().UTC()
			for i := range items {
				items[i].AgeHours = now.Sub(time.Unix(int64(items[i].CreatedUTC), 0).UTC()).Hours()
			}

			minHours := 0.0
			if olderThan != "" {
				h, err := parseDurationHours(olderThan)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --older-than: %w", err))
				}
				minHours = h
			}
			filtered := []queueItem{}
			for _, it := range items {
				if it.AgeHours >= minHours {
					filtered = append(filtered, it)
				}
			}

			if sortBy == "age" {
				sort.Slice(filtered, func(i, j int) bool {
					return filtered[i].AgeHours > filtered[j].AgeHours
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), filtered, flags)
			}
			renderQueueItems(cmd.OutOrStdout(), filtered)
			return nil
		},
	}
	cmd.Flags().StringVar(&sortBy, "sort", "age", "Sort order: age (default — oldest first)")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Filter to items older than this duration (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&only, "only", "", "Filter type: links | comments")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max items to fetch from modqueue")
	return cmd
}

func fetchModQueueItems(ctx context.Context, c *client.Client, sub, only string, limit int) ([]queueItem, error) {
	params := map[string]string{"limit": fmt.Sprintf("%d", limit)}
	if only != "" {
		params["only"] = only
	}
	body, err := c.Get(ctx, "/r/"+sub+"/about/modqueue", params)
	if err != nil {
		return nil, fmt.Errorf("fetching modqueue: %w", err)
	}
	var env struct {
		Data struct {
			Children []struct {
				Kind string `json:"kind"`
				Data struct {
					ID         string  `json:"id"`
					Name       string  `json:"name"`
					Subreddit  string  `json:"subreddit"`
					Author     string  `json:"author"`
					Title      string  `json:"title"`
					Body       string  `json:"body"`
					Permalink  string  `json:"permalink"`
					CreatedUTC float64 `json:"created_utc"`
					NumReports int     `json:"num_reports"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parsing modqueue: %w", err)
	}
	out := []queueItem{}
	for _, ch := range env.Data.Children {
		out = append(out, queueItem{
			ThingID:    ch.Data.Name,
			Kind:       ch.Kind,
			Sub:        ch.Data.Subreddit,
			Author:     ch.Data.Author,
			Title:      ch.Data.Title,
			Body:       truncate(ch.Data.Body, 200),
			Permalink:  ch.Data.Permalink,
			CreatedUTC: ch.Data.CreatedUTC,
			NumReports: ch.Data.NumReports,
		})
	}
	return out, nil
}

// parseDurationHours accepts strings like "24h", "7d", "30m", "2h30m" and
// returns the duration in hours.
func parseDurationHours(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	// Convert "d" suffix to multiplied hours since time.ParseDuration doesn't
	// understand days.
	if strings.HasSuffix(s, "d") {
		var days float64
		_, err := fmt.Sscanf(s, "%fd", &days)
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return d.Hours(), nil
}

func renderQueueItems(w io.Writer, items []queueItem) {
	if len(items) == 0 {
		fmt.Fprintln(w, "Modqueue is empty (or no items past the age threshold).")
		return
	}
	fmt.Fprintf(w, "Modqueue: %d items\n\n", len(items))
	for i, it := range items {
		fmt.Fprintf(w, "%d. [%s, %.1fh old] %d reports • u/%s\n",
			i+1, it.Kind, it.AgeHours, it.NumReports, it.Author)
		if it.Title != "" {
			fmt.Fprintf(w, "   %s\n", it.Title)
		}
		if it.Body != "" {
			fmt.Fprintf(w, "   %s\n", it.Body)
		}
		fmt.Fprintf(w, "   id=%s • https://reddit.com%s\n\n", it.ThingID, it.Permalink)
	}
}
