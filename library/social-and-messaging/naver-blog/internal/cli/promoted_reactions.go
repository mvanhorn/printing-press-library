// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written implementation of `reactions`. Parses the BLOG[...]
// q parameter shape into typed PostKey values and routes through
// reactionapi.GetReactions, which handles chunking and the JSON
// response shape. Bypasses resolveRead because the reaction endpoint
// is at apis.naver.com — a different host than the base URL — and
// the response shape is bespoke enough that envelope-unwrapping
// helpers can't see the like count.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/reactionapi"
)

// reactionResult is one output row: postId is the API's contentsId
// (e.g. "selly9401_224234460263"), likes is the count when present.
type reactionResult struct {
	PostID string `json:"post_id"`
	BlogID string `json:"blog_id"`
	LogNo  string `json:"log_no"`
	Likes  *int   `json:"likes"`
}

func newReactionsPromotedCmd(flags *rootFlags) *cobra.Command {
	var (
		flagPool          string
		flagQ             string
		flagIsDuplication bool
	)

	cmd := &cobra.Command{
		Use:   "reactions",
		Short: "Get like (공감) counts for one or more Naver Blog posts from the public reaction API.",
		Long: `Call Naver's public reaction-counts endpoint
  (https://apis.naver.com/blogserver/like/v1/search/contents)
to get like (공감) counts for one or more posts in a single batch.

The --q flag uses the same shape as Naver's internal URL:

  BLOG[<blog_id>_<log_no>,<blog_id>_<log_no>,...]

For example:

  --q 'BLOG[selly9401_224234460263,foodie_223999888777]'

Up to 20 keys per call are batched in one request; more than 20 are chunked transparently. Posts that are absent from the response (deleted/private/blog-deleted) appear in the output with likes=null, NOT likes=0, so callers can distinguish "unknown" from "zero".`,
		Example: `  naver-blog-pp-cli reactions --q 'BLOG[selly9401_224234460263]'
  naver-blog-pp-cli reactions --q 'BLOG[a_1,b_2,c_3]' --pool blogid`,
		Annotations: map[string]string{
			"pp:endpoint":   "reactions.get",
			"pp:method":     "GET",
			"pp:path":       "https://apis.naver.com/blogserver/like/v1/search/contents",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before required-flag validation so verify
			// dry-run probes succeed without forcing a sample q value.
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagQ) == "" {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "q is required",
						"usage": fmt.Sprintf("%s --q 'BLOG[<blog_id>_<log_no>,...]'", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(fmt.Errorf("required flag --q not set"))
			}
			keys, err := parseReactionQ(flagQ)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			counts, err := reactionapi.GetReactionsLimited(ctx, c.HTTPClient, c.Limiter(), keys)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			results := make([]reactionResult, 0, len(keys))
			for _, k := range keys {
				postID := k.BlogID + "_" + k.LogNo
				r := reactionResult{
					PostID: postID,
					BlogID: k.BlogID,
					LogNo:  k.LogNo,
				}
				if n, ok := counts[postID]; ok {
					r.Likes = &n
				}
				results = append(results, r)
			}
			// Stable output ordering — sort by the q-input order
			// already preserved above, then by post_id as tiebreaker.
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].PostID < results[j].PostID
			})
			_ = flagPool
			_ = flagIsDuplication
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagPool, "pool", "blogid", "Reaction pool (always 'blogid' for Naver Blog)")
	cmd.Flags().StringVar(&flagQ, "q", "", "Query composed as BLOG[id_logno,id_logno,...]. Required.")
	cmd.Flags().BoolVar(&flagIsDuplication, "dedupe", false, "Naver internal flag (false in production)")
	return cmd
}

// parseReactionQ parses "BLOG[a_1,b_2]" into a list of PostKey
// values. Returns an error when the shape doesn't match.
func parseReactionQ(raw string) ([]reactionapi.PostKey, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "BLOG[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("invalid --q shape %q: expected BLOG[<blog_id>_<log_no>,...]", raw)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(raw, "BLOG["), "]")
	if strings.TrimSpace(inner) == "" {
		return nil, fmt.Errorf("invalid --q %q: no post keys inside the brackets", raw)
	}
	out := make([]reactionapi.PostKey, 0)
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.LastIndex(part, "_")
		if idx <= 0 || idx >= len(part)-1 {
			return nil, fmt.Errorf("invalid post key %q in --q: expected <blog_id>_<log_no>", part)
		}
		out = append(out, reactionapi.PostKey{
			BlogID: part[:idx],
			LogNo:  part[idx+1:],
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid post keys parsed from --q %q", raw)
	}
	return out, nil
}
