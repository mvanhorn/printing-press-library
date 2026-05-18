// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type installerRow struct {
	Server        string `json:"server,omitempty"`
	Webapp        string `json:"webapp,omitempty"`
	SiteURL       string `json:"site_url,omitempty"`
	InstallerType string `json:"installer_type,omitempty"`
	Version       string `json:"version,omitempty"`
}

func newFleetInstallersCmd(flags *rootFlags) *cobra.Command {
	var typeFilter string

	cmd := &cobra.Command{
		Use:   "installers",
		Short: "List every webapp with a script-installer attached (WordPress, Joomla, etc.)",
		Example: `  runcloud-pp-cli fleet installers
  runcloud-pp-cli fleet installers --type wordpress`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would aggregate installers from local store)")
				return nil
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []installerRow{}, flags)
			}

			servers, err := loadServerMeta(db)
			if err != nil {
				return err
			}

			const q = `
				SELECT id, data
				FROM resources
				WHERE resource_type IN ('installers', 'webapps_installer', 'installed_scripts')
			`
			rows, err := db.Query(q)
			if err != nil {
				return err
			}
			defer rows.Close()

			var out []installerRow
			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					return err
				}
				t := jsonStringField(data.String, "type", "scriptType", "installer_type", "name")
				if typeFilter != "" && !strings.EqualFold(t, typeFilter) {
					continue
				}
				sid := jsonStringField(data.String, "serverId", "server_id")
				out = append(out, installerRow{
					Server:        servers[sid],
					Webapp:        jsonStringField(data.String, "webApplicationName", "webapp_name", "name"),
					SiteURL:       jsonStringField(data.String, "siteUrl", "url"),
					InstallerType: t,
					Version:       jsonStringField(data.String, "version"),
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by installer type (wordpress, joomla, drupal, magento, etc.)")
	return cmd
}
