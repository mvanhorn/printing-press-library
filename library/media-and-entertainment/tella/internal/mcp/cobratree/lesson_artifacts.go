// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cobratree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// These workflows intentionally exchange files; keep MCP paths in one artifact directory.
func lessonArtifactFlags(command string) map[string]string {
	switch command {
	case "videos storyboard-image":
		return map[string]string{"output": ".webp"}
	case "lessons ship":
		return map[string]string{"output-dir": ""}
	case "lessons pass":
		return map[string]string{"recipe": ".yaml|.yml"}
	case "clips find-retakes", "videos clips find-retakes":
		return map[string]string{"plan": ".json", "save-plan": ".json"}
	}
	return nil
}
func lessonMCPArguments(command string, args map[string]any) (map[string]any, error) {
	scoped := lessonArtifactFlags(command)
	if len(scoped) == 0 {
		return args, nil
	}
	out := map[string]any{}
	for k, v := range args {
		out[k] = v
	}
	if command == "lessons ship" {
		if _, ok := out["output-dir"]; !ok {
			out["output-dir"] = "delivery"
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(cwd, "tella-artifacts")
	if info, err := os.Lstat(root); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("tella-artifacts must be a real directory")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for flag, extensions := range scoped {
		value, exists := out[flag]
		if !exists {
			continue
		}
		name, ok := value.(string)
		if !ok || name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\\x00") || filepath.IsAbs(name) {
			return nil, fmt.Errorf("%s must be a plain filename inside tella-artifacts", flag)
		}
		if extensions != "" && !strings.Contains("|"+extensions+"|", "|"+strings.ToLower(filepath.Ext(name))+"|") {
			return nil, fmt.Errorf("%s requires %s", flag, extensions)
		}
		target := filepath.Join(root, name)
		if info, err := os.Lstat(target); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("artifact symlinks are refused")
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		out[flag] = target
	}
	return out, nil
}
