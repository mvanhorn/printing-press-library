// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Profile is a named set of flag values saved for reuse across invocations.
// Useful when an agent or human always reaches for the same flag combination —
// e.g. `--json --agent --view-id 123456789 --limit 50` — and wants to replay
// it via `--profile open-tickets` instead of retyping every call.
type Profile struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Values      map[string]string `json:"values"`
}

type profileStore struct {
	Profiles map[string]Profile `json:"profiles"`
}

// profileStorePath returns the XDG-config path for the profile store
// (default ~/.config/gorgias-pp-cli/profiles.json), co-located with
// config.toml. $XDG_CONFIG_HOME overrides the parent dir when set. On
// first call, lazily migrates any legacy ~/.gorgias-pp-cli/profiles.json
// from before the XDG fix so users don't lose saved profiles.
func profileStorePath() (string, error) {
	dir := filepath.Join(xdgConfigHome(), "gorgias-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating config dir: %w", err)
	}
	newPath := filepath.Join(dir, "profiles.json")
	migrateLegacyDotfile("profiles.json", newPath)
	return newPath, nil
}

func loadProfileStore() (*profileStore, error) {
	p, err := profileStorePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &profileStore{Profiles: map[string]Profile{}}, nil
		}
		return nil, fmt.Errorf("reading profiles: %w", err)
	}
	var s profileStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing profiles: %w", err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	return &s, nil
}

func saveProfileStore(s *profileStore) error {
	p, err := profileStorePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing profiles: %w", err)
	}
	return os.Rename(tmp, p)
}

// GetProfile returns a profile by name, or (nil, nil) if not found.
func GetProfile(name string) (*Profile, error) {
	s, err := loadProfileStore()
	if err != nil {
		return nil, err
	}
	if p, ok := s.Profiles[name]; ok {
		return &p, nil
	}
	return nil, nil
}

// ApplyProfileToFlags overlays profile values onto flags that the user has
// not set explicitly on the command line. Used from root.go's
// PersistentPreRunE so profile values feed the whole command tree.
func ApplyProfileToFlags(cmd *cobra.Command, profile *Profile) error {
	if profile == nil || len(profile.Values) == 0 {
		return nil
	}
	// Reserved flags that never come from a profile - they control profile
	// resolution itself or are dangerous to overlay.
	reserved := map[string]bool{
		"profile": true, "config": true, "help": true,
	}
	for name, value := range profile.Values {
		if reserved[name] {
			continue
		}
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			flag = cmd.InheritedFlags().Lookup(name)
		}
		if flag == nil {
			continue
		}
		if flag.Changed {
			continue
		}
		if err := flag.Value.Set(value); err != nil {
			return fmt.Errorf("applying profile value %s=%q: %w", name, value, err)
		}
	}
	return nil
}

// ListProfileNames returns profile names sorted alphabetically. Used by the
// agent-context subcommand to expose available_profiles at runtime.
func ListProfileNames() []string {
	s, err := loadProfileStore()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(s.Profiles))
	for name := range s.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func newProfileCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Named sets of flags saved for reuse",
		Long: "Profiles let you save commonly-used flag combinations under a short name and replay\n" +
			"them with `--profile <name>` on any future invocation.\n\n" +
			"Storage: $XDG_CONFIG_HOME/gorgias-pp-cli/profiles.json (default ~/.config/...).\n" +
			"Co-located with config.toml so all CLI state lives under one XDG path.\n\n" +
			"Sub-commands:\n" +
			"  * save <name>  — capture the non-default flags from the current invocation\n" +
			"  * use <name>   — print the flag values a profile will apply (no execution)\n" +
			"  * list         — list all saved profiles with their descriptions\n" +
			"  * show <name>  — emit one profile as JSON\n" +
			"  * delete <name>— remove a profile\n\n" +
			"Profile values are applied as flag overrides at the start of the run. Flags explicitly\n" +
			"set on the command line still win over profile values.",
	}
	cmd.AddCommand(newProfileSaveCmd(flags))
	cmd.AddCommand(newProfileUseCmd(flags))
	cmd.AddCommand(newProfileListCmd(flags))
	cmd.AddCommand(newProfileShowCmd(flags))
	cmd.AddCommand(newProfileDeleteCmd(flags))
	return cmd
}

