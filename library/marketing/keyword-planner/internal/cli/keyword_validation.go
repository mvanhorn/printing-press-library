// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	plannerNetworkGoogleSearch = "GOOGLE_SEARCH"

	maxPlannerSeeds = 20
	// Google applies the historical keyword cap per request. The collection
	// input may be larger because the CLI splits it into ordered batches.
	maxPlannerKeywords      = 10000
	maxPlannerInputKeywords = 1000000
	maxPlannerGeos          = 10
	// Keep line-file input bounded while still reading one byte beyond the
	// budget so an oversized file can never look like a successful truncation.
	maxPlannerFileBytes = 8 << 20

	// Google permits up to 10,000 ideas per page and historical keywords per
	// request. Use those caps by default so a normal collection does not spend
	// quota on needless extra pages/batches; callers can lower either value.
	defaultPlannerPageSize  = 10000
	defaultPlannerBatchSize = 10000

	// A zero page/batch budget means that collection continues until the API
	// has no continuation token. It is an output-independent collection cap.
	defaultPlannerMaxPages    = 0
	defaultPlannerMaxBatches  = 0
	defaultPlannerOutputLimit = 0
)

var plannerResourceNamePattern = regexp.MustCompile(`^(languageConstants|geoTargetConstants)/[1-9][0-9]*$`)
var plannerCustomerIDPattern = regexp.MustCompile(`^[0-9]+(?:-[0-9]+)*$`)

// plannerWindow is the closed-month interval sent to the Planner request.
// The API accepts no open month; the CLI validates this before any I/O.
type plannerWindow struct {
	Start string `json:"start_month"`
	End   string `json:"end_month"`
}

type plannerTargetFlags struct {
	customerID     string
	language       string
	geoTargets     []string
	allGeographies bool
	network        string
	includeAdult   bool
	start          string
	end            string
}

type plannerIdeasFlags struct {
	plannerTargetFlags
	seeds       []string
	seedFile    string
	dbPath      string
	envFile     string
	pageSize    int
	maxPages    int
	outputLimit int
}

type plannerHistoricalFlags struct {
	plannerTargetFlags
	keywords    []string
	keywordFile string
	dbPath      string
	envFile     string
	batchSize   int
	maxBatches  int
	outputLimit int
}

type plannerTarget struct {
	CustomerID     string        `json:"customer_id,omitempty"`
	Language       string        `json:"language"`
	GeoTargets     []string      `json:"geo_target_constants"`
	AllGeographies bool          `json:"all_geographies"`
	Network        string        `json:"network"`
	IncludeAdult   bool          `json:"include_adult_keywords"`
	Window         plannerWindow `json:"window"`
}

type plannerIdeasInput struct {
	plannerTarget
	Seeds       []string `json:"seeds"`
	PageSize    int      `json:"page_size"`
	MaxPages    int      `json:"max_pages"`
	OutputLimit int      `json:"output_limit"`
	Warnings    []string `json:"warnings,omitempty"`
}

type plannerHistoricalInput struct {
	plannerTarget
	Keywords    []string `json:"keywords"`
	BatchSize   int      `json:"batch_size"`
	MaxBatches  int      `json:"max_batches"`
	OutputLimit int      `json:"output_limit"`
}

// plannerDryRunResult is deliberately limited to non-secret request
// material. It may include a customer ID because that is a target identifier,
// but never includes OAuth material, access tokens, or request headers.
type plannerDryRunResult struct {
	DryRun            bool   `json:"dry_run"`
	Action            string `json:"action"`
	Request           any    `json:"request"`
	CustomerTarget    string `json:"customer_target"`
	UnresolvedMessage string `json:"unresolved_customer_target,omitempty"`
}

func bindPlannerTargetFlags(cmd *cobra.Command, flags *plannerTargetFlags) {
	if cmd == nil || flags == nil {
		return
	}
	cmd.Flags().StringVar(&flags.customerID, "customer-id", "", "Operating Google Ads customer ID (hyphens are accepted)")
	cmd.Flags().BoolVar(&flags.allGeographies, "all-geographies", false, "Explicitly acknowledge an all-geographies request")
	cmd.Flags().StringVar(&flags.network, "network", plannerNetworkGoogleSearch, "Google Ads network (GOOGLE_SEARCH)")
	cmd.Flags().BoolVar(&flags.includeAdult, "include-adult-keywords", false, "Include adult keyword ideas when supported")
	cmd.Flags().StringVar(&flags.start, "start", "", "First closed month, YYYY-MM")
	cmd.Flags().StringVar(&flags.end, "end", "", "Last closed month, YYYY-MM")
}

