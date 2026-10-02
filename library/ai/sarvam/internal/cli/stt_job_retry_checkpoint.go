// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Library patch: make replacement batch-STT jobs resumable across partial failures.

package cli

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/cliutil"
)

const sttRetryCheckpointVersion = 1

type preparedSTTRetryFile struct {
	Name string
	Path string
	Size int64
	File *os.File
}

type sttRetryCheckpoint struct {
	Version          int      `json:"version"`
	OriginalJobID    string   `json:"original_job_id"`
	ReplacementJobID string   `json:"replacement_job_id,omitempty"`
	Files            []string `json:"files"`
	UploadedFiles    []string `json:"uploaded_files,omitempty"`
	StartAttempted   bool     `json:"start_attempted,omitempty"`
	Started          bool     `json:"started,omitempty"`
}

// sttRetryLease serializes all work on one original job. The lock file stays
// exclusively owned until uploads and /start finish; the checkpoint itself is
// durable state, not a mutual-exclusion primitive.
type sttRetryLease struct {
	file *os.File
}

func acquireSTTRetryLease(checkpointPath string) (*sttRetryLease, error) {
	lockPath := checkpointPath + ".lock"
	for attempts := 0; attempts < 3; attempts++ {
		file, err := os.OpenFile(filepath.Clean(lockPath), os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- path is state-dir derived.
		if err != nil {
			return nil, fmt.Errorf("acquiring retry lock: %w", err)
		}
		openedInfo, statErr := file.Stat()
		pathInfo, pathErr := os.Lstat(filepath.Clean(lockPath)) // #nosec G304 -- path is state-dir derived.
		if statErr != nil || pathErr != nil || !openedInfo.Mode().IsRegular() || !pathInfo.Mode().IsRegular() || !os.SameFile(openedInfo, pathInfo) {
			_ = file.Close()
			if statErr != nil {
				return nil, fmt.Errorf("stating opened retry lock: %w", statErr)
			}
			if pathErr != nil && !errors.Is(pathErr, os.ErrNotExist) {
				return nil, fmt.Errorf("stating retry lock path: %w", pathErr)
			}
			if pathErr == nil && (!openedInfo.Mode().IsRegular() || !pathInfo.Mode().IsRegular()) {
				return nil, fmt.Errorf("retry lock %s is not a regular file", lockPath)
			}
			continue
		}
		locked, err := tryLockSTTRetryFile(file)
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("locking retry state: %w", err)
		}
		if !locked {
			_ = file.Close()
			return nil, fmt.Errorf("another retry for this job is active (lock %s); wait for it to finish", lockPath)
		}
		lease := &sttRetryLease{file: file}
		if err := file.Chmod(0o600); err != nil {
			_ = lease.Release()
			return nil, fmt.Errorf("securing retry lock: %w", err)
		}
		if err := file.Truncate(0); err != nil {
			_ = lease.Release()
			return nil, fmt.Errorf("resetting retry lock metadata: %w", err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			_ = lease.Release()
			return nil, fmt.Errorf("rewinding retry lock metadata: %w", err)
		}
		if err := json.NewEncoder(file).Encode(map[string]any{"pid": os.Getpid(), "acquired_at": time.Now().UTC()}); err != nil {
			_ = lease.Release()
			return nil, fmt.Errorf("writing retry lock metadata: %w", err)
		}
		return lease, nil
	}
	return nil, fmt.Errorf("retry lock path changed concurrently; rerun the command")
}

func (lease *sttRetryLease) Release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	unlockErr := unlockSTTRetryFile(lease.file)
	closeErr := lease.file.Close()
	lease.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

func safeRetryFilePath(dir, name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("unsafe input filename %q in job status", name)
	}
	return filepath.Join(dir, name), nil
}

func prepareSTTRetryFiles(dir string, names []string) ([]preparedSTTRetryFile, error) {
	prepared := make([]preparedSTTRetryFile, 0, len(names))
	for _, name := range names {
		path, err := safeRetryFilePath(dir, name)
		if err != nil {
			closePreparedSTTRetryFiles(prepared)
			return nil, err
		}
		pathInfo, err := os.Lstat(path)
		if err != nil {
			closePreparedSTTRetryFiles(prepared)
			return nil, fmt.Errorf("validating retry input %q: %w", name, err)
		}
		if !pathInfo.Mode().IsRegular() {
			closePreparedSTTRetryFiles(prepared)
			return nil, fmt.Errorf("retry input %q is not a regular file", name)
		}
		// #nosec G304 -- safeRetryFilePath confines this API-returned basename to --dir.
		file, err := os.Open(path)
		if err != nil {
			closePreparedSTTRetryFiles(prepared)
			return nil, fmt.Errorf("opening retry input %q: %w", name, err)
		}
		fileInfo, err := file.Stat()
		if err != nil || !fileInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
			_ = file.Close()
			closePreparedSTTRetryFiles(prepared)
			if err != nil {
				return nil, fmt.Errorf("stating retry input %q: %w", name, err)
			}
			return nil, fmt.Errorf("retry input %q changed while it was being opened", name)
		}
		prepared = append(prepared, preparedSTTRetryFile{Name: name, Path: path, Size: fileInfo.Size(), File: file})
	}
	return prepared, nil
}

func closePreparedSTTRetryFiles(files []preparedSTTRetryFile) {
	for _, file := range files {
		if file.File != nil {
			_ = file.File.Close()
		}
	}
}

func pendingSTTRetryFileNames(files, uploaded []string) []string {
	done := make(map[string]struct{}, len(uploaded))
	for _, name := range uploaded {
		done[name] = struct{}{}
	}
	pending := make([]string, 0, len(files))
	for _, name := range files {
		if _, ok := done[name]; !ok {
			pending = append(pending, name)
		}
	}
	return pending
}

