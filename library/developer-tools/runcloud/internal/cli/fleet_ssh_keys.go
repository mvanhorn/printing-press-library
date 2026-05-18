// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type sshKeyPair struct {
	ServerID     string `json:"server_id,omitempty"`
	SystemUserID string `json:"system_user_id,omitempty"`
}

type sshKeyRow struct {
	Fingerprint string       `json:"fingerprint"`
	Count       int          `json:"count"`
	Pairs       []sshKeyPair `json:"pairs,omitempty"`
}

func newFleetSSHKeysCmd(flags *rootFlags) *cobra.Command {
	var fingerprint string

	cmd := &cobra.Command{
		Use:   "ssh-keys",
		Short: "Inventory every SSH key across every (server, system_user) pair, deduped by fingerprint",
		Example: `  runcloud-pp-cli fleet ssh-keys
  runcloud-pp-cli fleet ssh-keys --fingerprint SHA256:abc...`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - would aggregate SSH keys from local store)")
				return nil
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "runcloud-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(no local store yet — run 'runcloud-pp-cli sync' first)")
				return printJSONFiltered(cmd.OutOrStdout(), []sshKeyRow{}, flags)
			}

			const q = `
				SELECT id, data
				FROM resources
				WHERE resource_type IN ('ssh_keys', 'sshcredentials', 'system_users_sshcredentials')
			`
			rows, err := db.Query(q)
			if err != nil {
				return err
			}
			defer rows.Close()

			grouped := map[string]*sshKeyRow{}
			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					return err
				}
				fp := jsonStringField(data.String, "fingerprint", "publicKeyFingerprint")
				if fp == "" {
					continue
				}
				if fingerprint != "" && fp != fingerprint {
					continue
				}
				g, ok := grouped[fp]
				if !ok {
					g = &sshKeyRow{Fingerprint: fp}
					grouped[fp] = g
				}
				g.Count++
				g.Pairs = append(g.Pairs, sshKeyPair{
					ServerID:     jsonStringField(data.String, "serverId", "server_id"),
					SystemUserID: jsonStringField(data.String, "systemUserId", "system_user_id"),
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			out := make([]sshKeyRow, 0, len(grouped))
			for _, g := range grouped {
				out = append(out, *g)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Fingerprint < out[j].Fingerprint })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "Restrict to a single fingerprint")
	return cmd
}