func newProfileSaveCmd(flags *rootFlags) *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "save <name> [--<flag> <value> ...]",
		Short: "Save the current invocation's non-default flags as a named profile",
		Example: `  gorgias-pp-cli profile save my-defaults --json --compact
  gorgias-pp-cli profile save open-tickets --view-id 123456789 --limit 50`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if strings.ContainsAny(name, `/\: `) {
				return fmt.Errorf("profile name %q contains reserved characters", name)
			}
			values := map[string]string{}
			// Walk inherited + local flags, capture only those the user set.
			skip := map[string]bool{"profile": true, "config": true, "help": true, "description": true}
			visit := func(fl *pflag.Flag) {
				if fl.Changed && !skip[fl.Name] {
					values[fl.Name] = fl.Value.String()
				}
			}
			cmd.InheritedFlags().VisitAll(visit)
			cmd.Flags().VisitAll(visit)
			if len(values) == 0 {
				return fmt.Errorf("no non-default flags set - pass at least one flag to save into %q", name)
			}
			s, err := loadProfileStore()
			if err != nil {
				return err
			}
			s.Profiles[name] = Profile{Name: name, Description: description, Values: values}
			if err := saveProfileStore(s); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), s.Profiles[name], flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved profile %q with %d values\n", name, len(values))
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Short description shown in 'profile list'")
	return cmd
}

func newProfileUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Print the flag values a profile will apply (does not execute anything)",
		Example: `  gorgias-pp-cli profile use my-defaults
  gorgias-pp-cli profile use open-tickets --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := GetProfile(args[0])
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("profile %q not found", args[0])
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), p, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "profile %q:\n", p.Name)
			if p.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  description: %s\n", p.Description)
			}
			keys := make([]string, 0, len(p.Values))
			for k := range p.Values {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "  --%s %s\n", k, p.Values[k])
			}
			return nil
		},
	}
}

func newProfileListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved profiles",
		Example: `  gorgias-pp-cli profile list
  gorgias-pp-cli profile list --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := loadProfileStore()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(s.Profiles))
			for n := range s.Profiles {
				names = append(names, n)
			}
			sort.Strings(names)
			if flags.asJSON {
				out := make([]map[string]any, 0, len(names))
				for _, n := range names {
					p := s.Profiles[n]
					out = append(out, map[string]any{
						"name":        p.Name,
						"description": p.Description,
						"field_count": len(p.Values),
					})
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			headers := []string{"NAME", "FIELDS", "DESCRIPTION"}
			rows := make([][]string, 0, len(names))
			for _, n := range names {
				p := s.Profiles[n]
				rows = append(rows, []string{p.Name, fmt.Sprintf("%d", len(p.Values)), p.Description})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
}

func newProfileShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a profile's values as JSON",
		Example: `  gorgias-pp-cli profile show my-defaults
  gorgias-pp-cli profile show open-tickets --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := GetProfile(args[0])
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("profile %q not found", args[0])
			}
			return printJSONFiltered(cmd.OutOrStdout(), p, flags)
		},
	}
}

func newProfileDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a profile",
		Example: `  gorgias-pp-cli profile delete my-defaults --yes
  gorgias-pp-cli profile delete old-profile --yes --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := loadProfileStore()
			if err != nil {
				return err
			}
			if _, ok := s.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			if !flags.yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "refusing to delete %q without --yes\n", name)
				return fmt.Errorf("confirmation required: pass --yes")
			}
			delete(s.Profiles, name)
			if err := saveProfileStore(s); err != nil {
				return err
			}
			// JSON envelope: {deleted: name}.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"deleted": name,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted profile %q\n", name)
			return nil
		},
	}
}
