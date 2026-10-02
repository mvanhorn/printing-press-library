package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/jreast"
)

func TestJRInputValidationBeforeNetwork(t *testing.T) {
	cases := []struct {
		name string
		ctor func(*rootFlags) *cobra.Command
		args []string
	}{
		{"status missing line", newNovelStatusCmd, nil},
		{"status invalid cap", newNovelStatusCmd, []string{"--line=sobuline", "--limit=0"}},
		{"status region conflict", newNovelStatusCmd, []string{"--line=tohoku:tazawakoline", "--region=kanto"}},
		{"status invalid age", newNovelStatusCmd, []string{"--line=sobuline", "--max-source-age=0s"}},
		{"impact missing input", newNovelImpactCmd, nil},
		{"impact invalid component", newNovelImpactCmd, []string{"--lines=yamanoteline"}},
		{"impact too many", newNovelImpactCmd, []string{"--lines=kanto:yamanoteline,kanto:yamanoteline,kanto:yamanoteline,kanto:yamanoteline,kanto:yamanoteline,kanto:yamanoteline,kanto:yamanoteline,kanto:yamanoteline,kanto:yamanoteline"}},
		{"lines invalid region", newJRLinesCmd, []string{"--region=unknown"}},
		{"certificates invalid slot", newNovelCertificatesCmd, []string{"--line=yamanoteline", "--slot=06"}},
		{"certificates slot needs line", newNovelCertificatesCmd, []string{"--slot=02"}},
		{"through certificate slot unavailable", newNovelCertificatesCmd, []string{"--line=ueno-tokyoline", "--slot=02"}},
		{"Sagami certificate slot unavailable", newNovelCertificatesCmd, []string{"--line=sagamiline", "--slot=02"}},
		{"prefixed route certificate slot unavailable", newNovelCertificatesCmd, []string{"--line=kanto:shonan-shinjukuline", "--slot=02"}},
		{"planned missing line", newNovelPlannedCmd, nil},
		{"coverage invalid timestamp", newNovelCoverageCmd, []string{"--at=2026-10-03"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &rootFlags{agent: true, noInput: true}
			cmd := tc.ctor(f)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			if e := cmd.Flags().Parse(tc.args); e != nil {
				t.Fatal(e)
			}
			e := cmd.RunE(cmd, nil)
			var typed *cliError
			if !errors.As(e, &typed) || typed.code != 2 {
				t.Fatalf("expected input error 2, got %v", e)
			}
		})
	}
}

type jrFixtureTransport func(*http.Request) (*http.Response, error)

