// Copyright 2026 beetz12. Licensed under Apache-2.0.
// Job context: your care.com job posts are the context every message is sent
// under (careNeedIdentifier). `find` requires a --job so searches are tied to a
// hiring need; that job becomes the "active job" that messaging defaults to.
// Hand-authored; safe across regen.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// careCtx returns a request-timeout-bounded context for a command.
func careCtx(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	to := flags.timeout
	if to <= 0 {
		to = 30 * time.Second
	}
	return context.WithTimeout(cmd.Context(), to)
}

type careJob struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	ServiceType string `json:"serviceType,omitempty"`
	Applicants  int    `json:"applicants"`
}

func careMySubject(ctx context.Context, flags *rootFlags) (string, error) {
	data, err := careGraphQL(ctx, flags, careQLoggedInUserOp, careQLoggedInUser, map[string]any{})
	if err != nil {
		return "", err
	}
	var w struct {
		U struct {
			Subject string `json:"subject"`
		} `json:"loggedInUser"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return "", err
	}
	if w.U.Subject == "" {
		return "", fmt.Errorf("could not read your care.com user id; run: care-pp-cli auth refresh")
	}
	return w.U.Subject, nil
}

func careListJobs(ctx context.Context, flags *rootFlags) ([]careJob, error) {
	subj, err := careMySubject(ctx, flags)
	if err != nil {
		return nil, err
	}
	vars := map[string]any{
		"seekerUuid": subj,
		"jobFilter":  map[string]any{"jobStatuses": []string{"OPEN", "CLOSED", "SUPPRESSED"}},
	}
	data, err := careGraphQL(ctx, flags, careQJobsBySeekerOp, careQJobsBySeeker, vars)
	if err != nil {
		return nil, err
	}
	var w struct {
		J struct {
			Edges []struct {
				Node struct {
					ID                string `json:"id"`
					Title             string `json:"title"`
					Status            string `json:"status"`
					ServiceType       string `json:"serviceType"`
					ApplicationCounts struct {
						Total int `json:"total"`
					} `json:"applicationCounts"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"jobsBySeekerUUID"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	out := []careJob{}
	for _, e := range w.J.Edges {
		out = append(out, careJob{ID: e.Node.ID, Title: e.Node.Title, Status: e.Node.Status,
			ServiceType: e.Node.ServiceType, Applicants: e.Node.ApplicationCounts.Total})
	}
	return out, nil
}

// --- active job persistence ---

func careActiveJobPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".care-pp-cli", "active-job.json")
}

func careReadActiveJob() (careJob, bool) {
	b, err := os.ReadFile(careActiveJobPath())
	if err != nil {
		return careJob{}, false
	}
	var j careJob
	if err := json.Unmarshal(b, &j); err != nil || j.ID == "" {
		return careJob{}, false
	}
	return j, true
}

func careWriteActiveJob(j careJob) error {
	p := careActiveJobPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(j)
	return os.WriteFile(p, b, 0o600)
}

// careResolveJob resolves a job by name (title substring) or numeric id. An
// empty ref falls back to the active job, then to the sole job if only one.
func careResolveJob(ctx context.Context, flags *rootFlags, ref string) (careJob, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		if j, ok := careReadActiveJob(); ok {
			return j, nil
		}
		jobs, err := careListJobs(ctx, flags)
		if err != nil {
			return careJob{}, err
		}
		if len(jobs) == 1 {
			return jobs[0], nil
		}
		return careJob{}, fmt.Errorf("no job specified and no active job set; pass --job <name|id> (see: care-pp-cli job list)")
	}
	jobs, err := careListJobs(ctx, flags)
	if err != nil {
		return careJob{}, err
	}
	if _, e := strconv.Atoi(ref); e == nil {
		for _, j := range jobs {
			if j.ID == ref {
				return j, nil
			}
		}
		return careJob{}, fmt.Errorf("job id %s is not one of your jobs (see: care-pp-cli job list)", ref)
	}
	var matches []careJob
	for _, j := range jobs {
		if strings.Contains(strings.ToLower(j.Title), strings.ToLower(ref)) {
			matches = append(matches, j)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return careJob{}, fmt.Errorf("no job matching %q (see: care-pp-cli job list)", ref)
	default:
		return careJob{}, fmt.Errorf("%q matches %d jobs; be more specific or use the id (see: care-pp-cli job list)", ref, len(matches))
	}
}

func newCareJobCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Your care.com job posts (the context messages are sent under)",
		Long:  "List your job posts and set the active one. Every message the CLI sends goes out under a job; `find` sets the active job, and messaging defaults to it.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCareJobListCmd(flags))
	cmd.AddCommand(newCareJobUseCmd(flags))
	return cmd
}

func newCareJobListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List your care.com job posts",
		Example:     "  care-pp-cli job list\n  care-pp-cli job list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := careCtx(cmd, flags)
			defer cancel()
			jobs, err := careListJobs(ctx, flags)
			if err != nil {
				return err
			}
			active, _ := careReadActiveJob()
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), jobs, flags)
			}
			w := cmd.OutOrStdout()
			if len(jobs) == 0 {
				fmt.Fprintln(w, "No job posts found.")
				return nil
			}
			for _, j := range jobs {
				marker := " "
				if j.ID == active.ID {
					marker = "*"
				}
				fmt.Fprintf(w, "%s %-10s %-8s %3d applicants  %s\n", marker, j.ID, j.Status, j.Applicants, j.Title)
			}
			fmt.Fprintln(w, "\n('*' = active job used for messaging; set with: care-pp-cli job use <name|id>)")
			return nil
		},
	}
}

func newCareJobUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "use <name or id>",
		Short:   "Set the active job (messaging sends under it)",
		Example: "  care-pp-cli job use \"Summer Sitter\"",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := careCtx(cmd, flags)
			defer cancel()
			j, err := careResolveJob(ctx, flags, strings.Join(args, " "))
			if err != nil {
				return err
			}
			if err := careWriteActiveJob(j); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active job set: %s (%s)\n", j.Title, j.ID)
			return nil
		},
	}
}
