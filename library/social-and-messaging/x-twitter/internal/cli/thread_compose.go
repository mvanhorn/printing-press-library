// thread compose — split a markdown document into a numbered tweet thread.
//
// Default: print preview. Pass --post to chain-post via the tweets endpoint.
// The splitter budgets the "(N/M)" numbering suffix BEFORE packing so the
// final per-tweet length stays within the limit. Code fences, paragraphs,
// and list items are atom boundaries; we never split inside a code fence.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/cliutil"
	"github.com/spf13/cobra"
)

const tweetCharLimit = 280

// postedTweet records one chain-posted tweet so --post can render a result.
type postedTweet struct {
	Index int    `json:"index"`
	Total int    `json:"total"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

func newNovelThreadComposeCmd(flags *rootFlags) *cobra.Command {
	var post bool
	cmd := &cobra.Command{
		Use:     "compose <markdown-file>",
		Short:   "Split a markdown file into a numbered tweet thread",
		Long:    "Pack the markdown into ≤280-character tweets, honoring atom boundaries (paragraphs, list items, code fences). Default behavior is dry-run; pass --post to chain-post the thread via the tweets endpoint.",
		Example: "  x-twitter-pp-cli thread compose draft.md\n  x-twitter-pp-cli thread compose draft.md --post",
		Args:    cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true", // dry-run is read-only; --post is gated and will require permission
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify/dry-run probes must not require the markdown file to exist:
			// short-circuit before any filesystem read (the no---post default
			// below is the real preview path for actual use).
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			parts, err := SplitForThread(string(data), tweetCharLimit)
			if err != nil {
				return err
			}
			if !post {
				return previewThread(cmd.OutOrStdout(), parts, flags)
			}
			// Side-effect floor: the verifier sets PRINTING_PRESS_VERIFY=1 in
			// mock-mode subprocesses; never dial out under it.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "verify-env: skipping thread post")
				return nil
			}
			return postThread(cmd, parts, flags)
		},
	}
	cmd.Flags().BoolVar(&post, "post", false, "Actually post the thread (default: dry-run)")
	return cmd
}

// SplitForThread packs markdown into a numbered thread.
// limit is the per-tweet character budget; the "(N/M)" suffix is reserved up front.
func SplitForThread(md string, limit int) ([]string, error) {
	atoms, err := mdAtoms(md)
	if err != nil {
		return nil, err
	}
	if len(atoms) == 0 {
		return nil, fmt.Errorf("no content")
	}
	count := 1
	var parts []string
	converged := false
	// Iterate to a fixed point: more parts → wider numbering → smaller budget.
	for i := 0; i < 6; i++ {
		suffixLen := utf8.RuneCountInString(fmt.Sprintf(" (%d/%d)", count, count))
		budget := limit - suffixLen
		if budget < 50 {
			return nil, fmt.Errorf("limit %d too small with thread numbering", limit)
		}
		parts = packAtoms(atoms, budget)
		if len(parts) == count {
			converged = true
			break
		}
		count = len(parts)
	}
	if !converged {
		// Pathological atom sizes kept the loop oscillating. The parts above
		// were packed against a stale count, so the (i/N) suffix could be wider
		// than budgeted and push a tweet over the limit. Re-pack once with the
		// budget sized for the actual final count; numbering is then guaranteed
		// to fit even if the split is slightly suboptimal.
		suffixLen := utf8.RuneCountInString(fmt.Sprintf(" (%d/%d)", len(parts), len(parts)))
		budget := limit - suffixLen
		if budget < 50 {
			return nil, fmt.Errorf("limit %d too small with thread numbering", limit)
		}
		parts = packAtoms(atoms, budget)
	}
	return parts, nil
}

func mdAtoms(md string) ([]string, error) {
	var atoms []string
	var cur []string
	inFence := false
	flush := func() {
		if len(cur) == 0 {
			return
		}
		atoms = append(atoms, strings.TrimRight(strings.Join(cur, "\n"), "\n"))
		cur = nil
	}
	scanner := bufio.NewScanner(strings.NewReader(md))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			if inFence {
				cur = append(cur, line)
				flush()
				inFence = false
			} else {
				flush()
				cur = []string{line}
				inFence = true
			}
			continue
		}
		if inFence {
			cur = append(cur, line)
			continue
		}
		if trim == "" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return atoms, scanner.Err()
}

func packAtoms(atoms []string, budget int) []string {
	var parts []string
	var buf strings.Builder
	flushBuf := func() {
		if buf.Len() == 0 {
			return
		}
		parts = append(parts, strings.TrimSpace(buf.String()))
		buf.Reset()
	}
	for _, a := range atoms {
		if utf8.RuneCountInString(a) > budget {
			flushBuf()
			parts = append(parts, splitLongAtom(a, budget)...)
			continue
		}
		add := a
		if buf.Len() > 0 {
			add = "\n\n" + a
		}
		if utf8.RuneCountInString(buf.String())+utf8.RuneCountInString(add) > budget {
			flushBuf()
			buf.WriteString(a)
		} else {
			buf.WriteString(add)
		}
	}
	flushBuf()
	return parts
}

func splitLongAtom(s string, budget int) []string {
	words := strings.Fields(s)
	var out []string
	var buf strings.Builder
	for _, w := range words {
		// Single word longer than budget: hard-cut.
		if utf8.RuneCountInString(w) > budget {
			if buf.Len() > 0 {
				out = append(out, buf.String())
				buf.Reset()
			}
			runes := []rune(w)
			for i := 0; i < len(runes); i += budget {
				end := i + budget
				if end > len(runes) {
					end = len(runes)
				}
				out = append(out, string(runes[i:end]))
			}
			continue
		}
		add := w
		if buf.Len() > 0 {
			add = " " + w
		}
		if utf8.RuneCountInString(buf.String())+utf8.RuneCountInString(add) > budget {
			out = append(out, buf.String())
			buf.Reset()
			buf.WriteString(w)
			continue
		}
		buf.WriteString(add)
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

// numberedTweet renders the final tweet text including the "(i/n)" suffix
// when the thread spans more than one tweet.
func numberedTweet(part string, i, n int) string {
	if n > 1 {
		return fmt.Sprintf("%s (%d/%d)", part, i+1, n)
	}
	return part
}

// previewThread prints the packed thread without posting. Machine output
// (--json/--agent) emits a bare JSON array an agent can parse; human output
// shows a per-tweet banner with char counts.
func previewThread(w io.Writer, parts []string, flags *rootFlags) error {
	n := len(parts)
	if flags != nil && flags.asJSON {
		preview := make([]postedTweet, 0, n)
		for i, p := range parts {
			body := numberedTweet(p, i, n)
			preview = append(preview, postedTweet{Index: i + 1, Total: n, Text: body})
		}
		return json.NewEncoder(w).Encode(preview)
	}
	for i, p := range parts {
		body := numberedTweet(p, i, n)
		fmt.Fprintf(w, "── tweet %d/%d (%d chars) ──\n%s\n\n", i+1, n, utf8.RuneCountInString(body), body)
	}
	fmt.Fprintln(w, "(DRY-RUN — pass --post to actually post)")
	return nil
}

// postThread chain-posts each packed tweet via POST /2/tweets, threading every
// tweet after the first as a reply to the previous tweet's id.
func postThread(cmd *cobra.Command, parts []string, flags *rootFlags) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	n := len(parts)
	posted := make([]postedTweet, 0, n)
	var prevID string
	for i, p := range parts {
		body := numberedTweet(p, i, n)
		reqBody := map[string]any{"text": body}
		if prevID != "" {
			reqBody["reply"] = map[string]any{"in_reply_to_tweet_id": prevID}
		}
		data, _, err := c.Post(ctx, "/2/tweets", reqBody)
		if err != nil {
			return fmt.Errorf("posting tweet %d/%d: %w", i+1, n, classifyAPIError(err, flags))
		}
		id := tweetIDFromCreateResponse(data)
		if id == "" {
			return fmt.Errorf("tweet %d/%d: create response did not include an id", i+1, n)
		}
		prevID = id
		posted = append(posted, postedTweet{Index: i + 1, Total: n, ID: id, Text: body})
	}
	if flags.asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(posted)
	}
	for _, t := range posted {
		fmt.Fprintf(cmd.OutOrStdout(), "posted %d/%d: %s\n", t.Index, t.Total, t.ID)
	}
	return nil
}

// tweetIDFromCreateResponse pulls data.id from a POST /2/tweets response.
func tweetIDFromCreateResponse(data []byte) string {
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &resp) != nil {
		return ""
	}
	return resp.Data.ID
}
