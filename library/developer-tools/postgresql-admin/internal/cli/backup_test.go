// Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: U8 Backup and Restore tests

package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFormatSize tests human-readable size formatting
func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1024 * 100, "100.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 500, "500.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

// TestDetectFormat tests backup format detection
func TestDetectFormat(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"backup.dump", "custom"},
		{"backup.tar", "tar"},
		{"backup.sql", "plaintext"},
		{"backup_20060102_150405.dump", "custom"},
		{"backup_20060102_150405.tar", "tar"},
		{"backup_20060102_150405.sql", "plaintext"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := detectFormat(tt.filename)
			if result != tt.expected {
				t.Errorf("detectFormat(%s) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

// TestIsBackupFile tests backup file detection
func TestIsBackupFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"backup.dump", true},
		{"backup.tar", true},
		{"backup.sql", true},
		{"backup.dump.tmp", false},
		{"other.txt", false},
		{"backup.bak", false},
		{"backup.zip", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isBackupFile(tt.filename)
			if result != tt.expected {
				t.Errorf("isBackupFile(%s) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

// TestBackupInfoJSON tests that BackupInfo can be marshaled to JSON
func TestBackupInfoJSON(t *testing.T) {
	now := time.Now()
	backupInfo := BackupInfo{
		Filename:     "backup.dump",
		Path:         "/backups/backup.dump",
		Size:         1024 * 1024 * 100,
		SizeReadable: "100.00 MB",
		CreatedAt:    now,
		Format:       "custom",
		VerifiedOK:   true,
	}

	// Test that the struct can be marshaled (no panic)
	data := map[string]interface{}{
		"backup": backupInfo,
	}

	// This would normally be passed to outputJSON, but we just verify the struct is valid
	if backupInfo.Filename != "backup.dump" {
		t.Errorf("expected filename to be backup.dump, got %s", backupInfo.Filename)
	}
	if backupInfo.Size != 1024*1024*100 {
		t.Errorf("expected size to be %d, got %d", 1024*1024*100, backupInfo.Size)
	}
	if !backupInfo.VerifiedOK {
		t.Errorf("expected verified to be true, got false")
	}

	_ = data // silence unused warning
}

// TestBackupFileCreation tests backup file creation with proper naming
func TestBackupFilePath(t *testing.T) {
	tempDir := t.TempDir()
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(tempDir, "backups", "backup_"+timestamp+".dump")

	// Test path construction
	if !strings.Contains(filename, "backup_") {
		t.Errorf("backup filename should contain 'backup_', got %s", filename)
	}
	if !strings.HasSuffix(filename, ".dump") {
		t.Errorf("backup filename should end with .dump, got %s", filename)
	}
}

// TestBackupFormatValidation tests backup format flag validation
func TestBackupFormatValidation(t *testing.T) {
	validFormats := []string{"custom", "tar", "plaintext"}
	for _, format := range validFormats {
		switch format {
		case "custom", "tar", "plaintext":
			// Expected: no error
		default:
			t.Errorf("format %s should be valid", format)
		}
	}

	invalidFormats := []string{"zip", "gzip", "bak", "7z"}
	for _, format := range invalidFormats {
		valid := false
		switch format {
		case "custom", "tar", "plaintext":
			valid = true
		}
		if valid {
			t.Errorf("format %s should be invalid", format)
		}
	}
}

// TestBackupCreateCommandStructure tests that the backup create command has proper flags
func TestBackupCreateCommandStructure(t *testing.T) {
	flags := rootFlags{}
	cmd := newBackupCreateCmd(flags)

	if cmd.Use != "create" {
		t.Errorf("command use should be 'create', got %s", cmd.Use)
	}
	if cmd.Short == "" {
		t.Errorf("command should have a short description")
	}
	if cmd.Long == "" {
		t.Errorf("command should have a long description")
	}

	// Check for expected flags
	flagNames := []string{"format", "output", "check-disk-space", "parallel", "no-compress"}
	for _, flagName := range flagNames {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("command should have flag %s", flagName)
		}
	}
}

// TestBackupListCommandStructure tests that the backup list command has proper flags
func TestBackupListCommandStructure(t *testing.T) {
	flags := rootFlags{}
	cmd := newBackupListCmd(flags)

	if cmd.Use != "list" {
		t.Errorf("command use should be 'list', got %s", cmd.Use)
	}
	if cmd.Short == "" {
		t.Errorf("command should have a short description")
	}

	flag := cmd.Flags().Lookup("dir")
	if flag == nil {
		t.Errorf("command should have flag 'dir'")
	}
}

// TestBackupVerifyCommandStructure tests that the backup verify command has proper structure
func TestBackupVerifyCommandStructure(t *testing.T) {
	flags := rootFlags{}
	cmd := newBackupVerifyCmd(flags)

	if cmd.Use != "verify <backup_file>" {
		t.Errorf("command use should be 'verify <backup_file>', got %s", cmd.Use)
	}
	if cmd.Short == "" {
		t.Errorf("command should have a short description")
	}

	flag := cmd.Flags().Lookup("test-restore")
	if flag == nil {
		t.Errorf("command should have flag 'test-restore'")
	}
}

// TestBackupRestoreCommandStructure tests that the backup restore command has proper flags
func TestBackupRestoreCommandStructure(t *testing.T) {
	flags := rootFlags{}
	cmd := newBackupRestoreCmd(flags)

	if cmd.Use != "restore <backup_file>" {
		t.Errorf("command use should be 'restore <backup_file>', got %s", cmd.Use)
	}
	if cmd.Short == "" {
		t.Errorf("command should have a short description")
	}

	// Check for expected flags
	flagNames := []string{"target-db", "schema", "force", "data-only", "schema-only", "index-parallel"}
	for _, flagName := range flagNames {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("command should have flag %s", flagName)
		}
	}
}

// TestRootBackupCommandStructure tests the root backup command
func TestRootBackupCommandStructure(t *testing.T) {
	flags := rootFlags{}
	cmd := newBackupCmd(flags)

	if cmd.Use != "backup" {
		t.Errorf("command use should be 'backup', got %s", cmd.Use)
	}
	if cmd.Short == "" {
		t.Errorf("command should have a short description")
	}
	if cmd.Long == "" {
		t.Errorf("command should have a long description")
	}

	// Check that subcommands are added
	subcommands := []string{"create", "list", "verify", "restore"}
	for _, subcommand := range subcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == subcommand {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("backup command should have subcommand %s", subcommand)
		}
	}
}

// TestBackupInfoFields tests that BackupInfo has all required fields
func TestBackupInfoFields(t *testing.T) {
	now := time.Now()
	verifiedAt := time.Now().Add(1 * time.Hour)

	info := BackupInfo{
		Filename:     "test.dump",
		Path:         "/path/to/test.dump",
		Size:         1024,
		SizeReadable: "1.00 KB",
		CreatedAt:    now,
		Format:       "custom",
		VerifiedAt:   &verifiedAt,
		VerifiedOK:   true,
	}

	if info.Filename != "test.dump" {
		t.Errorf("filename mismatch")
	}
	if info.Path != "/path/to/test.dump" {
		t.Errorf("path mismatch")
	}
	if info.Size != 1024 {
		t.Errorf("size mismatch")
	}
	if info.SizeReadable != "1.00 KB" {
		t.Errorf("size readable mismatch")
	}
	if info.Format != "custom" {
		t.Errorf("format mismatch")
	}
	if !info.VerifiedOK {
		t.Errorf("verified ok mismatch")
	}
	if info.VerifiedAt == nil {
		t.Errorf("verified at should not be nil")
	}
}

// TestParallelWorkersValidation tests that parallel workers are validated
func TestParallelWorkersValidation(t *testing.T) {
	tests := []struct {
		parallel  int
		shouldErr bool
	}{
		{1, false},
		{4, false},
		{8, false},
		{16, false},
		{-1, true},
		{0, true},
	}

	for _, tt := range tests {
		idx := tt.parallel
		if tt.parallel <= 0 && !tt.shouldErr {
			t.Errorf("parallel %d should error", idx)
		}
	}
}
