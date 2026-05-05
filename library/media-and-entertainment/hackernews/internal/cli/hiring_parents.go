package cli

import "github.com/spf13/cobra"

// newHiringCmd is the parent of `hiring filter`, `hiring stats`, and `hiring companies`.
// The HN "Ask HN: Who is hiring?" thread runs on the first weekday of every month;
// this group of commands mines those threads.
func newHiringCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hiring",
		Short: "Mine 'Ask HN: Who is hiring' threads (filter, stats, companies)",
		Long: `Tools for mining the monthly 'Ask HN: Who is hiring?' threads.

Subcommands:
  filter     Filter the latest thread by regex (haxor-news compatibility)
  stats      Aggregate the last N months: top languages, remote ratio, top companies
  companies  Companies that posted in M of the last N months — first-seen, last-seen, count`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newHiringFilterCmd(flags))
	cmd.AddCommand(newHiringStatsCmd(flags))
	cmd.AddCommand(newHiringCompaniesCmd(flags))
	return cmd
}

// newFreelanceCmd is the parent of `freelance filter`. Single subcommand for now;
// gives a future home for `freelance stats` if the demand emerges.
func newFreelanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freelance",
		Short: "Mine 'Ask HN: Freelancer? Seeking Freelancer?' threads",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newFreelanceFilterCmd(flags))
	return cmd
}
