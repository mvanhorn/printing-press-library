package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/mcp/cobratree"
	"github.com/spf13/cobra"
	"math"
	"testing"
)

func TestCarstayAgentEnvelopeAndProjection(t *testing.T) {
	for _, tc := range []struct {
		name         string
		agent        bool
		selectFields string
		wantMeta     bool
	}{{"json", false, "", true}, {"agent", true, "", true}, {"agent-projection", true, "results.id", true}, {"json-projection", false, "results.id", false}} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&b)
			cmd.Annotations = map[string]string{"pp:data-source": "live"}
			f := &rootFlags{asJSON: true, agent: tc.agent, compact: tc.agent, selectFields: tc.selectFields}
			v := carstayView{Meta: map[string]any{"source": "live", "availability": "unknown"}, Results: []map[string]any{{"id": "one", "name_ja": "名"}}, FetchFailures: []map[string]string{}}
			if e := carstayPrint(cmd, f, v); e != nil {
				t.Fatal(e)
			}
			var d map[string]json.RawMessage
			if e := json.Unmarshal(b.Bytes(), &d); e != nil {
				t.Fatal(e)
			}
			_, meta := d["meta"]
			if meta != tc.wantMeta {
				t.Fatalf("meta presence %v, output%s", meta, b.String())
			}
			var rows []map[string]any
			if e := json.Unmarshal(d["results"], &rows); e != nil || len(rows) != 1 {
				t.Fatalf("nested results: %s %v", b.String(), e)
			}
			if tc.selectFields != "" && len(rows[0]) != 1 {
				t.Fatal("projection did not narrow row")
			}
		})
	}
}
func TestCarstayEmptyCollectionStaysArray(t *testing.T) {
	for _, agent := range []bool{false, true} {
		var b bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&b)
		f := &rootFlags{asJSON: true, agent: agent}
		v := carstayView{Meta: map[string]any{"source": "live"}, Results: []map[string]any{}, FetchFailures: []map[string]string{}}
		if e := carstayPrint(cmd, f, v); e != nil {
			t.Fatal(e)
		}
		var d map[string]json.RawMessage
		_ = json.Unmarshal(b.Bytes(), &d)
		if string(d["results"]) != "[]" {
			t.Fatalf("not an array: %s", b.String())
		}
	}
}

func TestCarstayMCPRequiredInputContract(t *testing.T) {
	s := server.NewMCPServer("Carstay test", "0.1.0")
	cobratree.RegisterAll(s, RootCmd(), func() (string, error) { return "unused-cli", nil })
	tools := s.ListTools()
	for name, wanted := range map[string][]string{"spots_show": {"id"}, "spots_handoff": {"id"}, "spots_compare": {"first", "second"}, "spots_fit": {"id"}, "spots_audit": {"id"}, "spots_near": {"lat", "lon"}} {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("domain tool missing: %s", name)
		}
		b, err := json.Marshal(tool.Tool)
		if err != nil {
			t.Fatal(err)
		}
		var actual struct {
			InputSchema struct {
				Required   []string `json:"required"`
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
			} `json:"inputSchema"`
			Annotations struct {
				ReadOnly    bool `json:"readOnlyHint"`
				Destructive bool `json:"destructiveHint"`
			} `json:"annotations"`
		}
		if err = json.Unmarshal(b, &actual); err != nil {
			t.Fatal(err)
		}
		if !actual.Annotations.ReadOnly || actual.Annotations.Destructive {
			t.Fatalf("read classification: %s %s", name, b)
		}
		for _, key := range wanted {
			found := false
			for _, required := range actual.InputSchema.Required {
				if required == key {
					found = true
				}
			}
			if !found {
				t.Fatalf("required %s input absent on %s: %s", key, name, b)
			}
			if key != "lat" && key != "lon" && actual.InputSchema.Properties[key].Type != "string" {
				t.Fatalf("station ID must be one scalar input: %s", b)
			}
		}
	}
}

func TestNonfinitePublicRateIsUsageError(t *testing.T) {
	for _, rate := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		flags := &rootFlags{rateLimit: rate}
		cmd := newCarstayFindCmd(flags)
		if err := cmd.RunE(cmd, nil); err == nil || ExitCode(err) != 2 {
			t.Fatalf("expected pre-request usage error for %v, got %v", rate, err)
		}
	}
}

func TestCarstayMetadataOnlyAgentSelection(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	flags := &rootFlags{asJSON: true, agent: true, compact: true, selectFields: "meta.availability"}
	view := carstayView{Meta: map[string]any{"source": "live", "availability": "unknown"}, Results: []map[string]any{{"id": "one"}}, FetchFailures: []map[string]string{}}
	if err := carstayPrint(cmd, flags, view); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta    map[string]any `json:"meta"`
		Results []any          `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Results == nil || len(envelope.Results) != 0 || envelope.Meta["source"] != "live" || envelope.Meta["availability"] != "unknown" || envelope.Meta["results_omitted_by_select"] != true {
		t.Fatalf("unstable metadata-only envelope: %s", out.String())
	}
	if !flags.agent || flags.selectFields != "meta.availability" {
		t.Fatal("printing changed subsequent invocation flags")
	}
}
