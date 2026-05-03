package searchbackend

import (
	"context"
	"testing"
)

type testBackend struct{}

func (testBackend) Name() string { return "unit" }
func (testBackend) Search(context.Context, string, SearchOpts) ([]Result, error) {
	return []Result{{Title: "Result", URL: "https://example.com", Domain: "example.com"}}, nil
}
func (testBackend) ImageSearch(context.Context, string, SearchOpts) ([]Result, error) {
	return nil, ErrUnsupported
}

func TestByName(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "happy path", key: "UNIT_TEST_BACKEND", want: "unit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Register(tt.key, func() Backend { return testBackend{} })
			got := ByName(tt.key)
			if got.Name() != tt.want {
				t.Fatalf("Name = %q, want %q", got.Name(), tt.want)
			}
		})
	}
}
