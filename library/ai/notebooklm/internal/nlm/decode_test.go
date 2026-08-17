// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStripXSSIPrefix(t *testing.T) {
	in := ")]}'\n\n16990\n[[\"wrb.fr\"]]"
	got := StripXSSIPrefix(in)
	if !strings.Contains(got, "16990") {
		t.Fatalf("expected chunked body, got %q", got)
	}
}

func TestParseFrames(t *testing.T) {
	body := ")]}'\n\n16990\n[[\"wrb.fr\",\"wXbhsf\",\"[[\\\"nb-1\\\",\\\"Title\\\"]]\",null,null,null,\"generic\"]]\n"
	frames, err := ParseFrames(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || frames[0].RPCID != RPCListNotebooks {
		t.Fatalf("unexpected frames: %+v", frames)
	}
}

func TestParseNotebookList(t *testing.T) {
	raw := json.RawMessage(`[[["Mamba SSMs",[],"5bb6b8d6-8c0b-4493-a081-891ce2d432db","🐍"]]]`)
	nbs, err := parseNotebookList(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(nbs) != 1 || nbs[0].ID != "5bb6b8d6-8c0b-4493-a081-891ce2d432db" || nbs[0].Title != "Mamba SSMs" || nbs[0].Emoji != "🐍" {
		t.Fatalf("unexpected notebooks: %+v", nbs)
	}
}
