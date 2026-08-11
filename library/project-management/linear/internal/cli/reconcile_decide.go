package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// Decision ladder and execution plans for `reconcile`.
//
// Confidence bands pick a class of relationship. They do not pick a side
// effect: which write expresses "same work" is a policy the operator owns,
// which is why the action inside each band is a flag rather than a hardcoded
// mapping.

// Decisions. These are contract vocabulary, not workspace vocabulary.
const (
	decisionCreateNew         = "create_new"
	decisionAppendDescription = "append_description"
	decisionAddSubIssue       = "add_sub_issue"
	decisionAddComment        = "add_comment"
	decisionLinkRelated       = "link_related"
	decisionLinkDuplicate     = "link_duplicate"
)

// Bands.
const (
	bandSameWork    = "same_work"
	bandRelatedWork = "related_work"
	bandAmbiguous   = "ambiguous"
	bandCreate      = "create"
	bandForced      = "forced"
)

// Plan action names, as reported in executed.action. They name the concrete
// plan that ran, not the decision.
const (
	actionIssueCreate                   = "issue_create"
	actionIssueCreateSub                = "issue_create_sub"
	actionIssueUpdateAppendDescription  = "issue_update_append_description"
	actionIssueUpdateReparent           = "issue_update_reparent"
	actionCommentCreate                 = "comment_create"
	actionIssueRelationCreate           = "issue_relation_create"
	actionIssueCreateThenRelationCreate = "issue_create_then_relation_create"
	actionNoAction                      = "no_action"
)

// Plan and step statuses.
//
// statusFailed means a mutation was attempted and did not land. statusError is
// narrower and never appears on a step: the plan was refused before its first
// mutation, so there is nothing to report but the reason.
const (
	statusOK       = "ok"
	statusDryRun   = "dry_run"
	statusNoop     = "noop"
	statusConflict = "conflict"
	statusPartial  = "partial"
	statusFailed   = "failed"
	statusSkipped  = "skipped"
	statusError    = "error"
)

// GraphQL mutation names, as reported in executed.result.steps[].mutation.
const (
	mutationIssueCreate         = "issueCreate"
	mutationIssueUpdate         = "issueUpdate"
	mutationCommentCreate       = "commentCreate"
	mutationIssueRelationCreate = "issueRelationCreate"
)

// IssueRelationType enum values used by this command. Both are schema enum
// values, not workspace vocabulary.
const (
	relationTypeRelated   = "related"
	relationTypeDuplicate = "duplicate"
)

// Relation direction values for --relation-direction.
const (
	relationDirectionNewToTarget = "new-to-target"
	relationDirectionTargetToNew = "target-to-new"
)

// reconcileTarget is the issue a decision acts on.
type reconcileTarget struct {
	ID         string  `json:"id"`
	Identifier string  `json:"identifier"`
	Title      string  `json:"title"`
	URL        *string `json:"url"`
	StateName  *string `json:"state_name"`
	StateType  *string `json:"state_type"`
	ProjectID  *string `json:"project_id"`
	UpdatedAt  *string `json:"updated_at"`
}

// reconcileEvidence is one signal or modifier that produced a confidence.
// A null Value means the signal was absent and its weight left the
// denominator.
type reconcileEvidence struct {
	Signal string  `json:"signal"`
	Value  any     `json:"value"`
	Weight float64 `json:"weight"`
	Detail string  `json:"detail"`
}

type reconcileAlternative struct {
	Decision   string              `json:"decision"`
	Target     reconcileTarget     `json:"target"`
	Confidence float64             `json:"confidence"`
	Evidence   []reconcileEvidence `json:"evidence"`
}

type reconcileStep struct {
	Mutation string         `json:"mutation"`
	Status   string         `json:"status"`
	Input    map[string]any `json:"input,omitempty"`
	ID       *string        `json:"id"`
	Error    *string        `json:"error"`
}

type reconcileExecResult struct {
	Status                  string           `json:"status"`
	Steps                   []reconcileStep  `json:"steps,omitempty"`
	Issue                   *reconcileTarget `json:"issue"`
	CommentID               *string          `json:"comment_id"`
	RelationID              *string          `json:"relation_id"`
	RelationType            *string          `json:"relation_type"`
	DescriptionBeforeSHA256 *string          `json:"description_before_sha256"`
	DescriptionAfterSHA256  *string          `json:"description_after_sha256"`
	Error                   *string          `json:"error"`
}

type reconcileExecuted struct {
	Action string              `json:"action"`
	Result reconcileExecResult `json:"result"`
}

// attachExecutionFailure records a refusal that happened before the plan ran.
//
// executeDecision only builds an executed block once it starts a plan, so a
// refusal raised ahead of the first mutation (add_comment with no body, the
// ambiguous band under --on-ambiguous refuse, a declined confirmation) used to
// leave executed null and put its reason on a trailing text line next to the
// decision JSON. Anything reading stdout then had two documents to parse, one
// of which was not JSON. The document now carries its own failure and the exit
// code is unchanged, so the reason is available without a second stream.
func attachExecutionFailure(doc *reconcileDecisionDoc, err error) {
	if doc == nil || err == nil || doc.Executed != nil {
		return
	}
	doc.Executed = &reconcileExecuted{
		Action: actionNoAction,
		Result: reconcileExecResult{
			Status: statusError,
			Error:  optionalString(err.Error()),
		},
	}
}

