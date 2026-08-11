package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/groups"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// `reconcile` joins duplicate detection to the write surfaces.
//
// The CLI could already find near-duplicates and could already create, edit
// and comment. Nothing joined the two, so an agent that wanted to avoid
// filing a fourth ticket for the same work had to run a search, read a bare
// list, and decide in prose. reconcile is that join, made typed and
// auditable: one invocation searches, scores, decides, explains, and on
// request executes.

// Live search legs.
const (
	liveSearchOff      = "off"
	liveSearchSearch   = "search"
	liveSearchSemantic = "semantic"
	liveSearchBoth     = "both"
)

// --same-work-action values.
const (
	sameWorkAddComment        = decisionAddComment
	sameWorkAppendDescription = decisionAppendDescription
	sameWorkLinkDuplicate     = decisionLinkDuplicate
)

// --related-action values.
const (
	relatedLinkRelated = decisionLinkRelated
	relatedAddSubIssue = decisionAddSubIssue
	relatedCreateNew   = decisionCreateNew
)

// --on-ambiguous values.
const (
	onAmbiguousRefuse    = "refuse"
	onAmbiguousCreateNew = "create_new"
)

// --on-vanished-target values.
const (
	onVanishedRefuse    = "refuse"
	onVanishedRecompute = "recompute"
)

// --append-guard values.
const (
	appendGuardHash = "hash"
	appendGuardOff  = "off"
)

// reconcileOptions is the complete flag surface. Every number that shapes a
// decision lives here, bound to a flag, so there are no scoring or threshold
// constants in the binary.
type reconcileOptions struct {
	// Input mode
	title     string
	body      string
	bodyFile  string
	bodyStdin bool
	descAlias string
	descFile  string
	descStdin bool
	source    string
	team      string
	dbPath    string

	// Retrieval
	candidates           int
	matchMode            string
	minTokenLen          int
	maxQueryTokens       int
	queryBodyChars       int
	ftsWeightIdentifier  float64
	ftsWeightTitle       float64
	ftsWeightDescription float64
	candidateGroup       string
	includeOutOfGroup    bool
	liveSearch           string
	liveSearchLimit      int

	// Scoring
	ftsScale               float64
	weightTitleDice        float64
	weightTitleContainment float64
	weightFTS              float64
	weightBodyOverlap      float64
	weightSemantic         float64
	exactFloor             float64
	penaltyOutOfGroup      float64
	recencyHalflife        time.Duration
	recencyFloor           float64
	penaltyProjectMismatch float64
	nearTieEpsilon         float64

	// Thresholds
	thresholdDuplicate float64
	thresholdRelated   float64
	thresholdCreate    float64

	// Decision policy
	sameWorkAction   string
	relatedAction    string
	onAmbiguous      string
	onVanishedTarget string
	decision         string
	target           string

	// Execution
	execute                 bool
	appendSeparator         string
	appendHeader            string
	appendGuard             string
	expectDescriptionSHA256 string
	relationDirection       string
	duplicateStateName      string
	duplicateStateType      string
	hydrateAlternatives     bool

	// Forwarded to issueCreate
	project     string
	projectName string
	priority    int
	assignee    string
	labels      []string
	parent      string
	state       string
	stateName   string
	stateType   string
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	opts := &reconcileOptions{}
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Decide whether proposed work is a duplicate, a relative, or genuinely new, and optionally act on it",
		Long: `Search the synced issue index for work that already covers a proposal, score
every candidate, and emit a typed decision: create_new, append_description,
add_sub_issue, add_comment, link_related, or link_duplicate.

Two input modes. Proposal mode (--title, optionally --body) reconciles content
that does not exist in Linear yet, so create_new is reachable. Source mode
(--source ENG-412) reconciles an issue that already exists, which is excluded
from its own candidate set, so create_new degrades to a no-op.

The command is read-only until --execute. Every threshold, weight and
multiplier is a flag, and every one of them is reported in
.results.thresholds_used so a decision can be recomputed by hand.

The index auto-refreshes on the same contract as 'issues search'. A refresh
that cannot run exits 5 rather than serving an empty index, because an empty
index would otherwise read as "nothing similar exists".`,
		Example: `  # Decide, without writing anything
  linear-pp-cli reconcile --title "Clean up Kimi replay temp directories" --team SYMPH --agent

  # Decide from an existing issue
  linear-pp-cli reconcile --source SYMPH-309 --team SYMPH --agent

  # Act on the decision
  linear-pp-cli reconcile --title "x" --body-file /tmp/body.md --team SYMPH --agent --execute

  # Preview the exact mutations without sending them
  linear-pp-cli reconcile --title "x" --team SYMPH --agent --execute --dry-run

  # Pin the decision reviewed in an earlier phase
  linear-pp-cli reconcile --source SYMPH-309 --team SYMPH --agent --execute --decision link_duplicate --target SYMPH-201`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify mode: the mechanical verify pass invokes every command
			// with synthetic arguments against an empty store, which is not
			// a meaningful reconcile.
			if cliutil.IsVerifyEnv() {
				return nil
			}
			return runReconcile(cmd, flags, opts)
		},
	}
	bindReconcileFlags(cmd, opts)
	return cmd
}

