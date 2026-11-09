// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestValidateKlaviyoSourceProfileRequiresAccountAndOrganization(t *testing.T) {
	valid := platform.SourceProfile{CredentialRef: "op://Vault/Item/token", ExpectedAccountID: "account-1", ExpectedOrgName: "Acme"}
	if err := validateKlaviyoSourceProfile(valid); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []platform.SourceProfile{
		{ExpectedAccountID: valid.ExpectedAccountID, ExpectedOrgName: valid.ExpectedOrgName},
		{CredentialRef: valid.CredentialRef, ExpectedOrgName: valid.ExpectedOrgName},
		{CredentialRef: valid.CredentialRef, ExpectedAccountID: valid.ExpectedAccountID},
	} {
		if err := validateKlaviyoSourceProfile(invalid); err == nil {
			t.Fatalf("accepted invalid Klaviyo source: %#v", invalid)
		}
	}
}

func TestKlaviyoIdentityAdapterUsesLiveUncachedAccountProbe(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/accounts" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Klaviyo-API-Key private-key" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"acct-42","attributes":{"contact_information":{"organization_name":"Acme"}}}]}`))
	}))
	defer server.Close()
	adapter := klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()}
	observed, err := adapter.ProbeIdentity(context.Background(), platform.ResolvedCredentials{"credential": []byte("private-key")}, platform.SourceProfile{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || observed.AccountID != "acct-42" || observed.OrgName != "Acme" {
		t.Fatalf("unexpected probe: calls=%d observed=%+v", calls, observed)
	}
}

func TestKlaviyoIdentityAdapterFailureFixtures(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		body    string
		outcome platform.GateOutcome
	}{
		{"401", 401, `{}`, platform.GateInvalidCredentials}, {"403", 403, `{}`, platform.GateInsufficientScope},
		{"404", 404, `{}`, platform.GateIndeterminate}, {"429", 429, `{}`, platform.GateRateLimited},
		{"500", 500, `{}`, platform.GateUnavailable}, {"malformed", 200, `{}`, platform.GateIndeterminate},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			_, err := (klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()}).ProbeIdentity(context.Background(), platform.ResolvedCredentials{"credential": []byte("key")}, platform.SourceProfile{})
			var failure *platform.ProbeFailure
			if !errors.As(err, &failure) || failure.Outcome != test.outcome {
				t.Fatalf("failure = %#v", err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	_, err := (klaviyoIdentityAdapter{baseURL: server.URL, httpClient: &http.Client{Timeout: time.Millisecond}}).ProbeIdentity(context.Background(), platform.ResolvedCredentials{"credential": []byte("key")}, platform.SourceProfile{})
	var failure *platform.ProbeFailure
	if !errors.As(err, &failure) || failure.Outcome != platform.GateUnavailable {
		t.Fatalf("timeout = %#v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = (klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()}).ProbeIdentity(ctx, platform.ResolvedCredentials{"credential": []byte("key")}, platform.SourceProfile{})
	if !errors.As(err, &failure) || failure.Outcome != platform.GateUnavailable {
		t.Fatalf("cancellation = %#v", err)
	}
}
