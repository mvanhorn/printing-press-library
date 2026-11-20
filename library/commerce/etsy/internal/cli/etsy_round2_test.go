package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsy-ext: precommit scrubbing alone must remove both current and fixed legacy logs.
func TestEtsyRound2PrecommitLegacyLog(t *testing.T) {
	eraseIsolate(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ETSY_DATA_DIR", t.TempDir())
	data, e := cliutil.DataDir()
	s2Must(t, e)
	path := filepath.Join(data, "data.db")
	seedErase(t, path)
	legacy, e := learn.LegacyTeachLogPath()
	s2Must(t, e)
	if strings.HasPrefix(legacy, data+string(os.PathSeparator)) {
		t.Fatal("legacy path followed DataDir")
	}
	state, e := cliutil.StateDir()
	s2Must(t, e)
	paths := []string{legacy, legacy + ".1", filepath.Join(state, "teach.log")}
	for _, p := range paths {
		s2Must(t, os.MkdirAll(filepath.Dir(p), 0700))
		s2Must(t, os.WriteFile(p, []byte("receipt 1000000001\nkeep 1000000011\n"), 0600))
	}
	old := etsyPostCommitRewriteTeachLog
	etsyPostCommitRewriteTeachLog = func(map[string]bool) error { return nil }
	t.Cleanup(func() { etsyPostCommitRewriteTeachLog = old })
	_, e = eraseRun(t, "", "--receipt-id", "1000000001", "--yes", "--no-vacuum")
	s2Must(t, e)
	for _, p := range paths {
		body, e := os.ReadFile(p)
		s2Must(t, e)
		if string(body) != "keep 1000000011\n" {
			t.Fatalf("precommit did not scrub %s: %s", p, body)
		}
	}
}

func TestEtsyRound2LogScale(t *testing.T) {
	ids := map[string]bool{}
	for i := 0; i < 5000; i++ {
		ids[fmt.Sprint(1000000000+i)] = true
	}
	start := time.Now()
	for i := 0; i < 5000; i++ {
		if etsyLogMatches([]byte(fmt.Sprintf(`{"query":"other receipt %d"}`, 2000000000+i)), ids) {
			t.Fatal("unrelated token matched")
		}
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("log scrub quadratic: %v", elapsed)
	}
	if !etsyLogMatches([]byte(`{"query":"receipt 1000000001"}`), ids) {
		t.Fatal("matching token lost")
	}
}

func TestEtsyRound2WorstVacuum(t *testing.T) {
	eraseIsolate(t)
	data, e := cliutil.DataDir()
	s2Must(t, e)
	paths := []string{filepath.Join(data, "data-a.db"), filepath.Join(data, "data-z.db")}
	for _, p := range paths {
		seedErase(t, p)
	}
	ownerDB, e := store.Open(paths[1])
	s2Must(t, e)
	_, e = ownerDB.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('users','me','{"user_id":1000000999,"is_current_user":true}')`)
	s2Must(t, e)
	s2Must(t, ownerDB.Close())
	db, e := store.OpenReadOnly(paths[0])
	s2Must(t, e)
	defer db.Close()
	reader, e := db.DB().Begin()
	s2Must(t, e)
	defer reader.Rollback()
	var n int
	s2Must(t, reader.QueryRow(`SELECT count(*) FROM receipts`).Scan(&n))
	out, e := eraseRun(t, "", "--buyer-user-id", "1000000002", "--yes")
	s2Must(t, e)
	if !strings.Contains(out, "Own-id refusal unavailable") {
		t.Fatal("missing owner state was overwritten", out)
	}
	if !strings.Contains(out, "vacuum incomplete (checkpoint busy)") {
		t.Fatal(out)
	}
	for _, pair := range [][2]string{{"checkpoint_busy", "completed"}, {"optimize_busy", "checkpoint_busy"}, {"checkpoint_busy", "skipped_busy"}, {"skipped_busy", "error"}} {
		if etsyVacuumRank(pair[0]) == etsyVacuumRank(pair[1]) {
			t.Fatal("lost rank")
		}
	}
}

func TestEtsyRound2VacuumFlagsAndNoVictims(t *testing.T) {
	eraseIsolate(t)
	data, e := cliutil.DataDir()
	s2Must(t, e)
	seedErase(t, filepath.Join(data, "data.db"))
	if _, e = eraseRun(t, "", "--buyer-user-id", "1000000002", "--vacuum", "--no-vacuum"); e == nil {
		t.Fatal("conflicting flags accepted")
	}
	unsuppress, e := eraseRun(t, "", "--unsuppress", "--receipt-id", "1000000001", "--yes", "--json")
	s2Must(t, e)
	if strings.Contains(unsuppress, `"vacuum": "completed"`) {
		t.Fatal("unsuppress vacuumed a store without victims")
	}
	out, e := eraseRun(t, "", "--buyer-user-id", "1000000999", "--yes", "--json")
	s2Must(t, e)
	if strings.Contains(out, `"vacuum": "completed"`) {
		t.Fatal("vacuum ran without victims")
	}
}