func bindReconcileFlags(cmd *cobra.Command, opts *reconcileOptions) {
	f := cmd.Flags()

	f.StringVar(&opts.title, "title", "", "Proposed issue title (proposal mode, required unless --source)")
	f.StringVar(&opts.body, "body", "", "Proposed body markdown")
	f.StringVar(&opts.bodyFile, "body-file", "", "Read the proposed body markdown from a file")
	f.BoolVar(&opts.bodyStdin, "body-stdin", false, "Read the proposed body markdown from stdin")
	// Hidden aliases so muscle memory from 'issues create' does not produce
	// a usage error. reconcile content can land in a description or in a
	// comment depending on the decision, so the neutral noun is canonical.
	f.StringVar(&opts.descAlias, "description", "", "Alias for --body")
	f.StringVar(&opts.descFile, "description-file", "", "Alias for --body-file")
	f.BoolVar(&opts.descStdin, "description-stdin", false, "Alias for --body-stdin")
	for _, name := range []string{"description", "description-file", "description-stdin"} {
		_ = f.MarkHidden(name)
	}
	f.StringVar(&opts.source, "source", "", "Reconcile an existing issue by identifier or UUID (source mode)")
	f.StringVar(&opts.team, "team", "", "Team key, name, or UUID (required)")
	f.StringVar(&opts.dbPath, "db", "", "Database path")

	f.IntVar(&opts.candidates, "candidates", 50, "SQL LIMIT on the candidate query")
	f.StringVar(&opts.matchMode, "match-mode", store.MatchModeAny, "How query tokens are joined in the FTS MATCH expression: any (OR) or all (AND)")
	f.IntVar(&opts.minTokenLen, "min-token-len", 2, "Drop query tokens shorter than this")
	f.IntVar(&opts.maxQueryTokens, "max-query-tokens", 24, "Cap on tokens in the MATCH expression, in order of appearance")
	f.IntVar(&opts.queryBodyChars, "query-body-chars", 500, "Leading bytes of the body folded into the query text, 0 means title only")
	f.Float64Var(&opts.ftsWeightIdentifier, "fts-weight-identifier", 0.5, "bm25 column weight for identifier")
	f.Float64Var(&opts.ftsWeightTitle, "fts-weight-title", 5.0, "bm25 column weight for title")
	f.Float64Var(&opts.ftsWeightDescription, "fts-weight-description", 1.0, "bm25 column weight for description")
	f.StringVar(&opts.candidateGroup, "candidate-group", "active", "State group name resolved through the group registry, membership drives the state signal")
	f.BoolVar(&opts.includeOutOfGroup, "include-out-of-group", true, "Keep out-of-group candidates and penalise them, false drops them instead")
	f.StringVar(&opts.liveSearch, "live-search", liveSearchOff, "Live retrieval leg: off, search, semantic, or both")
	f.IntVar(&opts.liveSearchLimit, "live-search-limit", 25, "first: on the live search leg")

	f.Float64Var(&opts.ftsScale, "fts-scale", 8.0, "Divisor in the bm25 squash: fts_norm = 1 - exp(-magnitude / scale)")
	f.Float64Var(&opts.weightTitleDice, "weight-title-dice", 4.0, "Weight of symmetric title token overlap")
	f.Float64Var(&opts.weightTitleContainment, "weight-title-containment", 1.0, "Weight of asymmetric title token containment")
	f.Float64Var(&opts.weightFTS, "weight-fts", 2.0, "Weight of the normalised bm25 signal")
	f.Float64Var(&opts.weightBodyOverlap, "weight-body-overlap", 1.0, "Weight of body token overlap")
	f.Float64Var(&opts.weightSemantic, "weight-semantic", 1.0, "Weight of the semantic hit signal")
	f.Float64Var(&opts.exactFloor, "exact-floor", 0.95, "Confidence floor applied when folded titles are identical")
	f.Float64Var(&opts.penaltyOutOfGroup, "penalty-out-of-group", 0.65, "Multiplier for candidates outside --candidate-group")
	f.DurationVar(&opts.recencyHalflife, "recency-halflife", 2160*time.Hour, "Halflife of the recency multiplier")
	f.Float64Var(&opts.recencyFloor, "recency-floor", 0.70, "Lower bound of the recency multiplier")
	f.Float64Var(&opts.penaltyProjectMismatch, "penalty-project-mismatch", 0.90, "Multiplier when --project is set and the candidate sits in a different project")
	f.Float64Var(&opts.nearTieEpsilon, "near-tie-epsilon", 0.02, "Confidence gap below which the top two candidates are reported as a near tie")

	f.Float64Var(&opts.thresholdDuplicate, "threshold-duplicate", 0.85, "At or above this confidence: same-work band")
	f.Float64Var(&opts.thresholdRelated, "threshold-related", 0.55, "At or above this confidence, below duplicate: related-work band")
	f.Float64Var(&opts.thresholdCreate, "threshold-create", 0.40, "Below this confidence: create_new. Between create and related: ambiguous")

	f.StringVar(&opts.sameWorkAction, "same-work-action", sameWorkAddComment, "Action inside the same-work band: add_comment, append_description, or link_duplicate")
	f.StringVar(&opts.relatedAction, "related-action", relatedLinkRelated, "Action inside the related-work band: link_related, add_sub_issue, or create_new")
	f.StringVar(&opts.onAmbiguous, "on-ambiguous", onAmbiguousRefuse, "Behaviour in the ambiguous band: refuse or create_new")
	f.StringVar(&opts.onVanishedTarget, "on-vanished-target", onVanishedRefuse, "Behaviour when the chosen target no longer exists upstream: refuse or recompute")
	f.StringVar(&opts.decision, "decision", "", "Force the decision. Scoring still runs and is still reported")
	f.StringVar(&opts.target, "target", "", "Pin the target by identifier or UUID")

	f.BoolVar(&opts.execute, "execute", false, "Perform the decided action. Without it the command is read-only")
	f.StringVar(&opts.appendSeparator, "append-separator", "\n\n", "Inserted between the existing description and the appended body")
	f.StringVar(&opts.appendHeader, "append-header", "", "Optional line placed above the appended body")
	f.StringVar(&opts.appendGuard, "append-guard", appendGuardHash, "Concurrent-edit guard for append_description: hash or off")
	f.StringVar(&opts.expectDescriptionSHA256, "expect-description-sha256", "", "Refuse append_description unless the hydrated description hashes to this value")
	f.StringVar(&opts.relationDirection, "relation-direction", relationDirectionNewToTarget, "Which side of issueRelationCreate the reconciled issue occupies: new-to-target or target-to-new")
	f.StringVar(&opts.duplicateStateName, "duplicate-state-name", "", "Workflow state name for the reconciled issue after link_duplicate, resolved against --team")
	f.StringVar(&opts.duplicateStateType, "duplicate-state-type", "", "Workflow state type for the reconciled issue after link_duplicate, resolved against --team")
	f.BoolVar(&opts.hydrateAlternatives, "hydrate-alternatives", false, "Also hydrate alternatives live, one read each")

	f.StringVar(&opts.project, "project", "", "Project UUID, forwarded to issueCreate and used by --penalty-project-mismatch")
	f.StringVar(&opts.projectName, "project-name", "", "Resolve and attach project by exact name")
	f.IntVar(&opts.priority, "priority", 0, "Priority: 1=Urgent, 2=High, 3=Medium, 4=Low (0=None)")
	f.StringVar(&opts.assignee, "assignee", "", "Assignee user UUID")
	f.StringSliceVar(&opts.labels, "label", nil, "Label UUIDs (repeatable)")
	f.StringVar(&opts.parent, "parent", "", "Parent issue identifier or UUID for a created issue")
	f.StringVar(&opts.state, "state", "", "Workflow state UUID for a created issue")
	f.StringVar(&opts.stateName, "state-name", "", "Workflow state name for a created issue, resolved against --team")
	f.StringVar(&opts.stateType, "state-type", "", "Workflow state type for a created issue, resolved against --team")
}

