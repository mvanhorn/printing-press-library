package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

// reconcilePendingApply resolves durable job IDs before undo inspects local paths.
// Operations left unknown after a failed poll keep their job ID and are polled
// again here: a job that is still running can finish after undo reads the
// index, so an unresolved job stops the undo instead of letting it proceed.
func reconcilePendingApply(ctx context.Context, db *store.Store, poster dropboxBatchPoster, batchID, accountID string, force bool) ([]store.DropboxJournalOp, error) {
	batch, found, err := db.GetDropboxJournalBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("journal batch %q not found", batchID)
	}
	if _, err := checkJournalAccount(batch, accountID, force); err != nil {
		return nil, err
	}
	ops, err := db.ListDropboxJournalOps(ctx, batchID)
	if err != nil {
		return nil, err
	}
	jobs := map[string][]store.DropboxJournalOp{}
	pendingJobs := map[string]bool{}
	for _, op := range ops {
		if op.Result == "pending" && op.AsyncJobID == "" {
			if err := db.SetDropboxPendingJournalOpResult(ctx, batchID, op.Seq, "unknown", "pending operation has no async job ID"); err != nil {
				return nil, err
			}
			continue
		}
		if op.AsyncJobID != "" {
			jobs[op.AsyncJobID] = append(jobs[op.AsyncJobID], op)
			if op.Result == "pending" || op.Result == "unknown" {
				pendingJobs[op.AsyncJobID] = true
			}
		}
	}
	ids := make([]string, 0, len(jobs))
	for id := range jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	unresolved := make([]string, 0)
	for _, id := range ids {
		if !pendingJobs[id] {
			continue
		}
		group := jobs[id]
		parents := make([]store.DropboxJournalOp, 0, len(group))
		for _, op := range group {
			if op.Op != "delete_child" {
				parents = append(parents, op)
			}
		}
		sort.Slice(parents, func(i, j int) bool {
			if parents[i].JobIndex != nil && parents[j].JobIndex != nil {
				return *parents[i].JobIndex < *parents[j].JobIndex
			}
			return parents[i].Seq < parents[j].Seq
		})
		if len(parents) == 0 {
			continue
		}
		route := applyJobCheckRoute(parents[0].Op)
		raw, pollErr := pollExistingJob(ctx, poster, route, id)
		states := make(map[int]string, len(group))
		details := make(map[int]string, len(group))
		if pollErr != nil {
			state := "unknown"
			var failed *dropbox.BatchFailedError
			if errors.As(pollErr, &failed) {
				state = "failed"
			} else if !asyncJobGone(pollErr) {
				unresolved = append(unresolved, pollErr.Error())
			}
			for _, op := range parents {
				states[op.Seq] = state
				details[op.Seq] = pollErr.Error()
			}
		} else {
			entryCount := len(parents)
			for _, op := range parents {
				if op.JobIndex != nil && *op.JobIndex >= 0 && *op.JobIndex < 1000 && *op.JobIndex+1 > entryCount {
					entryCount = *op.JobIndex + 1
				}
			}
			entryErrs := batchEntries(raw, entryCount)
			for i, op := range parents {
				if op.JobIndex != nil && *op.JobIndex >= 0 && *op.JobIndex < 1000 {
					i = *op.JobIndex
				}
				states[op.Seq] = "ok"
				if entryErrs[i] != nil {
					states[op.Seq] = "failed"
					details[op.Seq] = entryErrs[i].Error()
				}
			}
		}
		for _, op := range group {
			if op.Result != "pending" && op.Result != "unknown" {
				continue
			}
			state, detail := states[op.Seq], details[op.Seq]
			if op.Op == "delete_child" {
				state, detail = "unknown", "parent delete not found in batch"
				for _, parent := range parents {
					if parent.Op == "delete" && strings.HasPrefix(strings.ToLower(op.Path), strings.ToLower(parent.Path)+"/") {
						state, detail = states[parent.Seq], details[parent.Seq]
						break
					}
				}
			}
			if op.Result == "unknown" {
				if state == "unknown" {
					continue
				}
				if err := db.SetDropboxJournalOpResult(ctx, batchID, op.Seq, state, detail); err != nil {
					return nil, err
				}
				continue
			}
			if err := db.SetDropboxPendingJournalOpResult(ctx, batchID, op.Seq, state, detail); err != nil {
				return nil, err
			}
		}
	}
	ops, err = db.ListDropboxJournalOps(ctx, batchID)
	if err != nil {
		return nil, err
	}
	batch, found, err = db.GetDropboxJournalBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if found && batch.Status == "running" {
		status := "complete"
		for _, op := range ops {
			if op.Result != "ok" && op.Result != "skipped" {
				status = "partial"
				break
			}
		}
		if err := db.SetDropboxJournalStatus(ctx, batchID, status); err != nil {
			return nil, err
		}
	}
	if len(unresolved) > 0 {
		return nil, fmt.Errorf("%d Dropbox jobs from batch %s are still unresolved (%s); undo would race them, so retry undo once they finish", len(unresolved), batchID, unresolved[0])
	}
	return ops, nil
}

// asyncJobGone reports that Dropbox no longer tracks a job ID, so the job is
// not running and the refreshed index shows its final effect.
func asyncJobGone(err error) bool {
	return dropbox.HasSummaryPrefix(err, "invalid_async_job_id") || dropbox.HasSummaryPrefix(err, "async_job_id/not_found")
}

func applyJobCheckRoute(kind string) string {
	switch kind {
	case "move":
		return "/files/move_batch/check_v2"
	case "mkdir":
		return "/files/create_folder_batch/check"
	case "delete":
		return "/files/delete_batch/check"
	default:
		return ""
	}
}

func pollExistingJob(ctx context.Context, poster dropboxBatchPoster, route, id string) (json.RawMessage, error) {
	if route == "" {
		return nil, fmt.Errorf("unknown batch check route")
	}
	for attempts := 0; attempts < 120; attempts++ {
		raw, err := poster.Read(ctx, route, map[string]string{"async_job_id": id})
		if err != nil {
			return nil, fmt.Errorf("async job %s unavailable: %w", id, err)
		}
		var state struct {
			Tag string `json:".tag"`
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, err
		}
		switch state.Tag {
		case "complete":
			return raw, nil
		case "failed":
			return nil, &dropbox.BatchFailedError{Payload: raw}
		case "in_progress":
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		default:
			return nil, fmt.Errorf("unknown async job state %q", state.Tag)
		}
	}
	return nil, fmt.Errorf("async job %s did not complete", id)
}
