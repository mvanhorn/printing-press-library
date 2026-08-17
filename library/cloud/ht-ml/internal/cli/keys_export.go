// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: export the local update_key vault for disaster
// recovery and cross-machine import. Survives generate --force.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelKeysExportCmd(flags *rootFlags) *cobra.Command {
	var flagOut string
	var insecurePlaintext bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all stored update_keys to a passphrase-sealed vault file",
		Long: trimNL(`
Export every site's update_key (and password) for disaster recovery and to move
to another machine. By default the vault is encrypted with AES-256-GCM under a
passphrase read from $HT_ML_VAULT_PASSPHRASE.

Losing an update_key with no backup orphans the site forever (no recovery
endpoint exists), so run this after publishing.`),
		Example: trimNL(`
  ht-ml-pp-cli keys export --insecure-plaintext
  HT_ML_VAULT_PASSPHRASE=secret ht-ml-pp-cli keys export --out ht-ml-keys.vault
  ht-ml-pp-cli keys export --insecure-plaintext --out keys.json`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "export-vault"}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "would export the update_key vault")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			secrets, err := db.AllSecrets()
			if err != nil {
				return err
			}
			records := make([]keyExportRecord, 0, len(secrets))
			for _, s := range secrets {
				if s.UpdateKey == "" {
					continue
				}
				records = append(records, keyExportRecord{
					SiteID:    s.SiteID,
					UpdateKey: s.UpdateKey,
					Password:  s.Password,
					URL:       s.URL,
					Title:     s.Title,
					CreatedAt: s.CreatedAt,
				})
			}
			plaintext, err := json.MarshalIndent(records, "", "  ")
			if err != nil {
				return err
			}

			var payload []byte
			if insecurePlaintext {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: writing update_keys UNENCRYPTED; protect this file and delete it when done.")
				payload = plaintext
			} else {
				pass, perr := vaultPassphrase()
				if perr != nil {
					return perr
				}
				payload, err = encryptVault(plaintext, pass)
				if err != nil {
					return apiErr(err)
				}
			}

			if flagOut == "" {
				// No destination: the vault payload (JSON array when
				// --insecure-plaintext, JSON envelope when encrypted) is itself
				// valid JSON on stdout.
				cmd.OutOrStdout().Write(payload)
				fmt.Fprintln(cmd.OutOrStdout())
				return nil
			}
			if err := os.WriteFile(flagOut, payload, 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", flagOut, err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"exported":  len(records),
					"out":       flagOut,
					"encrypted": !insecurePlaintext,
				}, flags)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "exported %d key(s) to %s\n", len(records), flagOut)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOut, "out", "", "Write the vault to this file (default: stdout)")
	cmd.Flags().BoolVar(&insecurePlaintext, "insecure-plaintext", false, "Export keys UNENCRYPTED (no passphrase); protect the file yourself")
	return cmd
}
