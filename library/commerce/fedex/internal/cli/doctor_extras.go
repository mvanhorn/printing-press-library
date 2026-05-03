// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

// runFedExDoctorExtras appends FedEx-specific health checks to the doctor
// report after the generic checks have run. Each entry is added under a
// "fedex_*" key so the standard renderer surfaces them as info lines.
func runFedExDoctorExtras(report map[string]any, cfg *config.Config, flags *rootFlags) {
	report["fedex_env"] = classifyFedExBaseURL(cfg)
	report["fedex_account_format"] = checkFedExAccountFormat(os.Getenv("FEDEX_ACCOUNT_NUMBER"))
	report["fedex_token_probe"] = probeFedExTokenEndpoint(cfg)
	report["fedex_bag_reminder"] = "Production users: register your label with FedEx Bar Code Analysis Group (BAG) before shipping high volume."
	report["fedex_store"] = checkFedExStore()
}

// classifyFedExBaseURL emits a sandbox/prod hint based on the configured base
// URL. Production usage without the explicit prod URL is the most common foot
// gun for SMB shippers — we surface a warning when the base URL points at the
// production host so the user is reminded to confirm.
func classifyFedExBaseURL(cfg *config.Config) string {
	if cfg == nil {
		return "unknown"
	}
	url := strings.ToLower(cfg.BaseURL)
	switch {
	case strings.Contains(url, "sandbox"):
		return "sandbox (apis-sandbox.fedex.com)"
	case strings.Contains(url, "apis.fedex.com"):
		return "PRODUCTION — confirm before shipping (live charges, real labels)"
	case url == "":
		return "not configured"
	default:
		return "custom: " + cfg.BaseURL
	}
}

// checkFedExAccountFormat returns "ok" when the env-var account number is the
// canonical 9-digit FedEx format. Empty input is reported as informational.
func checkFedExAccountFormat(acct string) string {
	if acct == "" {
		return "FEDEX_ACCOUNT_NUMBER not set"
	}
	if len(acct) != 9 {
		return fmt.Sprintf("suspicious: %d chars (expected 9 digits)", len(acct))
	}
	for _, r := range acct {
		if r < '0' || r > '9' {
			return "suspicious: contains non-digit characters"
		}
	}
	return "ok (9 digits)"
}

// probeFedExTokenEndpoint POSTs deliberately invalid client_credentials so we
// can verify the OAuth endpoint is reachable even without real credentials. A
// 4xx response (typically 401 from FedEx for bad client) means the endpoint is
// alive; a transport error means we couldn't reach it at all.
func probeFedExTokenEndpoint(cfg *config.Config) string {
	if cfg == nil || cfg.BaseURL == "" {
		return "skipped (no base URL)"
	}
	tokenURL := strings.TrimRight(cfg.BaseURL, "/") + "/oauth/token"
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"fedex-pp-cli-doctor-probe"},
		"client_secret": {"invalid"},
	}
	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "error: " + err.Error()
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "unreachable: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 600 {
		return fmt.Sprintf("reachable (HTTP %d)", resp.StatusCode)
	}
	return fmt.Sprintf("unexpected status %d", resp.StatusCode)
}

func checkFedExStore() string {
	st, err := store.Open("")
	if err != nil {
		return "not initialized: " + err.Error()
	}
	defer st.Close()
	return "ok (" + st.Path() + ")"
}
