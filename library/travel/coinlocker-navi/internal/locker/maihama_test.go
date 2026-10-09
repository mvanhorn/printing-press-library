// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseMaihamaIndex(t *testing.T) {
	blocks, asOf := ParseMaihamaIndex(fixture(t, "maihama_index.html"))
	if len(blocks) != 6 || blocks[0].GID != "0001" {
		t.Fatalf("blocks = %+v", blocks)
	}
	if asOf == nil || *asOf != "2026-10-09T11:00:00+09:00" {
		t.Fatalf("as of = %v", asOf)
	}
	if *blocks[0].Status != "6_or_more_empty" || *blocks[2].Status != "none_empty" {
		t.Fatalf("status = %v %v", *blocks[0].Status, *blocks[2].Status)
	}
}

func TestParseMaihamaBlock(t *testing.T) {
	b := MaihamaBlock{GID: "0001"}
	if err := ParseMaihamaBlock(&b, fixture(t, "maihama_0001.html"), DefaultNaviBase); err != nil {
		t.Fatal(err)
	}
	if b.Name != "2F蘇我側 Bブロック" || *b.LockerID != "2669" || len(b.Sizes) != 3 {
		t.Fatalf("block = %+v", b)
	}
	s := b.Sizes[0]
	if *s.Label != "S" || *s.Installed != 186 || *s.Empty != 87 {
		t.Fatalf("small = %+v", s)
	}
	if err := ParseMaihamaBlock(&MaihamaBlock{GID: "x"}, []byte("<div></div>"), DefaultNaviBase); err == nil {
		t.Fatal("empty fragment must error")
	}
}

func TestClientMaihama(t *testing.T) {
	index, block := fixture(t, "maihama_index.html"), fixture(t, "maihama_0001.html")
	var ajax int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == MaihamaPath && r.URL.Query().Get("gid") == "":
			w.Write(index)
		case r.URL.Path == MaihamaPath+"ajax":
			ajax++
			if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				w.WriteHeader(400)
				return
			}
			w.Write(block)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	board, err := testClient(srv).Maihama(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ajax != 6 || len(board.Blocks) != 6 || board.SourceAsOf == nil || *board.Station.EN != "Maihama Station" {
		t.Fatalf("ajax=%d blocks=%d asOf=%v", ajax, len(board.Blocks), board.SourceAsOf)
	}
}

func TestClientMaihamaEmptyIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>終了しました</body></html>"))
	}))
	defer srv.Close()
	if _, err := testClient(srv).Maihama(context.Background()); err == nil || !strings.Contains(err.Error(), "no locker blocks") {
		t.Fatalf("want no-blocks error, got %v", err)
	}
}
