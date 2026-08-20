// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	apiclient "github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
)

type taskClientStub struct {
	calls          []string
	getError       error
	getErrors      map[string]error
	postResponse   json.RawMessage
	postError      error
	readyResponses []json.RawMessage
}

func (s *taskClientStub) Post(path string, _ any) (json.RawMessage, int, error) {
	s.calls = append(s.calls, "POST "+path)
	if s.postResponse != nil {
		return s.postResponse, 200, s.postError
	}
	return json.RawMessage(`{"tasks":[{"id":"task-1","status_code":20100}]}`), 200, nil
}

func (s *taskClientStub) GetFresh(path string, _ map[string]string) (json.RawMessage, error) {
	s.calls = append(s.calls, "GET "+path)
	if path == "/v3/serp/google/organic/tasks_ready" {
		if len(s.readyResponses) > 0 {
			response := s.readyResponses[0]
			s.readyResponses = s.readyResponses[1:]
			return response, nil
		}
		return json.RawMessage(`{"tasks":[{"result":[{"id":"task-1"}]}]}`), nil
	}
	if s.getError != nil {
		return nil, s.getError
	}
	if err := s.getErrors[path]; err != nil {
		return nil, err
	}
	return json.RawMessage(`{"tasks":[{"id":"task-1","status_code":20000,"result":[{"items":[]}]}]}`), nil
}

func TestResolveTaskPathsUsesStructuredGETEndpoint(t *testing.T) {
	paths, err := resolveTaskPaths(RootCmd(), "/v3/serp/google/organic/task_post")
	if err != nil {
		t.Fatal(err)
	}
	if paths.ready != "/v3/serp/google/organic/tasks_ready" {
		t.Fatalf("ready path = %q", paths.ready)
	}
	if paths.get != "/v3/serp/google/organic/task_get/advanced/{id}" {
		t.Fatalf("get path = %q", paths.get)
	}
}

func TestResolveTaskPathsUsesOnPageSummaryOverride(t *testing.T) {
	paths, err := resolveTaskPaths(RootCmd(), "/v3/on_page/task_post")
	if err != nil {
		t.Fatal(err)
	}
	if paths.get != "/v3/on_page/summary/{id}" {
		t.Fatalf("get path = %q", paths.get)
	}
}

func TestResolveTaskPathsSelectsJSONOnlyRoute(t *testing.T) {
	paths, err := resolveTaskPaths(RootCmd(), "/v3/on_page/lighthouse/task_post")
	if err != nil {
		t.Fatal(err)
	}
	if paths.get != "/v3/on_page/lighthouse/task_get/json/{id}" {
		t.Fatalf("get path = %q", paths.get)
	}
}

func TestResolveTaskPathsRejectsSynthesizedResultRoute(t *testing.T) {
	_, err := resolveTaskPaths(RootCmd(), "/v3/unknown/family/task_post")
	if err == nil || !strings.Contains(err.Error(), "cannot resolve task result path") {
		t.Fatalf("error = %v", err)
	}
}

func TestTaskBundleIsHiddenFromMCP(t *testing.T) {
	cmd := newTaskBundleCmd(&rootFlags{})
	if cmd.Annotations["mcp:hidden"] != "true" {
		t.Fatalf("mcp:hidden annotation = %q, want true", cmd.Annotations["mcp:hidden"])
	}
}

func TestRunTaskLifecycleUsesFreshGETsAndPersistsResult(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		t.Fatal(err)
	}

	client := &taskClientStub{}
	paths := taskPaths{
		post:  "/v3/serp/google/organic/task_post",
		ready: "/v3/serp/google/organic/tasks_ready",
		get:   "/v3/serp/google/organic/task_get/advanced/{id}",
	}
	results, err := runTaskLifecycleWithStore(context.Background(), client, s, paths, "serp/google/organic/task_post", []any{map[string]any{"keyword": "trees"}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	wantCalls := []string{
		"POST /v3/serp/google/organic/task_post",
		"GET /v3/serp/google/organic/tasks_ready",
		"GET /v3/serp/google/organic/task_get/advanced/task-1",
	}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", client.calls, wantCalls)
	}
	var status, result string
	if err := s.DB().QueryRow(`SELECT status, result FROM task_ledger WHERE task_id = ?`, "task-1").Scan(&status, &result); err != nil {
		t.Fatal(err)
	}
	if status != "fetched" || result == "" {
		t.Fatalf("ledger status=%q result=%q", status, result)
	}
}

func TestRunTaskLifecycleFailsWhenAnyResultFetchFails(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		t.Fatal(err)
	}

	client := &taskClientStub{getError: errors.New("fetch failed")}
	paths := taskPaths{post: "/v3/serp/google/organic/task_post", ready: "/v3/serp/google/organic/tasks_ready", get: "/v3/serp/google/organic/task_get/advanced/{id}"}
	_, err = runTaskLifecycleWithStore(context.Background(), client, s, paths, "serp/google/organic/task_post", []any{map[string]any{"keyword": "trees"}}, time.Millisecond, nil)
	if err == nil {
		t.Fatal("expected result fetch error")
	}
}

