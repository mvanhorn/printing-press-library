// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: API key ring. Implemented body — regeneration preserves this file.
// pp:data-source computed
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type keyRingView struct {
	Active   string   `json:"active"`
	Keys     []string `json:"keys"`
	Masked   []string `json:"masked"`
	EnvTrap  string   `json:"envTrap,omitempty"`
	Hint     string   `json:"hint,omitempty"`
	Rotation string   `json:"rotation,omitempty"`
}

func keyRingViewNow(hint string) (*keyRingView, error) {
	ring, err := loadKeyRing()
	if err != nil {
		return nil, err
	}
	view := &keyRingView{Active: ring.Active, Keys: ring.names(), Masked: make([]string, 0), Hint: hint,
		Rotation: "commands with --rotate fail over to the next key when quota is exhausted"}
	for _, n := range view.Keys {
		view.Masked = append(view.Masked, n+"="+maskKey(ring.Keys[n]))
	}
	if os.Getenv("YOUTUBE_API_KEY") != "" {
		view.EnvTrap = "YOUTUBE_API_KEY is set in the environment and OVERRIDES the ring's active key; unset it or update it"
	}
	return view, nil
}

func newNovelAuthKeysCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keys",
		Short:   "Store multiple named YouTube API keys, switch between them instantly",
		Long:    "Use this command to add, list, and switch between stored API keys.\nDo NOT use it for one-off key entry; use 'auth set-token' instead.",
		Example: "  youtube-pp-cli keys use secondary --json\n  youtube-pp-cli keys list --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:parent-group":     "true",
			"pp:happy-args":       "action=list",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAuthKeysListCmd(flags))
	cmd.AddCommand(newAuthKeysAddCmd(flags))
	cmd.AddCommand(newAuthKeysUseCmd(flags))
	cmd.AddCommand(newAuthKeysRemoveCmd(flags))
	return cmd
}

func newAuthKeysListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List stored key names (values masked) and which is active",
		Example:     "  youtube-pp-cli keys list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth keys list")
			}
			view, err := keyRingViewNow("switch with 'auth keys use NAME'")
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
}

func newAuthKeysAddCmd(flags *rootFlags) *cobra.Command {
	var activate bool
	var keyStdin bool
	cmd := &cobra.Command{
		Use:         "add [name] [api-key]",
		Short:       "Store a named API key in the ring",
		Long:        "Prefer --key-stdin (pipe the key in) over the positional form: an argv key lands in shell history and is briefly visible to local process listings.",
		Example:     "  printf '%s' \"$NEW_KEY\" | youtube-pp-cli keys add secondary --key-stdin",
		Annotations: map[string]string{"pp:happy-args": "name=example-key;key=example-value", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth keys add")
			}
			var name, val string
			switch {
			case keyStdin && len(args) >= 1:
				name = args[0]
				data, rerr := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 4096))
				if rerr != nil {
					return usageErr(fmt.Errorf("reading key from stdin: %w", rerr))
				}
				val = strings.TrimSpace(string(data))
				if val == "" {
					return usageErr(fmt.Errorf("--key-stdin given but stdin was empty"))
				}
			case len(args) >= 2:
				name, val = args[0], args[1]
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("need a key name plus the key value (positional, or via --key-stdin)"))
			}
			ring, err := loadKeyRing()
			if err != nil {
				return configErr(err)
			}
			if _, exists := ring.Keys[name]; !exists {
				ring.Order = append(ring.Order, name)
			}
			ring.Keys[name] = val
			if ring.Active == "" {
				ring.Active = name
			}
			if err := saveKeyRing(ring); err != nil {
				return configErr(err)
			}
			hint := fmt.Sprintf("stored %q", name)
			if activate {
				if err := activateRingKey(ring, name, flags.configPath); err != nil {
					return configErr(err)
				}
				hint = fmt.Sprintf("stored and activated %q", name)
			}
			view, err := keyRingViewNow(hint)
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().BoolVar(&activate, "use", false, "Activate the key immediately after storing it")
	cmd.Flags().BoolVar(&keyStdin, "key-stdin", false, "Read the api key value from stdin instead of argv (avoids shell history)")
	return cmd
}

func newAuthKeysUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "use [name]",
		Short:       "Activate a stored key (writes it into the credential store every command reads)",
		Example:     "  youtube-pp-cli keys use secondary --json",
		Annotations: map[string]string{"pp:happy-args": "name=example-key", "pp:typed-exit-codes": "0,2,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth keys use")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("key name is required"))
			}
			ring, err := loadKeyRing()
			if err != nil {
				return configErr(err)
			}
			if err := activateRingKey(ring, args[0], flags.configPath); err != nil {
				return configErr(err)
			}
			view, err := keyRingViewNow(fmt.Sprintf("activated %q", args[0]))
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
}

func newAuthKeysRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "remove [name]",
		Short:       "Remove a stored key from the ring",
		Example:     "  youtube-pp-cli keys remove old-project",
		Annotations: map[string]string{"pp:happy-args": "name=example-key", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth keys remove")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("key name is required"))
			}
			name := args[0]
			ring, err := loadKeyRing()
			if err != nil {
				return configErr(err)
			}
			if _, ok := ring.Keys[name]; !ok {
				return notFoundErr(fmt.Errorf("no stored key named %q", name))
			}
			delete(ring.Keys, name)
			for i, n := range ring.Order {
				if n == name {
					ring.Order = append(ring.Order[:i], ring.Order[i+1:]...)
					break
				}
			}
			if ring.Active == name {
				ring.Active = ""
				if names := ring.names(); len(names) > 0 {
					ring.Active = names[0]
				}
			}
			if err := saveKeyRing(ring); err != nil {
				return configErr(err)
			}
			view, err := keyRingViewNow(fmt.Sprintf("removed %q (credential store untouched; run 'auth keys use' to switch)", name))
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
}
