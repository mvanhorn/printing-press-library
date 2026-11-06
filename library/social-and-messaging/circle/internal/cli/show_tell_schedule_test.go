// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
)

// TestNovelShowTellScheduleHelpWires smoke-tests that the show-tell schedule command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelShowTellScheduleHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"show-tell", "schedule", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show-tell schedule --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "schedule"} {
		if !strings.Contains(help, want) {
			t.Fatalf("show-tell schedule --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestShowTellDates(t *testing.T) {
	start := time.Date(2026, 11, 5, 18, 45, 30, 0, time.UTC)
	for _, tc := range []struct {
		name, every, from, until string
		want                     []string
		bad                      bool
	}{
		{"inclusive", "2w", "", "2026-12-03", []string{"2026-11-19T18:45:30Z", "2026-12-03T18:45:30Z"}, false},
		{"explicit start", "1w", "2026-11-06", "2026-11-13", []string{"2026-11-06T18:45:30Z", "2026-11-13T18:45:30Z"}, false},
		{"empty range", "2w", "", "2026-11-18", []string{}, false},
		{"hours are whole days", "48h", "", "2026-11-07", []string{"2026-11-07T18:45:30Z"}, false},
		{"fractional day", "36h", "", "2027-01-17", nil, true},
		{"zero", "0d", "", "2027-01-17", nil, true},
		{"negative", "-2w", "", "2027-01-17", nil, true},
		{"invalid cadence", "soon", "", "2027-01-17", nil, true},
		{"invalid from", "2w", "2026-02-30", "2027-01-17", nil, true},
		{"invalid until", "2w", "", "tomorrow", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cadence, err := showTellCadence(tc.every)
			var dates []time.Time
			if err == nil {
				dates, err = showTellDates(start, cadence, tc.from, tc.until)
			}
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v, bad=%v", err, tc.bad)
			}
			if tc.bad {
				return
			}
			got := make([]string, 0, len(dates))
			for _, date := range dates {
				got = append(got, date.Format(time.RFC3339))
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("dates=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestShowTellPlan(t *testing.T) {
	date := time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, eventName string
		start           time.Time
		want            string
	}{
		{"same day", "Show & Tell", date.Add(2 * time.Hour), "exists"},
		{"different name", "Workshop", date, "create"},
		{"next day", "Show & Tell", date.Add(24 * time.Hour), "create"},
		{"UTC day", "Show & Tell", time.Date(2026, 11, 20, 1, 0, 0, 0, time.FixedZone("offset", 8*3600)), "exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := showTellPlan("Show & Tell", []time.Time{date}, []circleEvent{{ID: json.Number("482719"), Name: tc.eventName, StartsAt: tc.start}}, date)
			if len(plan) != 1 || plan[0].Action != tc.want {
				t.Fatalf("plan=%+v", plan)
			}
			if tc.want == "exists" && plan[0].EventID != "482719" {
				t.Fatalf("missing existing ID: %+v", plan)
			}
		})
	}
}

func TestShowTellEventUpdate(t *testing.T) {
	start := time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)
	body := showTellEventUpdate(json.Number("1001070"), start, 90*time.Minute)
	event := body["event"].(map[string]any)
	if body["space_id"] != json.Number("1001070") || event["space_id"] != json.Number("1001070") {
		t.Fatalf("missing top-level or nested space_id: %v", body)
	}
	settings := event["event_setting_attributes"].(map[string]any)
	if event["status"] != "draft" || settings["starts_at"] != "2026-11-19T18:00:00Z" || settings["duration_in_seconds"] != int64(5400) {
		t.Fatalf("invalid update: %v", event)
	}
	for _, key := range []string{"send_email_confirmation", "send_email_reminder", "send_in_app_notification_confirmation", "send_in_app_notification_reminder", "send_publish_email"} {
		if settings[key] != false {
			t.Fatalf("notification %s not disabled", key)
		}
	}
	if _, ok := event["ends_at"]; ok {
		t.Fatal("ends_at is not an update property in the spec")
	}
}

type showTellScheduleTransport func(*http.Request) (*http.Response, error)

func (f showTellScheduleTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestShowTellScheduleRescheduleFailure(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusOK} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			testenv.Isolate(t)
			previousHooks := clientHooks
			t.Cleanup(func() { clientHooks = previousHooks })
			calls := make([]string, 0)
			registerClientHook(func(c *client.Client) error {
				c.Config.CircleApiToken = "schedule-unit-test"
				c.HTTPClient.Transport = showTellScheduleTransport(func(req *http.Request) (*http.Response, error) {
					call := req.Method + " " + req.URL.Path
					calls = append(calls, call)
					code, body := http.StatusOK, ""
					switch call {
					case "GET /api/admin/v2/events/30000788":
						body = `{"id":30000788,"name":"Show & Tell","space_id":1001070,"starts_at":"2026-11-05T18:00:00Z","duration_in_seconds":5400}`
					case "GET /api/admin/v2/events":
						body = `{"has_next_page":false,"records":[]}`
					case "POST /api/admin/v2/spaces/1001070/events/30000788/duplicate":
						body = `{"id":30000801}`
					case "PUT /api/admin/v2/events/30000801":
						var update struct {
							SpaceID int64 `json:"space_id"`
							Event   struct {
								SpaceID int64 `json:"space_id"`
							} `json:"event"`
						}
						if err := json.NewDecoder(req.Body).Decode(&update); err != nil || update.SpaceID != 1001070 || update.Event.SpaceID != 1001070 {
							t.Fatalf("invalid update body: %+v, error=%v", update, err)
						}
						code, body = status, `{"success":false,"message":"Reschedule rejected"}`
					default:
						return nil, fmt.Errorf("unexpected request: %s", call)
					}
					return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				})
				return nil
			})
			flags := &rootFlags{asJSON: true, yes: true, noCache: true}
			cmd := newNovelShowTellScheduleCmd(flags)
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--template", "30000788", "--every", "2w", "--until", "2026-12-03"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "30000801") {
				t.Fatalf("expected failure naming created event, got %v", err)
			}
			var result showTellSchedule
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("invalid result %q: %v", out.String(), err)
			}
			if result.Applied || len(result.Occurrences) != 2 || result.Occurrences[0].Action != "created-unscheduled" || result.Occurrences[0].EventID != "30000801" || result.Occurrences[1].Action != "create" || result.Occurrences[1].EventID != "" {
				t.Fatalf("invalid partial result: %+v", result)
			}
			if len(calls) != 4 {
				t.Fatalf("expected to stop after failed reschedule, got %v", calls)
			}
		})
	}
}

