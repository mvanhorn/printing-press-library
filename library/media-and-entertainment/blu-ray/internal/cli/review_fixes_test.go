package cli

import (
	"strings"
	"testing"
)

func TestFTSQueryStripsSyntax(t *testing.T) {
	got := ftsQuery(`Title:foo "bar" baz) +qux -zap *`)
	want := "Titlefoo* bar* baz* qux* zap*"
	if got != want {
		t.Fatalf("ftsQuery = %q, want %q", got, want)
	}
}

func TestSparkPlotUsesPointWidthForSmallSamples(t *testing.T) {
	got := sparkPlot([]historyRow{{Price: 10}, {Price: 20}, {Price: 30}})
	if cells := len([]rune(got)); cells != 3 {
		t.Fatalf("sparkPlot cells = %d, want 3 (%q)", cells, got)
	}
}

func TestDecodePermissiveXMLJoinsRetryError(t *testing.T) {
	var out struct {
		Value string `xml:"value"`
	}
	err := decodePermissiveXML([]byte("<root>\x00</root>"), &out)
	if err == nil {
		t.Fatal("decodePermissiveXML succeeded, want error")
	}
	parts := strings.Split(err.Error(), "\n")
	if len(parts) < 2 {
		t.Fatalf("joined error should expose both decode failures, got %q", err.Error())
	}
	if unwrapper, ok := err.(interface{ Unwrap() []error }); !ok || len(unwrapper.Unwrap()) != 2 {
		t.Fatalf("decodePermissiveXML error should join two causes, got %#v", err)
	}
}
