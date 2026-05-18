// PATCH: novel storage-usage aggregation over Storage bucket+object listings (// pp:client-call markers on the real HTTP calls).
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newStorageTopCmd is the top-level `storage` parent. Distinct from the
// spec-derived `projects storage` parent (which targets Management API
// /v1/projects/{ref}/config/storage). This parent hosts cross-bucket
// runtime helpers that hit the project's /storage/v1/... surface.
func newStorageTopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Per-project Storage runtime helpers (usage rollup)",
		Long: `Runtime Storage surface for the project named by SUPABASE_URL. Hits
<ref>.supabase.co/storage/v1/... — distinct from 'projects storage' which
configures storage at the Management-API level.

Object-level CRUD (upload/download/sign/delete) is a documented known gap; use
the Supabase Studio dashboard or supabase-js for now.`,
	}
	cmd.AddCommand(newStorageUsageCmd(flags))
	return cmd
}

func newStorageUsageCmd(flags *rootFlags) *cobra.Command {
	var bucketFilter string
	var maxObjects int

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Per-bucket size and object-count rollup",
		Long: `Lists buckets via Storage GET /storage/v1/bucket, then for each bucket pages
GET /storage/v1/object/list/<bucket> and aggregates file count + total bytes +
largest object. Use --bucket to scope to one bucket.

Uses the project credentials (publishable + optional service_role) configured
via SUPABASE_URL and SUPABASE_PUBLISHABLE_KEY / SUPABASE_SERVICE_ROLE_KEY.`,
		Example: strings.Trim(`
  # Total usage across all buckets
  supabase-pp-cli storage usage --json

  # Single bucket
  supabase-pp-cli storage usage --bucket avatars --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ps, err := newProjectSurface(false)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			// Step 1: list buckets (requires service_role or RLS-permissive policy).
			// pp:client-call — real Storage GET via project surface (https://<ref>.supabase.co/storage/v1/bucket)
			body, _, err := ps.do(ctx, "GET", "/storage/v1/bucket", nil, true)
			if err != nil {
				return apiErr(fmt.Errorf("listing buckets: %w", err))
			}
			var buckets []struct {
				Name      string `json:"name"`
				ID        string `json:"id"`
				Public    bool   `json:"public"`
				CreatedAt string `json:"created_at"`
				UpdatedAt string `json:"updated_at"`
			}
			if err := json.Unmarshal(body, &buckets); err != nil {
				return fmt.Errorf("parsing bucket list: %w", err)
			}

			type bucketUsage struct {
				Name          string `json:"name"`
				ID            string `json:"id"`
				Public        bool   `json:"public"`
				ObjectCount   int    `json:"object_count"`
				TotalBytes    int64  `json:"total_bytes"`
				LargestObject string `json:"largest_object"`
				LargestBytes  int64  `json:"largest_bytes"`
				Truncated     bool   `json:"truncated,omitempty"`
			}
			var results []bucketUsage

			for _, b := range buckets {
				if bucketFilter != "" && b.Name != bucketFilter && b.ID != bucketFilter {
					continue
				}
				usage := bucketUsage{Name: b.Name, ID: b.ID, Public: b.Public}
				// POST /storage/v1/object/list/<bucket> with {prefix:"", limit:N, offset:0}.
				// Bucket names may contain characters that require percent-encoding
				// (spaces, '+', '#'); url.PathEscape keeps the request URL valid.
				reqBody := fmt.Sprintf(`{"prefix":"","limit":%d,"offset":0}`, maxObjects)
				path := fmt.Sprintf("/storage/v1/object/list/%s", url.PathEscape(b.Name))
				// pp:client-call — real Storage POST via project surface (object list per bucket)
				listBody, _, listErr := ps.do(ctx, "POST", path, strings.NewReader(reqBody), true)
				if listErr != nil {
					usage.Truncated = true
					results = append(results, usage)
					continue
				}
				var objects []struct {
					Name     string `json:"name"`
					Metadata struct {
						Size int64 `json:"size"`
					} `json:"metadata"`
				}
				if err := json.Unmarshal(listBody, &objects); err != nil {
					usage.Truncated = true
					results = append(results, usage)
					continue
				}
				if len(objects) >= maxObjects {
					usage.Truncated = true
				}
				for _, o := range objects {
					usage.ObjectCount++
					usage.TotalBytes += o.Metadata.Size
					if o.Metadata.Size > usage.LargestBytes {
						usage.LargestBytes = o.Metadata.Size
						usage.LargestObject = o.Name
					}
				}
				results = append(results, usage)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"bucket_filter": bucketFilter,
					"bucket_count":  len(results),
					"buckets":       results,
				}, flags)
			}
			if len(results) == 0 {
				if bucketFilter != "" {
					fmt.Fprintf(out, "Bucket %q not found.\n", bucketFilter)
				} else {
					fmt.Fprintln(out, "No buckets found in this project.")
				}
				return nil
			}
			fmt.Fprintf(out, "Storage usage (%d bucket(s)):\n\n", len(results))
			fmt.Fprintf(out, "%-25s %-7s %-12s %s\n", "BUCKET", "OBJECTS", "BYTES", "LARGEST")
			fmt.Fprintf(out, "%-25s %-7s %-12s %s\n", "------", "-------", "-----", "-------")
			var grandObjects int
			var grandBytes int64
			for _, r := range results {
				suffix := ""
				if r.Truncated {
					suffix = " (truncated)"
				}
				fmt.Fprintf(out, "%-25s %-7d %-12d %s%s\n", truncate(r.Name, 23), r.ObjectCount, r.TotalBytes, truncate(r.LargestObject, 30), suffix)
				grandObjects += r.ObjectCount
				grandBytes += r.TotalBytes
			}
			fmt.Fprintf(out, "\nTotal: %d object(s) / %d bytes (%.2f MB)\n", grandObjects, grandBytes, float64(grandBytes)/1024/1024)
			return nil
		},
	}

	cmd.Flags().StringVar(&bucketFilter, "bucket", "", "Scope to one bucket (by name or id)")
	cmd.Flags().IntVar(&maxObjects, "max-objects", 1000, "Max objects to list per bucket (pages stop once reached)")
	return cmd
}