func (f jrFixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestJRClosedExpressCommandsDoNotFetchDetails(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, ctor := range []func(*rootFlags) *cobra.Command{newNovelStatusCmd, newNovelImpactCmd} {
		requests := 0
		http.DefaultTransport = jrFixtureTransport(func(req *http.Request) (*http.Response, error) {
			requests++
			body := `<p>情報提供時間は4:00～翌2:00となっています。</p>`
			status := 200
			if req.URL.Path != "/train_info/chyokyori.aspx" {
				status, body = 403, "Access Denied"
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})
		flags := &rootFlags{agent: true, asJSON: true, compact: true, noInput: true}
		cmd := ctor(flags)
		cmd.SetContext(context.Background())
		name := "line"
		if cmd.Name() == "impact" {
			name = "lines"
		}
		if e := cmd.Flags().Parse([]string{"--" + name + "=express:wakashio_sazamani"}); e != nil {
			t.Fatal(e)
		}
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		if e := cmd.RunE(cmd, nil); e != nil {
			t.Fatal(cmd.Name(), e, errOut.String())
		}
		var value jreast.Envelope[jreast.Line]
		if e := json.Unmarshal(out.Bytes(), &value); e != nil {
			t.Fatal(e)
		}
		if requests != 1 || value.Meta.RequestCount != 1 || len(value.Results) != 1 || value.Results[0].Assessment != "outside_reporting_hours" || len(value.Meta.FetchFailures) != 0 {
			t.Fatal(cmd.Name(), requests, out.String(), errOut.String())
		}
	}
}

func TestJRAgentKeepsSparseDisruptionFacts(t *testing.T) {
	var b bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&b)
	f := &rootFlags{agent: true, asJSON: true, compact: true, noInput: true}
	input := jreast.Envelope[jreast.Line]{Results: []jreast.Line{
		{ID: "kanto:sobuline", NoticeFactCount: 1, Notices: []jreast.Notice{{Direction: "both", Cause: "大雨", Sections: []jreast.Section{{From: "八街", To: "成東", Language: "ja"}}}}},
		{ID: "kanto:yamanoteline", Assessment: "normal_label_only"},
		{ID: "kanto:not-a-real-line", Assessment: "line_not_found"},
	}}
	if e := f.printJSON(cmd, input); e != nil {
		t.Fatal(e)
	}
	var output struct {
		Results []jreast.Line `json:"results"`
	}
	if e := json.Unmarshal(b.Bytes(), &output); e != nil {
		t.Fatal(e)
	}
	if len(output.Results) != 3 || len(output.Results[0].Notices) != 1 {
		t.Fatalf("sparse disruption notices were lost: %s", b.String())
	}
	n := output.Results[0].Notices[0]
	if n.Direction != "both" || n.Cause != "大雨" || len(n.Sections) != 1 || n.Sections[0].From != "八街" {
		t.Fatalf("disruption facts were lost: %+v", n)
	}
}

func TestJRSourceClosedStateDoesNotDependOnLocalClock(t *testing.T) {
	for _, tc := range []struct {
		states []jreast.SourceState
		want   bool
	}{
		{nil, false},
		{[]jreast.SourceState{{ReportingState: "open"}}, false},
		{[]jreast.SourceState{{ReportingState: "open"}, {ReportingState: "outside_reporting_hours"}}, true},
	} {
		if got := jrSnapshotClosed(jreast.Snapshot{Sources: tc.states}); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestJRCommandsStructuredDryRunAndSourceAnnotations(t *testing.T) {
	for _, ctor := range []func(*rootFlags) *cobra.Command{newJRAreasCmd, newJRLinesCmd, newNovelStatusCmd, newNovelImpactCmd, newNovelPlannedCmd, newNovelCertificatesCmd, newNovelCoverageCmd} {
		f := &rootFlags{dryRun: true, asJSON: true, noInput: true}
		cmd := ctor(f)
		var b bytes.Buffer
		cmd.SetOut(&b)
		cmd.SetErr(&bytes.Buffer{})
		if e := cmd.RunE(cmd, nil); e != nil {
			t.Fatal(cmd.Name(), e)
		}
		var j map[string]any
		if e := json.Unmarshal(b.Bytes(), &j); e != nil || j["dry_run"] != true {
			t.Fatalf("%s dry-run = %q, %v", cmd.Name(), b.String(), e)
		}
		if cmd.Annotations["mcp:read-only"] != "true" || cmd.Annotations["pp:data-source"] == "auto" || cmd.Annotations["pp:happy-args"] == "" {
			t.Fatal(cmd.Name(), cmd.Annotations)
		}
		if strings.Contains(cmd.Annotations["pp:happy-args"], "--agent=true") {
			t.Fatal("happy fixture must not render a boolean as a positional", cmd.Name())
		}
	}
}

func TestJRLiveCommandsRejectLocalCache(t *testing.T) {
	for _, ctor := range []func(*rootFlags) *cobra.Command{newJRAreasCmd, newJRLinesCmd, newNovelStatusCmd, newNovelImpactCmd, newNovelPlannedCmd, newNovelCertificatesCmd} {
		f := &rootFlags{dataSource: "local", agent: true}
		cmd := ctor(f)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		var e *cliError
		if err := cmd.RunE(cmd, nil); !errors.As(err, &e) || e.code != 2 {
			t.Fatal(cmd.Name(), err)
		}
	}
}
