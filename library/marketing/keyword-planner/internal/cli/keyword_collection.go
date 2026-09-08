// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	pressclient "github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/keywordapi"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
	"github.com/spf13/cobra"
)

// plannerCollectionOutput is the live command's typed result. Raw response
// bytes stay in the portfolio and are available through `portfolio show`; the
// collection response keeps the normalised rows and coverage small enough for
// agents to consume.
type plannerCollectionOutput struct {
	Snapshot           portfolio.Snapshot       `json:"snapshot"`
	Rows               []portfolio.Row          `json:"rows"`
	Coverage           []portfolio.CoverageView `json:"coverage"`
	ReceiptCount       int                      `json:"receipt_count"`
	StoredKeywordCount int                      `json:"stored_keyword_count"`
	StoredMonthlyCount int                      `json:"stored_monthly_count"`
	RenderedRowCount   int                      `json:"rendered_row_count"`
	RenderLimit        int                      `json:"render_limit"`
	RowsTruncated      bool                     `json:"rows_truncated"`
	Warnings           []string                 `json:"warnings,omitempty"`
}

var plannerCollectionFields = map[string]bool{
	"snapshot": true, "rows": true, "coverage": true, "receipt_count": true,
	"stored_keyword_count": true, "stored_monthly_count": true, "rendered_row_count": true,
	"render_limit": true, "rows_truncated": true, "warnings": true,
}

type plannerReceiptAttempt struct {
	ids    portfolio.ResponseIDs
	status int
}

// plannerReceiptCollector is deliberately local to the CLI orchestration. The
// transport invokes it once for every attempt, including retries and transport
// failures, before it classifies or decodes the response.
type plannerReceiptCollector struct {
	store    *portfolio.Store
	snapshot string
	items    []plannerReceiptAttempt
	err      error
}

func (c *plannerReceiptCollector) callback(ctx context.Context, endpoint, pageToken string, page, batch int, requestBody []byte, submitted []string, response keywordapi.Response) error {
	if c == nil || c.store == nil {
		return errors.New("planner receipt collector is not configured")
	}
	if c.err != nil {
		return c.err
	}
	ids, err := c.store.AppendReceipt(ctx, portfolio.ReceiptInput{
		SnapshotID:        c.snapshot,
		Endpoint:          endpoint,
		PageToken:         pageToken,
		PageNumber:        page,
		BatchNumber:       batch,
		Attempt:           response.Attempt,
		RequestBody:       append([]byte(nil), requestBody...),
		SubmittedKeywords: append([]string(nil), submitted...),
		HTTPStatus:        plannerResponseStatus(response),
		GoogleRequestID:   response.RequestID,
		Body:              append([]byte(nil), response.Body...),
		FetchedAt:         response.FetchedAt,
		ErrorCode:         plannerResponseErrorCode(response),
		ErrorMessage:      plannerResponseErrorMessage(response),
	})
	if err != nil {
		c.err = err
		return err
	}
	c.items = append(c.items, plannerReceiptAttempt{ids: ids, status: plannerResponseStatus(response)})
	return nil
}

func plannerResponseStatus(response keywordapi.Response) int {
	if response.Status != 0 {
		return response.Status
	}
	return response.StatusCode
}

func plannerResponseErrorCode(response keywordapi.Response) string {
	if response.BodyWithheld && response.FailureCode != "" {
		return string(response.FailureCode)
	}
	if response.Err != nil {
		if code := keywordapi.CodeOf(response.Err); code != "" && code != "unknown" {
			return string(code)
		}
		if plannerResponseStatus(response) > 0 {
			return string(keywordapi.CodeResponseRead)
		}
		return string(keywordapi.CodeTransport)
	}
	status := plannerResponseStatus(response)
	switch {
	case status >= 200 && status < 300:
		return ""
	case status == http.StatusUnauthorized:
		return string(keywordapi.CodeAuth)
	case status == http.StatusForbidden:
		return string(keywordapi.CodeAccessDenied)
	case status == http.StatusTooManyRequests:
		// A 429 is a transient rate-limit response unless the transport has
		// positively classified the body as daily quota. The callback runs
		// before that classification and must not invent the stronger cause.
		return "rate_limit"
	case status >= 500:
		return string(keywordapi.CodeUpstream5xx)
	case status > 0:
		return string(keywordapi.CodeUpstream)
	default:
		return string(keywordapi.CodeTransport)
	}
}

