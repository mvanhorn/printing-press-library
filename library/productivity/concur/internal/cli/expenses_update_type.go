// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/client"
	"github.com/spf13/cobra"
)

// PATCH(amend-2026-10-17: `expenses update --type` for itemization lines) --
// itemization lines created by receipt capture (or by hand) can be left with
// the placeholder type "Undefined", which blocks report submission, and there
// was no CLI path to fix them: `expenses update` had no type field and its
// PATCH fallback only drives Business Purpose and Amount. The API rejects
// expense writes for browser-session auth on both host families (verified
// with a no-op PATCH: 404 "No static resource"), so the change is made
// through the web form, which was verified live: deep link to the line, open
// the Expense Type picker, click the option, "Save Itemization". See
// .printing-press-patches/expenses-update-itemization-type.json.

// maxOfferedTypesListed caps the option names echoed back when the requested
// type is not offered, so the error stays readable.
const maxOfferedTypesListed = 25

// errConcurSignedOut means the automated browser landed on Concur's sign-in
// page instead of the requested form.
var errConcurSignedOut = errors.New("concur sign-in page shown")

// concurSignedOutError describes a signed-out browser, distinguishing a
// dedicated debug browser (log in there) from the isolated one.
func concurSignedOutError() error {
	if activeCDPPort != "" {
		return fmt.Errorf("the dedicated Concur browser on %s is no longer logged in -- log in to concursolutions.com in that window again, then re-run this command", describeCDPTarget(activeCDPPort))
	}
	return fmt.Errorf("not logged in to Concur in the automated browser -- log in manually in the opened Chrome window, then re-run this command (or set up a dedicated debug-enabled Chrome profile to skip this every time; see --help)")
}

// waitForRefOrSignIn is waitForRef that also stops as soon as the sign-in page
// appears. A deep link can take ~25s to render, so waiting out the full
// timeout before reporting an expired session would be a poor failure mode.
func waitForRefOrSignIn(nameSubstr, role string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		refs, err := agentBrowserSnapshotRefs()
		if err != nil {
			return "", err
		}
		if ref, ok := findRef(refs, nameSubstr, role); ok {
			return ref, nil
		}
		if _, signedOut := findRef(refs, "sign in", ""); signedOut {
			return "", errConcurSignedOut
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no element matching %q (role %q) appeared within %s -- the page structure may have changed", nameSubstr, role, timeout)
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// getOptionRefs returns the browser's current refs including picker options.
// Options are not always present in the interactive-only snapshot, so fall
// back to a plain snapshot when the interactive one carries none.
func getOptionRefs() (map[string]agentBrowserRefEntry, error) {
	if refs, err := agentBrowserSnapshotRefs(); err == nil {
		for _, entry := range refs {
			if entry.Role == "option" {
				return refs, nil
			}
		}
	}
	out, err := runAgentBrowser("snapshot", "--json")
	if err != nil {
		return nil, err
	}
	var env agentBrowserSnapshotEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("parsing plain agent-browser snapshot: %w", err)
	}
	return env.Data.Refs, nil
}

// printExpensesUpdateTypeDryRun reports the browser action --type would take,
// without contacting Concur or launching a browser.
func printExpensesUpdateTypeDryRun(cmd *cobra.Command, c *client.Client, flags *rootFlags, reportId, expenseId, wantType string) error {
	host, err := resolveReportsUIHost(c.RequestBaseURL())
	if err != nil {
		return err
	}
	deepLink := fmt.Sprintf("https://%s/nui/expense/reports/%s/expenses/%s/itemizations/%s", host, url.PathEscape(reportId), "<parent_expense_id>", url.PathEscape(expenseId))
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"action":    "update_type",
			"resource":  "expenses",
			"dry_run":   true,
			"deep_link": deepLink,
			"type":      wantType,
		}, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: would open %s to change type to %q\n", deepLink, wantType)
	return nil
}

