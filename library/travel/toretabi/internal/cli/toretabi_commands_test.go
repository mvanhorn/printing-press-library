// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/ticket"
)

func TestToretabiNegativeInputsAndDryRuns(t *testing.T) {
	testenv.Isolate(t)
	for _, tc := range []struct {
		args []string
		code int
	}{{[]string{"tickets", "get", "../bad", "--json"}, 2}, {[]string{"tickets", "compare", "tokai_043", "east_027", "--use-on", "2026-02-30", "--json"}, 2}, {[]string{"tickets", "cached", "--data-source", "live", "--json"}, 2}, {[]string{"tickets", "list", "--max-pages", "6", "--json"}, 2}, {[]string{"tickets", "get", "--dry-run", "--json"}, 0}, {[]string{"tickets", "compare", "--dry-run", "--json"}, 0}, {[]string{"tickets", "cached", "--dry-run", "--json"}, 0}} {
		cmd := RootCmd()
		var b bytes.Buffer
		cmd.SetOut(&b)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(append(tc.args, "--no-learn"))
		e := cmd.Execute()
		actualCode := 0
		if e != nil {
			actualCode = ExitCode(e)
		}
		if actualCode != tc.code {
			t.Fatalf("%v: %v %s", tc.args, e, b.String())
		}
		var out any
		if e = json.Unmarshal(b.Bytes(), &out); e != nil {
			t.Fatalf("%v lacks structured output: %s", tc.args, b.String())
		}
	}
}
func TestToretabiCachedProjectionAndEmpty(t *testing.T) {
	testenv.Isolate(t)
	home := t.TempDir()
	cache := filepath.Join(home, "cache", ticket.CacheFilename)
	x := ticket.Ticket{Summary: ticket.Summary{ID: "east_027", NameJA: "えちご", SourceURL: ticket.Origin + "/ticket/east_027.html", ObservedAt: "2026-10-04T11:00:00Z"}}
	if e := ticket.Save(context.Background(), cache, x); e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{"えちご", "never-match"} {
		cmd := RootCmd()
		var b bytes.Buffer
		cmd.SetOut(&b)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"tickets", "cached", "--home", home, "--query", query, "--json", "--select", "tickets.id,tickets.name_ja,note", "--no-learn"})
		if e := cmd.Execute(); e != nil {
			t.Fatal(e)
		}
		if query == "えちご" && !strings.Contains(b.String(), "east_027") {
			t.Fatalf("missing observation %s", b.String())
		}
		var out any
		if e := json.Unmarshal(b.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
	}
}

type fixtureDryRunClient bool

func (c fixtureDryRunClient) IsDryRun() bool { return bool(c) }
func TestDryRunMarkerRequiresConfiguredClient(t *testing.T) {
	for _, tc := range []struct {
		configured bool
		want       bool
	}{{true, true}, {false, false}} {
		if got := isDryRunResponseForClient(fixtureDryRunClient(tc.configured), json.RawMessage(`{"dry_run":true}`)); got != tc.want {
			t.Fatalf("configured=%v: got %v", tc.configured, got)
		}
	}
}

func TestToretabiNoGenericMutationEscape(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs([]string{"api", "POST", "/ticket/", "--dry-run", "--no-learn"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("generic mutation command exposed")
	}
}

func TestReviewerCachedOperatorIndependentStaleness(t *testing.T) {
	testenv.Isolate(t)
	home := t.TempDir()
	old := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	x := ticket.Ticket{Summary: ticket.Summary{ID: "east_027", NameJA: "えちご", SourceURL: ticket.Origin + "/ticket/east_027.html", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}, Operator: &ticket.OperatorEvidence{ObservedAt: &old, Status: "partial_rule_evidence", Transport: "live"}}
	if e := ticket.Save(context.Background(), filepath.Join(home, "cache", ticket.CacheFilename), x); e != nil {
		t.Fatal(e)
	}
	cmd := RootCmd()
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"tickets", "cached", "--home", home, "--max-age", "1s", "--json", "--no-learn"})
	if e := cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	var got struct {
		Tickets []ticket.Ticket `json:"tickets"`
	}
	if e := json.Unmarshal(b.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if len(got.Tickets) != 1 || got.Tickets[0].Operator == nil {
		t.Fatal(b.String())
	}
	if !got.Tickets[0].Operator.Stale {
		t.Errorf("hour-old operator observation labelled fresh: stale=%v age=%d publisher_stale=%v", got.Tickets[0].Operator.Stale, got.Tickets[0].Operator.ObservationAgeSeconds, got.Tickets[0].Stale)
	}
}

