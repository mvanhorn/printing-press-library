package client

// Local config access is intentionally opt-in for self-hosted Home Assistant.
// It mirrors ha-mcp-tools' mounted-config model and never treats arbitrary host
// paths as configuration files.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalConfigUnavailableError struct{ Detail string }

func (e *LocalConfigUnavailableError) Error() string {
	return "local Home Assistant config unavailable: " + e.Detail
}

func LocalConfigPath(path string, write bool) (string, error) {
	root := strings.TrimSpace(os.Getenv("HASS_CONFIG_DIR"))
	if root == "" {
		return "", &LocalConfigUnavailableError{Detail: "set HASS_CONFIG_DIR to the mounted Home Assistant configuration directory"}
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", &LocalConfigUnavailableError{Detail: err.Error()}
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", &LocalConfigUnavailableError{Detail: "configured path is not a readable directory"}
	}
	clean := filepath.Clean(strings.TrimPrefix(path, "/"))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(path) || !allowedConfigPath(clean, write) {
		return "", fmt.Errorf("path is outside the Home Assistant config allowlist")
	}
	if strings.EqualFold(filepath.Base(clean), "secrets.yaml") {
		return "", fmt.Errorf("secrets.yaml is never exposed")
	}
	candidate := filepath.Join(root, clean)
	check := candidate
	if write {
		check = filepath.Dir(candidate)
	}
	resolved, err := filepath.EvalSymlinks(check)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("symlink escapes HASS_CONFIG_DIR")
	}
	return candidate, nil
}

func allowedConfigPath(path string, write bool) bool {
	if path == "configuration.yaml" || strings.HasPrefix(path, "packages"+string(filepath.Separator)) && strings.HasSuffix(path, ".yaml") || strings.HasPrefix(path, "themes"+string(filepath.Separator)) && strings.HasSuffix(path, ".yaml") {
		return true
	}
	for _, dir := range []string{"www", "themes", "custom_templates", "dashboards"} {
		if path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)) {
			return true
		}
	}
	return !write && strings.HasPrefix(path, "blueprints"+string(filepath.Separator))
}

func LocalConfigRead(path string) ([]byte, error) {
	resolved, err := LocalConfigPath(path, false)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(resolved)
}

func LocalConfigWrite(path string, contents []byte) error {
	resolved, err := LocalConfigPath(path, true)
	if err != nil {
		return err
	}
	dir := filepath.Dir(resolved)
	tmp, err := os.CreateTemp(dir, ".pp-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(contents); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, resolved)
}

func LocalConfigDelete(path string) error {
	resolved, err := LocalConfigPath(path, true)
	if err != nil {
		return err
	}
	return os.Remove(resolved)
}

func LocalConfigList(path string) ([]os.DirEntry, error) {
	resolved, err := LocalConfigPath(path, false)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(resolved)
}
