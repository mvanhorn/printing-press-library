package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const transactionsQuery = `query GetTransactionsList($offset: Int, $limit: Int, $filters: TransactionFilterInput, $orderBy: TransactionOrdering) {
  allTransactions(filters: $filters) {
    totalCount
    results(offset: $offset, limit: $limit, orderBy: $orderBy) {
      id
      amount
      pending
      date
      plaidName
      notes
      category { id name }
      merchant { id name }
      account { id displayName }
      tags { id name color order }
    }
  }
}`

const createTransactionMutation = `mutation Common_CreateTransactionMutation($input: CreateTransactionMutationInput!) {
  createTransaction(input: $input) {
    errors {
      ...PayloadErrorFields
    }
    transaction {
      id
      amount
      date
      notes
      merchant { id name }
      category { id name }
      account { id displayName }
    }
  }
}

fragment PayloadErrorFields on PayloadError {
  fieldErrors {
    field
    messages
  }
  message
  code
}`

const updateTransactionMutation = `mutation Web_TransactionDrawerUpdateTransaction($input: UpdateTransactionMutationInput!) {
  updateTransaction(input: $input) {
    transaction {
      id
      amount
      pending
      date
      hideFromReports
      needsReview
      notes
      merchant { id name }
      category { id name }
      tags { id name }
    }
    errors {
      ...PayloadErrorFields
    }
  }
}

fragment PayloadErrorFields on PayloadError {
  fieldErrors {
    field
    messages
  }
  message
  code
}`

const deleteTransactionMutation = `mutation Common_DeleteTransactionMutation($input: DeleteTransactionMutationInput!) {
  deleteTransaction(input: $input) {
    deleted
    errors {
      ...PayloadErrorFields
    }
  }
}

fragment PayloadErrorFields on PayloadError {
  fieldErrors {
    field
    messages
  }
  message
  code
}`

const setTransactionTagsMutation = `mutation Web_SetTransactionTags($input: SetTransactionTagsInput!) {
  setTransactionTags(input: $input) {
    errors {
      ...PayloadErrorFields
    }
    transaction {
      id
      tags { id name }
    }
  }
}

fragment PayloadErrorFields on PayloadError {
  fieldErrors {
    field
    messages
  }
  message
  code
}`

func newTransactionsCmd() *cobra.Command {
	var jsonOut bool
	var limit, offset, days int
	var startDate, endDate, search, tagID, accountID string
	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "List recent transactions with date, merchant, category, account, amount, and tags.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			filters := map[string]any{"search": search, "categories": []string{}, "accounts": []string{}, "tags": []string{}}
			if accountID != "" {
				filters["accounts"] = []string{accountID}
			}
			if tagID != "" {
				filters["tags"] = []string{tagID}
			}
			if days > 0 && startDate == "" && endDate == "" {
				end := time.Now()
				start := end.AddDate(0, 0, -days)
				startDate = start.Format("2006-01-02")
				endDate = end.Format("2006-01-02")
			}
			if startDate != "" || endDate != "" {
				if startDate == "" || endDate == "" {
					return fmt.Errorf("--start and --end must be provided together")
				}
				filters["startDate"] = startDate
				filters["endDate"] = endDate
			}
			vars := map[string]any{"offset": offset, "limit": limit, "orderBy": "date", "filters": filters}
			data, err := graphql("GetTransactionsList", transactionsQuery, vars)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			root := asMap(data["allTransactions"])
			rows := [][]string{}
			for _, v := range asSlice(root["results"]) {
				txn := asMap(v)
				merchant := str(field(txn, "merchant", "name"))
				if merchant == "" {
					merchant = str(txn["plaidName"])
				}
				tags := []string{}
				for _, tv := range asSlice(txn["tags"]) {
					tags = append(tags, str(asMap(tv)["name"]))
				}
				rows = append(rows, []string{
					str(txn["date"]),
					merchant,
					str(field(txn, "category", "name")),
					str(field(txn, "account", "displayName")),
					money(txn["amount"]),
					strings.Join(tags, ", "),
				})
			}
			return table([]string{"Date", "Merchant", "Category", "Account", "Amount", "Tags"}, rows)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum transactions to fetch")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	cmd.Flags().IntVar(&days, "days", 30, "Look back this many days when --start/--end are omitted")
	cmd.Flags().StringVar(&startDate, "start", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endDate, "end", "", "End date YYYY-MM-DD")
	cmd.Flags().StringVar(&search, "search", "", "Search text")
	cmd.Flags().StringVar(&tagID, "tag-id", "", "Filter by Monarch tag ID")
	cmd.Flags().StringVar(&accountID, "account-id", "", "Filter by Monarch account ID")
	cmd.AddCommand(newTransactionCreateCmd())
	cmd.AddCommand(newTransactionUpdateCmd())
	cmd.AddCommand(newTransactionDeleteCmd())
	cmd.AddCommand(newTransactionSetTagsCmd())
	return cmd
}

