// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type serviceRow struct {
	ServerID    string `json:"server_id,omitempty"`
	ServerName  string `json:"server_name,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Status      string `json:"status,omitempty"`
}

func newFleetServicesCmd(flags *rootFlags) *cobra.Command {
	var notRunning bool
	var nameFilter string

	cmd := &cobra.Command{
		Use:   "services",
		Short: "Filter every server's services list for non-running rows or a named service",
		Example: `  runcloud-pp-cli fleet services --not-running
  runcloud-pp-cli fleet services --name nginx`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would aggregate services from local store)")
				return nil
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []serviceRow{}, flags)
			}

			servers, err := loadServerMeta(db)
			if err != nil {
				return err
			}

			const q = `
				SELECT id, data
				FROM resources
				WHERE resource_type IN ('services', 'servers_services', 'supervisor')
			`
			rows, err := db.Query(q)
			if err != nil {
				return err
			}
			defer rows.Close()

			var out []serviceRow
			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					return err
				}
				name := jsonStringField(data.String, "name", "serviceName", "service_name")
				status := jsonStringField(data.String, "status", "state")
				if notRunning && strings.EqualFold(status, "running") {
					continue
				}
				if nameFilter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(nameFilter)) {
					continue
				}
				sid := jsonStringField(data.String, "serverId", "server_id")
				out = append(out, serviceRow{
					ServerID:    sid,
					ServerName:  servers[sid],
					ServiceName: name,
					Status:      status,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().BoolVar(&notRunning, "not-running", false, "Only show services that are not running")
	cmd.Flags().StringVar(&nameFilter, "name", "", "Filter by service name substring (e.g. nginx)")
	return cmd
}
