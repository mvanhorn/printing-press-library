package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// PATCH(slack-settings-typed-body-578): requestBody parity for POST /integrations/slack/settings across the three places it is declared.
// spec.yaml (the print's input) must carry the typed body (channel: string, notifyEvents: array of string) instead of the untyped
// catalog placeholder, tools-manifest.json must expose both as body params, and the command must expose both as flags. A reprint from an
// untyped spec would silently drop the flags and leave only --stdin; see .printing-press-patches/slack-settings-typed-body-578.json.
func slackSettingsRequestBodyLines(t *testing.T) []string {
	t.Helper()
	lines := strings.Split(readRepoFile(t, "spec.yaml"), "\n")
	start := -1
	for i, l := range lines {
		if l == "  /integrations/slack/settings:" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("spec.yaml: POST /integrations/slack/settings missing")
	}
	var body []string
	in := false
	for _, l := range lines[start+1:] {
		if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "   ") {
			break // next path
		}
		switch strings.TrimSpace(l) {
		case "requestBody:":
			in = true
			continue
		case "responses:":
			in = false
		}
		if in {
			body = append(body, strings.TrimSpace(l))
		}
	}
	if len(body) == 0 {
		t.Fatal("spec.yaml: requestBody missing for POST /integrations/slack/settings")
	}
	return body
}

func containsInOrder(lines, needles []string) bool {
	i := 0
	for _, l := range lines {
		if i < len(needles) && l == needles[i] {
			i++
		}
	}
	return i == len(needles)
}

func TestSlackSettingsRequestBodyIsTypedInSpecManifestAndCommand(t *testing.T) {
	body := slackSettingsRequestBodyLines(t)
	if !containsInOrder(body, []string{"channel:", "type: string", "notifyEvents:", "type: array", "items:", "type: string"}) {
		t.Errorf("spec.yaml requestBody is not the typed channel / notifyEvents body:\n%s", strings.Join(body, "\n"))
	}
	for _, l := range body {
		if l == "additionalProperties: true" {
			t.Error("spec.yaml requestBody still carries the untyped catalog placeholder")
		}
	}

	var manifest any
	if err := json.Unmarshal([]byte(readRepoFile(t, "tools-manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	tool := findNamedObject(manifest, "integrations_post-slack-settings")
	if tool == nil {
		t.Fatal("tools-manifest.json: integrations_post-slack-settings missing")
	}
	got := map[string]string{}
	params, _ := tool["params"].([]any)
	for _, p := range params {
		pm, _ := p.(map[string]any)
		if pm["location"] == "body" {
			got[pm["name"].(string)], _ = pm["type"].(string)
		}
	}
	if got["channel"] != "string" || got["notifyEvents"] != "array" {
		t.Errorf("tools-manifest.json body params = %v, want channel: string and notifyEvents: array", got)
	}

	cmd := newIntegrationsPostSlackSettingsCmd(&rootFlags{})
	if cmd.Flags().Lookup("channel") == nil || cmd.Flags().Lookup("notify-events") == nil {
		t.Error("command lacks --channel or --notify-events")
	}
}

func findNamedObject(v any, name string) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		if x["name"] == name {
			return x
		}
		for _, c := range x {
			if m := findNamedObject(c, name); m != nil {
				return m
			}
		}
	case []any:
		for _, c := range x {
			if m := findNamedObject(c, name); m != nil {
				return m
			}
		}
	}
	return nil
}
