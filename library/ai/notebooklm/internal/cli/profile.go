// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/cliutil"
	"github.com/spf13/cobra"
)

type profileStore struct {
	Profiles map[string]map[string]string `json:"profiles"`
}

func profileStorePath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

func loadProfiles() (*profileStore, error) {
	p, err := profileStorePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p) // #nosec G304 -- profile path under user config dir
	if err != nil {
		if os.IsNotExist(err) {
			return &profileStore{Profiles: map[string]map[string]string{}}, nil
		}
		return nil, err
	}
	var s profileStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Profiles == nil {
		s.Profiles = map[string]map[string]string{}
	}
	return &s, nil
}

func saveProfiles(s *profileStore) error {
	p, err := profileStorePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func newProfileCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Save and reuse named flag sets for recurring workflows",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved profiles",
		Example: `  notebooklm-pp-cli profile list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadProfiles()
			if err != nil {
				return err
			}
			return printJSON(s.Profiles)
		},
	})
	var name string
	save := &cobra.Command{
		Use:   "save",
		Short: "Save current global flags under a profile name",
		Example: `  notebooklm-pp-cli profile save --name daily --json --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return usageErr(fmt.Errorf("--name is required"))
			}
			s, err := loadProfiles()
			if err != nil {
				return err
			}
			s.Profiles[name] = map[string]string{
				"json":  fmt.Sprintf("%v", flags.asJSON),
				"agent": fmt.Sprintf("%v", flags.agent),
			}
			if err := saveProfiles(s); err != nil {
				return err
			}
			return printJSON(map[string]string{"saved": name})
		},
	}
	save.Flags().StringVar(&name, "name", "", "Profile name")
	cmd.AddCommand(save)
	return cmd
}
