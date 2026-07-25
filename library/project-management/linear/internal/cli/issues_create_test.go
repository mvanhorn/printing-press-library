package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIssueInputFromFlags(t *testing.T) {
	tests := []struct {
		name string
		in   func() map[string]any
		want map[string]any
	}{
		{
			name: "minimal title and team only",
			in: func() map[string]any {
				return issueInputFromFlags("Fix login", "ENG", "", "", "", "", 0, nil)
			},
			want: map[string]any{"title": "Fix login", "teamId": "ENG"},
		},
		{
			name: "all optional fields populated",
			in: func() map[string]any {
				return issueInputFromFlags("Big feature", "team-uuid", "desc body", "assignee-uuid", "project-uuid", "state-uuid", 2, []string{"label-a", "label-b"})
			},
			want: map[string]any{
				"title":       "Big feature",
				"teamId":      "team-uuid",
				"description": "desc body",
				"assigneeId":  "assignee-uuid",
				"projectId":   "project-uuid",
				"stateId":     "state-uuid",
				"priority":    2,
				"labelIds":    []string{"label-a", "label-b"},
			},
		},
		{
			name: "priority zero is omitted",
			in: func() map[string]any {
				return issueInputFromFlags("x", "ENG", "", "", "", "", 0, nil)
			},
			want: map[string]any{"title": "x", "teamId": "ENG"},
		},
		{
			name: "empty optional strings are omitted",
			in: func() map[string]any {
				return issueInputFromFlags("x", "ENG", "", "", "", "", 3, nil)
			},
			want: map[string]any{"title": "x", "teamId": "ENG", "priority": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issueInputFromFlags mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestIssuesCreateComposesMutationInput(t *testing.T) {
	const (
		teamID     = "11111111-1111-1111-1111-111111111111"
		stateID    = "22222222-2222-2222-2222-222222222222"
		projectID  = "33333333-3333-3333-3333-333333333333"
		assigneeID = "44444444-4444-4444-4444-444444444444"
	)
	tests := []struct {
		name      string
		args      []string
		wantInput map[string]any
		wantState map[string]string
	}{
		{
			name:      "minimum viable title and team",
			args:      []string{"--title", "Fix login", "--team", teamID},
			wantInput: map[string]any{"title": "Fix login", "teamId": teamID},
		},
		{
			name: "fully populated mutation input",
			args: []string{
				"--title", "Big feature",
				"--team", teamID,
				"--description", "desc body",
				"--state", stateID,
				"--label", "label-a",
				"--label", "label-b",
				"--project", projectID,
				"--assignee", assigneeID,
				"--priority", "2",
			},
			wantInput: map[string]any{
				"title":       "Big feature",
				"teamId":      teamID,
				"description": "desc body",
				"stateId":     stateID,
				"labelIds":    []any{"label-a", "label-b"},
				"projectId":   projectID,
				"assigneeId":  assigneeID,
				"priority":    float64(2),
			},
		},
		{
			name:      "state name selector is preserved",
			args:      []string{"--title", "Named state", "--team", teamID, "--state-name", "In Progress"},
			wantInput: map[string]any{"title": "Named state", "teamId": teamID},
			wantState: map[string]string{"state_name": "In Progress"},
		},
		{
			name:      "state type is normalized",
			args:      []string{"--title", "Typed state", "--team", teamID, "--state-type", " STARTED "},
			wantInput: map[string]any{"title": "Typed state", "teamId": teamID},
			wantState: map[string]string{"state_type": "started"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"issues", "create"}, tt.args...)
			args = append(args, "--db", filepath.Join(t.TempDir(), "linear.db"), "--dry-run", "--json")
			out, err := executeRootForTest(args...)
			if err != nil {
				t.Fatalf("issues create dry-run failed: %v\n%s", err, out)
			}
			var preview struct {
				Event     string         `json:"event"`
				Mutation  string         `json:"mutation"`
				Input     map[string]any `json:"input"`
				StateName string         `json:"state_name"`
				StateType string         `json:"state_type"`
			}
			if err := json.Unmarshal([]byte(out), &preview); err != nil {
				t.Fatalf("dry-run output is not JSON: %v\n%s", err, out)
			}
			if preview.Event != "would_create_issue" || preview.Mutation != "issueCreate" {
				t.Fatalf("unexpected dry-run envelope: %+v", preview)
			}
			if !reflect.DeepEqual(preview.Input, tt.wantInput) {
				t.Errorf("mutation input mismatch\n got: %#v\nwant: %#v", preview.Input, tt.wantInput)
			}
			gotState := map[string]string{}
			if preview.StateName != "" {
				gotState["state_name"] = preview.StateName
			}
			if preview.StateType != "" {
				gotState["state_type"] = preview.StateType
			}
			wantState := tt.wantState
			if wantState == nil {
				wantState = map[string]string{}
			}
			if !reflect.DeepEqual(gotState, wantState) {
				t.Errorf("state selector mismatch\n got: %#v\nwant: %#v", gotState, tt.wantState)
			}
		})
	}
}

func TestIssuesCreateRejectsInvalidStateSelectors(t *testing.T) {
	const teamID = "11111111-1111-1111-1111-111111111111"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "state must be UUID",
			args:    []string{"--state", "In Progress"},
			wantErr: `--state expects a workflow state UUID (got "In Progress")`,
		},
		{
			name:    "state and state name are mutually exclusive",
			args:    []string{"--state", "22222222-2222-2222-2222-222222222222", "--state-name", "In Progress"},
			wantErr: "pass exactly one of --state, --state-name, or --state-type",
		},
		{
			name:    "state name and state type are mutually exclusive",
			args:    []string{"--state-name", "In Progress", "--state-type", "started"},
			wantErr: "pass exactly one of --state, --state-name, or --state-type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"issues", "create", "--title", "x", "--team", teamID}
			args = append(args, tt.args...)
			args = append(args, "--db", filepath.Join(t.TempDir(), "linear.db"), "--dry-run")
			_, err := executeRootForTest(args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected usage error containing %q, got %v", tt.wantErr, err)
			}
			if ExitCode(err) != 2 {
				t.Fatalf("expected exit code 2 usage error, got %d (%T: %v)", ExitCode(err), err, err)
			}
		})
	}
}
