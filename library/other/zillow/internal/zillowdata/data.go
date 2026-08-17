// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

// Package zillowdata loads and normalizes public Zillow Research CSV datasets.
package zillowdata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zillow/internal/cliutil"
)

const (
	DefaultBaseURL = "https://files.zillowstatic.com/research/public_csvs"
	maxCSVBytes    = 64 << 20
)

type Dataset struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Path    string `json:"path"`
	Unit    string `json:"unit"`
	Cadence string `json:"cadence"`
}

var datasets = []Dataset{
	{Key: "zhvi", Label: "Zillow Home Value Index", Path: "/zhvi/Metro_zhvi_uc_sfrcondo_tier_0.33_0.67_sm_sa_month.csv", Unit: "usd", Cadence: "monthly"},
	{Key: "zhvf", Label: "Zillow Home Value Forecast", Path: "/zhvf_growth/Metro_zhvf_growth_uc_sfrcondo_tier_0.33_0.67_sm_sa_month.csv", Unit: "percent", Cadence: "monthly"},
	{Key: "zori", Label: "Zillow Observed Rent Index", Path: "/zori/Metro_zori_uc_sfrcondomfr_sm_month.csv", Unit: "usd_per_month", Cadence: "monthly"},
	{Key: "inventory", Label: "For-Sale Inventory", Path: "/invt_fs/Metro_invt_fs_uc_sfrcondo_sm_month.csv", Unit: "listings", Cadence: "monthly"},
	{Key: "sales", Label: "Sales Count Nowcast", Path: "/sales_count_now/Metro_sales_count_now_uc_sfrcondo_month.csv", Unit: "sales", Cadence: "monthly"},
	{Key: "days_pending", Label: "Mean Days to Pending", Path: "/mean_doz_pending/Metro_mean_doz_pending_uc_sfrcondo_sm_month.csv", Unit: "days", Cadence: "monthly"},
	{Key: "market_temperature", Label: "Market Temperature Index", Path: "/market_temp_index/Metro_market_temp_index_uc_sfrcondo_month.csv", Unit: "index", Cadence: "monthly"},
	{Key: "homeowner_income", Label: "New Homeowner Income Needed", Path: "/new_homeowner_income_needed/Metro_new_homeowner_income_needed_downpayment_0.20_uc_sfrcondo_tier_0.33_0.67_sm_sa_month.csv", Unit: "usd_per_year", Cadence: "monthly"},
	{Key: "zordi", Label: "Zillow Observed Renter Demand Index", Path: "/zordi/Metro_zordi_uc_sfrcondomfr_month.csv", Unit: "index", Cadence: "monthly"},
	{Key: "sale_to_list", Label: "Mean Sale-to-List Ratio", Path: "/mean_sale_to_list/Metro_mean_sale_to_list_uc_sfrcondo_sm_month.csv", Unit: "ratio", Cadence: "monthly"},
	{Key: "price_cut_share", Label: "Share of Listings With a Price Cut", Path: "/perc_listings_price_cut/Metro_perc_listings_price_cut_uc_sfrcondo_sm_month.csv", Unit: "percent", Cadence: "monthly"},
	{Key: "total_monthly_payment", Label: "Total Monthly Payment", Path: "/total_monthly_payment/Metro_total_monthly_payment_downpayment_0.20_uc_sfrcondo_tier_0.33_0.67_sm_sa_month.csv", Unit: "usd_per_month", Cadence: "monthly"},
	{Key: "zhvi_bottom_tier", Label: "ZHVI Bottom Tier", Path: "/zhvi/Metro_zhvi_uc_sfrcondo_tier_0.0_0.33_sm_sa_month.csv", Unit: "usd", Cadence: "monthly"},
	{Key: "zhvi_top_tier", Label: "ZHVI Top Tier", Path: "/zhvi/Metro_zhvi_uc_sfrcondo_tier_0.67_1.0_sm_sa_month.csv", Unit: "usd", Cadence: "monthly"},
	{Key: "new_con_sales", Label: "New-Construction Sales", Path: "/new_con_sales_count_raw/Metro_new_con_sales_count_raw_uc_sfrcondo_month.csv", Unit: "sales", Cadence: "monthly"},
	{Key: "new_con_price", Label: "New-Construction Median Sale Price", Path: "/new_con_median_sale_price/Metro_new_con_median_sale_price_uc_sfrcondo_month.csv", Unit: "usd", Cadence: "monthly"},
	{Key: "new_con_price_per_sqft", Label: "New-Construction Median Sale Price per Square Foot", Path: "/new_con_median_sale_price_per_sqft/Metro_new_con_median_sale_price_per_sqft_uc_sfrcondo_month.csv", Unit: "usd_per_sqft", Cadence: "monthly"},
}

