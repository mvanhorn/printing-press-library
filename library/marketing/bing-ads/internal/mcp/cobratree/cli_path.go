// Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SiblingCLIPath resolves the companion CLI via sibling-of-executable,
// BING_ADS_CLI_PATH env var, then PATH.
func SiblingCLIPath() (string, error) {
	if exe, err := os.Executable(); err == nil {
		for _, candidate := range siblingCLICandidates(runtime.GOOS, exe) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	if v := os.Getenv("BING_ADS_CLI_PATH"); v != "" {
		return v, nil
	}
	return exec.LookPath(cliExecutableName(runtime.GOOS))
}

func siblingCLICandidates(goos, exePath string) []string {
	dir := filepath.Dir(exePath)
	name := "bing-ads-pp-cli"
	if goos == "windows" {
		return []string{filepath.Join(dir, name+".exe"), filepath.Join(dir, name)}
	}
	return []string{filepath.Join(dir, name)}
}

func cliExecutableName(goos string) string {
	name := "bing-ads-pp-cli"
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}
