package cli

import "github.com/spf13/cobra"

func init() {
	novelCommands = registerWalkingPadCommands
}

// registerWalkingPadCommands wires all hand-authored novel commands onto root.
// Generated commands (scan, doctor, status, capabilities, start, wake, set-speed,
// set-mode) are already registered by the generated root.go and MUST NOT be added
// here. `stop` is the exception: it is NOT generated (removed from the spec) because
// stopping is a held-connection ceremony, so it is hand-authored below.
func registerWalkingPadCommands(root *cobra.Command, flags *rootFlags) {
	// Live telemetry (read-only, long-running — mcp:hidden).
	root.AddCommand(newMonitorCmd(flags))
	// Live telemetry (read-only, one-shot — mcp:read-only).
	root.AddCommand(newLastRecordCmd(flags))
	// Live control (physical-effect, long-running — mcp:hidden).
	root.AddCommand(newRunCmd(flags))
	// Stop the belt with the firmware's ceremony (physical-effect — mcp:hidden).
	root.AddCommand(newStopCmd(flags))
	// Belt configuration preferences (configuration-risk — mcp:hidden).
	root.AddCommand(newPrefsCmd(flags))
	// Local history + analytics (read-only local store).
	root.AddCommand(newRecordCmd(flags))
	root.AddCommand(newTodayCmd(flags))
	root.AddCommand(newSessionsCmd(flags))
	root.AddCommand(newTrendsCmd(flags))
	root.AddCommand(newStreakCmd(flags))
	root.AddCommand(newCaloriesCmd(flags))
	root.AddCommand(newExportCmd(flags))
	// Local profile (body weight for calorie estimates).
	root.AddCommand(newProfileCmd(flags))
}