func plannerResponseErrorMessage(response keywordapi.Response) string {
	if response.BodyWithheld {
		return "response body withheld by credential-evidence policy"
	}
	if response.Err != nil {
		if plannerResponseStatus(response) > 0 {
			return "response body could not be read"
		}
		return "transport failure before a response was received"
	}
	if status := plannerResponseStatus(response); status > 0 && (status < 200 || status >= 300) {
		return fmt.Sprintf("Google Ads returned HTTP %d", status)
	}
	return ""
}

func (c *plannerReceiptCollector) final(response keywordapi.Response) (portfolio.ResponseIDs, bool) {
	if c == nil {
		return portfolio.ResponseIDs{}, false
	}
	for i := len(c.items) - 1; i >= 0; i-- {
		if c.items[i].ids.Attempt == response.Attempt {
			return c.items[i].ids, true
		}
	}
	return portfolio.ResponseIDs{}, false
}

// normalize attempts from one logical page/batch. A failed retry is durable
// evidence and receives failed coverage; it is not allowed to hide a later
// successful attempt. A successful attempt that cannot be normalised is fatal.
func (c *plannerReceiptCollector) normalize(ctx context.Context) error {
	if c == nil || c.store == nil {
		return errors.New("planner receipt collector is not configured")
	}
	var firstErr error
	for _, item := range c.items {
		_, err := c.store.NormalizeReceipt(ctx, item.ids.ReceiptID)
		if err == nil {
			continue
		}
		if item.status >= 200 && item.status < 300 && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func plannerAccountRequestBody() []byte {
	return []byte(`{"query":"SELECT customer.id, customer.currency_code FROM customer LIMIT 1"}`)
}

func plannerAccountPhase(ctx context.Context, client *keywordapi.Client, store *portfolio.Store, snapshotID string) (keywordapi.AccountInfo, portfolio.ResponseIDs, error) {
	if client == nil || store == nil {
		return keywordapi.AccountInfo{}, portfolio.ResponseIDs{}, errors.New("planner account phase is not configured")
	}
	collector := &plannerReceiptCollector{store: store, snapshot: snapshotID}
	account, err := client.Account(ctx, func(response keywordapi.Response) error {
		return collector.callback(ctx, portfolio.EndpointAccount, "", 0, 0, plannerAccountRequestBody(), nil, response)
	})
	if collector.err != nil {
		// Preserve the committed account receipt even when the callback reports
		// a storage error. The caller can then finish the snapshot incomplete
		// with the exact receipt IDs that were durable before the failure.
		ids, _ := collector.final(account.Response)
		return account, ids, collector.err
	}
	ids, ok := collector.final(account.Response)
	if err != nil {
		return account, ids, err
	}
	if !ok {
		return account, portfolio.ResponseIDs{}, errors.New("Google Ads account response was not preserved")
	}
	if err := store.SetAccountCurrency(ctx, snapshotID, account.CurrencyCode, account.Provenance, ids.ReceiptID); err != nil {
		return account, ids, err
	}
	return account, ids, nil
}

func plannerStartSnapshot(ctx context.Context, store *portfolio.Store, endpoint string, customerID, loginCustomerID string, language, network, currency, currencySource string, geos, seeds, keywords []string, window plannerWindow, body []byte, sourceVariant string, now time.Time) (portfolio.Snapshot, error) {
	return store.StartSnapshot(ctx, portfolio.SnapshotInput{
		RunID:              fmt.Sprintf("%s-%d", endpoint, now.UTC().UnixNano()),
		Endpoint:           endpoint,
		APIVersion:         "v25",
		DiscoveryRevision:  "20260831",
		CustomerID:         customerID,
		LoginCustomerID:    loginCustomerID,
		Language:           language,
		Network:            network,
		CurrencyCode:       currency,
		CurrencySource:     currencySource,
		GeoTargetConstants: append([]string(nil), geos...),
		SubmittedSeeds:     append([]string(nil), seeds...),
		SubmittedKeywords:  append([]string(nil), keywords...),
		RequestedStart:     window.Start,
		RequestedEnd:       window.End,
		RequestBody:        append([]byte(nil), body...),
		SourceVariant:      sourceVariant,
	}, now)
}

func finishPlannerSnapshot(ctx context.Context, store *portfolio.Store, snapshotID string, collectionErr error, warnings, failed, missing []string) error {
	if store == nil {
		return errors.New("planner portfolio store is not configured")
	}
	in := portfolio.FinishInput{
		SnapshotID:     snapshotID,
		Status:         portfolio.StatusComplete,
		Complete:       true,
		WarningFlags:   append([]string(nil), warnings...),
		FailedReceipts: append([]string(nil), failed...),
		MissingBatches: append([]string(nil), missing...),
	}
	if collectionErr != nil {
		in.Status = portfolio.StatusIncomplete
		in.Complete = false
	}
	return store.Finish(ctx, in)
}

func plannerAppendFailureID(ids []string, item portfolio.ResponseIDs) []string {
	if item.ReceiptID == "" {
		return ids
	}
	return append(ids, item.ReceiptID)
}

func plannerIdeasNextPage(body []byte) (string, error) {
	var envelope struct {
		NextPageToken string `json:"nextPageToken"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return "", fmt.Errorf("decode Ideas page continuation: %w", err)
	}
	return strings.TrimSpace(envelope.NextPageToken), nil
}

func collectPlannerIdeas(ctx context.Context, client *keywordapi.Client, store *portfolio.Store, input plannerIdeasInput, now time.Time) (plannerCollectionOutput, error) {
	return collectPlannerIdeasWithLogin(ctx, client, store, input, "", now)
}

func collectPlannerIdeasWithLogin(ctx context.Context, client *keywordapi.Client, store *portfolio.Store, input plannerIdeasInput, loginCustomerID string, now time.Time) (plannerCollectionOutput, error) {
	firstBody, err := buildPlannerIdeasRequest(input, "")
	if err != nil {
		return plannerCollectionOutput{}, err
	}
	snapshot, err := plannerStartSnapshot(ctx, store, portfolio.EndpointIdeas, input.CustomerID, loginCustomerID, input.Language, input.Network, "", "", input.GeoTargets, input.Seeds, nil, input.Window, firstBody, "keyword_seed", now)
	if err != nil {
		return plannerCollectionOutput{}, err
	}
	warnings := append([]string(nil), input.Warnings...)
	var collectionErr error
	failed := []string{}
	missing := []string{}

	_, accountIDs, err := plannerAccountPhase(ctx, client, store, snapshot.ID)
	if err != nil {
		failed = plannerAppendFailureID(failed, accountIDs)
		collectionErr = err
	}
	pageToken := ""
	page := 0
	pagesCollected := 0
	for collectionErr == nil {
		body, buildErr := buildPlannerIdeasRequest(input, pageToken)
		if buildErr != nil {
			collectionErr = buildErr
			break
		}
		collector := &plannerReceiptCollector{store: store, snapshot: snapshot.ID}
		response, callErr := client.Call(ctx, http.MethodPost, mustPlannerCustomerPath(input.CustomerID, "ideas"), body, func(response keywordapi.Response) error {
			return collector.callback(ctx, portfolio.EndpointIdeas, pageToken, page, 0, body, input.Seeds, response)
		})
		if collector.err != nil {
			collectionErr = collector.err
			break
		}
		finalIDs, finalOK := collector.final(response)
		if normErr := collector.normalize(ctx); normErr != nil {
			collectionErr = normErr
		}
		if callErr != nil && collectionErr == nil {
			collectionErr = callErr
		}
		if !finalOK && collectionErr == nil {
			collectionErr = errors.New("Ideas response was not preserved")
		}
		if collectionErr != nil {
			failed = plannerAppendFailureID(failed, finalIDs)
			break
		}
		pagesCollected++
		next, nextErr := plannerIdeasNextPage(response.Body)
		if nextErr != nil {
			collectionErr = nextErr
			failed = plannerAppendFailureID(failed, finalIDs)
			break
		}
		if next == "" {
			break
		}
		if input.MaxPages > 0 && pagesCollected >= input.MaxPages {
			missing = append(missing, fmt.Sprintf("page:%d", pagesCollected))
			collectionErr = fmt.Errorf("--max-pages stopped Ideas collection before continuation page %d; raise --max-pages or use zero for all pages", pagesCollected)
			break
		}
		page++
		pageToken = next
	}

	if finishErr := finishPlannerSnapshot(ctx, store, snapshot.ID, collectionErr, warnings, failed, missing); finishErr != nil {
		if collectionErr == nil {
			collectionErr = finishErr
		} else {
			collectionErr = fmt.Errorf("%w; finalize snapshot: %v", collectionErr, finishErr)
		}
	}
	if collectionErr != nil {
		return plannerCollectionPartialView(ctx, store, snapshot.ID, input.OutputLimit, warnings), collectionErr
	}
	return plannerCollectionView(ctx, store, snapshot.ID, input.OutputLimit, warnings)
}

func collectPlannerHistorical(ctx context.Context, client *keywordapi.Client, store *portfolio.Store, input plannerHistoricalInput, now time.Time) (plannerCollectionOutput, error) {
	return collectPlannerHistoricalWithLogin(ctx, client, store, input, "", now)
}

func collectPlannerHistoricalWithLogin(ctx context.Context, client *keywordapi.Client, store *portfolio.Store, input plannerHistoricalInput, loginCustomerID string, now time.Time) (plannerCollectionOutput, error) {
	batches, err := plannerKeywordBatches(input.Keywords, input.BatchSize)
	if err != nil {
		return plannerCollectionOutput{}, err
	}
	firstBody, err := buildPlannerHistoricalRequest(input, batches[0])
	if err != nil {
		return plannerCollectionOutput{}, err
	}
	snapshot, err := plannerStartSnapshot(ctx, store, portfolio.EndpointHistorical, input.CustomerID, loginCustomerID, input.Language, input.Network, "", "", input.GeoTargets, nil, input.Keywords, input.Window, firstBody, "keyword_list", now)
	if err != nil {
		return plannerCollectionOutput{}, err
	}
	warnings := []string{}
	var collectionErr error
	failed := []string{}
	missing := []string{}
	_, accountIDs, err := plannerAccountPhase(ctx, client, store, snapshot.ID)
	if err != nil {
		failed = plannerAppendFailureID(failed, accountIDs)
		collectionErr = err
	}

	for batchNumber, batch := range batches {
		if collectionErr != nil {
			break
		}
		if input.MaxBatches > 0 && batchNumber >= input.MaxBatches {
			for missingNumber := batchNumber; missingNumber < len(batches); missingNumber++ {
				missing = append(missing, fmt.Sprintf("batch:%d", missingNumber))
			}
			collectionErr = fmt.Errorf("--max-batches stopped historical collection before batch %d; raise --max-batches or use zero for all batches", batchNumber)
			break
		}
		body, buildErr := buildPlannerHistoricalRequest(input, batch)
		if buildErr != nil {
			collectionErr = buildErr
			break
		}
		collector := &plannerReceiptCollector{store: store, snapshot: snapshot.ID}
		response, callErr := client.Call(ctx, http.MethodPost, mustPlannerCustomerPath(input.CustomerID, "historical"), body, func(response keywordapi.Response) error {
			return collector.callback(ctx, portfolio.EndpointHistorical, "", 0, batchNumber, body, batch, response)
		})
		if collector.err != nil {
			collectionErr = collector.err
			break
		}
		finalIDs, finalOK := collector.final(response)
		if normErr := collector.normalize(ctx); normErr != nil {
			collectionErr = normErr
		}
		if callErr != nil && collectionErr == nil {
			collectionErr = callErr
		}
		if !finalOK && collectionErr == nil {
			collectionErr = errors.New("historical response was not preserved")
		}
		if collectionErr != nil {
			failed = plannerAppendFailureID(failed, finalIDs)
			break
		}
	}

	if finishErr := finishPlannerSnapshot(ctx, store, snapshot.ID, collectionErr, warnings, failed, missing); finishErr != nil {
		if collectionErr == nil {
			collectionErr = finishErr
		} else {
			collectionErr = fmt.Errorf("%w; finalize snapshot: %v", collectionErr, finishErr)
		}
	}
	if collectionErr != nil {
		return plannerCollectionPartialView(ctx, store, snapshot.ID, input.OutputLimit, warnings), collectionErr
	}
	return plannerCollectionView(ctx, store, snapshot.ID, input.OutputLimit, warnings)
}

func mustPlannerCustomerPath(customerID, method string) string {
	path, err := plannerCustomerPath(customerID, method)
	if err != nil {
		return ""
	}
	return path
}

func plannerCollectionView(ctx context.Context, store *portfolio.Store, snapshotID string, limit int, warnings []string) (plannerCollectionOutput, error) {
	// CollectionSummary keeps the response bounded: it reads only the
	// requested normalized rows and obtains full keyword/month counts with SQL,
	// without loading every raw response body just to render a limited result.
	summary, err := store.CollectionSummary(ctx, snapshotID, limit)
	if err != nil {
		return plannerCollectionOutput{}, err
	}
	return plannerCollectionOutput{
		Snapshot:           summary.Snapshot,
		Rows:               summary.Rows,
		Coverage:           summary.Coverage,
		ReceiptCount:       summary.ReceiptCount,
		StoredKeywordCount: summary.StoredKeywordCount,
		StoredMonthlyCount: summary.StoredMonthlyCount,
		RenderedRowCount:   summary.RenderedRowCount,
		RenderLimit:        summary.RenderLimit,
		RowsTruncated:      summary.RowsTruncated,
		Warnings:           append([]string(nil), warnings...),
	}, nil
}

func plannerCollectionPartialView(ctx context.Context, store *portfolio.Store, snapshotID string, limit int, warnings []string) plannerCollectionOutput {
	view, err := plannerCollectionView(ctx, store, snapshotID, limit, warnings)
	if err == nil {
		return view
	}
	return plannerCollectionOutput{Snapshot: portfolio.Snapshot{ID: snapshotID}, Rows: []portfolio.Row{}, Coverage: []portfolio.CoverageView{}, Warnings: append([]string(nil), warnings...)}
}

func writePlannerCollectionOutput(cmd *cobra.Command, flags *rootFlags, value plannerCollectionOutput) error {
	if plannerWantsHumanTerminal(cmd.OutOrStdout(), flags) {
		return writePlannerCollectionHuman(cmd.OutOrStdout(), value)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode Planner collection output: %w", err)
	}
	return printPlannerEvidenceOutput(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"}, plannerCollectionFields)
}

func writePlannerCommandDryRun(cmd *cobra.Command, flags *rootFlags, result plannerDryRunResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode Planner dry-run output: %w", err)
	}
	return printPlannerEvidenceOutput(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"})
}

func plannerDryRunRequest(input any, body []byte) map[string]any {
	result := map[string]any{}
	encoded, err := json.Marshal(input)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.UseNumber()
		_ = decoder.Decode(&result)
	}
	endpoint := ""
	customerID := ""
	switch typed := input.(type) {
	case plannerIdeasInput:
		endpoint, customerID = "ideas", typed.CustomerID
	case plannerHistoricalInput:
		endpoint, customerID = "historical", typed.CustomerID
	}
	result["api_version"] = "v25"
	result["discovery_revision"] = "20260831"
	result["transport"] = "REST"
	result["method"] = http.MethodPost
	if endpoint != "" {
		if path, err := plannerCustomerPath(customerID, endpoint); err == nil {
			result["path"] = path
		} else {
			result["path"] = "unresolved until customer target is supplied"
		}
	}
	result["request_body"] = json.RawMessage(append([]byte(nil), body...))
	return result
}

func plannerHasCompleteIdeasDryRunInput(cmd *cobra.Command) bool {
	return cmd != nil && hasChangedLocalFlags(cmd)
}

func plannerHasCompleteHistoricalDryRunInput(cmd *cobra.Command) bool {
	return cmd != nil && hasChangedLocalFlags(cmd)
}

func plannerRateLimitSettings(value float64) (float64, time.Duration, error) {
	switch {
	case math.IsNaN(value) || math.IsInf(value, 0):
		return 0, 0, errors.New("--rate-limit must be a finite value")
	case value == pressclient.RateLimitAuto:
		// The Planner policy keeps the default auto mode at one request per
		// second until server headers can be considered. keywordapi receives a
		// concrete ceiling and cross-process floor for the same policy.
		return 1, time.Second, nil
	case value <= 0:
		return 0, 0, fmt.Errorf("--rate-limit must be auto or a positive value no greater than 1; got %v", value)
	case value > 1:
		return 0, 0, fmt.Errorf("--rate-limit must be auto or a positive value no greater than 1; got %v", value)
	default:
		nanos := float64(time.Second) / value
		// time.Duration is a signed int64. Reject a positive rate whose
		// pacing interval cannot be represented before converting the float;
		// an overflowing conversion can otherwise wrap and appear as a valid
		// one-second floor.
		if math.IsInf(nanos, 0) || nanos >= float64(1<<63) {
			return 0, 0, fmt.Errorf("--rate-limit produces an interval larger than the supported duration; got %v", value)
		}
		interval := time.Duration(math.Ceil(nanos))
		if interval < time.Second {
			interval = time.Second
		}
		return value, interval, nil
	}
}

func validatePlannerRuntimeFlags(flags *rootFlags) error {
	if flags == nil {
		return nil
	}
	if flags.timeoutExplicit && flags.timeout <= 0 {
		return errors.New("--timeout must be a positive duration; zero and negative values are not supported")
	}
	_, _, err := plannerRateLimitSettings(flags.rateLimit)
	return err
}

func plannerLiveConfig(envFile, requestedCustomer string, roots ...*rootFlags) (keywordapi.Config, error) {
	var root *rootFlags
	if len(roots) > 0 {
		root = roots[0]
	}
	var ratePerSecond float64
	var minRequestInterval time.Duration
	if root != nil {
		var err error
		ratePerSecond, minRequestInterval, err = plannerRateLimitSettings(root.rateLimit)
		if err != nil {
			return keywordapi.Config{}, err
		}
	}
	config, err := keywordapi.LoadConfigWithDefault(envFile, plannerEnvFilePath("", plannerRootHome(root)))
	if err != nil {
		return keywordapi.Config{}, err
	}
	customerID := strings.TrimSpace(requestedCustomer)
	if customerID == "" {
		customerID = strings.TrimSpace(config.CustomerID)
	}
	customerID, err = normalizePlannerCustomerID(customerID)
	if err != nil {
		return keywordapi.Config{}, err
	}
	if customerID == "" {
		return keywordapi.Config{}, errors.New("customer target is unresolved; supply --customer-id or GOOGLE_ADS_CUSTOMER_ID")
	}
	config.CustomerID = customerID
	if strings.TrimSpace(config.LoginCustomerID) != "" {
		loginID, loginErr := normalizePlannerCustomerID(config.LoginCustomerID)
		if loginErr != nil {
			return keywordapi.Config{}, fmt.Errorf("GOOGLE_ADS_LOGIN_CUSTOMER_ID: %w", loginErr)
		}
		config.LoginCustomerID = loginID
	}
	if root != nil {
		config.RatePerSecond = ratePerSecond
		config.MinRequestInterval = minRequestInterval
		if root.timeout > 0 {
			config.Timeout = root.timeout
		}
	}
	return config, nil
}

func plannerErrorForCLI(err error) error {
	if err == nil {
		return nil
	}
	var existing *cliError
	if errors.As(err, &existing) {
		return err
	}
	if keywordapi.CodeOf(err) == keywordapi.CodeConfigFile {
		return configErr(err)
	}
	code := keywordapi.ExitCodeOf(err)
	if code <= 0 {
		code = 5
	}
	return &cliError{code: code, err: err}
}

func newPlannerIdeasCmd(flags *rootFlags) *cobra.Command {
	var planner plannerIdeasFlags
	cmd := &cobra.Command{
		Use:   "ideas",
		Short: "Collect keyword ideas into the immutable local portfolio",
		Long:  `Collect Google Keyword Planner ideas with explicit targeting and a closed monthly range. Every response attempt is saved before decoding, and the normalised result is available offline through the portfolio commands. Use --limit only to reduce rendered rows; it never truncates collection.`,
		Example: strings.Trim(`
  keyword-planner ideas --language languageConstants/1000 --geo geoTargetConstants/2840 --seed "beef steak" --seed "ribeye steak" --seed "beef brisket" --seed "pork chops" --seed "meat cutting" --seed butchery --agent
  keyword-planner ideas --language languageConstants/1000 --geo geoTargetConstants/2840 --seed-file seeds.txt --agent
`, "\n"),
		SilenceUsage: true,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0",
			"pp:happy-args":       "--language=languageConstants/1000;--geo=geoTargetConstants/2840;--seed=beef steak;--seed=ribeye steak;--seed=beef brisket;--seed=pork chops;--seed=meat cutting;--seed=butchery;--page-size=10000;--limit=3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePlannerRuntimeFlags(flags); err != nil {
				return usageErr(err)
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("ideas accepts flags only; unexpected argument %q", args[0]))
			}
			if dryRunOK(flags) {
				if !plannerHasCompleteIdeasDryRunInput(cmd) {
					return writeDryRun(cmd.OutOrStdout(), flags, "ideas")
				}
				input, err := resolvePlannerIdeasFlags(planner, time.Now().UTC())
				if err != nil {
					return usageErr(err)
				}
				body, err := buildPlannerIdeasRequest(input, "")
				if err != nil {
					return usageErr(err)
				}
				result := plannerDryRunForIdeas(input)
				result.Request = plannerDryRunRequest(input, body)
				return writePlannerCommandDryRun(cmd, flags, result)
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			input, err := resolvePlannerIdeasFlags(planner, time.Now().UTC())
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			config, err := plannerLiveConfig(planner.envFile, input.CustomerID, flags)
			if err != nil {
				return plannerErrorForCLI(err)
			}
			input.CustomerID = config.CustomerID
			if cmd.Flags().Lookup("max-pages") != nil && input.MaxPages == 0 && isDogfoodPlannerRead() {
				input.MaxPages = 1
			}
			store, err := portfolio.Open(ctx, plannerPortfolioDBPath(planner.dbPath, plannerRootHome(flags)))
			if err != nil {
				return configErr(err)
			}
			defer store.Close()
			view, err := collectPlannerIdeasWithLogin(ctx, keywordapi.NewClient(config), store, input, config.LoginCustomerID, time.Now().UTC())
			if err != nil {
				return plannerErrorForCLI(err)
			}
			return writePlannerCollectionOutput(cmd, flags, view)
		},
	}
	bindPlannerIdeasFlags(cmd, &planner)
	return cmd
}

func newPlannerHistoricalCmd(flags *rootFlags) *cobra.Command {
	var planner plannerHistoricalFlags
	cmd := &cobra.Command{
		Use:   "historical",
		Short: "Collect historical keyword metrics into the immutable local portfolio",
		Long:  `Collect Google Keyword Planner historical metrics in ordered batches. Submitted terms, returned terms, close-variant linkage, nullable monthly values, and every raw response attempt are preserved for offline evidence queries.`,
		Example: strings.Trim(`
  keyword-planner historical --language languageConstants/1000 --geo geoTargetConstants/2840 --keyword "beef steak" --keyword "beef brisket" --agent
  keyword-planner historical --language languageConstants/1000 --geo geoTargetConstants/2840 --keyword-file keywords.txt --batch-size 10000 --agent
`, "\n"),
		SilenceUsage: true,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0",
			"pp:happy-args":       "--language=languageConstants/1000;--geo=geoTargetConstants/2840;--keyword=beef steak;--keyword=beef brisket;--batch-size=10000;--limit=3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePlannerRuntimeFlags(flags); err != nil {
				return usageErr(err)
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("historical accepts flags only; unexpected argument %q", args[0]))
			}
			if dryRunOK(flags) {
				if !plannerHasCompleteHistoricalDryRunInput(cmd) {
					return writeDryRun(cmd.OutOrStdout(), flags, "historical")
				}
				input, err := resolvePlannerHistoricalFlags(planner, time.Now().UTC())
				if err != nil {
					return usageErr(err)
				}
				batches, err := plannerKeywordBatches(input.Keywords, input.BatchSize)
				if err != nil {
					return usageErr(err)
				}
				body, err := buildPlannerHistoricalRequest(input, batches[0])
				if err != nil {
					return usageErr(err)
				}
				result := plannerDryRunForHistorical(input)
				result.Request = plannerDryRunRequest(input, body)
				return writePlannerCommandDryRun(cmd, flags, result)
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			input, err := resolvePlannerHistoricalFlags(planner, time.Now().UTC())
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			config, err := plannerLiveConfig(planner.envFile, input.CustomerID, flags)
			if err != nil {
				return plannerErrorForCLI(err)
			}
			input.CustomerID = config.CustomerID
			if isDogfoodPlannerRead() && (input.MaxBatches == 0 || input.MaxBatches > 1) {
				input.MaxBatches = 1
			}
			store, err := portfolio.Open(ctx, plannerPortfolioDBPath(planner.dbPath, plannerRootHome(flags)))
			if err != nil {
				return configErr(err)
			}
			defer store.Close()
			view, err := collectPlannerHistoricalWithLogin(ctx, keywordapi.NewClient(config), store, input, config.LoginCustomerID, time.Now().UTC())
			if err != nil {
				return plannerErrorForCLI(err)
			}
			return writePlannerCollectionOutput(cmd, flags, view)
		},
	}
	bindPlannerHistoricalFlags(cmd, &planner)
	return cmd
}

func isDogfoodPlannerRead() bool {
	return strings.TrimSpace(os.Getenv("PRINTING_PRESS_DOGFOOD")) == "1"
}