// reconcileDecisionDoc is the object emitted at .results of the provenance
// envelope. No top-level key is named description, comments or attachments,
// because --compact strips exactly those three from a single object and would
// otherwise silently delete part of the contract.
type reconcileDecisionDoc struct {
	SchemaVersion  int                    `json:"schema_version"`
	Decision       string                 `json:"decision"`
	Confidence     float64                `json:"confidence"`
	Target         *reconcileTarget       `json:"target"`
	Alternatives   []reconcileAlternative `json:"alternatives"`
	Evidence       []reconcileEvidence    `json:"evidence"`
	ThresholdsUsed map[string]any         `json:"thresholds_used"`
	DryRun         bool                   `json:"dry_run"`
	Executed       *reconcileExecuted     `json:"executed"`
}

// reconcileDecisionSchemaVersion is incremented only on a breaking change to
// the decision object.
const reconcileDecisionSchemaVersion = 1

// scoredCandidate pairs a candidate with its arithmetic.
type scoredCandidate struct {
	cand  reconcileCandidate
	score reconcileScore
}

// rankCandidates orders by confidence descending, then updated_at descending,
// then identifier ascending, which makes the pick deterministic for equal
// scores.
func rankCandidates(scored []scoredCandidate) {
	sort.SliceStable(scored, func(i, j int) bool {
		a, b := scored[i], scored[j]
		if a.score.Confidence != b.score.Confidence {
			return a.score.Confidence > b.score.Confidence
		}
		if !a.cand.UpdatedAt.Equal(b.cand.UpdatedAt) {
			return a.cand.UpdatedAt.After(b.cand.UpdatedAt)
		}
		return a.cand.Identifier < b.cand.Identifier
	})
}

// bandFor maps a confidence onto a band.
func bandFor(c float64, opts *reconcileOptions) string {
	switch {
	case c >= opts.thresholdDuplicate:
		return bandSameWork
	case c >= opts.thresholdRelated:
		return bandRelatedWork
	case c >= opts.thresholdCreate:
		return bandAmbiguous
	default:
		return bandCreate
	}
}

// decisionForBand picks the action inside a band. The ambiguous band reports
// the decision the related band would have produced unless --on-ambiguous
// collapses it to create_new.
func decisionForBand(band string, opts *reconcileOptions) string {
	switch band {
	case bandSameWork:
		return opts.sameWorkAction
	case bandRelatedWork:
		return opts.relatedAction
	case bandAmbiguous:
		if opts.onAmbiguous == onAmbiguousCreateNew {
			return decisionCreateNew
		}
		return opts.relatedAction
	default:
		return decisionCreateNew
	}
}

// targetFromCandidate projects a scored candidate into the wire shape. url is
// null for a purely local candidate: the sync selection carries no url field,
// so the store cannot supply one.
func targetFromCandidate(c reconcileCandidate) reconcileTarget {
	t := reconcileTarget{
		ID:         c.ID,
		Identifier: c.Identifier,
		Title:      c.Title,
		URL:        optionalString(c.URL),
		StateName:  optionalString(c.StateName),
		StateType:  optionalString(c.StateType),
		ProjectID:  optionalString(c.ProjectID),
	}
	if !c.UpdatedAt.IsZero() {
		formatted := c.UpdatedAt.UTC().Format(rfc3339)
		t.UpdatedAt = &formatted
	}
	return t
}

const rfc3339 = "2006-01-02T15:04:05Z07:00"

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

// hydratedIssue is the live read of the chosen target. It is required for
// correctness, not decoration: append_description cannot compose without the
// current description, url cannot come from the store, and a null issue is
// the only proof that a local row outlived the issue upstream.
type hydratedIssue struct {
	ID          string
	Identifier  string
	Title       string
	Description string
	URL         string
	StateName   string
	StateType   string
	ProjectID   string
	UpdatedAt   string
}

