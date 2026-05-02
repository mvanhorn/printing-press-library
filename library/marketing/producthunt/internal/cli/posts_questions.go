package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newPostsQuestionsCmd(flags *rootFlags) *cobra.Command {
	var (
		count    int
		minVotes int
	)
	cmd := &cobra.Command{
		Use:   "questions <slug>",
		Short: "Filter a launch's comments to only those that look like genuine questions",
		Long: strings.Trim(`
Pulls comments for a launch and filters to those that look like genuine
questions: the body contains a question mark and at least one common
question verb ("how", "what", "why", "when", "can it", "does it",
"will it", "should I"). Ranked by vote count.

Use this on launch day or the morning after to identify which comments
deserve a real reply versus cheerleading or spam.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts questions notion --count 50 --json
  producthunt-pp-cli posts questions my-launch --min-votes 1
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{"slug": args[0], "first": count, "order": "VOTES_COUNT"}
			var resp phgql.PostCommentsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostCommentsQuery, vars, &resp); err != nil {
				return err
			}
			questions := make([]questionMatch, 0, len(resp.Post.Comments.Edges))
			for _, e := range resp.Post.Comments.Edges {
				if e.Node.VotesCount < minVotes {
					continue
				}
				if isQuestion(e.Node.Body) {
					questions = append(questions, questionMatch{
						ID:         e.Node.ID,
						VotesCount: e.Node.VotesCount,
						Body:       stripHTML(e.Node.Body),
						CreatedAt:  e.Node.CreatedAt.Format("2006-01-02T15:04:05Z"),
					})
				}
			}
			sort.Slice(questions, func(i, j int) bool { return questions[i].VotesCount > questions[j].VotesCount })

			out := questionsOut{
				PostName:  resp.Post.Name,
				PostID:    resp.Post.ID,
				Total:     len(resp.Post.Comments.Edges),
				Questions: questions,
				Note:      "Commenter identities are redacted by Product Hunt; the comment bodies and votes are intact.",
			}
			if !flags.asJSON && !flags.agent {
				out.Note += " Output rendered as JSON; pass --json to make it explicit."
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 50, "Number of comments to scan (max 50 per page)")
	cmd.Flags().IntVar(&minVotes, "min-votes", 0, "Skip comments under this vote count")
	return cmd
}

type questionMatch struct {
	ID         string `json:"id"`
	VotesCount int    `json:"votes_count"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

type questionsOut struct {
	PostName  string          `json:"post_name"`
	PostID    string          `json:"post_id"`
	Total     int             `json:"total_scanned"`
	Questions []questionMatch `json:"questions"`
	Note      string          `json:"note"`
}

var (
	questionVerbs = regexp.MustCompile(`(?i)\b(how|what|why|when|where|who|which|can|could|does|do|is it|will it|should i|would|won't|isn't|am i|any way|anyone|workaround)\b`)
	hasQMark      = regexp.MustCompile(`\?`)
	htmlTags      = regexp.MustCompile(`<[^>]+>`)
)

func isQuestion(body string) bool {
	plain := stripHTML(body)
	return hasQMark.MatchString(plain) && questionVerbs.MatchString(plain)
}

func stripHTML(s string) string {
	return strings.TrimSpace(htmlTags.ReplaceAllString(s, " "))
}

// suppress unused
var _ = fmt.Sprintf
