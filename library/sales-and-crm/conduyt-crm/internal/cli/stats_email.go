// Copyright 2026 Conduyt and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"github.com/spf13/cobra"
)

func newInsightsEmailStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "email-stats",
		Short:       "Email performance analytics — open rates, click rates, reply rates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  conduyt-crm-pp-cli insights email-stats
  conduyt-crm-pp-cli insights email-stats --limit 20 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("conduyt-crm-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'conduyt-crm-pp-cli sync' first.", err)
			}
			defer db.Close()

			items, err := db.List("send", 0)
			if err != nil {
				items, err = db.List("emails", 0)
				if err != nil {
					return fmt.Errorf("listing emails: %w", err)
				}
			}

			type templateStats struct {
				Template  string `json:"template"`
				Sent      int    `json:"sent"`
				Opened    int    `json:"opened"`
				Clicked   int    `json:"clicked"`
				Replied   int    `json:"replied"`
				Bounced   int    `json:"bounced"`
				OpenRate  string `json:"open_rate"`
				ClickRate string `json:"click_rate"`
				ReplyRate string `json:"reply_rate"`
			}

			var totalSent, totalOpened, totalClicked, totalReplied, totalBounced int
			byTemplate := make(map[string]*templateStats)

			for _, raw := range items {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}

				totalSent++
				tmpl, _ := obj["templateName"].(string)
				if tmpl == "" {
					tmpl, _ = obj["subject"].(string)
				}
				if tmpl == "" {
					tmpl = "unknown"
				}

				ts, ok := byTemplate[tmpl]
				if !ok {
					ts = &templateStats{Template: tmpl}
					byTemplate[tmpl] = ts
				}
				ts.Sent++

				if opened, ok := obj["opened"].(bool); ok && opened {
					totalOpened++
					ts.Opened++
				} else if _, ok := obj["openedAt"].(string); ok {
					totalOpened++
					ts.Opened++
				}

				if clicked, ok := obj["clicked"].(bool); ok && clicked {
					totalClicked++
					ts.Clicked++
				}

				if replied, ok := obj["replied"].(bool); ok && replied {
					totalReplied++
					ts.Replied++
				}

				if status, _ := obj["status"].(string); status == "bounced" {
					totalBounced++
					ts.Bounced++
				}
			}

			var templates []templateStats
			for _, ts := range byTemplate {
				if ts.Sent > 0 {
					ts.OpenRate = fmt.Sprintf("%.1f%%", float64(ts.Opened)*100/float64(ts.Sent))
					ts.ClickRate = fmt.Sprintf("%.1f%%", float64(ts.Clicked)*100/float64(ts.Sent))
					ts.ReplyRate = fmt.Sprintf("%.1f%%", float64(ts.Replied)*100/float64(ts.Sent))
				}
				templates = append(templates, *ts)
			}
			sort.Slice(templates, func(i, j int) bool { return templates[i].Sent > templates[j].Sent })
			if limit > 0 && len(templates) > limit {
				templates = templates[:limit]
			}

			openRate, clickRate, replyRate := "0.0%", "0.0%", "0.0%"
			if totalSent > 0 {
				openRate = fmt.Sprintf("%.1f%%", float64(totalOpened)*100/float64(totalSent))
				clickRate = fmt.Sprintf("%.1f%%", float64(totalClicked)*100/float64(totalSent))
				replyRate = fmt.Sprintf("%.1f%%", float64(totalReplied)*100/float64(totalSent))
			}

			result := map[string]any{
				"total_sent":    totalSent,
				"total_opened":  totalOpened,
				"total_clicked": totalClicked,
				"total_replied": totalReplied,
				"total_bounced": totalBounced,
				"open_rate":     openRate,
				"click_rate":    clickRate,
				"reply_rate":    replyRate,
				"by_template":   templates,
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Printf("Email Performance Summary\n")
			fmt.Printf("Sent: %d | Opened: %s | Clicked: %s | Replied: %s | Bounced: %d\n\n", totalSent, openRate, clickRate, replyRate, totalBounced)
			fmt.Println("Template\t\tSent\tOpen%\tClick%\tReply%")
			fmt.Println("--------\t\t----\t-----\t------\t------")
			for _, t := range templates {
				name := t.Template
				if len(name) > 24 {
					name = name[:21] + "..."
				}
				fmt.Printf("%s\t%d\t%s\t%s\t%s\n", name, t.Sent, t.OpenRate, t.ClickRate, t.ReplyRate)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max templates to show")
	return cmd
}
