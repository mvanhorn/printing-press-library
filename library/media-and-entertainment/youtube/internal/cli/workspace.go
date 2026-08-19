// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: named databank workspaces. Implemented body — regeneration preserves this file.
// pp:data-source computed
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"
	"strings"

	"github.com/spf13/cobra"
)

type workspaceView struct {
	Active     string            `json:"active"`
	Workspaces map[string]string `json:"workspaces"`
	Applied    bool              `json:"appliedToThisProcess,omitempty"`
	Hint       string            `json:"hint,omitempty"`
}

func newNovelWorkspaceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Short:   "Named databanks: keep the competitor machine in one database and explore a new niche in another, switching instantly.",
		Long:    "Use this command to create, list, and switch named databanks (e.g. an explore workspace for a new niche).\nDo NOT use it to manage tracked channels inside a workspace; use 'watch' instead.",
		Example: "  youtube-pp-cli workspace use niche-berlin --json\n  youtube-pp-cli workspace list --json\n  youtube-pp-cli workspace create explore",
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
	cmd.AddCommand(newWorkspaceListCmd(flags))
	cmd.AddCommand(newWorkspaceCreateCmd(flags))
	cmd.AddCommand(newWorkspaceUseCmd(flags))
	cmd.AddCommand(newWorkspaceRemoveCmd(flags))
	return cmd
}

func workspaceViewNow(applied bool, hint string) (*workspaceView, error) {
	reg, _, err := loadWorkspaceRegistry()
	if err != nil {
		return nil, err
	}
	if reg.Workspaces == nil {
		reg.Workspaces = map[string]string{}
	}
	return &workspaceView{Active: reg.Active, Workspaces: reg.Workspaces, Applied: applied, Hint: hint}, nil
}

func newWorkspaceListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List workspaces and show which one is active",
		Example:     "  youtube-pp-cli workspace list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "workspace list")
			}
			view, err := workspaceViewNow(false, "the 'default' workspace is the platform-default home; switch with 'workspace use NAME'")
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
}

func newWorkspaceCreateCmd(flags *rootFlags) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:         "create [name]",
		Short:       "Create a named workspace (its own database, cache, and config)",
		Example:     "  youtube-pp-cli workspace create explore",
		Annotations: map[string]string{"pp:happy-args": "name=example-workspace", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "workspace create")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("workspace name is required"))
			}
			name := args[0]
			if name == "default" {
				return usageErr(fmt.Errorf("'default' is reserved for the platform-default home"))
			}
			if !validWorkspaceName(name) {
				return usageErr(fmt.Errorf("invalid workspace name %q: use letters, digits, dot, dash, underscore only", name))
			}
			reg, _, err := loadWorkspaceRegistry()
			if err != nil {
				return configErr(err)
			}
			if _, exists := reg.Workspaces[name]; exists {
				return usageErr(fmt.Errorf("workspace %q already exists", name))
			}
			home := dir
			if home == "" {
				base, err := defaultWorkspacesBase()
				if err != nil {
					return configErr(err)
				}
				home = filepath.Join(base, name)
			}
			if err := os.MkdirAll(home, 0o700); err != nil {
				return configErr(err)
			}
			reg.Workspaces[name] = home
			if err := saveWorkspaceRegistry(reg); err != nil {
				return configErr(err)
			}
			view, err := workspaceViewNow(false, fmt.Sprintf("created %q at %s; activate with 'workspace use %s'", name, home, name))
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Explicit home directory for the workspace (default: config dir/workspaces/<name>)")
	return cmd
}