func bindPlannerIdeasFlags(cmd *cobra.Command, flags *plannerIdeasFlags) {
	if cmd == nil || flags == nil {
		return
	}
	cmd.Flags().StringVar(&flags.language, "language", "", "Language resource, for example languageConstants/1000")
	cmd.Flags().StringArrayVar(&flags.geoTargets, "geo", nil, "Geo target resource; repeat for multiple targets")
	bindPlannerTargetFlags(cmd, &flags.plannerTargetFlags)
	cmd.Flags().StringArrayVar(&flags.seeds, "seed", nil, "Seed keyword; repeat for multiple seeds")
	cmd.Flags().StringVar(&flags.seedFile, "seed-file", "", "File containing one seed keyword per line")
	cmd.Flags().StringVar(&flags.dbPath, "db", "", "Portfolio SQLite path (env KEYWORD_PLANNER_DB is used when omitted)")
	cmd.Flags().StringVar(&flags.envFile, "env-file", "", "Google Ads env file (env KEYWORD_PLANNER_ENV_FILE is used when omitted)")
	cmd.Flags().IntVar(&flags.pageSize, "page-size", defaultPlannerPageSize, "Maximum results requested per API page")
	cmd.Flags().IntVar(&flags.maxPages, "max-pages", defaultPlannerMaxPages, "Maximum API pages to collect; zero means all pages")
	cmd.Flags().IntVar(&flags.outputLimit, "limit", defaultPlannerOutputLimit, "Maximum rows to render after collection; zero means all rows")
}

func bindPlannerHistoricalFlags(cmd *cobra.Command, flags *plannerHistoricalFlags) {
	if cmd == nil || flags == nil {
		return
	}
	cmd.Flags().StringVar(&flags.language, "language", "", "Language resource, for example languageConstants/1000")
	cmd.Flags().StringArrayVar(&flags.geoTargets, "geo", nil, "Geo target resource; repeat for multiple targets")
	bindPlannerTargetFlags(cmd, &flags.plannerTargetFlags)
	cmd.Flags().StringArrayVar(&flags.keywords, "keyword", nil, "Keyword to measure; repeat for multiple keywords")
	cmd.Flags().StringVar(&flags.keywordFile, "keyword-file", "", "File containing one keyword per line")
	cmd.Flags().StringVar(&flags.dbPath, "db", "", "Portfolio SQLite path (env KEYWORD_PLANNER_DB is used when omitted)")
	cmd.Flags().StringVar(&flags.envFile, "env-file", "", "Google Ads env file (env KEYWORD_PLANNER_ENV_FILE is used when omitted)")
	cmd.Flags().IntVar(&flags.batchSize, "batch-size", defaultPlannerBatchSize, "Keywords per historical-metrics request")
	cmd.Flags().IntVar(&flags.maxBatches, "max-batches", defaultPlannerMaxBatches, "Maximum API batches to collect; zero means all batches")
	cmd.Flags().IntVar(&flags.outputLimit, "limit", defaultPlannerOutputLimit, "Maximum rows to render after collection; zero means all rows")
}

// resolvePlannerIdeasFlags validates all scalar request material before it
// reads a seed file. This ordering makes malformed resource names and ranges
// fail without touching the filesystem.
func resolvePlannerIdeasFlags(flags plannerIdeasFlags, now time.Time) (plannerIdeasInput, error) {
	target, err := validatePlannerTarget(flags.plannerTargetFlags, now)
	if err != nil {
		return plannerIdeasInput{}, err
	}
	if err := validatePlannerPageSize(flags.pageSize); err != nil {
		return plannerIdeasInput{}, err
	}
	if err := validatePlannerBudget(flags.maxPages, "max-pages"); err != nil {
		return plannerIdeasInput{}, err
	}
	if err := validatePlannerOutputLimit(flags.outputLimit); err != nil {
		return plannerIdeasInput{}, err
	}
	seeds, err := loadPlannerLines(flags.seeds, flags.seedFile, "seed", maxPlannerSeeds)
	if err != nil {
		return plannerIdeasInput{}, err
	}
	warnings, err := validatePlannerSeeds(seeds)
	if err != nil {
		return plannerIdeasInput{}, err
	}
	return plannerIdeasInput{
		plannerTarget: target,
		Seeds:         seeds,
		PageSize:      flags.pageSize,
		MaxPages:      flags.maxPages,
		OutputLimit:   flags.outputLimit,
		Warnings:      warnings,
	}, nil
}