func newTransactionCreateCmd() *cobra.Command {
	var date, accountID, merchantName, categoryID, notes string
	var amount float64
	var updateBalance, yes, jsonOut bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a manual transaction.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" || accountID == "" || merchantName == "" || categoryID == "" {
				return fmt.Errorf("--date, --account-id, --merchant, and --category-id are required")
			}
			if amount == 0 {
				return fmt.Errorf("--amount is required and cannot be zero")
			}
			input := map[string]any{
				"date":                date,
				"accountId":           accountID,
				"amount":              amount,
				"merchantName":        merchantName,
				"categoryId":          categoryID,
				"notes":               notes,
				"shouldUpdateBalance": updateBalance,
			}
			vars := map[string]any{"input": input}
			if !yes {
				return printDryRun("Common_CreateTransactionMutation", vars)
			}
			data, err := graphql("Common_CreateTransactionMutation", createTransactionMutation, vars)
			if err != nil {
				return err
			}
			root := asMap(data["createTransaction"])
			if msg := payloadErrors(root["errors"]); msg != "" {
				return fmt.Errorf("create transaction failed: %s", msg)
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("created transaction %s\n", str(field(root, "transaction", "id")))
			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Transaction date YYYY-MM-DD")
	cmd.Flags().StringVar(&accountID, "account-id", "", "Monarch account ID")
	cmd.Flags().Float64Var(&amount, "amount", 0, "Transaction amount")
	cmd.Flags().StringVar(&merchantName, "merchant", "", "Merchant name")
	cmd.Flags().StringVar(&categoryID, "category-id", "", "Monarch category ID")
	cmd.Flags().StringVar(&notes, "notes", "", "Transaction notes")
	cmd.Flags().BoolVar(&updateBalance, "update-balance", false, "Update the manual account balance")
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply the write; omitted means dry-run")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON after applying")
	return cmd
}