// validateReconcileFlags runs every check that does not need the store, so a
// bad invocation fails before anything is opened or refreshed.
func validateReconcileFlags(flags *rootFlags, opts *reconcileOptions) error {
	if opts.source != "" && opts.title != "" {
		return usageErr(fmt.Errorf("pass either --title (proposal mode) or --source (source mode), not both"))
	}
	if opts.source == "" && strings.TrimSpace(opts.title) == "" {
		return usageErr(fmt.Errorf("--title is required in proposal mode. Pass --source <ISSUE-REF> to reconcile an issue that already exists"))
	}
	if strings.TrimSpace(opts.team) == "" {
		return usageErr(fmt.Errorf("--team is required (team key like ENG, team name, or team UUID)"))
	}

	if err := requireOneOf("--match-mode", opts.matchMode, store.MatchModeAny, store.MatchModeAll); err != nil {
		return err
	}
	if err := requireOneOf("--live-search", opts.liveSearch, liveSearchOff, liveSearchSearch, liveSearchSemantic, liveSearchBoth); err != nil {
		return err
	}
	if err := requireOneOf("--same-work-action", opts.sameWorkAction, sameWorkAddComment, sameWorkAppendDescription, sameWorkLinkDuplicate); err != nil {
		return err
	}
	if err := requireOneOf("--related-action", opts.relatedAction, relatedLinkRelated, relatedAddSubIssue, relatedCreateNew); err != nil {
		return err
	}
	if err := requireOneOf("--on-ambiguous", opts.onAmbiguous, onAmbiguousRefuse, onAmbiguousCreateNew); err != nil {
		return err
	}
	if err := requireOneOf("--on-vanished-target", opts.onVanishedTarget, onVanishedRefuse, onVanishedRecompute); err != nil {
		return err
	}
	if err := requireOneOf("--append-guard", opts.appendGuard, appendGuardHash, appendGuardOff); err != nil {
		return err
	}
	if err := requireOneOf("--relation-direction", opts.relationDirection, relationDirectionNewToTarget, relationDirectionTargetToNew); err != nil {
		return err
	}

	if opts.candidates < 1 {
		return usageErr(fmt.Errorf("--candidates must be at least 1, got %d", opts.candidates))
	}
	if opts.minTokenLen < 1 {
		return usageErr(fmt.Errorf("--min-token-len must be at least 1, got %d", opts.minTokenLen))
	}
	if opts.maxQueryTokens < 1 {
		return usageErr(fmt.Errorf("--max-query-tokens must be at least 1, got %d", opts.maxQueryTokens))
	}
	if opts.queryBodyChars < 0 {
		return usageErr(fmt.Errorf("--query-body-chars cannot be negative, got %d", opts.queryBodyChars))
	}
	if opts.liveSearchLimit < 1 {
		return usageErr(fmt.Errorf("--live-search-limit must be at least 1, got %d", opts.liveSearchLimit))
	}

	// Threshold ordering. Callers branch on the band, so an inverted ladder
	// would silently reclassify every decision.
	for name, v := range map[string]float64{
		"--threshold-duplicate": opts.thresholdDuplicate,
		"--threshold-related":   opts.thresholdRelated,
		"--threshold-create":    opts.thresholdCreate,
		"--exact-floor":         opts.exactFloor,
		"--recency-floor":       opts.recencyFloor,
	} {
		if v < 0 || v > 1 {
			return usageErr(fmt.Errorf("%s must be between 0 and 1, got %v", name, v))
		}
	}
	if !(opts.thresholdCreate <= opts.thresholdRelated && opts.thresholdRelated <= opts.thresholdDuplicate) {
		return usageErr(fmt.Errorf("thresholds must satisfy 0 <= --threshold-create (%v) <= --threshold-related (%v) <= --threshold-duplicate (%v) <= 1",
			opts.thresholdCreate, opts.thresholdRelated, opts.thresholdDuplicate))
	}

	weights := map[string]float64{
		"--weight-title-dice":        opts.weightTitleDice,
		"--weight-title-containment": opts.weightTitleContainment,
		"--weight-fts":               opts.weightFTS,
		"--weight-body-overlap":      opts.weightBodyOverlap,
		"--weight-semantic":          opts.weightSemantic,
	}
	anyPositive := false
	for name, v := range weights {
		if v < 0 {
			return usageErr(fmt.Errorf("%s cannot be negative, got %v", name, v))
		}
		if v > 0 {
			anyPositive = true
		}
	}
	if !anyPositive {
		return usageErr(fmt.Errorf("at least one scoring weight must be greater than 0"))
	}
	for name, v := range map[string]float64{
		"--penalty-out-of-group":     opts.penaltyOutOfGroup,
		"--penalty-project-mismatch": opts.penaltyProjectMismatch,
	} {
		if v <= 0 || v > 1 {
			return usageErr(fmt.Errorf("%s must be in (0, 1], got %v", name, v))
		}
	}
	if opts.ftsScale <= 0 {
		return usageErr(fmt.Errorf("--fts-scale must be greater than 0, got %v", opts.ftsScale))
	}
	if opts.recencyHalflife <= 0 {
		return usageErr(fmt.Errorf("--recency-halflife must be greater than 0, got %v", opts.recencyHalflife))
	}
	if opts.nearTieEpsilon < 0 || opts.nearTieEpsilon > 1 {
		return usageErr(fmt.Errorf("--near-tie-epsilon must be between 0 and 1, got %v", opts.nearTieEpsilon))
	}

	if opts.decision != "" {
		if err := requireOneOf("--decision", opts.decision,
			decisionCreateNew, decisionAppendDescription, decisionAddSubIssue,
			decisionAddComment, decisionLinkRelated, decisionLinkDuplicate); err != nil {
			return err
		}
		if opts.decision != decisionCreateNew && strings.TrimSpace(opts.target) == "" {
			return usageErr(fmt.Errorf("--decision %s needs --target <ISSUE-REF>. Only --decision create_new can run without one", opts.decision))
		}
	}
	if opts.parent != "" && opts.decision == decisionAddSubIssue {
		return usageErr(fmt.Errorf("--parent conflicts with --decision add_sub_issue: the target is the parent"))
	}
	if opts.duplicateStateName != "" && opts.duplicateStateType != "" {
		return usageErr(fmt.Errorf("pass either --duplicate-state-name or --duplicate-state-type, not both"))
	}
	if opts.project != "" && opts.projectName != "" {
		return usageErr(fmt.Errorf("pass either --project <uuid> or --project-name <name>, not both"))
	}
	stateSelectors := 0
	for _, v := range []string{opts.state, opts.stateName, opts.stateType} {
		if v != "" {
			stateSelectors++
		}
	}
	if stateSelectors > 1 {
		return usageErr(fmt.Errorf("pass exactly one of --state, --state-name, or --state-type"))
	}
	if opts.state != "" && !store.IsUUID(opts.state) {
		return usageErr(fmt.Errorf("--state expects a workflow state UUID (got %q). Use --state-name %q instead", opts.state, opts.state))
	}
	for flagName, value := range map[string]*string{"--state-type": &opts.stateType, "--duplicate-state-type": &opts.duplicateStateType} {
		if *value == "" {
			continue
		}
		normalized, err := normalizeWorkflowStateType(*value)
		if err != nil {
			return fmt.Errorf("%s: %w", flagName, err)
		}
		*value = normalized
	}

	// The live legs contradict an explicit request for local-only data.
	if flags.dataSource == "local" && opts.liveSearch != liveSearchOff {
		return usageErr(fmt.Errorf("--data-source local contradicts --live-search %s: pass one or the other", opts.liveSearch))
	}

	if opts.execute {
		if flags.noInput && !flags.yes {
			return usageErr(fmt.Errorf("--execute needs explicit confirmation under --no-input: pass --yes (or use --agent, which implies both)"))
		}
		// Without hydration neither the current description nor the existing
		// relation set can be known, and both plans need one of them.
		if flags.dataSource == "local" {
			blocked := map[string]bool{
				decisionAppendDescription: true,
				decisionLinkRelated:       true,
				decisionLinkDuplicate:     true,
			}
			if blocked[opts.decision] {
				return usageErr(fmt.Errorf("--execute of %s needs live hydration: drop --data-source local", opts.decision))
			}
			if blocked[opts.sameWorkAction] || blocked[opts.relatedAction] {
				return usageErr(fmt.Errorf("--data-source local cannot execute append_description or link_* decisions: pick a different --same-work-action or --related-action, or drop --data-source local"))
			}
		}
	}
	return nil
}

