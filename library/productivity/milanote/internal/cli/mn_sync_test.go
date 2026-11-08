// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func itemsFor(chunks []milanote.Chunk) []pairItem {
	out := make([]pairItem, len(chunks))
	for i, c := range chunks {
		out[i] = pairItem{ID: string(rune('a' + i)), Kind: c.Kind, Hash: milanote.HashText(c.Normalized())}
	}
	return out
}

func hashesFor(chunks []milanote.Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = milanote.HashText(c.Normalized())
	}
	return out
}

func TestMatchChunksKeepsCardsAcrossEdits(t *testing.T) {
	before := milanote.ChunkMarkdown("## One\n\nfirst note\n\n## Two\n\nsecond note\n")
	items := itemsFor(before)
	after := milanote.ChunkMarkdown("## One\n\nfirst note, edited\n\n## Two\n\nsecond note\n\n## Three\n\nbrand new\n")
	got, used := matchChunks(after, hashesFor(after), items)
	want := []int{0, 1, 2, 3, -1, -1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chunk %d paired with %d, want %d (all %v)", i, got[i], want[i], got)
		}
	}
	for j, u := range used {
		if !u {
			t.Errorf("item %d left unused", j)
		}
	}
}

func TestMatchChunksFollowsMoves(t *testing.T) {
	before := milanote.ChunkMarkdown("alpha\n\n---\n\nbeta\n\n---\n\ngamma\n")
	items := itemsFor(before)
	after := milanote.ChunkMarkdown("gamma\n\n---\n\nalpha\n")
	got, used := matchChunks(after, hashesFor(after), items)
	if got[0] != 2 || got[1] != 0 {
		t.Fatalf("pairs = %v", got)
	}
	if used[1] {
		t.Fatal("beta left the file, so its card should be unpaired")
	}
}

func TestApplyLineEdits(t *testing.T) {
	text := "a\n\nb\n\nc\n"
	repl := "B1\nB2"
	got := applyLineEdits(text, []lineEdit{{start: 2, end: 3, text: &repl}, {start: 4, end: 5}}, []string{"tail"})
	if want := "a\n\nB1\nB2\n\n---\n\ntail\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNaturalLess(t *testing.T) {
	if !naturalLess("frame2.png", "frame10.png") || naturalLess("frame10.png", "frame2.png") {
		t.Fatal("numbers should sort by value")
	}
	if !naturalLess("a.png", "B.png") {
		t.Fatal("case should not matter")
	}
}

func TestCSVCards(t *testing.T) {
	docs, titles, err := csvCards([]byte("Shot,Lens,Notes\nWide,24mm,\nClose,85mm,hands\n"), "shot", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || titles[0] != "01 Wide" || titles[1] != "02 Close" {
		t.Fatalf("titles = %v", titles)
	}
	if md := milanote.ToMarkdown(docs[1]); md != "## 02 Close\n\n**Lens:** 85mm\n\n**Notes:** hands" {
		t.Fatalf("md = %q", md)
	}
	if _, _, err := csvCards([]byte("a,b\n1,2\n"), "missing", false); err == nil {
		t.Fatal("unknown title column should fail")
	}
}

func TestDiffSnapshots(t *testing.T) {
	mk := func(x float64, text string, checked bool) *boardSnapshot {
		return &boardSnapshot{Board: "b", Elements: map[string]*milanote.Element{
			"n": {ID: "n", ElementType: milanote.TypeCard, Location: milanote.CanvasAt("b", x, 2, 1), Content: milanote.NoteContent(milanote.MarkdownDoc(text), "")},
			"t": {ID: "t", ElementType: milanote.TypeTaskList, Location: milanote.CanvasAt("b", 40, 2, 2), Content: milanote.TaskListContent("", []milanote.Task{{ID: "k", Text: "shoot", Checked: checked}})},
		}}
	}
	got := map[string]bool{}
	for _, ch := range diffSnapshots(mk(2, "old", false), mk(9, "new", true)) {
		got[ch.Kind] = true
	}
	for _, k := range []string{"moved", "edited", "ticked"} {
		if !got[k] {
			t.Errorf("missing %s in %v", k, got)
		}
	}
}

func TestTreeOrder(t *testing.T) {
	got := treeOrder([]boardRow{
		{ID: "home", Depth: 0},
		{ID: "a", Parent: "home", Depth: 1},
		{ID: "b", Parent: "home", Depth: 1},
		{ID: "b1", Parent: "b", Depth: 2},
		{ID: "a1", Parent: "a", Depth: 2},
	})
	want := []string{"home", "a", "a1", "b", "b1"}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("position %d = %s, want %s", i, got[i].ID, id)
		}
	}
}

func TestSessionCookieFromChromePicksOnlyMilanote(t *testing.T) {
	cases := []struct {
		name    string
		cookies string
		want    string
	}{
		{"found", `[{"name":"sid","value":"other","domain":".example.com"},{"name":"` + milanote.SessionCookie + `","value":"tok","domain":".milanote.com"}]`, "tok"},
		{"same name elsewhere", `[{"name":"` + milanote.SessionCookie + `","value":"nope","domain":".evil.example"}]`, ""},
		{"logged out", `[]`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := websocket.Upgrader{}
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/json/version" {
					fmt.Fprintf(w, `{"webSocketDebuggerUrl":"ws%s/devtools/browser/x"}`, strings.TrimPrefix(srv.URL, "http"))
					return
				}
				conn, err := up.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"id":1,"result":{"cookies":`+tc.cookies+`}}`))
			}))
			defer srv.Close()
			u, _ := url.Parse(srv.URL)
			port, _ := strconv.Atoi(u.Port())
			got, err := sessionCookieFromChrome(port)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q err %v", got, err)
			}
		})
	}
}
