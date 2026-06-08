// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source live

type alertCluster struct {
	Key              string   `json:"key"`
	OrganizationName string   `json:"organizationName,omitempty"`
	ConditionName    string   `json:"conditionName"`
	DeviceCount      int      `json:"deviceCount"`
	AlertCount       int      `json:"alertCount"`
	FirstSeen        string   `json:"firstSeen"`
	LastSeen         string   `json:"lastSeen"`
	SampleUids       []string `json:"sampleUids"`
}

type alertStormsView struct {
	Clusters   []alertCluster `json:"clusters"`
	Count      int            `json:"count"`
	WindowSecs int64          `json:"window_secs"`
	MinCluster int            `json:"min_cluster"`
	Note       string         `json:"note,omitempty"`
}

// clusterAccumulator is the mutable per-bucket aggregate.
type clusterAccumulator struct {
	key       string
	orgName   string
	condition string
	devices   map[int64]struct{}
	alerts    int
	first     int64
	last      int64
	samples   []string
}

func newNovelAlertStormsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWindow     string
		flagMinCluster int
		flagLimit      int
	)

	cmd := &cobra.Command{
		Use:   "alert-storms",
		Short: "Collapse a flood of active alerts into ranked incidents grouped by organization, condition, and time window.",
		Long: `Fetch active alerts and bucket them by (organization, condition, time window) to
collapse a storm into a handful of ranked incident clusters. Clusters smaller
than --min-cluster are dropped.

Examples:
  ninjaone-pp-cli alert-storms
  ninjaone-pp-cli alert-storms --window 30m --min-cluster 3
  ninjaone-pp-cli alert-storms --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRunPreview(cmd, flags, "would fetch active alerts and collapse them into ranked incident clusters by org/condition/time-window")
			}

			window := time.Hour
			if flagWindow != "" {
				d, err := cliutil.ParseDurationLoose(flagWindow)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --window: %w", err))
				}
				window = d
			}
			windowSecs := int64(window.Seconds())
			if flagMinCluster < 1 {
				flagMinCluster = 2
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			orgs, err := fetchOrgs(ctx, c)
			if err != nil {
				return err
			}
			devices, _, err := fetchDevices(ctx, c, "", effectiveMaxScanPages(5))
			if err != nil {
				return err
			}
			_, devToOrg := deviceOrgIndex(devices)

			alerts, err := fetchAlerts(ctx, c)
			if err != nil {
				return err
			}

			buckets := map[string]*clusterAccumulator{}
			for _, a := range alerts {
				cond := a.ConditionName
				if cond == "" {
					cond = a.SourceName
				}
				orgID := devToOrg[a.DeviceID]
				orgName := orgs[orgID]
				orgPart := orgName
				if orgPart == "" {
					orgPart = "dev:" + strconv.FormatInt(a.DeviceID, 10)
				}
				ts := a.createSeconds()
				bucket := timeBucket(ts, windowSecs)
				key := fmt.Sprintf("%s|%s|%d", orgPart, cond, bucket)

				acc := buckets[key]
				if acc == nil {
					acc = &clusterAccumulator{
						key:       key,
						orgName:   orgName,
						condition: cond,
						devices:   map[int64]struct{}{},
						first:     ts,
						last:      ts,
						samples:   make([]string, 0, 5),
					}
					buckets[key] = acc
				}
				acc.devices[a.DeviceID] = struct{}{}
				acc.alerts++
				if ts < acc.first {
					acc.first = ts
				}
				if ts > acc.last {
					acc.last = ts
				}
				if len(acc.samples) < 5 && a.UID != "" {
					acc.samples = append(acc.samples, a.UID)
				}
			}

			clusters := make([]alertCluster, 0, len(buckets))
			for _, acc := range buckets {
				if acc.alerts < flagMinCluster {
					continue
				}
				clusters = append(clusters, alertCluster{
					Key:              acc.key,
					OrganizationName: acc.orgName,
					ConditionName:    acc.condition,
					DeviceCount:      len(acc.devices),
					AlertCount:       acc.alerts,
					FirstSeen:        isoFromEpoch(acc.first),
					LastSeen:         isoFromEpoch(acc.last),
					SampleUids:       acc.samples,
				})
			}
			sort.SliceStable(clusters, func(i, j int) bool {
				if clusters[i].AlertCount != clusters[j].AlertCount {
					return clusters[i].AlertCount > clusters[j].AlertCount
				}
				return clusters[i].Key < clusters[j].Key
			})
			if n := boundLimit(len(clusters), flagLimit); n < len(clusters) {
				clusters = clusters[:n]
			}

			view := alertStormsView{
				Clusters:   clusters,
				Count:      len(clusters),
				WindowSecs: windowSecs,
				MinCluster: flagMinCluster,
			}
			if len(clusters) == 0 {
				view.Note = fmt.Sprintf("no incident clusters of >= %d alerts within a %s window", flagMinCluster, window)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(clusters) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			headers := []string{"ORG", "CONDITION", "DEVICES", "ALERTS", "FIRST", "LAST"}
			rows := make([][]string, 0, len(clusters))
			for _, cl := range clusters {
				rows = append(rows, []string{cl.OrganizationName, cl.ConditionName, strconv.Itoa(cl.DeviceCount), strconv.Itoa(cl.AlertCount), cl.FirstSeen, cl.LastSeen})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "1h", "Time bucket width for clustering (e.g. 30m, 1h, 1d)")
	cmd.Flags().IntVar(&flagMinCluster, "min-cluster", 2, "Minimum alerts for a cluster to be reported")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of clusters to return (0 = all)")
	return cmd
}

func isoFromEpoch(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