// hydrateIssue reads one issue live with the shared IssueQuery document,
// which is the selection that carries url, description and relations.
// Returns (nil, nil) when the issue is gone upstream.
func hydrateIssue(c graphqlQueryer, issueID string) (*hydratedIssue, error) {
	var resp struct {
		Issue *struct {
			ID          string `json:"id"`
			Identifier  string `json:"identifier"`
			Title       string `json:"title"`
			Description string `json:"description"`
			URL         string `json:"url"`
			UpdatedAt   string `json:"updatedAt"`
			State       struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"state"`
			Project *struct {
				ID string `json:"id"`
			} `json:"project"`
		} `json:"issue"`
	}
	if err := c.QueryInto(client.IssueQuery, map[string]any{"id": issueID}, &resp); err != nil {
		return nil, err
	}
	if resp.Issue == nil || resp.Issue.ID == "" {
		return nil, nil
	}
	h := &hydratedIssue{
		ID:          resp.Issue.ID,
		Identifier:  resp.Issue.Identifier,
		Title:       resp.Issue.Title,
		Description: resp.Issue.Description,
		URL:         resp.Issue.URL,
		StateName:   resp.Issue.State.Name,
		StateType:   resp.Issue.State.Type,
		UpdatedAt:   resp.Issue.UpdatedAt,
	}
	if resp.Issue.Project != nil {
		h.ProjectID = resp.Issue.Project.ID
	}
	return h, nil
}

// applyHydration folds the live read back into the wire target.
func applyHydration(t *reconcileTarget, h *hydratedIssue) {
	if t == nil || h == nil {
		return
	}
	if h.URL != "" {
		t.URL = optionalString(h.URL)
	}
	if h.Title != "" {
		t.Title = h.Title
	}
	if h.Identifier != "" {
		t.Identifier = h.Identifier
	}
	if h.StateName != "" {
		t.StateName = optionalString(h.StateName)
	}
	if h.StateType != "" {
		t.StateType = optionalString(h.StateType)
	}
	if h.ProjectID != "" {
		t.ProjectID = optionalString(h.ProjectID)
	}
	if h.UpdatedAt != "" {
		t.UpdatedAt = optionalString(h.UpdatedAt)
	}
}

// reconcileExecContext carries everything the execution arms need.
type reconcileExecContext struct {
	cmd    *cobra.Command
	flags  *rootFlags
	opts   *reconcileOptions
	db     *store.Store
	dbPath string

	// write is the mutating client. Under --dry-run its transport prints the
	// request and returns an empty envelope, so the arms never call it in
	// dry-run mode and build the step inputs instead.
	write *client.Client
	// read is the reading client with dry-run disabled, so a mutation
	// preview can still show the ids Linear would receive.
	read *client.Client

	sourceMode       bool
	sourceIssueID    string
	sourceIdentifier string
	teamID           string
	teamInfo         issueTeamInfo

	title string
	body  string

	target    *reconcileTarget
	hydration *hydratedIssue

	// planDryRun is true when no write may be attempted.
	planDryRun bool
}

// executeDecision runs the plan for a decision and returns the executed
// block. The returned error carries the exit code. The caller emits the
// envelope first either way, because a caller that asked for a decision gets
// the decision even when the write failed.
func executeDecision(ctx *reconcileExecContext, decision string) (*reconcileExecuted, error) {
	switch decision {
	case decisionCreateNew:
		return execCreateNew(ctx)
	case decisionAddComment:
		return execAddComment(ctx)
	case decisionAppendDescription:
		return execAppendDescription(ctx)
	case decisionAddSubIssue:
		return execAddSubIssue(ctx)
	case decisionLinkRelated:
		return execLink(ctx, relationTypeRelated)
	case decisionLinkDuplicate:
		return execLink(ctx, relationTypeDuplicate)
	}
	return nil, usageErr(fmt.Errorf("unknown decision %q", decision))
}

func execCreateNew(ctx *reconcileExecContext) (*reconcileExecuted, error) {
	if ctx.sourceMode {
		// The content already exists as an issue, so there is nothing to
		// create. create_new degrades to no_action rather than duplicating
		// the source.
		return &reconcileExecuted{
			Action: actionNoAction,
			Result: reconcileExecResult{
				Status: statusNoop,
				Steps: []reconcileStep{{
					Mutation: mutationIssueCreate,
					Status:   statusSkipped,
					Error:    optionalString("source mode: the reconciled issue already exists, nothing to create"),
				}},
			},
		}, nil
	}
	input, err := buildIssueCreateInput(ctx, "")
	if err != nil {
		return nil, err
	}
	exec := &reconcileExecuted{Action: actionIssueCreate}
	step, target, err := ctx.runIssueCreate(input)
	exec.Result.Steps = append(exec.Result.Steps, step)
	if err != nil {
		exec.Result.Status = failureStatus(ctx)
		exec.Result.Error = optionalString(err.Error())
		return exec, err
	}
	exec.Result.Issue = target
	exec.Result.Status = okStatus(ctx)
	return exec, nil
}

func execAddSubIssue(ctx *reconcileExecContext) (*reconcileExecuted, error) {
	if ctx.target == nil {
		return nil, usageErr(fmt.Errorf("add_sub_issue requires a target"))
	}
	if ctx.sourceMode {
		// The reconciled issue already exists, so it is reparented instead
		// of created.
		if err := ctx.assertTrustModeAllows(ctx.sourceIssueID, ctx.sourceIdentifier); err != nil {
			return nil, err
		}
		input := map[string]any{"parentId": ctx.target.ID}
		exec := &reconcileExecuted{Action: actionIssueUpdateReparent}
		step, raw, err := ctx.runIssueUpdate(ctx.sourceIssueID, input)
		exec.Result.Steps = append(exec.Result.Steps, step)
		if err != nil {
			exec.Result.Status = failureStatus(ctx)
			exec.Result.Error = optionalString(err.Error())
			return exec, err
		}
		exec.Result.Issue = targetFromRawIssue(raw)
		exec.Result.Status = okStatus(ctx)
		return exec, nil
	}
	input, err := buildIssueCreateInput(ctx, ctx.target.ID)
	if err != nil {
		return nil, err
	}
	exec := &reconcileExecuted{Action: actionIssueCreateSub}
	step, target, err := ctx.runIssueCreate(input)
	exec.Result.Steps = append(exec.Result.Steps, step)
	if err != nil {
		exec.Result.Status = failureStatus(ctx)
		exec.Result.Error = optionalString(err.Error())
		return exec, err
	}
	exec.Result.Issue = target
	exec.Result.Status = okStatus(ctx)
	return exec, nil
}

func execAddComment(ctx *reconcileExecContext) (*reconcileExecuted, error) {
	if ctx.target == nil {
		return nil, usageErr(fmt.Errorf("add_comment requires a target"))
	}
	if strings.TrimSpace(ctx.body) == "" {
		return nil, usageErr(fmt.Errorf("add_comment needs a body: pass --body, --body-file, or --body-stdin (in source mode the source issue has an empty description)"))
	}
	if err := ctx.assertTrustModeAllows(ctx.target.ID, ctx.target.Identifier); err != nil {
		return nil, err
	}
	input := map[string]any{"issueId": ctx.target.ID, "body": ctx.body}
	exec := &reconcileExecuted{Action: actionCommentCreate}
	if ctx.planDryRun {
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
			Mutation: mutationCommentCreate,
			Status:   statusDryRun,
			Input:    input,
		})
		exec.Result.Status = statusDryRun
		return exec, nil
	}
	resp, err := ctx.write.Mutate(commentCreateMutationDoc, map[string]any{"input": input})
	if err != nil {
		wrapped := classifyMutationError(mutationCommentCreate, err, ctx.flags, nil)
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
			Mutation: mutationCommentCreate,
			Status:   statusFailed,
			Input:    input,
			Error:    optionalString(err.Error()),
		})
		exec.Result.Status = statusFailed
		exec.Result.Error = optionalString(err.Error())
		return exec, wrapped
	}
	var parsed struct {
		CommentCreate struct {
			Success bool `json:"success"`
			Comment struct {
				ID string `json:"id"`
			} `json:"comment"`
		} `json:"commentCreate"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return exec, apiErr(fmt.Errorf("parsing commentCreate response: %w", err))
	}
	if !parsed.CommentCreate.Success {
		err := apiErr(fmt.Errorf("Linear reported commentCreate success=false"))
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{Mutation: mutationCommentCreate, Status: statusFailed, Input: input, Error: optionalString(err.Error())})
		exec.Result.Status = statusFailed
		exec.Result.Error = optionalString(err.Error())
		return exec, err
	}
	exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
		Mutation: mutationCommentCreate,
		Status:   statusOK,
		Input:    input,
		ID:       optionalString(parsed.CommentCreate.Comment.ID),
	})
	exec.Result.CommentID = optionalString(parsed.CommentCreate.Comment.ID)
	exec.Result.Status = statusOK
	return exec, nil
}