func requireOneOf(flagName, value string, allowed ...string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return usageErr(fmt.Errorf("%s must be one of %s, got %q", flagName, strings.Join(allowed, ", "), value))
}

func runReconcile(cmd *cobra.Command, flags *rootFlags, opts *reconcileOptions) error {
	if err := validateReconcileFlags(flags, opts); err != nil {
		return err
	}

	title, body, err := readReconcileContent(cmd, opts)
	if err != nil {
		return err
	}

	dbPath := resolveDBPath(opts.dbPath)
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
	}
	defer db.Close()

	// Freshness gate. A refresh that could not run exits 5 rather than
	// serving an empty index, because "no candidates" and "no candidates in
	// a healthy index" are not interchangeable: only the second one may
	// produce create_new.
	freshness, err := ensureIssueSearchFresh(cmd, flags, dbPath, db)
	if err != nil {
		return err
	}

	teamInfo, err := resolveReconcileTeam(db, opts.team)
	if err != nil {
		return err
	}

	predicate, err := resolveCandidateGroup(flags, teamInfo.Key, opts.candidateGroup)
	if err != nil {
		return err
	}

	// A read client is needed for source hydration, the live legs and target
	// hydration. It is built lazily so a fully local run never touches auth.
	var readClient *client.Client
	needClient := func() (*client.Client, error) {
		if readClient != nil {
			return readClient, nil
		}
		c, err := newPortfolioLookupClient(flags)
		if err != nil {
			return nil, err
		}
		readClient = c
		return readClient, nil
	}

	sourceMode := opts.source != ""
	sourceIssueID := ""
	sourceIdentifier := ""
	if sourceMode {
		src, err := loadReconcileSource(db, flags, needClient, opts.source)
		if err != nil {
			return err
		}
		sourceIssueID = src.ID
		sourceIdentifier = src.Identifier
		title = src.Title
		body = src.Description
	}

	query := newReconcileQueryText(title, body, opts.queryBodyChars)
	queryText := title
	if opts.queryBodyChars > 0 && query.Body != "" {
		queryText = title + " " + query.Body
	}
	match, matchTokens := store.IssueSearchFTSQueryWith(queryText, store.FTSQueryOptions{
		MatchMode:   opts.matchMode,
		MinTokenLen: opts.minTokenLen,
		MaxTokens:   opts.maxQueryTokens,
	})

	rows, err := db.SearchIssueCandidates(store.CandidateQuery{
		Match:       match,
		TeamID:      teamInfo.ID,
		Limit:       opts.candidates,
		WeightID:    opts.ftsWeightIdentifier,
		WeightTitle: opts.ftsWeightTitle,
		WeightDesc:  opts.ftsWeightDescription,
		ExcludeID:   sourceIssueID,
	})
	if err != nil {
		return err
	}

	merged := make(map[string]*reconcileCandidate, len(rows)+opts.liveSearchLimit)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		c := &reconcileCandidate{IssueCandidate: row, HasFTS: true, Legs: []string{"local"}}
		merged[row.ID] = c
		order = append(order, row.ID)
	}

	semanticLegCounted := false
	liveContributed := false
	legNotes := []string{"local index"}
	if opts.liveSearch != liveSearchOff {
		c, err := needClient()
		if err != nil {
			return err
		}
		contributed, counted, note, err := runLiveLegs(c, db, flags, opts, predicate, teamInfo, queryText, sourceIssueID, merged, &order)
		if err != nil {
			// A caller that asked for the live leg asked for the recall it
			// provides, so a quiet downgrade to local-only would turn into a
			// false create_new.
			return err
		}
		liveContributed = liveContributed || contributed
		semanticLegCounted = counted
		legNotes = append(legNotes, note...)
	} else {
		legNotes = append(legNotes, "no live search")
	}

	tuning := reconcileTuning{
		FTSScale:               opts.ftsScale,
		WeightTitleDice:        opts.weightTitleDice,
		WeightTitleContainment: opts.weightTitleContainment,
		WeightFTS:              opts.weightFTS,
		WeightBodyOverlap:      opts.weightBodyOverlap,
		WeightSemantic:         opts.weightSemantic,
		ExactFloor:             opts.exactFloor,
		PenaltyOutOfGroup:      opts.penaltyOutOfGroup,
		RecencyHalflife:        opts.recencyHalflife,
		RecencyFloor:           opts.recencyFloor,
		PenaltyProjectMismatch: opts.penaltyProjectMismatch,
	}

	now := time.Now().UTC()
	scored := make([]scoredCandidate, 0, len(order))
	for _, id := range order {
		cand := merged[id]
		s := scoreCandidate(query, *cand, tuning, predicate, semanticLegCounted, opts.project, now)
		if !opts.includeOutOfGroup && !s.InGroup {
			continue
		}
		scored = append(scored, scoredCandidate{cand: *cand, score: s})
	}
	rankCandidates(scored)

	doc, execErr := decideAndMaybeExecute(cmd, flags, opts, decideInputs{
		db:               db,
		dbPath:           dbPath,
		needClient:       needClient,
		scored:           scored,
		teamInfo:         teamInfo,
		sourceMode:       sourceMode,
		sourceIssueID:    sourceIssueID,
		sourceIdentifier: sourceIdentifier,
		title:            title,
		body:             body,
		matchTokens:      matchTokens,
		match:            match,
		legNotes:         legNotes,
		freshness:        freshness,
		predicate:        predicate,
	})
	if doc == nil {
		return execErr
	}

	source := "local"
	if liveContributed {
		source = "mixed"
	}
	reason := "reconcile"
	switch doc.ThresholdsUsed["band"] {
	case bandForced:
		reason = "reconcile_forced"
	case bandAmbiguous:
		reason = "reconcile_ambiguous"
	}

	// A refusal raised before the plan started leaves executed null, so the
	// document has to carry the reason itself. Without this the only place the
	// reason existed was a trailing text line beside the JSON.
	attachExecutionFailure(doc, execErr)

	if err := emitReconcileDecision(cmd, flags, doc, DataProvenance{
		Source:       source,
		ResourceType: "issues",
		Reason:       reason,
		Freshness:    freshness,
	}); err != nil {
		return err
	}
	if execErr == nil {
		return nil
	}
	// The decision document already carries the failure, so nothing else may
	// reach stdout.
	if ExitCode(execErr) == 6 && flags.allowPartialFailure {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v (downgraded by --allow-partial-failure)\n", execErr)
		return nil
	}
	// jsonErrorMode is the predicate finalizeError uses, so marking the error
	// as already surfaced here is what keeps a second envelope off stdout, and
	// --agent with --json=false cannot slip past it.
	if jsonErrorMode(flags, nil) {
		flags.errorWritten = true
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", execErr)
	}
	return execErr
}

