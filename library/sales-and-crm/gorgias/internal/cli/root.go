// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/config"
	"github.com/spf13/cobra"
)

// version is wired in by the build (goreleaser ldflag -X .../cli.version=...).
// `0.0.0-dev` is the fallback when no ldflag is set AND we can't read a
// module version from the binary's BuildInfo (the typical "running from
// `go run` in a working tree" case). CLI and MCP binaries share the same
// fallback to keep introspection envelopes coherent across the pair.
var version = "2026.7.1"

// Version returns the current build's version string. Resolution order:
//
//  1. The goreleaser-injected ldflag (a release tag like "0.1.0").
//  2. The Go module version baked into the binary by `go install
//     <module>@vX.Y.Z` — Go records the resolved version in
//     `runtime/debug.ReadBuildInfo()`. This catches the case where a
//     user installs via `go install ...@latest` and would otherwise see
//     the literal "0.0.0-dev" default.
//  3. The "0.0.0-dev" fallback (local builds from a working tree).
func Version() string {
	if version != "0.0.0-dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		// info.Main.Version is "(devel)" for `go run` / `go build` in a
		// working tree and a real semver like "v0.1.0" for installs via
		// `go install <module>@vX.Y.Z`. We only adopt the latter.
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}

type rootFlags struct {
	asJSON        bool
	compact       bool
	csv           bool
	plain         bool
	quiet         bool
	dryRun        bool
	noCache       bool
	noInput       bool
	idempotent    bool
	ignoreMissing bool
	yes           bool
	agent         bool
	selectFields  string
	configPath    string
	profileName   string
	deliverSpec   string
	timeout       time.Duration
	rateLimit     float64
	dataSource    string
	freshnessMeta any

	// deliverBuf captures command output when --deliver is set to a
	// non-stdout sink. Flushed to the sink after Execute returns.
	deliverBuf  *bytes.Buffer
	deliverSink DeliverSink
}

// RootCmd returns the Cobra command tree without executing it. The MCP server
// uses this to mirror every user-facing command as an agent tool.
func RootCmd() *cobra.Command {
	var flags rootFlags
	return newRootCmd(&flags)
}

// Execute runs the CLI in non-interactive mode: never prompts, all values via flags or stdin.
// Execute runs the CLI and writes any error to stderr. It is the single
// emission point for command failures: Cobra's SilenceErrors keeps the
// library from printing, and main.go does NOT print again — it just sets
// the exit code from the returned error. That contract avoids the
// duplicate-error-line class of bug.
//
// When the caller passed --json or --agent (which implies --json), the
// error is serialized as `{"error": {"message": "...", "exit_code": N}}`
// on stderr so JSON-piping consumers see a parseable failure shape
// instead of plain text mixed into their stream.
func Execute() error {
	var flags rootFlags
	rootCmd := newRootCmd(&flags)

	err := rootCmd.Execute()
	if err != nil && strings.Contains(err.Error(), "unknown flag") {
		msg := err.Error()
		if idx := strings.Index(msg, "unknown flag: "); idx >= 0 {
			flagStr := strings.TrimSpace(msg[idx+len("unknown flag: "):])
			if suggestion := suggestFlag(flagStr, rootCmd); suggestion != "" {
				err = fmt.Errorf("%w\nhint: did you mean --%s?", err, suggestion)
			}
		}
	}
	if err == nil && flags.deliverBuf != nil {
		if derr := Deliver(flags.deliverSink, flags.deliverBuf.Bytes(), flags.compact); derr != nil {
			fmt.Fprintf(os.Stderr, "warning: deliver to %s:%s failed: %v\n", flags.deliverSink.Scheme, flags.deliverSink.Target, derr)
			return derr
		}
	}
	if err != nil && isCobraUsageError(err) {
		// Cobra/pflag pre-RunE errors (unknown flag, unknown command,
		// missing required, etc.) never flow through usageErr() because
		// they originate inside rootCmd.Execute() before any user RunE
		// runs. Without this wrap, ExitCode() falls through to the
		// default and emits 1 — clobbering the conventional code-2 for
		// usage errors that the helpers.go contract already promises.
		err = usageErr(err)
	}
	if err != nil {
		writeExecuteError(err, flags.asJSON)
	}
	return err
}

