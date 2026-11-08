package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// playwrightStorageState models Playwright storage-state JSON (cookies array).
type playwrightStorageState struct {
	Cookies []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Domain string `json:"domain"`
	} `json:"cookies"`
}

// ParseCookiesFile reads Playwright storage-state JSON or a raw Cookie header file.
func ParseCookiesFile(data []byte, source string) (*Session, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("cookies file is empty")
	}
	if strings.HasPrefix(trimmed, "{") {
		var state playwrightStorageState
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("parse storage state: %w", err)
		}
		cookies := map[string]string{}
		for _, c := range state.Cookies {
			if !strings.Contains(c.Domain, "spothero") {
				continue
			}
			if c.Name != "" && c.Value != "" {
				cookies[c.Name] = c.Value
			}
		}
		if len(cookies) == 0 {
			return nil, fmt.Errorf("no spothero.com cookies found in storage state")
		}
		return &Session{
			Cookies:   cookies,
			Source:    source,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	return ParseCookieInput(trimmed, source)
}

// ParseCookieInput parses a raw Cookie header string.
func ParseCookieInput(raw, source string) (*Session, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty cookie input")
	}
	cookies := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.Index(part, "=")
		if i <= 0 {
			continue
		}
		cookies[part[:i]] = part[i+1:]
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no cookies parsed")
	}
	return &Session{
		RawCookieHeader: raw,
		Cookies:         cookies,
		Source:          source,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}