func TestRunTaskLifecycleRecordsAcceptedTasksFromMixedPostResponse(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		t.Fatal(err)
	}

	client := &taskClientStub{
		postResponse: json.RawMessage(`{"tasks":[{"id":"task-1","status_code":20100,"status_message":"Task Created."},{"status_code":40501,"status_message":"Invalid field"}]}`),
		postError:    &apiclient.DataForSEOError{Scope: "task", TaskIndex: 1, StatusCode: 40501, StatusMessage: "Invalid field", Partial: true},
	}
	paths := taskPaths{post: "/v3/serp/google/organic/task_post", ready: "/v3/serp/google/organic/tasks_ready", get: "/v3/serp/google/organic/task_get/advanced/{id}"}
	var warnings strings.Builder
	results, err := runTaskLifecycleWithStore(context.Background(), client, s, paths, "serp/google/organic/task_post", []any{map[string]any{"keyword": "trees"}}, 0, func(message string) {
		warnings.WriteString(message)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if warning := warnings.String(); !strings.Contains(warning, "item 2 rejected") || !strings.Contains(warning, "40501") || !strings.Contains(warning, "Invalid field") {
		t.Fatalf("warning = %q", warning)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM task_ledger`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("ledger row count = %d, want 1", count)
	}
}

func TestRunTaskLifecycleReturnsClearErrorWhenPostAcceptsNone(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		t.Fatal(err)
	}

	client := &taskClientStub{postResponse: json.RawMessage(`{"tasks":[{"status_code":40501,"status_message":"Invalid field"}]}`)}
	paths := taskPaths{post: "/v3/serp/google/organic/task_post", ready: "/v3/serp/google/organic/tasks_ready", get: "/v3/serp/google/organic/task_get/advanced/{id}"}
	_, err = runTaskLifecycleWithStore(context.Background(), client, s, paths, "serp/google/organic/task_post", []any{map[string]any{"keyword": "trees"}}, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "accepted no tasks") || !strings.Contains(err.Error(), "all 1 task(s) were rejected") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunTaskLifecycleTaskIDInsertIsAtomic(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`CREATE TRIGGER reject_second_task BEFORE INSERT ON task_ledger WHEN NEW.task_id = 'task-2' BEGIN SELECT RAISE(ABORT, 'reject task-2'); END`); err != nil {
		t.Fatal(err)
	}

	client := &taskClientStub{postResponse: json.RawMessage(`{"tasks":[{"id":"task-1","status_code":20100},{"id":"task-2","status_code":20100}]}`)}
	paths := taskPaths{post: "/v3/serp/google/organic/task_post", ready: "/v3/serp/google/organic/tasks_ready", get: "/v3/serp/google/organic/task_get/advanced/{id}"}
	_, err = runTaskLifecycleWithStore(context.Background(), client, s, paths, "serp/google/organic/task_post", []any{map[string]any{"keyword": "trees"}}, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "recording task task-2") {
		t.Fatalf("error = %v", err)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM task_ledger`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("ledger row count = %d, want 0", count)
	}
}

func TestRunTaskLifecyclePersistsEarlierFetchBeforeLaterFailure(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		t.Fatal(err)
	}

	client := &taskClientStub{
		postResponse: json.RawMessage(`{"tasks":[{"id":"task-1","status_code":20100},{"id":"task-2","status_code":20100}]}`),
		readyResponses: []json.RawMessage{
			json.RawMessage(`{"tasks":[{"result":[{"id":"task-1"},{"id":"task-2"}]}]}`),
		},
		getErrors: map[string]error{"/v3/serp/google/organic/task_get/advanced/task-2": errors.New("second fetch failed")},
	}
	paths := taskPaths{post: "/v3/serp/google/organic/task_post", ready: "/v3/serp/google/organic/tasks_ready", get: "/v3/serp/google/organic/task_get/advanced/{id}"}
	results, err := runTaskLifecycleWithStore(context.Background(), client, s, paths, "serp/google/organic/task_post", []any{map[string]any{"keyword": "trees"}}, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "fetching task task-2") {
		t.Fatalf("error = %v", err)
	}
	if results != nil {
		t.Fatalf("results = %#v, want nil on partial fetch", results)
	}

	var firstStatus, firstResult, secondStatus string
	if err := s.DB().QueryRow(`SELECT status, result FROM task_ledger WHERE task_id = 'task-1'`).Scan(&firstStatus, &firstResult); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT status FROM task_ledger WHERE task_id = 'task-2'`).Scan(&secondStatus); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "fetched" || firstResult == "" {
		t.Fatalf("first task status=%q result=%q", firstStatus, firstResult)
	}
	if secondStatus != "pending" {
		t.Fatalf("second task status=%q, want pending", secondStatus)
	}
}

func TestRunTaskLifecycleIgnoresUnrelatedReadyTaskIDs(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		t.Fatal(err)
	}

	client := &taskClientStub{readyResponses: []json.RawMessage{
		json.RawMessage(`{"tasks":[{"result":[{"id":"other-task"}]}]}`),
		json.RawMessage(`{"tasks":[{"result":[{"id":"task-1"}]}]}`),
	}}
	paths := taskPaths{post: "/v3/serp/google/organic/task_post", ready: "/v3/serp/google/organic/tasks_ready", get: "/v3/serp/google/organic/task_get/advanced/{id}"}
	if _, err := runTaskLifecycleWithStore(context.Background(), client, s, paths, "serp/google/organic/task_post", []any{map[string]any{"keyword": "trees"}}, 0, nil); err != nil {
		t.Fatal(err)
	}

	wantCalls := []string{
		"POST /v3/serp/google/organic/task_post",
		"GET /v3/serp/google/organic/tasks_ready",
		"GET /v3/serp/google/organic/tasks_ready",
		"GET /v3/serp/google/organic/task_get/advanced/task-1",
	}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", client.calls, wantCalls)
	}
}
