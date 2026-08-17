// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/store"
)

const analysisEvidenceMaxAge = 24 * time.Hour

func freshCompleteSyncState(s *store.Store, resourceType string) (time.Time, bool, error) {
	cursor, lastSynced, _, err := s.GetSyncState(resourceType)
	if err != nil {
		return time.Time{}, false, err
	}
	if cursor != "complete" || lastSynced.IsZero() {
		return lastSynced, false, nil
	}
	now := time.Now()
	if lastSynced.After(now.Add(5*time.Minute)) || now.Sub(lastSynced) > analysisEvidenceMaxAge {
		return lastSynced, false, nil
	}
	return lastSynced, true, nil
}

func freshEvidenceTimestamp(value any) bool {
	timestamp := parseFlexibleTime(value)
	if timestamp.IsZero() {
		return false
	}
	now := time.Now()
	return !timestamp.After(now.Add(5*time.Minute)) && now.Sub(timestamp) <= analysisEvidenceMaxAge
}

func listLocalObjects(s *store.Store, resourceType string) ([]map[string]any, error) {
	rows, err := s.List(resourceType, 10000)
	if err != nil {
		return nil, err
	}
	objects := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		var object map[string]any
		if err := json.Unmarshal(row, &object); err != nil {
			return nil, fmt.Errorf("decoding local %s row: %w", resourceType, err)
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func objectContains(value any, needle string) bool {
	switch typed := value.(type) {
	case string:
		return typed == needle
	case []any:
		for _, item := range typed {
			if objectContains(item, needle) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if objectContains(item, needle) {
				return true
			}
		}
	}
	return false
}

func safeEntitySummary(resourceType string, object map[string]any) map[string]any {
	return map[string]any{
		"resource_type": resourceType,
		"id":            firstString(object, "id", "uuid", "appUuid", "app_uuid"),
		"name":          firstString(object, "name", "title"),
		"status":        firstString(object, "status"),
		"space_id":      firstString(object, "spaceId", "space_id"),
	}
}

func firstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key]; ok && value != nil {
			switch typed := value.(type) {
			case string:
				return typed
			case json.Number:
				return typed.String()
			case float64:
				return strconv.FormatFloat(typed, 'f', -1, 64)
			}
		}
	}
	return ""
}

func structuralFingerprint(value any) (string, []string) {
	paths := []string{}
	collectStructure("$", value, &paths)
	sort.Strings(paths)
	paths = uniqueStrings(paths)
	sum := sha256.Sum256([]byte(strings.Join(paths, "\n")))
	return hex.EncodeToString(sum[:8]), paths
}

func collectStructure(path string, value any, paths *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		*paths = append(*paths, path+":object")
		keys := sortedKeys(typed)
		for _, key := range keys {
			collectStructure(path+"."+key, typed[key], paths)
		}
	case []any:
		*paths = append(*paths, path+":array")
		for _, item := range typed {
			collectStructure(path+"[]", item, paths)
		}
	case nil:
		*paths = append(*paths, path+":null")
	case bool:
		*paths = append(*paths, path+":boolean")
	case float64, json.Number:
		*paths = append(*paths, path+":number")
	case string:
		*paths = append(*paths, path+":string")
	default:
		*paths = append(*paths, path+":"+fmt.Sprintf("%T", typed))
	}
}

func ensurePrivateDirectory(path string) error {
	clean := filepath.Clean(path)
	if err := os.MkdirAll(clean, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("private output directory must be a real directory, not a symlink")
	}
	return os.Chmod(clean, 0o700) // #nosec G302 -- private directories require owner-only execute permission.
}

func writePrivateFile(path string, data []byte) error {
	clean := filepath.Clean(path)
	if info, err := os.Lstat(clean); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("private output target must be a regular file, not a symlink")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(clean)
	temporary, err := os.CreateTemp(parent, ".screencloud-private-*")
	if err != nil {
		return err
	}
	tempPath := temporary.Name()
	defer os.Remove(tempPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, clean); err != nil {
		return err
	}
	return os.Chmod(clean, 0o600)
}

func hashLocalWorkingCopy(dir string) (map[string]string, error) {
	hashes := map[string]string{}
	for _, name := range []string{"index.html", "index.css", "script.js", "data.json"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Clean(dir), name)) // #nosec G304 -- selected working directory and fixed filenames.
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		sum := sha256.Sum256(raw)
		hashes[name] = hex.EncodeToString(sum[:8])
	}
	if len(hashes) == 0 {
		return nil, usageErr(fmt.Errorf("--dir contains none of index.html, index.css, script.js, or data.json"))
	}
	return hashes, nil
}

func parseFlexibleTime(value any) time.Time {
	switch typed := value.(type) {
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, typed); err == nil {
				return parsed
			}
		}
		if n, err := strconv.ParseInt(typed, 10, 64); err == nil {
			return unixFlexible(n)
		}
	case float64:
		return unixFlexible(int64(typed))
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return unixFlexible(n)
		}
	}
	return time.Time{}
}

func unixFlexible(value int64) time.Time {
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value)
	}
	if value > 1_000_000_000 {
		return time.Unix(value, 0)
	}
	return time.Time{}
}

func valueAt(object map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := object[key]; ok {
			return value
		}
	}
	return nil
}
