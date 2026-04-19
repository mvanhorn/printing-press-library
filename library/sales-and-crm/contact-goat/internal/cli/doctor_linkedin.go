// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/linkedin"
)

// checkLinkedIn adds LinkedIn-MCP health entries to the doctor report. It is
// called from newDoctorCmd alongside the built-in checks.
//
// Keys added:
//   - linkedin_python      : "ok (3.11.7)" or "missing" / "too old"
//   - linkedin_uvx         : "ok" / "missing"
//   - linkedin_binary      : "global linkedin-scraper-mcp" / "via uvx" / "none"
//   - linkedin_profile     : "ok" / "not logged in (run ...)"
func checkLinkedIn(report map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Python 3.10+
	if bin, verOut, err := linkedin.PythonAvailable(ctx); err != nil {
		report["linkedin_python"] = "missing — install Python 3.10+ from python.org or Homebrew"
	} else {
		verStr := strings.TrimSpace(verOut)
		if ok, parsed := parsePython310Plus(verStr); ok {
			report["linkedin_python"] = fmt.Sprintf("ok (%s at %s)", parsed, bin)
		} else {
			report["linkedin_python"] = fmt.Sprintf("too old (%s) — need 3.10+", parsed)
		}
	}

	// uvx / global binary
	if bin, ok := linkedin.GloballyInstalledBinary(); ok {
		report["linkedin_binary"] = fmt.Sprintf("ok (%s)", bin)
		if uvx, err := linkedin.UVXAvailable(); err == nil {
			report["linkedin_uvx"] = fmt.Sprintf("ok (%s)", uvx)
		} else {
			report["linkedin_uvx"] = "missing (not needed — global install present)"
		}
	} else if uvx, err := linkedin.UVXAvailable(); err == nil {
		report["linkedin_uvx"] = fmt.Sprintf("ok (%s)", uvx)
		report["linkedin_binary"] = "will launch via `uvx linkedin-scraper-mcp@latest`"
	} else {
		report["linkedin_uvx"] = "missing — install uv from https://astral.sh/uv"
		report["linkedin_binary"] = "missing — `pipx install linkedin-scraper-mcp` or install uv"
	}

	// Profile / login state
	profilePath, perr := linkedin.ProfilePath()
	if perr != nil {
		report["linkedin_profile"] = fmt.Sprintf("error: %s", perr)
	} else if ok, _ := linkedin.IsLoggedIn(); ok {
		report["linkedin_profile"] = fmt.Sprintf("ok (%s)", profilePath)
	} else {
		report["linkedin_profile"] = fmt.Sprintf(
			"not logged in — run `uvx linkedin-scraper-mcp@latest --login` (expects %s)",
			profilePath)
	}
}

var pythonVersionRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// parsePython310Plus inspects a `python --version` string like
// "Python 3.11.7" and reports whether it satisfies >= 3.10. Returns
// (ok, displayVersion).
func parsePython310Plus(verLine string) (bool, string) {
	m := pythonVersionRe.FindStringSubmatch(verLine)
	if m == nil {
		return false, verLine
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	v := m[0]
	if major > 3 || (major == 3 && minor >= 10) {
		return true, v
	}
	return false, v
}
