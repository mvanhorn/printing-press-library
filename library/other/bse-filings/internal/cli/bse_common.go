// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/store"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// openBSEStore opens the local store and guarantees the hand-built BSE tables
// exist. Every novel command that reads or writes holdings / concall_chunks /
// results_outcomes goes through here so the schema self-heals on a fresh DB.
func openBSEStore(cmd *cobra.Command) (*store.Store, error) {
	s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("bse-filings-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	if err := s.EnsureBSETables(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// holdingScripSet returns the set of holding scrip codes for membership tests.
func holdingScripSet(s *store.Store) (map[string]bool, error) {
	holdings, err := s.ListHoldings()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(holdings))
	for _, h := range holdings {
		set[h.ScripCode] = true
	}
	return set, nil
}

// bseAttachUserAgent matches the browser UA the generated client already sends.
// The corpfiling attachment host (www.bseindia.com) is outside the API client's
// base URL, so PDF fetches build their own request and must carry the same
// Referer + UA or the host returns an error page.
const bseAttachUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// fetchFilingPDF downloads a corporate-filing attachment by its ATTACHMENTNAME.
// Recent filings live under AttachLive/, older ones under AttachHis/; we try
// Live first and fall back to His. Returns the PDF bytes, or an error when
// neither location yields a PDF. pp:client-call — real HTTP request to the
// BSE corpfiling host.
func fetchFilingPDF(attachment string, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	bases := []string{
		"https://www.bseindia.com/xml-data/corpfiling/AttachLive/",
		"https://www.bseindia.com/xml-data/corpfiling/AttachHis/",
	}
	var lastErr error
	for _, base := range bases {
		body, ct, err := getPDF(httpClient, base+attachment)
		if err != nil {
			lastErr = err
			continue
		}
		if strings.Contains(ct, "pdf") || (len(body) > 4 && string(body[:4]) == "%PDF") {
			return body, nil
		}
		lastErr = fmt.Errorf("attachment %s at %s returned non-PDF content-type %q", attachment, base, ct)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("attachment %s not found", attachment)
	}
	return nil, lastErr
}

func getPDF(httpClient *http.Client, url string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", bseAttachUserAgent)
	req.Header.Set("Referer", "https://www.bseindia.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64 MB ceiling
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Header.Get("Content-Type"), fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// ymd formats a time as the YYYYMMDD string BSE date-window params expect.
func ymd(t time.Time) string { return t.Format("20060102") }

// requireNumericScrip rejects a scrip-code argument that is not all digits.
// BSE scrip codes are always numeric (e.g. 500325); validating up front turns
// a typo or a name passed where a code belongs into an actionable usage error
// (exit 2) instead of a confusing empty live-API response.
func requireNumericScrip(scrip string) error {
	scrip = strings.TrimSpace(scrip)
	if scrip == "" {
		return usageErr(fmt.Errorf("scrip code is required"))
	}
	for _, r := range scrip {
		if r < '0' || r > '9' {
			return usageErr(fmt.Errorf("invalid scrip code %q: BSE scrip codes are numeric (e.g. 500325) — use 'scrips <name>' to look one up", scrip))
		}
	}
	return nil
}

// parseDaysFlag reads a day-count flag that accepts either a bare integer
// ("7") or a duration-style "Nd"/"Nw" string ("7d", "2w"), matching the
// "--since 365d" convention used by concall-grep and cross. Hours are
// rounded up to whole days. Keeps --within and --no-filing-since consistent
// with the rest of the CLI's window flags.
func parseDaysFlag(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	// Bare integer is the simplest case.
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	cutoff, err := parseSinceDuration(s)
	if err != nil {
		return 0, fmt.Errorf("expected a day count like 7 or a window like 7d/2w, got %q", s)
	}
	// parseSinceDuration returns a past cutoff (now - duration), so time.Since
	// is the positive span; round to whole days.
	days := int(time.Since(cutoff).Hours()/24 + 0.5)
	if days < 0 {
		days = -days
	}
	return days, nil
}
