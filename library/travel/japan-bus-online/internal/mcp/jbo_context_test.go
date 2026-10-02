package mcp

import "testing"

func TestProviderContextMatchesSupportedWorkflow(t *testing.T) {
	commands := map[string]bool{}
	for _, resource := range providerContextResources() {
		if resource["syncable"] != false {
			t.Fatal("provider inventory cache advertised")
		}
		for _, command := range resource["commands"].([]string) {
			commands[command] = true
		}
	}
	for _, command := range []string{"routes list", "bus route", "bus services", "bus quote", "bus conditions"} {
		if !commands[command] {
			t.Fatalf("missing supported command %s", command)
		}
	}
}