func newWorkspaceUseCmd(flags *rootFlags) *cobra.Command {
	var noKey bool
	cmd := &cobra.Command{
		Use:         "use [name]",
		Short:       "Switch the active workspace for all future invocations",
		Example:     "  youtube-pp-cli workspace use niche-berlin --json",
		Annotations: map[string]string{"pp:happy-args": "name=default", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "workspace use")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("workspace name is required"))
			}
			name := args[0]
			if name != "default" && !validWorkspaceName(name) {
				return usageErr(fmt.Errorf("invalid workspace name %q", name))
			}
			reg, _, err := loadWorkspaceRegistry()
			if err != nil {
				return configErr(err)
			}
			if name != "default" {
				home, ok := reg.Workspaces[name]
				if !ok {
					return usageErr(fmt.Errorf("workspace %q does not exist; create it with 'workspace create %s'", name, name))
				}
				if err := os.MkdirAll(home, 0o700); err != nil {
					return configErr(err)
				}
			}
			reg.Active = name
			if err := saveWorkspaceRegistry(reg); err != nil {
				return configErr(err)
			}
			hint := "active for all future invocations"
			// Seed credentials into a fresh workspace from the key ring so the
			// operator does not have to re-auth per workspace.
			if name != "default" && !noKey {
				if home := reg.Workspaces[name]; home != "" {
					// INVARIANT: no early return between the Setenv and the
					// restore below — activateRingKey resolves its config dir
					// from the environment, and leaving YOUTUBE_HOME pointed at
					// the new workspace would misplace later writes in this
					// process.
					prev := os.Getenv("YOUTUBE_HOME")
					_ = os.Setenv("YOUTUBE_HOME", home)
					// PATCH(pr1755-greptile: preserve workspace credentials) — seed only a
					// workspace that affirmatively has no credential. Fail SAFE: a load
					// error, a perms/parse refusal, or any non-empty existing file all
					// mean preserve — never overwrite a credential we could not read
					// (maintainer review on PR #1755).
					seedFromRing := false
					creds, credStatus, credErr := cliutil.LoadCredentialsWithStatus()
					switch {
					case credErr != nil || credStatus.Refusal != nil:
						hint = "active; workspace credential file preserved (unreadable or refused — not overwriting)"
					case credStatus.Loaded && creds.HasValues():
						hint = "active; existing workspace credential preserved"
					default:
						if fi, statErr := os.Stat(credStatus.Path); statErr == nil && fi.Size() > 0 {
							hint = "active; existing non-empty credential file preserved"
						} else {
							seedFromRing = true
						}
					}
					if seedFromRing {
						ring, ringErr := loadKeyRing()
						if ringErr == nil && ring.Active != "" {
							if err := activateRingKey(ring, ring.Active, ""); err == nil {
								hint = fmt.Sprintf("active; key %q seeded from the key ring", ring.Active)
							}
						}
					}
					if prev == "" {
						_ = os.Unsetenv("YOUTUBE_HOME")
					} else {
						_ = os.Setenv("YOUTUBE_HOME", prev)
					}
				}
			}
			view, err := workspaceViewNow(false, hint)
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().BoolVar(&noKey, "no-key-seed", false, "Do not seed the new workspace's credentials from the key ring")
	return cmd
}

func newWorkspaceRemoveCmd(flags *rootFlags) *cobra.Command {
	var deleteData bool
	cmd := &cobra.Command{
		Use:         "remove [name]",
		Short:       "Remove a workspace from the registry (data kept unless --delete-data)",
		Example:     "  youtube-pp-cli workspace remove explore",
		Annotations: map[string]string{"pp:happy-args": "name=example-workspace", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "workspace remove")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("workspace name is required"))
			}
			name := args[0]
			if name == "default" {
				return usageErr(fmt.Errorf("the default workspace cannot be removed"))
			}
			reg, _, err := loadWorkspaceRegistry()
			if err != nil {
				return configErr(err)
			}
			home, ok := reg.Workspaces[name]
			if !ok {
				return notFoundErr(fmt.Errorf("workspace %q does not exist", name))
			}
			delete(reg.Workspaces, name)
			if reg.Active == name {
				reg.Active = "default"
			}
			if err := saveWorkspaceRegistry(reg); err != nil {
				return configErr(err)
			}
			hint := fmt.Sprintf("removed from registry; data kept at %s", home)
			if deleteData {
				base, berr := defaultWorkspacesBase()
				cleanHome, _ := filepath.Abs(filepath.Clean(home))
				cleanBase := ""
				if berr == nil {
					cleanBase, _ = filepath.Abs(filepath.Clean(base))
				}
				if berr != nil || cleanBase == "" || !strings.HasPrefix(cleanHome, cleanBase+string(filepath.Separator)) {
					return usageErr(fmt.Errorf("--delete-data refused: %s is outside the managed workspaces directory; delete it manually if intended", home))
				}
				if err := os.RemoveAll(cleanHome); err != nil {
					return configErr(fmt.Errorf("registry updated but deleting %s failed: %w", cleanHome, err))
				}
				hint = "removed from registry and data deleted"
			}
			view, err := workspaceViewNow(false, hint)
			if err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().BoolVar(&deleteData, "delete-data", false, "Also delete the workspace's home directory (databank included)")
	return cmd
}