func TestShowTellLedgerRoundTrip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state", "show-tell-schedule.json")
	ledger, err := readShowTellLedger(file)
	if err != nil || len(ledger) != 0 {
		t.Fatalf("missing ledger: %v, %v", ledger, err)
	}
	ledger["30000788"] = []showTellLedgerEntry{{Date: "2026-12-03", EventID: "30000897"}}
	ledger["3859021"] = []showTellLedgerEntry{}
	if err := writeShowTellLedger(file, ledger); err != nil {
		t.Fatal(err)
	}
	got, err := readShowTellLedger(file)
	if err != nil || !reflect.DeepEqual(got, ledger) {
		t.Fatalf("round trip: %v, %v", got, err)
	}
	data, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(data), `"event_id": 30000897`) {
		t.Fatalf("expected numeric event ID: %s, %v", data, err)
	}
}

func TestShowTellLedgerVerification(t *testing.T) {
	date := time.Date(2026, 12, 3, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		err  error
		want string
		bad  bool
	}{
		{"live draft", nil, "exists", false},
		{"deleted draft", fmt.Errorf("wrapped: %w", &client.APIError{StatusCode: 404}), "create", false},
		{"forbidden", &client.APIError{StatusCode: 403}, "", true},
		{"unavailable", errors.New("connection failed"), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := validShowTellEntries([]showTellLedgerEntry{{Date: "2026-12-03", EventID: "30000897"}}, func(id string) error {
				if id != "30000897" {
					t.Fatalf("unexpected id %s", id)
				}
				return tc.err
			})
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v", err)
			}
			if tc.bad {
				return
			}
			published := []circleEvent{{ID: "30000898", Name: "Show & Tell", StartsAt: date.Add(14 * 24 * time.Hour)}}
			plan := showTellPlan("Show & Tell", []time.Time{date, date.Add(14 * 24 * time.Hour)}, published, date, entries...)
			if plan[0].Action != tc.want || plan[1].Action != "exists" || plan[1].EventID != "30000898" {
				t.Fatalf("plan=%+v", plan)
			}
			if tc.want == "exists" && plan[0].EventID != "30000897" {
				t.Fatalf("missing ledger ID: %+v", plan)
			}
		})
	}
}

