// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: unified cross-clinic agenda. Jane forces a separate login per
// clinic subdomain and shows no combined view; this merges upcoming
// appointments from every logged-in clinic into one chronological list.

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newNovelAgendaCmd(flags *rootFlags) *cobra.Command {
	var allProfiles bool
	var includePast bool

	cmd := &cobra.Command{
		Use:   "agenda",
		Short: "See every appointment across every Jane clinic you use in one chronological view.",
		Long: `Merge your upcoming appointments from every logged-in Jane clinic into a
single chronological agenda. Jane keeps each clinic on its own subdomain with a
separate login and no combined view; this stitches them together.

By default it shows all logged-in clinics; pass --clinic to scope to one.`,
		Example:     "  janeapp-pp-cli agenda\n  janeapp-pp-cli agenda --agent --select clinic,date,start_at,practitioner,treatment",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			// Default to all logged-in clinics unless a specific --clinic was given.
			all := allProfiles || flags.clinicName == ""
			clinics, err := clinicsForRead(flags, all)
			if err != nil {
				return err
			}
			recs, err := gatherAppointments(cmd, flags, clinics)
			if err != nil {
				return err
			}
			if !includePast {
				now := time.Now()
				kept := make([]apptRecord, 0, len(recs))
				for _, r := range recs {
					if str(r.view["status"]) == "cancelled" {
						continue
					}
					if r.Start.IsZero() || !r.Start.Before(now) {
						kept = append(kept, r)
					}
				}
				recs = kept
			}
			return renderAppointments(cmd, flags, recs)
		},
	}
	cmd.Flags().BoolVar(&allProfiles, "all-clinics", false, "Force every logged-in clinic. Default: all clinics unless --clinic scopes to one.")
	cmd.Flags().BoolVar(&includePast, "include-past", false, "Include past appointments too")
	return cmd
}
