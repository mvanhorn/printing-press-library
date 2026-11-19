package cli

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestMarketReportCompositeNoLeak(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
	snap, err := mktGetSnapshot(db.DB(), id)
	if err != nil {
		t.Fatal(err)
	}
	snap.Progress.ReviewsCapped = []int64{1000000001}
	report := mktSummarize(db.DB(), &mktRun{snap: snap, unresolvedShops: 1}, "partial")
	want := []string{
		"Review collection capped at 1000 reviews per listing.",
		"1 shop(s) could not be resolved from /listings/batch and were skipped; resume the snapshot to retry.",
		fmt.Sprintf("etsy-pp-cli market snapshot --resume %d", id),
	}
	if report["note"] != strings.Join(want[:2], "; ") {
		t.Fatalf("SUMMARY_NOTE_SEPARATOR: %v", report["note"])
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := mktEmitReport(cmd, &rootFlags{}, report); err != nil {
		t.Fatal(err)
	}
	m := mktDecode(t, out.String())
	if !reflect.DeepEqual(anyNotesStrings(m), want) {
		t.Fatalf("COMPOSITE_NOTES_EXACT: %s", out.String())
	}
	if m["note"] != strings.Join(want, "; ") {
		t.Fatalf("COMPOSITE_NOTE_JOIN: %s", out.String())
	}
	mktCheckNotesInvariant(t, m)
}
