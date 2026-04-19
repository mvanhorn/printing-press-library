// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// config byok: store BYOK provider -> env-var-name mappings for use by the
// waterfall command. We store the env var NAME, never the key value — the
// CLI reads the value at runtime from os.Getenv.
//
// File format: ~/.config/contact-goat-pp-cli/byok.json
//   {
//     "providers": {
//       "hunter":  "HUNTER_API_KEY",
//       "apollo":  "APOLLO_API_KEY"
//     }
//   }

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const byokFilename = "byok.json"

// knownBYOKProviders is the recognized provider set. New providers can be
// added freely — unknown values are warned about but not rejected, which
// keeps the CLI forward-compatible with Deepline adding new BYOK options.
var knownBYOKProviders = map[string]string{
	"hunter":      "Hunter.io — email finder",
	"apollo":      "Apollo.io — B2B database",
	"clearbit":    "Clearbit — enrichment (deprecated upstream)",
	"dropcontact": "Dropcontact — GDPR email finder",
}

type byokFile struct {
	Providers map[string]string `json:"providers"`
}

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure contact-goat-pp-cli (BYOK providers, defaults, etc.)",
	}
	cmd.AddCommand(newConfigBYOKCmd(flags))
	return cmd
}

func newConfigBYOKCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "byok",
		Short: "Manage BYOK (bring-your-own-key) provider mappings",
		Long: `BYOK lets you pass your own Hunter / Apollo / etc. API keys through Deepline
so you pay the upstream provider directly and skip Deepline's credit markup.

This command stores only the env-var NAME for each provider. The key VALUE
itself is read at runtime from os.Getenv and never persisted on disk.

Subcommands:
  set <provider> <env-var-name>   record the mapping
  unset <provider>                remove a mapping
  list                            show all configured mappings (names only, never values)`,
		Example: `  contact-goat-pp-cli config byok set hunter HUNTER_API_KEY
  contact-goat-pp-cli config byok set apollo APOLLO_API_KEY
  contact-goat-pp-cli config byok list
  contact-goat-pp-cli config byok unset hunter`,
	}
	cmd.AddCommand(newConfigBYOKSetCmd(flags))
	cmd.AddCommand(newConfigBYOKUnsetCmd(flags))
	cmd.AddCommand(newConfigBYOKListCmd(flags))
	return cmd
}

func newConfigBYOKSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <provider> <env-var-name>",
		Short: "Record a BYOK provider -> env var name mapping",
		Long: `Set the env var NAME that holds your API key for a provider. The VALUE
of the env var is never read or stored by this command — the CLI reads it at
runtime only when a waterfall step actually needs it.`,
		Example: "  contact-goat-pp-cli config byok set hunter HUNTER_API_KEY",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(strings.TrimSpace(args[0]))
			envVar := strings.TrimSpace(args[1])
			if provider == "" || envVar == "" {
				return usageErr(fmt.Errorf("provider and env var name are both required"))
			}
			// Refuse to accept a value that looks like a key itself — defense in
			// depth against users pasting their Hunter key on the CLI.
			if looksLikeAPIKey(envVar) {
				return usageErr(fmt.Errorf("this looks like a key VALUE, not an env var NAME.\n" +
					"hint: pass an env var name like HUNTER_API_KEY, then `export HUNTER_API_KEY=...` separately"))
			}
			path := byokConfigPath()
			f := loadBYOKFile(path)
			if f.Providers == nil {
				f.Providers = map[string]string{}
			}
			f.Providers[provider] = envVar
			if err := writeBYOKFile(path, f); err != nil {
				return configErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved: %s -> $%s (value read at runtime, never stored)\n", provider, envVar)
			if _, known := knownBYOKProviders[provider]; !known {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %q is not a known BYOK provider; it was accepted anyway.\n", provider)
			}
			if os.Getenv(envVar) == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: $%s is not currently set. Export it before running `waterfall --byok`.\n", envVar)
			}
			return nil
		},
	}
}

func newConfigBYOKUnsetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "unset <provider>",
		Short:   "Remove a BYOK provider mapping",
		Example: "  contact-goat-pp-cli config byok unset hunter",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(strings.TrimSpace(args[0]))
			path := byokConfigPath()
			f := loadBYOKFile(path)
			if _, ok := f.Providers[provider]; !ok {
				return notFoundErr(fmt.Errorf("%s is not configured", provider))
			}
			delete(f.Providers, provider)
			if err := writeBYOKFile(path, f); err != nil {
				return configErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", provider)
			return nil
		},
	}
}

func newConfigBYOKListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "Show configured BYOK providers (env var names only, never values)",
		Example: "  contact-goat-pp-cli config byok list",
		RunE: func(cmd *cobra.Command, args []string) error {
			byok := readBYOKConfig()
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out := make(map[string]map[string]any, len(byok))
				for p, envVar := range byok {
					out[p] = map[string]any{
						"env_var_name": envVar,
						"env_var_set":  os.Getenv(envVar) != "",
					}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			w := cmd.OutOrStdout()
			if len(byok) == 0 {
				fmt.Fprintln(w, "no BYOK providers configured.")
				fmt.Fprintln(w, "Add one with: contact-goat-pp-cli config byok set hunter HUNTER_API_KEY")
				return nil
			}
			providers := make([]string, 0, len(byok))
			for p := range byok {
				providers = append(providers, p)
			}
			sort.Strings(providers)
			fmt.Fprintf(w, "%-12s %-28s %s\n", "PROVIDER", "ENV VAR", "SET?")
			for _, p := range providers {
				envVar := byok[p]
				set := "no"
				if os.Getenv(envVar) != "" {
					set = "yes"
				}
				fmt.Fprintf(w, "%-12s $%-27s %s\n", p, envVar, set)
			}
			return nil
		},
	}
}

// byokConfigPath returns the path to the byok.json file. It honors the
// --config flag's DIR part by sitting alongside the standard config file.
func byokConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "contact-goat-pp-cli", byokFilename)
}

// readBYOKConfig returns a flat provider -> env-var-name map. Safe to call on
// missing file — returns empty map.
func readBYOKConfig() map[string]string {
	f := loadBYOKFile(byokConfigPath())
	if f.Providers == nil {
		return map[string]string{}
	}
	return f.Providers
}

func loadBYOKFile(path string) byokFile {
	var f byokFile
	data, err := os.ReadFile(path)
	if err != nil {
		return f
	}
	_ = json.Unmarshal(data, &f)
	return f
}

func writeBYOKFile(path string, f byokFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing byok config: %w", err)
	}
	return nil
}

// looksLikeAPIKey is a best-effort guard against a user pasting a raw key as
// the env-var-name argument. It returns true for common key shapes (long
// base64-ish, dlp_/sk_/apollo_ prefixes, very long strings).
func looksLikeAPIKey(s string) bool {
	if len(s) > 40 {
		return true
	}
	for _, pfx := range []string{"sk-", "sk_", "dlp_", "dpl_", "apikey_", "bearer ", "apollo_", "hunter_"} {
		if strings.HasPrefix(strings.ToLower(s), pfx) {
			return true
		}
	}
	return false
}
