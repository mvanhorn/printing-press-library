package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/amazon"
)

type doctorResult struct {
	Profile         string `json:"profile"`
	CookiesLoaded   bool   `json:"cookies_loaded"`
	HasMarker       bool   `json:"has_marker"`
	HistoryOrders   int    `json:"history_orders"`
	HistoryItems    int    `json:"history_items"`
	LastPurchased   string `json:"last_purchased,omitempty"`
	AmazonReached   bool   `json:"amazon_reached"`
	DetectedAccount string `json:"detected_account,omitempty"`
	Error           string `json:"error,omitempty"`
}

func newDoctorCmd() *cobra.Command {
	var skipLive bool
	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       "Verify auth, local store, and amazon.com reachability",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			res := doctorResult{Profile: app.Profile.Name}
			if err := app.RequireSession(); err != nil {
				res.Error = err.Error()
			} else {
				res.CookiesLoaded = true
				res.HasMarker = app.Session.HasMarker()
			}
			orders, _ := app.Store.CountOrders()
			items, lastPurchased, _ := app.Store.CountPurchasedItems()
			res.HistoryOrders = orders
			res.HistoryItems = items
			if !lastPurchased.IsZero() {
				res.LastPurchased = lastPurchased.Format(time.RFC3339)
			}
			if !skipLive && app.Session != nil {
				ctx, cancel := contextWithTimeout(app.Ctx, 15*time.Second)
				defer cancel()
				client, err := amazon.New(app.Profile, app.Session)
				if err == nil {
					name, err := client.Ping(ctx)
					if err != nil {
						if errors.Is(err, amazon.ErrAuthExpired) {
							res.Error = "amazon.com responded with sign-in page (session expired)"
						} else if errors.Is(err, amazon.ErrRobotCheck) {
							res.Error = "amazon.com served a robot check; clear it in your browser and retry"
						} else {
							res.Error = err.Error()
						}
					} else {
						res.AmazonReached = true
						res.DetectedAccount = name
					}
				}
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "profile:         %s\n", res.Profile)
			fmt.Fprintf(cmd.OutOrStdout(), "cookies_loaded:  %v\n", res.CookiesLoaded)
			fmt.Fprintf(cmd.OutOrStdout(), "has_marker:      %v\n", res.HasMarker)
			fmt.Fprintf(cmd.OutOrStdout(), "history_orders:  %d\n", res.HistoryOrders)
			fmt.Fprintf(cmd.OutOrStdout(), "history_items:   %d\n", res.HistoryItems)
			if res.LastPurchased != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "last_purchased:  %s\n", res.LastPurchased)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "amazon_reached:  %v\n", res.AmazonReached)
			if res.DetectedAccount != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "account:         %s\n", res.DetectedAccount)
			}
			if res.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "error:           %s\n", res.Error)
				return coded(ExitConflict, "doctor reported issues; see above")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipLive, "skip-live", false, "Don't hit amazon.com; only check local state")
	return cmd
}
