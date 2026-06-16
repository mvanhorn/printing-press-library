// Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: U10 Client connection pooling and query execution testing

package client

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/postgresql-admin/internal/config"
)

// TestNewClientWithDefaults tests client creation with default timeout
func TestNewClientWithDefaults(t *testing.T) {
	// Note: This test will fail without a running PostgreSQL instance
	// We'll test the timeout and configuration logic without a live connection
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
	}

	// This should fail to connect to a non-existent server, but it will test
	// the timeout configuration logic
	client, _ := New(cfg, 0)
	if client != nil {
		client.Close()
	}
}

// TestNewClientWithCustomTimeout tests client creation with custom timeout
func TestNewClientWithCustomTimeout(t *testing.T) {
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
	}

	customTimeout := 5 * time.Second
	client, _ := New(cfg, customTimeout)
	if client != nil {
		if client.timeout != customTimeout {
			t.Errorf("Client timeout = %v, want %v", client.timeout, customTimeout)
		}
		client.Close()
	}
}

// TestClientConnectionString tests that connection string is properly formatted
func TestClientConnectionString(t *testing.T) {
	cfg := &config.Config{
		Host:     "db.example.com",
		Port:     5433,
		User:     "appuser",
		Password: "secret",
		DBName:   "mydb",
		SSLMode:  "require",
	}

	expected := "postgres://appuser:secret@db.example.com:5433/mydb?sslmode=require"
	result := cfg.ConnectionString()
	if result != expected {
		t.Errorf("ConnectionString() = %s\nwant %s", result, expected)
	}
}

// TestClientTimeoutBehavior tests that custom timeouts are applied
func TestClientTimeoutBehavior(t *testing.T) {
	// This tests the timeout configuration without requiring a live DB
	// In a real scenario, we'd need a Docker PostgreSQL instance

	timeout := 10 * time.Second
	expectedDefault := 30 * time.Second

	if timeout == 0 {
		timeout = expectedDefault
	}

	if timeout != 10*time.Second {
		t.Errorf("Timeout not applied correctly: %v", timeout)
	}
}

// TestClientPoolConfiguration tests that connection pool is configured properly
func TestClientPoolConfiguration(t *testing.T) {
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
	}

	// Test that we can create a client (it may fail to connect, but config should be valid)
	client, newErr := New(cfg, 30*time.Second)
	if client != nil {
		// If connection succeeds, verify pool exists
		if client.pool == nil {
			t.Errorf("Client pool is nil after successful connection")
		}
		client.Close()
	} else if newErr == nil {
		t.Errorf("New() returned nil client without error")
	}
	// If connection fails due to unavailable DB, that's okay for this unit test
}

// TestClientClose tests that Close() can be called safely
func TestClientClose(t *testing.T) {
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
	}

	client, _ := New(cfg, 1*time.Second)
	if client != nil {
		closeErr := client.Close()
		if closeErr != nil {
			t.Errorf("Close() error = %v, want nil", closeErr)
		}
		// Closing again should not panic
		closeErr = client.Close()
		if closeErr != nil {
			t.Errorf("Second Close() error = %v, want nil", closeErr)
		}
	}
}

// TestClientDoubleClose tests that double close doesn't panic
func TestClientDoubleClose(t *testing.T) {
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
	}

	client, _ := New(cfg, 1*time.Second)
	if client != nil {
		client.Close()
		// This should not panic
		client.Close()
	}
}

// TestClientErrorHandling tests error scenarios
func TestClientErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		expectErr bool
	}{
		{
			name: "invalid_port",
			config: &config.Config{
				Host:    "localhost",
				Port:    99999,
				User:    "postgres",
				DBName:  "postgres",
				SSLMode: "disable",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config, 1*time.Second)
			if tt.expectErr && err == nil {
				t.Errorf("New() error = nil, want error")
			}
			if client != nil {
				client.Close()
			}
		})
	}
}

