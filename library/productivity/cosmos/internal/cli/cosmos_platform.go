// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Cosmos tenant adapter preserved across Printing Press regenerations.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/cosmos/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/cosmos/internal/platform"
)

type cosmosEnvironmentResolver struct{}

func (cosmosEnvironmentResolver) Resolve(_ context.Context, reference string) ([]byte, error) {
	name := strings.TrimPrefix(strings.TrimSpace(reference), "env://")
	name = strings.TrimPrefix(name, "env:")
	if name != "COSMOS_TOKEN" {
		return nil, fmt.Errorf("unsupported Cosmos credential reference; use env:COSMOS_TOKEN")
	}
	value := os.Getenv(name)
	if value == "" {
		return nil, fmt.Errorf("credential environment variable %s is empty", name)
	}
	return []byte(value), nil
}

type cosmosIdentityAdapter struct{}

func (cosmosIdentityAdapter) EndpointClass() string { return "POST /graphql query GetMe" }

func (cosmosIdentityAdapter) ProbeIdentity(ctx context.Context, credentials platform.ResolvedCredentials, source platform.SourceProfile) (platform.ObservedIdentity, error) {
	token := credentials["credential"]
	if len(token) == 0 {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateInvalidCredentials, Err: fmt.Errorf("Cosmos credential is empty")}
	}
	baseURL := strings.TrimRight(source.ExpectedBaseURL, "/")
	body, _ := json.Marshal(map[string]any{"operationName": "GetMe", "query": cosmosGetMeQuery, "variables": map[string]any{}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/graphql?q=GetMe", bytes.NewReader(body))
	if err != nil {
		return platform.ObservedIdentity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-client-name", "cosmos-web")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateUnavailable, Err: fmt.Errorf("Cosmos identity probe failed: %w", err)}
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateUnavailable, Err: fmt.Errorf("read Cosmos identity response: %w", err)}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateInvalidCredentials, Err: fmt.Errorf("Cosmos rejected the selected credential")}
	}
	if resp.StatusCode == http.StatusForbidden {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateInsufficientScope, Err: fmt.Errorf("Cosmos credential cannot read the account identity")}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateUnavailable, Err: fmt.Errorf("Cosmos identity endpoint returned HTTP %d", resp.StatusCode)}
	}
	var envelope cosmosGraphQLEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateIndeterminate, Err: fmt.Errorf("decode Cosmos identity response: %w", err)}
	}
	if len(envelope.Errors) > 0 || len(envelope.Data) == 0 {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateIndeterminate, Err: fmt.Errorf("Cosmos identity response did not contain account data")}
	}
	accountID := idString(mapAt(envelope.Data, "me")["id"])
	if accountID == "" || accountID == "<nil>" {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateIndeterminate, Err: fmt.Errorf("Cosmos identity response omitted account id")}
	}
	return platform.ObservedIdentity{AccountID: accountID, BaseURL: baseURL}, nil
}

func validateCosmosSourceProfile(source platform.SourceProfile) error {
	if source.CredentialRef == "" {
		return fmt.Errorf("Cosmos client profile requires --credential-ref env:COSMOS_TOKEN")
	}
	if len(source.AdditionalCredentialRefs) > 0 || source.UsernameRef != "" {
		return fmt.Errorf("Cosmos client profile accepts only one bearer-token credential reference")
	}
	if strings.TrimSpace(source.ExpectedAccountID) == "" {
		return fmt.Errorf("Cosmos client profile requires --expected-account-id from 'cosmos-pp-cli auth identify --json'")
	}
	parsed, err := url.Parse(source.ExpectedBaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "api.cosmos.so" || parsed.Path != "" {
		return fmt.Errorf("Cosmos client profile expected base URL must be https://api.cosmos.so")
	}
	return nil
}

func ensureCosmosPlatformRegistered() {
	if registeredPlatformSource != nil {
		return
	}
	registerPlatformSource(platformSourceRegistration{
		Source:                    "cosmos",
		Adapter:                   cosmosIdentityAdapter{},
		CredentialResolverFactory: func() platform.CredentialResolver { return cosmosEnvironmentResolver{} },
		ValidateSourceProfile:     validateCosmosSourceProfile,
		BindClient: func(c *client.Client, session *platform.Session) error {
			credential := session.Credentials["credential"]
			if len(credential) == 0 {
				return fmt.Errorf("verified Cosmos profile has no bearer token")
			}
			c.Config.AuthHeaderVal = ""
			c.Config.AccessToken = ""
			c.Config.CosmosToken = string(credential)
			c.Config.AuthSource = "client-profile:" + session.ProfileName
			c.BaseURL = strings.TrimRight(session.SourceProfile.ExpectedBaseURL, "/")
			return c.SetPlatformEndpointBudget(platform.EndpointBudget{
				Class:       "POST /graphql",
				Steady:      1,
				Interval:    time.Second,
				Burst:       1,
				MaxAttempts: 3,
				RetryBudget: 30 * time.Second,
			})
		},
	})
}