// resolvePlannerHistoricalFlags has the same preflight ordering as ideas,
// while retaining the caller's keyword order for deterministic batching.
func resolvePlannerHistoricalFlags(flags plannerHistoricalFlags, now time.Time) (plannerHistoricalInput, error) {
	target, err := validatePlannerTarget(flags.plannerTargetFlags, now)
	if err != nil {
		return plannerHistoricalInput{}, err
	}
	if err := validatePlannerBatchSize(flags.batchSize); err != nil {
		return plannerHistoricalInput{}, err
	}
	if err := validatePlannerBudget(flags.maxBatches, "max-batches"); err != nil {
		return plannerHistoricalInput{}, err
	}
	if err := validatePlannerOutputLimit(flags.outputLimit); err != nil {
		return plannerHistoricalInput{}, err
	}
	keywords, err := loadPlannerLines(flags.keywords, flags.keywordFile, "keyword", maxPlannerInputKeywords)
	if err != nil {
		return plannerHistoricalInput{}, err
	}
	if err := validatePlannerKeywords(keywords); err != nil {
		return plannerHistoricalInput{}, err
	}
	return plannerHistoricalInput{
		plannerTarget: target,
		Keywords:      keywords,
		BatchSize:     flags.batchSize,
		MaxBatches:    flags.maxBatches,
		OutputLimit:   flags.outputLimit,
	}, nil
}

func validatePlannerTarget(flags plannerTargetFlags, now time.Time) (plannerTarget, error) {
	customerID, err := normalizePlannerCustomerID(flags.customerID)
	if err != nil {
		return plannerTarget{}, err
	}
	language, err := validatePlannerResourceName("language", flags.language, "languageConstants")
	if err != nil {
		return plannerTarget{}, err
	}
	geos, allGeographies, err := validatePlannerGeoTargets(flags.geoTargets, flags.allGeographies)
	if err != nil {
		return plannerTarget{}, err
	}
	network, err := validatePlannerNetwork(flags.network)
	if err != nil {
		return plannerTarget{}, err
	}
	window, err := resolvePlannerWindow(flags.start, flags.end, now)
	if err != nil {
		return plannerTarget{}, err
	}
	return plannerTarget{
		CustomerID:     customerID,
		Language:       language,
		GeoTargets:     geos,
		AllGeographies: allGeographies,
		Network:        network,
		IncludeAdult:   flags.includeAdult,
		Window:         window,
	}, nil
}

func loadPlannerLines(values []string, filePath, label string, max int) ([]string, error) {
	if len(values) > 0 && strings.TrimSpace(filePath) != "" {
		return nil, fmt.Errorf("%s and --%s-file cannot be used together", label, label)
	}
	if len(values) > 0 {
		result := make([]string, len(values))
		for i, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, fmt.Errorf("%s at index %d is empty", label, i)
			}
			result[i] = value
		}
		if len(result) > max {
			return nil, fmt.Errorf("too many %s values: got %d, maximum %d", label, len(result), max)
		}
		return result, nil
	}
	if strings.TrimSpace(filePath) == "" {
		return []string{}, nil
	}

	file, err := os.Open(filepath.Clean(filePath)) // #nosec G304 -- --seed-file/--keyword-file is an explicit operator-selected local input path.
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", label, err)
	}
	defer file.Close()

	if info, err := file.Stat(); err != nil {
		return nil, fmt.Errorf("stat %s file: %w", label, err)
	} else if info.Mode().IsRegular() && info.Size() > maxPlannerFileBytes {
		return nil, fmt.Errorf("%s file exceeds the maximum size of %d bytes", label, maxPlannerFileBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPlannerFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", label, err)
	}
	if int64(len(data)) > maxPlannerFileBytes {
		return nil, fmt.Errorf("%s file exceeds the maximum size of %d bytes", label, maxPlannerFileBytes)
	}

	result := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Scanner's default token budget is 64 KiB. Planner files are line based,
	// so allow one valid line to consume the complete bounded file.
	scanner.Buffer(make([]byte, 64*1024), maxPlannerFileBytes+1)
	for scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if value == "" {
			continue
		}
		result = append(result, value)
		if len(result) > max {
			return nil, fmt.Errorf("too many %s values in file: maximum %d", label, max)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s file: %w", label, err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s file contains no non-empty values", label)
	}
	return result, nil
}

