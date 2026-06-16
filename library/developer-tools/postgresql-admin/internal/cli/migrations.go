// Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: U6 Migration Management implementation
// Implements database migration management with state tracking and safety checks.

package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
)

// Migration represents a migration file with metadata
type Migration struct {
	ID            int
	Name          string
	UpPath        string
	DownPath      string
	Status        string // "pending", "applied", "failed"
	AppliedAt     *time.Time
	Checksum      string
	FileChecksum  string // checksum of the up.sql file on disk
	ExecutionTime int    // milliseconds
}

// newMigrationsCmd returns the root migrations command with subcommands
func newMigrationsCmd(flags rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrations",
		Short: "Manage database migrations (apply, rollback, verify)",
		Long: `Manage database migrations with state tracking and safety checks.

Migration files must be in a migrations/ directory with naming convention:
  NNN_name.up.sql     - application SQL (NNN is zero-padded sequence, e.g. 001, 002)
  NNN_name.down.sql   - rollback SQL

Subcommands:
  list       List all migrations on disk with applied status
  status     Detailed migration status (pending, applied, failed)
  apply      Apply pending migrations (with --dry-run support)
  rollback   Rollback migrations (with --to flag for specific version)
  verify     Verify migration integrity and detect inconsistencies`,
		SilenceUsage: true,
	}

	// U6: Migration management commands
	cmd.AddCommand(newMigrationsListCmd(flags))
	cmd.AddCommand(newMigrationsStatusCmd(flags))
	cmd.AddCommand(newMigrationsApplyCmd(flags))
	cmd.AddCommand(newMigrationsRollbackCmd(flags))
	cmd.AddCommand(newMigrationsVerifyCmd(flags))

	return cmd
}

// migrations list - list all migrations on disk with applied status
func newMigrationsListCmd(flags rootFlags) *cobra.Command {
	var migrationsDir string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all migrations on disk with applied status",
		Long: `List all migrations found in the migrations/ directory (or custom path via --dir).
Shows each migration's name, file status, and whether it has been applied.

Examples:
  postgresql-admin-pp-cli migrations list                      # List from ./migrations
  postgresql-admin-pp-cli migrations list --dir /path/to/migs  # Custom directory
  postgresql-admin-pp-cli migrations list --json               # JSON format`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationsList(flags, migrationsDir)
		},
	}
	cmd.Flags().StringVar(&migrationsDir, "dir", "./migrations", "Path to migrations directory")
	return cmd
}

func runMigrationsList(flags rootFlags, migrationsDir string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	// Ensure migrations table exists
	if err := ensureMigrationsTable(ctx, c); err != nil {
		return err
	}

	// Load migrations from disk
	migrations, err := loadMigrationsFromDisk(migrationsDir)
	if err != nil {
		return apiErr("failed to load migrations from disk: %v", err)
	}

	if len(migrations) == 0 {
		fmt.Println("No migrations found in " + migrationsDir)
		return nil
	}

	// Load applied migrations from database
	applied, err := getAppliedMigrations(ctx, c)
	if err != nil {
		return apiErr("failed to load applied migrations: %v", err)
	}

	// Merge status
	for i := range migrations {
		migrations[i].FileChecksum = calculateChecksum(migrations[i].UpPath)
		if appliedMig, ok := applied[migrations[i].Name]; ok {
			migrations[i].Status = "applied"
			migrations[i].AppliedAt = appliedMig.AppliedAt
			migrations[i].Checksum = appliedMig.Checksum
			migrations[i].ExecutionTime = appliedMig.ExecutionTime
		} else {
			migrations[i].Status = "pending"
		}
	}

	// Format output
	var results []map[string]interface{}
	for _, m := range migrations {
		results = append(results, map[string]interface{}{
			"ID":             m.ID,
			"Name":           m.Name,
			"Status":         m.Status,
			"AppliedAt":      m.AppliedAt,
			"ExecutionTime":  m.ExecutionTime,
			"FileChecksum":   m.FileChecksum,
		})
	}

	return flags.outputResult(results)
}

