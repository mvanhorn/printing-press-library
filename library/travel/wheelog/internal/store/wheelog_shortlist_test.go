package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"path/filepath"
	"testing"
	"time"
)

func TestWheelogObservationPairAndEmptyStore(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.InitWheelog(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := db.WheelogList(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("empty store %v,%v", items, err)
	}
	spot := wheelog.Spot{ID: 166345, Name: "Public restroom", Category: "toilet", ObservedAt: time.Now().UTC().Format(time.RFC3339), Questions: []wheelog.Question{}, Gaps: []string{}}
	if err := db.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	items, err = db.WheelogList(ctx)
	if err != nil || items[0].Previous != nil {
		t.Fatalf("invented baseline %v,%v", items, err)
	}
	spot.Name = "New recorded name"
	if err := db.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	items, err = db.WheelogList(ctx)
	if err != nil || items[0].Previous.Name != "Public restroom" || items[0].Latest.Name != "New recorded name" {
		t.Fatalf("rotation failed %+v,%v", items, err)
	}
	if err := db.RemoveWheelog(ctx, 166345); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get("spots", "166345"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("removed entry survived the search projection: %v", err)
	}
	items, err = db.WheelogList(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("remove failed %v,%v", items, err)
	}
}

func TestWheelogLatestZeroReportsReplaceOldEvidence(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.InitWheelog(ctx); err != nil {
		t.Fatal(err)
	}
	positive, zero := 1, 0
	spot := wheelog.Spot{ID: 166345, Name: "Public restroom", Category: "toilet", Questions: []wheelog.Question{{ID: 102, Positive: &positive, Negative: &zero, State: "reported_affirmative"}}}
	if err := db.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	spot.Questions[0].Positive = &zero
	spot.Questions[0].State = "unreported"
	if err := db.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	data, err := db.Get("spots", "166345")
	if err != nil {
		t.Fatal(err)
	}
	var latest wheelog.Spot
	if err := json.Unmarshal(data, &latest); err != nil {
		t.Fatal(err)
	}
	if *latest.Questions[0].Positive != 0 || latest.Questions[0].State != "unreported" {
		t.Fatal("index retained stale affirmative reports")
	}
}
