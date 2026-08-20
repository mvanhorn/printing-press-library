package sourcetypes

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by stub clients to signal "this source's
// real implementation is deferred." The dispatcher uses errors.Is on the
// error returned from LookupByName to decide whether to surface it as a
// stub-skip trace entry vs. a real error.
var ErrNotImplemented = errors.New("source not yet implemented")

// StubClient is a reusable stub Client implementation. Every stubbed
// Stage-2 source uses this — they are real Go packages with the same
// shape as real sources, but their LookupByName returns ErrNotImplemented.
// They satisfy the wiring test (the regions table imports them, the
// dispatcher imports the regions table, cli imports the dispatcher) and
// they make the future "promote stub to real" change a single-package
// edit, not a regions-table edit.
type StubClient struct {
	SlugName   string
	LocaleCode string
	Reason     string
}

func (s *StubClient) Slug() string   { return s.SlugName }
func (s *StubClient) Locale() string { return s.LocaleCode }
func (s *StubClient) IsStub() bool   { return true }
func (s *StubClient) LookupByName(ctx context.Context, name, city string, maxResults int) ([]Hit, error) {
	return nil, ErrNotImplemented
}

// StubReason returns the stub's reason string (or "" for non-stubs). For
// use by `coverage` and `status` when explaining why a source returned
// nothing.
func StubReason(c Client) string {
	if !c.IsStub() {
		return ""
	}
	if sc, ok := c.(*StubClient); ok {
		return sc.Reason
	}
	return "stubbed"
}