func TestToretabiIndependentCacheClocksReadOnlyAndDisabled(t *testing.T) {
	testenv.Isolate(t)
	for _, tc := range []struct {
		name                                                 string
		oldPublisher, oldOperator, unknownOperator, disabled bool
	}{{name: "operator_old", oldOperator: true}, {name: "publisher_old", oldPublisher: true}, {name: "disabled", oldPublisher: true, oldOperator: true, disabled: true}, {name: "unknown", unknownOperator: true}} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "cache", ticket.CacheFilename)
			now := time.Now().UTC()
			pub := now
			if tc.oldPublisher {
				pub = now.Add(-time.Hour)
			}
			op := now
			if tc.oldOperator {
				op = now.Add(-time.Hour)
			}
			opAt := op.Format(time.RFC3339Nano)
			x := ticket.Ticket{Summary: ticket.Summary{ID: "east_027", NameJA: "えちご", SourceURL: ticket.Origin + "/ticket/east_027.html", ObservedAt: pub.Format(time.RFC3339Nano)}, Operator: &ticket.OperatorEvidence{ObservedAt: &opAt, Stale: true, Status: "partial_rule_evidence", Transport: "live"}}
			if tc.unknownOperator {
				x.Operator.ObservedAt = nil
			}
			if e := ticket.Save(context.Background(), path, x); e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			for _, args := range [][]string{{"tickets", "cached"}, {"tickets", "get", "east_027", "--data-source", "local"}} {
				cmd := RootCmd()
				var b bytes.Buffer
				cmd.SetOut(&b)
				cmd.SetErr(&bytes.Buffer{})
				age := "5m"
				if tc.disabled {
					age = "0"
				}
				cmd.SetArgs(append(args, "--home", home, "--max-age", age, "--json", "--no-learn"))
				if e := cmd.Execute(); e != nil {
					t.Fatal(e)
				}
				var out struct {
					Ticket  ticket.Ticket   `json:"ticket"`
					Tickets []ticket.Ticket `json:"tickets"`
				}
				if e := json.Unmarshal(b.Bytes(), &out); e != nil {
					t.Fatal(e)
				}
				v := out.Ticket
				if len(out.Tickets) > 0 {
					v = out.Tickets[0]
				}
				if v.Operator == nil {
					t.Fatal("missing saved operator")
				}
				actualAt, e := time.Parse(time.RFC3339Nano, v.ObservedAt)
				if e != nil || !actualAt.Equal(pub) || v.Stale != (tc.oldPublisher && !tc.disabled) || v.Operator.Stale != (tc.oldOperator && !tc.disabled) {
					t.Fatalf("clocks: publisher_stale=%v operator_stale=%v publisher_at=%s original=%s", v.Stale, v.Operator.Stale, v.ObservedAt, x.ObservedAt)
				}
				if tc.unknownOperator {
					if v.Operator.ObservedAt != nil || v.Operator.Freshness != "unknown_clock" {
						t.Fatal(b.String())
					}
				} else if *v.Operator.ObservedAt != opAt {
					t.Fatal("operator clock changed")
				}
				if tc.disabled && v.Operator.Freshness != "disabled" {
					t.Fatal(b.String())
				}
			}
			after, e := os.ReadFile(path)
			if e != nil || string(before) != string(after) {
				t.Fatal("read wrote cache", e)
			}
		})
	}
}
