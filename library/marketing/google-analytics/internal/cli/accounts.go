// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

var accountNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

func accountPath(name string) (string, error) {
	if !accountNamePattern.MatchString(name) || strings.Contains(name, "..") || name == "default-account" || name == "config.toml" {
		return "", fmt.Errorf("invalid account name: use 1-63 lowercase letters, digits, dots, underscores or hyphens")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "google-analytics-pp-cli", name, "credentials.json"), nil
}

func defaultAccountPath() (string, error) {
	p, err := accountPath("default")
	return filepath.Join(filepath.Dir(filepath.Dir(p)), "default-account"), err
}

func defaultAccount() (string, error) {
	p, err := defaultAccountPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return "default", nil
	}
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(b))
	if _, err := accountPath(name); err != nil {
		return "", err
	}
	return name, nil
}

func resolveAccount(f *rootFlags) (string, string, error) {
	name := f.accountName
	if name == "" {
		name = os.Getenv("GOOGLE_ANALYTICS_ACCOUNT")
	}
	if f.credentials != "" {
		if name != "" {
			return "", "", fmt.Errorf("--credentials cannot be combined with --account/GOOGLE_ANALYTICS_ACCOUNT")
		}
		return f.credentials, "default", nil
	}
	if name == "" {
		var err error
		name, err = defaultAccount()
		if err != nil {
			return "", "", err
		}
	}
	if name == "default" {
		return os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"), name, nil
	}
	p, err := accountPath(name)
	return p, name, err
}

func writePrivate(p string, b []byte, exclusive bool) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if exclusive {
		return os.Link(f.Name(), p)
	}
	return os.Rename(f.Name(), p)
}

func newAuthCmd(f *rootFlags) *cobra.Command {
	auth := &cobra.Command{Use: "auth", Short: "Manage named saved Google service accounts"}
	auth.AddCommand(newAuthAddCmd(f), newAuthAccountsCmd(f), newAuthSetDefaultCmd(f), newAuthRemoveCmd(f))
	return auth
}

func newAuthAddCmd(f *rootFlags) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "add <name>", Aliases: []string{"login"}, Short: "Save a Google service-account JSON key locally (no browser login)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "default" {
			return fmt.Errorf("default is reserved for existing --credentials/GOOGLE_APPLICATION_CREDENTIALS usage")
		}
		p, err := accountPath(args[0])
		if err != nil {
			return err
		}
		if source == "" {
			return fmt.Errorf("--from-credentials is required")
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			return fmt.Errorf("account already exists or cannot be accessed")
		}
		b, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		var key ga4.ServiceAccountKey
		if err := json.Unmarshal(b, &key); err != nil {
			return fmt.Errorf("invalid service-account JSON")
		}
		if err := ga4.ValidateKey(key); err != nil {
			return err
		}
		if err := writePrivate(p, b, true); err != nil {
			return err
		}
		return printJSON(cmd.OutOrStdout(), map[string]any{"added": args[0]})
	}}
	cmd.Flags().StringVar(&source, "from-credentials", "", "Service-account JSON key to copy into the named account")
	return cmd
}

func newAuthAccountsCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "accounts", Short: "List saved account names without credentials", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := defaultAccountPath()
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(filepath.Dir(p))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "default" {
				continue
			}
			p, err := accountPath(e.Name())
			if err != nil {
				continue
			}
			if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
				names = append(names, e.Name())
			}
		}
		if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
			names = append(names, "default")
		}
		sort.Strings(names)
		def, err := defaultAccount()
		if err != nil {
			return err
		}
		_, active, err := resolveAccount(f)
		if err != nil {
			return err
		}
		return printJSON(cmd.OutOrStdout(), map[string]any{"accounts": names, "default_account": def, "active_account": active})
	}}
}

func newAuthSetDefaultCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "set-default <name>", Short: "Choose the default Google account", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := accountPath(args[0])
		if err != nil {
			return err
		}
		if args[0] != "default" {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("account is not saved: %w", err)
			}
		}
		marker, err := defaultAccountPath()
		if err != nil {
			return err
		}
		if err := writePrivate(marker, []byte(args[0]+"\n"), false); err != nil {
			return err
		}
		return printJSON(cmd.OutOrStdout(), map[string]any{"default_account": args[0]})
	}}
}

func newAuthRemoveCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "remove <name>", Short: "Remove a saved key; never delete the original imported file", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !f.yes {
			return fmt.Errorf("confirmation required: pass --yes")
		}
		p, err := accountPath(args[0])
		if err != nil {
			return err
		}
		if args[0] == "default" {
			return fmt.Errorf("legacy credentials are externally managed; only named saved accounts can be removed")
		}
		def, err := defaultAccount()
		if err != nil {
			return err
		}
		if def == args[0] {
			return fmt.Errorf("select another default with 'auth set-default' before removing this account")
		}
		if err := os.Remove(p); err != nil {
			return err
		}
		return printJSON(cmd.OutOrStdout(), map[string]any{"removed": args[0]})
	}}
}
