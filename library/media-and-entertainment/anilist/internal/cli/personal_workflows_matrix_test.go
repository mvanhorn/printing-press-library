// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func newGraphQLFixture(t *testing.T, respond func(graphQLRequest) any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			t.Fatalf("request = %s %s, want POST /", r.Method, r.URL.Path)
		}
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"data": respond(request)}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
}

func runPersonalWorkflow(t *testing.T, server *httptest.Server, args ...string) (string, error) {
	t.Helper()
	t.Setenv("ANILIST_BASE_URL", server.URL)
	t.Setenv("ANILIST_TOKEN", "test-token")
	cmd := RootCmd()
	var output bytes.Buffer
	setCommandOutput(cmd, &output)
	args = append(args, "--json", "--no-learn", "--home", t.TempDir())
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func setCommandOutput(cmd *cobra.Command, output *bytes.Buffer) {
	cmd.SetOut(output)
	cmd.SetErr(output)
	for _, child := range cmd.Commands() {
		setCommandOutput(child, output)
	}
}

func gqlPage(items any, hasNext bool) map[string]any {
	return map[string]any{"Page": map[string]any{"pageInfo": map[string]any{"hasNextPage": hasNext}, "mediaList": items}}
}

func mediaEntry(id, progress, priority int, score float64, listStatus, mediaStatus string, episodes, duration int) map[string]any {
	return map[string]any{
		"id": id + 1000, "progress": progress, "priority": priority, "score": score, "status": listStatus,
		"media": map[string]any{"id": id, "title": map[string]any{"userPreferred": fmt.Sprintf("Show %d", id)}, "episodes": episodes, "duration": duration, "status": mediaStatus},
	}
}

func intVariable(t *testing.T, request graphQLRequest, key string) int {
	t.Helper()
	v, ok := request.Variables[key].(float64)
	if !ok {
		t.Fatalf("variable %q = %#v, want JSON number", key, request.Variables[key])
	}
	return int(v)
}

func decodeObject(t *testing.T, text string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("decode object %q: %v", text, err)
	}
	return got
}

func decodeObjects(t *testing.T, text string) []map[string]any {
	t.Helper()
	var got []map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("decode array %q: %v", text, err)
	}
	return got
}

func setPersonalNow(t *testing.T, instant time.Time) {
	t.Helper()
	original := personalNow
	personalNow = func() time.Time { return instant }
	t.Cleanup(func() { personalNow = original })
}