// readReconcileContent resolves the body from the canonical triple or its
// hidden aliases, and refuses more than one source.
func readReconcileContent(cmd *cobra.Command, opts *reconcileOptions) (string, string, error) {
	body, bodySet, err := readMarkdownBody(cmd, markdownBodySpec{
		InlineFlag: "body", Inline: opts.body,
		FileFlag: "body-file", File: opts.bodyFile,
		StdinFlag: "body-stdin", Stdin: opts.bodyStdin,
		Label: "body",
	})
	if err != nil {
		return "", "", asUsageErr(err)
	}
	alias, aliasSet, err := readMarkdownBody(cmd, markdownBodySpec{
		InlineFlag: "description", Inline: opts.descAlias,
		FileFlag: "description-file", File: opts.descFile,
		StdinFlag: "description-stdin", Stdin: opts.descStdin,
		Label: "body",
	})
	if err != nil {
		return "", "", asUsageErr(err)
	}
	if bodySet && aliasSet {
		return "", "", usageErr(fmt.Errorf("pass only one body source: --body/--body-file/--body-stdin or their hidden --description* aliases"))
	}
	if aliasSet {
		body = alias
	}
	return opts.title, body, nil
}

func asUsageErr(err error) error {
	if err == nil {
		return nil
	}
	var typed *cliError
	if errors.As(err, &typed) {
		return err
	}
	return usageErr(err)
}

// resolveReconcileTeam maps --team to both the UUID the candidate query needs
// and the key the group registry needs.
func resolveReconcileTeam(db *store.Store, input string) (issueTeamInfo, error) {
	teamID, err := resolveTeamFilter(db, input)
	if err != nil {
		if !errors.Is(err, errTeamFilterNotFound) {
			return issueTeamInfo{}, err
		}
		return issueTeamInfo{}, notFoundErr(fmt.Errorf("%w. Run 'linear-pp-cli sync' if the team was added recently", err))
	}
	info := issueTeamInfo{ID: teamID}
	teams, err := db.ListTeams()
	if err != nil {
		return info, nil
	}
	for _, raw := range teams {
		var t struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		if t.ID == teamID {
			info.Key = t.Key
			break
		}
	}
	return info, nil
}

// resolveCandidateGroup resolves --candidate-group through the group
// registry. The split between exit 2 and exit 10 is owned by the registry: a
// bad token is a usage mistake, a malformed registry is a config failure, and
// reconcile passes both through unwrapped so that `reconcile
// --candidate-group typo` and `issues list --state typo` exit identically.
func resolveCandidateGroup(flags *rootFlags, teamKey, name string) (groups.Predicate, error) {
	cfgDir := ""
	if flags.configPath != "" {
		cfgDir = groups.DirForConfigPath(flags.configPath)
	}
	predicate, err := groups.Resolve(cfgDir, teamKey, name)
	if err != nil {
		if errors.Is(err, groups.ErrUnknownToken) {
			return predicate, usageErr(err)
		}
		var typed *cliError
		if errors.As(err, &typed) {
			return predicate, err
		}
		return predicate, configErr(err)
	}
	return predicate, nil
}

// reconcileSource is the issue behind --source.
type reconcileSource struct {
	ID          string
	Identifier  string
	Title       string
	Description string
}

