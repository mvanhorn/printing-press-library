// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

type webhookEntry struct {
	WebhookID string   `json:"webhook_id"`
	StoreID   string   `json:"store_id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
}

type webhookHostGroup struct {
	Host        string         `json:"host"`
	Stale       bool           `json:"stale"`
	StaleReason string         `json:"stale_reason,omitempty"`
	EventCount  int            `json:"event_count"`
	StoreCount  int            `json:"store_count"`
	Webhooks    []webhookEntry `json:"webhooks"`
}

type webhookAuditView struct {
	Hosts         []webhookHostGroup `json:"hosts"`
	TotalWebhooks int                `json:"total_webhooks"`
	StaleHosts    int                `json:"stale_hosts"`
	Note          string             `json:"note,omitempty"`
}

func newNovelWebhookAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "webhook-audit",
		Short: "Cross-store webhook coverage grouped by URL host, flagging stale destinations",
		Long: `Lists every webhook in the local 'webhooks' mirror, grouped by URL host.

Flags hosts as stale when the URL host matches well-known development tunnels
or local addresses: localhost, 127.0.0.1, *.ngrok.io / *.ngrok-free.app,
*.loca.lt, *.serveo.net, *.test, *.local, *.internal.

Use this for cross-store webhook coverage + stale-host detection. For pruning
the dead ones, pipe through the generated 'delete-webhook' per id.

Data source: local. Run 'sync --resources webhooks' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources webhooks\n  lemonsqueezy-pp-cli webhook-audit --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "webhooks") {
				hintIfStale(cmd, db, "webhooks", flags.maxAge)
			}

			view, err := buildWebhookAudit(db)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func buildWebhookAudit(db *store.Store) (webhookAuditView, error) {
	view := webhookAuditView{Hosts: []webhookHostGroup{}}

	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'webhooks' LIMIT 10000`)
	if err != nil {
		return view, fmt.Errorf("querying webhooks: %w", err)
	}
	defer rows.Close()

	byHost := map[string]*webhookHostGroup{}
	storeSet := map[string]map[string]bool{}

	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		var env struct {
			ID         string `json:"id"`
			Attributes struct {
				URL     string   `json:"url"`
				StoreID any      `json:"store_id"`
				Events  []string `json:"events"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
			continue
		}
		view.TotalWebhooks++

		hostname := extractHost(env.Attributes.URL)
		stale, reason := classifyHost(hostname)
		grp, ok := byHost[hostname]
		if !ok {
			grp = &webhookHostGroup{Host: hostname, Stale: stale, StaleReason: reason}
			byHost[hostname] = grp
			storeSet[hostname] = map[string]bool{}
		}
		entry := webhookEntry{
			WebhookID: env.ID,
			StoreID:   toStringLS(env.Attributes.StoreID),
			URL:       env.Attributes.URL,
			Events:    env.Attributes.Events,
		}
		grp.Webhooks = append(grp.Webhooks, entry)
		grp.EventCount += len(env.Attributes.Events)
		if entry.StoreID != "" {
			storeSet[hostname][entry.StoreID] = true
		}
	}

	for hostname, grp := range byHost {
		grp.StoreCount = len(storeSet[hostname])
		view.Hosts = append(view.Hosts, *grp)
		if grp.Stale {
			view.StaleHosts++
		}
	}
	sort.Slice(view.Hosts, func(i, j int) bool {
		if view.Hosts[i].Stale != view.Hosts[j].Stale {
			return view.Hosts[i].Stale
		}
		return view.Hosts[i].Host < view.Hosts[j].Host
	})

	if view.TotalWebhooks == 0 {
		view.Note = "no webhooks in local mirror; run 'sync --resources webhooks' first"
	}
	return view, nil
}

func extractHost(rawURL string) string {
	if rawURL == "" {
		return "(no url)"
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return strings.ToLower(u.Host)
}

func classifyHost(host string) (stale bool, reason string) {
	lower := strings.ToLower(host)
	// Strip an optional :port suffix before classification so case statements
	// only need to match the bare hostname.
	if idx := strings.IndexByte(lower, ':'); idx >= 0 {
		lower = lower[:idx]
	}
	switch {
	case lower == "localhost":
		return true, "localhost"
	case lower == "127.0.0.1" || lower == "::1":
		return true, "loopback IP"
	case strings.HasSuffix(lower, ".ngrok.io") || strings.HasSuffix(lower, ".ngrok-free.app") || strings.HasSuffix(lower, ".ngrok.app"):
		return true, "ngrok tunnel"
	case strings.HasSuffix(lower, ".loca.lt"):
		return true, "loca.lt tunnel"
	case strings.HasSuffix(lower, ".serveo.net"):
		return true, "serveo tunnel"
	case strings.HasSuffix(lower, ".test"):
		return true, ".test TLD (development)"
	case strings.HasSuffix(lower, ".local"):
		return true, ".local mDNS"
	case strings.HasSuffix(lower, ".internal"):
		return true, ".internal TLD"
	}
	return false, ""
}
