package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/lunch-money/internal/internalapi"

	"github.com/spf13/cobra"
)

func init() {
	internalSubcommandFactories = append(internalSubcommandFactories,
		newTrendsCmd,
		newStatsCmd,
		newCalendarCmd,
		newReferralCmd,
		newInternalPlaidAccountsCmd,
		newImportConfigsCmd,
		newTagColorsCmd,
	)
	// Extend api-tokens with a create subcommand.
}

func newTrendsCmd(_ *rootFlags) *cobra.Command {
	var start, end, groupBy string
	var includeRecurring, includeExcl, includePending bool
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Spending/income trends aggregations (GET /trends)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if start == "" {
				start = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
			}
			if end == "" {
				end = time.Now().Format("2006-01-02")
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.Trends(internalapi.TrendsOptions{
				StartDate:                start,
				EndDate:                  end,
				IncludeRecurring:         includeRecurring,
				IncludeExcludeFromTotals: includeExcl,
				IncludePending:           includePending,
				GroupBy:                  groupBy,
			})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().StringVar(&start, "start-date", "", "YYYY-MM-DD")
	cmd.Flags().StringVar(&end, "end-date", "", "YYYY-MM-DD")
	cmd.Flags().BoolVar(&includeRecurring, "include-recurring", true, "")
	cmd.Flags().BoolVar(&includeExcl, "include-exclude-from-totals", true, "")
	cmd.Flags().BoolVar(&includePending, "include-pending", false, "")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "category | payee | tag | asset | type")
	return cmd
}

func newStatsCmd(_ *rootFlags) *cobra.Command {
	var start, end string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Top transactions by price within window (GET /stats)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if start == "" {
				start = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
			}
			if end == "" {
				end = time.Now().Format("2006-01-02")
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.Stats(internalapi.TrendsOptions{
				StartDate:                start,
				EndDate:                  end,
				IncludeRecurring:         true,
				IncludeExcludeFromTotals: true,
			})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().StringVar(&start, "start-date", "", "YYYY-MM-DD")
	cmd.Flags().StringVar(&end, "end-date", "", "YYYY-MM-DD")
	return cmd
}

func newCalendarCmd(_ *rootFlags) *cobra.Command {
	var start, end string
	var includeRecurring bool
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Daily transaction grid for a date range (GET /calendar)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if start == "" {
				start = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
			}
			if end == "" {
				end = time.Now().Format("2006-01-02")
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.Calendar(start, end, includeRecurring)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().StringVar(&start, "start-date", "", "YYYY-MM-DD")
	cmd.Flags().StringVar(&end, "end-date", "", "YYYY-MM-DD")
	cmd.Flags().BoolVar(&includeRecurring, "include-recurring", false, "")
	return cmd
}

func newReferralCmd(_ *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "referral",
		Short: "Show your referral token + redemption count",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			r, err := c.Referral()
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(r)
		},
	}
}

func newInternalPlaidAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plaid-accounts",
		Short: "Plaid-linked accounts (via /v2/plaid_accounts)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List Plaid accounts with item_id, display_name, last sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.ListPlaidAccounts()
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <id>",
		Short: "Get one Plaid account by id",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: plaid-accounts get <id>")
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.GetPlaidAccount(id)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	})
	var fetchStart, fetchEnd string
	fetchCmd := &cobra.Command{
		Use:   "fetch [id]",
		Short: "Queue a Plaid fetch for one account or all eligible accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("usage: plaid-accounts fetch [id]")
			}
			opts := internalapi.PlaidFetchOptions{StartDate: fetchStart, EndDate: fetchEnd}
			if len(args) == 1 {
				id, err := strconv.ParseInt(args[0], 10, 64)
				if err != nil {
					return err
				}
				opts.ID = id
			}
			if dryRunOK(flags) {
				body := map[string]any{}
				if opts.ID != 0 {
					body["id"] = opts.ID
				}
				if opts.StartDate != "" {
					body["start_date"] = opts.StartDate
				}
				if opts.EndDate != "" {
					body["end_date"] = opts.EndDate
				}
				return writeInternalDryRun(cmd, "POST", "/v2/plaid_accounts/fetch", body)
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			status, err := c.TriggerPlaidFetch(opts)
			if err != nil {
				return err
			}
			// PATCH: Expose the captured internal Plaid fetch queue endpoint.
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"status":   status,
				"accepted": status >= 200 && status < 300,
			})
		},
	}
	fetchCmd.Flags().StringVar(&fetchStart, "start-date", "", "YYYY-MM-DD")
	fetchCmd.Flags().StringVar(&fetchEnd, "end-date", "", "YYYY-MM-DD")
	cmd.AddCommand(fetchCmd)
	return cmd
}

func newImportConfigsCmd(_ *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-configs",
		Short: "Saved CSV import column mappings",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved import configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			// PATCH: Surface captured CSV import preset listing without exposing commit flows.
			res, err := c.ListImportConfigs()
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	})
	return cmd
}

func newTagColorsCmd(_ *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tag-colors",
		Short: "Get tag color settings (GET /tags/colors)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.TagColors()
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
}
