package dropbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeBatchPoster struct {
	results []json.RawMessage
	calls   int
}

func (f *fakeBatchPoster) next() (json.RawMessage, error) {
	f.calls++
	return f.results[f.calls-1], nil
}
func (f *fakeBatchPoster) Write(context.Context, string, any) (json.RawMessage, error) {
	return f.next()
}
func (f *fakeBatchPoster) Read(context.Context, string, any) (json.RawMessage, error) {
	return f.next()
}

func TestRunBatchJob(t *testing.T) {
	f := &fakeBatchPoster{results: []json.RawMessage{json.RawMessage(`{".tag":"async_job_id","async_job_id":"j"}`), json.RawMessage(`{".tag":"in_progress"}`), json.RawMessage(`{".tag":"complete","entries":[]}`)}}
	got, err := RunBatchJob(context.Background(), f, "/submit", "/check", nil, 0)
	if err != nil || f.calls != 3 || string(got) != string(f.results[2]) {
		t.Fatalf("got %s, calls %d, err %v", got, f.calls, err)
	}
	f = &fakeBatchPoster{results: []json.RawMessage{json.RawMessage(`{".tag":"failed","failed":{".tag":"other"}}`)}}
	_, err = RunBatchJob(context.Background(), f, "/submit", "/check", nil, time.Millisecond)
	var failed *BatchFailedError
	if !errors.As(err, &failed) {
		t.Fatalf("failed status error = %v", err)
	}
}

type observedPoster struct{ callbackSeen *bool }

func (p observedPoster) Write(context.Context, string, any) (json.RawMessage, error) {
	return json.RawMessage(`{".tag":"async_job_id","async_job_id":"job-1"}`), nil
}
func (p observedPoster) Read(context.Context, string, any) (json.RawMessage, error) {
	if !*p.callbackSeen {
		return nil, errors.New("polled before job was journaled")
	}
	return json.RawMessage(`{".tag":"complete","entries":[]}`), nil
}
func TestRunBatchJobReportsJobBeforePoll(t *testing.T) {
	seen := false
	_, err := RunBatchJobObserved(context.Background(), observedPoster{&seen}, "/submit", "/check", nil, 0, func(id string) error {
		if id != "job-1" {
			t.Fatalf("job id=%q", id)
		}
		seen = true
		return nil
	})
	if err != nil || !seen {
		t.Fatalf("seen=%t err=%v", seen, err)
	}
}
