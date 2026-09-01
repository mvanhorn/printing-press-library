// PATCH(library): expose the shared semantic dispatcher through the CLI hook.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/semantic"
	"github.com/spf13/cobra"
)

func init() { registerNovelCommand(registerSemanticCommands) }

func registerSemanticCommands(root *cobra.Command, flags *rootFlags) {
	parent := &cobra.Command{Use: "semantic", Short: "Structured semantic KVM operations (28-op TOOL_SPEC parity)", Annotations: map[string]string{"pp:novel": "true"}}
	for _, name := range semantic.Operations {
		n := name
		cmd := &cobra.Command{
			Use: n, Short: "Execute semantic " + n + " operation", Annotations: map[string]string{"pp:novel": "true"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				if writeRequired(n) && !flags.yes {
					return usageErr(fmt.Errorf("--yes is required for semantic %s (write-gated)", n))
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				args := map[string]any{"write_enabled": flags.yes}
				// per-op flags
				if v, _ := cmd.Flags().GetString("key"); v != "" {
					args["key"] = v
				}
				if v, _ := cmd.Flags().GetString("combo"); v != "" {
					args["combo"] = v
				}
				if v, _ := cmd.Flags().GetString("text"); v != "" {
					args["text"] = v
				}
				if v, _ := cmd.Flags().GetString("machine"); v != "" {
					args["machine"] = v
				}
				if v, _ := cmd.Flags().GetString("target"); v != "" {
					args["target"] = v
				}
				if v, _ := cmd.Flags().GetString("name"); v != "" {
					args["name"] = v
				}
				if v, _ := cmd.Flags().GetString("revision"); v != "" {
					args["revision"] = v
				}
				if v, _ := cmd.Flags().GetString("command"); v != "" {
					args["command"] = v
				}
				if v, _ := cmd.Flags().GetString("transport"); v != "" {
					args["transport"] = v
				}
				if v, _ := cmd.Flags().GetString("button"); v != "" {
					args["button"] = v
				}
				if v, _ := cmd.Flags().GetString("policy"); v != "" {
					args["policy"] = v
				}
				if v, _ := cmd.Flags().GetString("verify_policy"); v != "" {
					args["verify_policy"] = v
				}
				if v, _ := cmd.Flags().GetString("path"); v != "" {
					args["path"] = v
				}
				if v, _ := cmd.Flags().GetString("file"); v != "" {
					args["path"] = v
				}
				if v, _ := cmd.Flags().GetString("confirmation"); v != "" {
					args["confirmation"] = v
				}
				if v, _ := cmd.Flags().GetString("search_text"); v != "" {
					args["search_text"] = v
				}
				if v, _ := cmd.Flags().GetString("approval_token"); v != "" {
					args["approval_token"] = v
				}
				if v, _ := cmd.Flags().GetString("plan"); v != "" {
					var pm map[string]any
					if err := json.Unmarshal([]byte(v), &pm); err == nil {
						args["plan"] = pm
					} else {
						args["plan"] = map[string]any{"target": v}
					}
				}
				if v, _ := cmd.Flags().GetString("actions"); v != "" {
					var actions []any
					if err := json.Unmarshal([]byte(v), &actions); err == nil {
						args["actions"] = actions
					}
				}
				if v, _ := cmd.Flags().GetString("args"); v != "" {
					var extra map[string]any
					if err := json.Unmarshal([]byte(v), &extra); err == nil {
						for k, val := range extra {
							args[k] = val
						}
					}
				}
				if cmd.Flags().Changed("x") {
					if v, _ := cmd.Flags().GetInt("x"); true {
						args["x"] = v
					}
				}
				if cmd.Flags().Changed("y") {
					if v, _ := cmd.Flags().GetInt("y"); true {
						args["y"] = v
					}
				}
				if cmd.Flags().Changed("x_pct") {
					if v, _ := cmd.Flags().GetFloat64("x_pct"); true {
						args["x_pct"] = v
					}
				}
				if cmd.Flags().Changed("y_pct") {
					if v, _ := cmd.Flags().GetFloat64("y_pct"); true {
						args["y_pct"] = v
					}
				}
				if cmd.Flags().Changed("dx") {
					if v, _ := cmd.Flags().GetInt("dx"); true {
						args["dx"] = v
					}
				}
				if cmd.Flags().Changed("dy") {
					if v, _ := cmd.Flags().GetInt("dy"); true {
						args["dy"] = v
					}
				}
				if cmd.Flags().Changed("count") {
					if v, _ := cmd.Flags().GetInt("count"); true {
						args["count"] = v
					}
				}
				if cmd.Flags().Changed("duration_ms") {
					if v, _ := cmd.Flags().GetInt("duration_ms"); true {
						args["duration_ms"] = v
					}
				}
				if cmd.Flags().Changed("preview_max_width") {
					if v, _ := cmd.Flags().GetInt("preview_max_width"); true {
						args["preview_max_width"] = v
					}
				}
				if cmd.Flags().Changed("settle_s") {
					if v, _ := cmd.Flags().GetFloat64("settle_s"); true {
						args["settle_s"] = v
					}
				}
				if cmd.Flags().Changed("approved") {
					if v, _ := cmd.Flags().GetBool("approved"); true {
						args["approved"] = v
					}
				}
				if cmd.Flags().Changed("ttl_s") {
					if v, _ := cmd.Flags().GetInt("ttl_s"); true {
						args["ttl_s"] = v
					}
				}
				out, err := semantic.Dispatch(cmd.Context(), c, n, args)
				if err != nil {
					return err
				}
				return flags.printJSON(cmd, json.RawMessage(out))
			},
		}
		// common flags for all ops that may use them; cobra ignores unused
		cmd.Flags().String("key", "", "key name")
		cmd.Flags().String("combo", "", "key combo (e.g. Ctrl+Alt+T)")
		cmd.Flags().String("text", "", "text to type")
		cmd.Flags().String("machine", "", "machine name")
		cmd.Flags().String("target", "", "target/host name")
		cmd.Flags().String("name", "", "workflow name")
		cmd.Flags().String("revision", "", "workflow revision")
		cmd.Flags().String("command", "", "exec command string")
		cmd.Flags().String("transport", "", "transport (ssh)")
		cmd.Flags().String("button", "", "mouse button")
		cmd.Flags().String("policy", "", "verify policy")
		cmd.Flags().String("verify_policy", "", "verify policy")
		cmd.Flags().String("path", "", "file path")
		cmd.Flags().String("file", "", "file path alias")
		cmd.Flags().String("confirmation", "", "reboot confirmation string")
		cmd.Flags().String("search_text", "", "OCR search text")
		cmd.Flags().String("approval_token", "", "sequence/workflow approval token")
		cmd.Flags().String("plan", "", "JSON plan object")
		cmd.Flags().String("actions", "", "JSON action array")
		cmd.Flags().String("args", "", "JSON object merged into operation args (escape hatch)")
		cmd.Flags().Int("x", 0, "normalized x")
		cmd.Flags().Int("y", 0, "normalized y")
		cmd.Flags().Float64("x_pct", 0, "x percent 0..100")
		cmd.Flags().Float64("y_pct", 0, "y percent 0..100")
		cmd.Flags().Int("dx", 0, "scroll dx")
		cmd.Flags().Int("dy", 0, "scroll dy")
		cmd.Flags().Int("count", 0, "click count")
		cmd.Flags().Int("duration_ms", 0, "hold duration ms")
		cmd.Flags().Int("preview_max_width", 0, "snapshot preview max width")
		cmd.Flags().Float64("settle_s", 0, "settle seconds")
		cmd.Flags().Bool("approved", false, "approval flag")
		cmd.Flags().Int("ttl_s", 0, "authorization TTL seconds")
		parent.AddCommand(cmd)
	}
	addNovelCommandIfAbsent(root, parent)
}

func writeRequired(op string) bool {
	switch op {
	case "host.reboot", "select", "target-switch", "hid_reset", "rearm_otg",
		"kvm_send_text", "kvm_send_keys", "keyboard", "kvm_hold_key", "kvm_release_all",
		"kvm_mouse_move", "mouse", "kvm_mouse_move_pct", "kvm_mouse_click", "kvm_mouse_scroll",
		"kvm_ocr_click", "exec_command",
		"kvm_sequence_authorize", "kvm_sequence_execute", "sequence",
		"kvm_workflow_authorize", "kvm_workflow_execute":
		return true
	default:
		return false
	}
}
