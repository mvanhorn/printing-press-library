package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

func newReportCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims, start, end, filter, order string
	var limit int
	c := &cobra.Command{Use: "report", Short: "Raw GA4 runReport wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest(metrics, dims, start, end, limit)
		if err := addRawDimensionFilter(&req, filter); err != nil {
			return fmt.Errorf("--filter must be JSON dimensionFilter: %w", err)
		}
		addOrder(&req, order)
		return runReportOutput(cmd, flags, req, "")
	}}
	reportFlags(c, &metrics, &dims, &start, &end, &limit)
	c.Flags().StringVar(&filter, "filter", "", "Raw JSON dimensionFilter")
	c.Flags().StringVar(&order, "order", "", "Order by metric/dimension name, prefix - for desc")
	return c
}
func newPivotCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims, start, end string
	var limit int
	c := &cobra.Command{Use: "pivot", Short: "Raw GA4 runPivotReport wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		base := reportRequest(metrics, dims, start, end, limit)
		pivotLimit := base.Limit
		base.Limit = ""
		req := ga4.RunPivotReportRequest{RunReportRequest: base}
		for _, d := range splitCSV(dims) {
			req.Pivots = append(req.Pivots, ga4.Pivot{FieldNames: []string{d}, Limit: pivotLimit})
		}
		raw, _, err := cl.RunPivotReport(context.Background(), p, req)
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
	reportFlags(c, &metrics, &dims, &start, &end, &limit)
	return c
}
func newBatchCmd(flags *rootFlags) *cobra.Command {
	var reportsJSON string
	c := &cobra.Command{Use: "batch", Short: "Raw GA4 batchRunReports wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		reports := []ga4.RunReportRequest{reportRequest("sessions,totalUsers", "date", "7daysAgo", "yesterday", 10)}
		if reportsJSON != "" {
			if err := json.Unmarshal([]byte(reportsJSON), &reports); err != nil {
				return fmt.Errorf("--reports must be a JSON array of RunReportRequest objects: %w", err)
			}
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.BatchRunReports(context.Background(), p, ga4.BatchRunReportsRequest{Requests: reports})
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
	c.Flags().StringVar(&reportsJSON, "reports", "", "JSON array of RunReportRequest bodies (property omitted; property comes from --property)")
	return c
}
func newRealtimeCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims string
	var limit int
	c := &cobra.Command{Use: "realtime", Short: "Raw GA4 runRealtimeReport wrapper", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.RunRealtimeReport(context.Background(), p, realtimeRequest(metrics, dims, limit))
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
	c.Flags().StringVar(&metrics, "metrics", "activeUsers", "Comma-separated realtime metrics")
	c.Flags().StringVar(&dims, "dimensions", "unifiedScreenName", "Comma-separated realtime dimensions")
	c.Flags().IntVar(&limit, "limit", 10, "Rows")
	return c
}
func newMetadataCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "metadata", Short: "List GA4 dimensions and metrics for a property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.GetMetadata(context.Background(), p)
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
}
func newCompatibilityCmd(flags *rootFlags) *cobra.Command {
	var metrics, dims string
	c := &cobra.Command{Use: "compatibility", Short: "Check GA4 metric/dimension compatibility", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.CheckCompatibility(context.Background(), p, compatibilityRequest(metrics, dims))
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
	c.Flags().StringVar(&metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Metrics")
	c.Flags().StringVar(&dims, "dimensions", "sessionDefaultChannelGroup", "Dimensions")
	return c
}
func runReportOutput(cmd *cobra.Command, flags *rootFlags, req ga4.RunReportRequest, human string) error {
	p, err := requireProperty(flags)
	if err != nil {
		return err
	}
	cl, _, err := flags.newClient()
	if err != nil {
		return err
	}
	raw, _, err := cl.RunReport(context.Background(), p, req)
	if err != nil {
		return err
	}
	if raw.Raw != nil {
		return output(cmd, flags, raw.Raw, human)
	}
	return output(cmd, flags, raw, human)
}
