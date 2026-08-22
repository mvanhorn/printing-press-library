// Feature 5: Pre-flight Conflict Detection
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/jules/internal/client"
)

// checkConflictsFunc validates that a new session won't conflict with in-flight work
func checkConflictsFunc(ctx context.Context, c *client.Client, sourceContext map[string]any, out io.Writer) (bool, []string, error) {
	conflicts := []string{}

	// Extract repo info from sourceContext
	repo, ok := sourceContext["repo"].(string)
	if !ok {
		return false, conflicts, nil // No repo context, skip check
	}

	// Query active sessions for this repo
	data, err := c.Get(ctx, "/sessions", map[string]string{
		"pageSize": "100",
	})
	if err != nil {
		fmt.Fprintf(out, "Warning: Could not check for conflicts: %v\n", err)
		return false, conflicts, nil
	}

	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		return false, conflicts, nil
	}

	sessions, ok := response["sessions"].([]any)
	if !ok {
		return false, conflicts, nil
	}

	// Check each active session for potential conflicts
	for _, s := range sessions {
		sessionMap, ok := s.(map[string]any)
		if !ok {
			continue
		}

		sessionID, _ := sessionMap["id"].(string)
		state, _ := sessionMap["state"].(string)

		// Only check in-flight sessions
		if state != "PLANNING" && state != "IN_PROGRESS" && state != "AWAITING_PLAN_APPROVAL" {
			continue
		}

		// Check if this session is working on the same repo
		sc, ok := sessionMap["sourceContext"].(map[string]any)
		if !ok {
			continue
		}

		sessionRepo, _ := sc["repo"].(string)
		if sessionRepo == repo {
			conflicts = append(conflicts, fmt.Sprintf("Session %s is already working on %s", sessionID, repo))
		}
	}

	// Check for overlapping file modifications
	// In a real implementation, this would analyze git trees and detect overlaps
	if len(conflicts) > 0 {
		fmt.Fprintf(out, "⚠️  Conflict detected: %d other sessions are modifying this repo\n", len(conflicts))
		for _, c := range conflicts {
			fmt.Fprintf(out, "  - %s\n", c)
		}
		return true, conflicts, nil
	}

	return false, conflicts, nil
}

// addConflictDetectionToSessionCreate adds --check-conflicts flag via wrapper
// This is handled in feature_quota_throttling.go by extending the create command
