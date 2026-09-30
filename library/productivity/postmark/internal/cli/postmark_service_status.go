// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

// Postmark's public status page API needs no credentials. The legacy
// /api/1.0 endpoints used by older clients return 404; /api/v1 is the API
// documented at status.postmarkapp.com/api.
var serviceStatusBaseURL = "https://status.postmarkapp.com"

// serviceStatusMaxAttempts bounds retries on 429 and 5xx per request.
var serviceStatusMaxAttempts = 3

const (
	serviceStatusMaxPages     = 5
	serviceStatusMaxBodyBytes = 4 << 20

	statusStateOperational = "operational"
	noticeTypePlanned      = "planned"
)

// serviceStatusClient is a small sibling client for the status page. It is
// separate from the Postmark API client because the host, auth, and error
// shapes differ.
type serviceStatusClient struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	base    string
}

func newServiceStatusClient(timeout time.Duration) *serviceStatusClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &serviceStatusClient{
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(2),
		base:    strings.TrimRight(serviceStatusBaseURL, "/"),
	}
}

// getJSON fetches base+path into v. Exhausted 429 retries surface as
// *cliutil.RateLimitError so a throttled check is never reported as "no
// incidents".
func (s *serviceStatusClient) getJSON(ctx context.Context, path string, v any) error {
	target := s.base + path
	attempts := serviceStatusMaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if err := s.limiter.Wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "postmark-pp-cli")
		// pp:client-call
		resp, err := s.http.Do(req)
		if err != nil {
			return fmt.Errorf("GET %s: %w", target, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, serviceStatusMaxBodyBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", target, readErr)
		}
		ctype := resp.Header.Get("Content-Type")
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			s.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			if attempt+1 < attempts {
				if err := sleepCtx(ctx, wait); err != nil {
					return err
				}
				continue
			}
			return &cliutil.RateLimitError{URL: target, RetryAfter: wait, Body: truncate(strings.TrimSpace(string(body)), 200)}
		case resp.StatusCode >= 500 && attempt+1 < attempts:
			if err := sleepCtx(ctx, cliutil.Backoff(attempt)); err != nil {
				return err
			}
			continue
		case resp.StatusCode != http.StatusOK:
			return fmt.Errorf("GET %s returned HTTP %d (%s)", target, resp.StatusCode, ctype)
		case !strings.Contains(strings.ToLower(ctype), "json"):
			return fmt.Errorf("GET %s returned %s instead of JSON", target, ctype)
		}
		s.limiter.OnSuccess()
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("parsing %s: %w", target, err)
		}
		return nil
	}
	return fmt.Errorf("GET %s: retries exhausted", target)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type statusPageResponse struct {
	Page struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		StateText string `json:"state_text"`
		URL       string `json:"url"`
		UpdatedAt string `json:"updated_at"`
	} `json:"page"`
}

type statusListMeta struct {
	NextPage *string `json:"next_page"`
}

type statusComponent struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	State    string `json:"state"`
	ParentID *int64 `json:"parent_id"`
	Position int    `json:"position"`
}

