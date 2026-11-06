// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"crypto/rand"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/config"
	"strings"
)

func isRequestKeyHeader(name string) bool {
	return strings.EqualFold(name, "Idempotency-Key") || strings.EqualFold(name, "X-Idempotency-Key")
}

func normalizedRequestKey(cfg *config.Config, overrides map[string]string) (string, error) {
	resolve := func(headers map[string]string) (string, error) {
		value := ""
		for name, candidate := range headers {
			if !isRequestKeyHeader(name) {
				continue
			}
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if value != "" && value != candidate {
				return "", fmt.Errorf("conflicting Idempotency-Key header aliases")
			}
			value = candidate
		}
		return value, nil
	}
	key, err := resolve(overrides)
	if err != nil {
		return "", err
	}
	if key == "" && cfg != nil {
		key, err = resolve(cfg.Headers)
		if err != nil {
			return "", err
		}
	}
	if key != "" {
		return key, nil
	}
	return newRequestKey()
}

func newRequestKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("creating request identity: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
