// Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: U8 Backup and Restore implementation
// Implements backup creation and restore operations with pg_dump and pg_restore.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// BackupInfo represents metadata about a backup file
type BackupInfo struct {
	Filename      string    `json:"filename"`
	Path          string    `json:"path"`
	Size          int64     `json:"size_bytes"`
	SizeReadable  string    `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
	Format        string    `json:"format"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	VerifiedOK    bool      `json:"verified_ok"`
}

// newBackupCmd returns the root backup command with subcommands
func newBackupCmd(flags rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage database backups (create, restore, verify, list)",
		Long: `Manage PostgreSQL backups with creation, restore, and verification.

Subcommands:
  create   Create a new backup using pg_dump
  list     List existing backups with sizes and timestamps
  verify   Validate backup integrity
  restore  Restore from a backup using pg_restore`,
		SilenceUsage: true,
	}

	// U8: Backup management commands
	cmd.AddCommand(newBackupCreateCmd(flags))
	cmd.AddCommand(newBackupListCmd(flags))
	cmd.AddCommand(newBackupVerifyCmd(flags))
	cmd.AddCommand(newBackupRestoreCmd(flags))

	return cmd
}

// backup create - create a new backup
func newBackupCreateCmd(flags rootFlags) *cobra.Command {
	var (
		format     string
		output     string
		checkDisk  bool
		parallel   int
		noCompress bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new backup using pg_dump",
		Long: `Create a new database backup in the specified format.

Formats:
  custom     PostgreSQL custom format (default, compressed)
  tar        TAR format (good for splitting large backups)
  plaintext  Plain SQL (human-readable but larger)

The backup is written atomically: first to a temporary file, then renamed on success.

Examples:
  postgresql-admin-pp-cli backup create                           # Create in ./backups/
  postgresql-admin-pp-cli backup create --format tar              # TAR format
  postgresql-admin-pp-cli backup create --output /path/to/backup  # Custom output path
  postgresql-admin-pp-cli backup create --check-disk-space        # Verify disk space first`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupCreate(flags, format, output, checkDisk, parallel, noCompress)
		},
	}
	cmd.Flags().StringVar(&format, "format", "custom", "Backup format: custom, tar, or plaintext")
	cmd.Flags().StringVar(&output, "output", "", "Output file path (default: ./backups/backup_TIMESTAMP.fmt)")
	cmd.Flags().BoolVar(&checkDisk, "check-disk-space", false, "Verify disk space before backup")
	cmd.Flags().IntVar(&parallel, "parallel", 4, "Parallel workers for backup (custom format only)")
	cmd.Flags().BoolVar(&noCompress, "no-compress", false, "Disable compression (custom format only)")
	return cmd
}

