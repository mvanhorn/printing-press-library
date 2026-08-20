package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/client"
	"github.com/spf13/cobra"
)

// parseArrayString parses a string that could be a JSON array of strings or a comma-separated list.
func parseArrayString(s string) []string {
	if s == "" {
		return nil
	}
	var res []string
	if err := json.Unmarshal([]byte(s), &res); err == nil {
		return res
	}
	parts := strings.Split(s, ",")
	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	return cleaned
}

// deptMember represents a member resolved from live API.
type deptMember struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// deptMemberResult represents a member for audit/values output.
type deptMemberResult struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Given    int64  `json:"given"`
	Received int64  `json:"received"`
}

// fetchDeptMembers fetches user IDs + emails for a department from live API.
func fetchDeptMembers(ctx context.Context, c *client.Client, flags *rootFlags, deptCanonical string, cmd *cobra.Command) ([]deptMember, error) {
	// Base confirmed at /users/departments (was wrong at bare /departments); this
	// exact sub-path segment for a specific department's members remains
	// unconfirmed, but /users/ is now the evidence-backed prefix, not a guess.
	path := replacePathParam("/users/departments/{department}/users", "department", deptCanonical)
	respRaw, _, err := resolveReadWithStrategyAndResponsePath(ctx, c, flags, "live", "departments", false, path, nil, nil, "", cmd.ErrOrStderr())
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}

	var members []deptMember
	if err := json.Unmarshal(respRaw, &members); err != nil {
		return nil, err
	}
	return members, nil
}

// checkMissingMirrorGuard returns true and prints missing mirror message if db doesn't exist.
func checkMissingMirrorGuard(cmd *cobra.Command, flags *rootFlags) (bool, string, error) {
	dbPath := defaultDBPath("bonusly-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: bonusly-pp-cli sync --resources <resource> --db %s\n", dbPath, dbPath)
		return true, "", nil
	}
	return false, dbPath, nil
}