func validatePlannerSeeds(seeds []string) ([]string, error) {
	if len(seeds) < 1 || len(seeds) > maxPlannerSeeds {
		return nil, fmt.Errorf("seed count must be between 1 and %d; got %d", maxPlannerSeeds, len(seeds))
	}
	if len(seeds) < 6 || len(seeds) > 8 {
		return []string{"seed_count_outside_suggested_6_to_8"}, nil
	}
	return []string{}, nil
}

func validatePlannerKeywords(keywords []string) error {
	if len(keywords) < 1 || len(keywords) > maxPlannerInputKeywords {
		return fmt.Errorf("keyword count must be between 1 and %d; got %d", maxPlannerInputKeywords, len(keywords))
	}
	return nil
}

func validatePlannerResourceName(kind, value, prefix string) (string, error) {
	value = strings.TrimSpace(value)
	if !plannerResourceNamePattern.MatchString(value) || !strings.HasPrefix(value, prefix+"/") {
		return "", fmt.Errorf("%s must be a resource such as %s/1000", kind, prefix)
	}
	identifier := strings.TrimPrefix(value, prefix+"/")
	if _, err := strconv.ParseUint(identifier, 10, 64); err != nil {
		return "", fmt.Errorf("%s resource identifier must be a positive decimal", kind)
	}
	return value, nil
}

func validatePlannerGeoTargets(values []string, allGeographies bool) ([]string, bool, error) {
	if allGeographies && len(values) > 0 {
		return nil, false, errors.New("--all-geographies cannot be combined with --geo")
	}
	if !allGeographies && len(values) == 0 {
		return nil, false, errors.New("provide --geo at least once or explicitly pass --all-geographies")
	}
	if allGeographies {
		return []string{}, true, nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		value, err := validatePlannerResourceName("geo", value, "geoTargetConstants")
		if err != nil {
			return nil, false, fmt.Errorf("geo at index %d: %w", i, err)
		}
		if _, ok := seen[value]; ok {
			return nil, false, fmt.Errorf("duplicate geo target %q", value)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) > maxPlannerGeos {
		return nil, false, fmt.Errorf("geo target count must be at most %d; got %d", maxPlannerGeos, len(result))
	}
	sort.Strings(result)
	return result, false, nil
}

func validatePlannerNetwork(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return plannerNetworkGoogleSearch, nil
	}
	if value != plannerNetworkGoogleSearch {
		return "", fmt.Errorf("network must be %s; got %q", plannerNetworkGoogleSearch, value)
	}
	return value, nil
}

func normalizePlannerCustomerID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !plannerCustomerIDPattern.MatchString(value) {
		return "", errors.New("customer-id must contain only decimal digits with optional hyphens")
	}
	return strings.ReplaceAll(value, "-", ""), nil
}

func validatePlannerPageSize(value int) error {
	if value < 1 || value > 10000 {
		return fmt.Errorf("page-size must be between 1 and 10000; got %d", value)
	}
	return nil
}

func validatePlannerBatchSize(value int) error {
	if value < 1 || value > maxPlannerKeywords {
		return fmt.Errorf("batch-size must be between 1 and %d; got %d", maxPlannerKeywords, value)
	}
	return nil
}

func validatePlannerBudget(value int, name string) error {
	if value < 0 {
		return fmt.Errorf("%s cannot be negative; got %d", name, value)
	}
	return nil
}

func validatePlannerOutputLimit(value int) error {
	if value < 0 {
		return fmt.Errorf("limit cannot be negative; got %d", value)
	}
	return nil
}

