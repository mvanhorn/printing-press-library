// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/runcloud/internal/store"
)

type sslAuditRow struct {
	ServerID   string `json:"server_id"`
	WebappID   string `json:"webapp_id,omitempty"`
	WebappName string `json:"webapp_name,omitempty"`
	Domain     string `json:"domain,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	Status     string `json:"status,omitempty"`
}

func newFleetSSLAuditCmd(flags *rootFlags) *cobra.Command {
	var expiringWithin string
	var missing bool

	cmd := &cobra.Command{
		Use:   "ssl-audit",
		Short: "Find SSL certificates that are missing, expiring soon, or expired across every server",
		Example: `  runcloud-pp-cli fleet ssl-audit --expiring-within 30d
  runcloud-pp-cli fleet ssl-audit --missing`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would read SSL rows from local store)")
				return nil
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []sslAuditRow{}, flags)
			}

			var threshold *time.Time
			if expiringWithin != "" {
				d, err := parseDurationDays(expiringWithin)
				if err != nil {
					return usageErr(fmt.Errorf("--expiring-within: %w", err))
				}
				t := time.Now().Add(d)
				threshold = &t
			}

			rows, err := queryFleetSSLRows(db)
			if err != nil {
				return fmt.Errorf("querying SSL rows: %w", err)
			}

			if missing {
				missingRows, err := queryFleetSSLMissing(db, rows)
				if err != nil {
					return fmt.Errorf("computing missing SSL: %w", err)
				}
				rows = missingRows
				if threshold != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "note: --expiring-within is ignored when --missing is set (rows have no expiry)")
				}
			} else if threshold != nil {
				rows = filterSSLByExpiry(rows, *threshold)
			}

			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&expiringWithin, "expiring-within", "", "Only show SSL certs expiring within this duration (e.g. 30d, 168h)")
	cmd.Flags().BoolVar(&missing, "missing", false, "Show webapps with no SSL row")
	return cmd
}

// parseDurationDays accepts standard Go durations plus the "Nd" form.
func parseDurationDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid days value %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func queryFleetSSLRows(db *store.Store) ([]sslAuditRow, error) {
	const q = `
		SELECT id, resource_type, data
		FROM resources
		WHERE resource_type IN ('ssl', 'webapps_ssl', 'ssl_certs', 'ssl_advanced')
	`
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sslAuditRow
	for rows.Next() {
		var id sql.NullString
		var rt sql.NullString
		var data sql.NullString
		if err := rows.Scan(&id, &rt, &data); err != nil {
			return nil, err
		}
		row := sslAuditRow{}
		row.ServerID = jsonStringField(data.String, "serverId", "server_id")
		row.WebappID = jsonStringField(data.String, "webApplicationId", "webapp_id", "webApplication.id", "id")
		row.WebappName = jsonStringField(data.String, "webApplication.name", "name", "webapp_name")
		row.Domain = jsonStringField(data.String, "domainName", "domain", "name")
		row.ExpiresAt = jsonStringField(data.String, "expiresOn", "expires_at", "validUntil")
		row.Status = jsonStringField(data.String, "status", "state")
		out = append(out, row)
	}
	return out, rows.Err()
}

// queryFleetSSLMissing returns synthetic rows for every webapp lacking an SSL row.
func queryFleetSSLMissing(db *store.Store, sslRows []sslAuditRow) ([]sslAuditRow, error) {
	have := map[string]bool{}
	for _, r := range sslRows {
		if r.WebappID != "" {
			have[r.WebappID] = true
		}
	}
	const q = `
		SELECT id, data
		FROM resources
		WHERE resource_type IN ('webapps', 'webapps_list')
	`
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sslAuditRow
	for rows.Next() {
		var id sql.NullString
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		webappID := id.String
		if webappID == "" {
			webappID = jsonStringField(data.String, "id")
		}
		if have[webappID] {
			continue
		}
		out = append(out, sslAuditRow{
			ServerID:   jsonStringField(data.String, "serverId", "server_id"),
			WebappID:   webappID,
			WebappName: jsonStringField(data.String, "name"),
			Domain:     jsonStringField(data.String, "domainName", "domain"),
			Status:     "missing",
		})
	}
	return out, rows.Err()
}

func filterSSLByExpiry(rows []sslAuditRow, before time.Time) []sslAuditRow {
	var out []sslAuditRow
	for _, r := range rows {
		if r.ExpiresAt == "" {
			continue
		}
		t, err := parseLooseTime(r.ExpiresAt)
		if err != nil {
			continue
		}
		if t.Before(before) {
			out = append(out, r)
		}
	}
	return out
}

func parseLooseTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q", s)
}
