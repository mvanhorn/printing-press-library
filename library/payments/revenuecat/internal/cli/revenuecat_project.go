// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"
)

// resolveProjectID returns the project id from the --project flag, then the
// REVENUECAT_PROJECT_ID environment variable. Returns a usageErr when neither
// is set so the caller exits with the conventional usage exit code.
//
// Every RevenueCat v2 path is /projects/{project_id}/..., but the novel
// commands resolve the id ergonomically rather than as a positional argument.
// Call this AFTER the dry-run short-circuit so --dry-run works with no project
// configured.
func resolveProjectID(projectFlag string) (string, error) {
	if p := strings.TrimSpace(projectFlag); p != "" {
		return p, nil
	}
	if p := strings.TrimSpace(os.Getenv("REVENUECAT_PROJECT_ID")); p != "" {
		return p, nil
	}
	return "", usageErr(fmt.Errorf("--project is required (or set REVENUECAT_PROJECT_ID)"))
}
