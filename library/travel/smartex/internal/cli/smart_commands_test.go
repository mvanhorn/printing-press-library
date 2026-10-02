package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/cliutil/testenv"
	"testing"
)

func TestSmartCommandSelectionAndUsage(t *testing.T) {
	testenv.Isolate(t)
	for _, tt := range []struct {
		args  []string
		code  int
		check func(map[string]any)
	}{
		{[]string{"window", "--date", "2026-10-31", "--now", "2026-10-01T09:00:00+09:00", "--json", "--select", "standard_seat_sales_open_jst,train_and_seat_confirmation_jst"}, 0, func(v map[string]any) {
			if len(v) != 2 || v["standard_seat_sales_open_jst"] != "2026-10-01T10:00:00+09:00" || v["train_and_seat_confirmation_jst"] != nil {
				t.Fatalf("narrow window:%+v", v)
			}
		}},
		{[]string{"fare", "--from", "Tokyo", "--to", "Shin-Osaka", "--data-source", "local"}, 2, nil},
		{[]string{"stations", "--data-source", "live"}, 2, nil},
		{[]string{"timetable", "--from", "Tokyo", "--to", "Shin-Osaka", "--date", "2026-02-30", "--offline", "--agent"}, 2, nil},
		{[]string{"timetable", "--from", "Tokyo", "--to", "Shin-Osaka", "--offline", "--agent"}, 0, checkOfflineTimetable},
		{[]string{"timetable", "--from", "Tokyo", "--to", "Shin-Osaka", "--data-source", "local", "--agent"}, 0, checkOfflineTimetable},
		{[]string{"baggage", "--length-cm", "NaN", "--width-cm", "20", "--height-cm", "20", "--weight-kg", "10"}, 2, nil},
		{[]string{"stations", "--query", "xxx-invalid", "--json"}, 0, func(v map[string]any) {
			if len(v["stations"].([]any)) != 0 {
				t.Fatal("negative station filter leaked matches")
			}
		}},
		{[]string{"fare", "--dry-run", "--agent"}, 0, func(v map[string]any) {
			if len(v) == 0 {
				t.Fatal("empty dryrun envelope")
			}
		}},
	} {
		root := RootCmd()
		var out, stderr bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&stderr)
		root.SetArgs(tt.args)
		err := root.Execute()
		code := 0
		if err != nil {
			var ce *cliError
			if errors.As(err, &ce) {
				code = ce.code
			} else {
				code = 1
			}
		}
		if code != tt.code {
			t.Fatalf("args%v code%d want%d error%v stderr%s", tt.args, code, tt.code, err, stderr.String())
		}
		if tt.check != nil {
			var v map[string]any
			if e := json.Unmarshal(out.Bytes(), &v); e != nil {
				t.Fatalf("args%v invalidJSON:%s %v", tt.args, out.String(), e)
			}
			tt.check(v)
		}
	}
}

func checkOfflineTimetable(v map[string]any) {
	if v["meta"].(map[string]any)["source"] != "local" {
		panic("offline timetable provenance must be local")
	}
	data := v["results"].(map[string]any)
	if data["snapshot_matches_current_pdf_links"] != nil || data["upstream_requests"] != float64(0) {
		panic("offline comparison must stay unknown without HTTP")
	}
}
