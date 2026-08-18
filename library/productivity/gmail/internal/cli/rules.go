// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `rules` family: a local rulebook of named cleanup recipes
// (query + trash-or-label action) stored at <auth-dir>/rules.json (0600).
// `rules run` (rules_run.go) replays every enabled rule through the SAME
// preview->confirm engine as `cleanup plan` — one merged plan, one token,
// and it NEVER auto-applies.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const rulebookVersion = 1

// mailRule is one named local recipe.
type mailRule struct {
	Name      string   `json:"name"`
	Query     string   `json:"q"`
	Action    string   `json:"action"` // trash | label
	Add       []string `json:"add,omitempty"`
	Remove    []string `json:"remove,omitempty"`
	Enabled   bool     `json:"enabled"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// rulebook is the on-disk shape of rules.json.
type rulebook struct {
	Version int        `json:"version"`
	Rules   []mailRule `json:"rules"`
}

func rulebookPath(authDir string) string {
	return filepath.Join(authDir, "rules.json")
}

// loadRulebook reads rules.json; a missing file is an empty book.
func loadRulebook(authDir string) (*rulebook, error) {
	b, err := os.ReadFile(filepath.Clean(rulebookPath(authDir))) // #nosec G304 -- app-derived auth-dir path.
	if err != nil {
		if os.IsNotExist(err) {
			return &rulebook{Version: rulebookVersion}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", rulebookPath(authDir), err)
	}
	var rb rulebook
	if err := json.Unmarshal(b, &rb); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", rulebookPath(authDir), err)
	}
	return &rb, nil
}

// saveRulebook writes rules.json (0600, dir 0700).
func saveRulebook(authDir string, rb *rulebook) error {
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return err
	}
	rb.Version = rulebookVersion
	b, err := json.MarshalIndent(rb, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rulebookPath(authDir), append(b, '\n'), 0o600)
}

func newNovelRulesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rules",
		Short:       "Named local cleanup recipes (query + trash/label action), replayed through the preview-confirm-undo engine as one merged plan",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRulesAddCmd(flags))
	cmd.AddCommand(newRulesListCmd(flags))
	cmd.AddCommand(newRulesRmCmd(flags))
	cmd.AddCommand(newNovelRulesRunCmd(flags))
	return cmd
}

func newRulesAddCmd(flags *rootFlags) *cobra.Command {
	var name, q, action, addCSV, removeCSV string
	var disabled bool

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a named rule (query + trash/label action) to the local rulebook — nothing runs until 'rules run'",
		Long: `Record a named recipe in <auth-dir>/rules.json (0600). Rules are
strictly local configuration: adding one performs no network call and
mutates nothing. 'rules run' previews every enabled rule as ONE merged
plan and stops at the plan + token; applying is always a separate,
explicit 'cleanup apply'.

Typed exits: 0 recorded / 2 usage (duplicate name, bad action).`,
		Example: `  gmail-pp-cli rules add --name stale-promos --q "category:promotions older_than:1y" --action trash
  gmail-pp-cli rules add --name newsletters --q "list:unsubscribe" --action label --add Newsletters --remove UNREAD`,
		Annotations: map[string]string{"mcp:local-write": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name = strings.TrimSpace(name)
			q = strings.TrimSpace(q)
			if name == "" || q == "" {
				return usageErr(fmt.Errorf("--name and --q are both required"))
			}
			act, err := parseCleanupAction(action, addCSV, removeCSV)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			authDir := gauthConfigDirFrom(flags.authDir)
			rb, err := loadRulebook(authDir)
			if err != nil {
				return err
			}
			for _, r := range rb.Rules {
				if strings.EqualFold(r.Name, name) {
					return usageErr(fmt.Errorf("a rule named %q already exists — 'rules rm --name %s' first, or pick another name", r.Name, r.Name))
				}
			}
			rule := mailRule{
				Name: name, Query: q, Action: act.Type,
				Add: act.Add, Remove: act.Remove,
				Enabled: !disabled, CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}
			rb.Rules = append(rb.Rules, rule)
			if err := saveRulebook(authDir, rb); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"added": rule, "rules": len(rb.Rules), "path": rulebookPath(authDir),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Unique rule name")
	cmd.Flags().StringVar(&q, "q", "", "Gmail search query the rule freezes ids from at run time")
	cmd.Flags().StringVar(&action, "action", "", "What the rule does: trash | label")
	cmd.Flags().StringVar(&addCSV, "add", "", "Labels to add (label action): comma-separated label IDs or names, resolved at run time")
	cmd.Flags().StringVar(&removeCSV, "remove", "", "Labels to remove (label action)")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "Record the rule disabled ('rules run' skips it)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("q")
	_ = cmd.MarkFlagRequired("action")
	return cmd
}

func newRulesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the local rulebook",
		Example:     `  gmail-pp-cli rules list --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rb, err := loadRulebook(gauthConfigDirFrom(flags.authDir))
			if err != nil {
				return err
			}
			if rb.Rules == nil {
				rb.Rules = []mailRule{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), rb.Rules, flags)
		},
	}
	return cmd
}

func newRulesRmCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:         "rm",
		Short:       "Remove a rule from the local rulebook by name",
		Example:     `  gmail-pp-cli rules rm --name stale-promos`,
		Annotations: map[string]string{"mcp:local-write": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name = strings.TrimSpace(name)
			if name == "" {
				return usageErr(fmt.Errorf("--name is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			authDir := gauthConfigDirFrom(flags.authDir)
			rb, err := loadRulebook(authDir)
			if err != nil {
				return err
			}
			kept := rb.Rules[:0]
			removed := false
			for _, r := range rb.Rules {
				if strings.EqualFold(r.Name, name) {
					removed = true
					continue
				}
				kept = append(kept, r)
			}
			if !removed {
				names := make([]string, 0, len(rb.Rules))
				for _, r := range rb.Rules {
					names = append(names, r.Name)
				}
				return usageErr(fmt.Errorf("no rule named %q (have: %s)", name, strings.Join(names, ", ")))
			}
			rb.Rules = kept
			if err := saveRulebook(authDir, rb); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"removed": name, "rules": len(rb.Rules),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Rule name to remove")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
