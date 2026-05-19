package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const doctorPassTTL = 30 * time.Minute

type doctorPassMarker struct {
	RecordedAt string `json:"recorded_at"`
	Detail     string `json:"detail"`
}

func appDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "facebook-marketplace-pp-cli"), nil
	case "windows":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(configDir, "facebook-marketplace-pp-cli"), nil
	default:
		dataDir := os.Getenv("XDG_DATA_HOME")
		if dataDir == "" {
			dataDir = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(dataDir, "facebook-marketplace-pp-cli"), nil
	}
}

func ensureAppDataDir() (string, error) {
	dir, err := appDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func doctorPassPath() (string, error) {
	dir, err := ensureAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "doctor-pass.json"), nil
}

func recordDoctorPass(report map[string]any) error {
	if !doctorReportAllowsWrites(report) {
		return nil
	}
	path, err := doctorPassPath()
	if err != nil {
		return err
	}
	marker := doctorPassMarker{
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
		Detail:     fmt.Sprintf("%v; %v", report["api"], report["credentials_detail"]),
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func doctorReportAllowsWrites(report map[string]any) bool {
	api := strings.ToLower(fmt.Sprintf("%v", report["api"]))
	credentials := strings.ToLower(fmt.Sprintf("%v", report["credentials"]))
	proof := strings.ToLower(fmt.Sprintf("%v", report["browser_session_proof"]))
	apiReachable := api == "reachable" || strings.HasPrefix(api, "reachable (http")
	return apiReachable && credentials == "valid" && proof == "valid"
}

func requireWriteCheckpoint(flags *rootFlags) (string, error) {
	if flags == nil || !flags.write {
		return "", fmt.Errorf("refusing to write: pass --write after reviewing the action")
	}
	path, err := doctorPassPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("refusing to write: run facebook-marketplace-pp-cli doctor successfully first")
	}
	var marker doctorPassMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return "", fmt.Errorf("refusing to write: doctor pass marker is corrupt; rerun doctor")
	}
	recordedAt, err := time.Parse(time.RFC3339, marker.RecordedAt)
	if err != nil {
		return "", fmt.Errorf("refusing to write: doctor pass marker has invalid timestamp; rerun doctor")
	}
	if time.Since(recordedAt) > doctorPassTTL {
		return "", fmt.Errorf("refusing to write: doctor pass is older than %s; rerun doctor", doctorPassTTL)
	}
	key, err := newIdempotencyKey()
	if err != nil {
		return "", err
	}
	if err := recordWriteState(key, "queued", "write checkpoint passed"); err != nil {
		return "", err
	}
	return key, nil
}

func newIdempotencyKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "fbm_" + hex.EncodeToString(b[:]), nil
}
