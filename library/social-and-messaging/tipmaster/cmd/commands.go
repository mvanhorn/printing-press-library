package cmd

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/tipmaster/internal"
)

var resolveCmd = &cobra.Command{
	Use:   "resolve <farcaster-username>",
	Short: "Resolve a Farcaster username to their XRPL wallet address",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := internal.NewClient()
		res, err := c.Get("/api/resolve/" + url.PathEscape(args[0]))
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var (
	lbPeriod string
	lbLimit  int
)

var leaderboardCmd = &cobra.Command{
	Use:   "leaderboard",
	Short: "Top tippers by RLUSD volume",
	RunE: func(cmd *cobra.Command, args []string) error {
		q := url.Values{}
		q.Set("period", lbPeriod)
		q.Set("limit", strconv.Itoa(lbLimit))
		c := internal.NewClient()
		res, err := c.Get("/api/leaderboard?" + q.Encode())
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var userCmd = &cobra.Command{
	Use:   "user <fid>",
	Short: "Look up a Farcaster user by FID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fid, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("FID must be a number, got %q", args[0])
		}
		c := internal.NewClient()
		res, err := c.Get(fmt.Sprintf("/api/user/%d", fid))
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "TipMaster service health and feature flags",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := internal.NewClient()
		res, err := c.Get("/api/status")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

func init() {
	leaderboardCmd.Flags().StringVar(&lbPeriod, "period", "week", "week or alltime")
	leaderboardCmd.Flags().IntVar(&lbLimit, "limit", 10, "number of results (max 25)")
}
