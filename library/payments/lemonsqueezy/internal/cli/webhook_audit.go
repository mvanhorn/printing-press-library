// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
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
or local addresses: localhost, 127.0.0.1, 0.0.0.0, ::1, RFC1918 private
ranges (10.*, 172.16-31.*, 192.168.*), link-local (fe80::*), *.ngrok.io /
*.ngrok-free.app / *.ngrok.app, *.loca.lt, *.serveo.net, *.test, *.local,
*.internal.

Use this for cross-store webhook coverage + stale-host detection. For pruning
the dead ones, pipe through the generated 'delete-webhook' per id.

Data source: local. Run 'sync --resources webhooks' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources webhooks\n  lemonsqueezy-pp-cli webhook-audit --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read-only local query — no mutation to suppress under --dry-run.
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "webhooks", nil, flags.maxAge)

			view, err := buildWebhookAudit(db)
			if err != nil {
				return err
			}
			return emitWebhookAudit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitWebhookAudit(cmd *cobra.Command, flags *rootFlags, view webhookAuditView) error {
	if len(view.Hosts) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Hosts))
		for _, h := range view.Hosts {
			items = append(items, map[string]any{
				"host":         h.Host,
				"stale":        h.Stale,
				"reason":       h.StaleReason,
				"webhooks":     len(h.Webhooks),
				"event_subs":   h.EventCount,
				"store_count":  h.StoreCount,
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d webhook(s) across %d host(s); %d host(s) flagged stale.\n",
			view.TotalWebhooks, len(view.Hosts), view.StaleHosts)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// webhookAuditScanCap bounds the webhooks scan. Saturation surfaces a
// stderr warning so the caller can tell apart "no webhooks" from "scan
// truncated".
const webhookAuditScanCap = 100000

func buildWebhookAudit(db *store.Store) (webhookAuditView, error) {
	view := webhookAuditView{Hosts: []webhookHostGroup{}}

	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'webhooks' LIMIT ?`,
		webhookAuditScanCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying webhooks: %w", err)
	}
	defer rows.Close()
	scannedWebhooks := 0

	byHost := map[string]*webhookHostGroup{}
	storeSet := map[string]map[string]bool{}

	for rows.Next() {
		scannedWebhooks++
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
	if scannedWebhooks >= webhookAuditScanCap {
		fmt.Fprintf(os.Stderr, "warning: webhook-audit hit the %d-webhook scan cap; some webhooks may be missing from this view\n", webhookAuditScanCap)
		view.Note = fmt.Sprintf("hit the %d-webhook scan cap; some entries may be missing.", webhookAuditScanCap)
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

	if view.TotalWebhooks == 0 && view.Note == "" {
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
	// IPv6 bracket notation ([::1] or [::1]:8080) must be handled before any
	// `:` port strip, otherwise net.ParseIP receives "[" or "[::1]:8080" and
	// returns nil. net.SplitHostPort handles both bracketed and plain forms;
	// fall back to a literal IndexByte strip for hostnames whose only colon
	// is the port separator.
	if h, _, err := net.SplitHostPort(lower); err == nil {
		lower = h
	} else if idx := strings.IndexByte(lower, ':'); idx >= 0 && !strings.Contains(lower, "::") && !strings.HasPrefix(lower, "[") {
		lower = lower[:idx]
	} else {
		lower = strings.Trim(lower, "[]")
	}
	// IP-shaped hosts: catch loopback, all-interfaces, and RFC1918 private
	// ranges. Production webhooks should not point at any of these.
	if ip := net.ParseIP(lower); ip != nil {
		switch {
		case ip.IsLoopback():
			return true, "loopback IP"
		case ip.IsUnspecified():
			return true, "all-interfaces IP"
		case ip.IsLinkLocalUnicast():
			return true, "link-local IP"
		case ip.IsPrivate():
			return true, "RFC1918 private IP"
		}
		return false, ""
	}
	// Hostname-shaped: catch dev TLDs and common tunneling services.
	switch {
	case lower == "localhost":
		return true, "localhost"
	case strings.HasSuffix(lower, ".ngrok.io") ||
		strings.HasSuffix(lower, ".ngrok-free.app") ||
		strings.HasSuffix(lower, ".ngrok.app") ||
		strings.HasSuffix(lower, ".ngrok-free.dev"):
		return true, "ngrok tunnel"
	case strings.HasSuffix(lower, ".loca.lt"):
		return true, "loca.lt tunnel"
	case strings.HasSuffix(lower, ".serveo.net"):
		return true, "serveo tunnel"
	case strings.HasSuffix(lower, ".trycloudflare.com"):
		return true, "cloudflare quick tunnel"
	case strings.HasSuffix(lower, ".test"):
		return true, ".test TLD (development)"
	case strings.HasSuffix(lower, ".local"):
		return true, ".local mDNS"
	case strings.HasSuffix(lower, ".internal"):
		return true, ".internal TLD"
	}
	return false, ""
}
