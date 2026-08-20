// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through fetchCPSC in cpsc_novel_support.go

package cli

import (
	"errors"
	"github.com/spf13/cobra"
	"net/url"
)

func newNovelPacketCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "packet RECALL_ID",
		Short:       "Join one recall's affected products, remedy, contact, images, incidents, and official URL into an action packet.",
		Example:     "cpsc-recalls-pp-cli packet 10000 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitCPSCDryRun(cmd, flags, "packet", map[string]any{"recall_id": args[0], "request_parameters": map[string]any{"RecallID": args[0]}})
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows, err := newCPSCFetcher(flags).fetch(ctx, url.Values{"RecallID": {args[0]}})
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return errors.New("recall not found")
			}
			r := rows[0]
			return emitCPSC(cmd, flags, "live", map[string]any{"recall_id": r["RecallID"], "title": r["Title"], "date": r["RecallDate"], "description": r["Description"], "products": r["Products"], "hazards": r["Hazards"], "injuries": r["Injuries"], "incidents": r["Incidents"], "remedies": r["Remedies"], "remedy_options": r["RemedyOptions"], "consumer_contact": r["ConsumerContact"], "images": r["Images"], "official_url": r["URL"], "caveats": cpscCaveats()})
		},
	}
	return cmd
}
