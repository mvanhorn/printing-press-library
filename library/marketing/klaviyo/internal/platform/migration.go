// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	_ "modernc.org/sqlite"
)

// LegacyMigrationState reports only artifact-level migration facts. Legacy
// config contents and database rows are never copied into receipts.
type LegacyMigrationState struct {
	Status    string                   `json:"status"`
	Preserved bool                     `json:"preserved"`
	Reason    string                   `json:"reason"`
	Config    *LegacyMigrationArtifact `json:"config,omitempty"`
	Database  *LegacyMigrationArtifact `json:"database,omitempty"`
}

type LegacyMigrationArtifact struct {
	Path               string `json:"path"`
	Status             string `json:"status"`
	Copied             bool   `json:"copied"`
	Destination        string `json:"destination,omitempty"`
	BackupPath         string `json:"backup_path,omitempty"`
	Evidence           string `json:"evidence,omitempty"`
	VerifiedForCleanup bool   `json:"verified_for_cleanup"`
}

const legacyBackupSuffix = ".printing-press-backup"

var ErrLegacyAdoptionConfirmationRequired = errors.New("legacy database evidence matched; rerun with --adopt-verified-legacy-db to copy it")

type LegacyDatabaseEvidence struct {
	Verified bool
	Method   string
	Reason   string
}

type LegacyDatabaseEvidenceInspector func(context.Context, *sql.DB, *Session) (LegacyDatabaseEvidence, error)

type LegacyDatabaseAdoptionOptions struct {
	ConfirmEvidence bool
	InspectEvidence LegacyDatabaseEvidenceInspector
}

// ReadLegacyConfig decodes only the allowlisted fields declared by target.
// Parse details are intentionally suppressed because a legacy file may contain
// raw credentials on unrelated lines.
func ReadLegacyConfig(path string, target any) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("legacy config path is empty")
	}
	actual := path
	info, err := os.Lstat(actual)
	if errors.Is(err, os.ErrNotExist) {
		actual += legacyBackupSuffix
		info, err = os.Lstat(actual)
	}
	if err != nil {
		return fmt.Errorf("inspect legacy config: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("legacy config must be a regular non-symlink file")
	}
	data, err := os.ReadFile(actual) // #nosec G304 -- explicit migration path, validated above.
	if err != nil {
		return fmt.Errorf("read legacy config: %w", err)
	}
	defer func() {
		for i := range data {
			data[i] = 0
		}
	}()
	if err := toml.Unmarshal(data, target); err != nil {
		return errors.New("legacy config could not be parsed safely")
	}
	return nil
}

func SaveSourceConfig(path string, value any) error {
	data, err := toml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode profile-scoped source config: %w", err)
	}
	return writePrivateFile(path, data)
}

func LoadSourceConfig(path string, value any) error {
	data, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- derived profile-owned path.
	if err != nil {
		return err
	}
	if err := toml.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode profile-scoped source config: %w", err)
	}
	return nil
}

// InspectLegacyMigrationPaths performs only filesystem metadata checks. It is
// safe before the live tenant gate because it never opens a config or database.
func InspectLegacyMigrationPaths(configPath, databasePath string) (LegacyMigrationState, error) {
	state := LegacyMigrationState{
		Status:    "none",
		Preserved: true,
		Reason:    "legacy state is adopted only after a live tenant gate and verified database ownership",
	}
	for kind, raw := range map[string]string{"config": configPath, "database": databasePath} {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				backupPath := path + legacyBackupSuffix
				backupInfo, backupErr := os.Lstat(backupPath)
				if backupErr == nil && backupInfo.Mode().IsRegular() && backupInfo.Mode()&os.ModeSymlink == 0 {
					artifact := &LegacyMigrationArtifact{Path: path, Status: "backed_up", Copied: false, BackupPath: backupPath}
					if kind == "config" {
						state.Config = artifact
					} else {
						state.Database = artifact
					}
					state.Status = "preserved"
					continue
				}
			}
			return LegacyMigrationState{}, fmt.Errorf("inspect legacy %s: %w", kind, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return LegacyMigrationState{}, fmt.Errorf("legacy %s may not be a symlink", kind)
		}
		if !info.Mode().IsRegular() {
			return LegacyMigrationState{}, fmt.Errorf("legacy %s must be a regular file", kind)
		}
		artifact := &LegacyMigrationArtifact{Path: path, Status: "preserved", Copied: false}
		if kind == "config" {
			state.Config = artifact
		} else {
			artifact.Status = "pending_verification"
			state.Database = artifact
		}
		state.Status = "preserved"
	}
	return state, nil
}

