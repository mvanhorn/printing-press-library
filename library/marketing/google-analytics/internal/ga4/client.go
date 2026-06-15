// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package ga4

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	HTTPClient *http.Client
	Token      string
	DataBase   string
	AdminBase  string
	AlphaBase  string
}

type APIError struct {
	Status int
	Body   string
}

func (e APIError) Error() string { return fmt.Sprintf("google api status %d: %s", e.Status, e.Body) }

func NewClient(token string, timeout time.Duration) *Client {
	return &Client{HTTPClient: &http.Client{Timeout: timeout}, Token: token, DataBase: "https://analyticsdata.googleapis.com/v1beta", AdminBase: "https://analyticsadmin.googleapis.com/v1beta", AlphaBase: "https://analyticsdata.googleapis.com/v1alpha"}
}

func (c *Client) RunReport(ctx context.Context, property string, req RunReportRequest) (ReportResponse, int, error) {
	var out ReportResponse
	st, err := c.post(ctx, c.dataURL(property, "runReport"), req, &out)
	return out, st, err
}
func (c *Client) RunPivotReport(ctx context.Context, property string, req RunPivotReportRequest) (map[string]any, int, error) {
	var out map[string]any
	st, err := c.post(ctx, c.dataURL(property, "runPivotReport"), req, &out)
	return out, st, err
}
func (c *Client) BatchRunReports(ctx context.Context, property string, req BatchRunReportsRequest) (map[string]any, int, error) {
	var out map[string]any
	st, err := c.post(ctx, c.dataURL(property, "batchRunReports"), req, &out)
	return out, st, err
}
func (c *Client) RunRealtimeReport(ctx context.Context, property string, req RunRealtimeReportRequest) (map[string]any, int, error) {
	var out map[string]any
	st, err := c.post(ctx, c.dataURL(property, "runRealtimeReport"), req, &out)
	return out, st, err
}
func (c *Client) CheckCompatibility(ctx context.Context, property string, req CheckCompatibilityRequest) (map[string]any, int, error) {
	var out map[string]any
	st, err := c.post(ctx, c.dataURL(property, "checkCompatibility"), req, &out)
	return out, st, err
}
func (c *Client) GetMetadata(ctx context.Context, property string) (map[string]any, int, error) {
	var out map[string]any
	st, err := c.get(ctx, fmt.Sprintf("%s/properties/%s/metadata", c.DataBase, url.PathEscape(property)), &out)
	return out, st, err
}
func (c *Client) RunFunnelReport(ctx context.Context, property string, req RunFunnelReportRequest) (map[string]any, int, error) {
	var out map[string]any
	st, err := c.post(ctx, fmt.Sprintf("%s/properties/%s:runFunnelReport", c.AlphaBase, url.PathEscape(property)), req, &out)
	return out, st, err
}
func (c *Client) AccountSummaries(ctx context.Context) (AccountSummariesResponse, int, error) {
	var combined AccountSummariesResponse
	lastStatus := 0
	pageToken := ""
	for {
		target := c.AdminBase + "/accountSummaries?pageSize=200"
		if pageToken != "" {
			target += "&pageToken=" + url.QueryEscape(pageToken)
		}
		var page AccountSummariesResponse
		st, err := c.get(ctx, target, &page)
		lastStatus = st
		if err != nil {
			return combined, st, err
		}
		combined.AccountSummaries = append(combined.AccountSummaries, page.AccountSummaries...)
		if page.NextPageToken == "" {
			return combined, lastStatus, nil
		}
		pageToken = page.NextPageToken
	}
}
func (c *Client) Property(ctx context.Context, property string) (Property, int, error) {
	var out Property
	st, err := c.get(ctx, fmt.Sprintf("%s/properties/%s", c.AdminBase, url.PathEscape(property)), &out)
	return out, st, err
}
func (c *Client) DataStreams(ctx context.Context, property string) (DataStreamsResponse, int, error) {
	var out DataStreamsResponse
	st, err := c.get(ctx, fmt.Sprintf("%s/properties/%s/dataStreams", c.AdminBase, url.PathEscape(property)), &out)
	return out, st, err
}

func (c *Client) dataURL(property, method string) string {
	return fmt.Sprintf("%s/properties/%s:%s", c.DataBase, url.PathEscape(property), method)
}

func (c *Client) get(ctx context.Context, target string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	return c.do(req, out)
}
func (c *Client) post(ctx context.Context, target string, body any, out any) (int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}
func (c *Client) do(req *http.Request, out any) (int, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("User-Agent", "google-analytics-pp-cli/1.1.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, APIError{Status: resp.StatusCode, Body: string(b)}
	}
	if len(strings.TrimSpace(string(b))) == 0 || out == nil {
		return resp.StatusCode, nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return resp.StatusCode, err
	}
	if rr, ok := out.(*ReportResponse); ok {
		_ = json.Unmarshal(b, &rr.Raw)
	}
	return resp.StatusCode, nil
}