func TestShowTellLedgerInvalid(t *testing.T) {
	for _, data := range []string{`null`, `{`, `{"30000788":[{"date":"tomorrow","event_id":30000897}]}`, `{"30000788":[{"date":"2026-12-03","event_id":0}]}`} {
		file := filepath.Join(t.TempDir(), "ledger.json")
		if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readShowTellLedger(file); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestShowTellScheduleLedgerRerun(t *testing.T) {
	testenv.Isolate(t)
	previousHooks := clientHooks
	t.Cleanup(func() { clientHooks = previousHooks })
	file := filepath.Join(t.TempDir(), "ledger.json")
	puts := 0
	registerClientHook(func(c *client.Client) error {
		c.Config.CircleApiToken = "schedule-unit-test"
		c.HTTPClient.Transport = showTellScheduleTransport(func(req *http.Request) (*http.Response, error) {
			body := ""
			code := http.StatusOK
			switch req.Method + " " + req.URL.Path {
			case "GET /api/admin/v2/events/30000788":
				body = `{"id":30000788,"name":"Show & Tell","space_id":1001070,"starts_at":"2026-11-19T18:00:00Z","duration_in_seconds":5400}`
			case "GET /api/admin/v2/events":
				body = `{"has_next_page":false,"records":[]}`
			case "GET /api/admin/v2/events/30000897", "GET /api/admin/v2/events/30000898":
				body = `{"id":30000897,"status":"draft"}`
			case "POST /api/admin/v2/spaces/1001070/events/30000788/duplicate":
				if puts == 0 {
					body = `{"id":30000897}`
				} else {
					body = `{"id":30000898}`
				}
			case "PUT /api/admin/v2/events/30000897", "PUT /api/admin/v2/events/30000898":
				puts++
				if puts == 2 {
					code = http.StatusBadRequest
					body = `{"success":false,"message":"rejected"}`
				} else {
					body = `{"id":30000897}`
				}
			default:
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})
		return nil
	})
	run := func(yes bool) (showTellSchedule, error) {
		cmd := newNovelShowTellScheduleCmd(&rootFlags{asJSON: true, yes: yes, noCache: true})
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"--template", "30000788", "--every", "2w", "--until", "2026-12-17", "--state-file", file})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		var result showTellSchedule
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		return result, err
	}
	if _, err := run(true); err == nil {
		t.Fatal("expected second reschedule to fail")
	}
	ledger, err := readShowTellLedger(file)
	if err != nil || len(ledger["30000788"]) != 2 || ledger["30000788"][0].Date != "2026-12-03" || ledger["30000788"][1].Status != "unscheduled" {
		t.Fatalf("partial run lost first occurrence: %v, %v", ledger, err)
	}
	result, err := run(false)
	if err != nil || result.Occurrences[0].Action != "exists" || result.Occurrences[0].EventID != "30000897" || result.Occurrences[1].Action != "exists" || puts != 2 {
		t.Fatalf("rerun=%+v, puts=%d, error=%v", result, puts, err)
	}
}

func TestShowTellScheduleDryRunSkipsLedger(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.json")
	if err := os.WriteFile(file, []byte("invalid ledger"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelShowTellScheduleCmd(&rootFlags{dryRun: true, asJSON: true})
	cmd.SetArgs([]string{"--state-file", file})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), `"dry_run":true`) {
		t.Fatalf("dry run read ledger: output=%s, error=%v", out.String(), err)
	}
}

func TestShowTellDatesDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ start, from, until, want string }{
		{"2026-03-01", "", "2026-03-08", "2026-03-08T12:00:00-04:00"},
		{"2026-10-25", "", "2026-11-01", "2026-11-01T12:00:00-05:00"},
		{"2026-03-01", "2026-03-08", "2026-03-08", "2026-03-08T12:00:00-04:00"},
	} {
		t.Run(tc.start+tc.from, func(t *testing.T) {
			start, err := time.ParseInLocation("2006-01-02 15:04", tc.start+" 12:00", loc)
			if err != nil {
				t.Fatal(err)
			}
			dates, err := showTellDates(start, 7*24*time.Hour, tc.from, tc.until)
			if err != nil || len(dates) != 1 || dates[0].Format(time.RFC3339) != tc.want {
				t.Fatalf("dates=%v, error=%v; want %s", dates, err, tc.want)
			}
		})
	}
}

