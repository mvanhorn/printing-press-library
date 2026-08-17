// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// bookmarks lists the signed-in user's saved Placeit templates. It resolves the
// user id automatically from the account endpoint and degrades gracefully (a
// clear "log in" hint, empty result, exit 0) when no Placeit session is present.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newBookmarksCmd(flags *rootFlags) *cobra.Command {
	var userID string
	cmd := &cobra.Command{
		Use:   "bookmarks",
		Short: "List your saved/bookmarked Placeit templates (requires a logged-in session)",
		Long: strings.Trim(`
List the templates you've bookmarked on Placeit. Your user id is resolved
automatically from your account session, so you usually just run 'bookmarks'.
Authenticate first by importing your Chrome session (see the auth login step in
the README Quick Start). Without a session this returns an empty list and a
hint to log in.`, "\n"),
		Example:     "  placeit-pp-cli bookmarks --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Resolve the user id: explicit flag wins, otherwise pull it from the
			// account endpoint (which also tells us whether a session exists).
			if userID == "" {
				acc, aerr := c.Get(ctx, "/api/v1/get_user_type_banner", nil)
				if aerr == nil {
					var am map[string]any
					if json.Unmarshal(acc, &am) == nil {
						if uid, ok := am["user_id"]; ok {
							userID = formatCLIParamValue(uid)
						}
					}
				}
			}
			if userID == "" || userID == "0" {
				fmt.Fprintln(cmd.ErrOrStderr(), "not signed in to Placeit — import your Chrome session via the auth login step in Quick Start, then retry")
				return flags.printJSON(cmd, []any{})
			}

			data, gerr := c.Get(ctx, "/api/v2/bookmarked_stages_from_user", map[string]string{"user_id": userID})
			if gerr != nil {
				// Treat an auth/transport failure as "no bookmarks reachable"
				// rather than a hard error, with a hint.
				fmt.Fprintf(cmd.ErrOrStderr(), "could not load bookmarks (session may be missing or expired): %v\n", gerr)
				return flags.printJSON(cmd, []any{})
			}
			var wrapper map[string]json.RawMessage
			if json.Unmarshal(data, &wrapper) == nil {
				if arr, ok := wrapper["bookmarkedStages"]; ok {
					return printOutputWithFlags(cmd.OutOrStdout(), arr, flags)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "Placeit user id (auto-resolved from your session when omitted)")
	return cmd
}
