package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureAnalyticsSchema(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "analytics.db")
	st, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer st.Close()

	// Idempotency: calling twice must not error.
	if err := st.EnsureAnalyticsSchema(ctx); err != nil {
		t.Fatalf("first EnsureAnalyticsSchema: %v", err)
	}
	if err := st.EnsureAnalyticsSchema(ctx); err != nil {
		t.Fatalf("second EnsureAnalyticsSchema: %v", err)
	}

	cases := []struct {
		slug     string
		igUserID string
		name     string
	}{
		{"acme", "17841400000000001", "Acme Brand"},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			_, err := st.DB().ExecContext(ctx,
				`INSERT OR REPLACE INTO ig_brands(slug, ig_user_id, name, username, added_at) VALUES (?,?,?,?,?)`,
				tc.slug, tc.igUserID, tc.name, tc.slug, time.Now().UTC().Format(time.RFC3339))
			if err != nil {
				t.Fatalf("insert brand: %v", err)
			}

			var gotID, gotName string
			row := st.DB().QueryRowContext(ctx,
				`SELECT ig_user_id, name FROM ig_brands WHERE slug = ?`, tc.slug)
			if err := row.Scan(&gotID, &gotName); err != nil {
				t.Fatalf("read back brand: %v", err)
			}
			if gotID != tc.igUserID {
				t.Errorf("ig_user_id = %q, want %q", gotID, tc.igUserID)
			}
			if gotName != tc.name {
				t.Errorf("name = %q, want %q", gotName, tc.name)
			}
		})
	}
}
