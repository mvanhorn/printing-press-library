package cli

// PATCH(reprint-637-freeform-json-flags): the free-form JSON flags the pre-reprint print had are accepted again and land in the
// body exactly as before (the parsed JSON under the one body key); they refuse to mix with the generated sub-flags.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFreeformJSONFlagsLandInTheBodyAsBefore(t *testing.T) {
	const obj = `{"k":"v","n":2,"nested":{"a":[1,"x"]}}`
	want := map[string]any{"k": "v", "n": float64(2), "nested": map[string]any{"a": []any{float64(1), "x"}}}
	cases := []struct {
		name  string
		build func(*rootFlags) *cobra.Command
		args  []string
		flag  string
		key   string
		sub   string // one generated sub-flag of the same object, to prove the refusal
	}{
		{"activities create-activity", newActivitiesCreateActivityCmd, []string{"--title", "t", "--type", "call"}, "metadata", "metadata", "--metadata-source"},
		{"automations update", newAutomationsUpdateCmd, []string{"A1", "--name", "n", "--trigger-event", "contact.created"}, "trigger-conditions", "triggerConditions", "--trigger-conditions-webhook-id"},
		{"bulk edit-contacts", newBulkEditContactsCmd, []string{"--ids", "a"}, "updates", "updates", "--updates-city"},
		{"bulk edit-deals", newBulkEditDealsCmd, []string{"--ids", "a"}, "updates", "updates", "--updates-title"},
		{"contacts create", newContactsCreateCmd, []string{"--first-name", "f"}, "custom-fields", "customFields", "--custom-fields-website"},
		{"forms create", newFormsCreateCmd, []string{"--name", "n", "--slug", "s", "--fields", "[]"}, "settings", "settings", "--settings-consent-link-url"},
		{"integrations connect --config", newIntegrationsConnectCmd, []string{"slack"}, "config", "config", "--config-channel"},
		{"integrations connect --credentials", newIntegrationsConnectCmd, []string{"slack"}, "credentials", "credentials", "--credentials-api-key"},
		{"scoring-rules create", newScoringRulesCreateCmd, []string{"--name", "n", "--points", "5"}, "condition", "condition", "--condition-field"},
	}
	for _, tc := range cases {
		cmd := tc.build(&rootFlags{})
		if cmd.Flags().Lookup(tc.flag) == nil {
			t.Errorf("%s: --%s is not registered", tc.name, tc.flag)
			continue
		}
		bodies, err := runNestedCmd(t, tc.build, append(append([]string{}, tc.args...), "--"+tc.flag, obj)...)
		if err != nil || len(bodies) != 1 {
			t.Errorf("%s --%s: bodies %v err %v", tc.name, tc.flag, bodies, err)
			continue
		}
		if !reflect.DeepEqual(bodies[0][tc.key], want) {
			t.Errorf("%s --%s: body[%s] = %#v, want %#v", tc.name, tc.flag, tc.key, bodies[0][tc.key], want)
		}
		if _, err := runNestedCmd(t, tc.build, append(append([]string{}, tc.args...), "--"+tc.flag, "{not json")...); err == nil || !strings.Contains(err.Error(), "--"+tc.flag) {
			t.Errorf("%s --%s: malformed JSON err = %v", tc.name, tc.flag, err)
		}
		if tc.sub == "" {
			t.Errorf("%s: no sub-flag in the row; every restored flag pins its mixing refusal", tc.name)
			continue
		}
		subFlag := strings.TrimPrefix(tc.sub, "--")
		if cmd.Flags().Lookup(subFlag) == nil {
			t.Errorf("%s: sub-flag %s not registered", tc.name, tc.sub)
			continue
		}
		b, err := runNestedCmd(t, tc.build, append(append([]string{}, tc.args...), "--"+tc.flag, obj, tc.sub, "x")...)
		if err == nil || !strings.Contains(err.Error(), "cannot be combined") || len(b) != 0 {
			t.Errorf("%s: --%s with %s: bodies %v err %v, want a refusal and no request", tc.name, tc.flag, tc.sub, b, err)
		}
	}
}

func TestEmailsCreateTemplateTypedFlagsAsBefore(t *testing.T) {
	b, err := runNestedCmd(t, newEmailsCreateTemplateCmd, "--name", "N", "--channel", "email", "--body", "hi", "--subject", "S", "--category", "c", "--body-html", "<p>hi</p>", "--is-active=false")
	if err != nil || len(b) != 1 {
		t.Fatalf("typed flags: bodies %v err %v", b, err)
	}
	want := map[string]any{"name": "N", "channel": "email", "body": "hi", "subject": "S", "category": "c", "bodyHtml": "<p>hi</p>", "isActive": false}
	if !reflect.DeepEqual(b[0], want) {
		t.Errorf("typed body = %v, want %v", b[0], want)
	}
	// the pre-reprint required flags still apply to the typed shape
	if b, err := runNestedCmd(t, newEmailsCreateTemplateCmd, "--name", "N", "--channel", "email"); err == nil || !strings.Contains(err.Error(), `"body"`) || len(b) != 0 {
		t.Errorf("missing --body: bodies %v err %v", b, err)
	}
	// --body-json still works on its own and refuses to mix with the typed flags
	if b, err := runNestedCmd(t, newEmailsCreateTemplateCmd, "--body-json", `{"name":"N","channel":"email","bodyMode":"html","html":"<p/>"}`); err != nil || len(b) != 1 || b[0]["bodyMode"] != "html" {
		t.Errorf("--body-json: bodies %v err %v", b, err)
	}
	if b, err := runNestedCmd(t, newEmailsCreateTemplateCmd, "--body-json", `{}`, "--name", "N"); err == nil || len(b) != 0 {
		t.Errorf("--body-json with --name: bodies %v err %v, want a refusal", b, err)
	}
}
