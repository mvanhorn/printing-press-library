package cmd

import (
	"errors"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
)

// ExitCode maps an error to a CLI exit code.
//
//	1 — generic API error
//	2 — authentication required / invalid credentials
//	3 — invalid user input
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var authErr *client.AuthError
	if errors.As(err, &authErr) {
		return 2
	}
	var inputErr *InputError
	if errors.As(err, &inputErr) {
		return 3
	}
	return 1
}

// InputError signals a user-input validation failure (exit code 3).
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }
