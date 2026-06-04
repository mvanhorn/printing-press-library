// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// userdata_write.go declares the `userdata` namespace and its `write`
// subcommand: a guarded low-level escape hatch over Superhuman's
// /v3/userdata.write CRDT mutation endpoint, for paths the typed commands
// (snooze, archive, reminders, drafts) do not cover.
//
// Guards (see KD2 in the plan): the write is dry-run-by-default and only
// fires with --apply; the path must start with "users/" so a typo cannot
// target an unrelated namespace.
//
// PATCH(2026-05-27-005 U2): new low-level userdata write escape hatch.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
)

// newUserdataCmd registers the `userdata` namespace. Today it carries the
// raw `write` escape hatch; a `read` counterpart can plug in later.
func newUserdataCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "userdata",
		Short: "Low-level Superhuman userdata CRDT surface (advanced)",
		Long: `Raw access to Superhuman's userdata.* CRDT endpoints.

This is an advanced escape hatch for paths the typed commands do not cover.
Prefer the dedicated commands (send, reminders, drafts, snippets) when one
exists for what you need.`,
	}
	cmd.AddCommand(newUserdataWriteCmd(flags))
	return cmd
}

// newUserdataWriteCmd registers `userdata write <path> <json>`.
func newUserdataWriteCmd(flags *rootFlags) *cobra.Command {
	// Deliberately a dedicated --apply gate, not the global --yes: --yes
	// auto-trues under --agent (see root.go PersistentPreRunE), which would
	// make a raw CRDT write auto-fire in agent mode. --apply must always be
	// passed explicitly to perform the write.
	var apply bool
	cmd := &cobra.Command{
		Use:   "write <path> <json>",
		Short: "Write a raw value to a Superhuman CRDT path (dry-run by default)",
		Long: `Write a raw JSON value to a Superhuman userdata CRDT path via
POST /v3/userdata.write.

This is dry-run by default: without --apply the command prints the request it
would send and exits without firing. Pass --apply to actually perform the
write.

The path must start with "users/" (e.g. users/<google-id>/settings/<key>) so
a typo cannot target an unrelated namespace. The value must be well-formed
JSON (object, array, string, number, bool, or null).`,
		Example: strings.Trim(`
  superhuman-pp-cli userdata write "users/106.../settings/probe" '{"hi":true}'
  superhuman-pp-cli userdata write "users/106.../settings/probe" '{"hi":true}' --apply`, "\n"),
		Annotations: map[string]string{
			"pp:endpoint": "userdata.write",
			"pp:method":   "POST",
			"pp:path":     "/v3/userdata.write",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			path := strings.TrimSpace(args[0])
			rawValue := args[1]

			if !strings.HasPrefix(path, "users/") {
				return usageErr(fmt.Errorf("userdata write: path must start with \"users/\" (got %q)", path))
			}
			if !json.Valid([]byte(rawValue)) {
				return usageErr(fmt.Errorf("userdata write: value is not well-formed JSON: %q", rawValue))
			}

			// Verify mode never performs a real write. Checked before the
			// dry-run guard so it stays reachable regardless of --apply,
			// mirroring lookup.go's ordering.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would POST /v3/userdata.write")
				return nil
			}

			body := map[string]any{
				"writes": []map[string]any{
					{"path": path, "value": json.RawMessage(rawValue)},
				},
			}

			// Dry-run by default: print the request envelope and stop unless
			// --apply is set. The global --dry-run also forces this path.
			if flags.dryRun || !apply {
				envelope := map[string]any{
					"endpoint": "/v3/userdata.write",
					"method":   "POST",
					"dry_run":  true,
					"body":     body,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Post("/v3/userdata.write", body)
			if err != nil {
				if errors.Is(err, auth.ErrUnauthorized) {
					return authErr(fmt.Errorf("userdata write: %w", err))
				}
				return apiErr(fmt.Errorf("userdata write: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually perform the write (without this, the command only prints the request)")
	return cmd
}