// runExpensesUpdateType implements `expenses update --type`. It only accepts
// itemization lines (expenses with a parentExpenseId): that is the flow
// verified live, and silently changing the type of a top-level expense could
// reset its form fields.
func runExpensesUpdateType(cmd *cobra.Command, c *client.Client, userId, contextType, reportId, expenseId, expensePath, wantType string) (json.RawMessage, int, error) {
	raw, err := c.GetNoCache(cmd.Context(), expensePath, nil)
	if err != nil {
		return nil, 0, err
	}
	var line struct {
		ParentExpenseId *string `json:"parentExpenseId"`
		ExpenseType     struct {
			Name string `json:"name"`
		} `json:"expenseType"`
		TransactionAmount struct {
			Value float64 `json:"value"`
		} `json:"transactionAmount"`
	}
	if err := json.Unmarshal(raw, &line); err != nil {
		return nil, 0, fmt.Errorf("parsing expense: %w", err)
	}
	if line.ParentExpenseId == nil || *line.ParentExpenseId == "" {
		return nil, 0, fmt.Errorf("--type is only supported for itemization lines (this expense has no parent); change a top-level expense's type in the Concur web UI")
	}
	if strings.EqualFold(strings.TrimSpace(line.ExpenseType.Name), strings.TrimSpace(wantType)) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Note: expense %s is already of type %q. No update needed.\n", expenseId, wantType)
		return raw, 200, nil
	}
	return updateExpenseTypeViaBrowserFallback(cmd, c, userId, contextType, reportId, expenseId, *line.ParentExpenseId, wantType, line.TransactionAmount.Value)
}

func updateExpenseTypeViaBrowserFallback(cmd *cobra.Command, c *client.Client, userId, contextType, reportId, expenseId, parentExpenseId, wantType string, initialAmount float64) (json.RawMessage, int, error) {
	const stepTimeout = 10 * time.Second
	// A fresh deep link renders the report and expense panels before the
	// itemization panel, which can take ~25s.
	const deepLinkRenderTimeout = 40 * time.Second

	host, err := resolveReportsUIHost(c.RequestBaseURL())
	if err != nil {
		return nil, 0, err
	}
	deepLink := fmt.Sprintf(
		"https://%s/nui/expense/reports/%s/expenses/%s/itemizations/%s",
		host, url.PathEscape(reportId), url.PathEscape(parentExpenseId), url.PathEscape(expenseId),
	)

	fmt.Fprintf(cmd.ErrOrStderr(), "Expense type changes use the browser (the API rejects expense writes for browser-session auth).\n")
	if target := refreshActiveCDPPort(); target != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Using dedicated Concur browser on %s (no separate login needed)\n", describeCDPTarget(target))
	}

	if _, err := runAgentBrowser("open", deepLink); err != nil {
		return nil, 0, err
	}
	time.Sleep(1500 * time.Millisecond)

	// Same one-time promotional interstitial the other fallbacks dismiss.
	if refs, err := agentBrowserSnapshotRefs(); err == nil {
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
	}

	comboboxRef, err := waitForRefOrSignIn("Expense Type", "combobox", deepLinkRenderTimeout)
	if errors.Is(err, errConcurSignedOut) {
		return nil, 0, concurSignedOutError()
	}
	if err != nil {
		return nil, 0, fmt.Errorf("could not find the Expense Type field on the itemization form: %w", err)
	}
	if _, err := runAgentBrowser("click", "@"+comboboxRef); err != nil {
		return nil, 0, fmt.Errorf("opening the Expense Type picker: %w", err)
	}
	time.Sleep(1500 * time.Millisecond)

	optionRefs, err := getOptionRefs()
	if err != nil {
		return nil, 0, err
	}
	wantName := strings.TrimSpace(wantType)
	var matchedRef string
	var offered []string
	seen := make(map[string]bool)
	for ref, entry := range optionRefs {
		if entry.Role != "option" {
			continue
		}
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		if !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			offered = append(offered, name)
		}
		if strings.EqualFold(name, wantName) {
			matchedRef = ref
		}
	}
	if matchedRef == "" {
		// Close the picker so nothing is left half-edited; nothing was saved.
		_, _ = runAgentBrowser("press", "Escape")
		sort.Slice(offered, func(i, j int) bool { return strings.ToLower(offered[i]) < strings.ToLower(offered[j]) })
		if len(offered) > maxOfferedTypesListed {
			offered = offered[:maxOfferedTypesListed]
		}
		return nil, 0, fmt.Errorf("expense type %q is not offered for this line under your Concur policy (the picker is policy-filtered, so some types in 'expense-types list' are not selectable); offered: %s", wantType, strings.Join(offered, ", "))
	}
	if _, err := runAgentBrowser("click", "@"+matchedRef); err != nil {
		return nil, 0, fmt.Errorf("selecting expense type %q: %w", wantType, err)
	}
	time.Sleep(1 * time.Second)

	saveRef, err := waitForRef("Save Itemization", "button", stepTimeout)
	if err != nil {
		return nil, 0, fmt.Errorf("could not find the Save Itemization button: %w", err)
	}
	// A successful save navigates back to the itemization list, so the URL
	// stops containing this line's id.
	//
	// PATCH(Greptile review, "Saved changes can look unsuccessful") -- the
	// helper can fail after the click already landed (the URL check or the
	// re-find of the Save button errors once the page has navigated away), so
	// its error does not prove nothing was saved. Hold it and read the line
	// back to decide, rather than reporting an ordinary failure for a type
	// that may already be changed.
	saveErr := clickWithNavigationRetry(saveRef, "itemizations/"+expenseId, "Save Itemization", "button", stepTimeout)
	time.Sleep(1 * time.Second)

	expensePath := "/expensereports/v4/users/{user_id}/context/{context_type}/reports/{report_id}/expenses/{expense_id}"
	expensePath = replacePathParam(expensePath, "user_id", formatCLIParamValue(userId))
	expensePath = replacePathParam(expensePath, "context_type", formatCLIParamValue(contextType))
	expensePath = replacePathParam(expensePath, "report_id", formatCLIParamValue(reportId))
	expensePath = replacePathParam(expensePath, "expense_id", formatCLIParamValue(expenseId))

	// From here the click may have landed: any failure that is not a
	// confirmed no-op means the line may already carry the new type, so wrap
	// errors the way the other mutating fallbacks do.
	updated, err := c.GetNoCache(cmd.Context(), expensePath, nil)
	if err != nil {
		cause := fmt.Errorf("fetching expense after browser fallback save: %w", err)
		if saveErr != nil {
			cause = fmt.Errorf("%w (the Save Itemization click had already reported: %v)", cause, saveErr)
		}
		return nil, 0, &expensesUpdatePartialSuccessError{
			reportId:  reportId,
			expenseId: expenseId,
			cause:     cause,
		}
	}
	if err := verifyExpenseTypeUpdateApplied(updated, wantType, initialAmount); err != nil {
		// The click helper failed and the line still shows its old type: the
		// two signals agree that nothing was saved, so this is an ordinary,
		// safely retryable failure rather than a possible partial success.
		if saveErr != nil && errors.Is(err, errExpenseTypeNotApplied) {
			return nil, 0, saveErr
		}
		return nil, 0, &expensesUpdatePartialSuccessError{
			reportId:  reportId,
			expenseId: expenseId,
			cause:     err,
		}
	}

	if saveErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Note: the Save Itemization click reported an error (%v), but re-reading the line shows type %q, so the save went through.\n", saveErr, wantType)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Expense type updated successfully via browser.\n")
	return updated, 200, nil
}

