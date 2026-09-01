package config

import (
	"fmt"
	"os"
	"strings"
)

// Settings mirrors the Python environment contract. Secrets are intentionally
// process-memory only; String never includes credential values.
type Settings struct {
	URL      string
	Token    string
	User     string
	Password string
	CABundle string
	Verify   bool
}

func SettingsFromEnv(environ map[string]string) (Settings, error) {
	get := func(k string) string { return strings.TrimSpace(environ[k]) }
	if environ == nil {
		environ = map[string]string{}
		get = func(k string) string { return strings.TrimSpace(os.Getenv(k)) }
	}
	s := Settings{URL: get("KVMCTL_URL"), Token: get("KVMCTL_TOKEN"), User: get("KVMCTL_USER"), Password: get("KVMCTL_PASSWORD"), CABundle: get("KVMCTL_CA_BUNDLE"), Verify: true}
	if s.URL == "" {
		return Settings{}, fmt.Errorf("KVMCTL_URL is required")
	}
	if s.Token == "" && s.User == "" && s.Password == "" {
		s.Token = get("KVMCTL_KVMD_TOKEN")
	}
	if s.Token == "" && ((s.User == "") != (s.Password == "")) {
		return Settings{}, fmt.Errorf("KVMCTL_USER and KVMCTL_PASSWORD must be provided together; password is missing")
	}
	if strings.Contains(strings.ToLower(get("KVMCTL_INSECURE")), "true") || get("KVMCTL_INSECURE") == "1" || strings.EqualFold(get("KVMCTL_INSECURE"), "yes") || strings.EqualFold(get("KVMCTL_INSECURE"), "on") {
		s.Verify = false
	}
	return s, nil
}

func (s Settings) String() string {
	return fmt.Sprintf("url=%s token=<redacted> user=%s password=<redacted> ca_bundle=%s verify=%t", s.URL, s.User, s.CABundle, s.Verify)
}