const commentCreateMutationDoc = `mutation($input: CommentCreateInput!) {
	commentCreate(input: $input) {
		success
		comment { id body createdAt url issue { id identifier } }
	}
}`

func execAppendDescription(ctx *reconcileExecContext) (*reconcileExecuted, error) {
	if ctx.target == nil {
		return nil, usageErr(fmt.Errorf("append_description requires a target"))
	}
	if strings.TrimSpace(ctx.body) == "" {
		return nil, usageErr(fmt.Errorf("append_description needs a body: pass --body, --body-file, or --body-stdin"))
	}
	if ctx.hydration == nil {
		return nil, usageErr(fmt.Errorf("append_description needs the target's current description, which requires hydration: drop --data-source local"))
	}
	if err := ctx.assertTrustModeAllows(ctx.target.ID, ctx.target.Identifier); err != nil {
		return nil, err
	}

	before := ctx.hydration.Description
	beforeHash := sha256Hex(before)
	exec := &reconcileExecuted{Action: actionIssueUpdateAppendDescription}
	exec.Result.DescriptionBeforeSHA256 = optionalString(beforeHash)

	// issueUpdate.description is a whole-field replace and IssueUpdateInput
	// carries no version or updatedAt precondition, so a read-modify-write
	// can silently discard a concurrent edit. The hash guard closes the
	// window as far as the API allows: nothing between hydration and the
	// mutation re-reads, so the window is one round trip.
	if ctx.opts.appendGuard == appendGuardHash && ctx.opts.expectDescriptionSHA256 != "" {
		if !strings.EqualFold(ctx.opts.expectDescriptionSHA256, beforeHash) {
			err := apiErr(fmt.Errorf("description of %s hashes to %s, not the expected %s: someone edited it since the decision was reviewed. Re-run reconcile, or pass --append-guard off to force the append", ctx.target.Identifier, beforeHash, ctx.opts.expectDescriptionSHA256))
			exec.Result.Status = statusConflict
			exec.Result.Error = optionalString(err.Error())
			exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
				Mutation: mutationIssueUpdate,
				Status:   statusSkipped,
				Error:    optionalString("description precondition failed"),
			})
			return exec, err
		}
	}

	addition := ctx.body
	if ctx.opts.appendHeader != "" {
		addition = ctx.opts.appendHeader + "\n" + addition
	}
	after := appendedDescriptionBodyWith(before, addition, ctx.opts.appendSeparator)
	input := map[string]any{"description": after}

	if ctx.planDryRun {
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
			Mutation: mutationIssueUpdate,
			Status:   statusDryRun,
			Input:    map[string]any{"id": ctx.target.ID, "input": input},
		})
		exec.Result.DescriptionAfterSHA256 = optionalString(sha256Hex(after))
		exec.Result.Status = statusDryRun
		return exec, nil
	}

	raw, err := setIssueDescription(ctx.write, ctx.target.ID, after)
	if err != nil {
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
			Mutation: mutationIssueUpdate,
			Status:   statusFailed,
			Input:    map[string]any{"id": ctx.target.ID, "input": input},
			Error:    optionalString(err.Error()),
		})
		exec.Result.Status = statusFailed
		exec.Result.Error = optionalString(err.Error())
		return exec, err
	}
	exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
		Mutation: mutationIssueUpdate,
		Status:   statusOK,
		Input:    map[string]any{"id": ctx.target.ID, "input": input},
		ID:       optionalString(ctx.target.ID),
	})
	exec.Result.DescriptionAfterSHA256 = optionalString(sha256Hex(after))
	if updated, ok := raw["description"].(string); ok {
		exec.Result.DescriptionAfterSHA256 = optionalString(sha256Hex(updated))
	}
	exec.Result.Issue = targetFromDecodedIssue(raw)
	exec.Result.Status = statusOK
	return exec, nil
}