// ValidateLegacyMigrationPaths prevents cleanup or adoption from targeting the
// active profile and its isolated artifacts.
func ValidateLegacyMigrationPaths(session *Session, state *LegacyMigrationState) error {
	if session == nil || state == nil {
		return errors.New("prepared tenant session and legacy state are required")
	}
	protectedFiles := []string{session.Paths.ProfileFile, session.Paths.ConfigFile, session.Paths.DataFile}
	protectedDirs := []string{session.Paths.CacheDir, session.Paths.StateDir, session.Paths.ReceiptsDir}
	seen := map[string]string{}
	for kind, artifact := range map[string]*LegacyMigrationArtifact{"config": state.Config, "database": state.Database} {
		if artifact == nil || strings.TrimSpace(artifact.Path) == "" {
			continue
		}
		candidate, err := filepath.Abs(filepath.Clean(artifact.Path))
		if err != nil {
			return fmt.Errorf("resolve legacy %s path: %w", kind, err)
		}
		if prior, ok := seen[candidate]; ok {
			return fmt.Errorf("legacy %s path duplicates legacy %s path", kind, prior)
		}
		seen[candidate] = kind
		for _, protected := range protectedFiles {
			resolved, err := filepath.Abs(filepath.Clean(protected))
			if err == nil && candidate == resolved {
				return fmt.Errorf("legacy %s path conflicts with a profile-owned artifact", kind)
			}
		}
		for _, protected := range protectedDirs {
			resolved, err := filepath.Abs(filepath.Clean(protected))
			if err == nil && pathWithin(candidate, resolved) {
				return fmt.Errorf("legacy %s path conflicts with a profile-owned directory", kind)
			}
		}
	}
	return nil
}

