// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
//
// `mirrors` reads each Overpass instance's own /status endpoint at call time.
// There is nothing to cache: the answer is "which host is healthy right now",
// and a cached copy of that is worse than no answer at all.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/subjects"

	"github.com/spf13/cobra"
)

func newNovelMirrorsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirrors",
		Short: "Shows which Overpass mirrors are answering and how many rate-limit slots each has free",
		Long: strings.Trim(`
Which Overpass instances are up right now.

Public Overpass mirrors fall over independently under load. On 2026-07-26,
three of the four hosts then under consideration refused the same query while
the remaining one served it in under a second; one of those four was later
dropped for serving a regional extract, leaving the three probed here. Every
search already fails over automatically across them, so this command is for
when queries are slow or failing and the question is whether the problem is the
query or the host.

Every mirror is probed at the same time under its own short timeout, so a hung
host costs the answer a few seconds rather than a minute.
`, "\n"),
		Example:     "  overpass-pp-cli mirrors",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			runner := subjects.NewRunner(flags.timeout)
			statuses := runner.CheckMirrors(ctx)

			if flags.asJSON {
				return flags.printJSONLive(cmd, map[string]any{"mirrors": statuses})
			}
			rows := make([][]string, 0, len(statuses))
			var healthy int
			for _, s := range statuses {
				state := red("down")
				if s.Healthy {
					state = green("up")
					healthy++
				}
				slots := "—"
				if s.Healthy {
					slots = fmt.Sprintf("%d of %d", s.SlotsFree, s.RateLimit)
				}
				rows = append(rows, []string{
					strings.TrimSuffix(strings.TrimPrefix(s.Mirror, "https://"), "/api/interpreter"),
					state, slots, fmt.Sprintf("%dms", s.TookMS), s.Detail,
				})
			}
			if err := flags.printTable(cmd, []string{"MIRROR", "STATE", "SLOTS FREE", "LATENCY", "DETAIL"}, rows); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if healthy == 0 {
				fmt.Fprintln(out, "\nno mirror is answering; searches will fail until one recovers")
			} else {
				fmt.Fprintf(out, "\n%d of %d mirrors healthy; searches fail over automatically in this order\n",
					healthy, len(statuses))
			}
			return nil
		},
	}
	return cmd
}
