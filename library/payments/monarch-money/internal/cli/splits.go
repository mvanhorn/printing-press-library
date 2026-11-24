package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const categoriesQuery = `query GetCategories {
  categories {
    id
    order
    name
    isSystemCategory
    isDisabled
    group { id name type }
  }
}`

const transactionDetailQuery = `query GetTransactionDrawer($id: UUID!, $redirectPosted: Boolean) {
  getTransaction(id: $id, redirectPosted: $redirectPosted) {
    id
    amount
    pending
    date
    originalDate
    hideFromReports
    needsReview
    plaidName
    notes
    hasSplitTransactions
    isSplitTransaction
    isManual
    splitTransactions {
      id
      amount
      notes
      merchant { id name }
      category { id name }
    }
    originalTransaction { id date amount merchant { id name } }
    attachments { id filename extension sizeBytes }
    account { id displayName }
    category { id name }
    goal { id }
    merchant { id name }
    tags { id name }
  }
}`

const updateTransactionSplitMutation = `mutation Common_SplitTransactionMutation($input: UpdateTransactionSplitMutationInput!) {
  updateTransactionSplit(input: $input) {
    errors {
      ...PayloadErrorFields
    }
    transaction {
      id
      amount
      hasSplitTransactions
      splitTransactions {
        id
        amount
        notes
        merchant { id name }
        category { id name }
      }
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

type splitLine struct {
	MerchantName string      `json:"merchantName"`
	Amount       json.Number `json:"amount"`
	CategoryID   string      `json:"categoryId"`
}

func newCategoriesCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "List transaction categories with group and ID.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := graphql("GetCategories", categoriesQuery, nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			rows := [][]string{}
			for _, v := range asSlice(data["categories"]) {
				c := asMap(v)
				if c["isDisabled"] == true {
					continue
				}
				rows = append(rows, []string{str(field(c, "group", "name")), str(c["name"]), str(c["id"])})
			}
			sortRows(rows, 0)
			return table([]string{"Group", "Category", "ID"}, rows)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON")
	return cmd
}

func fetchTransaction(id string) (map[string]any, error) {
	data, err := graphql("GetTransactionDrawer", transactionDetailQuery, map[string]any{"id": id, "redirectPosted": true})
	if err != nil {
		return nil, err
	}
	txn := asMap(data["getTransaction"])
	if txn == nil {
		return nil, fmt.Errorf("transaction %s not found", id)
	}
	return txn, nil
}

func newTransactionGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <transaction-id>",
		Short: "Show one transaction with splits, tags, notes, goal, and attachments as JSON.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			txn, err := fetchTransaction(args[0])
			if err != nil {
				return err
			}
			return printJSON(txn)
		},
	}
}

func toCents(text string) (int64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%q is not an amount", text)
	}
	if math.Abs(f) >= 1e12 {
		return 0, fmt.Errorf("amount %q is too large", text)
	}
	cents := math.Round(f * 100)
	if math.Abs(f*100-cents) > 1e-6 {
		return 0, fmt.Errorf("amount %q has more than two decimal places", text)
	}
	return int64(cents), nil
}

func centsString(c int64) string {
	sign := ""
	if c < 0 {
		sign = "-"
		c = -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// parseSplits strictly decodes split lines and checks they sum to expectTotal to the cent.
func parseSplits(raw []byte, expectTotal int64) ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	var lines []splitLine
	if err := dec.Decode(&lines); err != nil {
		return nil, fmt.Errorf("the split file is not valid (%w); it should be a list like [{\"merchantName\": ..., \"amount\": ..., \"categoryId\": ...}]", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("the split file has extra text after the list")
	}
	if len(lines) < 2 {
		return nil, fmt.Errorf("a split needs at least two lines; to undo a split, use --clear")
	}
	var sum int64
	out := make([]map[string]any, 0, len(lines))
	for i, l := range lines {
		if strings.TrimSpace(l.MerchantName) == "" || strings.TrimSpace(l.CategoryID) == "" {
			return nil, fmt.Errorf("line %d needs a merchantName and a categoryId", i+1)
		}
		c, err := toCents(l.Amount.String())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if c == 0 || (c < 0) != (expectTotal < 0) {
			return nil, fmt.Errorf("line %d: %s must not be zero and must have the same sign as the total, %s", i+1, centsString(c), centsString(expectTotal))
		}
		sum += c
		if (expectTotal < 0 && sum < expectTotal) || (expectTotal > 0 && sum > expectTotal) {
			return nil, fmt.Errorf("the lines add up to more than --expect-total %s by line %d", centsString(expectTotal), i+1)
		}
		out = append(out, map[string]any{"merchantName": l.MerchantName, "amount": float64(c) / 100, "categoryId": l.CategoryID})
	}
	if sum != expectTotal {
		return nil, fmt.Errorf("the lines add up to %s, but --expect-total is %s (off by %s)", centsString(sum), centsString(expectTotal), centsString(expectTotal-sum))
	}
	return out, nil
}

func newTransactionSplitCmd() *cobra.Command {
	var file, expectText string
	var clear, replace, yes bool
	cmd := &cobra.Command{
		Use:   "split <transaction-id>",
		Short: "Split a transaction across categories, or undo a split.",
		Long: `Split one transaction into lines, each with its own category.
