package cli

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAgentContextCredentialRequirements(t *testing.T) {
	out, _, err := s2Run(t, "agent-context")
	s2Must(t, err)
	var ctx struct {
		Auth struct {
			EnvVars []struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
			} `json:"env_vars"`
		} `json:"auth"`
	}
	s2Decode(t, out, &ctx)
	want := map[string]bool{"ETSY_API_KEY": true, "ETSY_SHARED_SECRET": true, "ETSY_OAUTH2": false, "ETSY_SHOP_ID": false}
	got := map[string]bool{}
	for _, v := range ctx.Auth.EnvVars {
		got[v.Name] = v.Required
	}
	for name, req := range want {
		if r, ok := got[name]; !ok || r != req {
			t.Fatalf("%s required=%v present=%v, want %v (all: %v)", name, r, ok, req, got)
		}
	}
}

func TestManifestUserConfigMatchesCredentials(t *testing.T) {
	raw, err := os.ReadFile("../../manifest.json")
	s2Must(t, err)
	var m struct {
		Server struct {
			MCP struct {
				Env map[string]string `json:"env"`
			} `json:"mcp_config"`
		} `json:"server"`
		UserConfig map[string]struct {
			Required  bool `json:"required"`
			Sensitive bool `json:"sensitive"`
		} `json:"user_config"`
	}
	s2Must(t, json.Unmarshal(raw, &m))
	for env, key := range map[string]string{"ETSY_API_KEY": "etsy_api_key", "ETSY_SHARED_SECRET": "etsy_shared_secret", "ETSY_OAUTH2": "etsy_oauth2", "ETSY_SHOP_ID": "etsy_shop_id"} {
		if m.Server.MCP.Env[env] != "${user_config."+key+"}" {
			t.Fatalf("env %s = %q", env, m.Server.MCP.Env[env])
		}
		if _, ok := m.UserConfig[key]; !ok {
			t.Fatalf("user_config missing %s", key)
		}
	}
	if !m.UserConfig["etsy_api_key"].Required || m.UserConfig["etsy_oauth2"].Required || m.UserConfig["etsy_shop_id"].Required {
		t.Fatalf("required flags wrong: %+v", m.UserConfig)
	}
	if !m.UserConfig["etsy_shared_secret"].Sensitive {
		t.Fatal("shared secret must be sensitive")
	}
}
