// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: key vault & recovery. The per-site update_key is
// returned only once at creation with no recovery endpoint, so the local store
// is its sole holder. Survives generate --force.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

// keyExportRecord is one site's recoverable secret material, used by the
// export/import vault round-trip.
type keyExportRecord struct {
	SiteID    string `json:"site_id"`
	UpdateKey string `json:"update_key"`
	Password  string `json:"password,omitempty"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func newNovelKeysCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage the once-only site update_keys: show, list, export, import",
		Long:  trimNL("Reveal, inventory, back up, and restore the per-site update_key. ht-ml.app returns each update_key only once with no recovery endpoint, so this local store is the only place it exists. Back it up with 'keys export'."),
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelKeysShowCmd(flags))
	cmd.AddCommand(newKeysListCmd(flags))
	cmd.AddCommand(newNovelKeysExportCmd(flags))
	cmd.AddCommand(newKeysImportCmd(flags))
	return cmd
}

func newNovelKeysShowCmd(flags *rootFlags) *cobra.Command {
	var reveal bool
	cmd := &cobra.Command{
		Use:         "show <site_id>",
		Short:       "Show a site's stored key metadata (use --reveal to print the secret)",
		Example:     "  ht-ml-pp-cli keys show e5051f46 --reveal",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site_id is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			site, err := db.GetSite(args[0])
			if err != nil {
				return err
			}
			if site == nil {
				return notFoundErr(fmt.Errorf("no site %q in the local store", args[0]))
			}
			out := map[string]any{
				"site_id": site.SiteID,
				"url":     site.URL,
				"title":   site.Title,
				"has_key": site.HasKey,
			}
			if reveal {
				out["update_key"] = site.UpdateKey
				if site.Password != "" {
					out["password"] = site.Password
				}
			} else if site.HasKey {
				out["update_key"] = "(hidden; pass --reveal to print)"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Print the secret update_key (and password) in clear text")
	return cmd
}

func newKeysListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List every site that has a stored update_key (no secrets printed)",
		Example:     "  ht-ml-pp-cli keys list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			sites, err := db.ListSites()
			if err != nil {
				return err
			}
			type row struct {
				SiteID string `json:"site_id"`
				URL    string `json:"url"`
				Title  string `json:"title,omitempty"`
				HasKey bool   `json:"has_key"`
			}
			out := make([]row, 0, len(sites))
			for _, s := range sites {
				out = append(out, row{SiteID: s.SiteID, URL: s.URL, Title: s.Title, HasKey: s.HasKey})
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("SITE_ID")+"\t"+bold("KEY")+"\t"+bold("URL"))
				for _, r := range out {
					mark := red("missing")
					if r.HasKey {
						mark = green("stored")
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\n", r.SiteID, mark, r.URL)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
}

func newKeysImportCmd(flags *rootFlags) *cobra.Command {
	var flagFile string
	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import update_keys from a vault file (encrypted or --insecure-plaintext export)",
		Long:  trimNL("Restore update_keys from a 'keys export' file (positional path or --file). Encrypted vaults need the passphrase in $HT_ML_VAULT_PASSPHRASE. Imported keys merge into the local store so update/rollback work from this machine."),
		Example: trimNL(`
  ht-ml-pp-cli keys import --file ht-ml-keys.vault
  HT_ML_VAULT_PASSPHRASE=secret ht-ml-pp-cli keys import --file ht-ml-keys.vault`),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := flagFile
			if path == "" && len(args) >= 1 {
				path = args[0]
			}
			if path == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if path == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a vault file path is required (positional or --file)"))
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "import-keys", "file": path}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "would import keys from", path)
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return usageErr(fmt.Errorf("reading %s: %w", path, err))
			}
			if isEncryptedVault(data) {
				pass, perr := vaultPassphrase()
				if perr != nil {
					return perr
				}
				plaintext, derr := decryptVault(data, pass)
				if derr != nil {
					return apiErr(derr)
				}
				data = plaintext
			}
			var records []keyExportRecord
			if err := json.Unmarshal(data, &records); err != nil {
				return apiErr(fmt.Errorf("parsing vault contents: %w", err))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			imported := 0
			for _, r := range records {
				if r.SiteID == "" {
					continue
				}
				rec := store.SiteRecord{
					SiteID:    r.SiteID,
					URL:       firstNonEmpty(r.URL, siteLiveURL(r.SiteID)),
					Title:     r.Title,
					UpdateKey: r.UpdateKey,
					Password:  r.Password,
					CreatedAt: r.CreatedAt,
				}
				if err := db.SaveSite(rec, "", false); err != nil {
					return fmt.Errorf("importing %s: %w", r.SiteID, err)
				}
				imported++
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s imported %d key(s)\n", green("ok:"), imported)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"imported": imported}, flags)
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Vault file to import (alternative to the positional argument)")
	return cmd
}