// execLink implements link_related and link_duplicate. In proposal mode the
// reconciled issue does not exist yet and IssueRelationCreateInput requires
// both sides, so the plan is two steps: create, then relate. In source mode it
// is one.
func execLink(ctx *reconcileExecContext, relationType string) (*reconcileExecuted, error) {
	if ctx.target == nil {
		return nil, usageErr(fmt.Errorf("%s requires a target", relationType))
	}
	if err := ctx.assertTrustModeAllows(ctx.target.ID, ctx.target.Identifier); err != nil {
		return nil, err
	}

	exec := &reconcileExecuted{Action: actionIssueRelationCreate}
	exec.Result.RelationType = optionalString(relationType)

	newIssueID := ctx.sourceIssueID
	if !ctx.sourceMode {
		exec.Action = actionIssueCreateThenRelationCreate
		input, err := buildIssueCreateInput(ctx, "")
		if err != nil {
			return nil, err
		}
		step, created, err := ctx.runIssueCreate(input)
		exec.Result.Steps = append(exec.Result.Steps, step)
		if err != nil {
			exec.Result.Status = failureStatus(ctx)
			exec.Result.Error = optionalString(err.Error())
			return exec, err
		}
		if created != nil {
			exec.Result.Issue = created
			newIssueID = created.ID
		}
	} else {
		if err := ctx.assertTrustModeAllows(ctx.sourceIssueID, ctx.sourceIdentifier); err != nil {
			return nil, err
		}
		// A relation that already exists is an error on the API side, and
		// hydration of the target cannot reveal it because relations hang
		// off the reconciled issue. issue_relations is never synced, so the
		// only way to know is to read them live.
		if !ctx.planDryRun {
			existing, err := ctx.findExistingRelation(ctx.sourceIssueID, ctx.target.ID, relationType)
			if err != nil {
				return exec, err
			}
			if existing != "" {
				if ctx.flags.idempotent {
					exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
						Mutation: mutationIssueRelationCreate,
						Status:   statusNoop,
						ID:       optionalString(existing),
					})
					exec.Result.RelationID = optionalString(existing)
					exec.Result.Status = statusNoop
					return exec, nil
				}
				err := apiErr(fmt.Errorf("%s and %s are already related as %q (relation %s). Pass --idempotent to treat this as a no-op", ctx.sourceIdentifier, ctx.target.Identifier, relationType, existing))
				exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
					Mutation: mutationIssueRelationCreate,
					Status:   statusFailed,
					ID:       optionalString(existing),
					Error:    optionalString(err.Error()),
				})
				exec.Result.Status = statusFailed
				exec.Result.Error = optionalString(err.Error())
				return exec, err
			}
		}
	}

	// With --relation-direction new-to-target (the default), the reconciled
	// issue is the one declared duplicate of or related to the existing one.
	// Which side Linear treats as surviving in its own duplicate UI is not
	// verified against a live workspace, which is why direction is a flag.
	issueID, relatedIssueID := newIssueID, ctx.target.ID
	if ctx.opts.relationDirection == relationDirectionTargetToNew {
		issueID, relatedIssueID = ctx.target.ID, newIssueID
	}
	relInput := map[string]any{"type": relationType}
	if issueID != "" {
		relInput["issueId"] = issueID
	}
	if relatedIssueID != "" {
		relInput["relatedIssueId"] = relatedIssueID
	}

	if ctx.planDryRun {
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
			Mutation: mutationIssueRelationCreate,
			Status:   statusDryRun,
			Input:    relInput,
		})
		if step, ok := ctx.planDuplicateStateMove(newIssueID, relationType); ok {
			exec.Result.Steps = append(exec.Result.Steps, step)
		}
		exec.Result.Status = statusDryRun
		return exec, nil
	}

	resp, err := ctx.write.Mutate(client.IssueRelationCreateMutation, map[string]any{"input": relInput})
	if err != nil {
		wrapped := classifyMutationError(mutationIssueRelationCreate, err, ctx.flags, nil)
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
			Mutation: mutationIssueRelationCreate,
			Status:   statusFailed,
			Input:    relInput,
			Error:    optionalString(err.Error()),
		})
		// A two-step plan whose first mutation succeeded and whose second
		// failed is a partial, not a failure: the issue exists upstream and
		// the caller has to know that.
		if len(exec.Result.Steps) > 1 {
			exec.Result.Status = statusPartial
			exec.Result.Error = optionalString(err.Error())
			return exec, partialFailureErr(fmt.Errorf("created the issue but issueRelationCreate failed: %w", err))
		}
		exec.Result.Status = statusFailed
		exec.Result.Error = optionalString(err.Error())
		return exec, wrapped
	}
	var parsed struct {
		IssueRelationCreate struct {
			Success       bool `json:"success"`
			IssueRelation struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"issueRelation"`
		} `json:"issueRelationCreate"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return exec, apiErr(fmt.Errorf("parsing issueRelationCreate response: %w", err))
	}
	if !parsed.IssueRelationCreate.Success {
		err := apiErr(fmt.Errorf("Linear reported issueRelationCreate success=false"))
		exec.Result.Steps = append(exec.Result.Steps, reconcileStep{Mutation: mutationIssueRelationCreate, Status: statusFailed, Input: relInput, Error: optionalString(err.Error())})
		if len(exec.Result.Steps) > 1 {
			exec.Result.Status = statusPartial
			exec.Result.Error = optionalString(err.Error())
			return exec, partialFailureErr(err)
		}
		exec.Result.Status = statusFailed
		exec.Result.Error = optionalString(err.Error())
		return exec, err
	}
	exec.Result.Steps = append(exec.Result.Steps, reconcileStep{
		Mutation: mutationIssueRelationCreate,
		Status:   statusOK,
		Input:    relInput,
		ID:       optionalString(parsed.IssueRelationCreate.IssueRelation.ID),
	})
	exec.Result.RelationID = optionalString(parsed.IssueRelationCreate.IssueRelation.ID)
	if parsed.IssueRelationCreate.IssueRelation.Type != "" {
		exec.Result.RelationType = optionalString(parsed.IssueRelationCreate.IssueRelation.Type)
	}
	exec.Result.Status = statusOK

	// link_duplicate never moves the target's state and never moves the
	// reconciled issue's state unless the operator names one, because the
	// state that expresses "duplicate" is a per-workspace object.
	if step, ok := ctx.planDuplicateStateMove(newIssueID, relationType); ok {
		stateID, err := ctx.resolveDuplicateStateID()
		if err != nil {
			step.Status = statusFailed
			step.Error = optionalString(err.Error())
			exec.Result.Steps = append(exec.Result.Steps, step)
			exec.Result.Status = statusPartial
			exec.Result.Error = optionalString(err.Error())
			return exec, partialFailureErr(fmt.Errorf("relation created but the state move failed: %w", err))
		}
		moveInput := map[string]any{"stateId": stateID}
		moveStep, _, err := ctx.runIssueUpdate(newIssueID, moveInput)
		exec.Result.Steps = append(exec.Result.Steps, moveStep)
		if err != nil {
			exec.Result.Status = statusPartial
			exec.Result.Error = optionalString(err.Error())
			return exec, partialFailureErr(fmt.Errorf("relation created but the state move failed: %w", err))
		}
	}
	return exec, nil
}

