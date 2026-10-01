// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Agent-mode safety and tailnet defaults applied to the whole command tree:
//   - In --agent mode, any mutating command runs as a dry run unless --yes
//     is passed, so an agent shows a plan before it changes the tailnet.
//   - TAILSCALE_TAILNET becomes the default for every --tailnet flag.

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
)

// tsMutationAnnotation marks commands that change tailnet state but carry no
// pp:method (e.g. the generated `import`). The hand-written safe-write
// commands do not need it: they are plan-only without --yes.
const tsMutationAnnotation = "pp:mutation"

func tsIsMutatingCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	ann := cmd.Annotations
	if ann[tsMutationAnnotation] == "true" {
		return true
	}
	if ann["mcp:read-only"] == "true" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(ann["pp:method"])) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

func applyTailnetEnvDefault(root *cobra.Command) {
	def := tsDefaultTailnet()
	if def == "-" {
		return
	}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if f := c.Flags().Lookup("tailnet"); f != nil && f.DefValue == "-" {
			_ = f.Value.Set(def)
			f.DefValue = def
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)
}

func init() {
	// Sync and other template-substituted paths resolve {tailnet} from
	// config TemplateVars, which only reads TAILSCALE_TAILNET. Default it to
	// "-" (the token's own tailnet) so sync works with no extra setup.
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.Config == nil {
			return nil
		}
		if c.Config.TemplateVars == nil {
			c.Config.TemplateVars = map[string]string{}
		}
		if strings.TrimSpace(c.Config.TemplateVars["tailnet"]) == "" {
			c.Config.TemplateVars["tailnet"] = tsDefaultTailnet()
		}
		return nil
	})
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		applyTailnetEnvDefault(root)
		guardSearchCommand(root, flags)
		// The API answers 404 when log streaming is not configured for the
		// requested log type, which is the normal state for most tailnets.
		if st, _, err := root.Find([]string{"tailnet", "logging", "get-log-streaming-status"}); err == nil && st != nil && st.Name() == "get-log-streaming-status" {
			if st.Annotations == nil {
				st.Annotations = map[string]string{}
			}
			st.Annotations["pp:typed-exit-codes"] = "0,3"
			st.Long = strings.TrimSpace(st.Long + "\n\nExit code 3 means log streaming is not configured for this log type.")
		}
		if imp, _, err := root.Find([]string{"import"}); err == nil && imp != nil && imp.Name() == "import" {
			if imp.Annotations == nil {
				imp.Annotations = map[string]string{}
			}
			imp.Annotations[tsMutationAnnotation] = "true"
			// The MCP mirror shells out without --agent, so import would POST
			// every record unconfirmed. Keep it off MCP; tailscale_execute
			// with confirm=true covers single creates.
			imp.Annotations["mcp:hidden"] = "true"
		}
		prev := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if prev != nil {
				if err := prev(cmd, args); err != nil {
					return err
				}
			}
			// Some generated commands (import) declare their own --dry-run,
			// which shadows the root flag and is what they read. Force that
			// one too, or the command sends live writes while the agent is
			// told it ran a dry run.
			local := cmd.Flags().Lookup("dry-run")
			alreadyDry := flags.dryRun || (local != nil && local.Value.String() == "true")
			if flags.agent && !flags.yes && !alreadyDry && tsIsMutatingCommand(cmd) {
				flags.dryRun = true
				if local != nil {
					_ = local.Value.Set("true")
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "agent mode: %q changes the tailnet, so this run is a dry run. Re-run with --yes to apply.\n", cmd.CommandPath())
			}
			return nil
		}
	})
}

// guardSearchCommand keeps `search` local. The Tailscale API has no search
// endpoint; the generator's name heuristic matches /tailnet/{tailnet}/dns/
// searchpaths instead. Depending on spec order it emits the GET (which would
// return DNS search domains as if they were results) or the POST (which sets
// DNS search paths; an early build of this CLI shipped that). Search reads
// only the local store populated by `sync`.
func guardSearchCommand(root *cobra.Command, flags *rootFlags) {
	s, _, err := root.Find([]string{"search"})
	if err != nil || s == nil || s.Name() != "search" || s.RunE == nil {
		return
	}
	prev := s.RunE
	s.Short = "Full-text search across locally synced data (run sync first)"
	s.Long = strings.TrimSpace(`
Search locally synced data with SQLite full-text search. The Tailscale API has
no search endpoint, so this command always reads the local store; run
'tailscale-pp-cli sync' first to populate it.`)
	s.Example = strings.Trim(`
  tailscale-pp-cli sync --resources devices,users
  tailscale-pp-cli search "macOS" --type devices
  tailscale-pp-cli search "home-mac" --json --limit 5`, "\n")
	s.RunE = func(cmd *cobra.Command, args []string) error {
		if flags.dataSource == "live" {
			return usageErr(fmt.Errorf("the Tailscale API has no search endpoint; search reads the local store (run 'tailscale-pp-cli sync' first)"))
		}
		flags.dataSource = "local"
		return prev(cmd, args)
	}
}
