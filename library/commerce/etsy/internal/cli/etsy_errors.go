package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/platform"
)

type etsyDiagnosis struct {
	Kind string
	Fix  string
}

var etsyScopeToken = regexp.MustCompile(`\b[a-z]+_[rwd]\b`)

// classifyEtsyError maps known Etsy failures to actionable, secret-free guidance.
func classifyEtsyError(status int, body string, retryAfter time.Duration) (etsyDiagnosis, bool) {
	lower := strings.ToLower(body)
	switch {
	case status == 403 && strings.Contains(lower, "should be in the format"):
		return etsyDiagnosis{"api_key_format", "Etsy requires x-api-key as keystring:shared_secret. Set ETSY_SHARED_SECRET to your app's shared secret (ETSY_API_KEY stays the keystring)."}, true
	case status == 403 && (strings.Contains(lower, "not found or not active") || strings.Contains(lower, "incorrect shared secret")):
		return etsyDiagnosis{"api_key_inactive", "Check the app is approved/active in your Etsy developer dashboard and that ETSY_API_KEY (keystring) and ETSY_SHARED_SECRET (shared secret) are not swapped."}, true
	case status == 403 && strings.Contains(lower, "scope"):
		scopes := make([]string, 0)
		seen := make(map[string]bool)
		for _, scope := range etsyScopeToken.FindAllString(lower, -1) {
			if !seen[scope] {
				scopes = append(scopes, scope)
				seen[scope] = true
			}
		}
		if len(scopes) == 0 {
			return etsyDiagnosis{"missing_scope", "Your OAuth token lacks a required scope for this endpoint; re-run `etsy-pp-cli auth login` with the scope listed in the Etsy docs for it (see `auth login --help`)."}, true
		}
		return etsyDiagnosis{"missing_scope", fmt.Sprintf("Your OAuth token lacks the required scope(s): %s. Re-run `etsy-pp-cli auth login --scopes <existing scopes>,%s`.", strings.Join(scopes, ", "), strings.Join(scopes, ","))}, true
	case status == 429:
		fix := "Etsy rate limit hit (5 requests/second, 5000/day per app)."
		if retryAfter > 0 {
			fix += fmt.Sprintf(" Retry after %s.", retryAfter.Round(time.Second))
		} else {
			fix += " Check `etsy-pp-cli doctor` for the remaining daily budget."
		}
		return etsyDiagnosis{"rate_limited", fix}, true
	default:
		return etsyDiagnosis{}, false
	}
}

func etsyErrorHint(err error) string {
	var apiError *client.APIError
	var limited *platform.RateLimitedError
	status, body := 0, ""
	var retryAfter time.Duration
	if errors.As(err, &apiError) {
		status, body = apiError.StatusCode, apiError.Body
	}
	if errors.As(err, &limited) {
		status, retryAfter = 429, limited.RetryAfter
	}
	if diagnosis, ok := classifyEtsyError(status, body, retryAfter); ok {
		return "hint: " + diagnosis.Fix
	}
	return ""
}