// loadReconcileSource reads the source issue from the local store, falling
// back to a live read when the store does not have it and the data source
// permits one.
func loadReconcileSource(db *store.Store, flags *rootFlags, needClient func() (*client.Client, error), ref string) (reconcileSource, error) {
	var src reconcileSource
	if raw, err := db.IssueByRef(ref); err == nil && len(raw) > 0 {
		var row struct {
			ID          string `json:"id"`
			Identifier  string `json:"identifier"`
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &row); err == nil && row.ID != "" {
			return reconcileSource{ID: row.ID, Identifier: row.Identifier, Title: row.Title, Description: row.Description}, nil
		}
	}
	if flags.dataSource == "local" {
		return src, notFoundErr(fmt.Errorf("--source %q is not in the local store and --data-source local forbids a live read. Run 'linear-pp-cli sync'", ref))
	}
	c, err := needClient()
	if err != nil {
		return src, err
	}
	raw, err := fetchIssueLive(c, ref)
	if err != nil {
		return src, classifyLiveReadError(err, flags)
	}
	var row struct {
		ID          string `json:"id"`
		Identifier  string `json:"identifier"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return src, fmt.Errorf("parsing --source issue: %w", err)
	}
	if row.ID == "" {
		return src, notFoundErr(fmt.Errorf("--source %q not found", ref))
	}
	return reconcileSource{ID: row.ID, Identifier: row.Identifier, Title: row.Title, Description: row.Description}, nil
}

// runLiveLegs adds live candidates to the merged set. It reports whether the
// live legs contributed anything, whether the semantic signal may be counted,
// and one note per leg for the evidence list.
func runLiveLegs(
	c *client.Client,
	db *store.Store,
	flags *rootFlags,
	opts *reconcileOptions,
	predicate groups.Predicate,
	teamInfo issueTeamInfo,
	queryText string,
	excludeID string,
	merged map[string]*reconcileCandidate,
	order *[]string,
) (bool, bool, []string, error) {
	var notes []string
	contributed := false
	semanticCounted := false

	absorb := func(node client.SearchIssueNode, leg string, semantic bool) {
		if node.ID == "" || node.ID == excludeID {
			return
		}
		contributed = true
		existing, ok := merged[node.ID]
		if !ok {
			cand := &reconcileCandidate{Legs: []string{leg}}
			cand.ID = node.ID
			cand.Identifier = node.Identifier
			cand.Title = node.Title
			cand.Description = node.Description
			cand.StateName = node.State.Name
			cand.StateType = node.State.Type
			cand.TeamID = node.Team.ID
			if node.Project != nil {
				cand.ProjectID = node.Project.ID
			}
			cand.UpdatedAt = parseSearchTime(node.UpdatedAt)
			cand.CreatedAt = parseSearchTime(node.CreatedAt)
			cand.URL = node.URL
			cand.SemanticHit = semantic
			merged[node.ID] = cand
			*order = append(*order, node.ID)
			upsertSearchNode(db, node)
			return
		}
		existing.Legs = append(existing.Legs, leg)
		if existing.URL == "" {
			existing.URL = node.URL
		}
		if semantic {
			existing.SemanticHit = true
		}
		upsertSearchNode(db, node)
	}

	if opts.liveSearch == liveSearchSearch || opts.liveSearch == liveSearchBoth {
		searchOpts := client.SearchIssuesOptions{
			Term:   queryText,
			First:  opts.liveSearchLimit,
			TeamID: teamInfo.ID,
		}
		// When out-of-group candidates are dropped rather than penalised,
		// the live leg is bounded by the same declared semantics.
		if !opts.includeOutOfGroup {
			if stateFilter := groups.LiveFilter(predicate); len(stateFilter) > 0 {
				searchOpts.Filter = map[string]any{"state": stateFilter}
			}
		}
		res, err := c.SearchIssues(searchOpts)
		if err != nil {
			return contributed, semanticCounted, notes, classifyAPIError(fmt.Errorf("searchIssues leg failed: %w", err), flags)
		}
		for _, node := range res.Nodes {
			absorb(node, "search", false)
		}
		notes = append(notes, fmt.Sprintf("live searchIssues returned %d of %d", len(res.Nodes), res.TotalCount))
	}

	if opts.liveSearch == liveSearchSemantic || opts.liveSearch == liveSearchBoth {
		res, err := c.SemanticSearchIssues(queryText, opts.liveSearchLimit)
		if err != nil {
			return contributed, semanticCounted, notes, classifyAPIError(fmt.Errorf("semanticSearch leg failed: %w", err), flags)
		}
		if res.Enabled {
			semanticCounted = true
			for _, node := range res.Nodes {
				if teamInfo.ID != "" && node.Team.ID != "" && node.Team.ID != teamInfo.ID {
					continue
				}
				absorb(node, "semantic", true)
			}
			notes = append(notes, fmt.Sprintf("live semanticSearch enabled, %d results", len(res.Nodes)))
		} else {
			notes = append(notes, "live semanticSearch reported disabled for this workspace, signal absent rather than negative")
		}
	}
	return contributed, semanticCounted, notes, nil
}

// upsertSearchNode write-throughs a live node into the local store, following
// the precedent set by the live issue list path.
func upsertSearchNode(db *store.Store, node client.SearchIssueNode) {
	if db == nil || node.ID == "" {
		return
	}
	raw, err := searchNodePayload(node)
	if err != nil {
		return
	}
	_ = db.UpsertIssue(node.ID, node.Identifier, node.Title, raw)
}

func parseSearchTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

// decideInputs bundles everything the ladder needs.
type decideInputs struct {
	db               *store.Store
	dbPath           string
	needClient       func() (*client.Client, error)
	scored           []scoredCandidate
	teamInfo         issueTeamInfo
	sourceMode       bool
	sourceIssueID    string
	sourceIdentifier string
	title            string
	body             string
	matchTokens      []string
	match            string
	legNotes         []string
	freshness        issueSearchFreshness
	predicate        groups.Predicate
}

// decideAndMaybeExecute walks the ladder, hydrates, builds the decision
// object and, under --execute, runs the plan. It always returns a document
// when one could be built, because a caller that asked for a decision gets
// the decision even when the write failed.
func decideAndMaybeExecute(cmd *cobra.Command, flags *rootFlags, opts *reconcileOptions, in decideInputs) (*reconcileDecisionDoc, error) {
	scored := in.scored

	// --target pins the candidate the decision acts on.
	if opts.target != "" {
		pinned, err := pinReconcileTarget(flags, in, opts.target)
		if err != nil {
			return nil, err
		}
		scored = pinned
	}

	var (
		chosen    *scoredCandidate
		band      string
		decision  string
		hydration *hydratedIssue
		// vanished holds candidates whose hydration proved the local row
		// outlived the issue. Nothing deletes the row, so they stay visible
		// in alternatives rather than disappearing from the report.
		vanished []scoredCandidate
	)

	for {
		chosen = nil
		if len(scored) > 0 {
			chosen = &scored[0]
		}
		topConfidence := 0.0
		if chosen != nil {
			topConfidence = chosen.score.Confidence
		}

		switch {
		case opts.decision != "":
			band = bandForced
			decision = opts.decision
		default:
			band = bandFor(topConfidence, opts)
			decision = decisionForBand(band, opts)
		}
		if chosen == nil {
			// Nothing scored, so there is nothing to act on.
			band = bandCreate
			if opts.decision != "" {
				band = bandForced
			}
			decision = decisionCreateNew
		}
		if decision == decisionCreateNew {
			chosen = nil
		}

		// Hydration is required for correctness, not decoration: it supplies
		// the current description, the url the store cannot hold, and the
		// proof that the local row did not outlive the issue.
		hydration = nil
		if chosen != nil && flags.dataSource != "local" {
			c, err := in.needClient()
			if err != nil {
				return nil, err
			}
			h, err := hydrateIssue(c, chosen.cand.ID)
			if err != nil {
				return nil, classifyLiveReadError(fmt.Errorf("hydrating %s: %w", chosen.cand.Identifier, err), flags)
			}
			if h == nil {
				if opts.onVanishedTarget == onVanishedRecompute {
					// Drop the candidate, rescore, and continue with the
					// next best.
					dropped := scored[0]
					dropped.cand.Legs = append(append([]string{}, dropped.cand.Legs...), "hydration returned null, candidate dropped")
					vanished = append(vanished, dropped)
					scored = append(scored[:0:0], scored[1:]...)
					continue
				}
				return nil, notFoundErr(fmt.Errorf("target %s no longer exists upstream: the local row outlived the issue. Nothing was written. Re-run after pruning the store, or pass --on-vanished-target recompute to fall through to the next best candidate", chosen.cand.Identifier))
			}
			hydration = h
		}
		break
	}

	if opts.parent != "" && decision == decisionAddSubIssue {
		return nil, usageErr(fmt.Errorf("--parent conflicts with a decision of add_sub_issue: the target is the parent"))
	}

	nearTie := false
	if len(scored) > 1 && math.Abs(scored[0].score.Confidence-scored[1].score.Confidence) <= opts.nearTieEpsilon {
		nearTie = true
	}

	doc := &reconcileDecisionDoc{
		SchemaVersion:  reconcileDecisionSchemaVersion,
		Decision:       decision,
		Alternatives:   []reconcileAlternative{},
		ThresholdsUsed: reconcileThresholdsUsed(opts, in.predicate, band),
		DryRun:         true,
	}

	bandDetail := reconcileBandDetail(band, opts, nearTie)
	if chosen != nil {
		doc.Confidence = chosen.score.Confidence
		target := targetFromCandidate(chosen.cand)
		applyHydration(&target, hydration)
		doc.Target = &target
		doc.Evidence = buildEvidence(chosen.cand, chosen.score, tuningFromOptions(opts), band, bandDetail)
	} else {
		// create_new never reports a confidence above 0. Confidence
		// describes a candidate, and create_new has none, so a caller must
		// not read create_new plus high confidence as "verified unique".
		doc.Confidence = 0
		doc.Evidence = []reconcileEvidence{
			{
				Signal: "source_leg",
				Value:  strings.Join(in.legNotes, "+"),
				Weight: 0,
				Detail: reconcileNoCandidateDetail(in, opts),
			},
			{
				Signal: "band",
				Value:  band,
				Weight: 0,
				Detail: bandDetail,
			},
		}
	}

	// Alternatives are every other scored candidate, with the decision each
	// would have produced on its own confidence.
	start := 0
	if chosen != nil {
		start = 1
	}
	for i := start; i < len(scored); i++ {
		alt := scored[i]
		altBand := bandFor(alt.score.Confidence, opts)
		altTarget := targetFromCandidate(alt.cand)
		if opts.hydrateAlternatives && flags.dataSource != "local" {
			c, err := in.needClient()
			if err != nil {
				return nil, err
			}
			if h, err := hydrateIssue(c, alt.cand.ID); err == nil && h != nil {
				applyHydration(&altTarget, h)
			}
		}
		doc.Alternatives = append(doc.Alternatives, reconcileAlternative{
			Decision:   decisionForBand(altBand, opts),
			Target:     altTarget,
			Confidence: alt.score.Confidence,
			Evidence:   buildEvidence(alt.cand, alt.score, tuningFromOptions(opts), altBand, reconcileBandDetail(altBand, opts, false)),
		})
	}
	for _, gone := range vanished {
		goneBand := bandFor(gone.score.Confidence, opts)
		doc.Alternatives = append(doc.Alternatives, reconcileAlternative{
			Decision:   decisionForBand(goneBand, opts),
			Target:     targetFromCandidate(gone.cand),
			Confidence: gone.score.Confidence,
			Evidence:   buildEvidence(gone.cand, gone.score, tuningFromOptions(opts), goneBand, reconcileBandDetail(goneBand, opts, false)),
		})
	}

	if !opts.execute {
		return doc, nil
	}

	// The ambiguous band under refuse produced a decision but must not act
	// on it: the best candidate is too similar to dismiss and too weak to
	// act on, and that call belongs to the operator.
	if band == bandAmbiguous && opts.onAmbiguous == onAmbiguousRefuse {
		return doc, partialFailureErr(fmt.Errorf("confidence %.4f fell in the ambiguous band [%v, %v): nothing was written. Raise --threshold-related, pass --on-ambiguous create_new, or pin the call with --decision and --target",
			doc.Confidence, opts.thresholdCreate, opts.thresholdRelated))
	}

	// Stale-by-request plus a write on someone else's issue needs the
	// explicit confirmation, not the interactive one.
	if decision != decisionCreateNew && in.freshness.StalePolicy == "allow" && !flags.yes {
		return doc, usageErr(fmt.Errorf("executing %s on a snapshot you explicitly allowed to be stale (--data-source local or --max-age 0) needs --yes", decision))
	}

	if err := confirmReconcileExecution(cmd, flags, decision, doc.Target); err != nil {
		return doc, err
	}

	writeClient, err := flags.newClient()
	if err != nil {
		return doc, err
	}
	readClient, err := in.needClient()
	if err != nil {
		return doc, err
	}

	ctx := &reconcileExecContext{
		cmd:              cmd,
		flags:            flags,
		opts:             opts,
		db:               in.db,
		dbPath:           in.dbPath,
		write:            writeClient,
		read:             readClient,
		sourceMode:       in.sourceMode,
		sourceIssueID:    in.sourceIssueID,
		sourceIdentifier: in.sourceIdentifier,
		teamID:           in.teamInfo.ID,
		teamInfo:         in.teamInfo,
		title:            in.title,
		body:             in.body,
		target:           doc.Target,
		hydration:        hydration,
		planDryRun:       flags.dryRun,
	}
	doc.DryRun = flags.dryRun
	executed, execErr := executeDecision(ctx, decision)
	doc.Executed = executed
	return doc, execErr
}

func tuningFromOptions(opts *reconcileOptions) reconcileTuning {
	return reconcileTuning{
		FTSScale:               opts.ftsScale,
		WeightTitleDice:        opts.weightTitleDice,
		WeightTitleContainment: opts.weightTitleContainment,
		WeightFTS:              opts.weightFTS,
		WeightBodyOverlap:      opts.weightBodyOverlap,
		WeightSemantic:         opts.weightSemantic,
		ExactFloor:             opts.exactFloor,
		PenaltyOutOfGroup:      opts.penaltyOutOfGroup,
		RecencyHalflife:        opts.recencyHalflife,
		RecencyFloor:           opts.recencyFloor,
		PenaltyProjectMismatch: opts.penaltyProjectMismatch,
	}
}

func reconcileBandDetail(band string, opts *reconcileOptions, nearTie bool) string {
	var detail string
	switch band {
	case bandSameWork:
		detail = fmt.Sprintf("confidence at or above --threshold-duplicate %v, action from --same-work-action", opts.thresholdDuplicate)
	case bandRelatedWork:
		detail = fmt.Sprintf("confidence in [%v, %v), action from --related-action", opts.thresholdRelated, opts.thresholdDuplicate)
	case bandAmbiguous:
		detail = fmt.Sprintf("confidence in [%v, %v), too similar to dismiss and too weak to act on, governed by --on-ambiguous %s", opts.thresholdCreate, opts.thresholdRelated, opts.onAmbiguous)
	case bandCreate:
		detail = fmt.Sprintf("confidence below --threshold-create %v", opts.thresholdCreate)
	case bandForced:
		detail = "--decision overrode the ladder. Scoring still ran and is reported in full"
	}
	if nearTie {
		detail += ". near_tie: the second candidate is within --near-tie-epsilon of the first, both appear in alternatives"
	}
	return detail
}

// reconcileNoCandidateDetail explains an empty candidate set honestly. Recall
// failure is invisible: FTS5 has no synonyms, no fuzzy matching and no
// transposition tolerance, so create_new is evidence about the index, never
// proof about the workspace.
func reconcileNoCandidateDetail(in decideInputs, opts *reconcileOptions) string {
	parts := []string{strings.Join(in.legNotes, ", ")}
	if in.freshness.Refreshed {
		parts = append(parts, fmt.Sprintf("index refreshed, 0 candidates matched, local_issue_count %d", in.freshness.LocalIssueCount))
	} else {
		parts = append(parts, fmt.Sprintf("0 candidates matched, local_issue_count %d", in.freshness.LocalIssueCount))
	}
	if opts.liveSearch == liveSearchOff {
		parts = append(parts, "local index only, no live search")
	}
	if len(in.matchTokens) > 0 {
		parts = append(parts, "query terms: "+strings.Join(in.matchTokens, ", "))
	} else {
		parts = append(parts, "no query tokens survived --min-token-len")
	}
	return strings.Join(parts, ". ")
}

// pinReconcileTarget reorders the scored set so the pinned issue is first,
// pulling it in from the store or from a live read when scoring never saw it.
func pinReconcileTarget(flags *rootFlags, in decideInputs, ref string) ([]scoredCandidate, error) {
	scored := in.scored
	for i := range scored {
		if strings.EqualFold(scored[i].cand.ID, ref) || strings.EqualFold(scored[i].cand.Identifier, ref) {
			pinned := scored[i]
			rest := make([]scoredCandidate, 0, len(scored))
			rest = append(rest, pinned)
			for j := range scored {
				if j != i {
					rest = append(rest, scored[j])
				}
			}
			return rest, nil
		}
	}

	// Not in the candidate set. Pull it in so --decision plus --target can
	// pin a call the caller already reviewed.
	if raw, err := in.db.IssueByRef(ref); err == nil && len(raw) > 0 {
		var row struct {
			ID          string `json:"id"`
			Identifier  string `json:"identifier"`
			Title       string `json:"title"`
			Description string `json:"description"`
			UpdatedAt   string `json:"updatedAt"`
			State       struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"state"`
			Project *struct {
				ID string `json:"id"`
			} `json:"project"`
		}
		if err := json.Unmarshal(raw, &row); err == nil && row.ID != "" {
			cand := reconcileCandidate{Legs: []string{"pinned by --target"}}
			cand.ID = row.ID
			cand.Identifier = row.Identifier
			cand.Title = row.Title
			cand.Description = row.Description
			cand.StateName = row.State.Name
			cand.StateType = row.State.Type
			cand.UpdatedAt = parseSearchTime(row.UpdatedAt)
			if row.Project != nil {
				cand.ProjectID = row.Project.ID
			}
			return append([]scoredCandidate{{cand: cand}}, scored...), nil
		}
	}
	if flags.dataSource == "local" {
		return nil, notFoundErr(fmt.Errorf("--target %q is not in the local store and --data-source local forbids a live read", ref))
	}
	c, err := in.needClient()
	if err != nil {
		return nil, err
	}
	issueID := ref
	if !store.IsUUID(ref) {
		resolved, err := resolveIssueID(c, ref)
		if err != nil {
			return nil, classifyLiveReadError(err, flags)
		}
		issueID = resolved
	}
	h, err := hydrateIssue(c, issueID)
	if err != nil {
		return nil, classifyLiveReadError(err, flags)
	}
	if h == nil {
		return nil, notFoundErr(fmt.Errorf("--target %q not found", ref))
	}
	cand := reconcileCandidate{Legs: []string{"pinned by --target"}}
	cand.ID = h.ID
	cand.Identifier = h.Identifier
	cand.Title = h.Title
	cand.Description = h.Description
	cand.StateName = h.StateName
	cand.StateType = h.StateType
	cand.ProjectID = h.ProjectID
	cand.UpdatedAt = parseSearchTime(h.UpdatedAt)
	cand.URL = h.URL
	return append([]scoredCandidate{{cand: cand}}, scored...), nil
}