func newTransactionUpdateCmd() *cobra.Command {
	var categoryID, merchantName, goalID, amountText, date, hideText, reviewText, notes string
	var yes, jsonOut bool
	cmd := &cobra.Command{
		Use:   "update <transaction-id>",
		Short: "Update a transaction by ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := map[string]any{"id": args[0]}
			if cmd.Flags().Changed("category-id") {
				input["category"] = categoryID
			}
			if cmd.Flags().Changed("merchant") {
				if strings.TrimSpace(merchantName) == "" {
					return fmt.Errorf("--merchant cannot be empty")
				}
				input["name"] = merchantName
			}
			if cmd.Flags().Changed("goal-id") {
				input["goalId"] = goalID
			}
			if cmd.Flags().Changed("amount") {
				amount, err := strconv.ParseFloat(amountText, 64)
				if err != nil {
					return fmt.Errorf("invalid --amount: %w", err)
				}
				input["amount"] = amount
			}
			if cmd.Flags().Changed("date") {
				input["date"] = date
			}
			if cmd.Flags().Changed("hide-from-reports") {
				v, err := strconv.ParseBool(hideText)
				if err != nil {
					return fmt.Errorf("--hide-from-reports must be true or false")
				}
				input["hideFromReports"] = v
			}
			if cmd.Flags().Changed("needs-review") {
				v, err := strconv.ParseBool(reviewText)
				if err != nil {
					return fmt.Errorf("--needs-review must be true or false")
				}
				input["needsReview"] = v
			}
			if cmd.Flags().Changed("notes") {
				input["notes"] = notes
			}
			if len(input) == 1 {
				return fmt.Errorf("at least one update flag is required")
			}
			vars := map[string]any{"input": input}
			if !yes {
				return printDryRun("Web_TransactionDrawerUpdateTransaction", vars)
			}
			data, err := graphql("Web_TransactionDrawerUpdateTransaction", updateTransactionMutation, vars)
			if err != nil {
				return err
			}
			root := asMap(data["updateTransaction"])
			if msg := payloadErrors(root["errors"]); msg != "" {
				return fmt.Errorf("update transaction failed: %s", msg)
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("updated transaction %s\n", str(field(root, "transaction", "id")))
			return nil
		},
	}
	cmd.Flags().StringVar(&categoryID, "category-id", "", "Set Monarch category ID")
	cmd.Flags().StringVar(&merchantName, "merchant", "", "Set merchant name")
	cmd.Flags().StringVar(&goalID, "goal-id", "", "Set Monarch goal ID; pass empty string to clear")
	cmd.Flags().StringVar(&amountText, "amount", "", "Set transaction amount")
	cmd.Flags().StringVar(&date, "date", "", "Set transaction date YYYY-MM-DD")
	cmd.Flags().StringVar(&hideText, "hide-from-reports", "", "Set hide-from-reports true or false")
	cmd.Flags().StringVar(&reviewText, "needs-review", "", "Set needs-review true or false")
	cmd.Flags().StringVar(&notes, "notes", "", "Set notes; pass empty string to clear")
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply the write; omitted means dry-run")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON after applying")
	return cmd
}

func newTransactionDeleteCmd() *cobra.Command {
	var yes, jsonOut bool
	cmd := &cobra.Command{
		Use:   "delete <transaction-id>",
		Short: "Delete a transaction by ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vars := map[string]any{"input": map[string]any{"transactionId": args[0]}}
			if !yes {
				return printDryRun("Common_DeleteTransactionMutation", vars)
			}
			data, err := graphql("Common_DeleteTransactionMutation", deleteTransactionMutation, vars)
			if err != nil {
				return err
			}
			root := asMap(data["deleteTransaction"])
			if msg := payloadErrors(root["errors"]); msg != "" {
				return fmt.Errorf("delete transaction failed: %s", msg)
			}
			if root["deleted"] != true {
				return fmt.Errorf("delete transaction failed")
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("deleted transaction %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply the write; omitted means dry-run")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON after applying")
	return cmd
}

func newTransactionSetTagsCmd() *cobra.Command {
	var tagIDs []string
	var clear, yes, jsonOut bool
	cmd := &cobra.Command{
		Use:   "set-tags <transaction-id>",
		Short: "Replace all tags on a transaction.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear && len(tagIDs) > 0 {
				return fmt.Errorf("--clear cannot be combined with --tag-id")
			}
			if !clear && len(tagIDs) == 0 {
				return fmt.Errorf("provide at least one --tag-id or pass --clear")
			}
			if clear {
				tagIDs = []string{}
			}
			vars := map[string]any{"input": map[string]any{"transactionId": args[0], "tagIds": tagIDs}}
			if !yes {
				return printDryRun("Web_SetTransactionTags", vars)
			}
			data, err := graphql("Web_SetTransactionTags", setTransactionTagsMutation, vars)
			if err != nil {
				return err
			}
			root := asMap(data["setTransactionTags"])
			if msg := payloadErrors(root["errors"]); msg != "" {
				return fmt.Errorf("set transaction tags failed: %s", msg)
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("updated tags for transaction %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&tagIDs, "tag-id", nil, "Monarch tag ID to set; repeat for multiple tags")
	cmd.Flags().BoolVar(&clear, "clear", false, "Remove all tags from the transaction")
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply the write; omitted means dry-run")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON after applying")
	return cmd
}
