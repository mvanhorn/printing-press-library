// Copyright 2026 klubieniecki and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/store"
)

type ashbyBoardResponse struct {
	APIVersion string          `json:"apiVersion"`
	Jobs       json.RawMessage `json:"jobs"`
}

type ashbyJobPosting struct {
	ID              string             `json:"id"`
	Title           string             `json:"title"`
	Location        string             `json:"location,omitempty"`
	Department      string             `json:"department,omitempty"`
	Team            string             `json:"team,omitempty"`
	IsListed        bool               `json:"isListed"`
	IsRemote        bool               `json:"isRemote"`
	WorkplaceType   string             `json:"workplaceType,omitempty"`
	Description     string             `json:"descriptionPlain,omitempty"`
	DescriptionHTML string             `json:"descriptionHtml,omitempty"`
	PublishedAt     string             `json:"publishedAt,omitempty"`
	EmploymentType  string             `json:"employmentType,omitempty"`
	JobURL          string             `json:"jobUrl"`
	ApplyURL        string             `json:"applyUrl"`
	Compensation    *ashbyCompensation `json:"compensation,omitempty"`
}

type ashbyCompensation struct {
	Summary           string                       `json:"compensationTierSummary,omitempty"`
	SalarySummary     string                       `json:"scrapeableCompensationSalarySummary,omitempty"`
	SummaryComponents []ashbyCompensationComponent `json:"summaryComponents,omitempty"`
}

type ashbyCompensationComponent struct {
	Type         string   `json:"compensationType"`
	Interval     string   `json:"interval,omitempty"`
	CurrencyCode string   `json:"currencyCode,omitempty"`
	MinValue     *float64 `json:"minValue,omitempty"`
	MaxValue     *float64 `json:"maxValue,omitempty"`
}

type ashbyPostingFilter struct {
	Query           string
	Department      string
	Team            string
	Location        string
	Workplace       string
	EmploymentType  string
	Currency        string
	PublishedSince  string
	SalaryMin       float64
	SalaryMax       float64
	Remote          bool
	HasCompensation bool
	Limit           int
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// The generated endpoint mirror can expose isListed=false records. Keep
		// the declared path reachable for generator conformance, but hide and
		// disable it in favor of the visibility-safe postings surface below.
		for _, child := range root.Commands() {
			if child.Name() == "posting-api" {
				child.Hidden = true
				child.RunE = func(cmd *cobra.Command, _ []string) error {
					return usageErr(errors.New("use 'ashby-pp-cli postings list <job-board-name>'; the raw endpoint mirror is disabled to prevent unlisted-job disclosure"))
				}
			}
		}
		addNovelCommandIfAbsent(root, newAshbyPostingsCmd(flags))
		addNovelCommandIfAbsent(root, newAshbySyncCmd(flags))
		addNovelCommandIfAbsent(root, newAshbySearchCmd(flags))
	})
}

func newAshbyPostingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "postings",
		Short:       "Manage public Ashby job postings",
		Long:        "Public Ashby Job Postings API (https://api.ashbyhq.com/posting-api/job-board). Every Ashby customer can expose an open job board with no authentication required.",
		Example:     "  ashby-pp-cli postings list ashby --remote",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAshbyPostingsListCmd(flags), newAshbyPostingsGetCmd(flags))
	return cmd
}

// pp:data-source live
func newAshbyPostingsListCmd(flags *rootFlags) *cobra.Command {
	var filter ashbyPostingFilter
	var includeCompensation bool
	cmd := &cobra.Command{
		Use:          "list <job-board-name>",
		Short:        "List publicly listed jobs for an Ashby company",
		Example:      "  ashby-pp-cli postings list ashby --remote --department Engineering",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Annotations:  map[string]string{"pp:endpoint": "postings.list", "pp:method": "GET", "pp:path": "/posting-api/job-board/{jobBoardName}", "pp:happy-args": "<job-board-name>=ashby", "pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if err := validateAshbyBoardName(args[0]); err != nil {
					return usageErr(err)
				}
				if _, err := parseAshbyPublishedSince(filter.PublishedSince); err != nil {
					return usageErr(err)
				}
				return writeAshbyPostingsDryRun(cmd, flags, "postings list", args[0], "", includeCompensation || filter.HasCompensation || filter.SalaryMin > 0 || filter.SalaryMax > 0 || filter.Currency != "", filter)
			}
			jobs, err := fetchAshbyJobs(cmd, flags, args[0], includeCompensation || filter.HasCompensation || filter.SalaryMin > 0 || filter.SalaryMax > 0 || filter.Currency != "")
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			jobs, err = filterAshbyJobs(jobs, filter)
			if err != nil {
				return usageErr(err)
			}
			return printAshbyJobs(cmd, flags, jobs, "live", args[0])
		},
	}
	addAshbyPostingFilterFlags(cmd, &filter)
	cmd.Flags().BoolVar(&includeCompensation, "include-compensation", false, "Include public compensation data when available")
	return cmd
}

