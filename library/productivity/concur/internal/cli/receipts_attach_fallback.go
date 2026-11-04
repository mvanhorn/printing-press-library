// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(amend-2026-11-03: Browser fallback for attaching receipts to an expense)

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/client"
	"github.com/spf13/cobra"
)

// receiptsPollTimeout and receiptsPollInterval are package-level variables
// allowing tests to inject shorter timeouts.
var receiptsPollTimeout = 30 * time.Second
var receiptsPollInterval = 1500 * time.Millisecond

type receiptUploadResult struct {
	ExpenseID      string `json:"expense_id"`
	ReportID       string `json:"report_id"`
	ReceiptImageID string `json:"receipt_image_id"`
	Via            string `json:"via"`
}

type receiptUploadPartialSuccessError struct {
	reportId  string
	expenseId string
	cause     error
}

func (e *receiptUploadPartialSuccessError) Error() string {
	return fmt.Sprintf(
		"the browser fallback's receipt upload command completed but the receipt image has not yet appeared on expense %s (report %s) -- the upload may still be processing. Check 'expenses list --no-cache' or the Concur web UI before retrying (retrying blindly risks uploading a duplicate receipt): %v",
		e.expenseId, e.reportId, e.cause,
	)
}

func (e *receiptUploadPartialSuccessError) Unwrap() error { return e.cause }

// attachReceiptViaBrowserFallback drives the Concur web UI via agent-browser to attach a receipt
// to an existing expense, bypassing the broken receipts API POST endpoint.
func attachReceiptViaBrowserFallback(cmd *cobra.Command, c *client.Client, flags *rootFlags, userId, contextType, reportId, expenseId, filePath string) ([]byte, error) {
	// 1. Fetch the report's expenses first to verify the target expense exists and does not already have a receipt.
	expenses, err := fetchReportExpenses(cmd.Context(), c, userId, contextType, reportId)
	if err != nil {
		return nil, fmt.Errorf("fetching expense list before browser fallback: %w", err)
	}

	var targetFound bool
	for _, item := range expenses {
		var exp struct {
			ExpenseID      string  `json:"expenseId"`
			ReceiptImageID *string `json:"receiptImageId"`
		}
		if err := json.Unmarshal(item, &exp); err == nil && exp.ExpenseID == expenseId {
			targetFound = true
			if exp.ReceiptImageID != nil && *exp.ReceiptImageID != "" {
				return nil, fmt.Errorf("expense already has a receipt image; refusing to attach a second one")
			}
			break
		}
	}

	if !targetFound {
		return nil, fmt.Errorf("expense %s not found in report %s", expenseId, reportId)
	}

	// Invalidate response cache before any browser write action
	defer c.InvalidateCache()

	// 2. Derive the web UI host and construct the expense edit view URL.
	host, err := resolveReportsUIHost(c.RequestBaseURL())
	if err != nil {
		return nil, err
	}

	editURL := fmt.Sprintf(
		"https://%s/nui/expense/reports/%s/expenses/%s",
		host, url.PathEscape(reportId), url.PathEscape(expenseId),
	)

	fmt.Fprintf(cmd.ErrOrStderr(), "HTTP API returned 404. Falling back to browser automation...\n")

	if port := refreshActiveCDPPort(); port != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Using dedicated Concur browser on %s (no separate login needed)\n", describeCDPTarget(port))
	}

	// 3. Open the page and wait for slow SPA load.
	if err := openPageTolerant(editURL, reportId); err != nil {
		return nil, err
	}
	time.Sleep(1500 * time.Millisecond)

	// Check if sign-in is required
	refs, err := agentBrowserSnapshotRefs()
	if err != nil {
		return nil, err
	}
	if _, ok := findRef(refs, "sign in", ""); ok {
		if activeCDPPort != "" {
			return nil, fmt.Errorf("the dedicated Concur browser on %s is no longer logged in -- log in to concursolutions.com in that window again, then re-run this command", describeCDPTarget(activeCDPPort))
		}
		return nil, fmt.Errorf("not logged in to Concur in the automated browser -- log in manually in the opened Chrome window, then re-run this command (or set up a dedicated debug-enabled Chrome profile to skip this every time; see --help)")
	}

	// Dismiss interstitial promo dialogs if any are open
	if ref, ok := findRef(refs, "close", "button"); ok {
		_, _ = runAgentBrowser("click", "@"+ref)
		time.Sleep(300 * time.Millisecond)
		if refs2, err := agentBrowserSnapshotRefs(); err == nil {
			if _, stillOpen := findRef(refs2, "close", "button"); stillOpen {
				_, _ = runAgentBrowser("press", "Escape")
				time.Sleep(300 * time.Millisecond)
			}
		}
	}

	// 4. Upload the receipt file using #upload-file selector, falling back to input[type=file].
	_, uploadErr := runAgentBrowser("upload", "#upload-file", filePath)
	if uploadErr != nil {
		if _, err2 := runAgentBrowser("upload", "input[type=file]", filePath); err2 != nil {
			return nil, fmt.Errorf("uploading receipt file via '#upload-file' failed: %v; fallback via 'input[type=file]' also failed: %w", uploadErr, err2)
		}
	}

	// 5. Poll fetchReportExpenses until the expense's receiptImageId becomes non-empty
	var receiptImageId string
	pollStart := time.Now()

	for {
		time.Sleep(receiptsPollInterval)
		expenses, err := fetchReportExpenses(cmd.Context(), c, userId, contextType, reportId)
		if err == nil {
			var found bool
			for _, item := range expenses {
				var exp struct {
					ExpenseID      string `json:"expenseId"`
					ReceiptImageID string `json:"receiptImageId"`
				}
				if json.Unmarshal(item, &exp) == nil && exp.ExpenseID == expenseId {
					found = true
					if exp.ReceiptImageID != "" {
						receiptImageId = exp.ReceiptImageID
						break
					}
				}
			}
			if receiptImageId != "" {
				break
			}
			if !found {
				return nil, fmt.Errorf("expense %s not found in report %s during polling", expenseId, reportId)
			}
		}
		if time.Since(pollStart) >= receiptsPollTimeout {
			return nil, &receiptUploadPartialSuccessError{
				reportId:  reportId,
				expenseId: expenseId,
				cause:     fmt.Errorf("receipt image did not appear on expense %s within %s", expenseId, receiptsPollTimeout),
			}
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Receipt attached successfully via browser.\n")

	result := receiptUploadResult{
		ExpenseID:      expenseId,
		ReportID:       reportId,
		ReceiptImageID: receiptImageId,
		Via:            "browser",
	}

	return json.Marshal(result)
}