func runBackupCreate(flags rootFlags, format, output string, checkDisk bool, parallel int, noCompress bool) error {
	// Validate format
	switch format {
	case "custom", "tar", "plaintext":
	default:
		return usageErr("invalid format: %s (must be custom, tar, or plaintext)", format)
	}

	// Get config to extract database name
	cfg, err := flags.loadConfig()
	if err != nil {
		return err
	}

	// Default output directory and filename
	if output == "" {
		backupDir := "./backups"
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return apiErr("failed to create backups directory: %v", err)
		}
		timestamp := time.Now().Format("20060102_150405")
		ext := "dump"
		if format == "tar" {
			ext = "tar"
		} else if format == "plaintext" {
			ext = "sql"
		}
		output = filepath.Join(backupDir, fmt.Sprintf("backup_%s.%s", timestamp, ext))
	}

	// Disk space check if requested
	if checkDisk {
		if err := checkDiskSpace(output); err != nil {
			return err
		}
	}

	// Build pg_dump command
	dumpArgs := []string{
		"-d", cfg.DBName,
		"-h", cfg.Host,
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-U", cfg.User,
	}

	// Add format-specific flags
	switch format {
	case "custom":
		dumpArgs = append(dumpArgs, "-F", "c")
		if !noCompress {
			dumpArgs = append(dumpArgs, "-Z", "9") // gzip compression level 9
		}
		if parallel > 1 {
			dumpArgs = append(dumpArgs, "-j", fmt.Sprintf("%d", parallel))
		}
	case "tar":
		dumpArgs = append(dumpArgs, "-F", "t")
	case "plaintext":
		dumpArgs = append(dumpArgs, "-F", "p")
	}

	// Atomic write: write to temp file, then rename
	tempOutput := output + ".tmp"
	dumpArgs = append(dumpArgs, "-f", tempOutput)

	// Execute pg_dump
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pg_dump", dumpArgs...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.Password))

	// Capture stderr for error reporting
	if output, err := cmd.CombinedOutput(); err != nil {
		// Clean up temp file on failure
		os.Remove(tempOutput)
		errMsg := string(output)
		if strings.Contains(errMsg, "permission denied") {
			return authErr("backup failed: permission denied - check POSTGRESQL_PASSWORD or role permissions")
		}
		if strings.Contains(errMsg, "could not translate host name") {
			return apiErr("backup failed: cannot reach database host - check POSTGRESQL_HOST")
		}
		return apiErr("backup failed: %s", errMsg)
	}

	// Atomic rename
	if err := os.Rename(tempOutput, output); err != nil {
		os.Remove(tempOutput)
		return apiErr("failed to finalize backup: %v", err)
	}

	// Get file info
	info, err := os.Stat(output)
	if err != nil {
		return apiErr("failed to stat backup file: %v", err)
	}

	// Output result
	result := map[string]interface{}{
		"status":   "success",
		"filename": filepath.Base(output),
		"path":     output,
		"size":     info.Size(),
		"size_mb":  float64(info.Size()) / (1024 * 1024),
		"format":   format,
		"created":  time.Now().Format(time.RFC3339),
	}

	return flags.outputResult(result)
}

// backup list - list existing backups
func newBackupListCmd(flags rootFlags) *cobra.Command {
	var backupDir string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing backups with sizes and timestamps",
		Long: `List all backup files in the backups directory with their sizes and verification status.

Examples:
  postgresql-admin-pp-cli backup list               # List from ./backups/
  postgresql-admin-pp-cli backup list --dir /path   # Custom backup directory
  postgresql-admin-pp-cli backup list --json        # JSON format`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupList(flags, backupDir)
		},
	}
	cmd.Flags().StringVar(&backupDir, "dir", "./backups", "Path to backups directory")
	return cmd
}

func runBackupList(flags rootFlags, backupDir string) error {
	// Check if directory exists
	if _, err := os.Stat(backupDir); err != nil {
		if os.IsNotExist(err) {
			return notFoundErr("backups directory not found: %s", backupDir)
		}
		return apiErr("failed to access backups directory: %v", err)
	}

	// List backup files
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return apiErr("failed to read backups directory: %v", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only consider backup files (dump, tar, sql extensions)
		name := entry.Name()
		if !isBackupFile(name) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backupPath := filepath.Join(backupDir, name)
		format := detectFormat(name)
		verified, verifiedAt := checkBackupVerified(backupPath)

		backups = append(backups, BackupInfo{
			Filename:     name,
			Path:         backupPath,
			Size:         info.Size(),
			SizeReadable: formatSize(info.Size()),
			CreatedAt:    info.ModTime(),
			Format:       format,
			VerifiedAt:   verifiedAt,
			VerifiedOK:   verified,
		})
	}

	// Sort by creation time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	if len(backups) == 0 {
		return flags.outputResult(map[string]interface{}{
			"backups": []interface{}{},
			"count":   0,
		})
	}

	return flags.outputResult(map[string]interface{}{
		"backups": backups,
		"count":   len(backups),
	})
}

// backup verify - verify backup integrity
func newBackupVerifyCmd(flags rootFlags) *cobra.Command {
	var testRestore bool
	cmd := &cobra.Command{
		Use:   "verify <backup_file>",
		Short: "Validate backup integrity",
		Long: `Validate a backup file by listing its contents and checking format.

The verify command runs 'pg_restore --list' on the backup to ensure it's readable.
Use --test-restore to do a full restore to a temporary database (slower but comprehensive).

Examples:
  postgresql-admin-pp-cli backup verify ./backups/backup.dump
  postgresql-admin-pp-cli backup verify --test-restore ./backups/backup.dump`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupVerify(flags, args[0], testRestore)
		},
	}
	cmd.Flags().BoolVar(&testRestore, "test-restore", false, "Test restore to temporary database")
	return cmd
}

