// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

// pp:data-source live

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newChallengeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "challenge",
		Short:       "Create a constrained 10+0 challenge after explicit confirmation.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelChallengeTenCmd(flags))
	return cmd
}

func newNovelChallengeTenCmd(flags *rootFlags) *cobra.Command {
	var color string
	var rated bool
	var send bool

	cmd := &cobra.Command{
		Use:         "ten <username>",
		Short:       "Create an explicit 10+0 Lichess challenge for a named player only after --send is supplied.",
		Example:     "  lichess-pp-cli challenge ten a-named-player --dry-run",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if color != "random" && color != "white" && color != "black" {
				return usageErr(fmt.Errorf("--color must be random, white, or black"))
			}
			username := strings.TrimSpace(args[0])
			if username == "" {
				return usageErr(fmt.Errorf("username must not be blank"))
			}
			plan := map[string]any{
				"username": username, "time_control": "10+0", "clock_limit_seconds": 600,
				"clock_increment_seconds": 0, "color": color, "rated": rated,
				"safety": "Creates a challenge only; it never accepts, plays, or analyzes a live game.",
			}
			if dryRunOK(flags) {
				plan["dry_run"] = true
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			if !send {
				plan["send_required"] = true
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostForm(cmd.Context(), "/api/challenge/"+url.PathEscape(username), url.Values{
				"clock.limit": {"600"}, "clock.increment": {"0"},
				"color": {color}, "rated": {fmt.Sprintf("%t", rated)},
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var response any
			if err := json.Unmarshal(data, &response); err != nil {
				return err
			}
			plan["challenge"] = response
			plan["sent"] = true
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	cmd.Flags().StringVar(&color, "color", "random", "Challenge color: random, white, or black")
	cmd.Flags().BoolVar(&rated, "rated", false, "Create a rated challenge when the account and opponent permit it")
	cmd.Flags().BoolVar(&send, "send", false, "Send the challenge; without this flag the command only prints the request plan")
	return cmd
}
