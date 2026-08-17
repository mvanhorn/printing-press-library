// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored fail-closed mutation gate for Judge.me.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/judgeme/internal/cliutil"
	"github.com/spf13/cobra"
)

func installJudgeMeMutationGate(root *cobra.Command, flags *rootFlags) {
	var apply bool
	root.PersistentFlags().BoolVar(&apply, "apply", false, "Explicitly authorize a remote write (mutating commands otherwise require --dry-run)")
	var wrap func(*cobra.Command)
	wrap = func(cmd *cobra.Command) {
		method := strings.ToUpper(cmd.Annotations["pp:method"])
		if cmd.RunE != nil && isJudgeMeMutation(method) {
			run := cmd.RunE
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if flags.dryRun || apply || (cliutil.IsVerifyEnv() && !cliutil.IsVerifyLiveHTTPEnv()) {
					return run(cmd, args)
				}
				return usageErr(fmt.Errorf("%s performs a remote %s; preview with --dry-run, then repeat with --apply", cmd.CommandPath(), method))
			}
		}
		for _, child := range cmd.Commands() {
			wrap(child)
		}
	}
	wrap(root)
}

func isJudgeMeMutation(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}
