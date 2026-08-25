// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Webhook delivery health: aggregate webhook-events by status per endpoint and surface payment failures.
// pp:data-source local
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/store"
	"github.com/spf13/cobra"
)

type whEvent struct {
	ID        string `json:"id"`
	WebhookID string `json:"webhookId"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type whEndpoint struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type whHealthRow struct {
	WebhookID       string `json:"webhook_id"`
	URL             string `json:"url,omitempty"`
	Total           int    `json:"total"`
	Delivered       int    `json:"delivered"`
	Failed          int    `json:"failed"`
	Retrying        int    `json:"retrying"`
	PaymentFailures int    `json:"payment_failures"`
	LastFailedAt    string `json:"last_failed_at,omitempty"`
}

func newNovelWebhooksHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagSince string
	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Aggregate webhook delivery events by status per endpoint and surface payment-failure events",
		Example:     "  quentli-pp-cli webhooks health --since 24h --json",
		Long:        "Use on-call to spot failed webhook deliveries and payment-failure signals fast instead of polling dashboards.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhooks health")
			}
			var since time.Duration
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = parsed
			}
			if dbPath == "" {
				dbPath = defaultDBPath("quentli-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: quentli-pp-cli sync --resources webhook-events,webhooks --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]whHealthRow, 0), flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "webhook-events")

			events, err := loadAll[whEvent](db, "webhook-events")
			if err != nil {
				return err
			}
			endpoints, err := loadAll[whEndpoint](db, "webhooks")
			if err != nil {
				return err
			}
			urlMap := map[string]string{}
			for _, e := range endpoints {
				urlMap[e.ID] = e.URL
			}
			group := map[string]*whHealthRow{}
			now := time.Now().UTC()
			for _, ev := range events {
				if since > 0 && ev.CreatedAt != "" {
					if ct := cliutil.ParseStoredTime(ev.CreatedAt); !ct.IsZero() && now.Sub(ct) > since {
						continue
					}
				}
				tr := group[ev.WebhookID]
				if tr == nil {
					tr = &whHealthRow{WebhookID: ev.WebhookID, URL: urlMap[ev.WebhookID]}
					group[ev.WebhookID] = tr
				}
				tr.Total++
				switch {
				case strings.EqualFold(ev.Status, "FAILED"):
					tr.Failed++
					tr.LastFailedAt = ev.CreatedAt
				case strings.EqualFold(ev.Status, "RETRYING") || strings.EqualFold(ev.Status, "PENDING"):
					tr.Retrying++
				default:
					tr.Delivered++
				}
				if strings.Contains(strings.ToUpper(ev.Type), "ATTEMPT_FAILED") {
					tr.PaymentFailures++
				}
			}
			rows := make([]whHealthRow, 0, len(group))
			for _, tr := range group {
				rows = append(rows, *tr)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Failed > rows[j].Failed })
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No webhook delivery events in the local mirror.")
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{
					"webhook_id":       r.WebhookID,
					"url":              r.URL,
					"delivered":        r.Delivered,
					"failed":           r.Failed,
					"retrying":         r.Retrying,
					"payment_failures": r.PaymentFailures,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "only consider delivery events in this window (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "path to the local database")
	return cmd
}

// register webhooks health under the generated webhooks command. Webhooks is a
// generator-owned command (conflict with a novel stub), so we attach the child
// via a hook instead of editing root.go.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		webhooksCmd, _, err := root.Find([]string{"webhooks"})
		if err == nil {
			addNovelCommandIfAbsent(webhooksCmd, newNovelWebhooksHealthCmd(flags))
		}
	})
}
