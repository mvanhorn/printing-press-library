// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
)

// typedExit signals cobra to exit with a non-zero code while still being
// recognized as a structured/typed result rather than a hard error.
type typedExit struct {
	code int
	msg  string
}

func (e *typedExit) Error() string { return e.msg }
func (e *typedExit) ExitCode() int { return e.code }

func stringContainsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(s)), strings.ToLower(strings.TrimSpace(substr)))
}

func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
