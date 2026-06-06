package cli

import "errors"

var As = errors.As

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func usageErr(err error) error    { return &cliError{code: 2, err: err} }
func notFoundErr(err error) error { return &cliError{code: 3, err: err} }
func apiErr(err error) error      { return &cliError{code: 5, err: err} }
