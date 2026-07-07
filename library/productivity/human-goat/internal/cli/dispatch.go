// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/magic"
)

func newNovelDispatchCmd(flags *rootFlags) *cobra.Command {
	var flagVia string
	var flagExecute bool

	cmd := &cobra.Command{
		Use:   "dispatch <task>",
		Short: "Routes a plain-language task to Magic (remote-doable) or TaskRabbit (in-person) by task shape, with a --via override.",
		Example: `  human-goat-pp-cli dispatch "call the dentist and reschedule my cleaning"
  human-goat-pp-cli dispatch "assemble an ikea dresser" --via taskrabbit`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !commandHasChangedFlags(cmd) {
				return cmd.Help()
			}
			task := strings.TrimSpace(strings.Join(args, " "))
			if task == "" {
				return usageErr(fmt.Errorf("missing task"))
			}

			route, reason, err := dispatchRoute(task, flagVia)
			if err != nil {
				return usageErr(err)
			}
			decision := dispatchDecision{
				Task:             task,
				Route:            route,
				Reason:           reason,
				SuggestedCommand: dispatchSuggestedCommand(route, task),
			}

			if dryRunOK(flags) || !flagExecute {
				return printDispatchDecision(cmd, flags, decision)
			}

			if route == "taskrabbit" {
				fmt.Fprintf(cmd.OutOrStdout(), "taskrabbit route requires `hire` (autonomous checkout is gated); run: %s\n", decision.SuggestedCommand)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would dispatch Magic task: %s\n", task)
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client, err := magic.NewClient()
			if err != nil {
				return fmt.Errorf("initialize Magic client: %w", err)
			}
			req, err := client.Send(ctx, magic.SendParams{
				Title:        "Errand",
				Instructions: task,
				Objective:    "Complete the errand and report back",
			})
			if err != nil {
				return fmt.Errorf("dispatch Magic task: %w", err)
			}
			persistMagicRequest(ctx, req)
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), magicRequestSummary(req), flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "request id: %s\n", req.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagVia, "via", "", "Route override: magic or taskrabbit")
	cmd.Flags().BoolVar(&flagExecute, "execute", false, "Create a Magic request for Magic routes; TaskRabbit checkout remains gated")
	return cmd
}

type dispatchDecision struct {
	Task             string `json:"task"`
	Route            string `json:"route"`
	Reason           string `json:"reason"`
	SuggestedCommand string `json:"suggested_command"`
}

func dispatchRoute(task, via string) (string, string, error) {
	via = strings.ToLower(strings.TrimSpace(via))
	switch via {
	case "magic", "taskrabbit":
		return via, "forced by --via", nil
	case "":
	default:
		return "", "", fmt.Errorf("--via must be magic or taskrabbit")
	}

	needle := strings.ToLower(task)
	remote := containsAnyPhrase(needle, []string{
		"call", "phone", "dial", "ask", "lookup", "look up", "research", "find out", "hours",
		"order online", "book online", "reservation", "schedule a", "email", "fill out", "data entry",
	})
	inPerson := containsAnyPhrase(needle, []string{
		"move", "moving", "haul", "assemble", "assembly", "mount", "mounting", "install", "clean",
		"cleaning", "yard", "furniture", "tv", "ikea", "pack", "unpack", "lift", "delivery in person",
	})

	switch {
	case remote && !inPerson:
		return "magic", "matched remote-doable task shape", nil
	case inPerson && !remote:
		return "taskrabbit", "matched in-person task shape", nil
	default:
		return "magic", "ambiguous; defaulting to magic (override with --via)", nil
	}
}

func containsAnyPhrase(s string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(s, phrase) {
			return true
		}
	}
	return false
}

func dispatchSuggestedCommand(route, task string) string {
	switch route {
	case "taskrabbit":
		return fmt.Sprintf("human-goat-pp-cli hire %q --on <date> --min-rating 4.8 --lat <lat> --lng <lng>", task)
	default:
		return fmt.Sprintf("human-goat-pp-cli send --title %q --instructions %q --objective %q", "Errand", task, "Complete the errand and report back")
	}
}

func printDispatchDecision(cmd *cobra.Command, flags *rootFlags, decision dispatchDecision) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), decision, flags)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "task: %s\n", decision.Task)
	fmt.Fprintf(w, "route: %s\n", decision.Route)
	fmt.Fprintf(w, "reason: %s\n", decision.Reason)
	fmt.Fprintf(w, "run: %s\n", decision.SuggestedCommand)
	return nil
}
