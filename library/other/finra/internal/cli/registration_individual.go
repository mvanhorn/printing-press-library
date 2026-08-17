// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/client"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelRegistrationIndividualCmd(flags *rootFlags) *cobra.Command {
	var flagCrd string

	cmd := &cobra.Command{
		Use:   "individual",
		Short: "Current registration snapshot for one person by CRD",
		Long: "Current registration snapshot for one person by --crd, via the confirmed Registration\n" +
			"Validation Individual by-id lookup.\n\n" +
			"Use this for a single current-snapshot lookup; use 'registration timeline' for the full\n" +
			"chronological history instead.\n\n" +
			"Requires a FINRA credential entitled for registration data — a basic-tier credential will\n" +
			"receive a 403 with a clear permission-denied message.",
		Example:     "--crd 1234567 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--crd=1000001", "pp:requires-tier": "entitled"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch the registration snapshot for CRD %s\n", flagCrd)
				return nil
			}
			if strings.TrimSpace(flagCrd) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--crd is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/data/group/{group}/name/{name}/id/{id}"
			path = replacePathParam(path, "group", "REGISTRATION")
			path = replacePathParam(path, "name", "REGISTRATIONVALIDATIONINDIVIDUAL")
			path = replacePathParam(path, "id", flagCrd)

			data, getErr := c.Get(ctx, path, nil)
			view := registrationIndividualView{CRD: flagCrd}
			if getErr != nil {
				var apiErrTyped *client.APIError
				if !errors.As(getErr, &apiErrTyped) || apiErrTyped.StatusCode != 404 {
					return classifyAPIError(getErr, flags)
				}
				view.Found = false
			} else {
				var record map[string]any
				if err := json.Unmarshal(data, &record); err != nil {
					return apiErr(fmt.Errorf("parsing REGISTRATION/REGISTRATIONVALIDATIONINDIVIDUAL response: %w", err))
				}
				view.Found = true
				view.Record = record
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				status := "not found"
				if view.Found {
					status = "found"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "CRD %s: %s\n", view.CRD, status)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCrd, "crd", "", "CRD number to look up (required)")
	return cmd
}

type registrationIndividualView struct {
	CRD    string         `json:"crd"`
	Found  bool           `json:"found"`
	Record map[string]any `json:"record,omitempty"`
}