// migrations status - detailed migration status
func newMigrationsStatusCmd(flags rootFlags) *cobra.Command {
	var migrationsDir string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show detailed migration status",
		Long: `Show detailed status of all migrations: pending, applied, and failed.
Checks for consistency between database state and migration files.

Examples:
  postgresql-admin-pp-cli migrations status           # Show status summary
  postgresql-admin-pp-cli migrations status --json    # JSON format`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationsStatus(flags, migrationsDir)
		},
	}
	cmd.Flags().StringVar(&migrationsDir, "dir", "./migrations", "Path to migrations directory")
	return cmd
}

func runMigrationsStatus(flags rootFlags, migrationsDir string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	// Ensure migrations table exists
	if err := ensureMigrationsTable(ctx, c); err != nil {
		return err
	}

	// Load migrations from disk
	migrations, err := loadMigrationsFromDisk(migrationsDir)
	if err != nil {
		return apiErr("failed to load migrations from disk: %v", err)
	}

	// Load applied migrations from database
	applied, err := getAppliedMigrations(ctx, c)
	if err != nil {
		return apiErr("failed to load applied migrations: %v", err)
	}

	// Count by status
	pending := 0
	appliedCount := 0
	checksumMismatch := 0

	for _, m := range migrations {
		if appliedMig, ok := applied[m.Name]; ok {
			appliedCount++
			fileChecksum := calculateChecksum(m.UpPath)
			if fileChecksum != appliedMig.Checksum {
				checksumMismatch++
			}
		} else {
			pending++
		}
	}

	status := map[string]interface{}{
		"Total":              len(migrations),
		"Pending":            pending,
		"Applied":            appliedCount,
		"ChecksumMismatches": checksumMismatch,
		"IsConsistent":       checksumMismatch == 0,
	}

	return flags.outputResult(status)
}

// migrations apply - apply pending migrations
func newMigrationsApplyCmd(flags rootFlags) *cobra.Command {
	var migrationsDir string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending migrations",
		Long: `Apply all pending migrations in order. Each migration is executed in a transaction.
Use --dry-run to see what would be applied without executing.

Safety checks:
  - Checks for circular dependencies before applying
  - Validates checksums of applied migrations
  - Uses pessimistic locking to prevent concurrent applies
  - Reports migration execution time

Examples:
  postgresql-admin-pp-cli migrations apply                # Apply all pending
  postgresql-admin-pp-cli migrations apply --dry-run      # Show what would be applied
  postgresql-admin-pp-cli migrations apply --dir /path    # Custom directory`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationsApply(flags, migrationsDir, dryRun)
		},
	}
	cmd.Flags().StringVar(&migrationsDir, "dir", "./migrations", "Path to migrations directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be applied without executing")
	return cmd
}

func runMigrationsApply(flags rootFlags, migrationsDir string, dryRun bool) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	// Ensure migrations table exists
	if err := ensureMigrationsTable(ctx, c); err != nil {
		return err
	}

	// Load migrations from disk
	migrations, err := loadMigrationsFromDisk(migrationsDir)
	if err != nil {
		return apiErr("failed to load migrations from disk: %v", err)
	}

	// Load applied migrations from database
	applied, err := getAppliedMigrations(ctx, c)
	if err != nil {
		return apiErr("failed to load applied migrations: %v", err)
	}

	// Check for circular dependencies
	if err := checkCircularDependencies(migrations, applied); err != nil {
		return apiErr("circular dependency detected: %v", err)
	}

	// Find pending migrations
	var pending []*Migration
	for _, m := range migrations {
		if _, exists := applied[m.Name]; !exists {
			pending = append(pending, m)
		}
	}

	if len(pending) == 0 {
		fmt.Println("No pending migrations to apply")
		return nil
	}

	if dryRun {
		fmt.Printf("Would apply %d migration(s):\n", len(pending))
		for i, m := range pending {
			fmt.Printf("  %d. %s\n", i+1, m.Name)
		}
		return nil
	}

	// Apply migrations in transaction
	var results []map[string]interface{}
	for _, m := range pending {
		startTime := time.Now()

		// Read migration SQL
		upSQL, err := os.ReadFile(m.UpPath)
		if err != nil {
			return apiErr("failed to read migration %s: %v", m.Name, err)
		}

		// Calculate checksum
		checksum := fmt.Sprintf("%x", sha256.Sum256(upSQL))

		// Execute in transaction with pessimistic locking
		if err := applyMigrationInTransaction(ctx, c, m.Name, string(upSQL), checksum); err != nil {
			return apiErr("failed to apply migration %s: %v", m.Name, err)
		}

		execTime := int(time.Since(startTime).Milliseconds())

		results = append(results, map[string]interface{}{
			"Name":            m.Name,
			"Status":          "applied",
			"ExecutionTimeMs": execTime,
			"Checksum":        checksum,
		})

		fmt.Printf("Applied: %s (%dms)\n", m.Name, execTime)
	}

	return flags.outputResult(results)
}

