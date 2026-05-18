// New endpoints integrated from chirantan/PR-325's substack CLI (kept as Go
// implementations under our existing resource-group naming where it fits, or
// under new top-level commands where there's no good home). All endpoints
// verified against Substack's live API.
//
// PATCH: integrated-from-pr325 — imports seven reachable API features from
// chirantan's PR #325 (drafts prepublish, tags list/create, publications
// authors, posts archive, posts ranked-authors, profiles posts,
// profiles from-linkedin, comments get) so this CLI does not regress on
// features that worked there. Recorded in .printing-press-patches.json.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// ---- posts archive ----

func newPostsArchiveCmd(flags *rootFlags) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Public archive of a publication's posts.",
		Long: `GET /api/v1/archive — returns the public post archive for the publication
named by --subdomain. Works for any public publication, not just your own.`,
		Example:     "  substack-creator-pp-cli posts archive --subdomain mypub --limit 20 --json",
		Annotations: map[string]string{"pp:endpoint": "posts.archive", "pp:method": "GET", "pp:path": "/archive", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if limit > 0 {
				params["limit"] = strconv.Itoa(limit)
			}
			if offset > 0 {
				params["offset"] = strconv.Itoa(offset)
			}
			data, err := c.Get("/archive", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum posts to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

// ---- posts ranked-authors ----

func newPostsRankedAuthorsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ranked-authors",
		Short: "Ranked list of authors for a publication.",
		Long: `GET /api/v1/publication/users/ranked — top-ranked authors of the
publication named by --subdomain.`,
		Example:     "  substack-creator-pp-cli posts ranked-authors --subdomain mypub --json",
		Annotations: map[string]string{"pp:endpoint": "posts.ranked-authors", "pp:method": "GET", "pp:path": "/publication/users/ranked", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/publication/users/ranked", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

// ---- publications authors ----

func newPublicationsAuthorsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "authors",
		Short: "List bylined authors of a publication.",
		Long: `GET /api/v1/publication/users — every user with a byline on the
publication named by --subdomain.`,
		Example:     "  substack-creator-pp-cli publications authors --subdomain mypub --json",
		Annotations: map[string]string{"pp:endpoint": "publications.authors", "pp:method": "GET", "pp:path": "/publication/users", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/publication/users", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

// ---- profiles posts ----

func newProfilesPostsCmd(flags *rootFlags) *cobra.Command {
	var profileUserID, profileHandle string
	var limit int
	cmd := &cobra.Command{
		Use:   "posts",
		Short: "All posts by a user across publications.",
		Long: `GET /api/v1/profile/posts?profile_user_id=N — lists every post the
named user has authored across every publication they belong to. Specify the
user via --user-id (numeric) or pass a handle via --handle to resolve it first.`,
		Example: `  substack-creator-pp-cli profiles posts --user-id 12345 --limit 10 --json
  substack-creator-pp-cli profiles posts --handle someone --json`,
		Annotations: map[string]string{"pp:endpoint": "profiles.posts", "pp:method": "GET", "pp:path": "/profile/posts", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if profileUserID == "" && profileHandle == "" {
				return usageErr(fmt.Errorf("--user-id or --handle is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// If a handle was given, resolve to numeric id first.
			if profileUserID == "" && profileHandle != "" {
				raw, err := c.Get("/user/"+profileHandle+"/public_profile", nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var prof struct {
					ID int64 `json:"id"`
				}
				if json.Unmarshal(raw, &prof) == nil && prof.ID > 0 {
					profileUserID = strconv.FormatInt(prof.ID, 10)
				} else {
					return fmt.Errorf("could not resolve --handle %q to a user_id", profileHandle)
				}
			}
			params := map[string]string{"profile_user_id": profileUserID}
			if limit > 0 {
				params["limit"] = strconv.Itoa(limit)
			}
			data, err := c.Get("/profile/posts", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&profileUserID, "user-id", "", "Substack user_id (numeric)")
	cmd.Flags().StringVar(&profileHandle, "handle", "", "Substack handle (resolved to user_id)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max posts to return")
	return cmd
}

// ---- profiles from-linkedin ----

func newProfilesFromLinkedinCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-linkedin <linkedin-handle>",
		Short: "Look up a Substack profile from a LinkedIn handle.",
		Long: `GET /api/v1/profile/search/linkedin/{handle} — Substack's own
LinkedIn-to-profile lookup. Useful for finding a writer's Substack from their
LinkedIn presence.`,
		Example:     "  substack-creator-pp-cli profiles from-linkedin someone --json",
		Annotations: map[string]string{"pp:endpoint": "profiles.from-linkedin", "pp:method": "GET", "pp:path": "/profile/search/linkedin/{handle}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/profile/search/linkedin/"+args[0], nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

// ---- comments get (single comment) ----

func newCommentsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <comment-id>",
		Short: "Get a single comment by ID (also works for Notes — same shape).",
		Long: `GET /api/v1/reader/comment/{id} — fetches a single comment. Substack treats
Notes as comments under the hood, so this endpoint works for Note IDs too.`,
		Example:     "  substack-creator-pp-cli comments get 12345 --json",
		Annotations: map[string]string{"pp:endpoint": "comments.get", "pp:method": "GET", "pp:path": "/reader/comment/{id}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/reader/comment/"+args[0], nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}
