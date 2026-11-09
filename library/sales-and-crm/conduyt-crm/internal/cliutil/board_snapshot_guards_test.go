package cliutil

import (
	"fmt"
	"strings"
	"testing"
)

const (
	gS1 = "44444444-4444-4444-8444-444444444441"
	gS2 = "44444444-4444-4444-8444-444444444442"
)

func gStages(n int, suffix string) string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("44444444-4444-4444-8444-%012x%s", i, suffix)
	}
	return strings.Join(ids, ",")
}

func TestParseBoardColumnsAcceptsAndNormalizes(t *testing.T) {
	for in, want := range map[string]string{
		gS1:                               gS1,
		gS1 + ":25," + gS2:                gS1 + ":25," + gS2,
		" " + gS1 + " , " + gS2 + ":007 ": gS1 + "," + gS2 + ":7",
		gS1 + ":1000," + gS2 + ":1":       gS1 + ":1000," + gS2 + ":1",
		gStages(50, ""):                   gStages(50, ""),
	} {
		got, err := ParseBoardColumns(in, 50)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestParseBoardColumnsRefusesWhatTheServerRefuses(t *testing.T) {
	for in, want := range map[string]string{
		"":                               "no blank entry",
		gS1 + ",":                        "no blank entry",
		"nope":                           "stage UUID",
		gS1 + ":5:6":                     "stage UUID",
		gS1 + ":0":                       "1-1000",
		gS1 + ":1001":                    "1-1000",
		gS1 + ":-1":                      "1-1000",
		gS1 + ":+5":                      "1-1000",
		gS1 + ":2.5":                     "1-1000",
		gS1 + ":":                        "1-1000",
		gS1 + "," + strings.ToUpper(gS1): "more than once",
		gStages(51, ""):                  "at most 50 columns",
		gStages(6, ":1000"):              "6000 rows",
	} {
		if _, err := ParseBoardColumns(in, 50); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", in, err, want)
		}
	}
	if _, err := ParseBoardColumns(gStages(50, ""), 101); err == nil || !strings.Contains(err.Error(), "5050 rows") {
		t.Errorf("the default limit counts toward the 5000: err = %v", err)
	}
}

func TestCheckBoardSnapshotParams(t *testing.T) {
	ok := func() map[string]any {
		return map[string]any{"board_snapshot": true, "pipeline": gS2, "board_columns": gS1}
	}
	if err := CheckBoardSnapshotParams(map[string]any{"status": "open"}); err != nil {
		t.Errorf("no snapshot params: %v", err)
	}
	if err := CheckBoardSnapshotParams(map[string]any{"board_snapshot": false, "page": float64(2)}); err != nil {
		t.Errorf("board_snapshot=false is the plain list: %v", err)
	}
	if err := CheckBoardSnapshotParams(ok()); err != nil {
		t.Errorf("valid: %v", err)
	}
	for _, alias := range []string{"pipeline_id", "pipelineId", "pipeline_name", "pipelineName"} {
		p := ok()
		delete(p, "pipeline")
		p[alias] = "x"
		if err := CheckBoardSnapshotParams(p); err != nil {
			t.Errorf("pipeline alias %s: %v", alias, err)
		}
	}
	// P3 (Codex r1): an alias counts only as a non-blank string; at least one must be.
	for _, alias := range boardSnapshotPipelineKeys {
		for name, bad := range map[string]any{"empty": "", "blank": "  ", "number": float64(5), "bool": true, "nil": nil, "object": map[string]any{}} {
			p := ok()
			delete(p, "pipeline")
			p[alias] = bad
			if err := CheckBoardSnapshotParams(p); err == nil || !strings.Contains(err.Error(), "needs pipeline") {
				t.Errorf("%s alias %s: err = %v, want needs pipeline", name, alias, err)
			}
		}
		p := ok()
		if alias == "pipeline" {
			delete(p, "pipeline")
			p["pipeline_id"] = gS2
		}
		p[alias] = " "
		if alias == "pipeline" {
			p["pipeline_id"] = gS2
		}
		if err := CheckBoardSnapshotParams(p); err != nil {
			t.Errorf("a blank %s beside a real pipeline: %v", alias, err)
		}
	}
	// per_page up to 1000 is the default column limit (Codex r1 P2): 201 and 1000 pass, 1000 x 5 = 5000 passes.
	for _, n := range []float64{201, 1000} {
		p := ok()
		p["per_page"] = n
		if err := CheckBoardSnapshotParams(p); err != nil {
			t.Errorf("per_page %v: %v", n, err)
		}
	}
	for _, k := range boardSnapshotConflicts {
		p := ok()
		p[k] = float64(1)
		if err := CheckBoardSnapshotParams(p); err == nil || !strings.Contains(err.Error(), "cannot be combined with "+k) {
			t.Errorf("conflict %s: err = %v", k, err)
		}
	}
	for name, tc := range map[string]struct {
		mutate func(map[string]any)
		want   string
	}{
		"string true":      {func(p map[string]any) { p["board_snapshot"] = "true" }, "JSON boolean"},
		"no pipeline":      {func(p map[string]any) { delete(p, "pipeline") }, "needs pipeline"},
		"no columns":       {func(p map[string]any) { delete(p, "board_columns") }, "needs board_columns"},
		"columns array":    {func(p map[string]any) { p["board_columns"] = []any{gS1} }, "must be a string"},
		"bad columns":      {func(p map[string]any) { p["board_columns"] = "nope" }, "stage UUID"},
		"columns, no mode": {func(p map[string]any) { delete(p, "board_snapshot") }, "needs board_snapshot=true"},
		"columns, false":   {func(p map[string]any) { p["board_snapshot"] = false }, "needs board_snapshot=true"},
		"per_page 1001":    {func(p map[string]any) { p["per_page"] = float64(1001) }, "per_page"},
		"per_page default": {func(p map[string]any) { p["per_page"] = float64(101); p["board_columns"] = gStages(50, "") }, "5050 rows"},
	} {
		p := ok()
		tc.mutate(p)
		if err := CheckBoardSnapshotParams(p); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}
