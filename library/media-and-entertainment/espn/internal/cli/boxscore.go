package cli

import (
	"github.com/spf13/cobra"
)

func newBoxscoreCmd(flags *rootFlags) *cobra.Command {
	var league string

	cmd := &cobra.Command{
		Use:     "boxscore <event-id>",
		Short:   "Show ESPN boxscore data for an event",
		Example: `  espn-pp-cli boxscore 401671793 --league nfl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if !flags.dryRun {
					return cmd.Help()
				}
				args = []string{"401671793"}
			}
			spec, err := resolveLeagueSpec(league)
			if err != nil {
				return err
			}
			client := newESPNClient(flags)
			data, err := client.Boxscore(spec.Sport, spec.League, args[0])
			if err != nil {
				return classifyAPIError(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), normalizeOutput(data), flags)
		},
	}

	cmd.Flags().StringVar(&league, "league", "", "League key (e.g. mlb, nfl, nba, nhl)")
	_ = cmd.MarkFlagRequired("league")
	return cmd
}
