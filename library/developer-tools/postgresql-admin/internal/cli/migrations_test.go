// Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: U6 Migration Management tests

package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadMigrationsFromDisk tests loading migrations from a temporary directory
func TestLoadMigrationsFromDisk(t *testing.T) {
	// Create temporary directory with migration files
	tmpDir := t.TempDir()

	// Create test migrations
	migrations := map[string]struct {
		upSQL   string
		downSQL string
	}{
		"001_init": {
			upSQL:   "CREATE TABLE users (id SERIAL PRIMARY KEY);",
			downSQL: "DROP TABLE users;",
		},
		"002_add_email": {
			upSQL:   "ALTER TABLE users ADD COLUMN email TEXT;",
			downSQL: "ALTER TABLE users DROP COLUMN email;",
		},
	}

	// Write files to disk
	for name, content := range migrations {
		upPath := filepath.Join(tmpDir, name+".up.sql")
		downPath := filepath.Join(tmpDir, name+".down.sql")

		if err := os.WriteFile(upPath, []byte(content.upSQL), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", upPath, err)
		}
		if err := os.WriteFile(downPath, []byte(content.downSQL), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", downPath, err)
		}
	}

	// Load migrations
	migs, err := loadMigrationsFromDisk(tmpDir)
	if err != nil {
		t.Fatalf("loadMigrationsFromDisk failed: %v", err)
	}

	// Verify results
	if len(migs) != 2 {
		t.Errorf("expected 2 migrations, got %d", len(migs))
	}

	// Verify sorting (001 should come before 002)
	if len(migs) >= 2 {
		if migs[0].Name != "001_init" {
			t.Errorf("expected first migration to be 001_init, got %s", migs[0].Name)
		}
		if migs[1].Name != "002_add_email" {
			t.Errorf("expected second migration to be 002_add_email, got %s", migs[1].Name)
		}
	}

	// Verify paths
	if migs[0].UpPath == "" || migs[0].DownPath == "" {
		t.Error("migration paths not set correctly")
	}
}

// TestLoadMigrationsFromDiskMissingDown tests handling of migrations with missing down.sql
func TestLoadMigrationsFromDiskMissingDown(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only up.sql (missing down.sql)
	upPath := filepath.Join(tmpDir, "001_init.up.sql")
	if err := os.WriteFile(upPath, []byte("CREATE TABLE users (id INT);"), 0644); err != nil {
		t.Fatalf("failed to write up file: %v", err)
	}

	// Load migrations
	migs, err := loadMigrationsFromDisk(tmpDir)
	if err != nil {
		t.Fatalf("loadMigrationsFromDisk failed: %v", err)
	}

	// Should return empty (migration not included without both up and down)
	if len(migs) != 0 {
		t.Errorf("expected 0 migrations (missing down.sql), got %d", len(migs))
	}
}

// TestCalculateChecksum tests SHA256 checksum calculation
func TestCalculateChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.sql")
	content := "CREATE TABLE test (id INT);"

	// Write test file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Calculate checksum
	checksum := calculateChecksum(filePath)

	// Verify against expected SHA256
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	if checksum != expected {
		t.Errorf("checksum mismatch: expected %s, got %s", expected, checksum)
	}
}

// TestCalculateChecksumNonexistent tests checksum of nonexistent file
func TestCalculateChecksumNonexistent(t *testing.T) {
	checksum := calculateChecksum("/nonexistent/file.sql")
	if checksum != "" {
		t.Errorf("expected empty checksum for nonexistent file, got %s", checksum)
	}
}

// TestMigrationStructure tests the Migration struct fields
func TestMigrationStructure(t *testing.T) {
	now := time.Now()
	mig := &Migration{
		ID:            1,
		Name:          "001_test",
		UpPath:        "/path/to/001_test.up.sql",
		DownPath:      "/path/to/001_test.down.sql",
		Status:        "applied",
		AppliedAt:     &now,
		Checksum:      "abc123def456",
		FileChecksum:  "abc123def456",
		ExecutionTime: 123,
	}

	// Verify all fields are accessible
	if mig.ID != 1 {
		t.Error("ID field not set correctly")
	}
	if mig.Name != "001_test" {
		t.Error("Name field not set correctly")
	}
	if mig.Status != "applied" {
		t.Error("Status field not set correctly")
	}
	if mig.AppliedAt == nil || mig.AppliedAt != &now {
		t.Error("AppliedAt field not set correctly")
	}
	if mig.ExecutionTime != 123 {
		t.Error("ExecutionTime field not set correctly")
	}
}

// TestMigrationStatusValues tests migration status constants
func TestMigrationStatusValues(t *testing.T) {
	statuses := []string{
		"pending",
		"applied",
		"failed",
	}

	for _, status := range statuses {
		mig := &Migration{Status: status}
		if mig.Status != status {
			t.Errorf("status %q not set correctly", status)
		}
	}
}

// TestLoadMigrationsFromDiskEmptyDir tests loading migrations from empty directory
func TestLoadMigrationsFromDiskEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Load migrations from empty directory
	migs, err := loadMigrationsFromDisk(tmpDir)
	if err != nil {
		t.Fatalf("loadMigrationsFromDisk failed: %v", err)
	}

	if len(migs) != 0 {
		t.Errorf("expected 0 migrations from empty dir, got %d", len(migs))
	}
}