// planDuplicateStateMove reports the placeholder step for an optional state
// move after link_duplicate, and whether one was requested at all.
func (ctx *reconcileExecContext) planDuplicateStateMove(issueID, relationType string) (reconcileStep, bool) {
	if relationType != relationTypeDuplicate {
		return reconcileStep{}, false
	}
	if ctx.opts.duplicateStateName == "" && ctx.opts.duplicateStateType == "" {
		return reconcileStep{}, false
	}
	input := map[string]any{}
	if ctx.opts.duplicateStateName != "" {
		input["state_name"] = ctx.opts.duplicateStateName
	}
	if ctx.opts.duplicateStateType != "" {
		input["state_type"] = ctx.opts.duplicateStateType
	}
	return reconcileStep{
		Mutation: mutationIssueUpdate,
		Status:   statusDryRun,
		Input:    map[string]any{"id": issueID, "input": input},
	}, true
}

func (ctx *reconcileExecContext) resolveDuplicateStateID() (string, error) {
	stateID, err := resolveWorkflowState(ctx.read, ctx.teamInfo, ctx.opts.duplicateStateName, ctx.opts.duplicateStateType)
	if err != nil {
		return "", classifyLiveReadError(err, ctx.flags)
	}
	return stateID, nil
}

// findExistingRelation returns the id of an existing relation of the given
// type between two issues, in either direction, or "".
func (ctx *reconcileExecContext) findExistingRelation(issueID, otherID, relationType string) (string, error) {
	result, err := ctx.read.FetchIssueRelations(issueID, 0)
	if err != nil {
		return "", classifyLiveReadError(err, ctx.flags)
	}
	for _, node := range result.Relations {
		if node.Type == relationType && node.RelatedIssue.ID == otherID {
			return node.ID, nil
		}
	}
	for _, node := range result.InverseRelations {
		if node.Type == relationType && node.Issue.ID == otherID {
			return node.ID, nil
		}
	}
	return "", nil
}

