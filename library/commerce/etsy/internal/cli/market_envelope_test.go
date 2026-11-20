package cli

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMarketEnvelopeEverySubcommand(t *testing.T) {
	check := func(t *testing.T, args ...string) map[string]any {
		t.Helper()
		out, err := mktRunCLI(t, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		m := mktDecode(t, out)
		if _, ok := m["results"]; !ok {
			t.Fatalf("missing results: %s", out)
		}
		notes, ok := m["notes"].([]any)
		if !ok {
			t.Fatalf("notes must be an array: %s", out)
		}
		seen := map[any]bool{}
		for _, n := range notes {
			if seen[n] {
				t.Fatalf("duplicate note: %v", n)
			}
			seen[n] = true
		}
		mktCheckNotesInvariant(t, m)
		return m
	}
	for _, state := range []string{"empty", "empty_snapshot", "populated"} {
		for _, command := range []string{"demand", "gaps", "price", "shops", "tags", "trend"} {
			t.Run(state+"_"+command, func(t *testing.T) {
				mktTestEnv(t)
				if state != "empty" {
					db := mktOpenFixtureStore(t)
					now := time.Now()
					for j := 0; j < 2; j++ {
						snap := mktFxSnapshot(t, db.DB(), now.Add(time.Duration(j-1)*time.Hour), []string{"linen apron"}, 1, "complete", mktFxProg{})
						if state == "populated" {
							for i := 0; i < 12; i++ {
								id := int64(1000000001 + i)
								shop := int64(9000001)
								if i%2 == 1 {
									shop = 555001
								}
								mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: i + 1, id: id, shop: shop, title: "linen apron", price: 2400, tags: []string{"linen apron"}})
								mktFxReviews(t, db.DB(), snap, id, i%4+1, now)
							}
							for _, shop := range []int64{9000001, 555001} {
								mktFxShop(t, db.DB(), snap, shop, 100, 20, now.Add(-time.Hour*24*400).Unix())
							}
						}
					}
					db.Close()
				}
				args := []string{"market", command}
				if command == "gaps" {
					rules := filepath.Join(t.TempDir(), "rules.json")
					if err := os.WriteFile(rules, []byte(`{"targets":[{"name":"linen apron","include":["linen apron"],"exclude":[]}]}`), 0600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--rules", rules)
				}
				m := check(t, args...)
				if state == "empty" && (len(m["notes"].([]any)) != 1 || !mktNotesContain(m, mktNoSnapshotHint)) {
					t.Fatalf("NOTE_NOT_IN_NOTES: empty hint: %v", m)
				}
				agentOut, agentErr := mktRunCLI(t, append(args, "--agent")...)
				if agentErr != nil {
					t.Fatalf("AGENT_ENVELOPE_PATH_BROKEN: %v", agentErr)
				}
				outer := mktDecode(t, agentOut)
				envelope, ok := outer["results"].(map[string]any)
				if !ok || outer["meta"] == nil {
					t.Fatalf("AGENT_ENVELOPE_PATH_BROKEN: %s", agentOut)
				}
				if _, ok := envelope["notes"].([]any); !ok {
					t.Fatalf("AGENT_ENVELOPE_PATH_BROKEN: results.notes: %s", agentOut)
				}
				if state == "populated" {
					if command == "tags" || command == "trend" {
						payload, ok := envelope["results"].(map[string]any)
						key := "tags"
						if command == "trend" {
							key = "common_terms"
						}
						if !ok || payload[key] == nil {
							t.Fatalf("AGENT_ENVELOPE_PATH_BROKEN: results.results.%s: %s", key, agentOut)
						}
					} else if rows, ok := envelope["results"].([]any); !ok || len(rows) == 0 {
						t.Fatalf("AGENT_ENVELOPE_PATH_BROKEN: results.results rows: %s", agentOut)
					}
				}

				check(t, append(args, "--dry-run")...)
				if state == "populated" && (command == "tags" || command == "trend") {
					r, ok := m["results"].(map[string]any)
					if !ok {
						t.Fatalf("object results required: %v", m)
					}
					keys := []string{"tags", "title_words", "basis", "min_support", "top_group_size", "rest_group_size"}
					if command == "trend" {
						keys = []string{"common_terms", "new_listings", "exited_listings", "position_moves", "price_changes", "favorites_deltas", "new_shops", "exited_shops", "skipped_terms"}
					}
					for _, k := range keys {
						if _, ok := r[k]; !ok {
							t.Errorf("missing results.%s", k)
						}
						if _, ok := m[k]; ok {
							t.Errorf("unexpected top-level %s", k)
						}
						if command == "tags" && (k == "tags" || k == "title_words") {
							if _, ok := r[k].([]any); !ok {
								t.Errorf("results.%s must be array", k)
							}
						}
					}
				}
				check(t, append(args, "--select", "results")...)
			})
		}
	}
	t.Run("summary_error", func(t *testing.T) {
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
		report["error"] = "synthetic snapshot failure"
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		flags := &rootFlags{selectFields: "results"}
		if err := mktEmitReport(cmd, flags, report); err != nil {
			t.Fatal(err)
		}
		m := mktDecode(t, out.String())
		notes := m["notes"].([]any)
		for _, want := range []any{"synthetic snapshot failure", "Review collection capped at 1000 reviews per listing.", "1 shop(s) could not be resolved from /listings/batch and were skipped; resume the snapshot to retry."} {
			found := false
			for _, n := range notes {
				if n == want {
					found = true
				}
			}
			if !found {
				t.Errorf("missing note %q: %v", want, notes)
			}
		}
		if flags.selectFields != "results" {
			t.Fatal("emitter mutated root flags")
		}
	})

	t.Run("snapshot", func(t *testing.T) {
		mktTestEnv(t)
		fake := newMktFakeEtsy(t)
		defer fake.Close()
		mktPointAtFake(t, fake)
		check(t, "market", "snapshot", "--term", "linen apron", "--dry-run")
		check(t, "market", "snapshot", "--terms", filepath.Join(t.TempDir(), "missing.txt"), "--dry-run")
		m := check(t, "market", "snapshot", "--term", "linen apron", "--max-calls", "20")
		check(t, "market", "snapshot", "--resume", fmt.Sprint(m["snapshot_id"]))
	})
}
