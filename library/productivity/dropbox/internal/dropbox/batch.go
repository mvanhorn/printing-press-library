package dropbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type BatchPoster interface {
	Write(context.Context, string, any) (json.RawMessage, error)
	Read(context.Context, string, any) (json.RawMessage, error)
}

type BatchFailedError struct {
	Payload json.RawMessage
}

func (e *BatchFailedError) Error() string { return fmt.Sprintf("Dropbox batch failed: %s", e.Payload) }

func RunBatchJob(ctx context.Context, poster BatchPoster, submitPath, checkPath string, body any, pollEvery time.Duration) (json.RawMessage, error) {
	return RunBatchJobObserved(ctx, poster, submitPath, checkPath, body, pollEvery, nil)
}

// RunBatchJobObserved reports an async job ID before the first poll, so a caller
// can durably associate its pending operations with the remote job.
func RunBatchJobObserved(ctx context.Context, poster BatchPoster, submitPath, checkPath string, body any, pollEvery time.Duration, onJob func(string) error) (json.RawMessage, error) {
	result, err := poster.Write(ctx, submitPath, body)
	if err != nil {
		return nil, err
	}
	for {
		var state struct {
			Tag string `json:".tag"`
			ID  string `json:"async_job_id"`
		}
		if err := json.Unmarshal(result, &state); err != nil {
			return nil, err
		}
		switch state.Tag {
		case "complete":
			return result, nil
		case "failed":
			return nil, &BatchFailedError{Payload: result}
		case "async_job_id":
			if state.ID == "" {
				return nil, fmt.Errorf("batch response lacks async_job_id")
			}
			if onJob != nil {
				if err := onJob(state.ID); err != nil {
					return nil, err
				}
				onJob = nil
			}
			body = map[string]string{"async_job_id": state.ID}
		case "in_progress":
			if body == nil {
				return nil, fmt.Errorf("batch polling lacks async_job_id")
			}
		default:
			return nil, fmt.Errorf("unknown batch status %q", state.Tag)
		}
		if pollEvery > 0 {
			timer := time.NewTimer(pollEvery)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		result, err = poster.Read(ctx, checkPath, body)
		if err != nil {
			return nil, err
		}
	}
}