// assertTrustModeAllows enforces --trust-mode strict, which advertises that
// it refuses to mutate Linear issues not in the local pp_created table.
// reconcile mutates issues it did not create, which is precisely the case the
// flag claims to guard, so it is the first honest consumer of that guard.
func (ctx *reconcileExecContext) assertTrustModeAllows(issueID, identifier string) error {
	if ctx.flags.trustMode != "strict" || issueID == "" {
		return nil
	}
	if ctx.db == nil {
		return usageErr(fmt.Errorf("--trust-mode strict needs the local pp_created ledger, which could not be opened"))
	}
	created, err := ctx.db.IsPPCreated(issueID)
	if err != nil {
		return fmt.Errorf("checking pp_created ledger: %w", err)
	}
	if !created {
		name := identifier
		if name == "" {
			name = issueID
		}
		return usageErr(fmt.Errorf("--trust-mode strict refuses to mutate %s: it is not in the local pp_created ledger", name))
	}
	return nil
}

const reconcileIssueCreateMutation = `mutation($input: IssueCreateInput!) {
	issueCreate(input: $input) {
		success
		issue {
			id identifier title description url priority createdAt updatedAt
			team { id key }
			state { id name type }
			assignee { id name displayName }
			project { id name }
			parent { id identifier title }
		}
	}
}`

// runIssueCreate performs the issueCreate step, records the new issue in the
// pp_created ledger and writes it back to the local store, mirroring
// `issues create`.
func (ctx *reconcileExecContext) runIssueCreate(input map[string]any) (reconcileStep, *reconcileTarget, error) {
	if ctx.planDryRun {
		return reconcileStep{
			Mutation: mutationIssueCreate,
			Status:   statusDryRun,
			Input:    input,
		}, nil, nil
	}
	resp, err := ctx.write.Mutate(reconcileIssueCreateMutation, map[string]any{"input": input})
	if err != nil {
		return reconcileStep{
			Mutation: mutationIssueCreate,
			Status:   statusFailed,
			Input:    input,
			Error:    optionalString(err.Error()),
		}, nil, classifyMutationError(mutationIssueCreate, err, ctx.flags, nil)
	}
	var parsed struct {
		IssueCreate struct {
			Success bool            `json:"success"`
			Issue   json.RawMessage `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return reconcileStep{Mutation: mutationIssueCreate, Status: statusFailed, Input: input, Error: optionalString(err.Error())},
			nil, apiErr(fmt.Errorf("parsing issueCreate response: %w", err))
	}
	if !parsed.IssueCreate.Success {
		failure := apiErr(fmt.Errorf("Linear reported issueCreate success=false"))
		return reconcileStep{Mutation: mutationIssueCreate, Status: statusFailed, Input: input, Error: optionalString(failure.Error())},
			nil, failure
	}
	target := targetFromRawIssue(parsed.IssueCreate.Issue)
	if target != nil {
		ctx.recordCreatedIssue(target, parsed.IssueCreate.Issue)
	}
	id := ""
	if target != nil {
		id = target.ID
	}
	return reconcileStep{
		Mutation: mutationIssueCreate,
		Status:   statusOK,
		Input:    input,
		ID:       optionalString(id),
	}, target, nil
}

func (ctx *reconcileExecContext) recordCreatedIssue(target *reconcileTarget, raw json.RawMessage) {
	session := resolvePPSession(ctx.flags, "")
	if session == "" || session == "current" {
		session = ppCurrentSession()
	}
	if ctx.db != nil {
		if err := ctx.db.RecordPPFixture(target.ID, target.Identifier, target.Title, session); err != nil {
			fmt.Fprintf(ctx.cmd.ErrOrStderr(), "warning: pp_created ledger write failed: %v\n", err)
		}
	}
	writeIssueBack(ctx.dbPath, raw)
}

func (ctx *reconcileExecContext) runIssueUpdate(issueID string, input map[string]any) (reconcileStep, json.RawMessage, error) {
	payload := map[string]any{"id": issueID, "input": input}
	if ctx.planDryRun {
		return reconcileStep{Mutation: mutationIssueUpdate, Status: statusDryRun, Input: payload}, nil, nil
	}
	const mutation = `mutation($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) {
			success
			issue {
				id identifier title description url updatedAt
				state { id name type }
				team { id key }
				project { id name }
				parent { id identifier title }
			}
		}
	}`
	resp, err := ctx.write.Mutate(mutation, map[string]any{"id": issueID, "input": input})
	if err != nil {
		return reconcileStep{Mutation: mutationIssueUpdate, Status: statusFailed, Input: payload, Error: optionalString(err.Error())},
			nil, classifyMutationError(mutationIssueUpdate, err, ctx.flags, nil)
	}
	var parsed struct {
		IssueUpdate struct {
			Success bool            `json:"success"`
			Issue   json.RawMessage `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return reconcileStep{Mutation: mutationIssueUpdate, Status: statusFailed, Input: payload, Error: optionalString(err.Error())},
			nil, apiErr(fmt.Errorf("parsing issueUpdate response: %w", err))
	}
	if !parsed.IssueUpdate.Success {
		failure := apiErr(fmt.Errorf("Linear reported issueUpdate success=false"))
		return reconcileStep{Mutation: mutationIssueUpdate, Status: statusFailed, Input: payload, Error: optionalString(failure.Error())},
			nil, failure
	}
	writeIssueBack(ctx.dbPath, parsed.IssueUpdate.Issue)
	return reconcileStep{Mutation: mutationIssueUpdate, Status: statusOK, Input: payload, ID: optionalString(issueID)}, parsed.IssueUpdate.Issue, nil
}