func validatePreparedSTTRetryFileSize(prepared preparedSTTRetryFile) error {
	info, err := prepared.File.Stat()
	if err != nil {
		return fmt.Errorf("stating retry input %q: %w", prepared.Name, err)
	}
	if !info.Mode().IsRegular() || info.Size() != prepared.Size {
		return fmt.Errorf("retry input %q changed size before upload completed; refusing to mark it uploaded", prepared.Name)
	}
	return nil
}

func sttRetryCheckpointPath(originalJobID string, flags *rootFlags) (string, error) {
	var dir string
	if flags != nil && flags.platformSession != nil {
		dir = flags.platformSession.Paths.StateDir
		if dir == "" {
			return "", errors.New("verified client profile has no state directory")
		}
	} else {
		var err error
		dir, err = cliutil.StateDir()
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating retry state directory: %w", err)
	}
	digest := sha256.Sum256([]byte(originalJobID))
	return filepath.Join(dir, fmt.Sprintf("stt-retry-%x.json", digest)), nil
}

func acquireSTTRetryCheckpoint(path, originalJobID string, files []string) (sttRetryCheckpoint, bool, error) {
	for attempts := 0; attempts < 2; attempts++ {
		checkpoint, exists, err := loadSTTRetryCheckpoint(path, originalJobID, files)
		if err != nil {
			return sttRetryCheckpoint{}, false, err
		}
		if exists {
			if checkpoint.Started {
				return checkpoint, true, nil
			}
			if checkpoint.StartAttempted {
				return sttRetryCheckpoint{}, false, fmt.Errorf("retry start for replacement job %s has an unknown outcome; inspect its provider status before removing checkpoint %s (refusing to send /start again)", checkpoint.ReplacementJobID, path)
			}
			if checkpoint.ReplacementJobID == "" {
				return sttRetryCheckpoint{}, false, fmt.Errorf("a prior retry initiation has an unknown outcome; inspect provider jobs before removing checkpoint %s (refusing to create a duplicate replacement)", path)
			}
			return checkpoint, true, nil
		}

		checkpoint = sttRetryCheckpoint{
			Version:       sttRetryCheckpointVersion,
			OriginalJobID: originalJobID,
			Files:         append([]string(nil), files...),
		}
		claimed, err := claimSTTRetryCheckpoint(path, checkpoint)
		if err != nil {
			return sttRetryCheckpoint{}, false, err
		}
		if claimed {
			return checkpoint, false, nil
		}
	}
	return sttRetryCheckpoint{}, false, fmt.Errorf("retry checkpoint changed concurrently; rerun the command")
}

func loadSTTRetryCheckpoint(path, originalJobID string, files []string) (sttRetryCheckpoint, bool, error) {
	data, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- path is derived from the CLI state directory.
	if errors.Is(err, os.ErrNotExist) {
		return sttRetryCheckpoint{}, false, nil
	}
	if err != nil {
		return sttRetryCheckpoint{}, false, fmt.Errorf("reading retry checkpoint: %w", err)
	}
	var checkpoint sttRetryCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return sttRetryCheckpoint{}, false, fmt.Errorf("parsing retry checkpoint %s: %w (refusing to create a duplicate replacement)", path, err)
	}
	if checkpoint.Version != sttRetryCheckpointVersion || checkpoint.OriginalJobID != originalJobID {
		return sttRetryCheckpoint{}, false, fmt.Errorf("retry checkpoint %s does not match job %q (refusing to create a duplicate replacement)", path, originalJobID)
	}
	if !sameSTTRetryFileSet(checkpoint.Files, files) {
		return sttRetryCheckpoint{}, false, fmt.Errorf("retry job %q already has a pending replacement for a different file set; finish it before changing the selection", originalJobID)
	}
	allowed := make(map[string]struct{}, len(files))
	for _, name := range files {
		allowed[name] = struct{}{}
	}
	for _, name := range checkpoint.UploadedFiles {
		if _, ok := allowed[name]; !ok {
			return sttRetryCheckpoint{}, false, fmt.Errorf("retry checkpoint %s contains an unknown uploaded file %q", path, name)
		}
	}
	return checkpoint, true, nil
}

func claimSTTRetryCheckpoint(path string, checkpoint sttRetryCheckpoint) (bool, error) {
	file, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- path is state-dir derived.
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claiming retry checkpoint: %w", err)
	}
	encodeErr := json.NewEncoder(file).Encode(checkpoint)
	closeErr := file.Close()
	if encodeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if encodeErr != nil {
			return false, fmt.Errorf("writing retry checkpoint: %w", encodeErr)
		}
		return false, fmt.Errorf("closing retry checkpoint: %w", closeErr)
	}
	return true, nil
}

func saveSTTRetryCheckpoint(path string, checkpoint sttRetryCheckpoint) error {
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return cliutil.AtomicWritePrivateFile(path, data, 0o600, 0o700)
}

func pendingSTTRetryError(replacementJobID, checkpointPath string, err error) error {
	if replacementJobID == "" {
		return fmt.Errorf("%w; retry initiation has an unknown outcome recorded at %s, so reruns will refuse to create a duplicate until it is reconciled", err, checkpointPath)
	}
	return fmt.Errorf("%w; replacement job %s remains resumable from %s — rerun this command", err, replacementJobID, checkpointPath)
}

func unsavedSTTRetryIDError(replacementJobID, checkpointPath string, err error) error {
	return configErr(fmt.Errorf("replacement job %s was created but its ID could not be saved: %w; inspect provider jobs and reconcile checkpoint %s before rerunning", replacementJobID, err, checkpointPath))
}

func sameSTTRetryFileSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := append([]string(nil), a...)
	bCopy := append([]string(nil), b...)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}
