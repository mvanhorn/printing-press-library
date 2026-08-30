// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"os"
	"strings"
)

// errorsAs is a thin wrapper so agoda_shared.go can stay free of the errors
// import while still doing typed-error matching.
func errorsAs(err error, target any) bool { return errors.As(err, target) }

// agodaCookie returns a session cookie for member-priced and account surfaces.
//
// Public search needs no credentials, so an empty value is a normal state rather
// than a configuration error.
func agodaCookie() string {
	for _, env := range []string{"AGODA_COOKIE", "AGODA_SESSION_COOKIE"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	return ""
}

// hasAgodaSession reports whether authenticated commands can run.
func hasAgodaSession() bool { return agodaCookie() != "" }
