package contract

import "testing"

func TestDocumentedContract(t *testing.T) {
	if len(Operations) != 60 {
		t.Fatalf("operations=%d", len(Operations))
	}
	for _, o := range Operations {
		t.Run(o.ID, func(t *testing.T) {
			if Find(o.ID) == nil {
				t.Fatal("missing")
			}
			if len(o.BodySchema) > 0 {
				if err := Validate(o.BodySchema, map[string]any{}); err != nil && o.BodySchema["required"] == nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		path string
		body map[string]any
		bad  bool
	}{{"/contacts", map[string]any{"email": "bad"}, true}, {"/contacts", map[string]any{"email": "reader@example.com", "lists": []any{1.0}}, false}, {"/contacts", map[string]any{"email": "reader@example.com", "lists": []any{"bad"}}, true}, {"/automations", map[string]any{"title": "Welcome", "trigger_type": "unknown"}, true}, {"/contacts/bulk-actions", map[string]any{"action": "delete", "target_id": 1}, true}} {
		op, _ := Match("POST", tc.path)
		err := Validate(op.BodySchema, tc.body)
		if (err != nil) != tc.bad {
			t.Errorf("%s err=%v", tc.path, err)
		}
	}
}
func TestExactPathsWin(t *testing.T) {
	for _, p := range []string{"/contacts/unsubscribed", "/contacts/bulk-actions/12"} {
		op, _ := Match("GET", p)
		if op == nil || op.Path == "/contacts/{id}" {
			t.Fatalf("incorrect route %s: %#v", p, op)
		}
	}
}