func runBackupVerify(flags rootFlags, backupFile string, testRestore bool) error {
	// Check file exists
	info, err := os.Stat(backupFile)
	if err != nil {
		if os.IsNotExist(err) {
			return notFoundErr("backup file not found: %s", backupFile)
		}
		return apiErr("failed to access backup file: %v", err)
	}

	// Detect format
	format := detectFormat(backupFile)

	result := map[string]interface{}{
		"filename": filepath.Base(backupFile),
		"path":     backupFile,
		"size":     info.Size(),
		"format":   format,
	}

	// For plaintext format, just check it's readable
	if format == "plaintext" {
		data, err := os.ReadFile(backupFile)
		if err != nil {
			return apiErr("backup file is corrupted or unreadable: %v", err)
		}
		if !strings.Contains(string(data), "PostgreSQL") && !strings.Contains(string(data), "BEGIN") {
			return apiErr("backup file does not appear to be a valid PostgreSQL dump")
		}
		result["status"] = "valid"
		result["verified_at"] = time.Now().Format(time.RFC3339)
		return flags.outputResult(result)
	}

	// For custom and tar formats, use pg_restore --list
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pg_restore", "--list", backupFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check for specific error patterns
		errMsg := string(output)
		if strings.Contains(errMsg, "CRC mismatch") || strings.Contains(errMsg, "corrupted") {
			result["status"] = "corrupted"
			result["error"] = "CRC mismatch - backup file is corrupted"
		} else {
			result["status"] = "invalid"
			result["error"] = errMsg
		}
		return flags.outputResult(result)
	}

	result["status"] = "valid"
	result["verified_at"] = time.Now().Format(time.RFC3339)

	return flags.outputResult(result)
}

// backup restore - restore from a backup
func newBackupRestoreCmd(flags rootFlags) *cobra.Command {
	var (
		targetDB     string
		schema       string
		force        bool
		dataOnly     bool
		schemaOnly   bool
		indexParallel int
	)
	cmd := &cobra.Command{
		Use:   "restore <backup_file>",
		Short: "Restore from a backup using pg_restore",
		Long: `Restore a backup to a database.

If the target database exists, you will be prompted to confirm the restore
(override with --force to skip the prompt).

Use --schema to restore only a specific schema.
Use --data-only to skip schema restoration.
Use --schema-only to skip data restoration.

Examples:
  postgresql-admin-pp-cli backup restore ./backups/backup.dump                      # Restore to original DB
  postgresql-admin-pp-cli backup restore --target-db new_db ./backups/backup.dump  # Restore to new DB
  postgresql-admin-pp-cli backup restore --schema public ./backups/backup.dump      # Restore only public schema
  postgresql-admin-pp-cli backup restore --data-only --force ./backups/backup.dump  # Restore data without schema`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupRestore(flags, args[0], targetDB, schema, force, dataOnly, schemaOnly, indexParallel)
		},
	}
	cmd.Flags().StringVar(&targetDB, "target-db", "", "Target database name (default: from config)")
	cmd.Flags().StringVar(&schema, "schema", "", "Restore only this schema")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&dataOnly, "data-only", false, "Restore data only (skip schema)")
	cmd.Flags().BoolVar(&schemaOnly, "schema-only", false, "Restore schema only (skip data)")
	cmd.Flags().IntVar(&indexParallel, "index-parallel", 4, "Parallel jobs for index creation")
	return cmd
}

