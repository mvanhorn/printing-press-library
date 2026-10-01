// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"encoding/base64"
	"strings"
)

// IsLiveModeCredential recognizes Stripe secret and restricted live keys,
// including keys wrapped in Bearer or HTTP Basic authorization headers.
func IsLiveModeCredential(value string) bool {
	credential := strings.TrimSpace(value)
	if scheme, value, found := strings.Cut(credential, " "); found {
		switch {
		case strings.EqualFold(scheme, "Bearer"):
			credential = strings.TrimSpace(value)
		case strings.EqualFold(scheme, "Basic"):
			credential = strings.TrimSpace(value)
			if decoded, err := base64.StdEncoding.DecodeString(credential); err == nil {
				credential, _, _ = strings.Cut(string(decoded), ":")
			}
		}
	}
	return strings.HasPrefix(credential, "sk_live_") || strings.HasPrefix(credential, "rk_live_")
}