func Datasets() []Dataset {
	out := make([]Dataset, len(datasets))
	copy(out, datasets)
	return out
}

func DatasetByKey(key string) (Dataset, bool) {
	key = NormalizeMetric(key)
	for _, d := range datasets {
		if d.Key == key {
			return d, true
		}
	}
	return Dataset{}, false
}

func NormalizeMetric(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.NewReplacer("-", "_", " ", "_").Replace(key)
	aliases := map[string]string{
		"home_values":       "zhvi",
		"rents":             "zori",
		"rent_demand":       "zordi",
		"price_cuts":        "price_cut_share",
		"days_to_pending":   "days_pending",
		"market_heat":       "market_temperature",
		"income_needed":     "homeowner_income",
		"monthly_payment":   "total_monthly_payment",
		"bottom_tier":       "zhvi_bottom_tier",
		"top_tier":          "zhvi_top_tier",
		"new_build_price":   "new_con_price",
		"new_build_sales":   "new_con_sales",
		"new_build_ppsf":    "new_con_price_per_sqft",
		"sale_to_list_mean": "sale_to_list",
	}
	if v, ok := aliases[key]; ok {
		return v
	}
	return key
}

type Row struct {
	RegionID   int64
	SizeRank   int
	RegionName string
	RegionType string
	StateName  string
	Values     map[time.Time]float64
}

func (r Row) DisplayName() string {
	if r.StateName == "" || strings.Contains(strings.ToLower(r.RegionName), strings.ToLower(r.StateName)) {
		return r.RegionName
	}
	return r.RegionName + ", " + r.StateName
}

func (r Row) Latest() (time.Time, float64, bool) {
	var latest time.Time
	var value float64
	for date, v := range r.Values {
		if date.After(latest) {
			latest, value = date, v
		}
	}
	return latest, value, !latest.IsZero()
}

func (r Row) ValueAt(date time.Time) (float64, bool) {
	v, ok := r.Values[date]
	return v, ok
}

func (r Row) SortedDates() []time.Time {
	out := make([]time.Time, 0, len(r.Values))
	for d := range r.Values {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func (r Row) ChangeMonths(months int) (float64, time.Time, time.Time, bool) {
	if months <= 0 {
		return 0, time.Time{}, time.Time{}, false
	}
	dates := r.SortedDates()
	if len(dates) <= months {
		return 0, time.Time{}, time.Time{}, false
	}
	end := dates[len(dates)-1]
	start := dates[len(dates)-1-months]
	startValue, endValue := r.Values[start], r.Values[end]
	if startValue == 0 {
		return 0, time.Time{}, time.Time{}, false
	}
	return (endValue/startValue - 1) * 100, start, end, true
}

type Table struct {
	Dataset      Dataset
	Rows         []Row
	SourceURL    string
	Source       string
	FetchedAt    time.Time
	ETag         string
	LastModified string
	SHA256       string
}

func (t *Table) ResolveRegion(query string) (Row, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Row{}, errors.New("region is required")
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		for _, row := range t.Rows {
			if row.RegionID == id {
				return row, nil
			}
		}
		return Row{}, fmt.Errorf("region id %d not found in %s", id, t.Dataset.Key)
	}
	lower := strings.ToLower(query)
	var exact, partial []Row
	for _, row := range t.Rows {
		name := strings.ToLower(row.RegionName)
		display := strings.ToLower(row.DisplayName())
		if lower == name || lower == display {
			exact = append(exact, row)
		} else if strings.Contains(display, lower) {
			partial = append(partial, row)
		}
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = partial
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return Row{}, fmt.Errorf("region %q not found in %s", query, t.Dataset.Key)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].SizeRank == candidates[j].SizeRank {
			return candidates[i].DisplayName() < candidates[j].DisplayName()
		}
		return candidates[i].SizeRank < candidates[j].SizeRank
	})
	names := make([]string, 0, min(5, len(candidates)))
	for _, row := range candidates[:min(5, len(candidates))] {
		names = append(names, fmt.Sprintf("%s (%d)", row.DisplayName(), row.RegionID))
	}
	return Row{}, fmt.Errorf("region %q is ambiguous: %s", query, strings.Join(names, "; "))
}

