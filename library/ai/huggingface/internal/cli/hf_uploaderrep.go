package cli

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

type uploaderRepResponse struct {
	hfx.Envelope
	User            string             `json:"user"`
	Trusted         bool               `json:"trusted_uploader"`
	ModelCount      int                `json:"model_count"`
	TotalDownloads  int                `json:"total_downloads"`
	TopModels       []uploaderTopModel `json:"top_models,omitempty"`
	RecentUploads   []uploaderTopModel `json:"recent_uploads,omitempty"`
	UntrustedReason string             `json:"untrusted_reason,omitempty"`
	Explain         string             `json:"explain,omitempty"`
}

type uploaderTopModel struct {
	ID           string `json:"id"`
	Downloads    int    `json:"downloads"`
	Likes        int    `json:"likes"`
	LastModified string `json:"last_modified"`
}

func newHFUploaderRepCmd(flags *rootFlags) *cobra.Command {
	var topN int
	cmd := &cobra.Command{
		Use:   "uploader-rep <user>",
		Short: "Uploader reputation: aggregate downloads, recency, model count, trusted-uploader badge.",
		Long: `uploader-rep aggregates a user/org's published models on HF and applies the
trusted-uploader allowlist (default: unsloth, bartowski, mradermacher).

Returns total model count, summed downloads, top models by downloads, and
the most-recently-modified uploads. Use to vet a quant uploader before
trusting their work in production.`,
		Example: `  huggingface-pp-cli uploader-rep bartowski
  huggingface-pp-cli uploader-rep mradermacher --top 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			user := args[0]
			ctx := cmd.Context()

			// Pull all of user's models — sort by downloads desc, then we
			// post-sort by lastModified for the recent-uploads slice.
			q := url.Values{}
			q.Set("author", user)
			q.Set("sort", "downloads")
			q.Set("direction", "-1")
			q.Set("limit", strconv.Itoa(500))
			q.Set("full", "false")

			ms, status, err := hfListModels(ctx, q, hfTokenForRequests())
			if err != nil {
				if status == 429 {
					return hfRateLimited("rate limited (HTTP 429)")
				}
				return err
			}
			if len(ms) == 0 {
				return hfNotFound("uploader %q has no models on HF (or hidden by privacy)", user)
			}

			total := 0
			for _, m := range ms {
				total += m.Downloads
			}

			// Top by downloads (already sorted)
			top := []uploaderTopModel{}
			for i, m := range ms {
				if i >= topN {
					break
				}
				top = append(top, uploaderTopModel{
					ID:           m.ID,
					Downloads:    m.Downloads,
					Likes:        m.Likes,
					LastModified: m.LastModified,
				})
			}

			// Recent — re-sort copy by lastModified
			recentCopy := make([]hfModel, len(ms))
			copy(recentCopy, ms)
			sort.Slice(recentCopy, func(i, j int) bool {
				return strings.Compare(recentCopy[i].LastModified, recentCopy[j].LastModified) > 0
			})
			recent := []uploaderTopModel{}
			for i, m := range recentCopy {
				if i >= topN {
					break
				}
				recent = append(recent, uploaderTopModel{
					ID:           m.ID,
					Downloads:    m.Downloads,
					Likes:        m.Likes,
					LastModified: m.LastModified,
				})
			}

			trusted := hfx.IsTrustedUploader(user)
			resp := uploaderRepResponse{
				Envelope:       hfx.NewEnvelope("uploader-rep"),
				User:           user,
				Trusted:        trusted,
				ModelCount:     len(ms),
				TotalDownloads: total,
				TopModels:      top,
				RecentUploads:  recent,
			}
			if !trusted {
				resp.UntrustedReason = "user not in default trusted set (unsloth, bartowski, mradermacher); evaluate based on download count + recency"
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: %s has %d models on HF totaling %d downloads. Trusted-uploader status: %v. Top by downloads + most-recent uploads listed.",
					user, len(ms), total, trusted)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			star := " "
			if trusted {
				star = "* (trusted)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uploader: %s %s\n", user, star)
			fmt.Fprintf(cmd.OutOrStdout(), "  models:    %d\n", resp.ModelCount)
			fmt.Fprintf(cmd.OutOrStdout(), "  downloads: %d (aggregate)\n", resp.TotalDownloads)
			if !trusted {
				fmt.Fprintf(cmd.OutOrStdout(), "  note:      %s\n", resp.UntrustedReason)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\n  Top by downloads:")
			for _, t := range resp.TopModels {
				fmt.Fprintf(cmd.OutOrStdout(), "    %-50s  %d dl  %d likes\n", t.ID, t.Downloads, t.Likes)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\n  Recent uploads:")
			for _, t := range resp.RecentUploads {
				fmt.Fprintf(cmd.OutOrStdout(), "    %-50s  %s\n", t.ID, t.LastModified)
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&topN, "top", 5, "How many top + recent models to surface (default: 5)")
	return cmd
}
