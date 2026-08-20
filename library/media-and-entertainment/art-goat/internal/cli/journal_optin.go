// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"

	"github.com/spf13/cobra"
)

// journalPrefs is the small persisted preference file for journal opt-ins.
// Kept deliberately tiny: opt-ins are off by default. The on-disk file
// lives next to the SQLite DB so it travels with the user's practice.
type journalPrefs struct {
	ShowStreak bool `json:"show_streak"`
}

func newJournalOptInCmd(flags *rootFlags) *cobra.Command {
	var showStreak bool
	var clear bool

	cmd := &cobra.Command{
		Use:   "opt-in",
		Short: "Opt into per-sit greetings (default: off)",
		Long: `Opt into per-sit greetings such as a streak indicator at the start of
each sit. Off by default; art-goat's stance is that habit-streak nudges
are an incidental feature, not the practice itself.

The preference is stored at ~/.local/share/art-goat-pp-cli/journal-prefs.json.`,
		Example: `  # Show a brief streak greeting at the start of each sit
  art-goat-pp-cli journal opt-in --show-streak

  # Turn it back off
  art-goat-pp-cli journal opt-in --clear

  # See the current setting
  art-goat-pp-cli journal opt-in`,
		Annotations: map[string]string{
			// Writes a preferences file when --show-streak or --clear is
			// passed, so mcp:hidden is the correct annotation. A false
			// readOnlyHint on a mutating tool is a real bug per AGENTS.md.
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return emitOptInVerifyEnvelope(cmd, flags)
			}

			prefs, err := loadJournalPrefs()
			if err != nil {
				return err
			}

			changed := false
			if clear {
				prefs.ShowStreak = false
				changed = true
			} else if cmd.Flags().Changed("show-streak") {
				prefs.ShowStreak = showStreak
				changed = true
			}

			if changed {
				if err := saveJournalPrefs(prefs); err != nil {
					return err
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"show_streak": prefs.ShowStreak,
					"path":        journalPrefsPath(),
				}, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "show_streak: %t\n", prefs.ShowStreak)
			fmt.Fprintf(out, "stored at:   %s\n", journalPrefsPath())
			return nil
		},
	}

	cmd.Flags().BoolVar(&showStreak, "show-streak", false, "Show a brief streak greeting at the start of each sit")
	cmd.Flags().BoolVar(&clear, "clear", false, "Turn all greetings off")
	return cmd
}

func journalPrefsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "art-goat-pp-cli", "journal-prefs.json")
}

func loadJournalPrefs() (journalPrefs, error) {
	path := journalPrefsPath()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return journalPrefs{}, nil
	}
	if err != nil {
		return journalPrefs{}, fmt.Errorf("read journal prefs: %w", err)
	}
	var p journalPrefs
	if len(b) == 0 {
		return p, nil
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("parse journal prefs: %w", err)
	}
	return p, nil
}

func saveJournalPrefs(p journalPrefs) error {
	path := journalPrefsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir journal prefs: %w", err)
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal prefs: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write journal prefs: %w", err)
	}
	return nil
}

func emitOptInVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "journal opt-in",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "opt-in writes a preferences file; PRINTING_PRESS_VERIFY=1 short-circuits the write",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
