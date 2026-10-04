// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, path := range [][]string{{"spots", "list"}, {"spots", "get"}, {"events", "list"}, {"events", "get"}, {"events", "region"}, {"events", "prefecture"}} {
			cmd, _, err := root.Find(path)
			if err != nil || cmd == root || cmd.Name() != path[1] {
				continue
			}
			kind, operation := path[0], path[1]
			examples := map[string]string{
				"spots list":        "iko-yo-pp-cli spots list --json",
				"events list":       "iko-yo-pp-cli events list --json",
				"spots get":         "iko-yo-pp-cli spots get --id 8220 --json",
				"events get":        "iko-yo-pp-cli events get --id 8412 --json",
				"events region":     "iko-yo-pp-cli events region --region 6 --json",
				"events prefecture": "iko-yo-pp-cli events prefecture --region 6 --prefecture 11 --json",
			}
			cmd.Example = strings.Trim("\n  "+examples[kind+" "+operation]+"\n", "\n")

			cmd.Flags().VisitAll(func(f *pflag.Flag) { delete(f.Annotations, cobra.BashCompOneRequiredFlag) })
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			cmd.Annotations["mcp:hidden"] = "true"
			cmd.Annotations["mcp:read-only"] = "true"
			cmd.Annotations["pp:data-source"] = "live"
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, kind+" "+operation)
				}
				if len(args) > 0 {
					return usageErr(fmt.Errorf("%s %s accepts flags; use --id, --region or --prefecture", kind, operation))
				}
				if err := validateDataSourceStrategy(flags, "live"); err != nil {
					return usageErr(err)
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				ctx, capCancel := context.WithTimeout(ctx, time.Minute)
				defer capCancel()
				c := trip.New(flags.rateLimit)
				tripSource(cmd, flags, "live")
				if operation == "get" {
					id, _ := cmd.Flags().GetString("id")
					ref := kind + "/" + id
					if _, _, err := trip.ParseReference(ref); err != nil {
						return usageErr(err)
					}
					r, err := c.Inspect(ctx, ref)
					if err != nil {
						return tripError(err)
					}
					if err := tripSave(ctx, flags, []trip.Record{r}); err != nil {
						return apiErr(err)
					}
					return tripPrintRecord(cmd, flags, r)
				}
				region, prefecture := 0, 0
				if operation == "region" || operation == "prefecture" {
					v, _ := cmd.Flags().GetString("region")
					var err error
					region, err = strconv.Atoi(v)
					if err != nil || region < 1 || region > 11 {
						return usageErr(fmt.Errorf("--region must be an observed source ID from 1 to 11"))
					}
				}
				if operation == "prefecture" {
					v, _ := cmd.Flags().GetString("prefecture")
					var err error
					prefecture, err = strconv.Atoi(v)
					if err != nil || prefecture < 1 || prefecture > 47 {
						return usageErr(fmt.Errorf("--prefecture must be an observed source ID from 1 to 47"))
					}
				}
				view, records, err := c.Discover(ctx, kind, region, prefecture, 1, trip.Query{Kind: kind, AgeMonths: -1, Limit: 50})
				if err != nil {
					return tripError(err)
				}
				if err := tripSave(ctx, flags, records); err != nil {
					return apiErr(err)
				}
				return tripPrintDiscovery(cmd, flags, view)
			}
		}
	})
}
