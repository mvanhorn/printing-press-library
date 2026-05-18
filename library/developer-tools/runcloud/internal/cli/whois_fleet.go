// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type whoisFleetRow struct {
	Server     string `json:"server,omitempty"`
	Webapp     string `json:"webapp,omitempty"`
	SystemUser string `json:"system_user,omitempty"`
	SSLStatus  string `json:"ssl_status,omitempty"`
	Match      string `json:"match,omitempty"`
}

func newWhoisFleetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whois-fleet [domain-or-pattern]",
		Short: "Look up which server, webapp, and system user host a domain",
		Long: `Given a domain (or substring pattern), search the local store for the server,
webapp, and system user that host it, plus its SSL status.

Reads from the local SQLite store populated by 'sync'.`,
		Example: `  runcloud-pp-cli whois-fleet example.com
  runcloud-pp-cli whois-fleet '%.example.com'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would search local store)")
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}

			pattern := args[0]
			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []whoisFleetRow{}, flags)
			}

			like := pattern
			if !strings.ContainsAny(like, "%_") {
				like = "%" + like + "%"
			}
			const q = `
				SELECT resource_type, id, data
				FROM resources
				WHERE resource_type IN ('webapps', 'domains', 'servers')
				  AND lower(data) LIKE lower(?)
			`
			rows, err := db.Query(q, like)
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}
			defer rows.Close()

			var out []whoisFleetRow
			for rows.Next() {
				var rt, id, data sql.NullString
				if err := rows.Scan(&rt, &id, &data); err != nil {
					return err
				}
				out = append(out, whoisFleetRow{
					Server:     jsonStringField(data.String, "serverName", "server.name", "serverId", "server_id"),
					Webapp:     jsonStringField(data.String, "name", "webApplication.name"),
					SystemUser: jsonStringField(data.String, "systemUserName", "system_user", "user.name"),
					SSLStatus:  jsonStringField(data.String, "ssl.status", "sslStatus"),
					Match:      rt.String,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
