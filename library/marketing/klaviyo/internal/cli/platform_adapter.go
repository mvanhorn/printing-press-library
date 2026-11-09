// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

type klaviyoIdentityAdapter struct {
	baseURL    string
	httpClient *http.Client
}

type klaviyoLegacyConfig struct {
	BaseURL string `toml:"base_url"`
}

func (a klaviyoIdentityAdapter) EndpointClass() string { return "GET /api/accounts" }

var waitForIdentityProbeRetry = func(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a klaviyoIdentityAdapter) ProbeIdentity(ctx context.Context, credentials platform.ResolvedCredentials, _ platform.SourceProfile) (platform.ObservedIdentity, error) {
	baseURL := strings.TrimRight(a.baseURL, "/")
	if baseURL == "" {
		baseURL = "https://a.klaviyo.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/accounts", nil)
	if err != nil {
		return platform.ObservedIdentity{}, err
	}
	req.Header.Set("Authorization", "Klaviyo-API-Key "+strings.TrimSpace(string(credentials["credential"])))
	req.Header.Set("revision", "2026-04-15")
	session := platform.SessionFromContext(ctx)
	var resp *http.Response
	for attempt := 0; attempt < 4; attempt++ {
		// The platform gate records the initial request; the adapter owns retries.
		if attempt > 0 {
			session.RecordRateLimitRequest()
		}
		resp, err = a.httpClient.Do(req)
		if err != nil {
			return platform.ObservedIdentity{}, klaviyoProbeFailure(ctx, 0, err)
		}
		if attempt == 3 || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500) {
			break
		}
		resp.Body.Close()
		now := time.Now()
		retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
		if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err != nil || seconds < 0 {
			if date, err := http.ParseTime(retryAfter); err != nil || !date.After(now) {
				// Preserve the probe's deterministic fallback instead of the shared helper's jitter.
				retryAfter = strconv.Itoa(1 << attempt)
			}
		}
		wait := min(platform.RetryAfterDelay(retryAfter, attempt, now, nil), 30*time.Second)
		session.RecordRateLimitRetry(wait)
		if err := waitForIdentityProbeRetry(ctx, wait); err != nil {
			return platform.ObservedIdentity{}, klaviyoProbeFailure(ctx, 0, err)
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return platform.ObservedIdentity{}, klaviyoProbeFailure(ctx, resp.StatusCode, errors.New("identity endpoint rejected request"))
	}
	var payload struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				ContactInformation struct {
					OrganizationName string `json:"organization_name"`
				} `json:"contact_information"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || len(payload.Data) != 1 || strings.TrimSpace(payload.Data[0].ID) == "" || strings.TrimSpace(payload.Data[0].Attributes.ContactInformation.OrganizationName) == "" {
		return platform.ObservedIdentity{}, &platform.ProbeFailure{Outcome: platform.GateIndeterminate, Err: errors.New("Klaviyo accounts response did not contain one immutable account ID and organization name")}
	}
	return platform.ObservedIdentity{AccountID: payload.Data[0].ID, OrgName: payload.Data[0].Attributes.ContactInformation.OrganizationName}, nil
}

func klaviyoProbeFailure(ctx context.Context, status int, err error) error {
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &platform.ProbeFailure{Outcome: platform.GateUnavailable, Err: err}
	}
	outcome := platform.GateIndeterminate
	switch {
	case status == 401:
		outcome = platform.GateInvalidCredentials
	case status == 403:
		outcome = platform.GateInsufficientScope
	case status == 429:
		outcome = platform.GateRateLimited
	case status >= 500:
		outcome = platform.GateUnavailable
	}
	return &platform.ProbeFailure{Outcome: outcome, Err: fmt.Errorf("Klaviyo identity probe failed with status %d", status)}
}

func bindKlaviyoPlatformClient(c *client.Client, session *platform.Session) error {
	// The checked-in endpoint-budget inventory identifies this declaration as a
	// published ceiling or an explicitly conservative internal policy.
	if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "*", Steady: 15, Interval: time.Minute, Burst: 1, MaxAttempts: 4, RetryBudget: time.Minute}); err != nil {
		return err
	}
	credential := strings.TrimSpace(string(session.Credentials["credential"]))
	if credential == "" {
		return errors.New("client profile credential is empty")
	}
	c.Config.AuthHeaderVal = ""
	c.Config.KlaviyoApiKey = credential
	c.Config.AuthSource = "client-profile:" + session.ProfileName
	c.BaseURL = "https://a.klaviyo.com"
	if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "GET /api/accounts", Steady: 15, Interval: time.Minute, Burst: 1, MaxAttempts: 4, RetryBudget: time.Minute}); err != nil {
		return err
	}
	return c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "GET /api/profiles", Steady: 15, Interval: time.Minute, Burst: 1, MaxAttempts: 4, RetryBudget: time.Minute})
}

func validateKlaviyoSourceProfile(source platform.SourceProfile) error {
	if strings.TrimSpace(source.CredentialRef) == "" {
		return errors.New("Klaviyo source requires an exact API-key credential_ref")
	}
	if strings.TrimSpace(source.ExpectedAccountID) == "" {
		return errors.New("Klaviyo source requires trusted expected_account_id")
	}
	if strings.TrimSpace(source.ExpectedOrgName) == "" {
		return errors.New("Klaviyo source requires expected_org_name")
	}
	return nil
}

func validateKlaviyoLegacyInputs(cmd *cobra.Command, flags *rootFlags, session *platform.Session) error {
	for name, value := range map[string]string{"--config": flags.configPath, "KLAVIYO_CONFIG": os.Getenv("KLAVIYO_CONFIG"), "KLAVIYO_API_KEY": platformLegacyCredentialEnvironment(session, "KLAVIYO_API_KEY"), "KLAVIYO_BASE_URL": os.Getenv("KLAVIYO_BASE_URL")} {
		if strings.TrimSpace(value) != "" {
			return fmt.Errorf("configuration conflict: %s cannot override a selected client profile", name)
		}
	}
	if dbFlag := cmd.Flags().Lookup("db"); dbFlag != nil && dbFlag.Changed && filepath.Clean(dbFlag.Value.String()) != filepath.Clean(session.Paths.DataFile) {
		return errors.New("configuration conflict: --db does not match the client profile data path")
	}
	return nil
}

func prepareKlaviyoLegacyMigration(flags *rootFlags, _ *platformSourceFlags, inputs *platformLegacyMigrationFlags) error {
	if inputs.accountName == "" && strings.TrimSpace(inputs.configPath) != "" {
		return errors.New("Klaviyo legacy config migration requires --legacy-account")
	}
	if inputs.accountName == "" && strings.TrimSpace(flags.configPath) != "" {
		return errors.New("Klaviyo named-account migration requires --legacy-account instead of an unscoped --config")
	}
	if inputs.accountName == "" {
		return nil
	}
	if err := validateKlaviyoAccountName(inputs.accountName); err != nil {
		return err
	}
	accountPath, err := klaviyoAccountConfigPath(inputs.accountName)
	if err != nil {
		return err
	}
	if inputs.configPath != "" && filepath.Clean(inputs.configPath) != filepath.Clean(accountPath) {
		return errors.New("--legacy-config does not match the selected --legacy-account")
	}
	inputs.configPath = accountPath
	var legacy klaviyoLegacyConfig
	if err := platform.ReadLegacyConfig(accountPath, &legacy); err != nil {
		return err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(legacy.BaseURL), "/")
	if baseURL != "" && baseURL != "https://a.klaviyo.com" {
		return errors.New("legacy Klaviyo account uses an unexpected API base URL")
	}
	inputs.configVerified = true
	return nil
}

func inspectKlaviyoLegacyDatabase(ctx context.Context, db *sql.DB, session *platform.Session) (platform.LegacyDatabaseEvidence, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type IN ('account', 'accounts') ORDER BY id LIMIT 2`)
	if err != nil {
		return platform.LegacyDatabaseEvidence{Method: "klaviyo-account-row", Reason: "legacy Klaviyo database has no bounded account identity evidence"}, nil
	}
	defer rows.Close()
	identities := map[string]struct{}{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return platform.LegacyDatabaseEvidence{}, err
		}
		var item struct {
			ID         string `json:"id"`
			Attributes struct {
				ContactInformation struct {
					OrganizationName string `json:"organization_name"`
				} `json:"contact_information"`
			} `json:"attributes"`
		}
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		accountID := strings.TrimSpace(item.ID)
		orgName := strings.TrimSpace(item.Attributes.ContactInformation.OrganizationName)
		if accountID != "" && orgName != "" {
			identities[accountID+"\x00"+orgName] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return platform.LegacyDatabaseEvidence{}, err
	}
	want := session.ExpectedIdentity["account_id"] + "\x00" + session.ExpectedIdentity["org_name"]
	_, matched := identities[want]
	return platform.LegacyDatabaseEvidence{Verified: matched && len(identities) == 1, Method: "klaviyo-account-row", Reason: "legacy Klaviyo account identity did not match the verified profile"}, nil
}

func klaviyoResourceHealth(_ context.Context, session *platform.Session) ([]platform.ResourceHealth, error) {
	return store.InspectResourceHealth(session.Paths.DataFile, session)
}

func init() {
	registerPlatformSource(platformSourceRegistration{
		Source:                 "klaviyo",
		Adapter:                klaviyoIdentityAdapter{httpClient: &http.Client{Timeout: 30 * time.Second}},
		BindClient:             bindKlaviyoPlatformClient,
		ValidateSourceProfile:  validateKlaviyoSourceProfile,
		ValidateLegacyInputs:   validateKlaviyoLegacyInputs,
		PrepareLegacyMigration: prepareKlaviyoLegacyMigration,
		LegacyDatabaseEvidence: inspectKlaviyoLegacyDatabase,
		ResourceHealth:         klaviyoResourceHealth,
		WindowMaximumDays:      364,
		WindowMaximumName:      "klaviyo_metric_aggregation_364_days",
	})
}
