package cli

import "github.com/spf13/cobra"

const accountsQuery = `query GetAccounts {
  accounts {
    id
    displayName
    currentBalance
    displayBalance
    isAsset
    isHidden
    type { display name }
    subtype { display name }
    institution { name }
    credential { institution { name } }
  }
}`

func newAccountsCmd() *cobra.Command {
	var jsonOut bool
	var limit int
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "List financial accounts with balances, account type, and institution.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := graphql("GetAccounts", accountsQuery, nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			rows := [][]string{}
			accounts := asSlice(data["accounts"])
			if limit > 0 && limit < len(accounts) {
				accounts = accounts[:limit]
			}
			for _, v := range accounts {
				acct := asMap(v)
				inst := str(field(acct, "institution", "name"))
				if inst == "" {
					inst = str(field(acct, "credential", "institution", "name"))
				}
				rows = append(rows, []string{
					str(acct["displayName"]),
					str(field(acct, "type", "display")),
					inst,
					money(acct["currentBalance"]),
				})
			}
			sortRows(rows, 0)
			return table([]string{"Name", "Type", "Institution", "Balance"}, rows)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum accounts to display; 0 means all accounts")
	return cmd
}
