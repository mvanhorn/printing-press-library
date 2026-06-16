// Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: U10 Config and environment variable testing

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDefaults tests that default values are set correctly
func TestLoadDefaults(t *testing.T) {
	// Unset all environment variables to ensure defaults are used
	env := map[string]string{
		"POSTGRESQL_HOST":     os.Getenv("POSTGRESQL_HOST"),
		"POSTGRESQL_PORT":     os.Getenv("POSTGRESQL_PORT"),
		"POSTGRESQL_USER":     os.Getenv("POSTGRESQL_USER"),
		"POSTGRESQL_PASSWORD": os.Getenv("POSTGRESQL_PASSWORD"),
		"POSTGRESQL_DBNAME":   os.Getenv("POSTGRESQL_DBNAME"),
		"POSTGRESQL_SSLMODE":  os.Getenv("POSTGRESQL_SSLMODE"),
		"POSTGRESQL_CONFIG":   os.Getenv("POSTGRESQL_CONFIG"),
	}
	defer func() {
		// Restore environment variables
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Clear all env vars
	os.Unsetenv("POSTGRESQL_HOST")
	os.Unsetenv("POSTGRESQL_PORT")
	os.Unsetenv("POSTGRESQL_USER")
	os.Unsetenv("POSTGRESQL_PASSWORD")
	os.Unsetenv("POSTGRESQL_DBNAME")
	os.Unsetenv("POSTGRESQL_SSLMODE")
	os.Unsetenv("POSTGRESQL_CONFIG")

	// Set user to avoid "user is required" error
	os.Setenv("POSTGRESQL_USER", "postgres")

	cfg, err := Load("/nonexistent/config.toml")
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host = %s, want localhost", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("Port = %d, want 5432", cfg.Port)
	}
	if cfg.DBName != "postgres" {
		t.Errorf("DBName = %s, want postgres", cfg.DBName)
	}
	if cfg.SSLMode != "prefer" {
		t.Errorf("SSLMode = %s, want prefer", cfg.SSLMode)
	}
}