// reconcileThresholdsUsed reports every knob that shaped the number, keyed by
// the flag name without the leading dashes, so the decision is reproducible.
func reconcileThresholdsUsed(opts *reconcileOptions, predicate groups.Predicate, band string) map[string]any {
	return map[string]any{
		"threshold_duplicate":      opts.thresholdDuplicate,
		"threshold_related":        opts.thresholdRelated,
		"threshold_create":         opts.thresholdCreate,
		"exact_floor":              opts.exactFloor,
		"fts_scale":                opts.ftsScale,
		"weight_title_dice":        opts.weightTitleDice,
		"weight_title_containment": opts.weightTitleContainment,
		"weight_fts":               opts.weightFTS,
		"weight_body_overlap":      opts.weightBodyOverlap,
		"weight_semantic":          opts.weightSemantic,
		"penalty_out_of_group":     opts.penaltyOutOfGroup,
		"penalty_project_mismatch": opts.penaltyProjectMismatch,
		"recency_halflife_days":    opts.recencyHalflife.Hours() / 24,
		"recency_floor":            opts.recencyFloor,
		"near_tie_epsilon":         opts.nearTieEpsilon,
		"candidates":               opts.candidates,
		"match_mode":               opts.matchMode,
		"min_token_len":            opts.minTokenLen,
		"max_query_tokens":         opts.maxQueryTokens,
		"query_body_chars":         opts.queryBodyChars,
		"fts_weight_identifier":    opts.ftsWeightIdentifier,
		"fts_weight_title":         opts.ftsWeightTitle,
		"fts_weight_description":   opts.ftsWeightDescription,
		"candidate_group":          opts.candidateGroup,
		"candidate_group_source":   predicate.Source(),
		"include_out_of_group":     opts.includeOutOfGroup,
		"live_search":              opts.liveSearch,
		"live_search_limit":        opts.liveSearchLimit,
		"same_work_action":         opts.sameWorkAction,
		"related_action":           opts.relatedAction,
		"on_ambiguous":             opts.onAmbiguous,
		"on_vanished_target":       opts.onVanishedTarget,
		"relation_direction":       opts.relationDirection,
		"append_guard":             opts.appendGuard,
		"band":                     band,
	}
}

