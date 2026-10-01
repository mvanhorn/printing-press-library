package mcp

import (
	"strings"

	"github.com/spf13/cobra"
)

// Raw device writes cannot consume the target/plan-bound workflow token.
// Keep these routes closed until they share that authorization protocol.
func physicalWriteBlocked(method, path string) bool {
	return method != "GET" && method != "HEAD" && (strings.HasPrefix(path, "/api/hid/") || path == "/api/streamer/set_params" || path == "/api/system/otg_functions")
}

const physicalWriteGuidance = "direct physical MCP writes are disabled because they lack target- and operation-bound authorization. Use an authorized sequence/workflow for supported keyboard and mouse actions. Raw HID reset, streamer parameters, and OTG configuration have no authorized MCP workflow support yet."

// The semantic dispatcher shares its operation catalog with the interactive
// CLI. MCP admits only observation and planning operations; every other
// operation fails closed before a client can contact the device. Keep this
// list explicit so a newly added semantic write is not exposed by default.
var semanticMCPReadOperations = []string{
	"capabilities", "snapshot", "ocr", "verify", "observe", "verify-text",
	"host.identity.inspect", "host.graphics.inspect", "service.render_access.inspect",
	"kvm_status", "kvm_ocr_screenshot", "kvm_sequence_plan",
	"kvm_workflow_list", "kvm_workflow_inspect", "status",
}

func semanticMCPReadAllowed(operation string) bool {
	for _, allowed := range semanticMCPReadOperations {
		if operation == allowed {
			return true
		}
	}
	return false
}

// Cobra mirrors shell out to the CLI, bypassing the typed MCP endpoint gate.
// Suppress direct physical paths on the MCP-only command tree. The CLI itself
// remains available, as do sequence and workflow execution with their bound,
// one-time authorization tokens.
var physicalCobraMirrorPaths = [][]string{
	{"act"}, {"keyboard"}, {"mouse"}, {"target-switch"}, {"semantic"},
	{"machines", "select"},
}

func hidePhysicalCobraMirrors(root *cobra.Command) {
	for _, names := range physicalCobraMirrorPaths {
		cmd := root
		for _, name := range names {
			var child *cobra.Command
			for _, candidate := range cmd.Commands() {
				if candidate.Name() == name {
					child = candidate
					break
				}
			}
			if child == nil {
				cmd = nil
				break
			}
			cmd = child
		}
		if cmd != nil {
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			cmd.Annotations["mcp:hidden"] = "true"
		}
	}
}
