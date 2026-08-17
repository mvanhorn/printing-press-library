// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// Delete an expense. Expensify's /DeleteMoneyRequest needs the IOU reportID and
// reportActionID, not just the transactionID; when only --transaction-id is
// given, this command resolves those refs automatically before deleting.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newExpenseDeleteCmd(flags *rootFlags) *cobra.Command {
	var bodyTransactionID string
	var bodyReportID string
	var bodyReportActionID string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an expense",
		Example: "  expensify-pp-cli expense delete --transaction-id <id>\n" +
			"  expensify-pp-cli expense delete --transaction-id <id> --report-id <rid> --report-action-id <aid>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("transaction-id") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "transaction-id")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/DeleteMoneyRequest"
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				body = map[string]any{}
				if bodyTransactionID != "" {
					body["transactionID"] = bodyTransactionID
				}
				if bodyReportID != "" {
					body["reportID"] = bodyReportID
				}
				if bodyReportActionID != "" {
					body["reportActionID"] = bodyReportActionID
				}
				// /DeleteMoneyRequest is a no-op (returns jsonCode 200 with no changes)
				// unless it gets the IOU reportID + reportActionID. Resolve them from
				// the transactionID when the caller didn't supply them.
				if !flags.dryRun && bodyTransactionID != "" && (bodyReportID == "" || bodyReportActionID == "") {
					rid, aid, rerr := resolveExpenseDeleteRefs(cmd.Context(), c, bodyTransactionID)
					if rerr != nil {
						return fmt.Errorf("resolving report refs for transaction %s: %w (pass --report-id and --report-action-id to skip lookup)", bodyTransactionID, rerr)
					}
					if bodyReportID == "" {
						body["reportID"] = rid
					}
					if bodyReportActionID == "" {
						body["reportActionID"] = aid
					}
				}
			}
			data, statusCode, err := c.Post(cmd.Context(), path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							return nil
						}
					}
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if flags.quiet {
					return nil
				}
				filtered := data
				if flags.compact {
					filtered = compactFields(filtered)
				}
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "expense",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyTransactionID, "transaction-id", "", "Expense transaction ID")
	cmd.Flags().StringVar(&bodyReportID, "report-id", "", "IOU report ID (auto-resolved from the transaction if omitted)")
	cmd.Flags().StringVar(&bodyReportActionID, "report-action-id", "", "IOU report action ID (auto-resolved from the transaction if omitted)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}

// resolveExpenseDeleteRefs looks up the IOU reportID and reportActionID for a
// transaction so /DeleteMoneyRequest actually deletes it. It reads the
// transaction (for its reportID), then the report's actions (for the IOU action
// that references the transaction).
func resolveExpenseDeleteRefs(ctx context.Context, c apiPoster, transactionID string) (reportID, reportActionID string, err error) {
	// 1) transaction -> reportID
	txData, _, terr := c.Post(ctx, "/Get", map[string]any{
		"returnValueList":   "transactionList",
		"transactionIDList": transactionID,
	})
	if terr != nil {
		return "", "", terr
	}
	reportID = findStringForTransaction(txData, transactionID, "reportID")
	if reportID == "" {
		return "", "", fmt.Errorf("could not find reportID for transaction (is the transaction ID correct?)")
	}
	// 2) report -> IOU reportActionID for this transaction
	rData, _, rerr := c.Post(ctx, "/OpenReport", map[string]any{"reportID": reportID})
	if rerr != nil {
		return "", "", rerr
	}
	reportActionID = findReportActionIDForTransaction(rData, transactionID)
	if reportActionID == "" {
		return "", "", fmt.Errorf("could not find the IOU report action for transaction %s", transactionID)
	}
	return reportID, reportActionID, nil
}

// apiPoster is the subset of the client used here, kept small for testability.
type apiPoster interface {
	Post(ctx context.Context, path string, body any) (json.RawMessage, int, error)
}

// findStringForTransaction walks a Get(transactionList) response and returns the
// value of `key` on the object whose transactionID matches.
func findStringForTransaction(data json.RawMessage, transactionID, key string) string {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return ""
	}
	var found string
	var walk func(any)
	walk = func(n any) {
		if found != "" {
			return
		}
		switch v := n.(type) {
		case map[string]any:
			if idMatches(v["transactionID"], transactionID) {
				if s := asString(v[key]); s != "" {
					found = s
					return
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(root)
	return found
}

// findReportActionIDForTransaction walks an OpenReport response and returns the
// reportActionID of the IOU action that references the given transaction.
func findReportActionIDForTransaction(data json.RawMessage, transactionID string) string {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return ""
	}
	var found string
	var walk func(any)
	walk = func(n any) {
		if found != "" {
			return
		}
		switch v := n.(type) {
		case map[string]any:
			if om, ok := v["originalMessage"].(map[string]any); ok {
				if idMatches(om["IOUTransactionID"], transactionID) || idMatches(om["transactionID"], transactionID) {
					if s := asString(v["reportActionID"]); s != "" {
						found = s
						return
					}
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(root)
	return found
}

// idMatches compares an Expensify ID field (which may be a string or a number)
// against a string ID.
func idMatches(v any, id string) bool {
	return asString(v) == id && id != ""
}

// asString renders a JSON string or number ID as a string.
func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	case json.Number:
		return t.String()
	}
	return ""
}
