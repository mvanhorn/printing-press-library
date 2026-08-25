// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// tenantSurface is one probed endpoint family and what it reported.
type tenantSurface struct {
	Surface string `json:"surface"`
	Gate    string `json:"gate"`
	Rows    int    `json:"rows"`
	Message string `json:"message,omitempty"`
}

type tenantReport struct {
	Account   string          `json:"account"`
	Exists    bool            `json:"exists"`
	Locations int             `json:"locations"`
	Surfaces  []tenantSurface `json:"surfaces"`
	Note      string          `json:"note,omitempty"`
}

func newNovelTenantCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant [account]",
		Short: "Report which surfaces an account actually exposes: open, sign-in-gated, or plan-gated.",
		Long: strings.Trim(`
Probe an iClassPro account and classify each catalog surface.

This exists because iClassPro reports both of its access gates with HTTP 200 and
an empty list, distinguishing them only by a human-readable message. A client
that reads status codes alone cannot tell "this gym has no camps" from "this gym
requires you to sign in before it will show you anything". Run this first
against any unfamiliar account.`, "\n"),
		Example:     "  iclasspro-pp-cli tenant examplegym --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<account>=scottsdalegymnastics"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tenant")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			report := tenantReport{Account: account, Surfaces: make([]tenantSurface, 0, 7)}

			probe := func(name, path string, params map[string]string) icpGate {
				rows, env, gate, perr := icpGet(ctx, c, path, params)
				s := tenantSurface{Surface: name, Rows: len(rows), Message: env.Message, Gate: string(gate)}
				if perr != nil {
					s.Gate = "error"
					s.Message = perr.Error()
				}
				report.Surfaces = append(report.Surfaces, s)
				return gate
			}

			if locGate := probe("locations", "/"+account+"/locations", nil); locGate == icpGateNotFound {
				report.Note = icpGateNote(account, icpGateNotFound)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), report, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), report.Note)
				return nil
			}
			report.Exists = true

			locs, _, err := icpLocations(ctx, c, account)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			report.Locations = len(locs)

			locID := 1
			if len(locs) > 0 {
				locID = locs[0].ID
			}

			classGate := probe("classes", "/"+account+"/classes", map[string]string{"limit": "1", "page": "1"})
			probe("camps", "/"+account+"/camps", map[string]string{"locationId": fmt.Sprint(locID), "limit": "1"})
			probe("booking-menu", fmt.Sprintf("/%s/bookings/%d", account, locID), nil)
			probe("class-programs", fmt.Sprintf("/%s/class-programs/%d", account, locID), nil)
			probe("levels", fmt.Sprintf("/%s/levels/active/%d", account, locID), nil)
			probe("appointments", "/"+account+"/appointments", nil)

			if classGate == icpGateSignIn {
				report.Note = icpGateNote(account, icpGateSignIn)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Account:   %s\n", report.Account)
			fmt.Fprintf(cmd.OutOrStdout(), "Locations: %d\n\n", report.Locations)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SURFACE\tSTATUS\tROWS\tMESSAGE")
			for _, s := range report.Surfaces {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", s.Surface, s.Gate, s.Rows, truncate(s.Message, 48))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if report.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", report.Note)
			}
			return nil
		},
	}
	return cmd
}
