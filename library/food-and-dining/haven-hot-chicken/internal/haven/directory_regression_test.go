package haven

import (
	"context"
	"database/sql"
	"testing"
)

func TestSaveReplacesCompleteDirectoryKeepsHistory(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	for _, tc := range []struct {
		locations []Location
		want      int
	}{
		{[]Location{{ID: 1, Name: "Old"}, {ID: 2, Name: "Current"}}, 2},
		{[]Location{{ID: 2, Name: "Current"}}, 1},
		{nil, 1},
		{[]Location{}, 0},
	} {
		if err = Save(ctx, db, tc.locations, []Snapshot{sample(1)}); err != nil {
			t.Fatal(err)
		}
		rows, err := LoadLocations(ctx, db)
		if err != nil || len(rows) != tc.want {
			t.Fatalf("directory: %v %v", rows, err)
		}
	}
	history, err := Latest(ctx, db, 1, 10)
	if err != nil || len(history) != 4 {
		t.Fatalf("lost history: %d %v", len(history), err)
	}
}