func TestShowTellPlanPast(t *testing.T) {
	now := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		offset time.Duration
		want   string
	}{{-time.Second, "past"}, {0, "create"}, {time.Second, "create"}} {
		plan := showTellPlan("Show & Tell", []time.Time{now.Add(tc.offset)}, nil, now)
		if plan[0].Action != tc.want {
			t.Fatalf("offset %s: %+v", tc.offset, plan)
		}
		data, err := json.Marshal(plan)
		if err != nil || strings.Contains(string(data), "event_id") {
			t.Fatalf("missing ID was serialized: %s, %v", data, err)
		}
	}
}

func TestShowTellScheduleReviewCases(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		yes, ledger, past, fail bool
	}{{name: "missing ledger preview"}, {name: "preview prune", ledger: true}, {name: "apply prune", yes: true, ledger: true}, {name: "past apply", yes: true, past: true}, {name: "partial duplicate failure", yes: true, fail: true}} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			previousHooks := clientHooks
			t.Cleanup(func() { clientHooks = previousHooks })
			file := filepath.Join(t.TempDir(), "ledger.json")
			original := []byte(`{"30000788":[{"date":"2099-01-02","event_id":30000800}]}`)
			if tc.ledger {
				if err := os.WriteFile(file, original, 0600); err != nil {
					t.Fatal(err)
				}
			}
			start, until := "2099-01-01", "2099-01-03"
			if tc.past {
				start, until = "2000-01-01", "2000-01-03"
			}
			posts := 0
			registerClientHook(func(c *client.Client) error {
				c.Config.CircleApiToken = "schedule-unit-test"
				c.HTTPClient.Transport = showTellScheduleTransport(func(req *http.Request) (*http.Response, error) {
					code, body := 200, ""
					switch req.Method + " " + req.URL.Path {
					case "GET /api/admin/v2/events/30000788":
						body = fmt.Sprintf(`{"id":30000788,"name":"Show & Tell","space_id":1001070,"starts_at":"%sT18:00:00Z","duration_in_seconds":5400}`, start)
					case "GET /api/admin/v2/events":
						body = `{"has_next_page":false,"records":[]}`
					case "GET /api/admin/v2/events/30000800":
						code, body = 404, `{"success":false,"message":"gone"}`
					case "POST /api/admin/v2/spaces/1001070/events/30000788/duplicate":
						posts++
						if tc.fail && posts == 2 {
							code, body = 400, `{"success":false,"message":"rejected"}`
						} else {
							body = fmt.Sprintf(`{"id":%d}`, 30000800+posts)
						}
					case "PUT /api/admin/v2/events/30000801", "PUT /api/admin/v2/events/30000802":
						body = `{"success":true}`
					default:
						return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				})
				return nil
			})
			cmd := newNovelShowTellScheduleCmd(&rootFlags{asJSON: true, yes: tc.yes, noCache: true})
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--template", "30000788", "--every", "1d", "--until", until, "--state-file", file})
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			err := cmd.Execute()
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v", err)
			}
			var result showTellSchedule
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("output %s: %v", &out, err)
			}
			if !tc.ledger && (len(result.Warnings) != 1 || !strings.Contains(stderr.String(), result.Warnings[0])) {
				t.Fatalf("missing warning: %+v, %s", result, &stderr)
			}
			if (!tc.yes || tc.past) && posts != 0 {
				t.Fatalf("unexpected writes: %d", posts)
			}
			if tc.past && (result.Occurrences[0].Action != "past" || result.Occurrences[1].Action != "past") {
				t.Fatalf("past plan: %+v", result)
			}
			if tc.fail && (result.Occurrences[0].EventID != "30000801" || result.Occurrences[0].Action != "created") {
				t.Fatalf("lost partial result: %+v", result)
			}
			if !tc.yes {
				data, err := os.ReadFile(file)
				if tc.ledger && (err != nil || !bytes.Equal(data, original)) {
					t.Fatalf("preview changed ledger: %s, %v", data, err)
				}
				if !tc.ledger && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("preview created ledger: %v", err)
				}
			}
			if tc.ledger && tc.yes {
				ledger, err := readShowTellLedger(file)
				if err != nil || len(ledger["30000788"]) != 2 || ledger["30000788"][0].EventID == "30000800" {
					t.Fatalf("prune failed: %v, %v", ledger, err)
				}
			}
		})
	}
}
