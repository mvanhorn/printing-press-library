// Copyright 2026 rai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var pollSeconds int
	var maxPolls int

	cmd := &cobra.Command{
		Use:   "watch [match-id]",
		Short: "Poll a match score on a loop, printing updates as they happen",
		Long: `Watch a single match by ID — polls /match_info at a configurable interval
and prints new score states. Useful for following a match in the terminal without
keeping a browser tab open.`,
		Example: "  cricapi-pp-cli watch abc-123\n  cricapi-pp-cli watch abc-123 --interval 60 --max 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			matchID := strings.TrimSpace(args[0])
			if matchID == "" {
				return cmd.Help()
			}
			if pollSeconds < 15 {
				pollSeconds = 15
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			seen := ""
			polls := 0
			for {
				path := "/match_info"
				params := map[string]string{"id": matchID, "offset": "0"}
				data, _, ferr := resolveRead(cmd.Context(), c, flags, "matches", false, path, params, nil)
				if ferr != nil {
					return classifyAPIError(ferr, flags)
				}

				var env struct {
					Data teamMatch `json:"data"`
				}
				if err := json.Unmarshal(data, &env); err == nil {
					m := env.Data
					snap := fmt.Sprintf("%s | %s", m.Status, m.Name)
					if snap != seen {
						ts := time.Now().Format("15:04:05")
						fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", ts, snap)
						seen = snap
					}
					if m.Ended {
						fmt.Fprintln(cmd.OutOrStdout(), "Match ended.")
						return nil
					}
				}

				polls++
				if maxPolls > 0 && polls >= maxPolls {
					return nil
				}

				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(time.Duration(pollSeconds) * time.Second):
				}
			}
		},
	}
	cmd.Flags().IntVar(&pollSeconds, "interval", 30, "Poll interval in seconds (minimum 15)")
	cmd.Flags().IntVar(&maxPolls, "max", 0, "Max number of polls before exiting (0 = until match ends)")
	return cmd
}
