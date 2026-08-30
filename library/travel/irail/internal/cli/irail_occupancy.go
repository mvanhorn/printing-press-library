// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored command: occupancy report.
//
// The only write endpoint iRail exposes, and the only tool that surfaces it.
// It contributes a crowd-sourced crowding level for one departure.
//
// Nothing is sent unless --send is passed: the default prints the exact payload
// so the caller can see what would be submitted.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/cliutil"
)

var occupancyLevels = map[string]bool{"low": true, "medium": true, "high": true}

type occupancyPayload struct {
	Connection string `json:"connection"`
	From       string `json:"from"`
	Date       string `json:"date"`
	Vehicle    string `json:"vehicle"`
	Occupancy  string `json:"occupancy"`
}

type occupancyView struct {
	Sent    bool             `json:"sent"`
	Status  int              `json:"status,omitempty"`
	Payload occupancyPayload `json:"payload"`
	Note    string           `json:"note,omitempty"`
}

func newIrailOccupancyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "occupancy",
		Short:       "Contribute crowding feedback for a departure",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newIrailOccupancyReportCmd(flags))
	return cmd
}

func newIrailOccupancyReportCmd(flags *rootFlags) *cobra.Command {
	var flagConnection string
	var flagLevel string
	var flagSend bool

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report how busy a train was (prints the payload; --send submits it)",
		Long: "Submits a crowd-sourced occupancy level for one departure to iRail.\n\n" +
			"This is the only endpoint in this CLI that writes to a remote service. By\n" +
			"default it prints the exact payload and sends nothing; pass --send to submit.\n\n" +
			"The connection URI comes from the departureConnection field of 'board list',\n" +
			"for example http://irail.be/connections/8813003/20260724/IC2344",
		Example: `  irail-pp-cli occupancy report --connection http://irail.be/connections/8813003/20260724/IC2344 --level high
  irail-pp-cli occupancy report --connection http://irail.be/connections/8813003/20260724/IC2344 --level low --send`,
		// Mutates remote state when --send is given, so no read-only hint.
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report occupancy feedback to iRail")
				return nil
			}
			if flagConnection == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--connection is required (the departureConnection URI from 'board list')"))
			}
			level := strings.ToLower(strings.TrimSpace(flagLevel))
			if !occupancyLevels[level] {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--level must be one of low, medium, high (got %q)", flagLevel))
			}

			payload, err := occupancyPayloadFrom(flagConnection, level)
			if err != nil {
				return usageErr(err)
			}

			view := occupancyView{Payload: payload}

			// Never submit crowd-sourced data from a verification run.
			if cliutil.IsVerifyEnv() {
				view.Note = "verification environment: nothing was submitted"
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if !flagSend {
				view.Note = "nothing was sent; re-run with --send to submit this to iRail"
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would submit occupancy %q for %s\n%s\n",
					level, payload.Vehicle, view.Note)
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// pp:client-call
			_, code, err := c.Post(ctx, "/feedback/occupancy", payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view.Sent = true
			view.Status = code
			view.Note = "submitted to iRail"

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "submitted occupancy %q for %s (HTTP %d)\n", level, payload.Vehicle, code)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagConnection, "connection", "", "departureConnection URI from 'board list'")
	cmd.Flags().StringVar(&flagLevel, "level", "", "Crowding level: low, medium or high")
	cmd.Flags().BoolVar(&flagSend, "send", false, "Actually submit the feedback to iRail")
	return cmd
}

// occupancyPayloadFrom derives the full feedback body from a departure
// connection URI of the form
// http://irail.be/connections/<stationId>/<yyyymmdd>/<vehicle>.
//
// Note the feedback endpoint takes yyyymmdd, unlike the ddmmyy the read
// endpoints use.
func occupancyPayloadFrom(connection, level string) (occupancyPayload, error) {
	parts := strings.Split(strings.TrimSuffix(connection, "/"), "/")
	if len(parts) < 3 {
		return occupancyPayload{}, fmt.Errorf(
			"could not read %q; expected a URI like http://irail.be/connections/8813003/20260724/IC2344", connection)
	}
	vehicle := parts[len(parts)-1]
	date := parts[len(parts)-2]
	stationID := parts[len(parts)-3]

	if len(date) != 8 {
		return occupancyPayload{}, fmt.Errorf(
			"could not read the date from %q; expected yyyymmdd, got %q", connection, date)
	}
	// Station ids appear unpadded inside connection URIs but 9 digits wide in
	// station URIs.
	for len(stationID) < 9 {
		stationID = "0" + stationID
	}

	return occupancyPayload{
		Connection: connection,
		From:       "http://irail.be/stations/NMBS/" + stationID,
		Date:       date,
		Vehicle:    "http://irail.be/vehicle/" + vehicle,
		Occupancy:  "http://api.irail.be/terms/" + level,
	}, nil
}