// TestLoadMigrationsFromDiskNonexistent tests loading from nonexistent directory
func TestLoadMigrationsFromDiskNonexistent(t *testing.T) {
	// Load migrations from nonexistent directory
	migs, err := loadMigrationsFromDisk("/nonexistent/migrations/dir")

	// Should return nil (no error, but no migrations)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if migs != nil && len(migs) > 0 {
		t.Errorf("expected no migrations from nonexistent dir, got %d", len(migs))
	}
}

// TestLoadMigrationsFromDiskSorting tests correct sorting of migrations
func TestLoadMigrationsFromDiskSorting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create migrations in reverse order (to test sorting)
	order := []string{"003", "001", "002"}
	for _, num := range order {
		upPath := filepath.Join(tmpDir, num+"_test.up.sql")
		downPath := filepath.Join(tmpDir, num+"_test.down.sql")

		if err := os.WriteFile(upPath, []byte("-- up"), 0644); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
		if err := os.WriteFile(downPath, []byte("-- down"), 0644); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
	}

	// Load migrations
	migs, err := loadMigrationsFromDisk(tmpDir)
	if err != nil {
		t.Fatalf("loadMigrationsFromDisk failed: %v", err)
	}

	// Verify sorted order
	if len(migs) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migs))
	}

	expectedOrder := []string{"001_test", "002_test", "003_test"}
	for i, expected := range expectedOrder {
		if migs[i].Name != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, migs[i].Name)
		}
	}
}

// TestLoadMigrationsParsesMigrationName tests that migration name is correctly parsed
func TestLoadMigrationsParsesMigrationName(t *testing.T) {
	tmpDir := t.TempDir()

	// Create migration with descriptive name
	name := "001_create_users_table"
	upPath := filepath.Join(tmpDir, name+".up.sql")
	downPath := filepath.Join(tmpDir, name+".down.sql")

	if err := os.WriteFile(upPath, []byte("CREATE TABLE users (id INT);"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := os.WriteFile(downPath, []byte("DROP TABLE users;"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Load migrations
	migs, err := loadMigrationsFromDisk(tmpDir)
	if err != nil {
		t.Fatalf("loadMigrationsFromDisk failed: %v", err)
	}

	if len(migs) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(migs))
	}

	if migs[0].Name != name {
		t.Errorf("expected name %s, got %s", name, migs[0].Name)
	}
}

// BenchmarkCalculateChecksum benchmarks the checksum calculation
func BenchmarkCalculateChecksum(b *testing.B) {
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "test.sql")
	content := "CREATE TABLE test (id INT PRIMARY KEY, name TEXT, email TEXT);"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateChecksum(filePath)
	}
}

// TestMigrationFilePaths tests that file paths are correctly set
func TestMigrationFilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	migName := "001_init"

	upPath := filepath.Join(tmpDir, migName+".up.sql")
	downPath := filepath.Join(tmpDir, migName+".down.sql")

	if err := os.WriteFile(upPath, []byte("-- up"), 0644); err != nil {
		t.Fatalf("failed to write up file: %v", err)
	}
	if err := os.WriteFile(downPath, []byte("-- down"), 0644); err != nil {
		t.Fatalf("failed to write down file: %v", err)
	}

	migs, err := loadMigrationsFromDisk(tmpDir)
	if err != nil {
		t.Fatalf("loadMigrationsFromDisk failed: %v", err)
	}

	if len(migs) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(migs))
	}

	mig := migs[0]
	if !filepath.IsAbs(mig.UpPath) && !filepath.IsAbs(upPath) {
		// Allow both absolute and relative paths for this test
	}
	if mig.UpPath == "" {
		t.Error("up path not set")
	}
	if mig.DownPath == "" {
		t.Error("down path not set")
	}
}

// TestMigrationStatusTransitions tests migration status transitions
func TestMigrationStatusTransitions(t *testing.T) {
	mig := &Migration{
		ID:     1,
		Name:   "001_test",
		Status: "pending",
	}

	// Transition to applied
	mig.Status = "applied"
	if mig.Status != "applied" {
		t.Error("status transition to applied failed")
	}

	// Transition to failed
	mig.Status = "failed"
	if mig.Status != "failed" {
		t.Error("status transition to failed failed")
	}
}

// TestCheckCircularDependencies tests the circular dependency check (placeholder)
func TestCheckCircularDependencies(t *testing.T) {
	// Create test migrations with no dependencies
	migs := []*Migration{
		{Name: "001_init"},
		{Name: "002_add_users"},
	}
	applied := make(map[string]*Migration)

	// Should not return error for valid migrations
	err := checkCircularDependencies(migs, applied)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestMigrationTimestamp tests that timestamps are properly handled
func TestMigrationTimestamp(t *testing.T) {
	now := time.Now()
	mig := &Migration{
		Name:      "001_test",
		AppliedAt: &now,
	}

	if mig.AppliedAt == nil {
		t.Error("AppliedAt should not be nil")
	}

	// Verify timestamp is set correctly
	if mig.AppliedAt.Unix() != now.Unix() {
		t.Error("timestamp not set correctly")
	}
}

// TestMigrationChecksumFormat tests checksum format
func TestMigrationChecksumFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.sql")

	if err := os.WriteFile(filePath, []byte("SELECT 1;"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	checksum := calculateChecksum(filePath)

	// Verify checksum is hex string
	if len(checksum) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("expected 64 hex characters, got %d", len(checksum))
	}

	// Verify all characters are hex
	for _, ch := range checksum {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("invalid hex character: %c", ch)
		}
	}
}