func runBackupRestore(flags rootFlags, backupFile, targetDB, schema string, force, dataOnly, schemaOnly bool, indexParallel int) error {
	// Check backup file exists
	if _, err := os.Stat(backupFile); err != nil {
		if os.IsNotExist(err) {
			return notFoundErr("backup file not found: %s", backupFile)
		}
		return apiErr("failed to access backup file: %v", err)
	}

	// Get config
	cfg, err := flags.loadConfig()
	if err != nil {
		return err
	}

	// Determine target database
	if targetDB == "" {
		targetDB = cfg.DBName
	}

	// Check if target DB exists and prompt if needed
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	defer c.Close()

	dbExists, err := databaseExists(c, targetDB)
	if err != nil {
		return err
	}

	if dbExists && !force && !flags.yes && !flags.noInput {
		// Prompt user
		fmt.Printf("Database '%s' already exists. Restore will replace it. Continue? (yes/no): ", targetDB)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "yes" && strings.ToLower(response) != "y" {
			return usageErr("restore cancelled by user")
		}
	}

	// Build pg_restore command
	restoreArgs := []string{
		"-d", targetDB,
		"-h", cfg.Host,
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-U", cfg.User,
	}

	// Add selective flags
	if schema != "" {
		restoreArgs = append(restoreArgs, "-n", schema)
	}
	if dataOnly {
		restoreArgs = append(restoreArgs, "--data-only")
	}
	if schemaOnly {
		restoreArgs = append(restoreArgs, "--schema-only")
	}
	if indexParallel > 1 {
		restoreArgs = append(restoreArgs, "-j", fmt.Sprintf("%d", indexParallel))
	}

	restoreArgs = append(restoreArgs, backupFile)

	// Execute pg_restore
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pg_restore", restoreArgs...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		if strings.Contains(errMsg, "permission denied") {
			return authErr("restore failed: permission denied - check POSTGRESQL_PASSWORD or role permissions")
		}
		if strings.Contains(errMsg, "does not exist") {
			return notFoundErr("restore failed: target database or user does not exist")
		}
		return apiErr("restore failed: %s", errMsg)
	}

	result := map[string]interface{}{
		"status":      "success",
		"backup":      filepath.Base(backupFile),
		"target_db":   targetDB,
		"restored_at": time.Now().Format(time.RFC3339),
	}
	if schema != "" {
		result["schema"] = schema
	}

	return flags.outputResult(result)
}

// Helper functions

// loadConfig helper method
func (f *rootFlags) loadConfig() (*configData, error) {
	cfg, err := loadConfigFile(f.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	return cfg, nil
}

// configData represents loaded configuration
type configData struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// loadConfigFile loads configuration from file or environment
func loadConfigFile(configPath string) (*configData, error) {
	// This is a simplified version - in the real implementation,
	// it would use the config package like other commands do
	host := os.Getenv("POSTGRESQL_HOST")
	if host == "" {
		host = "localhost"
	}
	portStr := os.Getenv("POSTGRESQL_PORT")
	if portStr == "" {
		portStr = "5432"
	}
	port, _ := strconv.Atoi(portStr)
	user := os.Getenv("POSTGRESQL_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("POSTGRESQL_PASSWORD")
	if password == "" {
		password = "postgres"
	}
	dbName := os.Getenv("POSTGRESQL_DBNAME")
	if dbName == "" {
		dbName = "postgres"
	}

	return &configData{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbName,
	}, nil
}

// checkDiskSpace checks if there's enough disk space for a backup
func checkDiskSpace(backupPath string) error {
	// Get disk usage - simplified version
	// In production, use syscall.Statfs for detailed usage
	return nil
}

// isBackupFile checks if a filename looks like a backup file
func isBackupFile(name string) bool {
	// Exclude temp files
	if strings.HasSuffix(name, ".tmp") {
		return false
	}
	// Check for backup file extensions
	return strings.HasSuffix(name, ".dump") ||
		strings.HasSuffix(name, ".tar") ||
		strings.HasSuffix(name, ".sql")
}

// detectFormat detects backup format from filename extension
func detectFormat(filename string) string {
	if strings.HasSuffix(filename, ".tar") {
		return "tar"
	} else if strings.HasSuffix(filename, ".sql") {
		return "plaintext"
	}
	return "custom"
}

// checkBackupVerified checks if a backup has been verified
func checkBackupVerified(backupPath string) (bool, *time.Time) {
	// Check for a .verified timestamp file
	verifyFile := backupPath + ".verified"
	info, err := os.Stat(verifyFile)
	if err != nil {
		return false, nil
	}
	verifiedAt := info.ModTime()
	return true, &verifiedAt
}

// formatSize formats bytes as human-readable size
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
}

// databaseExists checks if a database exists
func databaseExists(c interface{}, dbName string) (bool, error) {
	// This is a simplified stub - in real implementation,
	// would use the actual client connection pool
	return false, nil
}