func TestScheduleTonightMatrix(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		now  time.Time
		span time.Duration
	}{
		{name: "ordinary local day", now: time.Date(2026, 1, 15, 9, 0, 0, 0, location), span: 24 * time.Hour},
		{name: "DST spring-forward local day", now: time.Date(2026, 3, 8, 9, 0, 0, 0, location), span: 23 * time.Hour},
	} {
		t.Run(test.name, func(t *testing.T) {
			setPersonalNow(t, test.now)
			start := time.Date(test.now.Year(), test.now.Month(), test.now.Day(), 0, 0, 0, 0, location)
			end := start.AddDate(0, 0, 1)
			server := newGraphQLFixture(t, func(request graphQLRequest) any {
				switch {
				case strings.Contains(request.Query, "Viewer"):
					return map[string]any{"Viewer": map[string]any{"id": 7}}
				case strings.Contains(request.Query, "mediaList(userId"):
					return gqlPage([]any{mediaEntry(11, 0, 0, 0, "CURRENT", "RELEASING", 12, 24)}, false)
				case strings.Contains(request.Query, "airingSchedules"):
					if got := intVariable(t, request, "from"); got != int(start.Unix()-1) {
						t.Fatalf("from = %d, want %d", got, start.Unix()-1)
					}
					if got := intVariable(t, request, "to"); got != int(end.Unix()) {
						t.Fatalf("to = %d, want %d", got, end.Unix())
					}
					if end.Sub(start) != test.span {
						t.Fatalf("window span = %s, want %s", end.Sub(start), test.span)
					}
					return map[string]any{"Page": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "airingSchedules": []any{map[string]any{"episode": 1, "airingAt": start.Unix(), "media": map[string]any{"id": 11, "title": map[string]any{"userPreferred": "Show 11"}}}}}}
				default:
					t.Fatalf("unexpected query: %s", request.Query)
				}
				return nil
			})
			defer server.Close()
			output, err := runPersonalWorkflow(t, server, "schedule", "tonight", "--timezone", "America/New_York")
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeObjects(t, output); len(got) != 1 || int(got[0]["episode"].(float64)) != 1 {
				t.Fatalf("schedule output = %s", output)
			}
		})
	}

	t.Run("all pages only followed and exclusive next midnight", func(t *testing.T) {
		setPersonalNow(t, time.Date(2026, 1, 15, 9, 0, 0, 0, location))
		start := time.Date(2026, 1, 15, 0, 0, 0, 0, location)
		end := start.AddDate(0, 0, 1)
		server := newGraphQLFixture(t, func(request graphQLRequest) any {
			switch {
			case strings.Contains(request.Query, "Viewer"):
				return map[string]any{"Viewer": map[string]any{"id": 7}}
			case strings.Contains(request.Query, "mediaList(userId"):
				if intVariable(t, request, "page") == 1 {
					return gqlPage([]any{mediaEntry(11, 0, 0, 0, "CURRENT", "RELEASING", 12, 24)}, true)
				}
				return gqlPage([]any{mediaEntry(22, 0, 0, 0, "CURRENT", "RELEASING", 12, 24)}, false)
			case strings.Contains(request.Query, "airingSchedules"):
				if intVariable(t, request, "page") == 1 {
					return map[string]any{"Page": map[string]any{"pageInfo": map[string]any{"hasNextPage": true}, "airingSchedules": []any{
						map[string]any{"episode": 1, "airingAt": start.Unix(), "media": map[string]any{"id": 11, "title": map[string]any{"userPreferred": "Followed"}}},
						map[string]any{"episode": 2, "airingAt": start.Add(time.Hour).Unix(), "media": map[string]any{"id": 999, "title": map[string]any{"userPreferred": "Unfollowed"}}},
						map[string]any{"episode": 3, "airingAt": end.Unix(), "media": map[string]any{"id": 11, "title": map[string]any{"userPreferred": "Tomorrow"}}},
					}}}
				}
				return map[string]any{"Page": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "airingSchedules": []any{map[string]any{"episode": 4, "airingAt": start.Add(2 * time.Hour).Unix(), "media": map[string]any{"id": 22, "title": map[string]any{"userPreferred": "Second page"}}}}}}
			default:
				t.Fatalf("unexpected query: %s", request.Query)
			}
			return nil
		})
		defer server.Close()
		output, err := runPersonalWorkflow(t, server, "schedule", "tonight", "--timezone", "America/New_York")
		if err != nil {
			t.Fatal(err)
		}
		got := decodeObjects(t, output)
		if len(got) != 2 || int(got[0]["media"].(map[string]any)["id"].(float64)) != 11 || int(got[1]["media"].(map[string]any)["id"].(float64)) != 22 {
			t.Fatalf("schedule output = %s", output)
		}
	})

	server := newGraphQLFixture(t, func(graphQLRequest) any { t.Fatal("invalid zone made HTTP request"); return nil })
	defer server.Close()
	if _, err := runPersonalWorkflow(t, server, "schedule", "tonight", "--timezone", "not/a/zone"); err == nil {
		t.Fatal("invalid IANA timezone succeeded")
	}
}

