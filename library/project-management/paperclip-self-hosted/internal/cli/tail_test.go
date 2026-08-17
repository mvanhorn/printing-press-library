package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type failingTailClient struct {
	err error
}

func (c failingTailClient) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	return nil, c.err
}

func TestFetchInitialTailReturnsErrorForSinglePoll(t *testing.T) {
	want := errors.New("fetch failed")
	err := fetchInitialTail(context.Background(), failingTailClient{err: want}, "/issues", json.NewEncoder(&bytes.Buffer{}), false)
	if !errors.Is(err, want) {
		t.Fatalf("fetchInitialTail error = %v, want %v", err, want)
	}
}

func TestFetchInitialTailKeepsFollowingAfterError(t *testing.T) {
	err := fetchInitialTail(context.Background(), failingTailClient{err: errors.New("temporary")}, "/issues", json.NewEncoder(&bytes.Buffer{}), true)
	if err != nil {
		t.Fatalf("fetchInitialTail error = %v, want nil", err)
	}
}

func TestValidateTailOptionsRejectsNonPositiveFollowInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		if err := validateTailOptions(true, interval); err == nil {
			t.Fatalf("validateTailOptions(true, %s) returned nil", interval)
		}
	}
	if err := validateTailOptions(false, 0); err != nil {
		t.Fatalf("single poll should not require a positive interval: %v", err)
	}
}
