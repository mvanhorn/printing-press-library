// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/locker"

	"github.com/spf13/cobra"
)

func nowJST() time.Time { return time.Now().In(locker.JST) }

func newLockerClient(flags *rootFlags) *locker.Client {
	return locker.New(flags.timeout)
}

// lockerMeta is the shared envelope metadata.
type lockerMeta struct {
	Command       string              `json:"command"`
	Query         map[string]any      `json:"query"`
	FetchedAt     string              `json:"fetched_at"`
	SourceURLs    []string            `json:"source_urls"`
	RequestCount  int                 `json:"request_count"`
	Scanned       *int                `json:"scanned_records,omitempty"`
	Returned      *int                `json:"returned,omitempty"`
	SourceTotal   *int                `json:"source_total,omitempty"`
	MoreAtSource  *bool               `json:"more_at_source,omitempty"`
	Excluded      map[string]int      `json:"excluded,omitempty"`
	Notes         []string            `json:"notes"`
	FetchFailures []map[string]string `json:"fetch_failures,omitempty"`
}

func newLockerMeta(command string, c *locker.Client, fetchedAt time.Time) lockerMeta {
	return lockerMeta{
		Command:      command,
		Query:        map[string]any{},
		FetchedAt:    fetchedAt.Format(time.RFC3339),
		SourceURLs:   c.SourceURLs(),
		RequestCount: c.Requests(),
		Notes:        []string{},
	}
}

func intPtr(n int) *int    { return &n }
func boolPtr(b bool) *bool { return &b }

// classifyLockerErr maps source errors to typed exit codes and adds a hint
// that says what to do next.
func classifyLockerErr(what string, err error) error {
	if err == nil {
		return nil
	}
	var rl *cliutil.RateLimitError
	var he *locker.HTTPError
	var ne net.Error
	switch {
	case errors.As(err, &rl):
		return rateLimitErr(fmt.Errorf("%s: %w\nhint: wait a minute before you try again; this CLI already sends about 1 request per second to each host", what, err))
	case errors.Is(err, locker.ErrNotFound):
		return notFoundErr(fmt.Errorf("%s: %w\nhint: check the locker id; 'near <station>' lists the current ids", what, err))
	case errors.As(err, &he) && he.Status == 403:
		return rateLimitErr(fmt.Errorf("%s: %w (the source refused the request)\nhint: wait a minute before you try again; this CLI already sends about 1 request per second to each host", what, err))
	case errors.As(err, &he) && he.Status >= 500:
		return apiErr(fmt.Errorf("%s: %w\nhint: the source failed on its side; retry later", what, err))
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()):
		return apiErr(fmt.Errorf("%s: %w\nhint: the source did not answer within --timeout; retry, or set a longer --timeout (Multi Ekicube can take 10 s or more)", what, err))
	case errors.As(err, &ne):
		return apiErr(fmt.Errorf("%s: %w\nhint: check your network connection, then run 'coinlocker-navi-pp-cli doctor'", what, err))
	}
	return apiErr(fmt.Errorf("%s: %w", what, err))
}

// parseLatLon reads --lat/--lon; both or neither must be set.
func parseLatLon(cmd *cobra.Command, latS, lonS string) (*float64, *float64, error) {
	latSet, lonSet := cmd.Flags().Changed("lat"), cmd.Flags().Changed("lon")
	if !latSet && !lonSet {
		return nil, nil, nil
	}
	if latSet != lonSet {
		return nil, nil, fmt.Errorf("--lat and --lon must be used together")
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(latS), 64)
	if err != nil || lat < 20 || lat > 46 {
		return nil, nil, fmt.Errorf("--lat %q: want a latitude in Japan (20 to 46)", latS)
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(lonS), 64)
	if err != nil || lon < 122 || lon > 154 {
		return nil, nil, fmt.Errorf("--lon %q: want a longitude in Japan (122 to 154)", lonS)
	}
	return &lat, &lon, nil
}

func normSize(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return "", nil
	}
	if locker.SizeRank(s) == 0 {
		return "", fmt.Errorf("--size %q: want S, M, L or XL", s)
	}
	return s, nil
}

func normGate(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "inside", "outside":
		return s, nil
	}
	return "", fmt.Errorf("--gate %q: want inside or outside", s)
}

func normDate(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return nowJST().Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", fmt.Errorf("--date %q: want YYYY-MM-DD", s)
	}
	return s, nil
}

func strOr(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

// validateLimitPages checks the shared --limit (1-100) and --max-pages (1-10) ranges.
func validateLimitPages(limit, maxPages int) error {
	if limit < 1 || limit > 100 {
		return fmt.Errorf("--limit %d: want 1 to 100", limit)
	}
	if maxPages < 1 || maxPages > 10 {
		return fmt.Errorf("--max-pages %d: want 1 to 10", maxPages)
	}
	return nil
}

// defaultEkicubeDates fills the Multi Ekicube from_at/end_at pair: an empty
// from_at becomes today in JST, and an empty end_at follows from_at.
func defaultEkicubeDates(from, end string, now time.Time) (string, string) {
	from, end = strings.TrimSpace(from), strings.TrimSpace(end)
	if from == "" {
		from = now.In(locker.JST).Format("2006-01-02")
	}
	if end == "" {
		end = from
	}
	return from, end
}