type cacheMeta struct {
	Dataset      string    `json:"dataset"`
	SourceURL    string    `json:"source_url"`
	FetchedAt    time.Time `json:"fetched_at"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	SHA256       string    `json:"sha256"`
}

type Loader struct {
	BaseURL    string
	CacheDir   string
	HTTPClient *http.Client
	MaxAge     time.Duration
	Limiter    *cliutil.AdaptiveLimiter
}

func (l Loader) Load(ctx context.Context, metric, mode string) (*Table, error) {
	dataset, ok := DatasetByKey(metric)
	if !ok {
		return nil, fmt.Errorf("unknown metric %q", metric)
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "auto", "live", "local":
	default:
		return nil, fmt.Errorf("invalid data source %q", mode)
	}
	cachePath := filepath.Join(l.CacheDir, "datasets", dataset.Key+".csv")
	if mode != "live" {
		if table, err := l.loadCache(dataset, cachePath); err == nil {
			if mode == "local" || l.MaxAge <= 0 || time.Since(table.FetchedAt) <= l.MaxAge {
				return table, nil
			}
		} else if mode == "local" {
			return nil, fmt.Errorf("local dataset %s unavailable: %w", dataset.Key, err)
		}
	}
	table, body, meta, err := l.fetch(ctx, dataset)
	if err != nil {
		if mode == "auto" {
			if cached, cacheErr := l.loadCache(dataset, cachePath); cacheErr == nil {
				cached.Source = "stale-cache"
				return cached, nil
			}
		}
		return nil, err
	}
	if err := writeAtomic(cachePath, body, 0o600); err != nil {
		return nil, fmt.Errorf("writing dataset cache: %w", err)
	}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := writeAtomic(cachePath+".meta.json", append(metaBytes, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("writing dataset metadata: %w", err)
	}
	return table, nil
}

func (l Loader) fetch(ctx context.Context, dataset Dataset) (*Table, []byte, cacheMeta, error) {
	l.Limiter.Wait()
	baseURL := strings.TrimRight(l.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	sourceURL := baseURL + dataset.Path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, nil, cacheMeta{}, err
	}
	req.Header.Set("Accept", "text/csv,*/*;q=0.8")
	req.Header.Set("User-Agent", "zillow-pp-cli/0.2.0")
	httpClient := l.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, cacheMeta{}, fmt.Errorf("fetching %s: %w", dataset.Key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		l.Limiter.OnRateLimit()
		retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
		if retryAfter == "" {
			retryAfter = "unspecified"
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, nil, cacheMeta{}, fmt.Errorf("fetching %s: HTTP 429 rate limited (Retry-After: %s)", dataset.Key, retryAfter)
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, nil, cacheMeta{}, fmt.Errorf("fetching %s: HTTP %d", dataset.Key, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCSVBytes+1))
	if err != nil {
		return nil, nil, cacheMeta{}, fmt.Errorf("reading %s: %w", dataset.Key, err)
	}
	if len(body) > maxCSVBytes {
		return nil, nil, cacheMeta{}, fmt.Errorf("%s exceeds %d-byte safety limit", dataset.Key, maxCSVBytes)
	}
	sum := sha256.Sum256(body)
	fetchedAt := time.Now().UTC()
	table, err := Parse(dataset, bytes.NewReader(body))
	if err != nil {
		return nil, nil, cacheMeta{}, fmt.Errorf("parsing %s: %w", dataset.Key, err)
	}
	table.SourceURL = sourceURL
	table.Source = "live"
	table.FetchedAt = fetchedAt
	table.ETag = resp.Header.Get("ETag")
	table.LastModified = resp.Header.Get("Last-Modified")
	table.SHA256 = hex.EncodeToString(sum[:])
	l.Limiter.OnSuccess()
	meta := cacheMeta{
		Dataset: dataset.Key, SourceURL: sourceURL, FetchedAt: fetchedAt,
		ETag: table.ETag, LastModified: table.LastModified, SHA256: table.SHA256,
	}
	return table, body, meta, nil
}

func (l Loader) loadCache(dataset Dataset, path string) (*Table, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	table, err := Parse(dataset, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var meta cacheMeta
	if metaBytes, metaErr := os.ReadFile(path + ".meta.json"); metaErr == nil {
		_ = json.Unmarshal(metaBytes, &meta)
	}
	if meta.FetchedAt.IsZero() {
		if info, statErr := os.Stat(path); statErr == nil {
			meta.FetchedAt = info.ModTime().UTC()
		}
	}
	table.SourceURL = meta.SourceURL
	if table.SourceURL == "" {
		table.SourceURL = strings.TrimRight(l.BaseURL, "/") + dataset.Path
	}
	table.Source = "cache"
	table.FetchedAt = meta.FetchedAt
	table.ETag = meta.ETag
	table.LastModified = meta.LastModified
	table.SHA256 = meta.SHA256
	return table, nil
}

func Parse(dataset Dataset, reader io.Reader) (*Table, error) {
	r := csv.NewReader(reader)
	r.FieldsPerRecord = -1
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	index := make(map[string]int, len(headers))
	var dateColumns []struct {
		index int
		date  time.Time
	}
	for i, header := range headers {
		header = strings.TrimSpace(header)
		index[header] = i
		if date, parseErr := time.Parse("2006-01-02", header); parseErr == nil {
			dateColumns = append(dateColumns, struct {
				index int
				date  time.Time
			}{i, date})
		}
	}
	if len(dateColumns) == 0 {
		return nil, errors.New("no YYYY-MM-DD observation columns")
	}
	regionIDIndex, ok := index["RegionID"]
	if !ok {
		return nil, errors.New("missing RegionID column")
	}
	regionNameIndex, ok := index["RegionName"]
	if !ok {
		return nil, errors.New("missing RegionName column")
	}
	var rows []Row
	for {
		record, readErr := r.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("reading row: %w", readErr)
		}
		if regionIDIndex >= len(record) || regionNameIndex >= len(record) {
			continue
		}
		regionID, parseErr := strconv.ParseInt(strings.TrimSpace(record[regionIDIndex]), 10, 64)
		if parseErr != nil {
			continue
		}
		row := Row{
			RegionID: regionID, RegionName: strings.TrimSpace(record[regionNameIndex]),
			RegionType: field(record, columnIndex(index, "RegionType")), StateName: field(record, columnIndex(index, "StateName")),
			Values: make(map[time.Time]float64, len(dateColumns)),
		}
		if sizeRank := field(record, columnIndex(index, "SizeRank")); sizeRank != "" {
			row.SizeRank, _ = strconv.Atoi(sizeRank)
		}
		for _, column := range dateColumns {
			if column.index >= len(record) {
				continue
			}
			text := strings.TrimSpace(record[column.index])
			if text == "" {
				continue
			}
			value, valueErr := strconv.ParseFloat(text, 64)
			if valueErr == nil {
				row.Values[column.date] = value
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, errors.New("no region rows")
	}
	return &Table{Dataset: dataset, Rows: rows}, nil
}

func columnIndex(index map[string]int, name string) int {
	if value, ok := index[name]; ok {
		return value
	}
	return -1
}

func field(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".zillow-cache-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	// ponytail: cache is disposable; direct replacement avoids a backup file.
	_ = os.Remove(path)
	return os.Rename(tempPath, path)
}