// buildIssueCreateInput assembles IssueCreateInput from the reconciled
// content plus the forwarded `issues create` flags. Resolution order and
// mutual exclusions are those of `issues create`. reconcile does not
// reinterpret them.
func buildIssueCreateInput(ctx *reconcileExecContext, parentID string) (map[string]any, error) {
	opts := ctx.opts
	input := map[string]any{
		"title":  ctx.title,
		"teamId": ctx.teamID,
	}
	if ctx.body != "" {
		input["description"] = ctx.body
	}
	if opts.priority > 0 {
		input["priority"] = opts.priority
	}
	if opts.assignee != "" {
		input["assigneeId"] = opts.assignee
	}
	if len(opts.labels) > 0 {
		input["labelIds"] = opts.labels
	}
	if opts.state != "" {
		input["stateId"] = opts.state
	}
	if opts.project != "" || opts.projectName != "" {
		projectID, err := resolveProjectFlag(ctx.read, opts.project, opts.projectName, opts.team, ctx.flags)
		if err != nil {
			return nil, err
		}
		if projectID != "" {
			input["projectId"] = projectID
		}
	}
	switch {
	case parentID != "":
		input["parentId"] = parentID
	case opts.parent != "":
		resolved, err := resolveParentIssueID(ctx.read, opts.parent)
		if err != nil {
			return nil, classifyLiveReadError(err, ctx.flags)
		}
		input["parentId"] = resolved
	}
	if opts.stateName != "" || opts.stateType != "" {
		stateID, err := resolveWorkflowState(ctx.read, ctx.teamInfo, opts.stateName, opts.stateType)
		if err != nil {
			return nil, classifyLiveReadError(err, ctx.flags)
		}
		input["stateId"] = stateID
	}
	if len(opts.labels) > 0 {
		if err := validateIssueLabelTeams(ctx.read, opts.labels, ctx.teamInfo); err != nil {
			return nil, classifyLiveReadError(err, ctx.flags)
		}
	}
	return input, nil
}

func targetFromRawIssue(raw json.RawMessage) *reconcileTarget {
	if len(raw) == 0 {
		return nil
	}
	var issue struct {
		ID         string `json:"id"`
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
		URL        string `json:"url"`
		UpdatedAt  string `json:"updatedAt"`
		State      struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"state"`
		Project *struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if err := json.Unmarshal(raw, &issue); err != nil || issue.ID == "" {
		return nil
	}
	t := &reconcileTarget{
		ID:         issue.ID,
		Identifier: issue.Identifier,
		Title:      issue.Title,
		URL:        optionalString(issue.URL),
		StateName:  optionalString(issue.State.Name),
		StateType:  optionalString(issue.State.Type),
		UpdatedAt:  optionalString(issue.UpdatedAt),
	}
	if issue.Project != nil {
		t.ProjectID = optionalString(issue.Project.ID)
	}
	return t
}

// targetFromDecodedIssue projects an already-decoded issue payload, which is
// what the shared description writer returns.
func targetFromDecodedIssue(obj map[string]any) *reconcileTarget {
	if obj == nil {
		return nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	return targetFromRawIssue(raw)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// okStatus and failureStatus keep the dry-run shape consistent: under
// --execute --dry-run every plan reports dry_run rather than ok.
func okStatus(ctx *reconcileExecContext) string {
	if ctx.planDryRun {
		return statusDryRun
	}
	return statusOK
}

func failureStatus(ctx *reconcileExecContext) string {
	if ctx.planDryRun {
		return statusDryRun
	}
	return statusFailed
}