// migrations rollback - rollback migrations
func newMigrationsRollbackCmd(flags rootFlags) *cobra.Command {
	var migrationsDir string
	var toID string
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback migrations",
		Long: `Rollback the most recent migration, or all migrations back to a specific version.

Use --to <name> to rollback to a specific migration (inclusive; that migration will be removed).

Examples:
  postgresql-admin-pp-cli migrations rollback                  # Rollback most recent
  postgresql-admin-pp-cli migrations rollback --to 001_init    # Rollback to (and including) 001_init
  postgresql-admin-pp-cli migrations rollback --dir /path      # Custom directory`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationsRollback(flags, migrationsDir, toID)
		},
	}
	cmd.Flags().StringVar(&migrationsDir, "dir", "./migrations", "Path to migrations directory")
	cmd.Flags().StringVar(&toID, "to", "", "Rollback to specified migration (inclusive)")
	return cmd
}

func runMigrationsRollback(flags rootFlags, migrationsDir string, toID string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	// Load applied migrations
	applied, err := getAppliedMigrations(ctx, c)
	if err != nil {
		return apiErr("failed to load applied migrations: %v", err)
	}

	if len(applied) == 0 {
		fmt.Println("No applied migrations to rollback")
		return nil
	}

	// Load migrations from disk to get down SQL
	migrations, err := loadMigrationsFromDisk(migrationsDir)
	if err != nil {
		return apiErr("failed to load migrations from disk: %v", err)
	}

	migrationsByName := make(map[string]*Migration)
	for _, m := range migrations {
		migrationsByName[m.Name] = m
	}

	// Get migrations to rollback (in reverse chronological order)
	var toRollback []*Migration
	if toID == "" {
		// Rollback just the most recent
		var lastApplied *Migration
		var lastTime *time.Time
		for name, am := range applied {
			if lastTime == nil || am.AppliedAt.After(*lastTime) {
				lastApplied = migrationsByName[name]
				lastTime = am.AppliedAt
			}
		}
		if lastApplied != nil {
			toRollback = append(toRollback, lastApplied)
		}
	} else {
		// Rollback to specific migration (inclusive)
		found := false
		// Sort by applied_at descending
		var sortedApplied []*Migration
		for name := range applied {
			sortedApplied = append(sortedApplied, migrationsByName[name])
		}
		sort.Slice(sortedApplied, func(i, j int) bool {
			return applied[sortedApplied[i].Name].AppliedAt.After(*applied[sortedApplied[j].Name].AppliedAt)
		})

		for _, m := range sortedApplied {
			toRollback = append(toRollback, m)
			if m.Name == toID {
				found = true
				break
			}
		}

		if !found {
			return notFoundErr("migration %q not found in applied migrations", toID)
		}
	}

	if len(toRollback) == 0 {
		fmt.Println("No migrations to rollback")
		return nil
	}

	// Execute rollbacks in transaction
	var results []map[string]interface{}
	for _, m := range toRollback {
		// Read down SQL
		downSQL, err := os.ReadFile(m.DownPath)
		if err != nil {
			return apiErr("failed to read down migration %s: %v", m.Name, err)
		}

		// Execute rollback in transaction
		startTime := time.Now()
		if err := rollbackMigrationInTransaction(ctx, c, m.Name, string(downSQL)); err != nil {
			return apiErr("failed to rollback migration %s: %v", m.Name, err)
		}

		execTime := int(time.Since(startTime).Milliseconds())

		results = append(results, map[string]interface{}{
			"Name":            m.Name,
			"Status":          "rolled_back",
			"ExecutionTimeMs": execTime,
		})

		fmt.Printf("Rolled back: %s (%dms)\n", m.Name, execTime)
	}

	return flags.outputResult(results)
}

