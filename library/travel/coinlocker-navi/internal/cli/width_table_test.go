// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisplayWidth(t *testing.T) {
	cases := map[string]int{"abc": 3, "東京駅": 6, "ｱ": 1, "Ｌ": 2, "06時00分～22時00分": 18, "": 0}
	for s, want := range cases {
		if got := displayWidth(s); got != want {
			t.Errorf("%q: width %d, want %d", s, got, want)
		}
	}
}

func TestWidthTableAlignsCJK(t *testing.T) {
	tw := &widthTable{}
	tw.add("ID", "NAME", "PAY")
	tw.add("1", "丸の内地下南口", "cash")
	tw.add("22", "east exit", "suica\tpasmo")
	var b bytes.Buffer
	if err := tw.write(&b); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	col := -1
	for _, l := range lines {
		i := strings.LastIndex(l, "  ")
		w := displayWidth(l[:i+2])
		if col >= 0 && w != col {
			t.Fatalf("PAY column starts at %d, want %d:\n%s", w, col, b.String())
		}
		col = w
	}
	if strings.Contains(b.String(), "\t") {
		t.Fatal("tabs must not reach the table")
	}
}

func TestTruncateWidth(t *testing.T) {
	if got := truncateWidth("東京駅八重洲口", 9); got != "東京駅..." || displayWidth(got) > 9 {
		t.Fatalf("got %q", got)
	}
	if got := truncateWidth("short", 9); got != "short" {
		t.Fatalf("got %q", got)
	}
}
