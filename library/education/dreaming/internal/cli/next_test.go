// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"
)

// seedNovelStore points the default store at a temp HOME and seeds the videos
// and playlist tables the novel commands (next, diet) query. Returns after the
// store is closed so the command under test can reopen it.
func seedNovelStore(t *testing.T, videos []string, playlist []string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	s, err := store.OpenWithContext(context.Background(), filepath.Join(tmp, ".local", "share", "dreaming-pp-cli", "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	for _, stmt := range videos {
		if _, err := s.DB().Exec(stmt); err != nil {
			t.Fatalf("seed videos: %v\n%s", err, stmt)
		}
	}
	for _, stmt := range playlist {
		if _, err := s.DB().Exec(stmt); err != nil {
			t.Fatalf("seed playlist: %v\n%s", err, stmt)
		}
	}
}

func runNovelCmd(t *testing.T, cmd interface {
	SetOut(io.Writer)
	SetErr(io.Writer)
	SetArgs([]string)
	ExecuteContext(context.Context) error
}, args []string) string {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return out.String()
}

func TestNextCommand(t *testing.T) {
	seedNovelStore(t,
		[]string{
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v-thirty', '{}', 'B thirty', 'beginner', 30)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v-ten', '{}', 'A ten', 'beginner', 10)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v-unrated', '{}', 'C unrated', 'beginner', NULL)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v-other-level', '{}', 'D other level', 'intermediate', 5)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v-watched', '{}', 'E watched', 'beginner', 20)`,
		},
		[]string{
			`INSERT INTO playlist (id, data, added_date, video_id) VALUES ('p1', '{}', '2026-01-01', 'v-watched')`,
		},
	)

	cases := []struct {
		name    string
		args    []string
		wantIDs []string
	}{
		{
			// Level band scopes the query, watched videos are excluded, rated
			// videos come first in ascending difficulty, unrated sort last.
			name:    "band filter, watched exclusion, unrated last",
			args:    []string{"--level", "beginner"},
			wantIDs: []string{"v-ten", "v-thirty", "v-unrated"},
		},
		{
			name:    "include watched keeps difficulty order",
			args:    []string{"--level", "beginner", "--include-watched"},
			wantIDs: []string{"v-ten", "v-watched", "v-thirty", "v-unrated"},
		},
		{
			name:    "limit truncates",
			args:    []string{"--level", "beginner", "--limit", "1"},
			wantIDs: []string{"v-ten"},
		},
		{
			name:    "other level band",
			args:    []string{"--level", "intermediate"},
			wantIDs: []string{"v-other-level"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{asJSON: true}
			out := runNovelCmd(t, newNovelNextCmd(flags), tc.args)
			var got []nextVideo
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("unmarshal %q: %v", out, err)
			}
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %d videos, want %d: %s", len(got), len(tc.wantIDs), out)
			}
			for i, want := range tc.wantIDs {
				if got[i].ID != want {
					t.Errorf("position %d: got %s, want %s (full order: %s)", i, got[i].ID, want, out)
				}
			}
		})
	}
}

func TestNextAutoLevelDoesNotDefaultToLevelOne(t *testing.T) {
	seedNovelStore(t,
		[]string{`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v1', '{}', 'Beginner video', 'beginner', 10)`},
		nil,
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("DREAMING_BASE_URL", server.URL)
	t.Setenv("DREAMING_TOKEN", "test-token")

	flags := &rootFlags{asJSON: true}
	cmd := newNovelNextCmd(flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("got error %v, want user-fetch error instead of Level 1 results", err)
	}
}
