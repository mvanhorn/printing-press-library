// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/store"
)

type foodsPageClient struct {
	t     *testing.T
	pages []string
}

func (c *foodsPageClient) RateLimit() float64 { return 0 }

func (c *foodsPageClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	c.t.Helper()
	if path != "/v1/foods/list" || params["pageSize"] != "100" {
		c.t.Fatalf("unexpected foods request: path=%q params=%v", path, params)
	}
	page := params["pageNumber"]
	c.pages = append(c.pages, page)
	switch page {
	case "0":
		items := make([]map[string]any, 100)
		for i := range items {
			items[i] = map[string]any{"fdcId": i + 1, "description": fmt.Sprintf("Food %d", i+1)}
		}
		data, err := json.Marshal(items)
		return data, err
	case "1":
		return json.RawMessage(`[{"fdcId":101,"description":"Food 101"}]`), nil
	default:
		c.t.Fatalf("unexpected pageNumber %q", page)
		return nil, nil
	}
}

func TestFoodsSyncUsesPagePagination(t *testing.T) {
	defaults := determinePaginationDefaults("foods")
	if !resourceSupportsPagination("foods") {
		t.Fatal("foods must be marked paginated")
	}
	if defaults.cursorType != "page" || defaults.cursorParam != "pageNumber" || defaults.limitParam != "pageSize" {
		t.Fatalf("foods pagination defaults = %#v", defaults)
	}
}

func TestFoodsSyncReadsEveryPageFromListEndpoint(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "nutrition.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &foodsPageClient{t: t}
	result := syncResource(context.Background(), c, db, "foods", "", true, 0, false, false, nil, io.Discard)
	if result.Err != nil || result.Count != 101 {
		t.Fatalf("sync result = %#v", result)
	}
	if len(c.pages) != 2 || c.pages[0] != "0" || c.pages[1] != "1" {
		t.Fatalf("requested pages = %v, want [0 1]", c.pages)
	}
	count, err := db.Count("foods")
	if err != nil || count != 101 {
		t.Fatalf("stored foods = %d, err = %v", count, err)
	}
}
