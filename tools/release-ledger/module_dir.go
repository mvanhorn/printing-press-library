package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveModuleDir returns the directory that holds this CLI's go.mod.
//
// A catalog root that already has go.mod is that directory. Some CLIs cannot
// put the module there: a module path ending in /vN is invalid, so the module
// lives one directory down (library/ai/v0/vzero). When the catalog root has no
// go.mod, prefer the shared .goreleaser.yaml builds[].dir, and otherwise the
// single immediate child directory that contains a go.mod. Anything else stays
// the catalog root, which is what the stamper already did.
//
// tools/release-ledger/resolve-module-dir.sh is the same rule for the release
// workflows. TestResolveModuleDirMatchesShell keeps them together.
func resolveModuleDir(catalog string) (string, error) {
	if exists(filepath.Join(catalog, "go.mod")) {
		return catalog, nil
	}
	rel, err := goreleaserModuleRel(catalog)
	if err != nil {
		return "", err
	}
	if rel != "" {
		candidate := filepath.Join(catalog, rel)
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
	}
	rel, err = soleNestedModuleRel(catalog)
	if err != nil {
		return "", err
	}
	if rel != "" {
		return filepath.Join(catalog, rel), nil
	}
	return catalog, nil
}

func goreleaserModuleRel(catalog string) (string, error) {
	data, err := os.ReadFile(filepath.Join(catalog, ".goreleaser.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var dirs []string
	inBuilds := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indentOf(line) == 0 {
			inBuilds = trimmed == "builds:"
			continue
		}
		if !inBuilds {
			continue
		}
		item := trimmed
		if strings.HasPrefix(item, "- ") {
			item = strings.TrimSpace(strings.TrimPrefix(item, "- "))
		}
		key, val, ok := strings.Cut(item, ":")
		if !ok || strings.TrimSpace(key) != "dir" {
			continue
		}
		normalized, err := normalizeBuildDir(val)
		if err != nil {
			return "", fmt.Errorf("%s: %w", catalog, err)
		}
		if normalized != "" {
			dirs = append(dirs, normalized)
		}
	}
	unique := uniqueStrings(dirs)
	if len(unique) > 1 {
		return "", fmt.Errorf("%s: .goreleaser.yaml builds disagree on dir: %s", catalog, strings.Join(unique, ", "))
	}
	if len(unique) == 1 {
		return unique[0], nil
	}
	return "", nil
}

func normalizeBuildDir(raw string) (string, error) {
	val := strings.TrimSpace(raw)
	if hash := strings.Index(val, " #"); hash >= 0 {
		val = strings.TrimSpace(val[:hash])
	}
	val = strings.Trim(val, `"'`)
	val = strings.TrimSpace(val)
	if val == "" || val == "." || val == "./" {
		return "", nil
	}
	val = strings.TrimPrefix(val, "./")
	val = strings.TrimSuffix(val, "/")
	if val == "" || val == "." {
		return "", nil
	}
	if strings.Contains(val, "..") || strings.HasPrefix(val, "/") || strings.ContainsAny(val, " \t\\") {
		return "", fmt.Errorf("refusing goreleaser build dir %q", strings.TrimSpace(raw))
	}
	return val, nil
}

func soleNestedModuleRel(catalog string) (string, error) {
	entries, err := os.ReadDir(catalog)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") || !entry.IsDir() {
			continue
		}
		if exists(filepath.Join(catalog, name, "go.mod")) {
			found = append(found, name)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	return "", nil
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
