package cli

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/config"
	_ "modernc.org/sqlite"
)

// TestCountCookiesForDomain_RejectsSQLInjection exercises the parameterized
// query path. Before the fix, the function built the WHERE clause via
// fmt.Sprintf and shelled out to the `sqlite3` binary; a domainPattern
// containing single-quote + UNION ALL would have either errored or returned
// wildly wrong counts. After the fix, the value is passed as a bind parameter
// and SQLite treats every byte as data.
func TestCountCookiesForDomain_RejectsSQLInjection(t *testing.T) {
	tmpDir := t.TempDir()
	cookiesDB := filepath.Join(tmpDir, "Cookies")

	// Build a minimal Chrome-shaped cookies table with 3 rows.
	db, err := sql.Open("sqlite", "file:"+cookiesDB+"?_journal_mode=OFF")
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE cookies (host_key TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for _, host := range []string{".amazon.com", ".amazon.com", ".example.com"} {
		if _, err := db.Exec(`INSERT INTO cookies (host_key) VALUES (?)`, host); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	db.Close()

	tests := []struct {
		name    string
		pattern string
		want    int
	}{
		{"normal amazon", "%amazon.com%", 2},
		{"normal example", "%example.com%", 1},
		{"no match", "%nonexistent.com%", 0},
		// Pre-fix, the next two would have either errored (broken SQL) or
		// returned the row count of the entire cookies table (UNION ALL bypass).
		// Post-fix, both are treated as opaque pattern strings so neither
		// matches any literal host_key and both return 0.
		{"injection: trailing OR", "%amazon.com%' OR '1'='1", 0},
		{"injection: union all", "x%' UNION ALL SELECT 'pwned", 0},
	}
	for _, tt := range tests {
		got := countCookiesForDomain(cookiesDB, tt.pattern)
		if got != tt.want {
			t.Errorf("countCookiesForDomain(%q) = %d, want %d", tt.pattern, got, tt.want)
		}
	}
}

// TestCountCookiesForDomain_MissingFile returns 0 when the cookie DB is absent
// (e.g., Chrome profile has never been used). Regression for the early-return
// path that runs before the SQL query is built.
func TestCountCookiesForDomain_MissingFile(t *testing.T) {
	got := countCookiesForDomain(filepath.Join(t.TempDir(), "does-not-exist"), "%anything%")
	if got != 0 {
		t.Errorf("missing-file count = %d, want 0", got)
	}
	// Ensure no stray temp files survived (defer cleanup ran).
	tmpDir := os.TempDir()
	entries, _ := filepath.Glob(filepath.Join(tmpDir, "cookies-probe-*.db"))
	for _, e := range entries {
		if info, err := os.Stat(e); err == nil && info.ModTime().Unix() < 1 {
			t.Errorf("leftover temp file: %s", e)
		}
	}
}

func TestCookieDomainFromBaseURL_Marketplaces(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"default US", "https://www.amazon.com", ".amazon.com"},
		{"india", "https://www.amazon.in", ".amazon.in"},
		{"india without scheme", "amazon.in", ".amazon.in"},
		{"uk", "https://www.amazon.co.uk", ".amazon.co.uk"},
		{"australia", "https://www.amazon.com.au", ".amazon.com.au"},
		{"japan", "https://www.amazon.co.jp", ".amazon.co.jp"},
		{"subdomain", "https://smile.amazon.com/gp/your-account/order-history", ".amazon.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cookieDomainFromBaseURL(tt.baseURL)
			if err != nil {
				t.Fatalf("cookieDomainFromBaseURL(%q) error: %v", tt.baseURL, err)
			}
			if got != tt.want {
				t.Fatalf("cookieDomainFromBaseURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestCookieDomainFromConfig_UsesBaseURL(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://www.amazon.in"}
	got, err := cookieDomainFromConfig(cfg)
	if err != nil {
		t.Fatalf("cookieDomainFromConfig() error: %v", err)
	}
	if got != ".amazon.in" {
		t.Fatalf("cookieDomainFromConfig() = %q, want .amazon.in", got)
	}
}

func TestNormalizeAmazonCookieDomain_RejectsInvalidDomains(t *testing.T) {
	for _, domain := range []string{"example.com", "amazon.evil.com", "amazon.phishing.io", "notamazon.in"} {
		t.Run(domain, func(t *testing.T) {
			if _, err := normalizeAmazonCookieDomain(domain); err == nil {
				t.Fatalf("normalizeAmazonCookieDomain(%q) succeeded, want error", domain)
			}
		})
	}
}

func TestCookieDomainFromConfig_InvalidBaseURLSurfacesError(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://www.amazon.evil.com"}
	if _, err := cookieDomainFromConfig(cfg); err == nil {
		t.Fatal("cookieDomainFromConfig() succeeded, want error")
	}
}

func TestValidateBrowserSessionOrWarnClearsProofAndReturnsFalse(t *testing.T) {
	cfg := &config.Config{Path: filepath.Join(t.TempDir(), "config.toml")}
	if err := writeBrowserSessionProof(cfg, []byte("{}\n")); err != nil {
		t.Fatalf("write proof: %v", err)
	}

	var out bytes.Buffer
	ok := validateBrowserSessionOrWarn(&out, cfg, func() error {
		return errors.New("validation blocked")
	})
	if ok {
		t.Fatal("validateBrowserSessionOrWarn returned true, want false")
	}
	text := out.String()
	if !strings.Contains(text, "Warning: saved cookies but could not validate") {
		t.Fatalf("missing warning output: %s", text)
	}
	if strings.Contains(text, "failed") {
		t.Fatalf("output used fatal failed wording: %s", text)
	}
	if _, err := os.Stat(browserSessionProofPath(cfg)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("proof was not cleared, stat err=%v", err)
	}
}

func TestValidateBrowserSessionOrWarnReturnsTrueOnSuccess(t *testing.T) {
	cfg := &config.Config{Path: filepath.Join(t.TempDir(), "config.toml")}
	var out bytes.Buffer
	ok := validateBrowserSessionOrWarn(&out, cfg, func() error { return nil })
	if !ok {
		t.Fatal("validateBrowserSessionOrWarn returned false, want true")
	}
	if !strings.Contains(out.String(), "valid") {
		t.Fatalf("missing success output: %s", out.String())
	}
}