// pp:data-source live
func newAshbyPostingsGetCmd(flags *rootFlags) *cobra.Command {
	var includeCompensation bool
	cmd := &cobra.Command{
		Use:          "get <job-board-name> <posting-id>",
		Short:        "Retrieve one publicly listed Ashby posting",
		Example:      "  ashby-pp-cli postings get ashby 7458d4e9-da2e-47bd-98cb-adfda43d42b2",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		Annotations:  map[string]string{"pp:endpoint": "postings.get", "pp:method": "GET", "pp:path": "/posting-api/job-board/{jobBoardName}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if err := validateAshbyBoardName(args[0]); err != nil {
					return usageErr(err)
				}
				return writeAshbyPostingsDryRun(cmd, flags, "postings get", args[0], args[1], includeCompensation, ashbyPostingFilter{})
			}
			jobs, err := fetchAshbyJobs(cmd, flags, args[0], includeCompensation)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			for _, job := range jobs {
				if job.IsListed && job.ID == args[1] {
					return printAshbyJobs(cmd, flags, []ashbyJobPosting{job}, "live", args[0])
				}
			}
			return notFoundErr(errors.New("posting not found or is not publicly listed"))
		},
	}
	cmd.Flags().BoolVar(&includeCompensation, "include-compensation", false, "Include public compensation data when available")
	return cmd
}

// pp:data-source live
func newAshbySyncCmd(flags *rootFlags) *cobra.Command {
	var includeCompensation bool
	cmd := &cobra.Command{
		Use:          "sync <job-board-name> [job-board-name...]",
		Short:        "Sync publicly listed jobs from one or more Ashby boards",
		Example:      "  ashby-pp-cli sync ashby --include-compensation",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		Annotations:  map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, boards []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("ashby-pp-cli"))
			if err != nil {
				return fmt.Errorf("sync: open local store: %w", err)
			}
			defer db.Close()
			total := 0
			removed := 0
			for _, board := range boards {
				jobs, err := fetchAshbyJobs(cmd, flags, board, includeCompensation)
				if err != nil {
					return fmt.Errorf("sync %s: %w", board, err)
				}
				stored, deleted, err := persistAshbyBoardSnapshot(db, board, jobs)
				if err != nil {
					return fmt.Errorf("sync %s: persist: %w", board, err)
				}
				scoped := "postings:" + strings.ToLower(board)
				if err := db.SaveSyncState(scoped, "", stored); err != nil {
					return fmt.Errorf("sync %s: save state: %w", board, err)
				}
				total += stored
				removed += deleted
			}
			result, _ := json.Marshal(map[string]any{"boards": boards, "stored": total, "removed": removed, "syncedAt": time.Now().UTC().Format(time.RFC3339)})
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), result, flags, map[string]any{"source": "live"})
		},
	}
	cmd.Flags().BoolVar(&includeCompensation, "include-compensation", false, "Include public compensation data when available")
	return cmd
}

func persistAshbyBoardSnapshot(db *store.Store, board string, jobs []ashbyJobPosting) (int, int, error) {
	listed := listedAshbyJobs(jobs)
	items := make([]json.RawMessage, 0, len(listed))
	for _, job := range listed {
		if strings.TrimSpace(job.ID) == "" {
			return 0, 0, errors.New("listed Ashby posting is missing an id")
		}
		raw, err := json.Marshal(job)
		if err != nil {
			return 0, 0, err
		}
		items = append(items, raw)
	}
	scoped := "postings:" + strings.ToLower(board)
	return db.ReplaceGenericSnapshot(scoped, items)
}

