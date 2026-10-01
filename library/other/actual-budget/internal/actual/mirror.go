// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil"
)

// Manifest records what a mirror contains and when it was pulled.
type Manifest struct {
	SyncID        string    `json:"sync_id"`
	FileID        string    `json:"file_id"`
	GroupID       string    `json:"group_id"`
	Name          string    `json:"name"`
	Encrypted     bool      `json:"encrypted"`
	ServerURL     string    `json:"server_url"`
	ServerVersion string    `json:"server_version,omitempty"`
	PulledAt      time.Time `json:"pulled_at"`
	DBBytes       int       `json:"db_bytes"`
	BudgetType    string    `json:"budget_type,omitempty"`
}

var safeSyncID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// MirrorRoot is the directory that holds one subdirectory per pulled budget.
func MirrorRoot() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mirrors"), nil
}

// MirrorDir returns the directory for one budget's mirror.
func MirrorDir(syncID string) (string, error) {
	if !safeSyncID.MatchString(syncID) {
		return "", fmt.Errorf("invalid sync id %q", syncID)
	}
	root, err := MirrorRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, syncID), nil
}

// DBPath returns the db.sqlite path of a budget's mirror.
func DBPath(syncID string) (string, error) {
	dir, err := MirrorDir(syncID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DBFileName), nil
}

// ReadManifest loads the manifest of a pulled mirror.
func ReadManifest(syncID string) (*Manifest, error) {
	dir, err := MirrorDir(syncID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "pull.json")) // #nosec G304 -- dir is under the CLI data root; MirrorDir rejects unsafe sync ids
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("reading mirror manifest: %w", err)
	}
	return &m, nil
}

// LocateMirror finds the mirror for syncID, which may be a group id (the
// sync id Actual shows) or a file id. It returns the db.sqlite path and its
// manifest (nil when the mirror predates pull.json). Among mirrors whose
// directory, group id or file id matches, the most recently pulled wins, so
// a directory left behind by an older pull keyed on the file id cannot
// shadow a newer one. When nothing matches, found is false and path is
// empty; DBPath gives where a pull keyed on syncID would write.
func LocateMirror(syncID string) (path string, manifest *Manifest, found bool, err error) {
	if _, err := MirrorDir(syncID); err != nil {
		return "", nil, false, err
	}
	root, err := MirrorRoot()
	if err != nil {
		return "", nil, false, err
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || !safeSyncID.MatchString(e.Name()) {
			continue
		}
		db := filepath.Join(root, e.Name(), DBFileName)
		if _, statErr := os.Stat(db); statErr != nil {
			continue
		}
		m, _ := ReadManifest(e.Name())
		match := e.Name() == syncID || (m != nil && (m.GroupID == syncID || m.FileID == syncID))
		if !match {
			continue
		}
		if !found || (m != nil && (manifest == nil || m.PulledAt.After(manifest.PulledAt))) {
			path, manifest, found = db, m, true
		}
	}
	return path, manifest, found, nil
}

// PullOptions configures a mirror pull.
type PullOptions struct {
	Password           string
	SessionToken       string
	SyncID             string
	EncryptionPassword string
	// KeepZip additionally writes the raw (decrypted) budget zip as a backup.
	KeepZip bool
}

// PullResult reports a completed pull.
type PullResult struct {
	Manifest *Manifest `json:"manifest"`
	DBPath   string    `json:"db_path"`
	ZipPath  string    `json:"zip_path,omitempty"`
}

// Authenticate logs in with a session token or password.
func Authenticate(ctx context.Context, c *Client, password, sessionToken string) error {
	if sessionToken != "" {
		c.Token = sessionToken
		return nil
	}
	return c.Login(ctx, password)
}

// FindFile resolves a sync id (the groupId shown in Actual's settings) to the
// server file. A file id is accepted too.
func FindFile(files []RemoteFile, syncID string) (*RemoteFile, error) {
	for i := range files {
		f := &files[i]
		if f.Deleted != 0 {
			continue
		}
		if f.GroupID == syncID || f.FileID == syncID {
			return f, nil
		}
	}
	return nil, &ServerError{Status: 404, Reason: "file-not-found", Detail: "no budget with sync id " + syncID, Path: "/sync/list-user-files"}
}

