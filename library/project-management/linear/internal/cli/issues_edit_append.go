package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"

	"github.com/spf13/cobra"
)

// issueDescriptionAppendMutation is the issueUpdate document used by the
// append primitive. It selects the same issue fields as the main
// "issues edit" mutation so callers can render the result identically.
const issueDescriptionAppendMutation = `mutation($id: String!, $input: IssueUpdateInput!) {
	issueUpdate(id: $id, input: $input) {
		success
		issue {
			id identifier title description url priority estimate dueDate createdAt updatedAt
			state { id name type }
			team { id key name }
			project { id name }
			assignee { id name displayName email }
			parent { id identifier title }
			children { nodes { id identifier title } }
		}
	}
}`

// descriptionAppendFlags carries the --description-append flag family for
// "issues edit". The flags are registered from issues_edit.go so the whole
// append path lives in one file.
type descriptionAppendFlags struct {
	text  string
	file  string
	stdin bool
}

// register adds the append flag family to the given command.
func (d *descriptionAppendFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&d.text, "description-append", "", "Append this markdown to the existing description instead of replacing it (read-modify-write, joined with a blank line)")
	cmd.Flags().StringVar(&d.file, "description-append-file", "", "Append markdown read from this file to the existing description")
	cmd.Flags().BoolVar(&d.stdin, "description-append-stdin", false, "Append markdown read from stdin to the existing description")
}

// resolve returns the append body and whether any append source was given.
// It rejects a combination with the replace-mode --description flags at exit
// code 2, because appending and replacing the same field in one call is
// always a mistake rather than a merge. The rejection happens before any
// body is read, so --description-stdin and --description-append-stdin in the
// same call fail with the conflict rather than with a drained stdin.
func (d *descriptionAppendFlags) resolve(cmd *cobra.Command, descriptionReplaceSet bool) (string, bool, error) {
	requested := cmd.Flags().Changed("description-append") ||
		cmd.Flags().Changed("description-append-file") ||
		d.stdin
	if !requested {
		return "", false, nil
	}
	if descriptionReplaceSet {
		return "", false, usageErr(fmt.Errorf("pass either a --description* replace flag or a --description-append* flag, not both"))
	}
	body, set, err := readMarkdownBody(cmd, markdownBodySpec{
		InlineFlag: "description-append",
		Inline:     d.text,
		FileFlag:   "description-append-file",
		File:       d.file,
		StdinFlag:  "description-append-stdin",
		Stdin:      d.stdin,
		Label:      "description append",
	})
	if err != nil {
		return "", false, err
	}
	if !set || strings.TrimSpace(body) == "" {
		return "", false, usageErr(fmt.Errorf("--description-append needs a non-empty body"))
	}
	return body, true, nil
}

// defaultDescriptionAppendSeparator is the blank line placed between an
// existing description and an appended block.
const defaultDescriptionAppendSeparator = "\n\n"

// appendedDescriptionBody joins an addition onto an existing description with
// exactly one blank line between them. An empty existing body yields the
// addition alone, so the first append never leaves leading whitespace.
func appendedDescriptionBody(existing, addition string) string {
	return appendedDescriptionBodyWith(existing, addition, defaultDescriptionAppendSeparator)
}

// appendedDescriptionBodyWith is the pure composer behind every append path.
// A caller that needs its own separator or its own header block calls this
// directly and then commits the result with setIssueDescription.
func appendedDescriptionBodyWith(existing, addition, separator string) string {
	trimmedExisting := strings.TrimRight(existing, "\n")
	trimmedAddition := strings.TrimLeft(addition, "\n")
	if strings.TrimSpace(trimmedExisting) == "" {
		return trimmedAddition
	}
	return trimmedExisting + separator + trimmedAddition
}

// setIssueDescription writes a fully composed description onto one issue and
// returns the decoded issueUpdate.issue payload. It performs no read, so the
// caller owns both the composition and any before/after hashing.
//
//	func setIssueDescription(c *client.Client, issueID string, body string) (map[string]any, error)
//
// issueID must be the issue UUID, because nothing here resolves identifiers.
func setIssueDescription(c *client.Client, issueID string, body string) (map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("setIssueDescription: nil Linear client")
	}
	if strings.TrimSpace(issueID) == "" {
		return nil, usageErr(fmt.Errorf("setIssueDescription: empty issue id"))
	}
	resp, err := c.Mutate(issueDescriptionAppendMutation, map[string]any{
		"id":    issueID,
		"input": map[string]any{"description": body},
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		IssueUpdate struct {
			Success bool           `json:"success"`
			Issue   map[string]any `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return nil, fmt.Errorf("parsing issueUpdate response: %w", err)
	}
	if !parsed.IssueUpdate.Success {
		return nil, apiErr(fmt.Errorf("Linear reported issueUpdate success=false for issue %q", issueID))
	}
	return parsed.IssueUpdate.Issue, nil
}

// appendIssueDescriptionFrom appends to a description the caller already
// hydrated, so no read happens between hydration and the write. This is the
// variant the reconcile command uses when it holds an append guard hash.
//
//	func appendIssueDescriptionFrom(c *client.Client, issueID string, existing string, text string) (before string, after string, err error)
//
// issueID must be the issue UUID. before is the hydrated body exactly as
// passed in, after is the committed body, and both are returned even though
// only after was written, so the caller can hash the pair.
func appendIssueDescriptionFrom(c *client.Client, issueID string, existing string, text string) (string, string, error) {
	if strings.TrimSpace(text) == "" {
		return existing, existing, usageErr(fmt.Errorf("appendIssueDescriptionFrom: empty append body for issue %q", issueID))
	}
	after := appendedDescriptionBody(existing, text)
	if _, err := setIssueDescription(c, issueID, after); err != nil {
		return existing, existing, err
	}
	return existing, after, nil
}

// appendIssueDescription performs a standalone read-modify-write append on a
// single issue description: it reads the current body with the same live
// issue read the edit command uses, joins the addition after a blank line,
// and commits the result through issueUpdate.
//
// Exact signature, relied on by the reconcile command:
//
//	func appendIssueDescription(c *client.Client, issueID string, text string) (map[string]any, error)
//
// c is the *client.Client that issues_edit.go builds via flags.newClient().
// issueID may be an issue UUID or a TEAM-NUMBER identifier. The returned map
// is the decoded issueUpdate.issue payload. A caller that already hydrated
// the issue should use appendIssueDescriptionFrom instead and skip the read.
func appendIssueDescription(c *client.Client, issueID string, text string) (map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("appendIssueDescription: nil Linear client")
	}
	if strings.TrimSpace(issueID) == "" {
		return nil, usageErr(fmt.Errorf("appendIssueDescription: empty issue reference"))
	}
	if strings.TrimSpace(text) == "" {
		return nil, usageErr(fmt.Errorf("appendIssueDescription: empty append body for issue %q", issueID))
	}

	raw, err := fetchIssueLive(c, issueID)
	if err != nil {
		return nil, err
	}
	var existing struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &existing); err != nil {
		return nil, fmt.Errorf("parsing existing issue %q: %w", issueID, err)
	}
	if existing.ID == "" {
		return nil, notFoundErr(fmt.Errorf("issue %q did not include an id", issueID))
	}
	return setIssueDescription(c, existing.ID, appendedDescriptionBody(existing.Description, text))
}
