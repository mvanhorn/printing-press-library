// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func klaviyoRequestContext(c any) context.Context {
	if bound, ok := c.(interface{ PlatformSession() *platform.Session }); ok {
		if session := bound.PlatformSession(); session != nil {
			return session.Context()
		}
	}
	return context.Background()
}

func defaultDBPathForClient(name string, c *client.Client) (string, error) {
	if c == nil {
		return "", fmt.Errorf("cannot resolve account-scoped database without a Klaviyo client")
	}
	if session := c.PlatformSession(); session != nil {
		return session.Paths.DataFile, nil
	}
	return defaultDBPathForFingerprint(name, client.AccountFingerprint(c.Config))
}

func defaultDBPathForFingerprint(name, fingerprint string) (string, error) {
	if fingerprint == "" || fingerprint == "unconfigured" {
		return "", fmt.Errorf("cannot resolve account-scoped database without Klaviyo credentials; configure an account or pass --db explicitly")
	}
	dataRoot := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		dataRoot = filepath.Join(home, ".local", "share")
	}
	accountPath := filepath.Join(dataRoot, name, "accounts", fingerprint, "data.db")
	if _, err := os.Stat(accountPath); err == nil {
		return accountPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking account-scoped database: %w", err)
	}

	// Older releases wrote every tenant to one unscoped database. Its tenant
	// cannot be inferred safely from the rows, so silently adopting or copying
	// it would recreate the cross-tenant poisoning this path prevents. Refuse
	// with both paths visible so the operator can verify and migrate it
	// explicitly, or start a clean account-scoped sync.
	legacyCandidates := []string{filepath.Join(dataRoot, name, "data.db")}
	if home, err := os.UserHomeDir(); err == nil {
		legacyCandidates = append(legacyCandidates, filepath.Join(home, ".local", "share", name, "data.db"))
	}
	seen := map[string]bool{}
	for _, legacyPath := range legacyCandidates {
		if seen[legacyPath] {
			continue
		}
		seen[legacyPath] = true
		if _, err := os.Stat(legacyPath); err == nil {
			return "", fmt.Errorf("legacy unscoped Klaviyo database found at %s; refusing automatic use because its tenant identity is unknown. Verify the tenant and pass --db %s explicitly, or sync into the account-scoped path %s", legacyPath, legacyPath, accountPath)
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("checking legacy database %s: %w", legacyPath, err)
		}
	}
	return accountPath, nil
}

func extractResponseData(data json.RawMessage) json.RawMessage {
	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return data
	}
	if envelope.Data == nil || envelope.Status == "" {
		return data // No status field = not an envelope, might be regular "data" field
	}
	switch envelope.Status {
	case "success", "ok", "OK", "Success":
		return envelope.Data
	default:
		return data
	}
}

func cursorFromNextLink(nextLink, cursorParam string) string {
	if nextLink == "" || cursorParam == "" {
		return ""
	}
	parsed, err := url.Parse(nextLink)
	if err != nil {
		return ""
	}
	return parsed.Query().Get(cursorParam)
}

func (f *rootFlags) defaultDBPath(name string) (string, error) {
	if f != nil && f.platformSession != nil {
		return f.platformSession.Paths.DataFile, nil
	}
	cfg, err := loadEffectiveKlaviyoConfig(f.configPath)
	if err != nil {
		return "", configErr(err)
	}
	return defaultDBPathForFingerprint(name, client.AccountFingerprint(cfg))
}
