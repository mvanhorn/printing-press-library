// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/store"
)

// browseArticlesLocal renders synced articles for a vertical (and optional
// sub-vertical) from the local store. Used as the offline branch by
// articles_browse and articles_browse_sub when --data-source local.
func browseArticlesLocal(vertical, sub string, limit int, flags *rootFlags) error {
	db, err := openStoreOrErr()
	if err != nil {
		return fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()
	rows, err := selectLocalArticles(db, vertical, sub, limit)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"vertical":     vertical,
		"sub_vertical": sub,
		"count":        len(rows),
		"results":      rows,
		"source":       "local",
	}
	return emitFromFlags(flags, payload, func() {
		if len(rows) == 0 {
			fmt.Printf("No locally-synced articles for vertical %q.\n", vertical)
			return
		}
		fmt.Printf("Articles in %s (local) — %d results\n", vertical, len(rows))
		for i, a := range rows {
			fmt.Printf("%2d. %s\n    %s\n", i+1, a.Title, a.URL)
		}
	})
}

func selectLocalArticles(db *store.Store, vertical, sub string, limit int) ([]food52.ArticleSummary, error) {
	q := "SELECT data FROM articles WHERE vertical = ?"
	args := []any{vertical}
	if sub != "" {
		q += " AND json_extract(data, '$.sub_vertical') = ?"
		args = append(args, sub)
	}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("local query: %w", err)
	}
	defer rows.Close()
	out := []food52.ArticleSummary{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var a food52.ArticleSummary
		if err := json.Unmarshal(data, &a); err == nil {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}
