// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type phpAuditRow struct {
	ServerID       string `json:"server_id,omitempty"`
	ServerName     string `json:"server_name,omitempty"`
	WebappID       string `json:"webapp_id,omitempty"`
	WebappName     string `json:"webapp_name,omitempty"`
	PHPVersion     string `json:"php_version,omitempty"`
	BelowThreshold bool   `json:"below_threshold"`
}

func newFleetPHPAuditCmd(flags *rootFlags) *cobra.Command {
	var below string

	cmd := &cobra.Command{
		Use:         "php-audit",
		Short:       "Find webapps and servers running below a given PHP version",
		Example:     `  runcloud-pp-cli fleet php-audit --below 8.2`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would read PHP versions from local store)")
				return nil
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []phpAuditRow{}, flags)
			}

			threshold := parseSemver(below) // zero means no threshold; everything reports false

			servers, err := loadServerMeta(db)
			if err != nil {
				return err
			}

			// Webapps
			const wq = `SELECT id, data FROM resources WHERE resource_type IN ('webapps', 'webapps_list')`
			rows, err := db.Query(wq)
			if err != nil {
				return err
			}
			defer rows.Close()

			var out []phpAuditRow
			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					return err
				}
				ver := jsonStringField(data.String, "phpVersion", "php_version")
				serverID := jsonStringField(data.String, "serverId", "server_id")
				row := phpAuditRow{
					ServerID:   serverID,
					ServerName: servers[serverID],
					WebappID:   id.String,
					WebappName: jsonStringField(data.String, "name"),
					PHPVersion: ver,
				}
				if below != "" && ver != "" {
					row.BelowThreshold = compareSemver(parseSemver(ver), threshold) < 0
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			// Server-level PHP CLI versions
			const sq = `SELECT id, data FROM resources WHERE resource_type = 'servers'`
			srows, err := db.Query(sq)
			if err != nil {
				return err
			}
			defer srows.Close()
			for srows.Next() {
				var id, data sql.NullString
				if err := srows.Scan(&id, &data); err != nil {
					return err
				}
				ver := jsonStringField(data.String, "phpCliVersion", "php_cli_version")
				if ver == "" {
					continue
				}
				row := phpAuditRow{
					ServerID:   id.String,
					ServerName: jsonStringField(data.String, "name"),
					PHPVersion: ver,
				}
				if below != "" {
					row.BelowThreshold = compareSemver(parseSemver(ver), threshold) < 0
				}
				out = append(out, row)
			}
			if err := srows.Err(); err != nil {
				return err
			}

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&below, "below", "", "PHP version threshold; rows below this are flagged (e.g. 8.2)")
	return cmd
}

// loadServerMeta returns id -> name for synced servers.
func loadServerMeta(db interface {
	Query(string, ...any) (*sql.Rows, error)
}) (map[string]string, error) {
	rows, err := db.Query(`SELECT id, data FROM resources WHERE resource_type = 'servers'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		out[id.String] = jsonStringField(data.String, "name")
	}
	return out, rows.Err()
}

// parseSemver parses a loose semver-shape (e.g. "8.2", "8.2.10", "php82") into
// up to 3 numeric components. Non-numeric prefixes are stripped.
func parseSemver(s string) [3]int {
	var out [3]int
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "php")
	s = strings.TrimPrefix(s, "v")
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	for i := 0; i < len(parts) && i < 3; i++ {
		digits := ""
		for _, c := range parts[i] {
			if c >= '0' && c <= '9' {
				digits += string(c)
			} else {
				break
			}
		}
		if digits == "" {
			continue
		}
		n, _ := strconv.Atoi(digits)
		out[i] = n
	}
	return out
}

func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
