package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// PATCH(slack-settings-typed-body-578): the operation-set drift gate cannot see request schemas, so this pins that spec.yaml (the print's input)
// and upstream/openapi.yaml (the drift copy) carry the same typed body for POST /integrations/slack/settings; see .printing-press-patches.json.
func slackSettingsRequestBody(t *testing.T, file string) any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", file))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	op, ok := doc.Paths["/integrations/slack/settings"]["post"].(map[string]any)
	if !ok {
		t.Fatalf("%s: POST /integrations/slack/settings missing", file)
	}
	body, ok := op["requestBody"]
	if !ok {
		t.Fatalf("%s: requestBody missing", file)
	}
	return body
}

func TestSlackSettingsRequestBodyMatchesBetweenSpecAndUpstream(t *testing.T) {
	spec := slackSettingsRequestBody(t, "spec.yaml")
	upstream := slackSettingsRequestBody(t, "upstream/openapi.yaml")
	if !reflect.DeepEqual(spec, upstream) {
		t.Fatalf("requestBody differs between spec.yaml and upstream/openapi.yaml:\n spec:     %#v\n upstream: %#v", spec, upstream)
	}
	// and it is the typed body, not the untyped placeholder
	schema := spec.(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	ch, _ := props["channel"].(map[string]any)
	ne, _ := props["notifyEvents"].(map[string]any)
	items, _ := ne["items"].(map[string]any)
	if ch["type"] != "string" || ne["type"] != "array" || items["type"] != "string" {
		t.Errorf("channel/notifyEvents are not typed string / array of string: %#v", props)
	}
}
