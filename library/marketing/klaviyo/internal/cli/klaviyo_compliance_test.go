// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCampaignMessageScheduleRequiresComplianceBeforePOST(t *testing.T) {
	for _, stdin := range []bool{false, true} {
		for _, compliant := range []bool{false, true} {
			for _, dryRun := range []bool{false, true} {
				t.Run(fmt.Sprintf("stdin=%t/compliant=%t/dry-run=%t", stdin, compliant, dryRun), func(t *testing.T) {
					html := "<p>Hello</p>"
					if compliant {
						html += `{% unsubscribe %}`
					}
					fixture := &campaignModelFixture{t: t, html: map[string]string{"TPL0001": html}, bodies: map[string]map[string]any{}}
					posts, templateReads := 0, 0
					campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/api/templates/TPL0001" {
							templateReads++
						}
						if r.Method == http.MethodPost {
							posts++
							if r.URL.Path != "/api/campaign-message-schedule" || templateReads == 0 {
								t.Errorf("POST reached %s before compliance: template reads=%d", r.URL.Path, templateReads)
							}
						}
						fixture.ServeHTTP(w, r)
					}))
					args := []string{"--data-type", "campaign-message-schedule", "--data-attributes-strategy", "immediate",
						"--data-relationships-campaign-message", fmt.Sprintf(`{"data":{"type":"campaign-message","id":%q}}`, modelMessageID)}
					if stdin {
						bodyPath := filepath.Join(t.TempDir(), "schedule.json")
						body := fmt.Sprintf(`{"data":{"type":"campaign-message-schedule","attributes":{"strategy":"immediate"},"relationships":{"campaign-message":{"data":{"type":"campaign-message","id":%q}}}}}`, modelMessageID)
						if err := os.WriteFile(bodyPath, []byte(body), 0600); err != nil {
							t.Fatal(err)
						}
						file, err := os.Open(bodyPath)
						if err != nil {
							t.Fatal(err)
						}
						old := os.Stdin
						os.Stdin = file
						t.Cleanup(func() { os.Stdin = old; file.Close() })
						args = []string{"--stdin"}
					}
					cmd := newCampaignMessageScheduleScheduleCampaignMessageCmd(&rootFlags{asJSON: true, noCache: true, dryRun: dryRun})
					cmd.SetOut(&bytes.Buffer{})
					cmd.SetErr(&bytes.Buffer{})
					cmd.SetArgs(args)
					err := cmd.Execute()
					switch {
					case dryRun:
						if err != nil || len(fixture.requests) != 0 {
							t.Fatalf("dry-run: error=%v requests=%v", err, fixture.requests)
						}
					case compliant:
						if err != nil || posts != 1 {
							t.Fatalf("compliant schedule: error=%v posts=%d", err, posts)
						}
					default:
						if err == nil || !strings.Contains(err.Error(), "campaign compliance preflight failed") || posts != 0 {
							t.Fatalf("noncompliant schedule: error=%v posts=%d", err, posts)
						}
					}
				})
			}
		}
	}
}
