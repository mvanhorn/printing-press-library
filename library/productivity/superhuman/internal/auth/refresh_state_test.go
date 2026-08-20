// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"testing"
	"time"
)

func TestClassifyRefreshStateOK(t *testing.T) {
	now := time.Unix(2000, 0)
	got := ClassifyRefreshStateAt(AccountTokens{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expires:      now.Add(time.Hour).UnixMilli(),
		SuperhumanToken: SuperhumanToken{
			Token:   "id",
			Expires: now.Add(time.Hour).UnixMilli(),
		},
	}, now)
	if got.State != RefreshStateOK {
		t.Fatalf("state = %s, want %s", got.State, RefreshStateOK)
	}
	if got.AccessTokenState != "valid" || got.SuperhumanTokenState != "valid" {
		t.Fatalf("unexpected token states: %#v", got)
	}
}

func TestClassifyRefreshStateExpiredAccessCanRefresh(t *testing.T) {
	now := time.Unix(2000, 0)
	got := ClassifyRefreshStateAt(AccountTokens{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expires:      now.Add(-time.Minute).UnixMilli(),
		SuperhumanToken: SuperhumanToken{
			Token:   "id",
			Expires: now.Add(time.Hour).UnixMilli(),
		},
	}, now)
	if got.State != RefreshStateExpiredAccessCanRefresh {
		t.Fatalf("state = %s, want %s", got.State, RefreshStateExpiredAccessCanRefresh)
	}
	if got.Hint == "" {
		t.Fatal("expected recovery hint")
	}
}

func TestClassifyRefreshStateExpiredRefreshNeedsRelogin(t *testing.T) {
	now := time.Unix(2000, 0)
	got := ClassifyRefreshStateAt(AccountTokens{
		AccessToken: "access",
		Expires:     now.Add(time.Hour).UnixMilli(),
		SuperhumanToken: SuperhumanToken{
			Token:   "id",
			Expires: now.Add(-time.Minute).UnixMilli(),
		},
	}, now)
	if got.State != RefreshStateExpiredRefreshNeedsRelogin {
		t.Fatalf("state = %s, want %s", got.State, RefreshStateExpiredRefreshNeedsRelogin)
	}
	if got.HasRefreshToken {
		t.Fatal("expected missing refresh token")
	}
}

func TestClassifyRefreshStateUnknownExpiry(t *testing.T) {
	got := ClassifyRefreshStateAt(AccountTokens{}, time.Unix(2000, 0))
	if got.State != RefreshStateOK {
		t.Fatalf("state = %s, want %s", got.State, RefreshStateOK)
	}
	if got.AccessTokenState != "unknown" || got.SuperhumanTokenState != "unknown" {
		t.Fatalf("unexpected token states: %#v", got)
	}
}
