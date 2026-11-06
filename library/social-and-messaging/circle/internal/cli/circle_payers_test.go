// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/store"
	"github.com/spf13/cobra"
)

func TestCirclePayersCappedCheckpoint(t *testing.T) {
	for _, cappedResource := range []string{"", "community-member-subscriptions", "community-member-charges"} {
		t.Run("cap="+cappedResource, func(t *testing.T) {
			testenv.Isolate(t)
			previousHooks := clientHooks
			t.Cleanup(func() { clientHooks = previousHooks })
			gets := 0
			registerClientHook(func(c *client.Client) error {
				c.Config.CircleApiToken = "payer-unit-test"
				c.HTTPClient.Transport = showTellScheduleTransport(func(req *http.Request) (*http.Response, error) {
					if req.Method != "GET" || (req.URL.Path != "/api/admin/v2/community_member_subscriptions" && req.URL.Path != "/api/admin/v2/community_member_charges") {
						return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					gets++
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"records":[],"has_next_page":false}`)), Request: req}, nil
				})
				return nil
			})
			file := filepath.Join(t.TempDir(), "payers.db")
			db, err := store.Open(file)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			watermark := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
			for _, resource := range []string{"community-member-subscriptions", "community-member-charges"} {
				if _, _, _, err := db.BindSyncScope(resource, "test"); err != nil {
					t.Fatal(err)
				}
				// A capped/latest-only walk with a derived watermark also clears its cursor.
				if err := saveSyncResourceCheckpoint(db, resource, "test", "", 0, watermark, resource != cappedResource); err != nil {
					t.Fatal(err)
				}
				cursor, synced, _, err := db.GetSyncState(resource)
				if err != nil || cursor != "" || !synced.Equal(watermark) {
					t.Fatalf("checkpoint changed: cursor=%q synced=%s error=%v", cursor, synced, err)
				}
			}
			for _, mode := range []string{"local", "auto"} {
				cmd := &cobra.Command{}
				cmd.SetErr(io.Discard)
				payers, source, err := loadCirclePayers(context.Background(), cmd, &rootFlags{dataSource: mode, noCache: true}, file, "")
				if cappedResource != "" && mode == "local" {
					if err == nil || !strings.Contains(err.Error(), "sync missing or incomplete") {
						t.Fatalf("capped local error = %v", err)
					}
					continue
				}
				wantSource := "local"
				if cappedResource != "" {
					wantSource = "live"
				}
				if err != nil || source != wantSource || len(payers) != 0 {
					t.Fatalf("mode=%s payers=%v source=%s error=%v", mode, payers, source, err)
				}
			}
			wantGets := 0
			if cappedResource != "" {
				wantGets = 2
			}
			if gets != wantGets {
				t.Fatalf("live requests=%d, want %d", gets, wantGets)
			}
		})
	}
}
