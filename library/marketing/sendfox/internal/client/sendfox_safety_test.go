package client

import (
	"context"
	"testing"
)

func TestSendfoxMutationApproval(t *testing.T) {
	body := map[string]any{"email": "reader@example.com"}
	for _, tc := range []struct {
		name, method, path         string
		body                       any
		write, sensitive, dry, bad bool
	}{{"unapproved", "POST", "/contacts", body, false, false, false, true}, {"approved", "POST", "/contacts", body, true, false, false, false}, {"send-needs-human", "POST", "/campaigns/42/send", nil, true, false, false, true}, {"approved-send", "POST", "/campaigns/42/send", nil, true, true, false, false}, {"delete-preview", "DELETE", "/contacts/42", nil, false, false, true, false}, {"invalid-preview", "POST", "/contacts", map[string]any{"email": "invalid"}, false, false, true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sendfoxPrepare(WithMutationApproval(context.Background(), tc.write, tc.sensitive), tc.method, tc.path, nil, tc.body, tc.dry)
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
func TestSendfoxSafeDefaults(t *testing.T) {
	for _, tc := range []struct {
		path, key string
		body      map[string]any
		want      bool
	}{{"/automations", "active", map[string]any{"title": "Welcome"}, false}, {"/contacts/bulk-actions", "dry_run", map[string]any{"action": "apply_tag", "target_id": 4}, true}} {
		got, err := sendfoxPrepare(context.Background(), "POST", tc.path, nil, tc.body, true)
		if err != nil {
			t.Fatal(err)
		}
		if got.(map[string]any)[tc.key] != tc.want {
			t.Fatal(got)
		}
		if _, ok := tc.body[tc.key]; ok {
			t.Fatal("mutated caller")
		}
	}
}

func TestSendfoxVerifyCannotDialProduction(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")
	c, rec := newClientWithRecorder(t)
	c.BaseURL = "https://api.sendfox.com"
	_, _, err := c.do(context.Background(), "DELETE", "/contacts/12", nil, nil, nil)
	if err == nil || rec.calls != 0 {
		t.Fatalf("production verify err=%v calls=%d", err, rec.calls)
	}
}
