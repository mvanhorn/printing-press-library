package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco/internal/client"
)

// FetchReceiptsWithClient runs the receipts GraphQL query using an existing
// client, returning parsed receipts. Exported for the MCP tools package.
func FetchReceiptsWithClient(ctx context.Context, c *client.Client, startDate, endDate string) ([]costcoReceipt, error) {
	body := map[string]any{
		"query":     receiptsQuery,
		"variables": map[string]string{"startDate": startDate, "endDate": endDate},
	}
	data, _, err := c.PostQueryWithParams(ctx, costcoGraphQLPath, nil, body)
	if err != nil {
		return nil, err
	}
	var env receiptsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding receipts response: %w", err)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("costco API error: %s", env.Errors[0].Message)
	}
	return env.Data.Receipts, nil
}

// ReceiptSummaryRow is the exported form of receiptSummary.
type ReceiptSummaryRow = receiptSummary

// SummarizeReceipts converts raw receipts to summary rows, optionally filtered
// by channel type ("all", "warehouse", "gas", "carwash").
func SummarizeReceipts(receipts []costcoReceipt, typ string) []ReceiptSummaryRow {
	rows := make([]ReceiptSummaryRow, 0, len(receipts))
	for _, r := range receipts {
		if !matchesType(r, typ) {
			continue
		}
		rows = append(rows, summarize(r))
	}
	return rows
}

// SpendRow is the exported form of spendRow.
type SpendRow = spendRow

// AggregateSpendFromReceipts rolls up receipts by dimension.
func AggregateSpendFromReceipts(receipts []costcoReceipt, by string) ([]SpendRow, error) {
	return aggregateSpend(receipts, by)
}

// ItemHistoryRow is the exported form of itemHistoryRow.
type ItemHistoryRow = itemHistoryRow

// MatchItemHistoryFromReceipts searches line items for a query string.
func MatchItemHistoryFromReceipts(receipts []costcoReceipt, query string) []ItemHistoryRow {
	return matchItemHistory(receipts, query)
}

// SavingsView is the exported form of savingsView.
type SavingsView = savingsView

// ComputeSavingsFromReceipts totals instant savings and coupons.
func ComputeSavingsFromReceipts(receipts []costcoReceipt, start, end string) SavingsView {
	return computeSavings(receipts, start, end)
}

// ReturnRow is the exported form of returnRow.
type ReturnRow = returnRow

// ItemsInReturnWindow returns items still inside a return window.
func ItemsInReturnWindow(receipts []costcoReceipt, days int) []ReturnRow {
	return itemsInWindow(receipts, days, time.Now())
}

// DepthResult is the exported form of depthResult.
type DepthResult = depthResult

// ProbeHistoryDepth runs the backward date-range walk.
func ProbeHistoryDepth(ctx context.Context, c *client.Client, maxYears int) (DepthResult, error) {
	end := todayDate()
	uiFloor := time.Now().AddDate(-uiCapYears, 0, 0).Format(dateLayout)
	ladder := depthLadder(maxYears, false)
	probes := make([]depthProbe, 0, len(ladder))
	for _, yb := range ladder {
		start := time.Now().AddDate(-yb, 0, 0).Format(dateLayout)
		receipts, err := FetchReceiptsWithClient(ctx, c, start, end)
		if err != nil {
			return DepthResult{}, err
		}
		probes = append(probes, depthProbe{
			YearsBack:       yb,
			StartDate:       start,
			ReceiptCount:    len(receipts),
			EarliestReceipt: earliestDate(receipts),
		})
	}
	return analyzeDepth(probes, uiFloor), nil
}

// ResolveRange is the exported form of resolveRange.
func ResolveRange(since, until string, years int) (string, string, error) {
	return resolveRange(since, until, years)
}