// TestLoadFromTOML tests loading configuration from a TOML file
func TestLoadFromTOML(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_HOST":     os.Getenv("POSTGRESQL_HOST"),
		"POSTGRESQL_PORT":     os.Getenv("POSTGRESQL_PORT"),
		"POSTGRESQL_USER":     os.Getenv("POSTGRESQL_USER"),
		"POSTGRESQL_PASSWORD": os.Getenv("POSTGRESQL_PASSWORD"),
		"POSTGRESQL_DBNAME":   os.Getenv("POSTGRESQL_DBNAME"),
		"POSTGRESQL_SSLMODE":  os.Getenv("POSTGRESQL_SSLMODE"),
		"POSTGRESQL_CONFIG":   os.Getenv("POSTGRESQL_CONFIG"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Unsetenv("POSTGRESQL_HOST")
	os.Unsetenv("POSTGRESQL_PORT")
	os.Unsetenv("POSTGRESQL_USER")
	os.Unsetenv("POSTGRESQL_PASSWORD")
	os.Unsetenv("POSTGRESQL_DBNAME")
	os.Unsetenv("POSTGRESQL_SSLMODE")
	os.Unsetenv("POSTGRESQL_CONFIG")

	// Create a temporary TOML file
	tomlContent := `host = "db.example.com"
port = 5433
user = "appuser"
password = "secret123"
dbname = "mydb"
sslmode = "require"
`
	tmpFile := "/tmp/test_config.toml"
	if err := os.WriteFile(tmpFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.Host != "db.example.com" {
		t.Errorf("Host = %s, want db.example.com", cfg.Host)
	}
	if cfg.Port != 5433 {
		t.Errorf("Port = %d, want 5433", cfg.Port)
	}
	if cfg.User != "appuser" {
		t.Errorf("User = %s, want appuser", cfg.User)
	}
	if cfg.Password != "secret123" {
		t.Errorf("Password = %s, want secret123", cfg.Password)
	}
	if cfg.DBName != "mydb" {
		t.Errorf("DBName = %s, want mydb", cfg.DBName)
	}
	if cfg.SSLMode != "require" {
		t.Errorf("SSLMode = %s, want require", cfg.SSLMode)
	}
}

// TestEnvVarOverride tests that environment variables override TOML values
func TestEnvVarOverride(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_HOST":     os.Getenv("POSTGRESQL_HOST"),
		"POSTGRESQL_PORT":     os.Getenv("POSTGRESQL_PORT"),
		"POSTGRESQL_USER":     os.Getenv("POSTGRESQL_USER"),
		"POSTGRESQL_PASSWORD": os.Getenv("POSTGRESQL_PASSWORD"),
		"POSTGRESQL_DBNAME":   os.Getenv("POSTGRESQL_DBNAME"),
		"POSTGRESQL_SSLMODE":  os.Getenv("POSTGRESQL_SSLMODE"),
		"POSTGRESQL_CONFIG":   os.Getenv("POSTGRESQL_CONFIG"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	tomlContent := `host = "db.example.com"
port = 5433
user = "appuser"
password = "secret123"
dbname = "mydb"
sslmode = "require"
`
	tmpFile := "/tmp/test_config_override.toml"
	if err := os.WriteFile(tmpFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	defer os.Remove(tmpFile)

	// Set environment variables to override TOML values
	os.Setenv("POSTGRESQL_HOST", "env.example.com")
	os.Setenv("POSTGRESQL_PORT", "5434")
	os.Setenv("POSTGRESQL_USER", "envuser")
	os.Setenv("POSTGRESQL_PASSWORD", "envpass")
	os.Setenv("POSTGRESQL_DBNAME", "envdb")
	os.Setenv("POSTGRESQL_SSLMODE", "disable")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.Host != "env.example.com" {
		t.Errorf("Host = %s, want env.example.com (from env var)", cfg.Host)
	}
	if cfg.Port != 5434 {
		t.Errorf("Port = %d, want 5434 (from env var)", cfg.Port)
	}
	if cfg.User != "envuser" {
		t.Errorf("User = %s, want envuser (from env var)", cfg.User)
	}
	if cfg.Password != "envpass" {
		t.Errorf("Password = %s, want envpass (from env var)", cfg.Password)
	}
	if cfg.DBName != "envdb" {
		t.Errorf("DBName = %s, want envdb (from env var)", cfg.DBName)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("SSLMode = %s, want disable (from env var)", cfg.SSLMode)
	}
}

// TestMissingRequiredFields tests that Load returns errors for missing required fields
func TestMissingRequiredFields(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_HOST":     os.Getenv("POSTGRESQL_HOST"),
		"POSTGRESQL_PORT":     os.Getenv("POSTGRESQL_PORT"),
		"POSTGRESQL_USER":     os.Getenv("POSTGRESQL_USER"),
		"POSTGRESQL_PASSWORD": os.Getenv("POSTGRESQL_PASSWORD"),
		"POSTGRESQL_DBNAME":   os.Getenv("POSTGRESQL_DBNAME"),
		"POSTGRESQL_SSLMODE":  os.Getenv("POSTGRESQL_SSLMODE"),
		"POSTGRESQL_CONFIG":   os.Getenv("POSTGRESQL_CONFIG"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Unsetenv("POSTGRESQL_HOST")
	os.Unsetenv("POSTGRESQL_PORT")
	os.Unsetenv("POSTGRESQL_USER")
	os.Unsetenv("POSTGRESQL_PASSWORD")
	os.Unsetenv("POSTGRESQL_DBNAME")
	os.Unsetenv("POSTGRESQL_SSLMODE")
	os.Unsetenv("POSTGRESQL_CONFIG")

	tests := []struct {
		name    string
		setupFn func()
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing_host",
			setupFn: func() {
				os.Setenv("POSTGRESQL_USER", "user")
				// HOST not set - should use default localhost and pass
			},
			wantErr: false,
		},
		{
			name: "missing_user",
			setupFn: func() {
				os.Unsetenv("POSTGRESQL_USER")
			},
			wantErr: true,
			errMsg:  "user is required",
		},
		{
			name: "invalid_port",
			setupFn: func() {
				os.Setenv("POSTGRESQL_USER", "user")
				os.Setenv("POSTGRESQL_PORT", "not_a_number")
			},
			wantErr: true,
			errMsg:  "invalid POSTGRESQL_PORT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset env vars for each test
			os.Unsetenv("POSTGRESQL_HOST")
			os.Unsetenv("POSTGRESQL_PORT")
			os.Unsetenv("POSTGRESQL_USER")
			os.Unsetenv("POSTGRESQL_PASSWORD")
			os.Unsetenv("POSTGRESQL_DBNAME")
			os.Unsetenv("POSTGRESQL_SSLMODE")

			tt.setupFn()

			cfg, err := Load("/nonexistent/config.toml")
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Load() error = %v, want error containing %s", err, tt.errMsg)
				}
			}
			if !tt.wantErr && cfg == nil {
				t.Errorf("Load() returned nil config when error was nil")
			}
		})
	}
}

// TestConnectionString tests that ConnectionString() returns a valid DSN
func TestConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "basic_no_password",
			config: &Config{
				Host:    "localhost",
				Port:    5432,
				User:    "postgres",
				DBName:  "postgres",
				SSLMode: "prefer",
			},
			expected: "postgres://postgres@localhost:5432/postgres?sslmode=prefer",
		},
		{
			name: "with_password",
			config: &Config{
				Host:     "db.example.com",
				Port:     5433,
				User:     "appuser",
				Password: "secret123",
				DBName:   "mydb",
				SSLMode:  "require",
			},
			expected: "postgres://appuser:secret123@db.example.com:5433/mydb?sslmode=require",
		},
		{
			name: "password_with_special_chars",
			config: &Config{
				Host:     "localhost",
				Port:    5432,
				User:     "user",
				Password: "pass@word:123",
				DBName:   "db",
				SSLMode:  "disable",
			},
			expected: "postgres://user:pass@word:123@localhost:5432/db?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ConnectionString()
			if result != tt.expected {
				t.Errorf("ConnectionString() = %s\nwant %s", result, tt.expected)
			}
		})
	}
}

