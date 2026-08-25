// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newGiveCmd(flags *rootFlags) *cobra.Command {
	var flagTo []string
	var flagAmount int
	var flagMessage string
	var flagHashtag string

	cmd := &cobra.Command{
		Use:     "give",
		Short:   "Give recognition to one or more colleagues",
		Example: "  bonusly-pp-cli give --to jane@example.com --amount 50 --message 'great work' --hashtag teamwork",
		Annotations: map[string]string{
			"pp:happy-args": "--to=jane@example.com;--amount=50;--message=great work;--hashtag=teamwork",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}

			// Validate required flags
			if len(flagTo) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is required"))
			}
			if flagAmount <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount is required and must be positive"))
			}
			if flagMessage == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--message is required"))
			}
			if flagHashtag == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--hashtag is required"))
			}

			hashtag := flagHashtag
			if strings.HasPrefix(hashtag, "#") {
				hashtag = strings.TrimPrefix(hashtag, "#")
			}

			var mentions []string
			for _, t := range flagTo {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				if !strings.HasPrefix(t, "@") {
					t = "@" + t
				}
				mentions = append(mentions, t)
			}
			if len(mentions) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to must include at least one non-empty recipient"))
			}
			mentionsStr := strings.Join(mentions, " ")
			reasonSent := fmt.Sprintf("+%d %s %s #%s", flagAmount, mentionsStr, flagMessage, hashtag)

			if dryRunOK(flags) {
				// pp:hand-edit bonusly-endpoint-fix — was an unconditional
				// Fprintln, so --dry-run --json produced plain text instead
				// of JSON. writeDryRun already handles the --json branch.
				return writeDryRun(cmd.OutOrStdout(), flags, "give "+reasonSent)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			body := map[string]any{
				"reason": reasonSent,
			}

			respRaw, _, err := c.Post(ctx, "/bonuses", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var apiResult map[string]any
			_ = json.Unmarshal(respRaw, &apiResult)

			res := map[string]any{
				"reason_sent": reasonSent,
				"result":      apiResult,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully sent recognition! Reason sent: %s\n", reasonSent)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&flagTo, "to", nil, "Who to recognize (comma-separated emails or display names)")
	cmd.Flags().IntVar(&flagAmount, "amount", 0, "Amount of points to give")
	cmd.Flags().StringVar(&flagMessage, "message", "", "The message to include with the recognition")
	cmd.Flags().StringVar(&flagHashtag, "hashtag", "", "The hashtag representing a company value")

	return cmd
}
