package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadCookies reads cookies imported by `auth login --chrome`.
// The preferred file is ~/.config/airbnb-vrbo-pp-cli/cookies.json, but the
// generated auth command may also store a raw Cookie header in config.toml.
func LoadCookies() ([]*http.Cookie, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".config", "airbnb-vrbo-pp-cli", "cookies.json")
	data, err := os.ReadFile(path)
	if err == nil {
		return parseCookieFile(data)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read cookies.json: %w", err)
	}
	cfgData, err := os.ReadFile(filepath.Join(home, ".config", "airbnb-vrbo-pp-cli", "config.toml"))
	if err != nil {
		return nil, fmt.Errorf("no cookie file at %s: %w", path, err)
	}
	raw := extractTOMLCookieHeader(string(cfgData))
	if raw == "" {
		return nil, fmt.Errorf("no cookies configured; run auth login --chrome")
	}
	return ParseCookieHeader(raw), nil
}

func parseCookieFile(data []byte) ([]*http.Cookie, error) {
	var arr []struct {
		Name    string  `json:"name"`
		Value   string  `json:"value"`
		Domain  string  `json:"domain"`
		Path    string  `json:"path"`
		Expires float64 `json:"expires"`
	}
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		out := make([]*http.Cookie, 0, len(arr))
		for _, c := range arr {
			if c.Name == "" {
				continue
			}
			ck := &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path}
			if c.Expires > 0 {
				ck.Expires = time.Unix(int64(c.Expires), 0)
			}
			out = append(out, ck)
		}
		return out, nil
	}
	var raw struct {
		Cookies string `json:"cookies"`
		Header  string `json:"cookie_header"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse cookies.json: %w", err)
	}
	header := raw.Cookies
	if header == "" {
		header = raw.Header
	}
	if header == "" {
		return nil, fmt.Errorf("cookies.json contains no cookies")
	}
	return ParseCookieHeader(header), nil
}

// ParseCookieHeader converts a Cookie header into []*http.Cookie.
func ParseCookieHeader(header string) []*http.Cookie {
	var out []*http.Cookie
	for _, part := range strings.Split(header, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" {
			continue
		}
		out = append(out, &http.Cookie{Name: name, Value: value, Path: "/"})
	}
	return out
}

func extractTOMLCookieHeader(s string) string {
	for _, key := range []string{"cookies", "cookie_header", "token"} {
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, key+" ") && !strings.HasPrefix(line, key+"=") {
				continue
			}
			_, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			value = strings.Trim(value, `"`)
			if strings.Contains(value, "=") {
				return value
			}
		}
	}
	return ""
}