// pp:data-source local
func newAshbySearchCmd(flags *rootFlags) *cobra.Command {
	var board string
	var limit int
	cmd := &cobra.Command{
		Use:          "search <query>",
		Short:        "Full-text search across locally synced Ashby jobs",
		Example:      "  ashby-pp-cli search engineering --board ashby --limit 20",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Annotations:  map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenReadOnlyContext(cmd.Context(), defaultDBPath("ashby-pp-cli"))
			if err != nil {
				return fmt.Errorf("open local store: %w; run 'ashby-pp-cli sync <board>' first", err)
			}
			defer db.Close()
			resource := ""
			if board != "" {
				resource = "postings:" + strings.ToLower(board)
			}
			items, err := db.Search(args[0], limit, resource)
			if err != nil {
				return fmt.Errorf("search local jobs: %w", err)
			}
			data, _ := json.Marshal(items)
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "local"})
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Limit results to a synced job board")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")
	return cmd
}

func fetchAshbyJobs(cmd *cobra.Command, flags *rootFlags, board string, includeCompensation bool) ([]ashbyJobPosting, error) {
	if err := validateAshbyBoardName(board); err != nil {
		return nil, usageErr(err)
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := replacePathParam("/posting-api/job-board/{jobBoardName}", "jobBoardName", board)
	params := map[string]string{}
	if includeCompensation {
		params["includeCompensation"] = "true"
	}
	raw, err := c.Get(cmd.Context(), path, params)
	if err != nil {
		return nil, err
	}
	return decodeAshbyBoardJobs(raw)
}

func validateAshbyBoardName(board string) error {
	if strings.TrimSpace(board) == "" || strings.ContainsAny(board, "/?#") {
		return fmt.Errorf("invalid job board name %q", board)
	}
	return nil
}

func writeAshbyPostingsDryRun(cmd *cobra.Command, flags *rootFlags, action, board, postingID string, includeCompensation bool, filter ashbyPostingFilter) error {
	query := map[string]string{}
	if includeCompensation {
		query["includeCompensation"] = "true"
	}
	preview := map[string]any{
		"dry_run": true,
		"action":  action,
		"would":   "fetch the public board and filter listed jobs; no request sent",
		"method":  "GET",
		"path":    replacePathParam("/posting-api/job-board/{jobBoardName}", "jobBoardName", board),
		"query":   query,
		"board":   board,
	}
	if action == "postings get" {
		preview["posting_id"] = postingID
	}
	if action == "postings list" {
		preview["filters"] = map[string]any{
			"query": filter.Query, "department": filter.Department, "team": filter.Team,
			"location": filter.Location, "workplace": filter.Workplace,
			"employment_type": filter.EmploymentType, "currency": filter.Currency,
			"published_since": filter.PublishedSince, "salary_min": filter.SalaryMin,
			"salary_max": filter.SalaryMax, "remote": filter.Remote,
			"has_compensation": filter.HasCompensation, "limit": filter.Limit,
		}
	}
	if flags != nil && flags.asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(preview)
	}
	detail := fmt.Sprintf("filters %v", preview["filters"])
	if action == "postings get" {
		detail = fmt.Sprintf("posting ID %q", postingID)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would GET %s with query %v, then %s by %s; no request sent\n", preview["path"], query, action, detail)
	return err
}

func decodeAshbyBoardJobs(raw json.RawMessage) ([]ashbyJobPosting, error) {
	var response ashbyBoardResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode Ashby job board response: %w", err)
	}
	jobsJSON := strings.TrimSpace(string(response.Jobs))
	if jobsJSON == "" || jobsJSON == "null" {
		return nil, errors.New("decode Ashby job board response: missing complete jobs array")
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(response.Jobs, &rows); err != nil {
		return nil, fmt.Errorf("decode Ashby job board jobs: %w", err)
	}
	for i, row := range rows {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(row, &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("decode Ashby job board jobs: row %d is not a job object", i)
		}
		var id string
		if err := json.Unmarshal(fields["id"], &id); err != nil || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("decode Ashby job board jobs: row %d has no id", i)
		}
		listed := strings.TrimSpace(string(fields["isListed"]))
		if listed != "true" && listed != "false" {
			return nil, fmt.Errorf("decode Ashby job board jobs: row %d has no listing state", i)
		}
	}
	var jobs []ashbyJobPosting
	if err := json.Unmarshal(response.Jobs, &jobs); err != nil {
		return nil, fmt.Errorf("decode Ashby job board jobs: %w", err)
	}
	return jobs, nil
}

func listedAshbyJobs(jobs []ashbyJobPosting) []ashbyJobPosting {
	listed := make([]ashbyJobPosting, 0, len(jobs))
	for _, job := range jobs {
		if job.IsListed {
			listed = append(listed, job)
		}
	}
	return listed
}