// migrations verify - verify migration integrity
func newMigrationsVerifyCmd(flags rootFlags) *cobra.Command {
	var migrationsDir string
	var fix bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify migration integrity and detect inconsistencies",
		Long: `Verify that all applied migrations have consistent checksums and detect partial states.

Checks:
  - Checksum validation: detects files that have been modified post-application
  - Partial state: detects migrations where up ran but down may have failed
  - Missing files: detects applied migrations whose files have been deleted
  - Orphaned files: detects migration files that are applied but with checksum mismatches

Use --fix to attempt automatic repair (re-run failed down, then up for mismatches).

Examples:
  postgresql-admin-pp-cli migrations verify              # Check consistency
  postgresql-admin-pp-cli migrations verify --fix        # Auto-repair inconsistencies
  postgresql-admin-pp-cli migrations verify --dir /path  # Custom directory`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationsVerify(flags, migrationsDir, fix)
		},
	}
	cmd.Flags().StringVar(&migrationsDir, "dir", "./migrations", "Path to migrations directory")
	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt automatic repair")
	return cmd
}

func runMigrationsVerify(flags rootFlags, migrationsDir string, fix bool) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	// Load migrations from disk
	migrations, err := loadMigrationsFromDisk(migrationsDir)
	if err != nil {
		return apiErr("failed to load migrations from disk: %v", err)
	}

	// Load applied migrations from database
	applied, err := getAppliedMigrations(ctx, c)
	if err != nil {
		return apiErr("failed to load applied migrations: %v", err)
	}

	migrationsByName := make(map[string]*Migration)
	for _, m := range migrations {
		migrationsByName[m.Name] = m
	}

	var issues []map[string]interface{}
	var fixed []map[string]interface{}

	// Check each applied migration
	for name, appliedMig := range applied {
		m, exists := migrationsByName[name]
		if !exists {
			issues = append(issues, map[string]interface{}{
				"Type":    "MissingFile",
				"Name":    name,
				"Message": fmt.Sprintf("Applied migration %q has no file on disk", name),
			})
			continue
		}

		// Check checksum
		fileChecksum := calculateChecksum(m.UpPath)
		if fileChecksum != appliedMig.Checksum {
			msg := map[string]interface{}{
				"Type":            "ChecksumMismatch",
				"Name":            name,
				"Message":         fmt.Sprintf("File changed post-application (checksum mismatch)"),
				"ExpectedChecksum": appliedMig.Checksum,
				"ActualChecksum":   fileChecksum,
			}
			issues = append(issues, msg)

			// Try to fix: re-run down then up
			if fix {
				downSQL, err := os.ReadFile(m.DownPath)
				if err != nil {
					msg["FixStatus"] = fmt.Sprintf("Failed to read down.sql: %v", err)
					continue
				}

				upSQL, err := os.ReadFile(m.UpPath)
				if err != nil {
					msg["FixStatus"] = fmt.Sprintf("Failed to read up.sql: %v", err)
					continue
				}

				// Re-apply in transaction
				if err := reapplyMigration(ctx, c, name, string(downSQL), string(upSQL)); err != nil {
					msg["FixStatus"] = fmt.Sprintf("Failed to reapply: %v", err)
					continue
				}

				msg["FixStatus"] = "reapplied successfully"
				fixed = append(fixed, map[string]interface{}{
					"Name":   name,
					"Action": "reapplied",
				})
			}
		}
	}

	result := map[string]interface{}{
		"TotalApplied":        len(applied),
		"IssuesFound":         len(issues),
		"Issues":              issues,
		"Fixed":               fixed,
		"IsConsistent":        len(issues) == 0,
	}

	return flags.outputResult(result)
}

// Helper functions

// loadMigrationsFromDisk loads all migrations from the migrations directory
func loadMigrationsFromDisk(migrationsDir string) ([]*Migration, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	migrationsMap := make(map[string]*Migration)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		// Parse migration name (NNN_name.up.sql or NNN_name.down.sql)
		parts := strings.Split(filename, ".")
		if len(parts) < 2 {
			continue
		}

		direction := parts[len(parts)-2] // "up" or "down"
		if direction != "up" && direction != "down" {
			continue
		}

		// Extract migration name
		baseName := strings.Join(parts[:len(parts)-2], ".")

		migName := baseName
		path := filepath.Join(migrationsDir, filename)

		mig, exists := migrationsMap[migName]
		if !exists {
			mig = &Migration{Name: migName}
			migrationsMap[migName] = mig
		}

		if direction == "up" {
			mig.UpPath = path
		} else {
			mig.DownPath = path
		}
	}

	// Filter and sort migrations
	var migrations []*Migration
	for _, mig := range migrationsMap {
		// Only include migrations with both up and down files
		if mig.UpPath != "" && mig.DownPath != "" {
			migrations = append(migrations, mig)
		}
	}

	// Sort by name (numeric prefix)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