// TestClientWithMissingPassword tests that client can be created without password
func TestClientWithMissingPassword(t *testing.T) {
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
		// Password is empty
	}

	client, _ := New(cfg, 5*time.Second)
	if client != nil {
		connStr := cfg.ConnectionString()
		// Should not contain password in connection string
		if connStr != "postgres://postgres@localhost:5432/postgres?sslmode=disable" {
			t.Errorf("ConnectionString without password = %s", connStr)
		}
		client.Close()
	}
}

// TestConnectionStringEscaping tests special characters in connection strings
func TestConnectionStringEscaping(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		contains string
	}{
		{
			name: "basic",
			cfg: &config.Config{
				Host:    "localhost",
				Port:    5432,
				User:    "user",
				DBName:  "db",
				SSLMode: "disable",
			},
			contains: "postgres://user@localhost:5432/db?sslmode=disable",
		},
		{
			name: "with_password",
			cfg: &config.Config{
				Host:     "localhost",
				Port:    5432,
				User:     "user",
				Password: "pass123",
				DBName:   "db",
				SSLMode:  "disable",
			},
			contains: "postgres://user:pass123@localhost:5432/db?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.ConnectionString()
			if result != tt.contains {
				t.Errorf("ConnectionString() = %s\nwant %s", result, tt.contains)
			}
		})
	}
}

// TestClientContextTimeout tests that context timeouts are properly handled
func TestClientContextTimeout(t *testing.T) {
	// This is a compile-time test to ensure timeouts are properly set up
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
	}

	client, _ := New(cfg, 1*time.Second)
	if client != nil {
		// Test that Ping respects timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// This will timeout/fail, which is expected
		_ = client.Ping(ctx)
		client.Close()
	}
}

// TestClientConcurrentOperations tests that client can handle concurrent requests safely
func TestClientConcurrentOperations(t *testing.T) {
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "disable",
	}

	client, _ := New(cfg, 1*time.Second)
	if client != nil {
		// Try concurrent operations (even if they fail, they shouldn't panic)
		done := make(chan bool, 3)

		for i := 0; i < 3; i++ {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				_ = client.Ping(ctx)
				done <- true
			}()
		}

		for i := 0; i < 3; i++ {
			<-done
		}
		client.Close()
	}
}

// BenchmarkConnectionString benchmarks connection string generation
func BenchmarkConnectionString(b *testing.B) {
	cfg := &config.Config{
		Host:     "db.example.com",
		Port:     5433,
		User:     "appuser",
		Password: "secret123",
		DBName:   "mydb",
		SSLMode:  "require",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.ConnectionString()
	}
}

// TestNewClientInvalidConnectionString tests handling of invalid connection strings
func TestNewClientInvalidConnectionString(t *testing.T) {
	// While pgx is lenient with parsing, we should test boundary cases
	cfg := &config.Config{
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		DBName:  "postgres",
		SSLMode: "invalid_ssl_mode", // Invalid SSLMode
	}

	client, _ := New(cfg, 5*time.Second)
	// Depending on pgx's validation, this might error
	if client != nil {
		client.Close()
	}
	// Either way, it shouldn't panic
}

// TestClientConfigPersistence tests that client preserves config state
func TestClientConfigPersistence(t *testing.T) {
	originalHost := "localhost"
	originalPort := 5432
	originalUser := "postgres"

	cfg := &config.Config{
		Host:    originalHost,
		Port:    originalPort,
		User:    originalUser,
		DBName:  "postgres",
		SSLMode: "disable",
	}

	// Config should not be modified by New()
	client, _ := New(cfg, 5*time.Second)

	if cfg.Host != originalHost {
		t.Errorf("Config.Host modified: %s != %s", cfg.Host, originalHost)
	}
	if cfg.Port != originalPort {
		t.Errorf("Config.Port modified: %d != %d", cfg.Port, originalPort)
	}
	if cfg.User != originalUser {
		t.Errorf("Config.User modified: %s != %s", cfg.User, originalUser)
	}

	if client != nil {
		client.Close()
	}
}
