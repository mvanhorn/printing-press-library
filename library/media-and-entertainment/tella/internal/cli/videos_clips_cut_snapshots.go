// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type cutSnapshot struct {
	VideoID      string    `json:"video_id"`
	ClipID       string    `json:"clip_id"`
	CreatedAt    time.Time `json:"created_at"`
	Cuts         any       `json:"cuts"`
	ExpectedCuts any       `json:"expected_cuts,omitempty"`
}

func captureCutSnapshot(api cleanAPI, videoID, clipID string) (cutSnapshot, error) {
	data, err := api.Get(fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, clipID), nil)
	if err != nil {
		return cutSnapshot{}, err
	}
	cuts, err := clipCutsFromResponse(data)
	if err != nil {
		return cutSnapshot{}, err
	}
	return cutSnapshot{VideoID: videoID, ClipID: clipID, CreatedAt: time.Now().UTC(), Cuts: cuts}, nil
}

func clipCutsFromResponse(data json.RawMessage) (any, error) {
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("parsing clip response cuts: %w", err)
	}
	clip := response
	if nested, ok := response["clip"].(map[string]any); ok {
		clip = nested
	}
	cuts, ok := clip["cuts"]
	if !ok {
		return nil, fmt.Errorf("clip response is missing cuts")
	}
	if _, ok := cuts.([]any); !ok {
		return nil, fmt.Errorf("clip response cuts is not an array")
	}
	return cuts, nil
}

func saveCutSnapshot(api cleanAPI, videoID, clipID string) (string, error) {
	snapshot, err := captureCutSnapshot(api, videoID, clipID)
	if err != nil {
		return "", err
	}
	return persistCutSnapshot(snapshot)
}

func persistCutSnapshot(snapshot cutSnapshot) (string, error) {
	dir, err := cutSnapshotDir(snapshot.VideoID, snapshot.ClipID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, snapshot.CreatedAt.Format("20060102T150405.000000000Z")+".json")
	return path, writeCutSnapshot(path, snapshot)
}

func writeCutSnapshot(path string, snapshot cutSnapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".cut-snapshot-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func updateCutSnapshotFiles(paths map[string]string, snapshots []cutSnapshot) error {
	var errs []error
	for _, snapshot := range snapshots {
		path := paths[snapshot.ClipID]
		if path == "" {
			continue
		}
		if err := writeCutSnapshot(path, snapshot); err != nil {
			errs = append(errs, fmt.Errorf("updating recovery snapshot for clip %s: %w", snapshot.ClipID, err))
		}
	}
	return errors.Join(errs...)
}

func restoreCutSnapshot(api cleanAPI, snapshot cutSnapshot) (int, error) {
	_, status, err := api.Patch(
		fmt.Sprintf("/v1/videos/%s/clips/%s", snapshot.VideoID, snapshot.ClipID),
		map[string]any{"cuts": snapshot.Cuts},
	)
	return status, err
}

func readCutSnapshot(path string) (cutSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cutSnapshot{}, err
	}
	var snapshot cutSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return cutSnapshot{}, fmt.Errorf("parsing cuts snapshot %s: %w", path, err)
	}
	if snapshot.VideoID == "" || snapshot.ClipID == "" {
		return cutSnapshot{}, fmt.Errorf("cuts snapshot %s is missing its video or clip ID", path)
	}
	if _, ok := snapshot.Cuts.([]any); !ok {
		return cutSnapshot{}, fmt.Errorf("cuts snapshot %s does not contain a cuts array", path)
	}
	if snapshot.ExpectedCuts != nil {
		if _, ok := snapshot.ExpectedCuts.([]any); !ok {
			return cutSnapshot{}, fmt.Errorf("cuts snapshot %s does not contain an expected_cuts array", path)
		}
	}
	return snapshot, nil
}

func latestCutSnapshotPath(videoID, clipID string) (string, error) {
	dir, err := cutSnapshotDir(videoID, clipID)
	if err != nil {
		return "", err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no cut snapshots found for video %s clip %s; pass --snapshot or use restore-cuts --cuts", videoID, clipID)
	}
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

func cutSnapshotDir(videoID, clipID string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "tella-pp-cli", "cut-snapshots", safePathSegment(videoID), safePathSegment(clipID)), nil
}

func safePathSegment(value string) string {
	out := []rune(value)
	for i, current := range out {
		if !(current == '-' || current == '_' || current == '.' || current >= '0' && current <= '9' || current >= 'a' && current <= 'z' || current >= 'A' && current <= 'Z') {
			out[i] = '_'
		}
	}
	return string(out)
}