// Pull downloads, decrypts and unpacks a budget into its mirror directory.
func Pull(ctx context.Context, c *Client, opts PullOptions) (*PullResult, error) {
	if opts.SyncID == "" {
		return nil, errors.New("sync id is not set (ACTUAL_SYNC_ID or --budget-sync-id)")
	}
	if err := Authenticate(ctx, c, opts.Password, opts.SessionToken); err != nil {
		return nil, err
	}
	files, err := c.ListFiles(ctx)
	if err != nil {
		return nil, err
	}
	file, err := FindFile(files, opts.SyncID)
	if err != nil {
		return nil, err
	}
	info, err := c.FileInfo(ctx, file.FileID)
	if err != nil {
		return nil, err
	}
	raw, err := c.Download(ctx, file.FileID)
	if err != nil {
		return nil, err
	}
	if info.EncryptMeta != nil {
		if opts.EncryptionPassword == "" {
			return nil, ErrMissingKey
		}
		k, err := c.KeyInfo(ctx, file.FileID)
		if err != nil {
			return nil, err
		}
		key, err := DeriveKey(opts.EncryptionPassword, k.Salt)
		if err != nil {
			return nil, err
		}
		if raw, err = Decrypt(key, raw, info.EncryptMeta); err != nil {
			return nil, err
		}
	}
	arc, err := ExtractArchive(raw)
	if err != nil {
		return nil, err
	}
	// Key the mirror on the group id (the sync id Actual shows) even when
	// the caller passed a file id, so every command finds the same directory.
	mirrorID := file.GroupID
	if mirrorID == "" {
		mirrorID = opts.SyncID
	}
	dir, err := MirrorDir(mirrorID)
	if err != nil {
		return nil, err
	}
	version, _ := c.ServerVersion(ctx)
	m := &Manifest{
		SyncID:        mirrorID,
		FileID:        file.FileID,
		GroupID:       file.GroupID,
		Name:          file.Name,
		Encrypted:     info.EncryptMeta != nil,
		ServerURL:     RedactURL(c.BaseURL),
		ServerVersion: version,
		PulledAt:      time.Now().UTC(),
		DBBytes:       len(arc.DB),
	}
	var meta struct {
		BudgetType string `json:"budgetType"`
	}
	if json.Unmarshal(arc.Metadata, &meta) == nil {
		m.BudgetType = meta.BudgetType
	}
	dbPath := filepath.Join(dir, DBFileName)
	// The budget is personal financial data: keep it owner-only.
	if err := cliutil.AtomicWritePrivateFile(dbPath, arc.DB, 0o600, 0o700); err != nil {
		return nil, fmt.Errorf("writing mirror: %w", err)
	}
	// A stale WAL/SHM from an earlier pull would be replayed onto the new file.
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	if err := cliutil.AtomicWritePrivateFile(filepath.Join(dir, "metadata.json"), arc.Metadata, 0o600, 0o700); err != nil {
		return nil, err
	}
	res := &PullResult{Manifest: m, DBPath: dbPath}
	if opts.KeepZip {
		zipPath := filepath.Join(dir, "backups", m.PulledAt.Format("20060102T150405Z")+".zip")
		if err := cliutil.AtomicWritePrivateFile(zipPath, raw, 0o600, 0o700); err != nil {
			return nil, err
		}
		res.ZipPath = zipPath
	}
	mj, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := cliutil.AtomicWritePrivateFile(filepath.Join(dir, "pull.json"), mj, 0o600, 0o700); err != nil {
		return nil, err
	}
	return res, nil
}

// RedactURL drops userinfo, query and fragment from a URL for display and
// for the mirror manifest.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// OpenReadOnly opens a mirror database for queries. The file is opened
// read-only with query_only so no command can modify the downloaded budget.
func OpenReadOnly(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	// Build the URI with net/url so '?', '#' or '%' in the data-dir path are
	// escaped instead of displacing mode=ro and query_only.
	dsn := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("opening budget mirror %s: %w", path, err)
	}
	return db, nil
}
