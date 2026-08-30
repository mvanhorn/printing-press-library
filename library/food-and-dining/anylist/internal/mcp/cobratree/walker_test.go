package cobratree

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

func TestRegisterAllIncludingEndpointsExposesCLICommands(t *testing.T) {
	newRoot := func() *cobra.Command {
		root := &cobra.Command{Use: "anylist"}
		root.AddCommand(
			&cobra.Command{
				Use: "create",
				Run: func(*cobra.Command, []string) {},
				Annotations: map[string]string{
					EndpointAnnotation: "folders.create",
				},
			},
			&cobra.Command{
				Use: "sync",
				Run: func(*cobra.Command, []string) {},
			},
		)
		return root
	}
	cliPath := func() (string, error) { return "/bin/true", nil }

	withoutEndpoints := server.NewMCPServer("test", "1.0.0")
	RegisterAll(withoutEndpoints, newRoot(), cliPath)
	if _, ok := withoutEndpoints.ListTools()["create"]; ok {
		t.Fatal("RegisterAll unexpectedly exposed an endpoint command")
	}

	withEndpoints := server.NewMCPServer("test", "1.0.0")
	RegisterAllIncludingEndpoints(withEndpoints, newRoot(), cliPath)
	if _, ok := withEndpoints.ListTools()["create"]; !ok {
		t.Fatal("RegisterAllIncludingEndpoints did not expose an endpoint command")
	}
	if _, ok := withEndpoints.ListTools()["sync"]; !ok {
		t.Fatal("RegisterAllIncludingEndpoints dropped a novel command")
	}
}
