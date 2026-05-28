// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import "time"

// RefreshState is the coarse account credential state surfaced by doctor.
type RefreshState string

const (
	RefreshStateOK                         RefreshState = "ok"
	RefreshStateExpiredAccessCanRefresh    RefreshState = "expired_access_can_refresh"
	RefreshStateExpiredRefreshNeedsRelogin RefreshState = "expired_refresh_needs_relogin"
)

// RefreshStateReport is safe to expose in JSON diagnostics. It intentionally
// reports timestamps and booleans, not token values.
type RefreshStateReport struct {
	State                    RefreshState `json:"state"`
	AccessTokenState         string       `json:"access_token_state"`
	SuperhumanTokenState     string       `json:"superhuman_token_state"`
	HasRefreshToken          bool         `json:"has_refresh_token"`
	AccessTokenExpiresAt     int64        `json:"access_token_expires_at,omitempty"`
	SuperhumanTokenExpiresAt int64        `json:"superhuman_token_expires_at,omitempty"`
	Hint                     string       `json:"hint,omitempty"`
}

// ClassifyRefreshState returns the current refresh state using time.Now.
func ClassifyRefreshState(account AccountTokens) RefreshStateReport {
	return ClassifyRefreshStateAt(account, time.Now())
}

// ClassifyRefreshStateAt is the deterministic form used by tests.
func ClassifyRefreshStateAt(account AccountTokens, now time.Time) RefreshStateReport {
	report := RefreshStateReport{
		State:                RefreshStateOK,
		AccessTokenState:     tokenExpiryState(account.Expires, now),
		SuperhumanTokenState: tokenExpiryState(account.SuperhumanToken.Expires, now),
		HasRefreshToken:      account.RefreshToken != "",
	}
	if account.Expires > 0 {
		report.AccessTokenExpiresAt = account.Expires
	}
	if account.SuperhumanToken.Expires > 0 {
		report.SuperhumanTokenExpiresAt = account.SuperhumanToken.Expires
	}

	accessExpired := report.AccessTokenState == "expired"
	superhumanExpired := report.SuperhumanTokenState == "expired"
	if accessExpired || superhumanExpired {
		if report.HasRefreshToken {
			report.State = RefreshStateExpiredAccessCanRefresh
			report.Hint = "automatic refresh can recover; if it fails, run 'auth login --chrome'"
		} else {
			report.State = RefreshStateExpiredRefreshNeedsRelogin
			report.Hint = "run 'auth login --chrome' to re-authenticate"
		}
	}
	return report
}

func tokenExpiryState(expiresMs int64, now time.Time) string {
	if expiresMs <= 0 {
		return "unknown"
	}
	if !time.UnixMilli(expiresMs).After(now) {
		return "expired"
	}
	return "valid"
}
