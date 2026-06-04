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

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
	"github.com/spf13/cobra"
)

type webhookEntry struct {
	WebhookID   string   `json:"webhook_id"`
	Name        string   `json:"name,omitempty"`
	URL         string   `json:"url"`
	Environment string   `json:"environment,omitempty"`
	AppID       string   `json:"app_id,omitempty"`
	Events      []string `json:"event_types"`
}

type webhookHostGroup struct {
	Host        string         `json:"host"`
	Stale       bool           `json:"stale"`
	StaleReason string         `json:"stale_reason,omitempty"`
	Duplicate   bool           `json:"duplicate"`
	EventCount  int            `json:"event_count"`
	Webhooks    []webhookEntry `json:"webhooks"`
}

type webhookAuditView struct {
	ProjectID      string             `json:"project_id"`
	Hosts          []webhookHostGroup `json:"hosts"`
	TotalWebhooks  int                `json:"total_webhooks"`
	StaleHosts     int                `json:"stale_hosts"`
	DuplicateHosts int                `json:"duplicate_hosts"`
	Note           string             `json:"note,omitempty"`
}

func newNovelWebhookAuditCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "webhook-audit",
		Short: "Webhook integrations grouped by destination host, flagging duplicate and stale destinations",
		Long: `Lists every webhook integration in the local 'integrations' mirror, grouped by
URL host. Flags a host as DUPLICATE when more than one webhook points at it, and
as STALE when the host matches well-known development tunnels or local
addresses: localhost, 127.0.0.1, 0.0.0.0, ::1, RFC1918 private ranges,
link-local, *.ngrok.io / *.ngrok-free.app / *.ngrok.app, *.loca.lt,
*.serveo.net, *.trycloudflare.com, *.test, *.local, *.internal.

Data source: local. Run 'sync' for webhook integrations first.`,
		Example: "  revenuecat-pp-cli webhook-audit --project proj1ab2c3d4 --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would group local webhook integrations by host and flag stale/duplicate destinations")
				return nil
			}
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("revenuecat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "integrations", nil, flags.maxAge)

			view, err := buildWebhookAudit(db, projectID)
			if err != nil {
				return err
			}
			return emitWebhookAudit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitWebhookAudit(cmd *cobra.Command, flags *rootFlags, view webhookAuditView) error {
	if len(view.Hosts) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Hosts))
		for _, h := range view.Hosts {
			items = append(items, map[string]any{
				"host":       h.Host,
				"stale":      h.Stale,
				"reason":     h.StaleReason,
				"duplicate":  h.Duplicate,
				"webhooks":   len(h.Webhooks),
				"event_subs": h.EventCount,
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d webhook(s) across %d host(s); %d stale, %d duplicate.\n",
			view.TotalWebhooks, len(view.Hosts), view.StaleHosts, view.DuplicateHosts)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// webhookAuditScanCap bounds the integrations scan.
const webhookAuditScanCap = 100000

func buildWebhookAudit(db *store.Store, projectID string) (webhookAuditView, error) {
	view := webhookAuditView{ProjectID: projectID, Hosts: []webhookHostGroup{}}

	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'integrations' LIMIT ?`,
		webhookAuditScanCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying integrations: %w", err)
	}
	defer rows.Close()
	scanned := 0

	byHost := map[string]*webhookHostGroup{}

	for rows.Next() {
		scanned++
		var data sql.NullString
		if rows.Scan(&data) != nil || !data.Valid {
			continue
		}
		var obj struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			URL         string   `json:"url"`
			Environment string   `json:"environment"`
			AppID       string   `json:"app_id"`
			EventTypes  []string `json:"event_types"`
		}
		if json.Unmarshal([]byte(data.String), &obj) != nil {
			continue
		}
		view.TotalWebhooks++

		host := extractHost(obj.URL)
		stale, reason := classifyHost(host)
		grp, ok := byHost[host]
		if !ok {
			grp = &webhookHostGroup{Host: host, Stale: stale, StaleReason: reason}
			byHost[host] = grp
		}
		grp.Webhooks = append(grp.Webhooks, webhookEntry{
			WebhookID:   obj.ID,
			Name:        obj.Name,
			URL:         obj.URL,
			Environment: obj.Environment,
			AppID:       obj.AppID,
			Events:      obj.EventTypes,
		})
		grp.EventCount += len(obj.EventTypes)
	}
	if err := rows.Err(); err != nil {
		return view, fmt.Errorf("iterating integrations: %w", err)
	}
	if scanned >= webhookAuditScanCap {
		fmt.Fprintf(os.Stderr, "warning: webhook-audit hit the %d-webhook scan cap; some webhooks may be missing from this view\n", webhookAuditScanCap)
		view.Note = fmt.Sprintf("hit the %d-webhook scan cap; some entries may be missing.", webhookAuditScanCap)
	}

	for _, grp := range byHost {
		grp.Duplicate = len(grp.Webhooks) > 1
		view.Hosts = append(view.Hosts, *grp)
		if grp.Stale {
			view.StaleHosts++
		}
		if grp.Duplicate {
			view.DuplicateHosts++
		}
	}
	sort.Slice(view.Hosts, func(i, j int) bool {
		if view.Hosts[i].Stale != view.Hosts[j].Stale {
			return view.Hosts[i].Stale
		}
		if view.Hosts[i].Duplicate != view.Hosts[j].Duplicate {
			return view.Hosts[i].Duplicate
		}
		return view.Hosts[i].Host < view.Hosts[j].Host
	})

	if view.TotalWebhooks == 0 && view.Note == "" {
		view.Note = "no webhook integrations in local mirror; run 'sync' first"
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
	// IPv6 bracket notation must be handled before any ':' port strip.
	if h, _, err := net.SplitHostPort(lower); err == nil {
		lower = h
	} else if idx := strings.IndexByte(lower, ':'); idx >= 0 && !strings.Contains(lower, "::") && !strings.HasPrefix(lower, "[") {
		lower = lower[:idx]
	} else {
		lower = strings.Trim(lower, "[]")
	}
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
