// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence support for clickup-2-pp-cli. Shared parsing and
// helpers for the local-store project-management commands (my-day,
// changed-since, workload, time-in-status, stale, unblocked, resolve).

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
)

// pmTask is the slice of a synced ClickUp task the PM commands reason about.
// Everything else stays in Raw for --select / --json passthrough.
type pmTask struct {
	ID           string          `json:"id"`
	CustomID     string          `json:"custom_id,omitempty"`
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	StatusType   string          `json:"status_type"`
	Assignees    []pmAssignee    `json:"assignees,omitempty"`
	DueDate      int64           `json:"due_date_ms,omitempty"`
	DateUpdated  int64           `json:"date_updated_ms,omitempty"`
	DateClosed   int64           `json:"date_closed_ms,omitempty"`
	TimeEstimate int64           `json:"time_estimate_ms,omitempty"`
	ListID       string          `json:"list_id,omitempty"`
	SpaceID      string          `json:"space_id,omitempty"`
	URL          string          `json:"url,omitempty"`
	Deps         []pmDependency  `json:"-"`
	Raw          json.RawMessage `json:"-"`
}

type pmAssignee struct {
	ID       int64  `json:"id"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

type pmDependency struct {
	TaskID    string `json:"task_id"`
	DependsOn string `json:"depends_on"`
	Type      int    `json:"type"`
}

// open reports whether the task is in a non-closed status.
func (t pmTask) open() bool {
	return strings.ToLower(t.StatusType) != "closed" && t.DateClosed == 0
}

// msToString renders a ms-epoch as RFC3339, or "" for zero.
func msToString(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// parseMSField pulls a ClickUp millisecond-epoch field that the API returns
// as either a JSON string ("1709251200000") or a number. Returns 0 when
// absent/null/unparseable.
func parseMSField(v any) int64 {
	switch x := v.(type) {
	case string:
		if x == "" {
			return 0
		}
		n, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0
		}
		return n
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	}
	return 0
}

// parsePMTask decodes one synced task JSON blob into a pmTask.
func parsePMTask(raw json.RawMessage) (pmTask, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return pmTask{}, false
	}
	t := pmTask{Raw: raw}
	if v, ok := obj["id"].(string); ok {
		t.ID = v
	}
	if t.ID == "" {
		return pmTask{}, false
	}
	if v, ok := obj["custom_id"].(string); ok {
		t.CustomID = v
	}
	if v, ok := obj["name"].(string); ok {
		t.Name = v
	}
	if s, ok := obj["status"].(map[string]any); ok {
		if v, ok := s["status"].(string); ok {
			t.Status = v
		}
		if v, ok := s["type"].(string); ok {
			t.StatusType = v
		}
	}
	if arr, ok := obj["assignees"].([]any); ok {
		for _, a := range arr {
			m, ok := a.(map[string]any)
			if !ok {
				continue
			}
			as := pmAssignee{}
			switch id := m["id"].(type) {
			case float64:
				as.ID = int64(id)
			case string:
				as.ID, _ = strconv.ParseInt(id, 10, 64)
			}
			if v, ok := m["username"].(string); ok {
				as.Username = v
			}
			if v, ok := m["email"].(string); ok {
				as.Email = v
			}
			t.Assignees = append(t.Assignees, as)
		}
	}
	t.DueDate = parseMSField(obj["due_date"])
	t.DateUpdated = parseMSField(obj["date_updated"])
	t.DateClosed = parseMSField(obj["date_closed"])
	t.TimeEstimate = parseMSField(obj["time_estimate"])
	if l, ok := obj["list"].(map[string]any); ok {
		if v, ok := l["id"].(string); ok {
			t.ListID = v
		}
	}
	if sp, ok := obj["space"].(map[string]any); ok {
		if v, ok := sp["id"].(string); ok {
			t.SpaceID = v
		}
	}
	if v, ok := obj["url"].(string); ok {
		t.URL = v
	}
	if arr, ok := obj["dependencies"].([]any); ok {
		for _, d := range arr {
			m, ok := d.(map[string]any)
			if !ok {
				continue
			}
			dep := pmDependency{}
			if v, ok := m["task_id"].(string); ok {
				dep.TaskID = v
			}
			if v, ok := m["depends_on"].(string); ok {
				dep.DependsOn = v
			}
			switch ty := m["type"].(type) {
			case float64:
				dep.Type = int(ty)
			case string:
				dep.Type, _ = strconv.Atoi(ty)
			}
			t.Deps = append(t.Deps, dep)
		}
	}
	return t, true
}

// loadPMTasks reads every synced task from the local store and parses it.
// Returns a clear, actionable error when the store has no tasks yet.
func loadPMTasks(db *store.Store) ([]pmTask, error) {
	rows, err := db.List("task", pmListLimit)
	if err != nil {
		return nil, fmt.Errorf("reading tasks from local store: %w", err)
	}
	tasks := make([]pmTask, 0, len(rows))
	for _, r := range rows {
		if t, ok := parsePMTask(r); ok {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

// noTasksHint is the shared message for an empty task store.
const pmListLimit = 1000000

const noTasksHint = "no tasks in the local store. Run 'clickup-2-pp-cli sync' first to populate it."

// matchAssignee reports whether the task is assigned to the given selector,
// which may be a numeric id, a username, or "me" (resolved upstream to an id).
func (t pmTask) matchAssignee(selector string, meID int64) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return true
	}
	for _, a := range t.Assignees {
		if selector == "me" && meID != 0 && a.ID == meID {
			return true
		}
		if strconv.FormatInt(a.ID, 10) == selector {
			return true
		}
		if strings.EqualFold(a.Username, selector) {
			return true
		}
		if strings.EqualFold(a.Email, selector) {
			return true
		}
	}
	return false
}

// resolveMeID best-effort reads the authenticated user's numeric id from a
// synced `user` record (GET /v2/user => {"user":{"id":...}}). Returns 0 when
// unavailable, so callers degrade gracefully offline.
func resolveMeID(db *store.Store) int64 {
	for _, rt := range []string{"user", "team_member", "member"} {
		rows, err := db.List(rt, pmListLimit)
		if err != nil {
			continue
		}
		for _, r := range rows {
			var obj map[string]any
			if json.Unmarshal(r, &obj) != nil {
				continue
			}
			// Unwrap {"user": {...}} envelope if present.
			if u, ok := obj["user"].(map[string]any); ok {
				obj = u
			}
			switch id := obj["id"].(type) {
			case float64:
				return int64(id)
			case string:
				if n, err := strconv.ParseInt(id, 10, 64); err == nil {
					return n
				}
			}
		}
	}
	return 0
}