func pathWithin(path, directory string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

// AdoptVerifiedLegacyDatabase copies a legacy SQLite database only when its
// existing tenant metadata matches the freshly verified session. Missing,
// malformed, mixed, and wrong-tenant databases are quarantined in place.
func AdoptVerifiedLegacyDatabase(ctx context.Context, session *Session, state *LegacyMigrationState, options ...LegacyDatabaseAdoptionOptions) error {
	if state == nil || state.Database == nil {
		return nil
	}
	if session == nil || session.GateOutcome != GateVerified {
		return errors.New("verified tenant session is required before legacy database inspection")
	}
	sourcePath := state.Database.Path
	targetPath := session.Paths.DataFile
	if targetPath == "" {
		return errors.New("profile-scoped migration destination is empty")
	}
	if filepath.Clean(sourcePath) == filepath.Clean(targetPath) {
		return errors.New("legacy database must differ from the profile-scoped destination")
	}
	if state.Database.Status == "backed_up" {
		target, err := sql.Open("sqlite", readOnlySQLiteDSN(targetPath))
		if err != nil {
			return fmt.Errorf("open existing migration destination: %w", err)
		}
		defer target.Close()
		if err := VerifyTenantMetadataReadOnly(ctx, target, session); err != nil {
			return fmt.Errorf("backed-up legacy database has no verified destination: %w", err)
		}
		state.Database.Status = "already_adopted"
		state.Database.Destination = targetPath
		state.Status = "adopted"
		return nil
	}

	var option LegacyDatabaseAdoptionOptions
	if len(options) > 0 {
		option = options[0]
	}
	source, err := sql.Open("sqlite", readOnlySQLiteDSN(sourcePath))
	if err != nil {
		markLegacyDatabaseQuarantined(state, "legacy database could not be opened read-only")
		return nil
	}
	defer source.Close()
	if err := source.PingContext(ctx); err != nil {
		markLegacyDatabaseQuarantined(state, "legacy database could not be inspected read-only")
		return nil
	}
	metadataMatched := false
	evidenceMethod := ""
	if metadataErr := VerifyTenantMetadataReadOnly(ctx, source, session); metadataErr == nil {
		metadataMatched = true
		evidenceMethod = "verified-tenant-metadata"
	} else if errors.Is(metadataErr, ErrTenantMetadataMissing) && option.InspectEvidence != nil {
		evidence, evidenceErr := option.InspectEvidence(ctx, source, session)
		if evidenceErr != nil {
			markLegacyDatabaseQuarantined(state, "legacy database evidence scan was indeterminate")
			return nil
		}
		state.Database.Evidence = strings.TrimSpace(evidence.Method)
		if !evidence.Verified {
			markLegacyDatabaseQuarantined(state, evidence.Reason)
			return nil
		}
		if !option.ConfirmEvidence {
			state.Database.Status = "confirmation_required"
			state.Status = "confirmation_required"
			state.Reason = ErrLegacyAdoptionConfirmationRequired.Error()
			return ErrLegacyAdoptionConfirmationRequired
		}
		evidenceMethod = strings.TrimSpace(evidence.Method)
		if evidenceMethod == "" {
			evidenceMethod = "provider-evidence"
		}
	} else {
		markLegacyDatabaseQuarantined(state, "legacy database tenant metadata was missing, malformed, or mismatched")
		return nil
	}
	state.Database.VerifiedForCleanup = true

	verifyExistingDestination := func() error {
		target, openErr := sql.Open("sqlite", readOnlySQLiteDSN(targetPath))
		if openErr != nil {
			return fmt.Errorf("open existing migration destination: %w", openErr)
		}
		defer target.Close()
		if verifyErr := VerifyTenantMetadataReadOnly(ctx, target, session); verifyErr != nil {
			return fmt.Errorf("existing migration destination is not owned by the verified tenant: %w", verifyErr)
		}
		state.Database.Status = "already_adopted"
		state.Database.Copied = false
		state.Database.Destination = targetPath
		state.Status = "adopted"
		return nil
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return verifyExistingDestination()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect migration destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return fmt.Errorf("create migration destination directory: %w", err)
	}
	// PATCH(greptile-g3): finish a private copy before publishing without replacement.
	temporary, err := os.CreateTemp(filepath.Dir(targetPath), ".legacy-migration-*.db")
	if err != nil {
		return fmt.Errorf("create migration temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close migration temporary file: %w", err)
	}
	quotedTarget := strings.ReplaceAll(temporaryPath, "'", "''")
	if _, err := source.ExecContext(ctx, "VACUUM INTO '"+quotedTarget+"'"); err != nil {
		return fmt.Errorf("copy verified legacy database: %w", err)
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return fmt.Errorf("secure migrated database: %w", err)
	}
	target, err := sql.Open("sqlite", temporaryPath)
	if err != nil {
		return fmt.Errorf("open migrated database: %w", err)
	}
	provenance := "client-migrate:" + evidenceMethod
	if metadataMatched {
		err = VerifyTenantMetadata(ctx, target, session)
		if err == nil {
			_, err = target.ExecContext(ctx, `UPDATE pp_tenant_metadata SET migration_provenance = ? WHERE id = 1`, provenance)
		}
	} else {
		err = InitializeTenantMetadata(ctx, target, session, provenance)
	}
	if err != nil {
		_ = target.Close()
		return fmt.Errorf("verify migrated database: %w", err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close migrated database: %w", err)
	}
	if err := os.Link(temporaryPath, targetPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return verifyExistingDestination()
		}
		return fmt.Errorf("publish migrated database: %w", err)
	}
	state.Database.Status = "adopted"
	state.Database.Copied = true
	state.Database.Destination = targetPath
	state.Status = "adopted"
	return nil
}

func markLegacyDatabaseQuarantined(state *LegacyMigrationState, reason string) {
	state.Database.Status = "quarantined"
	state.Database.Copied = false
	state.Database.VerifiedForCleanup = false
	state.Status = "quarantined"
	if strings.TrimSpace(reason) == "" {
		reason = "legacy database is missing matching verified tenant evidence"
	}
	state.Reason = reason + "; use a clean profile-scoped sync"
}

// BackupLegacyArtifacts is the explicit cleanup step. It never deletes legacy
// state: verified artifacts are renamed beside the original path to a stable,
// recoverable backup. Unverified databases must remain quarantined in place.
func BackupLegacyArtifacts(state *LegacyMigrationState) error {
	if state == nil {
		return nil
	}
	if state.Database != nil && state.Database.Status == "quarantined" {
		return errors.New("refusing cleanup of a quarantined legacy database")
	}
	artifacts := []*LegacyMigrationArtifact{state.Config, state.Database}
	for _, artifact := range artifacts {
		if artifact == nil || artifact.Status == "backed_up" || artifact.BackupPath != "" || artifact.Path == "" {
			continue
		}
		if !artifact.VerifiedForCleanup {
			return fmt.Errorf("refusing cleanup of unverified legacy artifact: %s", artifact.Path)
		}
		backupPath := artifact.Path + legacyBackupSuffix
		if _, err := os.Lstat(backupPath); err == nil {
			return fmt.Errorf("legacy backup already exists: %s", backupPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect legacy backup: %w", err)
		}
	}
	moved := make([]*LegacyMigrationArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact == nil || artifact.Status == "backed_up" || artifact.BackupPath != "" || artifact.Path == "" {
			continue
		}
		backupPath := artifact.Path + legacyBackupSuffix
		if err := os.Rename(artifact.Path, backupPath); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(moved[i].BackupPath, moved[i].Path)
				moved[i].BackupPath = ""
			}
			return fmt.Errorf("back up legacy artifact: %w", err)
		}
		if err := os.Chmod(backupPath, 0o600); err != nil {
			_ = os.Rename(backupPath, artifact.Path)
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(moved[i].BackupPath, moved[i].Path)
				moved[i].BackupPath = ""
				moved[i].Status = "preserved"
			}
			return fmt.Errorf("secure legacy backup: %w", err)
		}
		artifact.BackupPath = backupPath
		artifact.Status = "backed_up"
		moved = append(moved, artifact)
	}
	if len(moved) > 0 {
		state.Status = "backed_up"
		state.Reason = "legacy artifacts were preserved in explicit recoverable backups"
	}
	return nil
}

func readOnlySQLiteDSN(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "mode=ro"}).String()
}