type statusNotice struct {
	ID            int64  `json:"id"`
	Type          string `json:"type"`
	State         string `json:"state"`
	TimelineState string `json:"timeline_state"`
	Subject       string `json:"subject"`
	URL           string `json:"url"`
	BeganAt       string `json:"began_at"`
	BeginsAt      string `json:"begins_at"`
	EndsAt        string `json:"ends_at"`
	LatestUpdate  *struct {
		State     string `json:"state"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	} `json:"latest_update"`
}

type serviceStatusComponent struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Parent string `json:"parent,omitempty"`
}

type serviceStatusNotice struct {
	ID             int64  `json:"id"`
	Type           string `json:"type"`
	State          string `json:"state"`
	Subject        string `json:"subject"`
	URL            string `json:"url"`
	BeganAt        string `json:"began_at,omitempty"`
	EndsAt         string `json:"ends_at,omitempty"`
	LatestUpdate   string `json:"latest_update,omitempty"`
	LatestUpdateAt string `json:"latest_update_at,omitempty"`
}

type serviceStatusView struct {
	State               string                   `json:"state"`
	StateText           string                   `json:"state_text"`
	Operational         bool                     `json:"operational"`
	PageURL             string                   `json:"page_url"`
	UpdatedAt           string                   `json:"updated_at"`
	DegradedComponents  []serviceStatusComponent `json:"degraded_components"`
	Components          []serviceStatusComponent `json:"components"`
	OpenIncidents       []serviceStatusNotice    `json:"open_incidents"`
	MaintenanceUnderway []serviceStatusNotice    `json:"maintenance_underway"`
	CheckedAt           string                   `json:"checked_at"`
	Source              string                   `json:"source"`
}

// nextStatusPath keeps pagination on the status host: next_page may be a
// relative path or an absolute URL on the same host.
func nextStatusPath(next *string) string {
	if next == nil || strings.TrimSpace(*next) == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(*next))
	if err != nil || (u.Host != "" && !strings.EqualFold(u.Host, "status.postmarkapp.com")) {
		return ""
	}
	path := u.EscapedPath()
	if !strings.HasPrefix(path, "/api/") {
		return ""
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path
}

func (s *serviceStatusClient) components(ctx context.Context) ([]statusComponent, error) {
	out := make([]statusComponent, 0)
	path := "/api/v1/components"
	for page := 0; page < serviceStatusMaxPages && path != ""; page++ {
		var resp struct {
			Components []statusComponent `json:"components"`
			Meta       statusListMeta    `json:"meta"`
		}
		if err := s.getJSON(ctx, path, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.Components...)
		path = nextStatusPath(resp.Meta.NextPage)
	}
	return out, nil
}

func (s *serviceStatusClient) currentNotices(ctx context.Context) ([]statusNotice, error) {
	out := make([]statusNotice, 0)
	path := "/api/v1/notices?" + url.Values{"filter[timeline_state_eq]": {"present"}}.Encode()
	for page := 0; page < serviceStatusMaxPages && path != ""; page++ {
		var resp struct {
			Notices []statusNotice `json:"notices"`
			Meta    statusListMeta `json:"meta"`
		}
		if err := s.getJSON(ctx, path, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.Notices...)
		path = nextStatusPath(resp.Meta.NextPage)
	}
	return out, nil
}

// buildServiceStatusView assembles the report. Components carry their parent
// name; notices are split into incidents and underway maintenance.
func buildServiceStatusView(page statusPageResponse, comps []statusComponent, notices []statusNotice, now time.Time) serviceStatusView {
	view := serviceStatusView{
		State:               page.Page.State,
		StateText:           page.Page.StateText,
		Operational:         page.Page.State == statusStateOperational,
		PageURL:             page.Page.URL,
		UpdatedAt:           page.Page.UpdatedAt,
		DegradedComponents:  make([]serviceStatusComponent, 0),
		Components:          make([]serviceStatusComponent, 0, len(comps)),
		OpenIncidents:       make([]serviceStatusNotice, 0),
		MaintenanceUnderway: make([]serviceStatusNotice, 0),
		CheckedAt:           now.UTC().Format(time.RFC3339),
		Source:              serviceStatusBaseURL + "/api/v1",
	}
	names := map[int64]string{}
	for _, c := range comps {
		names[c.ID] = c.Name
	}
	sorted := append([]statusComponent(nil), comps...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
	for _, c := range sorted {
		row := serviceStatusComponent{ID: c.ID, Name: c.Name, State: c.State}
		if c.ParentID != nil {
			row.Parent = names[*c.ParentID]
		}
		view.Components = append(view.Components, row)
		if c.State != "" && c.State != statusStateOperational {
			view.DegradedComponents = append(view.DegradedComponents, row)
		}
	}
	for _, n := range notices {
		row := serviceStatusNotice{ID: n.ID, Type: n.Type, State: n.State, Subject: n.Subject, URL: n.URL, BeganAt: n.BeganAt, EndsAt: n.EndsAt}
		if row.BeganAt == "" {
			row.BeganAt = n.BeginsAt
		}
		if n.LatestUpdate != nil {
			row.LatestUpdate, row.LatestUpdateAt = n.LatestUpdate.Content, n.LatestUpdate.CreatedAt
		}
		if n.Type == noticeTypePlanned {
			view.MaintenanceUnderway = append(view.MaintenanceUnderway, row)
			continue
		}
		view.OpenIncidents = append(view.OpenIncidents, row)
	}
	return view
}

func newServiceStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-status",
		Short: "Show Postmark's platform status: overall state, degraded components, and open incidents",
		Long: strings.Trim(`
Answer "is it Postmark or us?" during a sending problem. Reads Postmark's
public status page API (no token needed) and reports the overall state,
every component that is not operational, open incidents, and maintenance
that is underway right now. A rate-limited or unreachable status page is
reported as an error, never as "all clear".`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli service-status --json
  postmark-pp-cli service-status --agent --select state,open_incidents`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "service-status")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			sc := newServiceStatusClient(flags.timeout)
			var page statusPageResponse
			if err := sc.getJSON(ctx, "/api/v1/status", &page); err != nil {
				return serviceStatusErr(err)
			}
			comps, err := sc.components(ctx)
			if err != nil {
				return serviceStatusErr(err)
			}
			notices, err := sc.currentNotices(ctx)
			if err != nil {
				return serviceStatusErr(err)
			}
			view := buildServiceStatusView(page, comps, notices, time.Now())
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "parent", "latest_update", "ends_at")
			}
			printServiceStatusHuman(cmd.OutOrStdout(), view)
			return nil
		},
	}
	return cmd
}

func serviceStatusErr(err error) error {
	if isRateLimited(err) {
		return rateLimitErr(err)
	}
	return apiErr(fmt.Errorf("postmark status page: %w", err))
}

func printServiceStatusHuman(w io.Writer, v serviceStatusView) {
	fmt.Fprintf(w, "Postmark status: %s (%s)\n", v.StateText, v.State)
	if len(v.DegradedComponents) == 0 {
		fmt.Fprintf(w, "All %d components operational.\n", len(v.Components))
	} else {
		fmt.Fprintln(w, "Components not operational:")
		for _, c := range v.DegradedComponents {
			name := c.Name
			if c.Parent != "" {
				name = c.Parent + " / " + c.Name
			}
			fmt.Fprintf(w, "  %s: %s\n", name, c.State)
		}
	}
	if len(v.OpenIncidents) == 0 {
		fmt.Fprintln(w, "No open incidents.")
	}
	for _, n := range v.OpenIncidents {
		fmt.Fprintf(w, "Incident: %s [%s] since %s\n  %s\n  %s\n", n.Subject, n.State, n.BeganAt, n.LatestUpdate, n.URL)
	}
	for _, n := range v.MaintenanceUnderway {
		fmt.Fprintf(w, "Maintenance: %s [%s] until %s\n  %s\n", n.Subject, n.State, n.EndsAt, n.URL)
	}
	fmt.Fprintf(w, "Source: %s (checked %s)\n", v.Source, v.CheckedAt)
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newServiceStatusCmd(flags))
	})
}
