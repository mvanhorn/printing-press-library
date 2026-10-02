// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"strings"
	"testing"
)

func TestJapaneseLCBExceptionIsScopedAndParsed(t *testing.T) {
	rows := JapaneseBaggage(`<p>※渋谷～成田空港間のLCBバス(Low Cost Bus)線のお預かり可能な手荷物は、1名様につき、1個までとなります。</p><p>一般 2個</p>`)
	if len(rows) != 1 || rows[0].Value != 1 || rows[0].RouteIDs[0] != "Narita-ShibuyaLCB" || !strings.Contains(rows[0].Scope, "overrides") {
		t.Fatalf("%+v", rows)
	}
	rows = JapaneseBaggage(`<p>exception wording changed</p>`)
	if rows[0].Value != nil || !strings.Contains(rows[0].Summary, "unknown") {
		t.Fatalf("%+v", rows)
	}
}
