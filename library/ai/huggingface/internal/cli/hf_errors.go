// Seed-shadow exit-code helpers per docs/plans/2026-05-09-hf-cli-printing-press-seed.md.
//
// Framework's cliError + ExitCode (helpers.go + root.go) handles structured
// exits for spec-derived commands using framework codes (2/3/4/5/7/10).
// The seed's hf-specific commands contract a different code set (0/2/3/4/5/6).
//
// Rather than rewrite the framework's helpers (which would change behavior of
// auth/sync/profile/etc.), we provide hf-prefixed helpers that wrap cliError
// with the seed's code numbers. Hand-built hf commands return these; ExitCode
// reads .code unchanged. Schema entries already document seed codes.
package cli

import (
	"fmt"
)

// Seed exit codes (from internal/hfx/exit.go; mirrored here as call sites).
//
//   0 = ok
//   2 = not-found
//   3 = backend-incompatible
//   4 = already-cached
//   5 = rate-limited
//   6 = config/state-missing

func hfNotFound(format string, a ...any) error {
	return &cliError{code: 2, err: fmt.Errorf(format, a...)}
}

func hfBackendUnsupported(format string, a ...any) error {
	return &cliError{code: 3, err: fmt.Errorf(format, a...)}
}

func hfAlreadyCached(format string, a ...any) error {
	return &cliError{code: 4, err: fmt.Errorf(format, a...)}
}

func hfRateLimited(format string, a ...any) error {
	return &cliError{code: 5, err: fmt.Errorf(format, a...)}
}

func hfConfigMissing(format string, a ...any) error {
	return &cliError{code: 6, err: fmt.Errorf(format, a...)}
}
