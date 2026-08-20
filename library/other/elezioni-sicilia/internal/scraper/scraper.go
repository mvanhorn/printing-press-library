// Package scraper fetches and parses HTML pages from www.elezioni.regione.sicilia.it.
// The site uses ISO-8859-15 encoding and a self-signed TLS certificate.
package scraper

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
)

const (
	BaseURL2026 = "https://www.elezioni.regione.sicilia.it"
)

// ArchiveURL returns the base URL for a given election year.
// 2026 uses HTTPS; older years use HTTP.
func ArchiveURL(anno int) string {
	switch anno {
	case 2026:
		return "https://www.elezioni.regione.sicilia.it"
	case 2025:
		return "http://www.elezioni.regione.sicilia.it/comunali2025"
	case 2020:
		return "http://www.elezioni.regione.sicilia.it/comunali2020"
	default:
		return fmt.Sprintf("http://www.elezioni.regione.sicilia.it/comunali%d/primoTurno", anno)
	}
}

// KnownYears lists the communal election years available on the site.
var KnownYears = []int{2026, 2025, 2024, 2023, 2022, 2021, 2020, 2019, 2018, 2017, 2016, 2015, 2014, 2013, 2012, 2011, 2010, 2009}

// Province lists the 9 Sicilian provinces in the site.
var Province = []string{"AG", "CL", "CT", "EN", "ME", "PA", "RG", "SR", "TP"}

var defaultClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // site uses self-signed cert
	},
}

// limiter paces outbound requests at 2 req/s and adapts on 429 responses.
// The site does not publish rate limits; 2 req/s is a conservative default.
var limiter = cliutil.NewAdaptiveLimiter(2)

// Fetch retrieves a URL and returns the body decoded from ISO-8859-15 to UTF-8.
// It enforces a per-process rate limit (2 req/s) and retries once on HTTP 429.
func Fetch(url string) (string, error) {
	return fetchWithRetry(url, 1)
}

func fetchWithRetry(url string, retriesLeft int) (string, error) {
	// Pace requests using the adaptive rate limiter.
	limiter.Wait()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; elezioni-sicilia-pp-cli)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := defaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Handle rate limiting: back off and retry once.
	if resp.StatusCode == http.StatusTooManyRequests {
		limiter.OnRateLimit()
		if retriesLeft > 0 {
			wait := cliutil.RetryAfter(resp)
			time.Sleep(wait)
			return fetchWithRetry(url, retriesLeft-1)
		}
		return "", &cliutil.RateLimitError{URL: url, RetryAfter: cliutil.RetryAfter(resp)}
	}

	if resp.StatusCode >= 400 {
		limiter.OnSuccess() // non-429 error still counts as a valid response
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	limiter.OnSuccess()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return decodeLatin1(raw), nil
}

// decodeLatin1 converts a Latin-1 / ISO-8859-15 byte slice to a UTF-8 string.
// For the characters that appear in Italian electoral data, ISO-8859-15 and ISO-8859-1
// (Latin-1) are identical, so we use the simple rune(byte) cast.
func decodeLatin1(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

// IsUnavailable returns true when the page body contains the "not available" message
// that the server emits for pages that are currently offline (e.g. regionali2022).
func IsUnavailable(body string) bool {
	return strings.Contains(body, "non é al momento disponibile") ||
		strings.Contains(body, "non è al momento disponibile")
}

// ScrutinioState describes the state of vote counting for a comune.
type ScrutinioState string

const (
	ScrutinioInCorso  ScrutinioState = "in_corso"
	ScrutinioParziale ScrutinioState = "parziale"
	ScrutinioCompleto ScrutinioState = "completo"
)

// ExtractScrutinioState detects the scrutinio state from a report page body.
func ExtractScrutinioState(body string) (ScrutinioState, string) {
	switch {
	case strings.Contains(body, "scrutinio completo"):
		return ScrutinioCompleto, ""
	case strings.Contains(body, "scrutinio parziale"):
		// Extract "N su M sezioni"
		detail := extractScrutinioParziale(body)
		return ScrutinioParziale, detail
	case strings.Contains(body, "gli scrutini sono ancora in corso"):
		return ScrutinioInCorso, ""
	default:
		return ScrutinioInCorso, ""
	}
}

// parzRe matches "n. 56 su n. 57 sezioni"
var parzRe = regexp.MustCompile(`n\.\s*(\d+)\s+su\s+n\.\s*(\d+)\s+sezioni`)

func extractScrutinioParziale(body string) string {
	m := parzRe.FindStringSubmatch(body)
	if len(m) < 3 {
		return ""
	}
	return fmt.Sprintf("%s/%s sezioni", m[1], m[2])
}
