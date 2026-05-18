// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type blockedIPRow struct {
	IP               string `json:"ip"`
	ServersCount     int    `json:"servers_count"`
	FirstSeen        string `json:"first_seen,omitempty"`
	LastSeen         string `json:"last_seen,omitempty"`
	BlockedOnServers string `json:"blocked_on_servers,omitempty"`
}

func newFleetBlockedIPsCmd(flags *rootFlags) *cobra.Command {
	var since string
	var ipFilter string

	cmd := &cobra.Command{
		Use:   "blocked-ips",
		Short: "Show every fail2ban-blocked IP across the fleet, deduped, with server count",
		Example: `  runcloud-pp-cli fleet blocked-ips --since 7d
  runcloud-pp-cli fleet blocked-ips --ip 1.2.3.4`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would aggregate blocked IPs from local store)")
				return nil
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []blockedIPRow{}, flags)
			}

			var sinceCut *time.Time
			if since != "" {
				d, err := parseDurationDays(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				t := time.Now().Add(-d)
				sinceCut = &t
			}

			servers, err := loadServerMeta(db)
			if err != nil {
				return err
			}

			const q = `
				SELECT id, data
				FROM resources
				WHERE resource_type IN ('blocked_ips', 'fail2ban', 'security_blocked_ips', 'fail2ban_jails')
			`
			rows, err := db.Query(q)
			if err != nil {
				return err
			}
			defer rows.Close()

			type agg struct {
				count     int
				firstSeen time.Time
				lastSeen  time.Time
				servers   map[string]bool
			}
			grouped := map[string]*agg{}

			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					return err
				}
				ip := jsonStringField(data.String, "ip", "ipAddress", "address")
				if ip == "" {
					continue
				}
				if ipFilter != "" && ip != ipFilter {
					continue
				}
				serverID := jsonStringField(data.String, "serverId", "server_id")
				createdStr := jsonStringField(data.String, "createdAt", "created_at", "blockedAt", "blocked_at")
				created, _ := parseLooseTime(createdStr)

				if sinceCut != nil && !created.IsZero() && created.Before(*sinceCut) {
					continue
				}

				g, ok := grouped[ip]
				if !ok {
					g = &agg{servers: map[string]bool{}}
					grouped[ip] = g
				}
				g.count++
				if !created.IsZero() {
					if g.firstSeen.IsZero() || created.Before(g.firstSeen) {
						g.firstSeen = created
					}
					if created.After(g.lastSeen) {
						g.lastSeen = created
					}
				}
				if serverID != "" {
					g.servers[serverID] = true
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}

			out := make([]blockedIPRow, 0, len(grouped))
			for ip, g := range grouped {
				names := make([]string, 0, len(g.servers))
				for sid := range g.servers {
					if n := servers[sid]; n != "" {
						names = append(names, n)
					} else {
						names = append(names, sid)
					}
				}
				sort.Strings(names)
				row := blockedIPRow{
					IP:               ip,
					ServersCount:     len(g.servers),
					BlockedOnServers: strings.Join(names, ","),
				}
				if !g.firstSeen.IsZero() {
					row.FirstSeen = g.firstSeen.UTC().Format(time.RFC3339)
				}
				if !g.lastSeen.IsZero() {
					row.LastSeen = g.lastSeen.UTC().Format(time.RFC3339)
				}
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only include IPs blocked within this window (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&ipFilter, "ip", "", "Restrict output to a single IP address")
	return cmd
}