func TestProgressCheckInMatrix(t *testing.T) {
	t.Run("ambiguous title and no-entry reject without mutation", func(t *testing.T) {
		for _, test := range []struct {
			name string
			args []string
			data func(graphQLRequest) any
		}{
			{name: "ambiguous", args: []string{"progress", "check-in", "Cowboy", "--episode", "2"}, data: func(request graphQLRequest) any {
				switch {
				case strings.Contains(request.Query, "Viewer"):
					return map[string]any{"Viewer": map[string]any{"id": 7}}
				case strings.Contains(request.Query, "media(search"):
					return map[string]any{"Page": map[string]any{"media": []any{map[string]any{"id": 1}, map[string]any{"id": 2}}}}
				}
				t.Fatalf("unexpected query: %s", request.Query)
				return nil
			}},
			{name: "no entry", args: []string{"progress", "check-in", "42", "--episode", "2"}, data: func(request graphQLRequest) any {
				if strings.Contains(request.Query, "Viewer") {
					return map[string]any{"Viewer": map[string]any{"id": 7}}
				}
				if strings.Contains(request.Query, "MediaList(userId") {
					return map[string]any{"MediaList": nil}
				}
				t.Fatalf("unexpected query: %s", request.Query)
				return nil
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				mutation := false
				server := newGraphQLFixture(t, func(request graphQLRequest) any {
					if strings.Contains(request.Query, "mutation") {
						mutation = true
					}
					return test.data(request)
				})
				defer server.Close()
				if _, err := runPersonalWorkflow(t, server, test.args...); err == nil {
					t.Fatal("expected error")
				}
				if mutation {
					t.Fatal("rejection sent mutation")
				}
			})
		}
	})

	entry := func(progress int) map[string]any {
		return mediaEntry(42, progress, 0, 0, "CURRENT", "RELEASING", 12, 24)
	}
	t.Run("preview is non-mutating and IDs bypass search", func(t *testing.T) {
		mutation, search := false, false
		server := newGraphQLFixture(t, func(request graphQLRequest) any {
			switch {
			case strings.Contains(request.Query, "mutation"):
				mutation = true
			case strings.Contains(request.Query, "media(search"):
				search = true
			case strings.Contains(request.Query, "Viewer"):
				return map[string]any{"Viewer": map[string]any{"id": 7}}
			case strings.Contains(request.Query, "MediaList(userId"):
				return map[string]any{"MediaList": entry(3)}
			default:
				t.Fatalf("unexpected query: %s", request.Query)
			}
			return map[string]any{}
		})
		defer server.Close()
		output, err := runPersonalWorkflow(t, server, "progress", "check-in", "42", "--episode", "5")
		if err != nil {
			t.Fatal(err)
		}
		got := decodeObject(t, output)
		if got["before"] != float64(3) || got["after"] != float64(5) || mutation || search {
			t.Fatalf("preview=%s mutation=%t search=%t", output, mutation, search)
		}
	})

	for _, test := range []struct {
		name             string
		episode          int
		first, second    int
		mutationProgress int
		wantError        bool
	}{
		{name: "regression", episode: 2, first: 3, wantError: true},
		{name: "known total overflow", episode: 13, first: 3, wantError: true},
		{name: "JIT drift", episode: 5, first: 3, second: 4, wantError: true},
		{name: "returned mismatch", episode: 5, first: 3, second: 3, mutationProgress: 4, wantError: true},
		{name: "apply uses exact payload", episode: 5, first: 3, second: 3, mutationProgress: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			entryReads, mutation := 0, false
			server := newGraphQLFixture(t, func(request graphQLRequest) any {
				switch {
				case strings.Contains(request.Query, "Viewer"):
					return map[string]any{"Viewer": map[string]any{"id": 7}}
				case strings.Contains(request.Query, "MediaList(userId"):
					entryReads++
					progress := test.first
					if entryReads == 2 {
						progress = test.second
					}
					return map[string]any{"MediaList": entry(progress)}
				case strings.Contains(request.Query, "SaveMediaListEntry"):
					mutation = true
					if got := intVariable(t, request, "media"); got != 42 {
						t.Fatalf("media = %d, want 42", got)
					}
					if got := intVariable(t, request, "progress"); got != test.episode {
						t.Fatalf("progress = %d, want %d", got, test.episode)
					}
					return map[string]any{"SaveMediaListEntry": entry(test.mutationProgress)}
				default:
					t.Fatalf("unexpected query: %s", request.Query)
				}
				return nil
			})
			defer server.Close()
			_, err := runPersonalWorkflow(t, server, "progress", "check-in", "42", "--episode", fmt.Sprint(test.episode), "--apply")
			if (err != nil) != test.wantError {
				t.Fatalf("err = %v, wantError %t", err, test.wantError)
			}
			if test.wantError && (test.name == "regression" || test.name == "known total overflow" || test.name == "JIT drift") && mutation {
				t.Fatal("rejected update sent mutation")
			}
			if !test.wantError && !mutation {
				t.Fatal("valid apply did not mutate")
			}
		})
	}
}

