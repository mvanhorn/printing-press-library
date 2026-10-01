package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/jma"
	"github.com/spf13/cobra"
	"testing"
)

func TestJMAProjectionPreservesProvenance(t *testing.T) {
	c := &cobra.Command{}
	var out bytes.Buffer
	c.SetOut(&out)
	v := jma.Envelope{Meta: jma.Meta{Coverage: "partial"}, Results: map[string]any{"areas": []any{map[string]any{"id": "1", "name": "Tokyo"}, map[string]any{"id": "2", "name": "Osaka"}}}}
	if e := emitJMA(c, v, "areas.id"); e != nil {
		t.Fatal(e)
	}
	var o map[string]any
	json.Unmarshal(out.Bytes(), &o)
	if o["meta"].(map[string]any)["coverage"] != "partial" {
		t.Fatal(o)
	}
	a := o["results"].(map[string]any)["areas"].([]any)
	if len(a[0].(map[string]any)) != 1 {
		t.Fatal(a)
	}
	out.Reset()
	if e := emitJMA(c, v, "areas.bogus"); e == nil || out.Len() != 0 {
		t.Fatal("invalid projection must fail before stdout")
	}
}

func TestJMAEmptyListProjectionKeepsEmptyResults(t *testing.T) {
	root := &cobra.Command{Use: "jma-pp-cli"}
	parent := &cobra.Command{Use: "typhoons"}
	c := &cobra.Command{Use: "list"}
	root.AddCommand(parent)
	parent.AddCommand(c)
	var out bytes.Buffer
	c.SetOut(&out)
	v := jma.Envelope{Meta: jma.Meta{Coverage: "active index"}, Results: []any{}}
	if e := emitJMA(c, v, "results.id,results.issued_at"); e != nil {
		t.Fatal(e)
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	if len(got["results"].([]any)) != 0 {
		t.Fatal(got)
	}
	out.Reset()
	if e := emitJMA(c, v, "results.bogus"); e == nil || out.Len() != 0 {
		t.Fatal("unknown empty-list projection accepted")
	}
}

func TestJMAEmptyWarningDetailProjection(t *testing.T) {
	root := &cobra.Command{Use: "jma-pp-cli"}
	root.PersistentFlags().Bool("detail", false, "")
	parent := &cobra.Command{Use: "warnings"}
	c := &cobra.Command{Use: "get"}
	root.AddCommand(parent)
	parent.AddCommand(c)
	root.PersistentFlags().Set("detail", "true")
	var out bytes.Buffer
	c.SetOut(&out)
	v := jma.Envelope{Results: map[string]any{"municipalities": []any{map[string]any{"events": []any{}}}}}
	if e := emitJMA(c, v, "municipalities.events.source_properties,municipalities.events.additions_ja"); e != nil {
		t.Fatal(e)
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	events := got["results"].(map[string]any)["municipalities"].([]any)[0].(map[string]any)["events"].([]any)
	if len(events) != 0 {
		t.Fatal(got)
	}
}