// errExpenseTypeNotApplied marks a re-fetched line that still shows its old
// type, as opposed to one whose amount changed unexpectedly.
var errExpenseTypeNotApplied = errors.New("type save did not actually apply")

// verifyExpenseTypeUpdateApplied checks that a re-fetched line actually shows
// the requested type and an unchanged amount. A successful GET alone is not
// proof the save happened (see verifyExpenseUpdateApplied for the
// live-reproduced false-success this guards against).
func verifyExpenseTypeUpdateApplied(raw json.RawMessage, wantType string, wantAmount float64) error {
	var got struct {
		ExpenseType struct {
			Name string `json:"name"`
		} `json:"expenseType"`
		TransactionAmount struct {
			Value float64 `json:"value"`
		} `json:"transactionAmount"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return fmt.Errorf("parsing re-fetched expense to verify the type save applied: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(got.ExpenseType.Name), strings.TrimSpace(wantType)) {
		return fmt.Errorf("%w: requested type %q but the re-fetched expense still shows %q -- the Save Itemization click may not have registered", errExpenseTypeNotApplied, wantType, got.ExpenseType.Name)
	}
	const amountEpsilon = 0.005
	if got.TransactionAmount.Value < wantAmount-amountEpsilon || got.TransactionAmount.Value > wantAmount+amountEpsilon {
		return fmt.Errorf("type save changed the amount unexpectedly: it was %.2f and is now %.2f", wantAmount, got.TransactionAmount.Value)
	}
	return nil
}
