// Hand-written for whoop-pp-cli — `whoami` is the "beat row 7 of the absorb
// manifest" surface that combines user profile + body measurement into a
// single agent-friendly call. Greg's 1.0 CLI made you call two separate
// commands; the brief calls out a single `whoami` summary as a beat-not-just-
// match win. Read-only, MCP-safe.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/whoop/internal/client"

	"github.com/spf13/cobra"
)

func newWhoamiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Summary of your WHOOP profile + body measurement in one shot",
		Long: `Fetches /v1/user/profile/basic and /v1/user/measurement/body in
sequence and returns a flattened JSON envelope. Useful for skill startup
("hi, I'm whoami-ing you to verify your token works") and as a single tool
in the MCP surface for agents that want one identity call.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  whoop-pp-cli whoami --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			profile, perr := getOne(c, "/v1/user/profile/basic")
			body, berr := getOne(c, "/v1/user/measurement/body")
			if perr != nil && berr != nil {
				return fmt.Errorf("both profile and body lookups failed: profile=%v / body=%v", perr, berr)
			}
			result := map[string]any{}
			if profile != nil {
				result["profile"] = profile
			} else {
				result["profile_error"] = perr.Error()
			}
			if body != nil {
				result["body_measurement"] = body
			} else {
				result["body_measurement_error"] = berr.Error()
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func getOne(c *client.Client, path string) (any, error) {
	raw, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw), nil
	}
	return obj, nil
}