// writeExecuteError emits the failure on stderr. In JSON mode it's a
// `{"error": {...}}` envelope; otherwise it's the bare message Cobra
// would have printed (we suppressed Cobra's print to keep error output
// to a single line). When the command already emitted a structured
// failure on stdout (e.g. `doctor --json` puts `fail_on_triggered`
// inside its report), it wraps its error with silenceEmission() so this
// function suppresses the duplicate envelope — exit code is still set.
func writeExecuteError(err error, asJSON bool) {
	if isEmissionSilenced(err) {
		return
	}
	if asJSON {
		envelope := map[string]any{
			"error": map[string]any{
				"message":   err.Error(),
				"exit_code": ExitCode(err),
			},
		}
		buf, mErr := json.Marshal(envelope)
		if mErr == nil {
			fmt.Fprintln(os.Stderr, string(buf))
			return
		}
	}
	fmt.Fprintln(os.Stderr, "Error: "+err.Error())
}

// silencedEmissionErr wraps an error to signal that the underlying
// command has already emitted its structured diagnostic on stdout (or
// otherwise) and writeExecuteError must NOT emit another envelope on
// stderr. Exit code is preserved via ExitCode(err).
type silencedEmissionErr struct{ inner error }

func (e *silencedEmissionErr) Error() string { return e.inner.Error() }
func (e *silencedEmissionErr) Unwrap() error { return e.inner }

func silenceEmission(err error) error {
	if err == nil {
		return nil
	}
	return &silencedEmissionErr{inner: err}
}

func isEmissionSilenced(err error) bool {
	var se *silencedEmissionErr
	return errors.As(err, &se)
}