func TestBacklogPickMatrix(t *testing.T) {
	server := newGraphQLFixture(t, func(graphQLRequest) any { t.Fatal("invalid bounds made HTTP request"); return nil })
	defer server.Close()
	for _, args := range [][]string{{"backlog", "pick"}, {"backlog", "pick", "--max-episodes", "0", "--max-runtime-minutes", "30"}} {
		if _, err := runPersonalWorkflow(t, server, args...); err == nil {
			t.Fatalf("%v succeeded", args)
		}
	}

	entries := []personalEntry{
		decodeEntry(mediaEntry(10, 0, 5, 80, "PLANNING", "RELEASING", 12, 24)),
		decodeEntry(mediaEntry(20, 0, 4, 99, "PLANNING", "RELEASING", 1, 1)),
		decodeEntry(mediaEntry(30, 0, 5, 70, "PLANNING", "RELEASING", 1, 1)),
		decodeEntry(mediaEntry(40, 0, 5, 80, "PLANNING", "RELEASING", 13, 1)),
		decodeEntry(mediaEntry(50, 0, 5, 80, "PLANNING", "RELEASING", 12, 25)),
		decodeEntry(mediaEntry(60, 0, 5, 80, "PLANNING", "RELEASING", 0, 24)),
		decodeEntry(mediaEntry(70, 0, 5, 80, "PLANNING", "FINISHED", 12, 24)),
		decodeEntry(mediaEntry(80, 0, 5, 80, "COMPLETED", "RELEASING", 12, 24)),
	}
	ranked := rankBacklogCandidates(entries, 12, 24)
	if got := ranked[0].Media.ID; got != 10 {
		t.Fatalf("best candidate = %d, want 10", got)
	}

	t.Run("all planning pages are consumed", func(t *testing.T) {
		server := newGraphQLFixture(t, func(request graphQLRequest) any {
			switch {
			case strings.Contains(request.Query, "Viewer"):
				return map[string]any{"Viewer": map[string]any{"id": 7}}
			case strings.Contains(request.Query, "mediaList(userId"):
				if intVariable(t, request, "page") == 1 {
					return gqlPage([]any{mediaEntry(1, 0, 1, 1, "PLANNING", "RELEASING", 13, 24)}, true)
				}
				return gqlPage([]any{mediaEntry(2, 0, 2, 1, "PLANNING", "RELEASING", 12, 24)}, false)
			default:
				t.Fatalf("unexpected query: %s", request.Query)
			}
			return nil
		})
		defer server.Close()
		output, err := runPersonalWorkflow(t, server, "backlog", "pick", "--max-episodes", "12", "--max-runtime-minutes", "24")
		if err != nil {
			t.Fatal(err)
		}
		if got := int(decodeObject(t, output)["media_id"].(float64)); got != 2 {
			t.Fatalf("picked %d, want page-two 2", got)
		}
	})

	ordered := []personalEntry{
		decodeEntry(mediaEntry(900, 0, 2, 10, "PLANNING", "RELEASING", 12, 24)),
		decodeEntry(mediaEntry(800, 0, 2, 20, "PLANNING", "RELEASING", 12, 24)),
		decodeEntry(mediaEntry(700, 0, 2, 20, "PLANNING", "RELEASING", 11, 24)),
		decodeEntry(mediaEntry(600, 0, 2, 20, "PLANNING", "RELEASING", 11, 23)),
		decodeEntry(mediaEntry(500, 0, 2, 20, "PLANNING", "RELEASING", 11, 23)),
	}
	gotOrder := rankBacklogCandidates(ordered, 20, 30)
	gotIDs := make([]int, len(gotOrder))
	for i := range gotOrder {
		gotIDs[i] = gotOrder[i].Media.ID
	}
	if want := []int{500, 600, 700, 800, 900}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("rank order = %v, want %v", gotIDs, want)
	}
}

func decodeEntry(value map[string]any) personalEntry {
	data, _ := json.Marshal(value)
	var entry personalEntry
	_ = json.Unmarshal(data, &entry)
	return entry
}