func filterAshbyJobs(jobs []ashbyJobPosting, filter ashbyPostingFilter) ([]ashbyJobPosting, error) {
	since, err := parseAshbyPublishedSince(filter.PublishedSince)
	if err != nil {
		return nil, err
	}
	type datedJob struct {
		job       ashbyJobPosting
		published time.Time
		valid     bool
	}
	result := make([]datedJob, 0, len(jobs))
	for _, job := range jobs {
		if !job.IsListed || (filter.Remote && !job.IsRemote) || !containsFold(job.Department, filter.Department) || !containsFold(job.Team, filter.Team) || !containsFold(job.Location, filter.Location) || !containsFold(job.WorkplaceType, filter.Workplace) || !containsFold(job.EmploymentType, filter.EmploymentType) {
			continue
		}
		if filter.Query != "" && !containsFold(strings.Join([]string{job.Title, job.Department, job.Team, job.Location, job.Description}, " "), filter.Query) {
			continue
		}
		published, parseErr := time.Parse(time.RFC3339Nano, job.PublishedAt)
		if !since.IsZero() && (parseErr != nil || published.Before(since)) {
			continue
		}
		if !matchesAshbyCompensation(job.Compensation, filter) {
			continue
		}
		result = append(result, datedJob{job: job, published: published, valid: parseErr == nil})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].valid && result[j].valid {
			return result[i].published.After(result[j].published)
		}
		if result[i].valid || result[j].valid {
			return result[i].valid
		}
		return result[i].job.PublishedAt > result[j].job.PublishedAt
	})
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	filtered := make([]ashbyJobPosting, len(result))
	for i, item := range result {
		filtered[i] = item.job
	}
	return filtered, nil
}

func parseAshbyPublishedSince(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	since, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --published-since value %q: use YYYY-MM-DD", value)
	}
	return since, nil
}

func containsFold(value, wanted string) bool {
	return wanted == "" || strings.Contains(strings.ToLower(value), strings.ToLower(wanted))
}

func matchesAshbyCompensation(comp *ashbyCompensation, filter ashbyPostingFilter) bool {
	if filter.HasCompensation && comp == nil {
		return false
	}
	if filter.Currency == "" && filter.SalaryMin <= 0 && filter.SalaryMax <= 0 {
		return true
	}
	if comp == nil {
		return false
	}
	for _, component := range comp.SummaryComponents {
		if !strings.EqualFold(component.Type, "Salary") || (filter.Currency != "" && !strings.EqualFold(component.CurrencyCode, filter.Currency)) {
			continue
		}
		if filter.SalaryMin > 0 && (component.MaxValue == nil || *component.MaxValue < filter.SalaryMin) {
			continue
		}
		if filter.SalaryMax > 0 && (component.MinValue == nil || *component.MinValue > filter.SalaryMax) {
			continue
		}
		return true
	}
	return false
}

func addAshbyPostingFilterFlags(cmd *cobra.Command, filter *ashbyPostingFilter) {
	cmd.Flags().StringVarP(&filter.Query, "query", "q", "", "Search title, team, department, location, and description")
	cmd.Flags().StringVar(&filter.Department, "department", "", "Filter by department")
	cmd.Flags().StringVar(&filter.Team, "team", "", "Filter by team")
	cmd.Flags().StringVar(&filter.Location, "location", "", "Filter by primary location")
	cmd.Flags().StringVar(&filter.Workplace, "workplace", "", "Filter by workplace type: Remote, Hybrid, or OnSite")
	cmd.Flags().StringVar(&filter.EmploymentType, "employment-type", "", "Filter by employment type")
	cmd.Flags().StringVar(&filter.PublishedSince, "published-since", "", "Only jobs published on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&filter.Currency, "currency", "", "Filter salary components by currency code")
	cmd.Flags().Float64Var(&filter.SalaryMin, "salary-min", 0, "Require a salary range reaching at least this value")
	cmd.Flags().Float64Var(&filter.SalaryMax, "salary-max", 0, "Require a salary range starting at or below this value")
	cmd.Flags().BoolVar(&filter.Remote, "remote", false, "Only remote jobs")
	cmd.Flags().BoolVar(&filter.HasCompensation, "has-compensation", false, "Only jobs with disclosed compensation")
	cmd.Flags().IntVar(&filter.Limit, "limit", 0, "Maximum number of results; 0 means all")
}

func printAshbyJobs(cmd *cobra.Command, flags *rootFlags, jobs []ashbyJobPosting, source, board string) error {
	data, err := json.Marshal(jobs)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": source, "board": board})
}
