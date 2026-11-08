// Package exitcode holds Printing Press-style typed process exits.
package exitcode

import "fmt"

const (
	OK        = 0
	Usage     = 2
	NotFound  = 3
	Auth      = 4
	API       = 5
	Transient = 7
)

// Error is a failure that should become a process exit code.
type Error struct {
	Code   int
	Err    error
	Silent bool
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

func As(err error, target **Error) bool {
	for err != nil {
		if ex, ok := err.(*Error); ok {
			*target = ex
			return true
		}
		type unwrap interface{ Unwrap() error }
		u, ok := err.(unwrap)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func Usagef(format string, args ...any) error {
	return &Error{Code: Usage, Err: fmt.Errorf(format, args...)}
}

func Authf(format string, args ...any) error {
	return &Error{Code: Auth, Err: fmt.Errorf(format, args...)}
}

func NotFoundf(format string, args ...any) error {
	return &Error{Code: NotFound, Err: fmt.Errorf(format, args...)}
}

func APIf(format string, args ...any) error {
	return &Error{Code: API, Err: fmt.Errorf(format, args...)}
}

func Transientf(format string, args ...any) error {
	return &Error{Code: Transient, Err: fmt.Errorf(format, args...)}
}