// isCobraUsageError reports whether err matches one of Cobra/pflag's
// pre-RunE usage-error shapes. Detection is by message prefix to match
// the same approach the unknown-flag hint path uses above; neither
// Cobra nor pflag exports typed sentinels for these.
//
// Patterns are anchored to the literal punctuation Cobra and pflag
// emit so an application's own RunE error that happens to contain the
// substring "required flag" or "invalid argument" doesn't get
// misclassified as a usage error.
//
// Patterns covered (Cobra v1.x + pflag v1.x as of 2026-05):
//   - "unknown flag: --foo"                            (pflag)
//   - "unknown shorthand flag: 'x' in -x"              (pflag)
//   - "unknown command \"foo\" for ..."                (Cobra)
//   - "required flag(s) \"foo\" not set"               (Cobra MarkFlagRequired)
//   - "flag needs an argument: --foo"                  (pflag, missing value)
//   - "invalid argument \"x\" for \"--y\" flag: ..."   (pflag, parse failure)
//
// Returns false for nil err.
func isCobraUsageError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "unknown flag") ||
		strings.HasPrefix(msg, "unknown shorthand flag") ||
		strings.HasPrefix(msg, "unknown command") ||
		// Cobra's MarkFlagRequired uses the plural form `required flag(s)`,
		// but several hand-written validators in the generated commands
		// emit the singular `required flag` (no parens). Match both so the
		// exit code is consistently 2 (usage error) across the surface.
		strings.HasPrefix(msg, `required flag(s) "`) ||
		strings.HasPrefix(msg, `required flag "`) ||
		strings.HasPrefix(msg, "flag needs an argument:") ||
		strings.HasPrefix(msg, `invalid argument "`)
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "gorgias-pp-cli",
		Short: `Gorgias CLI — Every Gorgias support workflow, agent-native, in one binary.`,
		// SilenceUsage stops Cobra dumping --help on a runtime error.
		// SilenceErrors stops Cobra printing the error itself; main.go is
		// the single emission point. Without this, every failing command
		// prints the error twice — once from Cobra, once from main.
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version(),
	}
	rootCmd.SetVersionTemplate("gorgias-pp-cli {{ .Version }}\n")

	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Return only key fields (id, name, status, timestamps) for minimal token usage")
	rootCmd.PersistentFlags().BoolVar(&flags.csv, "csv", false, "Output as CSV (table and array responses)")
	rootCmd.PersistentFlags().BoolVar(&flags.plain, "plain", false, "Output as plain tab-separated text")
	rootCmd.PersistentFlags().BoolVar(&flags.quiet, "quiet", false, "Bare output, one value per line")
	rootCmd.PersistentFlags().StringVar(&flags.configPath, "config", "", "Config file path")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Request timeout")
	rootCmd.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "Show request without sending")
	rootCmd.PersistentFlags().BoolVar(&flags.noCache, "no-cache", false, "Bypass response cache")
	rootCmd.PersistentFlags().BoolVar(&flags.noInput, "no-input", false, "Disable all interactive prompts (for CI/agents)")
	rootCmd.PersistentFlags().BoolVar(&flags.idempotent, "idempotent", false, "Treat already-existing create results as a successful no-op")
	rootCmd.PersistentFlags().BoolVar(&flags.ignoreMissing, "ignore-missing", false, "Treat missing delete targets as a successful no-op")
	rootCmd.PersistentFlags().StringVar(&flags.selectFields, "select", "", "Comma-separated fields to include in output (e.g. --select id,name,status)")
	rootCmd.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Skip confirmation prompts (for agents and scripts)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&humanFriendly, "human-friendly", false, "Enable colored output and rich formatting")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Set all agent-friendly defaults (--json --compact --no-input --no-color --yes)")
	rootCmd.PersistentFlags().StringVar(&flags.dataSource, "data-source", "auto", "Data source for read commands: auto (live with local fallback), live (API only), local (synced data only)")
	rootCmd.PersistentFlags().StringVar(&flags.profileName, "profile", "", "Apply values from a saved profile (see 'gorgias-pp-cli profile list')")
	rootCmd.PersistentFlags().StringVar(&flags.deliverSpec, "deliver", "", "Route output to a sink: stdout (default), file:<path>, webhook:<url>")
	rootCmd.PersistentFlags().Float64Var(&flags.rateLimit, "rate-limit", 0, "Max requests per second (0 to disable)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.deliverSpec != "" {
			sink, err := ParseDeliverSink(flags.deliverSpec)
			if err != nil {
				return err
			}
			flags.deliverSink = sink
			if sink.Scheme != "stdout" && sink.Scheme != "" {
				flags.deliverBuf = &bytes.Buffer{}
				cmd.SetOut(io.MultiWriter(os.Stdout, flags.deliverBuf))
			}
		}
		if flags.profileName != "" {
			profile, err := GetProfile(flags.profileName)
			if err != nil {
				return err
			}
			if profile == nil {
				available := ListProfileNames()
				if len(available) == 0 {
					return fmt.Errorf("profile %q not found (no profiles saved yet; run '%s profile save <name> --<flag> <value>')", flags.profileName, cmd.Root().Name())
				}
				return fmt.Errorf("profile %q not found; available: %s", flags.profileName, strings.Join(available, ", "))
			}
			if err := ApplyProfileToFlags(cmd, profile); err != nil {
				return err
			}
		}
		if flags.agent {
			if !cmd.Flags().Changed("json") {
				flags.asJSON = true
			}
			if !cmd.Flags().Changed("compact") {
				flags.compact = true
			}
			if !cmd.Flags().Changed("no-input") {
				flags.noInput = true
			}
			if !cmd.Flags().Changed("yes") {
				flags.yes = true
			}
			if !cmd.Flags().Changed("no-color") {
				noColor = true
			}
		}
		switch flags.dataSource {
		case "auto", "live", "local":
			// valid
		default:
			return fmt.Errorf("invalid --data-source value %q: must be auto, live, or local", flags.dataSource)
		}
		// Opportunistic refresh when GORGIAS_AUTO_REFRESH_TTL is set and the
		// local mirror's per-resource sync_state is older than the TTL. No-op
		// when the env var is unset, which keeps behavior predictable for
		// agents that prefer explicit syncs. Failures are warnings only.
		autoRefreshIfStale(cmd, flags)
		return nil
	}
	rootCmd.AddCommand(newAccountCmd(flags))
	rootCmd.AddCommand(newCustomFieldsCmd(flags))
	rootCmd.AddCommand(newCustomersCmd(flags))
	rootCmd.AddCommand(newEventsCmd(flags))
	rootCmd.AddCommand(newGorgiasJobsCmd(flags))
	rootCmd.AddCommand(newIntegrationsCmd(flags))
	rootCmd.AddCommand(newMacrosCmd(flags))
	rootCmd.AddCommand(newPhoneCmd(flags))
	rootCmd.AddCommand(newRulesCmd(flags))
	rootCmd.AddCommand(newSatisfactionSurveysCmd(flags))
	rootCmd.AddCommand(newTagsCmd(flags))
	rootCmd.AddCommand(newTeamsCmd(flags))
	rootCmd.AddCommand(newTicketsCmd(flags))
	rootCmd.AddCommand(newUsersCmd(flags))
	rootCmd.AddCommand(newViewsCmd(flags))
	rootCmd.AddCommand(newWidgetsCmd(flags))
	rootCmd.AddCommand(newDoctorCmd(flags))
	rootCmd.AddCommand(newAuthCmd(flags))
	rootCmd.AddCommand(newAgentContextCmd(rootCmd))
	rootCmd.AddCommand(newProfileCmd(flags))
	rootCmd.AddCommand(newFeedbackCmd(flags))
	rootCmd.AddCommand(newWhichCmd(flags))
	rootCmd.AddCommand(newExportCmd(flags))
	rootCmd.AddCommand(newImportCmd(flags))
	rootCmd.AddCommand(newSearchCmd(flags))
	rootCmd.AddCommand(newSQLCmd(flags))
	rootCmd.AddCommand(newSyncCmd(flags))
	rootCmd.AddCommand(newTailCmd(flags))
	rootCmd.AddCommand(newAnalyticsCmd(flags))
	rootCmd.AddCommand(newWorkflowCmd(flags))
	rootCmd.AddCommand(newStaleCmd(flags))
	rootCmd.AddCommand(newOrphansCmd(flags))
	rootCmd.AddCommand(newLoadCmd(flags))
	rootCmd.AddCommand(newAPICmd(flags))
	rootCmd.AddCommand(newMessagesPromotedCmd(flags))
	rootCmd.AddCommand(newPickupsPromotedCmd(flags))
	rootCmd.AddCommand(newReportingPromotedCmd(flags))
	rootCmd.AddCommand(newTicketSearchPromotedCmd(flags))
	rootCmd.AddCommand(newVersionCliCmd(flags))

	return rootCmd
}

func ExitCode(err error) int {
	var codeErr *cliError
	if As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}

func (f *rootFlags) newClient() (*client.Client, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	c := client.New(cfg, f.timeout, f.rateLimit)
	c.DryRun = f.dryRun
	c.NoCache = f.noCache
	return c, nil
}

func (f *rootFlags) printTable(w *cobra.Command, headers []string, rows [][]string) error {
	if f.asJSON {
		return fmt.Errorf("use printJSON for JSON output")
	}
	tw := tabwriter.NewWriter(w.OutOrStdout(), 2, 4, 2, ' ', 0)
	header := ""
	for i, h := range headers {
		if i > 0 {
			header += "\t"
		}
		header += h
	}
	fmt.Fprintln(tw, header)
	for _, row := range rows {
		line := ""
		for i, cell := range row {
			if i > 0 {
				line += "\t"
			}
			line += cell
		}
		fmt.Fprintln(tw, line)
	}
	return tw.Flush()
}

func newVersionCliCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Example: `  gorgias-pp-cli version
  gorgias-pp-cli version --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			v := Version()
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"name":    "gorgias-pp-cli",
					"version": v,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "gorgias-pp-cli %s\n", v)
			return nil
		},
	}
}
