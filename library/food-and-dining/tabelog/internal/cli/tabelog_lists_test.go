package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/domain"
)

// Exercise the cached-data path, bypassing fresh parser normalization on purpose.
func TestSavedAlternativesTreatLegacyPlaceholderCategoriesAsUnknown(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	flags := &rootFlags{agent: true, dataSource: "local", timeout: 5 * time.Second}
	ctx := context.Background()
	db, nb, err := tripOpen(ctx, flags)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []domain.Restaurant{
		{ID: "13005012", Name: "Anchor", URL: "https://tabelog.com/en/tokyo/A1301/A130101/13005012/", Categories: []string{"Bar"}, Area: domain.Area{Prefecture: "tokyo", Area1: "A1301", Area2: "A130101", Verified: true}, FetchedAt: time.Now(), Surface: "listing", Evidence: map[string]string{"categories": "known"}},
		{ID: "13120361", Name: "Legacy unknown", URL: "https://tabelog.com/en/tokyo/A1301/A130101/13120361/", Categories: []string{" - "}, Area: domain.Area{Prefecture: "tokyo", Area1: "A1301", Area2: "A130101", Verified: true}, FetchedAt: time.Now(), Surface: "listing", Evidence: map[string]string{"categories": "known"}},
	} {
		if err := tripReplace(ctx, nb, r); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := nb.Add(ctx, "backups", r.ID, "retained note", true); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := newTabelogListAlternativesCmd(flags)
	cmd.SetContext(ctx)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"backups", "--for", "13005012", "--match", "area,category"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cached alternatives: %v; %s", err, stderr.String())
	}
	var output struct {
		Items []map[string]any `json:"items"`
		Meta  domain.Meta      `json:"meta"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Items) != 0 || output.Meta.Criteria["unmatched"] != float64(0) || output.Meta.Criteria["unevaluable"] != float64(1) {
		t.Fatalf("legacy placeholder was treated as a factual mismatch: %s", stdout.String())
	}
}
