// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored support for the novel archive commands (series, latest, workbooks).

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"kakao-transparency-pp-cli/internal/client"
	"kakao-transparency-pp-cli/internal/cliutil"
)

// kakaoArchiveStartYear is the first half-year Kakao published (1H 2012).
const kakaoArchiveStartYear = 2012

// flexString absorbs the API's language-dependent typing: count fields arrive
// as strings in the Korean payload ("517") and as bare numbers in the English
// payload (517). Either way the value lands as its string form.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexString(n.String())
	return nil
}

type kakaoReportEnvelope struct {
	Success bool            `json:"success"`
	Data    kakaoReportData `json:"data"`
}

type kakaoReportData struct {
	Year       string           `json:"year"`
	HalfYear   string           `json:"halfYear"`
	HalfYearID int              `json:"halfYearId"`
	Title      string           `json:"title"`
	Content    string           `json:"content"`
	FileURL    string           `json:"fileUrl"`
	EnFileURL  string           `json:"enFileUrl"`
	PrevYn     int              `json:"prevYn"`
	NextYn     int              `json:"nextYn"`
	Reports    []kakaoReportRow `json:"reports"`
}

type kakaoReportRow struct {
	ServiceCorp       string     `json:"serviceCorp"`
	Category          string     `json:"category"`
	EnCategory        string     `json:"enCategory"`
	NumberOfRequests  flexString `json:"numberOfRequests"`
	NumberOfProcesses flexString `json:"numberOfProcesses"`
	NumberOfAccounts  flexString `json:"numberOfAccounts"`
}

// fetchKakaoReport loads one half-year report in English. The API answers
// unpublished periods with its HTML error page (observed as both HTTP 200 and
// HTTP 500), so any HTTP-status error or a body that does not parse as the
// JSON envelope is reported as "not published" (ok=false), not an error.
// Transient server faults on published periods are absorbed by the client's
// own retry pass before they can be misread here.
func fetchKakaoReport(ctx context.Context, c *client.Client, year, half int) (kakaoReportData, bool, error) {
	return fetchKakaoReportLang(ctx, c, year, half, "en")
}

// fetchKakaoReportLang is fetchKakaoReport with an explicit Accept-Language.
// The localized fields (title, content, category, fileUrl) follow the
// negotiated language; workbooks fetches "ko" so fileUrl stays the Korean
// edition while enFileUrl carries the English one.
func fetchKakaoReportLang(ctx context.Context, c *client.Client, year, half int, lang string) (kakaoReportData, bool, error) {
	path := "/api/transparency/{year}/{halfYearId}"
	path = replacePathParam(path, "year", formatCLIParamValue(year))
	path = replacePathParam(path, "halfYearId", formatCLIParamValue(half))
	raw, err := c.GetWithHeaders(ctx, path, nil, map[string]string{"Accept-Language": lang})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return kakaoReportData{}, false, nil
		}
		return kakaoReportData{}, false, err
	}
	var env kakaoReportEnvelope
	if json.Unmarshal(raw, &env) != nil || !env.Success {
		return kakaoReportData{}, false, nil
	}
	return env.Data, true, nil
}

// walkKakaoArchive visits every published half-year from (year, half) forward,
// following each report's own nextYn cursor so the walk never requests an
// unpublished period (which would cost the client's full server-error retry
// pass). The visit callback returns false to stop early.
func walkKakaoArchive(ctx context.Context, c *client.Client, year, half int, visit func(kakaoReportData) bool) error {
	return walkKakaoArchiveLang(ctx, c, year, half, "en", visit)
}

// walkKakaoArchiveLang is walkKakaoArchive with an explicit Accept-Language.
func walkKakaoArchiveLang(ctx context.Context, c *client.Client, year, half int, lang string, visit func(kakaoReportData) bool) error {
	// Hard bound: the archive cannot extend past the wall clock's year plus
	// one half-year of publishing lag, whatever nextYn claims.
	lastYear := time.Now().Year() + 1
	for year <= lastYear {
		data, ok, err := fetchKakaoReportLang(ctx, c, year, half, lang)
		if err != nil {
			return err
		}
		if ok {
			if !visit(data) {
				return nil
			}
			if data.NextYn == 0 {
				return nil
			}
		}
		if half == 1 {
			half = 2
		} else {
			half = 1
			year++
		}
	}
	return nil
}

// kakaoSeedPeriod returns a period that is essentially guaranteed to be
// published already: the first half of last year (reports publish with about
// a half-year lag). latest starts here and follows nextYn forward.
func kakaoSeedPeriod() (int, int) {
	return time.Now().Year() - 1, 1
}

// kakaoServiceSlug maps the API's Korean service-corporation names to the
// stable slugs the --service filter accepts.
func kakaoServiceSlug(serviceCorp string) string {
	switch strings.TrimSpace(serviceCorp) {
	case "카카오":
		return "kakao"
	case "다음":
		return "daum"
	default:
		return strings.ToLower(strings.TrimSpace(serviceCorp))
	}
}

// halfLabel renders a half-year id as the compact 1H/2H label used in output rows.
func halfLabel(half int) string {
	if half == 2 {
		return "2H"
	}
	return "1H"
}

// kakaoSeriesStart clamps the --since flag to the archive start. Live dogfood
// runs under a flat per-command timeout, so the walk starts at last year there
// instead of covering the full archive.
func kakaoSeriesStart(since int) int {
	first := since
	if first < kakaoArchiveStartYear {
		first = kakaoArchiveStartYear
	}
	now := time.Now().Year()
	if cliutil.IsDogfoodEnv() && first < now-1 {
		first = now - 1
	}
	if first > now {
		first = now
	}
	return first
}
