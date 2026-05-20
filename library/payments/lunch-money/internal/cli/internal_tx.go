package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/lunch-money/internal/internalapi"

	"github.com/spf13/cobra"
)

func init() {
	internalSubcommandFactories = append(internalSubcommandFactories, newInternalTransactionsCmd)
}

// newInternalTransactionsCmd groups internal-API transaction mutations that
// the public v2 API doesn't expose: group, ungroup, split, unsplit.
func newInternalTransactionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "Transaction grouping/splitting via internal API",
	}
	cmd.AddCommand(newTxGroupCmd(flags))
	cmd.AddCommand(newTxUngroupCmd(flags))
	cmd.AddCommand(newTxBulkUngroupCmd(flags))
	cmd.AddCommand(newTxSplitCmd(flags))
	cmd.AddCommand(newTxBulkUnsplitCmd(flags))
	return cmd
}

func newTxGroupCmd(flags *rootFlags) *cobra.Command {
	var date, payee, notes string
	var categoryID int64
	var tagIDs []int64
	cmd := &cobra.Command{
		Use:     "group <tx_id>...",
		Short:   "Group N transactions into a parent (POST /v2/transactions/group)",
		Example: `  internal transactions group 12345 12345 --date 2026-05-07 --payee "Apple subscriptions" --category-id 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("need at least 2 transaction ids to group")
			}
			if date == "" {
				return fmt.Errorf("--date is required")
			}
			if payee == "" {
				return fmt.Errorf("--payee is required")
			}
			ids := make([]int64, 0, len(args))
			for _, a := range args {
				n, err := strconv.ParseInt(a, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid tx id %q: %w", a, err)
				}
				ids = append(ids, n)
			}
			req := internalapi.TransactionGroupCreate{
				IDs:   ids,
				Date:  date,
				Payee: payee,
			}
			if categoryID != 0 {
				req.CategoryID = &categoryID
			}
			if notes != "" {
				req.Notes = &notes
			}
			if len(tagIDs) > 0 {
				req.TagIDs = tagIDs
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "POST", "/v2/transactions/group", req)
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.CreateTransactionGroup(req)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Group transaction date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&payee, "payee", "", "Group payee name (required)")
	cmd.Flags().StringVar(&notes, "notes", "", "Group notes")
	cmd.Flags().Int64Var(&categoryID, "category-id", 0, "Category for the group")
	cmd.Flags().Int64SliceVar(&tagIDs, "tag-id", nil, "Tag id (repeatable)")
	return cmd
}

func newTxUngroupCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "ungroup <group_id>",
		Short: "Dissolve a transaction group (DELETE /v2/transactions/group/{id})",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: ungroup <group_id>")
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "DELETE", fmt.Sprintf("/v2/transactions/group/%d", id), nil)
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			if err := c.UngroupTransactionGroup(id); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok: group dissolved")
			return nil
		},
	}
}

func newTxBulkUngroupCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "bulk-ungroup <tx_id>...",
		Short: "Bulk-ungroup by child transaction id (PUT /transactions/group/bulk_ungroup)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("need at least one transaction id")
			}
			ids := make([]int64, 0, len(args))
			for _, a := range args {
				n, err := strconv.ParseInt(a, 10, 64)
				if err != nil {
					return err
				}
				ids = append(ids, n)
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "PUT", "/transactions/group/bulk_ungroup", map[string]any{"transaction_ids": ids})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.BulkUngroupTransactions(ids)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
}

func newTxSplitCmd(flags *rootFlags) *cobra.Command {
	var childSpec []string
	cmd := &cobra.Command{
		Use:   "split <parent_id>",
		Short: "Split one transaction into N children (POST /v2/transactions/split/{id})",
		Long: `Each --child takes a colon-separated spec: "amount:payee:category_id[:notes]"
Use --child-pct for percentage splits: "pct:payee:category_id[:notes]"`,
		Example: `  internal transactions split 12345 --child "11.00:Half A:12345" --child "11.00:Half B:12345"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: split <parent_id> --child")
			}
			parentID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			if len(childSpec) < 2 {
				return fmt.Errorf("need at least 2 --child entries (you can't split into 1)")
			}
			children := make([]internalapi.SplitChild, 0, len(childSpec))
			for i, spec := range childSpec {
				parts := strings.SplitN(spec, ":", 4)
				if len(parts) < 3 {
					return fmt.Errorf("--child #%d malformed; expected `amount:payee:category_id[:notes]`", i+1)
				}
				amt := parts[0]
				payee := parts[1]
				catID, err := strconv.ParseInt(parts[2], 10, 64)
				if err != nil {
					return fmt.Errorf("--child #%d: invalid category_id %q", i+1, parts[2])
				}
				child := internalapi.SplitChild{
					Payee:      payee,
					Amount:     &amt,
					CategoryID: &catID,
				}
				if len(parts) >= 4 && parts[3] != "" {
					n := parts[3]
					child.Notes = &n
				}
				children = append(children, child)
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "POST", fmt.Sprintf("/v2/transactions/split/%d", parentID), map[string]any{"child_transactions": children})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.SplitTransaction(parentID, children)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().StringArrayVar(&childSpec, "child", nil, "Child spec `amount:payee:category_id[:notes]` (repeatable)")
	return cmd
}

func newTxBulkUnsplitCmd(flags *rootFlags) *cobra.Command {
	var removeParents bool
	cmd := &cobra.Command{
		Use:   "unsplit <parent_id>...",
		Short: "Reverse a split (PUT /transactions/split/bulk_unsplit)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("need at least one parent_id")
			}
			ids := make([]int64, 0, len(args))
			for _, a := range args {
				n, err := strconv.ParseInt(a, 10, 64)
				if err != nil {
					return err
				}
				ids = append(ids, n)
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "PUT", "/transactions/split/bulk_unsplit", map[string]any{
					"transaction_ids": ids,
					"remove_parents":  removeParents,
				})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.BulkUnsplitTransactions(ids, removeParents)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().BoolVar(&removeParents, "remove-parents", false, "Also delete the parent transaction(s)")
	return cmd
}