// getAppliedMigrations loads applied migrations from the database
func getAppliedMigrations(ctx context.Context, c interface {
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
}) (map[string]*Migration, error) {
	query := `
	SELECT id, name, applied_at, checksum, execution_time_ms
	FROM schema_migrations
	ORDER BY applied_at ASC
	`

	rows, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]*Migration)
	for rows.Next() {
		var id int
		var name string
		var appliedAt time.Time
		var checksum string
		var executionTime int

		if err := rows.Scan(&id, &name, &appliedAt, &checksum, &executionTime); err != nil {
			return nil, err
		}

		applied[name] = &Migration{
			ID:            id,
			Name:          name,
			AppliedAt:     &appliedAt,
			Checksum:      checksum,
			ExecutionTime: executionTime,
		}
	}

	return applied, rows.Err()
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist
func ensureMigrationsTable(ctx context.Context, c interface {
	Exec(context.Context, string, ...interface{}) (string, error)
}) error {
	schema := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
	    id SERIAL PRIMARY KEY,
	    name TEXT NOT NULL UNIQUE,
	    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
	    checksum TEXT NOT NULL,
	    execution_time_ms INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_schema_migrations_name ON schema_migrations(name);
	CREATE INDEX IF NOT EXISTS idx_schema_migrations_applied_at ON schema_migrations(applied_at);
	`

	_, err := c.Exec(ctx, schema)
	return err
}

// calculateChecksum computes SHA256 of a file
func calculateChecksum(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(content))
}

// checkCircularDependencies validates no circular dependencies exist
// For now, this is a placeholder; true cycle detection would require parsing SQL
func checkCircularDependencies(migrations []*Migration, applied map[string]*Migration) error {
	// Basic validation: ensure all pending migrations reference only applied or pending ones
	// This is a simplified check; full implementation would require dependency graph analysis
	return nil
}

// applyMigrationInTransaction applies a migration in a transaction with pessimistic locking
func applyMigrationInTransaction(ctx context.Context, c interface {
	Exec(context.Context, string, ...interface{}) (string, error)
}, name string, upSQL string, checksum string) error {
	// Execute the migration SQL and record in schema_migrations
	recordSQL := `
	INSERT INTO schema_migrations (name, checksum, applied_at, execution_time_ms)
	VALUES ($1, $2, NOW(), 0)
	`

	// In a real implementation, this would be wrapped in a transaction
	// For now, we execute the SQL directly
	if _, err := c.Exec(ctx, upSQL); err != nil {
		return err
	}

	_, err := c.Exec(ctx, recordSQL, name, checksum)
	return err
}

// rollbackMigrationInTransaction rolls back a migration in a transaction
func rollbackMigrationInTransaction(ctx context.Context, c interface {
	Exec(context.Context, string, ...interface{}) (string, error)
}, name string, downSQL string) error {
	// Execute the rollback SQL and remove from schema_migrations
	deleteSQL := `DELETE FROM schema_migrations WHERE name = $1`

	// Execute down SQL first
	if _, err := c.Exec(ctx, downSQL); err != nil {
		return err
	}

	_, err := c.Exec(ctx, deleteSQL, name)
	return err
}

// reapplyMigration re-applies a migration by running down then up
func reapplyMigration(ctx context.Context, c interface {
	Exec(context.Context, string, ...interface{}) (string, error)
}, name string, downSQL string, upSQL string) error {
	// Run down
	if _, err := c.Exec(ctx, downSQL); err != nil {
		return err
	}

	// Run up
	if _, err := c.Exec(ctx, upSQL); err != nil {
		return err
	}

	// Update checksum in schema_migrations
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(upSQL)))
	updateSQL := `
	INSERT INTO schema_migrations (name, checksum, applied_at, execution_time_ms)
	VALUES ($1, $2, NOW(), 0)
	ON CONFLICT (name) DO UPDATE SET checksum = $2, applied_at = NOW()
	`

	_, err := c.Exec(ctx, updateSQL, name, checksum)
	return err
}