func TestProgressCatchUpMatrix(t *testing.T) {
	for _, asOf := range []string{"2026-01-15T12:00:00Z", "2026-01-15T12:00:01Z"} {
		t.Run(asOf, func(t *testing.T) {
			instant, _ := time.Parse(time.RFC3339, asOf)
			server := newCatchUpFixture(t, instant, false)
			defer server.Close()
			output, err := runPersonalWorkflow(t, server, "progress", "catch-up", "--as-of", asOf)
			if err != nil {
				t.Fatal(err)
			}
			got := decodeObjects(t, output)
			if asOf == "2026-01-15T12:00:00Z" && len(got) != 0 {
				t.Fatalf("before airing output = %s", output)
			}
			if asOf == "2026-01-15T12:00:01Z" && (len(got) != 1 || int(got[0]["highest_aired_episode"].(float64)) != 4) {
				t.Fatalf("after airing output = %s", output)
			}
		})
	}

	t.Run("all pages, highest aired, positive gaps, and no mutations", func(t *testing.T) {
		instant, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
		server := newCatchUpFixture(t, instant, true)
		defer server.Close()
		output, err := runPersonalWorkflow(t, server, "progress", "catch-up", "--as-of", instant.Format(time.RFC3339))
		if err != nil {
			t.Fatal(err)
		}
		got := decodeObjects(t, output)
		if len(got) != 1 || int(got[0]["media_id"].(float64)) != 2 || int(got[0]["highest_aired_episode"].(float64)) != 5 || int(got[0]["gap"].(float64)) != 3 {
			t.Fatalf("catch-up output = %s", output)
		}
	})

	server := newGraphQLFixture(t, func(graphQLRequest) any { t.Fatal("invalid as-of made HTTP request"); return nil })
	defer server.Close()
	if _, err := runPersonalWorkflow(t, server, "progress", "catch-up", "--as-of", "tomorrow"); err == nil {
		t.Fatal("invalid RFC3339 accepted")
	}
}

func newCatchUpFixture(t *testing.T, instant time.Time, multipage bool) *httptest.Server {
	t.Helper()
	return newGraphQLFixture(t, func(request graphQLRequest) any {
		if strings.Contains(request.Query, "mutation") {
			t.Fatal("catch-up issued mutation")
		}
		switch {
		case strings.Contains(request.Query, "Viewer"):
			return map[string]any{"Viewer": map[string]any{"id": 7}}
		case strings.Contains(request.Query, "mediaList(userId"):
			if multipage && intVariable(t, request, "page") == 1 {
				return gqlPage([]any{mediaEntry(1, 9, 0, 0, "CURRENT", "RELEASING", 12, 24)}, true)
			}
			if multipage {
				return gqlPage([]any{mediaEntry(2, 2, 0, 0, "CURRENT", "RELEASING", 12, 24)}, false)
			}
			return gqlPage([]any{mediaEntry(1, 3, 0, 0, "CURRENT", "RELEASING", 12, 24)}, false)
		case strings.Contains(request.Query, "airingSchedules"):
			if got := intVariable(t, request, "until"); got != int(instant.Unix()+1) {
				t.Fatalf("until = %d, want %d", got, instant.Unix()+1)
			}
			if multipage && intVariable(t, request, "page") == 1 {
				return map[string]any{"Page": map[string]any{"pageInfo": map[string]any{"hasNextPage": true}, "airingSchedules": []any{
					map[string]any{"episode": 4, "airingAt": instant.Add(-time.Hour).Unix(), "media": map[string]any{"id": 2}},
					map[string]any{"episode": 9, "airingAt": instant.Add(-time.Hour).Unix(), "media": map[string]any{"id": 1}},
				}}}
			}
			if multipage {
				return map[string]any{"Page": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "airingSchedules": []any{map[string]any{"episode": 5, "airingAt": instant.Unix(), "media": map[string]any{"id": 2}}, map[string]any{"episode": 6, "airingAt": instant.Add(time.Second).Unix(), "media": map[string]any{"id": 2}}}}}
			}
			airing := time.Date(2026, 1, 15, 12, 0, 1, 0, time.UTC)
			return map[string]any{"Page": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "airingSchedules": []any{map[string]any{"episode": 4, "airingAt": airing.Unix(), "media": map[string]any{"id": 1}}}}}
		default:
			t.Fatalf("unexpected query: %s", request.Query)
		}
		return nil
	})
}
