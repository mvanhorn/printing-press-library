// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelTodayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "today",
		Short:       "An opinionated public-domain pick to read today, rotated against your recent sits",
		Long:        "today serves one public-domain book to read, chosen from a rotating set of canonical reading and skipping anything you've sat with recently. Sit with it via: passage sit <id>.",
		Example:     "  passage today --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			p, err := openPractice(cmd)
			if err != nil {
				return err
			}
			defer p.Close()
			recent, err := p.RecentSitIDs(ctx, 14)
			if err != nil {
				return fmt.Errorf("reading recent reflections: %w", err)
			}

			gc := gutClient()
			// Rotate the topic by day so the pick varies but is stable within a day.
			start := time.Now().YearDay() % len(todayTopics)
			for off := 0; off < len(todayTopics); off++ {
				topic := todayTopics[(start+off)%len(todayTopics)]
				books, err := gc.Search(ctx, topic, true)
				if err != nil {
					continue
				}
				for _, b := range books {
					if recent[b.ID] || b.TextURL() == "" {
						continue
					}
					out := struct {
						ID     int    `json:"gutenberg_id"`
						Title  string `json:"title"`
						Author string `json:"author"`
						Topic  string `json:"topic"`
						Why    string `json:"why"`
						SitCmd string `json:"sit_cmd"`
					}{b.ID, b.Title, b.AuthorLine(), topic,
						fmt.Sprintf("today's rotation is '%s'; %d readers have downloaded this; you haven't sat with it recently", topic, b.Downloads),
						fmt.Sprintf("passage sit %d", b.ID)}
					return bookRender(cmd, flags, out,
						[]string{"Id", "Title", "Author", "Why"},
						[][]string{{fmt.Sprintf("%d", b.ID), b.Title, b.AuthorLine(), out.Why}})
				}
			}
			return fmt.Errorf("couldn't reach a public-domain pick right now — check your connection and retry")
		},
	}
	return cmd
}
