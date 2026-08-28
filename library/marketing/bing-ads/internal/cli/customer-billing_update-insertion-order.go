// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCustomerBillingUpdateInsertionOrderCmd(flags *rootFlags) *cobra.Command {
	var bodyInsertionOrderAccountId string
	var bodyInsertionOrderAccountNumber string
	var bodyInsertionOrderBookingCountryCode string
	var bodyInsertionOrderBudgetRemaining float64
	var bodyInsertionOrderBudgetRemainingPercent float64
	var bodyInsertionOrderBudgetSpent float64
	var bodyInsertionOrderBudgetSpentPercent float64
	var bodyInsertionOrderComment string
	var bodyInsertionOrderEndDate string
	var bodyInsertionOrderId string
	var bodyInsertionOrderIsEndless bool
	var bodyInsertionOrderIsInSeries bool
	var bodyInsertionOrderIsUnlimited bool
	var bodyInsertionOrderLastModifiedByUserId string
	var bodyInsertionOrderLastModifiedTime string
	var bodyInsertionOrderName string
	var bodyInsertionOrderNotificationThreshold float64
	var bodyInsertionOrderPendingChangesChangeStatus string
	var bodyInsertionOrderPendingChangesComment string
	var bodyInsertionOrderPendingChangesEndDate string
	var bodyInsertionOrderPendingChangesModifiedDateTime string
	var bodyInsertionOrderPendingChangesName string
	var bodyInsertionOrderPendingChangesNotificationThreshold float64
	var bodyInsertionOrderPendingChangesPurchaseOrder string
	var bodyInsertionOrderPendingChangesReferenceId string
	var bodyInsertionOrderPendingChangesRequestedByUserId int
	var bodyInsertionOrderPendingChangesSpendCapAmount float64
	var bodyInsertionOrderPendingChangesStartDate string
	var bodyInsertionOrderPurchaseOrder string
	var bodyInsertionOrderReferenceId string
	var bodyInsertionOrderSeriesFrequencyType string
	var bodyInsertionOrderSeriesName string
	var bodyInsertionOrderSpendCapAmount float64
	var bodyInsertionOrderStartDate string
	var bodyInsertionOrderStatus string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update-insertion-order",
		Short:       "update_insertion_order",
		Example:     "  bing-ads-pp-cli customer-billing update-insertion-order",
		Annotations: map[string]string{"pp:endpoint": "customer-billing.update-insertion-order", "pp:method": "PUT", "pp:path": "/CustomerBilling/v13/InsertionOrder"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CustomerBilling/v13/InsertionOrder"
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			var body any
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
				bodyMap := map[string]any{}
				body = map[string]any{"InsertionOrder": bodyMap}
				if cmd.Flags().Changed("insertion-order-account-id") || bodyInsertionOrderAccountId != "" {
					bodyMap["AccountId"] = bodyInsertionOrderAccountId
				}
				if cmd.Flags().Changed("insertion-order-account-number") || bodyInsertionOrderAccountNumber != "" {
					bodyMap["AccountNumber"] = bodyInsertionOrderAccountNumber
				}
				if cmd.Flags().Changed("insertion-order-booking-country-code") || bodyInsertionOrderBookingCountryCode != "" {
					bodyMap["BookingCountryCode"] = bodyInsertionOrderBookingCountryCode
				}
				if cmd.Flags().Changed("insertion-order-budget-remaining") || bodyInsertionOrderBudgetRemaining != 0.0 {
					bodyMap["BudgetRemaining"] = bodyInsertionOrderBudgetRemaining
				}
				if cmd.Flags().Changed("insertion-order-budget-remaining-percent") || bodyInsertionOrderBudgetRemainingPercent != 0.0 {
					bodyMap["BudgetRemainingPercent"] = bodyInsertionOrderBudgetRemainingPercent
				}
				if cmd.Flags().Changed("insertion-order-budget-spent") || bodyInsertionOrderBudgetSpent != 0.0 {
					bodyMap["BudgetSpent"] = bodyInsertionOrderBudgetSpent
				}
				if cmd.Flags().Changed("insertion-order-budget-spent-percent") || bodyInsertionOrderBudgetSpentPercent != 0.0 {
					bodyMap["BudgetSpentPercent"] = bodyInsertionOrderBudgetSpentPercent
				}
				if cmd.Flags().Changed("insertion-order-comment") || bodyInsertionOrderComment != "" {
					bodyMap["Comment"] = bodyInsertionOrderComment
				}
				if cmd.Flags().Changed("insertion-order-end-date") || bodyInsertionOrderEndDate != "" {
					bodyMap["EndDate"] = bodyInsertionOrderEndDate
				}
				if cmd.Flags().Changed("insertion-order-id") || bodyInsertionOrderId != "" {
					bodyMap["Id"] = bodyInsertionOrderId
				}
				if cmd.Flags().Changed("insertion-order-is-endless") {
					bodyMap["IsEndless"] = bodyInsertionOrderIsEndless
				}
				if cmd.Flags().Changed("insertion-order-is-in-series") {
					bodyMap["IsInSeries"] = bodyInsertionOrderIsInSeries
				}
				if cmd.Flags().Changed("insertion-order-is-unlimited") {
					bodyMap["IsUnlimited"] = bodyInsertionOrderIsUnlimited
				}
				if cmd.Flags().Changed("insertion-order-last-modified-by-user-id") || bodyInsertionOrderLastModifiedByUserId != "" {
					bodyMap["LastModifiedByUserId"] = bodyInsertionOrderLastModifiedByUserId
				}
				if cmd.Flags().Changed("insertion-order-last-modified-time") || bodyInsertionOrderLastModifiedTime != "" {
					bodyMap["LastModifiedTime"] = bodyInsertionOrderLastModifiedTime
				}
				if cmd.Flags().Changed("insertion-order-name") || bodyInsertionOrderName != "" {
					bodyMap["Name"] = bodyInsertionOrderName
				}
				if cmd.Flags().Changed("insertion-order-notification-threshold") || bodyInsertionOrderNotificationThreshold != 0.0 {
					bodyMap["NotificationThreshold"] = bodyInsertionOrderNotificationThreshold
				}
				{
					nestedInsertionOrderPendingChanges := map[string]any{}
					if cmd.Flags().Changed("insertion-order-pending-changes-change-status") || bodyInsertionOrderPendingChangesChangeStatus != "" {
						nestedInsertionOrderPendingChanges["ChangeStatus"] = bodyInsertionOrderPendingChangesChangeStatus
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-comment") || bodyInsertionOrderPendingChangesComment != "" {
						nestedInsertionOrderPendingChanges["Comment"] = bodyInsertionOrderPendingChangesComment
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-end-date") || bodyInsertionOrderPendingChangesEndDate != "" {
						nestedInsertionOrderPendingChanges["EndDate"] = bodyInsertionOrderPendingChangesEndDate
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-modified-date-time") || bodyInsertionOrderPendingChangesModifiedDateTime != "" {
						nestedInsertionOrderPendingChanges["ModifiedDateTime"] = bodyInsertionOrderPendingChangesModifiedDateTime
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-name") || bodyInsertionOrderPendingChangesName != "" {
						nestedInsertionOrderPendingChanges["Name"] = bodyInsertionOrderPendingChangesName
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-notification-threshold") || bodyInsertionOrderPendingChangesNotificationThreshold != 0.0 {
						nestedInsertionOrderPendingChanges["NotificationThreshold"] = bodyInsertionOrderPendingChangesNotificationThreshold
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-purchase-order") || bodyInsertionOrderPendingChangesPurchaseOrder != "" {
						nestedInsertionOrderPendingChanges["PurchaseOrder"] = bodyInsertionOrderPendingChangesPurchaseOrder
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-reference-id") || bodyInsertionOrderPendingChangesReferenceId != "" {
						nestedInsertionOrderPendingChanges["ReferenceId"] = bodyInsertionOrderPendingChangesReferenceId
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-requested-by-user-id") || bodyInsertionOrderPendingChangesRequestedByUserId != 0 {
						nestedInsertionOrderPendingChanges["RequestedByUserId"] = bodyInsertionOrderPendingChangesRequestedByUserId
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-spend-cap-amount") || bodyInsertionOrderPendingChangesSpendCapAmount != 0.0 {
						nestedInsertionOrderPendingChanges["SpendCapAmount"] = bodyInsertionOrderPendingChangesSpendCapAmount
					}
					if cmd.Flags().Changed("insertion-order-pending-changes-start-date") || bodyInsertionOrderPendingChangesStartDate != "" {
						nestedInsertionOrderPendingChanges["StartDate"] = bodyInsertionOrderPendingChangesStartDate
					}
					if len(nestedInsertionOrderPendingChanges) > 0 {
						bodyMap["PendingChanges"] = nestedInsertionOrderPendingChanges
					}
				}
				if cmd.Flags().Changed("insertion-order-purchase-order") || bodyInsertionOrderPurchaseOrder != "" {
					bodyMap["PurchaseOrder"] = bodyInsertionOrderPurchaseOrder
				}
				if cmd.Flags().Changed("insertion-order-reference-id") || bodyInsertionOrderReferenceId != "" {
					bodyMap["ReferenceId"] = bodyInsertionOrderReferenceId
				}
				if cmd.Flags().Changed("insertion-order-series-frequency-type") || bodyInsertionOrderSeriesFrequencyType != "" {
					bodyMap["SeriesFrequencyType"] = bodyInsertionOrderSeriesFrequencyType
				}
				if cmd.Flags().Changed("insertion-order-series-name") || bodyInsertionOrderSeriesName != "" {
					bodyMap["SeriesName"] = bodyInsertionOrderSeriesName
				}
				if cmd.Flags().Changed("insertion-order-spend-cap-amount") || bodyInsertionOrderSpendCapAmount != 0.0 {
					bodyMap["SpendCapAmount"] = bodyInsertionOrderSpendCapAmount
				}
				if cmd.Flags().Changed("insertion-order-start-date") || bodyInsertionOrderStartDate != "" {
					bodyMap["StartDate"] = bodyInsertionOrderStartDate
				}
				if cmd.Flags().Changed("insertion-order-status") || bodyInsertionOrderStatus != "" {
					bodyMap["Status"] = bodyInsertionOrderStatus
				}
			}
			data, statusCode, err := c.PutWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			// Inspect the mutate response body for a partial-failure-shaped
			// field (e.g. Google Ads `partialFailureError`). Several Google
			// APIs return 200 OK with a partial-failure field when some
			// operations in the batch failed; ignoring it silently swallows
			// real failures. Detection runs before output-mode selection so
			// the exit code is consistent regardless of how stdout is
			// rendered. --dry-run short-circuits because no real request
			// was sent.
			var partialFailure *partialFailureReport
			if !flags.dryRun && statusCode >= 200 && statusCode < 300 {
				partialFailure = detectPartialFailure(data)
				if partialFailure != nil {
					fmt.Fprintf(os.Stderr, "warning: partial failure detected in %s response: %s\n", "customer-billing", partialFailure.Message)
					if len(partialFailure.ResourceNames) > 0 {
						fmt.Fprintf(os.Stderr, "         succeeded: %d operation(s)\n", len(partialFailure.ResourceNames))
					}
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						if partialFailure != nil && !flags.allowPartialFailure {
							return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-billing", partialFailure.Message))
						}
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
							if partialFailure != nil && !flags.allowPartialFailure {
								return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-billing", partialFailure.Message))
							}
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					if partialFailure != nil && !flags.allowPartialFailure {
						return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-billing", partialFailure.Message))
					}
					return nil
				}
				envelope := map[string]any{
					"action":   "put",
					"resource": "customer-billing",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300 && (partialFailure == nil || flags.allowPartialFailure),
				}
				if flags.agent {
					envelope["meta"] = map[string]any{"source": "live"}
				}
				if partialFailure != nil {
					envelope["partial_failure"] = partialFailure
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				// Verify-mode synthetic envelope detection runs against RAW data
				// (before --compact/--select filtering) so the sentinel field is
				// guaranteed to be visible even if the operator passes a filter
				// flag that would otherwise strip it. Surfaces a top-level
				// verify_noop signal + flips success to false. Mirrors the dry_run
				// shape above.
				if len(data) > 0 {
					var rawParsed any
					if err := json.Unmarshal(data, &rawParsed); err == nil {
						if m, ok := rawParsed.(map[string]any); ok {
							if v, ok := m["__pp_verify_synthetic__"].(bool); ok && v {
								envelope["verify_noop"] = true
								envelope["success"] = false
							}
						}
					}
				}
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered, map[string]bool{"LastModifiedTime": true})
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						if flags.agent {
							envelope["results"] = parsed
						} else {
							envelope["data"] = parsed
						}
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				resultKey := "data"
				if flags.agent {
					resultKey = "results"
				}
				structured, err := wrapPlatformStructuredOutput(json.RawMessage(envelopeJSON), flags, resultKey, true)
				if err != nil {
					return err
				}
				if perr := printOutput(cmd.OutOrStdout(), structured, true); perr != nil {
					return perr
				}
				if partialFailure != nil && !flags.allowPartialFailure {
					return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-billing", partialFailure.Message))
				}
				return nil
			}
			// Fall-through for mutate paths that did not hit the table or
			// asJSON branches: --quiet, --csv, --plain, and default terminal
			// raw output. printOutputWithFlags renders the body, then the
			// typed partial-failure exit fires unless --allow-partial-failure
			// downgrades it. Without this guard a partial failure would exit
			// 0 for these output modes — the exact silent-swallow regression
			// the surrounding patch is preventing for asJSON / piped output.
			if perr := printOutputWithFlags(cmd.OutOrStdout(), data, flags); perr != nil {
				return perr
			}
			if partialFailure != nil && !flags.allowPartialFailure {
				return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-billing", partialFailure.Message))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bodyInsertionOrderAccountId, "insertion-order-account-id", "", "Account id")
	cmd.Flags().StringVar(&bodyInsertionOrderAccountNumber, "insertion-order-account-number", "", "Account number")
	cmd.Flags().StringVar(&bodyInsertionOrderBookingCountryCode, "insertion-order-booking-country-code", "", "Booking country code")
	cmd.Flags().Float64Var(&bodyInsertionOrderBudgetRemaining, "insertion-order-budget-remaining", 0.0, "Budget remaining")
	cmd.Flags().Float64Var(&bodyInsertionOrderBudgetRemainingPercent, "insertion-order-budget-remaining-percent", 0.0, "Budget remaining percent")
	cmd.Flags().Float64Var(&bodyInsertionOrderBudgetSpent, "insertion-order-budget-spent", 0.0, "Budget spent")
	cmd.Flags().Float64Var(&bodyInsertionOrderBudgetSpentPercent, "insertion-order-budget-spent-percent", 0.0, "Budget spent percent")
	cmd.Flags().StringVar(&bodyInsertionOrderComment, "insertion-order-comment", "", "Comment")
	cmd.Flags().StringVar(&bodyInsertionOrderEndDate, "insertion-order-end-date", "", "End date")
	cmd.Flags().StringVar(&bodyInsertionOrderId, "insertion-order-id", "", "Id")
	cmd.Flags().BoolVar(&bodyInsertionOrderIsEndless, "insertion-order-is-endless", false, "Is endless")
	cmd.Flags().BoolVar(&bodyInsertionOrderIsInSeries, "insertion-order-is-in-series", false, "Is in series")
	cmd.Flags().BoolVar(&bodyInsertionOrderIsUnlimited, "insertion-order-is-unlimited", false, "Is unlimited")
	cmd.Flags().StringVar(&bodyInsertionOrderLastModifiedByUserId, "insertion-order-last-modified-by-user-id", "", "Last modified by user id")
	cmd.Flags().StringVar(&bodyInsertionOrderLastModifiedTime, "insertion-order-last-modified-time", "", "Last modified time")
	cmd.Flags().StringVar(&bodyInsertionOrderName, "insertion-order-name", "", "Name")
	cmd.Flags().Float64Var(&bodyInsertionOrderNotificationThreshold, "insertion-order-notification-threshold", 0.0, "Notification threshold")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesChangeStatus, "insertion-order-pending-changes-change-status", "", "Change status")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesComment, "insertion-order-pending-changes-comment", "", "Comment")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesEndDate, "insertion-order-pending-changes-end-date", "", "End date")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesModifiedDateTime, "insertion-order-pending-changes-modified-date-time", "", "Modified date time")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesName, "insertion-order-pending-changes-name", "", "Name")
	cmd.Flags().Float64Var(&bodyInsertionOrderPendingChangesNotificationThreshold, "insertion-order-pending-changes-notification-threshold", 0.0, "Notification threshold")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesPurchaseOrder, "insertion-order-pending-changes-purchase-order", "", "Purchase order")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesReferenceId, "insertion-order-pending-changes-reference-id", "", "Reference id")
	cmd.Flags().IntVar(&bodyInsertionOrderPendingChangesRequestedByUserId, "insertion-order-pending-changes-requested-by-user-id", 0, "Requested by user id")
	cmd.Flags().Float64Var(&bodyInsertionOrderPendingChangesSpendCapAmount, "insertion-order-pending-changes-spend-cap-amount", 0.0, "Spend cap amount")
	cmd.Flags().StringVar(&bodyInsertionOrderPendingChangesStartDate, "insertion-order-pending-changes-start-date", "", "Start date")
	cmd.Flags().StringVar(&bodyInsertionOrderPurchaseOrder, "insertion-order-purchase-order", "", "Purchase order")
	cmd.Flags().StringVar(&bodyInsertionOrderReferenceId, "insertion-order-reference-id", "", "Reference id")
	cmd.Flags().StringVar(&bodyInsertionOrderSeriesFrequencyType, "insertion-order-series-frequency-type", "", "Series frequency type")
	cmd.Flags().StringVar(&bodyInsertionOrderSeriesName, "insertion-order-series-name", "", "Series name")
	cmd.Flags().Float64Var(&bodyInsertionOrderSpendCapAmount, "insertion-order-spend-cap-amount", 0.0, "Spend cap amount")
	cmd.Flags().StringVar(&bodyInsertionOrderStartDate, "insertion-order-start-date", "", "Start date")
	cmd.Flags().StringVar(&bodyInsertionOrderStatus, "insertion-order-status", "", "Status")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
