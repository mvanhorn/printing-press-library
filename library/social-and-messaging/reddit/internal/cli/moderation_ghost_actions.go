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

type ghostAction struct {
	TargetID    string      `json:"target_id"`
	TargetType  string      `json:"target_type"`
	TargetTitle string      `json:"target_title,omitempty"`
	Chain       []ghostStep `json:"chain"`
	Reversed    bool        `json:"reversed"`
	Subreddit   string      `json:"subreddit"`
}

type ghostStep struct {
	Mod        string  `json:"mod"`
	Action     string  `json:"action"`
	CreatedUTC float64 `json:"created_utc"`
}

// newModGhostActionsCmd reads the modlog and detects action chains on the
// same target_id where different mods acted in conflicting ways (e.g.,
// approve→remove). Surfaces silent mod-team disagreements.
func newModGhostActionsCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "ghost-actions <subreddit>",
		Short: "Detect approve→remove (or vice versa) chains by different mods",
		Long: `Walk the modlog over a recent window and surface targets where
multiple moderators acted in conflicting ways:
  - mod A approved, mod B later removed
  - mod B removed, mod C later approved

These chains indicate silent mod-team disagreements that don't appear in any
existing dashboard.`,
		Example: `  reddit-pp-cli mod ghost-actions programming --since 7d
  reddit-pp-cli mod ghost-actions mysub --since 30d --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			sub := strings.TrimPrefix(strings.TrimPrefix(args[0], "r/"), "/r/")
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			windowHours, err := parseDurationHours(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			cutoff := time.Now().Add(-time.Duration(windowHours) * time.Hour).Unix()

			body, err := c.Get(cmd.Context(), "/r/"+sub+"/about/log", map[string]string{
				"limit": fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return apiErr(fmt.Errorf("fetching modlog: %w", err))
			}

			ghosts := computeGhostActions(body, sub, cutoff)

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), ghosts, flags)
			}
			renderGhostActions(cmd.OutOrStdout(), ghosts, since)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Look back this duration (e.g. 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max modlog entries to walk")
	return cmd
}

func computeGhostActions(body []byte, sub string, cutoff int64) []ghostAction {
	var env struct {
		Data struct {
			Children []struct {
				Data struct {
					CreatedUTC     float64 `json:"created_utc"`
					Action         string  `json:"action"`
					Mod            string  `json:"mod"`
					TargetFullname string  `json:"target_fullname"`
					TargetTitle    string  `json:"target_title"`
					TargetAuthor   string  `json:"target_author"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}

	chains := map[string]*ghostAction{}
	for _, ch := range env.Data.Children {
		if int64(ch.Data.CreatedUTC) < cutoff {
			continue
		}
		if ch.Data.TargetFullname == "" {
			continue
		}
		// Filter to mod actions of interest
		act := ch.Data.Action
		if !isModerationAction(act) {
			continue
		}

		g, ok := chains[ch.Data.TargetFullname]
		if !ok {
			g = &ghostAction{
				TargetID:    ch.Data.TargetFullname,
				TargetType:  fullnameKind(ch.Data.TargetFullname),
				TargetTitle: ch.Data.TargetTitle,
				Subreddit:   sub,
			}
			chains[ch.Data.TargetFullname] = g
		}
		g.Chain = append(g.Chain, ghostStep{
			Mod:        ch.Data.Mod,
			Action:     act,
			CreatedUTC: ch.Data.CreatedUTC,
		})
	}

	out := []ghostAction{}
	for _, g := range chains {
		// Sort chain by time
		sort.Slice(g.Chain, func(i, j int) bool {
			return g.Chain[i].CreatedUTC < g.Chain[j].CreatedUTC
		})
		if isReversed(g.Chain) {
			g.Reversed = true
			out = append(out, *g)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		// Most recent reversal first
		return out[i].Chain[len(out[i].Chain)-1].CreatedUTC > out[j].Chain[len(out[j].Chain)-1].CreatedUTC
	})
	return out
}

func isModerationAction(a string) bool {
	switch a {
	case "removelink", "removecomment", "spamlink", "spamcomment",
		"approvelink", "approvecomment":
		return true
	}
	return false
}

func isReversed(chain []ghostStep) bool {
	if len(chain) < 2 {
		return false
	}
	// Detect at least one polarity flip across distinct mods
	for i := 1; i < len(chain); i++ {
		prev := chain[i-1]
		curr := chain[i]
		if prev.Mod == "" || curr.Mod == "" || prev.Mod == curr.Mod {
			continue
		}
		if isRemoveAction(prev.Action) && isApproveAction(curr.Action) {
			return true
		}
		if isApproveAction(prev.Action) && isRemoveAction(curr.Action) {
			return true
		}
	}
	return false
}

func isRemoveAction(a string) bool {
	return a == "removelink" || a == "removecomment" || a == "spamlink" || a == "spamcomment"
}

func isApproveAction(a string) bool {
	return a == "approvelink" || a == "approvecomment"
}

func fullnameKind(name string) string {
	if strings.HasPrefix(name, "t1_") {
		return "comment"
	}
	if strings.HasPrefix(name, "t3_") {
		return "submission"
	}
	return name
}

func renderGhostActions(w io.Writer, ghosts []ghostAction, window string) {
	if len(ghosts) == 0 {
		fmt.Fprintf(w, "No reversal chains found in the last %s.\n", window)
		return
	}
	fmt.Fprintf(w, "%d ghost-action chain(s) in the last %s\n\n", len(ghosts), window)
	for i, g := range ghosts {
		fmt.Fprintf(w, "%d. %s %s — %s\n", i+1, g.TargetType, g.TargetID, g.TargetTitle)
		for _, s := range g.Chain {
			when := time.Unix(int64(s.CreatedUTC), 0).UTC().Format(time.RFC3339)
			fmt.Fprintf(w, "   %s — u/%s %s\n", when, s.Mod, s.Action)
		}
		fmt.Fprintln(w, "")
	}
}

var _ = context.Background
