// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/url"
	"strings"
)

// sttJobPathSegment keeps user and provider job IDs within one URL segment.
func sttJobPathSegment(jobID string) (string, error) {
	if strings.TrimSpace(jobID) == "" {
		return "", fmt.Errorf("job ID is empty")
	}
	if jobID == "." || jobID == ".." {
		return "", fmt.Errorf("dot-only job ID is invalid")
	}
	return url.PathEscape(jobID), nil
}
