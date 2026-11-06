// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestShowTellVTT(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"empty", "WEBVTT\n\n", ""},
		{"header cues and speaker merge", "WEBVTT\nKind: captions\n\n1\n00:00:00.000 --> 00:00:02.000\nAda: Hello\nworld\n\ncue-two\n00:00:02.000 --> 00:00:04.000 align:start\nAda: Again\n\n3\n00:00:04.000 --> 00:00:05.000\nBo: Hi", "Ada: Hello world Again\nBo: Hi"},
		{"note block", "WEBVTT\n\nNOTE private note\nignore me\n00:00:00.000 --> 00:00:02.000\nhidden\n\n1\n00:00:02.000 --> 00:00:04.000\nVisible", "Visible"},
		{"voice markup and entities", "\ufeffWEBVTT\r\n\r\ncue\r\n00:00:00.000 --> 00:00:02.000\r\n<v Ada><b>Hello</b> &amp; welcome</v>\r\n\r\nnext\r\n00:00:02.000 --> 00:00:04.000\r\n<v Ada>back</v>", "Ada: Hello & welcome back"},
		{"style and region", "WEBVTT\n\nSTYLE\n::cue { color: red }\n\nREGION\nid:foo\n\n00:00:00.000 --> 00:00:01.000\nText", "Text"},
		{"numeric dialogue", "WEBVTT\n\n1\n00:00:00.000 --> 00:00:01.000\n42", "42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := showTellVTT(tc.input); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShowTellRoom(t *testing.T) {
	start := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	room := func(id string, offset time.Duration) circleRoom {
		at := start.Add(offset)
		return circleRoom{ID: json.Number(id), LiveAt: &at}
	}
	for _, tc := range []struct {
		name  string
		rooms []circleRoom
		want  string
	}{
		{"none", nil, ""},
		{"null live_at", []circleRoom{{ID: "1"}}, ""},
		{"lower boundary", []circleRoom{room("1", -30*time.Minute)}, "1"},
		{"upper boundary", []circleRoom{room("2", 90*time.Minute)}, "2"},
		{"outside", []circleRoom{room("1", -30*time.Minute-time.Second), room("2", 90*time.Minute+time.Second)}, ""},
		{"closest absolute distance", []circleRoom{room("1", 20*time.Minute), room("2", -5*time.Minute), room("3", 10*time.Minute)}, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := showTellRoom(tc.rooms, start, end).String(); got != tc.want {
				t.Fatalf("room=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestCircleEventSchedule(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		seconds   int64
		bad       bool
	}{
		{"ends at", `{"starts_at":"2026-11-05T18:00:00Z","ends_at":"2026-11-05T19:30:00Z"}`, 5400, false},
		{"duration", `{"starts_at":"2026-11-05T18:00:00Z","duration_in_seconds":3600}`, 3600, false},
		{"no start", `{"duration_in_seconds":3600}`, 0, true},
		{"no duration", `{"starts_at":"2026-11-05T18:00:00Z"}`, 0, true},
		{"backwards", `{"starts_at":"2026-11-05T18:00:00Z","ends_at":"2026-11-05T17:00:00Z"}`, 0, true},
		{"negative", `{"starts_at":"2026-11-05T18:00:00Z","duration_in_seconds":-1}`, 0, true},
		{"overflow", `{"starts_at":"2026-11-05T18:00:00Z","duration_in_seconds":9223372036854775807}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var event circleEvent
			if err := json.Unmarshal([]byte(tc.raw), &event); err != nil {
				t.Fatal(err)
			}
			_, duration, err := event.schedule()
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v", err)
			}
			if !tc.bad && int64(duration/time.Second) != tc.seconds {
				t.Fatalf("duration=%s", duration)
			}
		})
	}
}

func TestCircleDecode(t *testing.T) {
	for _, tc := range []struct {
		raw string
		bad bool
	}{
		{`{"success":false,"message":"Denied"}`, true},
		{`{"id":1000969}`, false},
		{`not JSON`, true},
	} {
		if err := circleDecode([]byte(tc.raw), nil); (err != nil) != tc.bad {
			t.Fatalf("decode %s: %v", tc.raw, err)
		}
	}
}

func TestCirclePositiveID(t *testing.T) {
	for _, tc := range []struct {
		id  string
		bad bool
	}{{"1000969", false}, {"", true}, {"0", true}, {"-1", true}, {"1/duplicate", true}, {"+1", true}, {" 1", true}} {
		if err := circlePositiveID(tc.id); (err != nil) != tc.bad {
			t.Fatalf("ID %q: %v", tc.id, err)
		}
	}
}

func TestCircleTranscript(t *testing.T) {
	const vtt = "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello"
	for _, tc := range []struct {
		name, entry, body, redirect string
		bad                         bool
		calls                       int
	}{
		{name: "inline", entry: vtt},
		{name: "http", entry: "http://recording.example/transcript", bad: true},
		{name: "ftp", entry: "ftp://recording.example/transcript", bad: true},
		{name: "credentials", entry: "https://user:password@recording.example/transcript", bad: true},
		{name: "download", entry: "https://recording.example/transcript", body: vtt, calls: 1},
		{name: "at limit", entry: "https://recording.example/transcript", body: vtt + strings.Repeat(" ", (10<<20)-len(vtt)), calls: 1},
		{name: "too large", entry: "https://recording.example/transcript", body: strings.Repeat("x", (10<<20)+1), bad: true, calls: 1},
		{name: "https redirect", entry: "https://recording.example/transcript", body: vtt, redirect: "https://storage.example/transcript", calls: 2},
		{name: "downgrade redirect", entry: "https://recording.example/transcript", redirect: "http://storage.example/transcript", bad: true, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			calls := 0
			http.DefaultTransport = showTellScheduleTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Header.Get("Authorization") != "" {
					t.Fatal("authentication leaked")
				}
				code, headers := 200, make(http.Header)
				if calls == 1 && tc.redirect != "" {
					code = 302
					headers.Set("Location", tc.redirect)
				}
				return &http.Response{StatusCode: code, Header: headers, Body: io.NopCloser(strings.NewReader(tc.body)), Request: req}, nil
			})
			got, err := circleTranscript(context.Background(), tc.entry)
			if (err != nil) != tc.bad || calls != tc.calls {
				t.Fatalf("got=%q, error=%v, calls=%d", got, err, calls)
			}
			if !tc.bad && got != "Hello" {
				t.Fatalf("transcript=%q", got)
			}
		})
	}
}

func TestCircleTranscriptRedirectLimit(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://storage.example/transcript", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{9, 10} {
		if err := circleTranscriptRedirect(req, make([]*http.Request, count)); (err != nil) != (count == 10) {
			t.Fatalf("count %d: %v", count, err)
		}
	}
}