func resolvePlannerWindow(start, end string, now time.Time) (plannerWindow, error) {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if (start == "") != (end == "") {
		return plannerWindow{}, errors.New("--start and --end must be supplied together")
	}
	if start == "" {
		current := plannerMonthFromTime(now)
		last := plannerMonthAdd(current, -1)
		first := plannerMonthAdd(last, -11)
		return plannerWindow{Start: first.String(), End: last.String()}, nil
	}
	from, err := parsePlannerMonth(start)
	if err != nil {
		return plannerWindow{}, err
	}
	to, err := parsePlannerMonth(end)
	if err != nil {
		return plannerWindow{}, err
	}
	if from.after(to) {
		return plannerWindow{}, fmt.Errorf("start month %q is after end month %q", start, end)
	}
	if !to.before(plannerMonthFromTime(now)) {
		return plannerWindow{}, fmt.Errorf("end month %q is current or future; only closed months are allowed", end)
	}
	return plannerWindow{Start: from.String(), End: to.String()}, nil
}

type plannerMonth struct {
	year  int
	month int
}

func parsePlannerMonth(value string) (plannerMonth, error) {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[4] != '-' {
		return plannerMonth{}, fmt.Errorf("month %q must use YYYY-MM", value)
	}
	for i, r := range value {
		if i == 4 {
			continue
		}
		if r < '0' || r > '9' {
			return plannerMonth{}, fmt.Errorf("month %q must use YYYY-MM", value)
		}
	}
	year, yearErr := strconv.Atoi(value[:4])
	month, monthErr := strconv.Atoi(value[5:])
	if yearErr != nil || monthErr != nil || year < 1 || month < 1 || month > 12 {
		return plannerMonth{}, fmt.Errorf("month %q is invalid", value)
	}
	return plannerMonth{year: year, month: month}, nil
}

func (m plannerMonth) String() string { return fmt.Sprintf("%04d-%02d", m.year, m.month) }

func (m plannerMonth) before(other plannerMonth) bool {
	return m.year < other.year || (m.year == other.year && m.month < other.month)
}

func (m plannerMonth) after(other plannerMonth) bool {
	return other.before(m)
}

func plannerMonthFromTime(now time.Time) plannerMonth {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	return plannerMonth{year: now.Year(), month: int(now.Month())}
}

func plannerMonthAdd(value plannerMonth, delta int) plannerMonth {
	total := value.year*12 + value.month - 1 + delta
	return plannerMonth{year: total / 12, month: total%12 + 1}
}

func plannerDryRunForIdeas(input plannerIdeasInput) plannerDryRunResult {
	return plannerDryRunResult{
		DryRun:            true,
		Action:            "ideas",
		Request:           input,
		CustomerTarget:    plannerCustomerTarget(input.CustomerID),
		UnresolvedMessage: plannerUnresolvedCustomerMessage(input.CustomerID),
	}
}

func plannerDryRunForHistorical(input plannerHistoricalInput) plannerDryRunResult {
	return plannerDryRunResult{
		DryRun:            true,
		Action:            "historical",
		Request:           input,
		CustomerTarget:    plannerCustomerTarget(input.CustomerID),
		UnresolvedMessage: plannerUnresolvedCustomerMessage(input.CustomerID),
	}
}

func plannerCustomerTarget(customerID string) string {
	if strings.TrimSpace(customerID) == "" {
		return "unresolved"
	}
	return "resolved"
}

func plannerUnresolvedCustomerMessage(customerID string) string {
	if strings.TrimSpace(customerID) == "" {
		return "customer target is unresolved; supply --customer-id or the approved nonsecret binding before a live call"
	}
	return ""
}

func writePlannerDryRun(w io.Writer, flags *rootFlags, result plannerDryRunResult) error {
	if flags != nil && flags.asJSON {
		return json.NewEncoder(w).Encode(result)
	}
	_, err := fmt.Fprintf(w, "dry-run: %s request validated; customer target %s\n", result.Action, result.CustomerTarget)
	if result.UnresolvedMessage != "" {
		_, _ = fmt.Fprintf(w, "note: %s\n", result.UnresolvedMessage)
	}
	return err
}
