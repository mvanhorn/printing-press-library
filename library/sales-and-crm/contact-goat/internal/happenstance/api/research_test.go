// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResearch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/research" {
			t.Errorf("path = %q, want /research", r.URL.Path)
		}
		var body createResearchRequest
		readBody(t, r, &body)
		if body.Description != "deep dossier on Alice" {
			t.Errorf("description = %q", body.Description)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"rsch_1","url":"https://happenstance.ai/research/rsch_1"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.Research(context.Background(), "deep dossier on Alice")
	if err != nil {
		t.Fatalf("Research() error = %v", err)
	}
	if env.Id != "rsch_1" {
		t.Errorf("id = %q, want rsch_1", env.Id)
	}
}

func TestResearch_EmptyDescription(t *testing.T) {
	c := NewClient(testKey)
	_, err := c.Research(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestPollResearch_RunningThenCompletedWithProfile(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch n {
		case 1:
			io.WriteString(w, `{"id":"rsch_1","status":"RUNNING"}`)
		default:
			io.WriteString(w, `{"id":"rsch_1","status":"COMPLETED","profile":{"summary":"Alice runs the NBA Front Office.","employment":[{"title":"VP","company":"NBA"}]}}`)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.PollResearch(context.Background(), "rsch_1", &PollResearchOptions{
		Timeout:  5 * time.Second,
		Interval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PollResearch() error = %v", err)
	}
	if env.Status != StatusCompleted {
		t.Errorf("status = %q, want COMPLETED", env.Status)
	}
	if env.Profile == nil {
		t.Fatal("profile is nil; expected populated profile on COMPLETED")
	}
	if env.Profile.Summary != "Alice runs the NBA Front Office." {
		t.Errorf("summary = %q", env.Profile.Summary)
	}
	if len(env.Profile.Employment) != 1 {
		t.Fatalf("employment len = %d, want 1", len(env.Profile.Employment))
	}
}

func TestPollResearch_FailedAmbiguousIsTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"rsch_amb","status":"FAILED_AMBIGUOUS"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.PollResearch(context.Background(), "rsch_amb", &PollResearchOptions{
		Timeout:  100 * time.Millisecond,
		Interval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PollResearch() should not error on terminal status; got %v", err)
	}
	if env.Status != StatusFailedAmbiguous {
		t.Errorf("status = %q, want FAILED_AMBIGUOUS", env.Status)
	}
}

// TestPollResearch_ContextCancelledMidPoll covers the plan's required
// "Research honors ctx.Done() mid-poll and returns ctx.Err()" scenario.
func TestPollResearch_ContextCancelledMidPoll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"rsch_cancel","status":"RUNNING"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := c.PollResearch(ctx, "rsch_cancel", &PollResearchOptions{
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected ctx.Err() on cancellation, got nil")
	}
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestPollResearch_TimeoutHitsRunningCeilingNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"rsch_stuck","status":"RUNNING"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.PollResearch(context.Background(), "rsch_stuck", &PollResearchOptions{
		Timeout:  50 * time.Millisecond,
		Interval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PollResearch() should not error on timeout; got %v", err)
	}
	if env.Status != StatusRunning {
		t.Errorf("status = %q, want RUNNING", env.Status)
	}
}

func TestGetResearch_PathAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/research/rsch_42" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"rsch_42","status":"COMPLETED","profile":{"summary":"hi"}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.GetResearch(context.Background(), "rsch_42")
	if err != nil {
		t.Fatalf("GetResearch() error = %v", err)
	}
	if env.Id != "rsch_42" || env.Profile == nil || env.Profile.Summary != "hi" {
		t.Errorf("decoded envelope = %+v", env)
	}
}

func TestGroups_HappyAndEmpty(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"populated", `{"groups":[{"id":"g1","name":"NBA"},{"id":"g2","name":"YC"}]}`, 2},
		{"empty array", `{"groups":[]}`, 0},
		{"omitted field", `{}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/groups" {
					t.Errorf("path = %q", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			c := newTestClient(t, srv)
			gs, err := c.Groups(context.Background())
			if err != nil {
				t.Fatalf("Groups() error = %v", err)
			}
			if gs == nil {
				t.Fatal("Groups() returned nil; expected non-nil empty slice")
			}
			if len(gs) != tc.want {
				t.Errorf("len = %d, want %d", len(gs), tc.want)
			}
		})
	}
}

func TestGroup_PopulatesMembersAndDefendsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/groups/") {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		// No "members" field at all — Group must still return non-nil slice.
		io.WriteString(w, `{"id":"g_nomembers","name":"empty","member_count":0}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	g, err := c.Group(context.Background(), "g_nomembers")
	if err != nil {
		t.Fatalf("Group() error = %v", err)
	}
	if g.Members == nil {
		t.Error("Group.Members is nil; expected non-nil empty slice for safe range")
	}
}

func TestGroup_EmptyID(t *testing.T) {
	c := NewClient(testKey)
	_, err := c.Group(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestUsage_DecodesEverything(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/usage" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{
			"balance_credits": 42,
			"has_credits": true,
			"purchases": [{"id":"p1","credits":50,"amount_usd":"5.00","created_at":"2026-04-01T00:00:00Z"}],
			"usage": [{"id":"u1","kind":"search","credits":2,"created_at":"2026-04-19T00:00:00Z"}],
			"auto_reload": {"enabled":true,"threshold_credits":10,"top_up_credits":50}
		}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	u, err := c.Usage(context.Background())
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if u.BalanceCredits != 42 || !u.HasCredits {
		t.Errorf("balance/has_credits = %d/%v", u.BalanceCredits, u.HasCredits)
	}
	if len(u.Purchases) != 1 || u.Purchases[0].Credits != 50 {
		t.Errorf("purchases = %+v", u.Purchases)
	}
	if len(u.Usage) != 1 || u.Usage[0].Kind != "search" {
		t.Errorf("usage = %+v", u.Usage)
	}
	if u.AutoReload == nil || !u.AutoReload.Enabled {
		t.Errorf("auto_reload = %+v", u.AutoReload)
	}
}

func TestUsage_DefendsAgainstNilSlices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"balance_credits":0,"has_credits":false}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	u, err := c.Usage(context.Background())
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if u.Purchases == nil || u.Usage == nil {
		t.Errorf("nil slices returned: purchases=%v usage=%v", u.Purchases, u.Usage)
	}
}
