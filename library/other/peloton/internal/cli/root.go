// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

// Package cli is the Cobra command surface for peloton-pp-cli.
//
// The CLI follows printing-press conventions: typed exit codes (0/2/3/4/5/7),
// auto-JSON when stdout isn't a TTY, and --compact for token-thrifty output.
// See AGENTS.md / SKILL.md in this directory for the agent-facing contract.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootFlags holds globals shared by every subcommand. Pulled out so we can
// test commands in isolation without reaching into a package var.
type rootFlags struct {
	configPath string
	asJSON     bool
	compact    bool
}

// Version is the CLI's reported version. CONTRIBUTING.md requires every
// printing-press CLI to respond to --version; cobra wires this string into
// the auto-generated --version flag once it's set on the root command.
const Version = "0.1.0"

// Execute is the only export — main.go calls this and propagates the error
// to ExitCode for the right exit status.
func Execute() error {
	flags := &rootFlags{}
	root := &cobra.Command{
		Use:     "peloton-pp-cli",
		Short:   "Peloton workout, ride, and music history CLI",
		Version: Version,
		Long: `peloton-pp-cli reads your Peloton history through the same private
API the members.onepeloton.com SPA uses. There is no public Peloton
API; auth happens via a browser harvest of the Auth0 SPA localStorage
token (run "peloton-pp-cli auth login" to bootstrap).

Once authenticated, commands run against api.onepeloton.com directly.
Tokens last about an hour; re-run "auth login" when one expires.`,
		SilenceUsage:  true, // errors print once, not twice
		SilenceErrors: true, // we handle stderr in main.go via ExitCode
	}
	root.PersistentFlags().StringVar(&flags.configPath, "config", "",
		"Path to config TOML (default: ~/.config/peloton-pp-cli/config.toml)")
	root.PersistentFlags().BoolVar(&flags.asJSON, "json", false,
		"Force JSON output even on a TTY")
	root.PersistentFlags().BoolVar(&flags.compact, "compact", false,
		"Project to high-gravity fields only (~60-80% fewer tokens; implies --json)")

	root.AddCommand(newAuthCmd(flags))
	root.AddCommand(newMeCmd(flags))
	root.AddCommand(newWorkoutsCmd(flags))
	root.AddCommand(newRideCmd(flags))
	root.AddCommand(newDiscoveriesCmd(flags))
	root.AddCommand(newSyncCmd(flags))
	root.AddCommand(newSearchCmd(flags))
	root.AddCommand(newVersionCmd())

	return root.Execute()
}

// --- Typed exit codes ---
//
// These mirror the printing-press convention. Wrapping errors with a CodedErr
// lets ExitCode peel off the right status without parsing strings.

// Code is the typed exit code enum. 0 = success, 2 = bad invocation,
// 3 = not found, 4 = auth needed, 5 = upstream API error, 7 = rate-limited.
type Code int

const (
	CodeSuccess     Code = 0
	CodeUsage       Code = 2
	CodeNotFound    Code = 3
	CodeAuth        Code = 4
	CodeAPI         Code = 5
	CodeRateLimited Code = 7
)

// CodedErr wraps an error with an exit code. Use Errf to construct.
type CodedErr struct {
	Code Code
	Err  error
}

func (c *CodedErr) Error() string { return c.Err.Error() }
func (c *CodedErr) Unwrap() error { return c.Err }

// Errf is a typed-fmt.Errorf. Returns a *CodedErr that ExitCode can read.
func Errf(code Code, format string, args ...any) error {
	return &CodedErr{Code: code, Err: fmt.Errorf(format, args...)}
}

// ExitCode returns the exit code for an error. Untyped errors default to 1
// (generic failure) so we never silently exit 0 on an error path.
func ExitCode(err error) int {
	if err == nil {
		return int(CodeSuccess)
	}
	var ce *CodedErr
	if errors.As(err, &ce) {
		return int(ce.Code)
	}
	return 1
}

// newVersionCmd is the `version` subcommand. We keep this alongside cobra's
// auto-generated --version flag because some catalog convention scripts call
// `<cli> version` (no flag) — easier to ship both than to argue about it.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stdout, "peloton-pp-cli "+Version)
		},
	}
}
