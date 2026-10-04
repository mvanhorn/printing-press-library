// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Regression test for fix F2: a hidden app must not read as "no Steam app
// matched the title". Offline httptest only.

package steam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestSteamAppHiddenMessageIsNotTitleMatch: the Item error for an app hidden
// from anonymous requests (age or region gate) must carry its own message,
// must not reuse ErrAppNotFound's "matched the title" text, and must still
// satisfy errors.Is for BOTH sentinels.
func TestSteamAppHiddenMessageIsNotTitleMatch(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != getItemsPath {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"response":{"store_items":[{"item_type":0,"id":1245690,"success":15,"visible":false,"name":"","store_url_path":"app/0/","store_url_slug":"","appid":0}]}}`)
	})

	_, err := c.Item(context.Background(), 1245690)
	if err == nil {
		t.Fatal("Item on a hidden app must fail")
	}
	if !strings.Contains(err.Error(), "is hidden from anonymous store requests") {
		t.Errorf("error = %v, want it to say the app is hidden from anonymous store requests", err)
	}
	if strings.Contains(err.Error(), "matched the title") {
		t.Errorf("error = %v, must not reuse ErrAppNotFound's title-match text", err)
	}
	if !errors.Is(err, ErrAppNotFound) {
		t.Errorf("errors.Is(err, ErrAppNotFound) = false, want true (exit code 3 depends on it); err=%v", err)
	}
	if !errors.Is(err, ErrAppHidden) {
		t.Errorf("errors.Is(err, ErrAppHidden) = false, want true; err=%v", err)
	}
}