Put the lines in a JSON file:

  [
    {"merchantName": "Costco", "amount": -60.00, "categoryId": "CATEGORY_ID"},
    {"merchantName": "Costco", "amount": -22.17, "categoryId": "CATEGORY_ID"}
  ]

Find category IDs with "categories". See the transaction with "transactions get".

Give each line the same sign as the transaction: negative for a purchase,
positive for a refund. The lines must add up to --expect-total to the cent.

Nothing changes until you add --yes. Without it, the command only shows what
it would send.

With --yes, the command checks the transaction again and stops if:
  - its amount is not --expect-total
  - Monarch returns a different transaction, which happens when a pending
    charge posts
  - it is one line of another split
  - it is already split (add --replace to overwrite)

The output shows the transaction before and after. To undo, split it again
with the old lines, or use --clear.`,
		Example: `  monarch-money-pp-cli transactions split 123456 --file splits.json --expect-total -82.17
  monarch-money-pp-cli transactions split 123456 --file splits.json --expect-total -82.17 --yes
  monarch-money-pp-cli transactions split 123456 --clear --expect-total -82.17 --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear == (file != "") {
				return fmt.Errorf("choose one: --file to split, or --clear to undo a split")
			}
			if !cmd.Flags().Changed("expect-total") {
				return fmt.Errorf("--expect-total is required: give the transaction's amount, for example -142.37")
			}
			expect, err := toCents(expectText)
			if err != nil || expect == 0 {
				return fmt.Errorf("--expect-total must be an amount like -142.37, and not zero")
			}
			splits := []map[string]any{}
			if !clear {
				raw, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				if splits, err = parseSplits(raw, expect); err != nil {
					return err
				}
			}
			vars := map[string]any{"input": map[string]any{"transactionId": args[0], "splitData": splits}}
			if !yes {
				return printDryRun("Common_SplitTransactionMutation", vars)
			}
			before, err := fetchTransaction(args[0])
			if err != nil {
				return err
			}
			actual, err := toCents(str(before["amount"]))
			if err != nil {
				return fmt.Errorf("could not read the transaction's amount: %w", err)
			}
			switch {
			case str(before["id"]) != args[0]:
				return fmt.Errorf("stopped: Monarch returned transaction %s instead of %s, usually because a pending charge posted; use %s; nothing was changed", str(before["id"]), args[0], str(before["id"]))
			case actual != expect:
				return fmt.Errorf("stopped: the transaction's amount is %s, not %s; nothing was changed", centsString(actual), centsString(expect))
			case before["isSplitTransaction"] == true:
				return fmt.Errorf("stopped: %s is one line of a split; use the original transaction's ID (see originalTransaction in transactions get); nothing was changed", args[0])
			case clear && before["hasSplitTransactions"] != true:
				return fmt.Errorf("stopped: this transaction is not split, so there is nothing to undo")
			case !clear && before["hasSplitTransactions"] == true && !replace:
				return fmt.Errorf("stopped: this transaction is already split; check it with transactions get, then add --replace to overwrite it; nothing was changed")
			}
			data, err := graphql("Common_SplitTransactionMutation", updateTransactionSplitMutation, vars)
			if err != nil {
				return err
			}
			root := asMap(data["updateTransactionSplit"])
			if msg := payloadErrors(root["errors"]); msg != "" {
				return fmt.Errorf("Monarch rejected the split: %s", msg)
			}
			return printJSON(map[string]any{"before": before, "after": root["transaction"]})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON file with the split lines")
	cmd.Flags().BoolVar(&clear, "clear", false, "Undo the split so the transaction is one line again")
	cmd.Flags().StringVar(&expectText, "expect-total", "", "The transaction's amount, for example -82.17 (required)")
	cmd.Flags().BoolVar(&replace, "replace", false, "Overwrite a split that already exists")
	cmd.Flags().BoolVar(&yes, "yes", false, "Make the change (without it, nothing changes)")
	return cmd
}
