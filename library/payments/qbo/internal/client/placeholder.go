// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/config"
)

func authHeaderLooksLikePlaceholderCredential(header string) bool {
	if scheme, encoded, ok := strings.Cut(strings.TrimSpace(header), " "); ok && strings.EqualFold(scheme, "Basic") {
		encoded = strings.TrimSpace(encoded)
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil && authHeaderLooksLikePlaceholderCredential(string(decoded)) {
			return true
		}
	}
	if !strings.Contains(header, "<") && !strings.Contains(header, "YOUR_TOKEN_HERE") && !strings.Contains(header, "your-token") && !strings.Contains(header, "your-key") {
		return false
	}
	for _, field := range strings.Fields(header) {
		field = strings.Trim(field, `"'`)
		if idx := strings.LastIndex(field, "="); idx >= 0 {
			field = field[idx+1:]
		}
		if idx := strings.Index(field, ":"); idx >= 0 {
			if looksLikeCredentialPlaceholder(field[:idx]) || looksLikeCredentialPlaceholder(field[idx+1:]) {
				return true
			}
		}
		if looksLikeCredentialPlaceholder(field) {
			return true
		}
	}
	return looksLikeCredentialPlaceholder(header)
}

func looksLikeCredentialPlaceholder(value string) bool {
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	switch value {
	case "<your-token>", "<your-key>", "<paste-your-key>", "YOUR_TOKEN_HERE", "your-token-here":
		return true
	}
	if len(value) < 3 || value[0] != '<' || value[len(value)-1] != '>' {
		return false
	}
	for _, r := range value[1 : len(value)-1] {
		if r != '_' && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func authPlaceholderCredentialError(cfg *config.Config) error {
	return authPlaceholderCredentialErrorWithSetup(cfg, "export QBO_CLIENT_ID=<your-token> or qbo-pp-cli auth set-token <token>")
}

func authPlaceholderCredentialErrorWithSetup(cfg *config.Config, setup string) error {
	location := "config file"
	if cfg != nil && cfg.Path != "" {
		location = cfg.Path
	}
	return fmt.Errorf("%w configured in %s; set a real token with: %s", ErrPlaceholderCredential, location, setup)
}