// confirmReconcileExecution applies the confirmation contract: confirm before
// acting unless --yes, and hard-error under --no-input when confirmation was
// not waived.
func confirmReconcileExecution(cmd *cobra.Command, flags *rootFlags, decision string, target *reconcileTarget) error {
	if flags.yes || flags.dryRun {
		return nil
	}
	if flags.noInput {
		return usageErr(fmt.Errorf("--execute needs explicit confirmation under --no-input: pass --yes"))
	}
	name := "a new issue"
	if target != nil {
		name = target.Identifier
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Execute %s against %s? [y/N] ", decision, name)
	var resp string
	fmt.Fscanln(cmd.InOrStdin(), &resp)
	if !strings.EqualFold(resp, "y") && !strings.EqualFold(resp, "yes") {
		return usageErr(fmt.Errorf("aborted"))
	}
	return nil
}

// emitReconcileDecision writes the decision inside the provenance envelope,
// or renders it for a human on a terminal.
func emitReconcileDecision(cmd *cobra.Command, flags *rootFlags, doc *reconcileDecisionDoc, prov DataProvenance) error {
	asJSON := flags.asJSON || !isTerminal(cmd.OutOrStdout())
	if !asJSON {
		return renderReconcileHuman(cmd, doc)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	wrapped, err := wrapWithProvenance(data, prov)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}

func renderReconcileHuman(cmd *cobra.Command, doc *reconcileDecisionDoc) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s  (confidence %.4f, band %v)\n", bold(doc.Decision), doc.Confidence, doc.ThresholdsUsed["band"])
	if doc.Target != nil {
		url := ""
		if doc.Target.URL != nil {
			url = "  " + *doc.Target.URL
		}
		fmt.Fprintf(out, "target: %s  %s%s\n", doc.Target.Identifier, doc.Target.Title, url)
	}
	shown := 0
	for _, ev := range doc.Evidence {
		if ev.Value == nil {
			continue
		}
		fmt.Fprintf(out, "  %-18s %-10v weight %-6.2f %s\n", ev.Signal, ev.Value, ev.Weight, ev.Detail)
		shown++
		if shown == 3 {
			break
		}
	}
	if len(doc.Alternatives) > 0 {
		fmt.Fprintln(out, "\nalternatives:")
		tw := newTabWriter(out)
		fmt.Fprintln(tw, "ID\tCONFIDENCE\tWOULD BE\tTITLE")
		for _, alt := range doc.Alternatives {
			fmt.Fprintf(tw, "%s\t%.4f\t%s\t%s\n", alt.Target.Identifier, alt.Confidence, alt.Decision, truncate(alt.Target.Title, 45))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	if doc.Executed != nil {
		fmt.Fprintf(out, "\nexecuted: %s -> %s\n", doc.Executed.Action, doc.Executed.Result.Status)
		for _, step := range doc.Executed.Result.Steps {
			fmt.Fprintf(out, "  %-22s %s\n", step.Mutation, step.Status)
		}
	} else if doc.DryRun {
		fmt.Fprintln(out, "\nno write attempted, pass --execute to act on this decision")
	}
	return nil
}