// TestConfigPath tests that the config path is set correctly
func TestConfigPath(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_CONFIG": os.Getenv("POSTGRESQL_CONFIG"),
		"POSTGRESQL_USER":   os.Getenv("POSTGRESQL_USER"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Unsetenv("POSTGRESQL_CONFIG")
	os.Setenv("POSTGRESQL_USER", "postgres")

	customPath := "/custom/path/config.toml"
	cfg, err := Load(customPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Path != customPath {
		t.Errorf("cfg.Path = %s, want %s", cfg.Path, customPath)
	}
}

// TestLoadFromPOSTGRESQLCONFIG tests that POSTGRESQL_CONFIG env var works
func TestLoadFromPOSTGRESQLCONFIG(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_CONFIG": os.Getenv("POSTGRESQL_CONFIG"),
		"POSTGRESQL_USER":   os.Getenv("POSTGRESQL_USER"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	tomlContent := `host = "from_env_config"
user = "envuser"
`
	tmpFile := "/tmp/env_config.toml"
	if err := os.WriteFile(tmpFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	defer os.Remove(tmpFile)

	os.Setenv("POSTGRESQL_CONFIG", tmpFile)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Path != tmpFile {
		t.Errorf("cfg.Path = %s, want %s", cfg.Path, tmpFile)
	}
	if cfg.Host != "from_env_config" {
		t.Errorf("cfg.Host = %s, want from_env_config", cfg.Host)
	}
}

// TestLoadFromDefaultPath tests that the default config path is used
func TestLoadFromDefaultPath(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_CONFIG": os.Getenv("POSTGRESQL_CONFIG"),
		"POSTGRESQL_USER":   os.Getenv("POSTGRESQL_USER"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Unsetenv("POSTGRESQL_CONFIG")
	os.Setenv("POSTGRESQL_USER", "postgres")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".config", "postgresql-admin-pp-cli", "config.toml")
	if cfg.Path != expectedPath {
		t.Errorf("cfg.Path = %s, want %s", cfg.Path, expectedPath)
	}
}

// TestInvalidTOMLSyntax tests that invalid TOML syntax is caught
func TestInvalidTOMLSyntax(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_USER": os.Getenv("POSTGRESQL_USER"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Setenv("POSTGRESQL_USER", "postgres")

	// Invalid TOML syntax
	tomlContent := `host = "localhost"
port = "not a number, not quoted in TOML"
`
	tmpFile := "/tmp/invalid_config.toml"
	if err := os.WriteFile(tmpFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	if err == nil {
		t.Errorf("Load() error = nil, want parse error")
	}
	if cfg != nil {
		t.Errorf("Load() returned non-nil config on parse error")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestConfigConcurrentAccess tests that config is safe for concurrent access
func TestConfigConcurrentAccess(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_USER": os.Getenv("POSTGRESQL_USER"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Setenv("POSTGRESQL_USER", "testuser")

	cfg, err := Load("/nonexistent.toml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Multiple goroutines accessing config concurrently should not panic
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_ = cfg.ConnectionString()
			_ = cfg.Host
			_ = cfg.Port
			_ = cfg.User
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		<-done
	}
}

// TestPartialTOMLLoad tests that a TOML file with missing fields uses defaults
func TestPartialTOMLLoad(t *testing.T) {
	env := map[string]string{
		"POSTGRESQL_HOST":     os.Getenv("POSTGRESQL_HOST"),
		"POSTGRESQL_PORT":     os.Getenv("POSTGRESQL_PORT"),
		"POSTGRESQL_USER":     os.Getenv("POSTGRESQL_USER"),
		"POSTGRESQL_PASSWORD": os.Getenv("POSTGRESQL_PASSWORD"),
		"POSTGRESQL_DBNAME":   os.Getenv("POSTGRESQL_DBNAME"),
		"POSTGRESQL_SSLMODE":  os.Getenv("POSTGRESQL_SSLMODE"),
		"POSTGRESQL_CONFIG":   os.Getenv("POSTGRESQL_CONFIG"),
	}
	defer func() {
		for k, v := range env {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Unsetenv("POSTGRESQL_HOST")
	os.Unsetenv("POSTGRESQL_PORT")
	os.Unsetenv("POSTGRESQL_USER")
	os.Unsetenv("POSTGRESQL_PASSWORD")
	os.Unsetenv("POSTGRESQL_DBNAME")
	os.Unsetenv("POSTGRESQL_SSLMODE")
	os.Unsetenv("POSTGRESQL_CONFIG")

	// Minimal TOML file with only required fields
	tomlContent := `user = "testuser"`
	tmpFile := "/tmp/partial_config.toml"
	if err := os.WriteFile(tmpFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check that defaults are still applied
	if cfg.Host != "localhost" {
		t.Errorf("Host = %s, want localhost (default)", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("Port = %d, want 5432 (default)", cfg.Port)
	}
	if cfg.User != "testuser" {
		t.Errorf("User = %s, want testuser (from TOML)", cfg.User)
	}
	if cfg.DBName != "postgres" {
		t.Errorf("DBName = %s, want postgres (default)", cfg.DBName)
	}
	if cfg.SSLMode != "prefer" {
		t.Errorf("SSLMode = %s, want prefer (default)", cfg.SSLMode)
	}
}
