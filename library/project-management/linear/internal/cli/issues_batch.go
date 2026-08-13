package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// Issue batch mutations (GAP-034).
//
// Bulk triage against a rate-limited API was N round trips with no
// backpressure signal. Linear ships two transactional batch mutations and
// neither had a surface:
//
//	issueBatchCreate(input: IssueBatchCreateInput!)   input = { issues: [IssueCreateInput!]! }
//	issueBatchUpdate(input: IssueUpdateInput!, ids: [UUID!]!)
//
// Both cap at 50 per call, both return IssueBatchPayload { issues, success }.
// The two commands are registered on the issues parent in issues.go.

// issueBatchMax is Linear's own ceiling, stated in both places:
// IssueBatchCreateInput is documented "Up to 50 issues can be created in a
// single batch" and issueBatchUpdate.ids is "Can't be more than 50 at a
// time". Exceeding it is refused here rather than round-tripped, and the
// batch is never silently split because the whole point of these mutations
// is that they are one transaction.
const issueBatchMax = 50

// issueCreateInputFields is the IssueCreateInput field set, transcribed from
// api-inventory.json. A record key outside this set (and outside the
// resolution aliases below) is a usage error, so a typo like "titel" or
// "team_id" fails loudly instead of being dropped on the floor by the API.
//
// boardOrder is deliberately absent: Linear marks it deprecated.
var issueCreateInputFields = map[string]bool{
	"id":                         true,
	"title":                      true,
	"description":                true,
	"descriptionData":            true,
	"assigneeId":                 true,
	"delegateId":                 true,
	"parentId":                   true,
	"priority":                   true,
	"estimate":                   true,
	"subscriberIds":              true,
	"labelIds":                   true,
	"teamId":                     true,
	"cycleId":                    true,
	"projectId":                  true,
	"projectMilestoneId":         true,
	"lastAppliedTemplateId":      true,
	"stateId":                    true,
	"referenceCommentId":         true,
	"sourceCommentId":            true,
	"sourcePullRequestCommentId": true,
	"sortOrder":                  true,
	"prioritySortOrder":          true,
	"subIssueSortOrder":          true,
	"dueDate":                    true,
	"createAsUser":               true,
	"displayIconUrl":             true,
	"preserveSortOrderOnCreate":  true,
	"createdAt":                  true,
	"slaBreachesAt":              true,
	"slaStartedAt":               true,
	"templateId":                 true,
	"completedAt":                true,
	"slaType":                    true,
	"useDefaultTemplate":         true,
	"releaseIds":                 true,
	"inheritsSharedAccess":       true,
}

// issueUpdateInputFields is the IssueUpdateInput field set, same source and
// same reason. Used to validate --set. boardOrder is deprecated and absent.
var issueUpdateInputFields = map[string]bool{
	"title":                     true,
	"description":               true,
	"descriptionData":           true,
	"assigneeId":                true,
	"delegateId":                true,
	"parentId":                  true,
	"priority":                  true,
	"estimate":                  true,
	"subscriberIds":             true,
	"labelIds":                  true,
	"addedLabelIds":             true,
	"removedLabelIds":           true,
	"releaseIds":                true,
	"addedReleaseIds":           true,
	"removedReleaseIds":         true,
	"teamId":                    true,
	"cycleId":                   true,
	"projectId":                 true,
	"projectMilestoneId":        true,
	"lastAppliedTemplateId":     true,
	"stateId":                   true,
	"sortOrder":                 true,
	"prioritySortOrder":         true,
	"subIssueSortOrder":         true,
	"dueDate":                   true,
	"inheritsSharedAccess":      true,
	"trusted":                   true,
	"trashed":                   true,
	"slaBreachesAt":             true,
	"slaStartedAt":              true,
	"snoozedUntilAt":            true,
	"snoozedById":               true,
	"slaType":                   true,
	"autoClosedByParentClosing": true,
}

// Resolution aliases accepted in a batch-create record. None of them is an
// IssueCreateInput field: each is resolved to a real one before the call, and
// the alias key itself is never sent.
const (
	batchAliasTeam      = "team"      // team key, team name, or UUID  -> teamId
	batchAliasStateName = "stateName" // workflow state name           -> stateId
	batchAliasStateType = "stateType" // WorkflowState.type value      -> stateId
)

// batchIssue mirrors the batchIssueFields projection in
// internal/client/batch_queries.go, which mirrors what issueCreate selects,
// so a batch-created issue feeds the pp_created ledger and the local
// write-back through the same shape a single create does.
type batchIssue struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Priority    int    `json:"priority"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Team        struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	} `json:"team"`
	State struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Assignee *struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"assignee,omitempty"`
	Project *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project,omitempty"`
	Parent *struct {
		ID         string `json:"id"`
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
	} `json:"parent,omitempty"`
}

// ---------------------------------------------------------------------------
// issues batch-create
// ---------------------------------------------------------------------------

func newIssuesBatchCreateCmd(flags *rootFlags) *cobra.Command {
	var file string
	var teamFlag, projectFlag string
	var labelsFlag []string
	var session string
	cmd := &cobra.Command{
		Use:   "batch-create --file <path>",
		Short: "Create up to 50 issues in one transaction and record them in the pp_created ledger",
		Long: `Create many issues in a single issueBatchCreate call.

--file reads JSONL (one JSON object per line), a JSON array, or a single
JSON object. Pass - to read stdin. Each record is an IssueCreateInput, so
keys are Linear's own camelCase input field names (title, teamId,
description, assigneeId, labelIds, priority, estimate, dueDate, parentId,
projectId, cycleId, stateId, and the rest). An unknown key is refused by
name rather than silently dropped.

Three resolution aliases are accepted on top of those, and none of them is
sent to the API:

  team        a team key, team name, or UUID, resolved to teamId
  stateName   a workflow state name, resolved to stateId within the record's team
  stateType   a WorkflowState.type value, resolved to stateId within the record's team

--team, --project and --label supply defaults for records that do not carry
their own teamId, projectId or labelIds.

Linear caps the batch at 50. A larger file is refused rather than split,
because a split is no longer one transaction.

Every created issue is written to the local pp_created ledger with a session
tag, exactly like 'issues create', so pp-test list shows them and pp-cleanup
can archive them without touching pre-existing tickets.`,
		Example: `  linear-pp-cli issues batch-create --file /tmp/issues.jsonl --team ENG --agent
  cat issues.json | linear-pp-cli issues batch-create --file - --agent
  linear-pp-cli issues batch-create --file /tmp/issues.jsonl --team ENG --dry-run --agent`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(file) == "" {
				return usageErr(fmt.Errorf("--file is required: a path to JSONL or JSON, or - for stdin"))
			}
			// trust-mode strict requires a session tag so every fixture this
			// call creates is recoverable by pp-cleanup, matching what
			// 'issues create' demands.
			if flags.trustMode == "strict" && resolvePPSession(flags, session) == "" {
				return usageErr(fmt.Errorf("trust-mode=strict: pass --session <tag> (or set PP_SESSION env) so these fixtures are recoverable by pp-cleanup"))
			}

			records, err := readIssueBatchRecords(cmd, file)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				return usageErr(fmt.Errorf("%s contained no issue records", file))
			}
			if len(records) > issueBatchMax {
				return usageErr(fmt.Errorf("issueBatchCreate accepts at most %d issues per call, got %d. Split the file, because splitting it here would silently turn one transaction into several", issueBatchMax, len(records)))
			}

			dbPath := inheritedDBPath(cmd)

			// The dry run stays offline: aliases are echoed back unresolved
			// so the preview shows exactly what was asked for, with the
			// resolution work named rather than faked.
			if flags.dryRun {
				preview := make([]map[string]any, 0, len(records))
				pending := map[string]any{}
				for index, record := range records {
					input, unresolved, err := buildBatchCreateInput(nil, dbPath, index, record, teamFlag, projectFlag, labelsFlag)
					if err != nil {
						return err
					}
					preview = append(preview, input)
					for key, value := range unresolved {
						pending[fmt.Sprintf("%d.%s", index, key)] = value
					}
				}
				fields := map[string]any{
					"input": map[string]any{"issues": preview},
					"count": len(preview),
				}
				if len(pending) > 0 {
					fields["pending_resolution"] = pending
				}
				return renderMutationDryRun(cmd, flags, "would_batch_create_issues", "issueBatchCreate", fields)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			inputs := make([]map[string]any, 0, len(records))
			for index, record := range records {
				input, _, err := buildBatchCreateInput(c, dbPath, index, record, teamFlag, projectFlag, labelsFlag)
				if err != nil {
					return err
				}
				inputs = append(inputs, input)
			}

			resp, err := c.Mutate(client.IssueBatchCreateMutation, map[string]any{
				"input": map[string]any{"issues": inputs},
			})
			if err != nil {
				return classifyGraphQLCreateError("issueBatchCreate", err, flags)
			}
			issues, err := decodeIssueBatchPayload(resp, "issueBatchCreate")
			if err != nil {
				return err
			}

			sess := resolvePPSession(flags, session)
			if sess == "" || sess == "current" {
				sess = ppCurrentSession()
			}
			recordBatchCreatedIssues(dbPath, issues, sess)

			out, err := json.Marshal(map[string]any{
				"issues":  issues,
				"created": len(issues),
				"session": sess,
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "issues", true)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSONL, JSON array, or single JSON object of IssueCreateInput records. - reads stdin (required)")
	cmd.Flags().StringVar(&teamFlag, "team", "", "Default team key, name, or UUID for records without teamId or team")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Default project UUID for records without projectId")
	cmd.Flags().StringSliceVar(&labelsFlag, "label", nil, "Default label UUIDs for records without labelIds (repeatable)")
	cmd.Flags().StringVar(&session, "session", "", "Session tag for the pp_created ledger (defaults to PP_SESSION env or current run timestamp)")
	return cmd
}

// readIssueBatchRecords loads a JSON array, a single JSON object, or JSONL.
// The three are told apart by trying them in that order rather than by
// extension, so a .json file holding JSONL still works.
func readIssueBatchRecords(cmd *cobra.Command, path string) ([]map[string]any, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(cmd.InOrStdin())
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, usageErr(fmt.Errorf("reading %s: %w", path, err))
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, usageErr(fmt.Errorf("%s is empty", path))
	}

	var asArray []map[string]any
	if json.Unmarshal(raw, &asArray) == nil {
		return asArray, nil
	}
	var asObject map[string]any
	if json.Unmarshal(raw, &asObject) == nil {
		return []map[string]any{asObject}, nil
	}

	records := make([]map[string]any, 0, 16)
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(trimmed), &record); err != nil {
			return nil, usageErr(fmt.Errorf("%s line %d is not a JSON object: %w", path, lineNumber+1, err))
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, usageErr(fmt.Errorf("%s parsed as neither a JSON array, a JSON object, nor JSONL", path))
	}
	return records, nil
}

// buildBatchCreateInput validates one record and turns it into an
// IssueCreateInput. c may be nil on the --dry-run path, in which case the
// aliases that need a lookup are left in place and returned in the second
// value so the preview can name what it did not resolve.
func buildBatchCreateInput(c graphqlQueryer, dbPath string, index int, record map[string]any, teamDefault, projectDefault string, labelDefaults []string) (map[string]any, map[string]any, error) {
	if len(record) == 0 {
		return nil, nil, usageErr(fmt.Errorf("record %d is empty. Every record must be a JSON object carrying at least a title and a team", index))
	}
	input := map[string]any{}
	unresolved := map[string]any{}
	var teamRef, stateName, stateType string

	for _, key := range sortedKeys(record) {
		value := record[key]
		switch key {
		case batchAliasTeam:
			text, ok := value.(string)
			if !ok {
				return nil, nil, usageErr(fmt.Errorf("record %d: %q must be a string (team key, team name, or UUID)", index, batchAliasTeam))
			}
			teamRef = strings.TrimSpace(text)
		case batchAliasStateName:
			text, ok := value.(string)
			if !ok {
				return nil, nil, usageErr(fmt.Errorf("record %d: %q must be a string", index, batchAliasStateName))
			}
			stateName = strings.TrimSpace(text)
		case batchAliasStateType:
			text, ok := value.(string)
			if !ok {
				return nil, nil, usageErr(fmt.Errorf("record %d: %q must be a string", index, batchAliasStateType))
			}
			normalized, err := normalizeWorkflowStateType(text)
			if err != nil {
				return nil, nil, fmt.Errorf("record %d: %w", index, err)
			}
			stateType = normalized
		default:
			if !issueCreateInputFields[key] {
				return nil, nil, usageErr(fmt.Errorf("record %d: %q is not an IssueCreateInput field and not one of the %s, %s, %s aliases. Run 'linear-pp-cli issues batch-create --help' for the accepted keys", index, key, batchAliasTeam, batchAliasStateName, batchAliasStateType))
			}
			input[key] = value
		}
	}

	if projectDefault != "" {
		if _, ok := input["projectId"]; !ok {
			input["projectId"] = projectDefault
		}
	}
	if len(labelDefaults) > 0 {
		if _, ok := input["labelIds"]; !ok {
			input["labelIds"] = labelDefaults
		}
	}
	if teamRef == "" && teamDefault != "" {
		if _, ok := input["teamId"]; !ok {
			teamRef = teamDefault
		}
	}

	if teamRef != "" {
		if _, ok := input["teamId"]; ok {
			return nil, nil, usageErr(fmt.Errorf("record %d: pass either teamId or the %s alias, not both", index, batchAliasTeam))
		}
		resolved, err := resolveWriteTeamID(c, dbPath, teamRef)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case store.IsUUID(resolved):
			input["teamId"] = resolved
		case c == nil:
			// --dry-run stays offline, so an unresolvable key is reported
			// as pending rather than guessed at.
			unresolved["teamId"] = teamRef
		default:
			return nil, nil, notFoundErr(fmt.Errorf("record %d: team %q did not resolve to a team UUID", index, teamRef))
		}
	}
	if _, ok := input["teamId"]; !ok && len(unresolved) == 0 {
		return nil, nil, usageErr(fmt.Errorf("record %d: IssueCreateInput.teamId is required. Set teamId, set the %s alias, or pass --team", index, batchAliasTeam))
	}

	if stateName != "" && stateType != "" {
		return nil, nil, usageErr(fmt.Errorf("record %d: pass either %s or %s, not both", index, batchAliasStateName, batchAliasStateType))
	}
	if stateName != "" || stateType != "" {
		if _, ok := input["stateId"]; ok {
			return nil, nil, usageErr(fmt.Errorf("record %d: pass either stateId or a state alias, not both", index))
		}
		teamID, _ := input["teamId"].(string)
		if c == nil || teamID == "" {
			unresolved["stateId"] = firstNonEmpty(stateName, stateType)
		} else {
			stateID, err := resolveWorkflowState(c, issueTeamInfo{ID: teamID}, stateName, stateType)
			if err != nil {
				return nil, nil, fmt.Errorf("record %d: %w", index, err)
			}
			input["stateId"] = stateID
		}
	}
	return input, unresolved, nil
}

// sortedKeys keeps record iteration deterministic so an error names the same
// offending key on every run and a dry-run preview is byte-stable.
func sortedKeys(record map[string]any) []string {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// decodeIssueBatchPayload unwraps IssueBatchPayload. success=false is an API
// error, matching how every other mutation in this CLI treats it.
func decodeIssueBatchPayload(resp json.RawMessage, mutationKey string) ([]batchIssue, error) {
	var root map[string]struct {
		Success bool         `json:"success"`
		Issues  []batchIssue `json:"issues"`
	}
	if err := json.Unmarshal(resp, &root); err != nil {
		return nil, fmt.Errorf("parsing %s response: %w", mutationKey, err)
	}
	payload, ok := root[mutationKey]
	if !ok {
		return nil, fmt.Errorf("%s response missing %q", mutationKey, mutationKey)
	}
	if !payload.Success {
		return nil, apiErr(fmt.Errorf("Linear reported %s success=false", mutationKey))
	}
	return payload.Issues, nil
}

// recordBatchCreatedIssues writes every created issue into the pp_created
// ledger and mirrors it into the local issues table, which is exactly what
// 'issues create' does for its single issue. Without this a batch create
// would be invisible to pp-test list and unreachable by pp-cleanup, and
// --trust-mode strict would refuse to touch issues this CLI just made.
//
// Ledger failures are warnings on stderr, not command failures: the issues
// exist upstream either way and reporting the create as failed would be a
// lie.
func recordBatchCreatedIssues(dbPath string, issues []batchIssue, session string) {
	if len(issues) == 0 {
		return
	}
	resolved := resolveDBPath(dbPath)
	db, err := store.Open(resolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open ledger at %s: %v\n", resolved, err)
		return
	}
	defer db.Close()
	for _, issue := range issues {
		if issue.ID == "" {
			continue
		}
		if err := db.RecordPPFixture(issue.ID, issue.Identifier, issue.Title, session); err != nil {
			fmt.Fprintf(os.Stderr, "warning: pp_created ledger write failed for %s: %v\n", firstNonEmpty(issue.Identifier, issue.ID), err)
		}
		payload, err := json.Marshal(batchIssueWriteBack(issue))
		if err != nil {
			continue
		}
		if err := db.UpsertIssue(issue.ID, issue.Identifier, issue.Title, payload); err != nil {
			fmt.Fprintf(os.Stderr, "warning: local store write-back failed for %s: %v\n", firstNonEmpty(issue.Identifier, issue.ID), err)
		}
	}
}

// batchIssueWriteBack shapes one created issue the way sync writes the
// issues.data column, so 'issues list' sees a batch-created ticket without
// waiting for the next sync.
func batchIssueWriteBack(issue batchIssue) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	wb := map[string]any{
		"id":          issue.ID,
		"identifier":  issue.Identifier,
		"title":       issue.Title,
		"description": issue.Description,
		"url":         issue.URL,
		"priority":    issue.Priority,
		"team": map[string]any{
			"id":  issue.Team.ID,
			"key": issue.Team.Key,
		},
		"teamId": issue.Team.ID,
		"state": map[string]any{
			"id":   issue.State.ID,
			"name": issue.State.Name,
			"type": issue.State.Type,
		},
		"createdAt": firstNonEmpty(issue.CreatedAt, now),
		"updatedAt": firstNonEmpty(issue.UpdatedAt, now),
	}
	if issue.Assignee != nil {
		wb["assignee"] = map[string]any{
			"id":          issue.Assignee.ID,
			"name":        issue.Assignee.Name,
			"displayName": issue.Assignee.DisplayName,
		}
		wb["assigneeId"] = issue.Assignee.ID
	}
	if issue.Project != nil {
		wb["project"] = map[string]any{
			"id":   issue.Project.ID,
			"name": issue.Project.Name,
		}
		wb["projectId"] = issue.Project.ID
	}
	if issue.Parent != nil {
		wb["parent"] = map[string]any{
			"id":         issue.Parent.ID,
			"identifier": issue.Parent.Identifier,
			"title":      issue.Parent.Title,
		}
		wb["parentId"] = issue.Parent.ID
	}
	return wb
}

// ---------------------------------------------------------------------------
// issues batch-update
// ---------------------------------------------------------------------------

func newIssuesBatchUpdateCmd(flags *rootFlags) *cobra.Command {
	var ids []string
	var setJSON string
	var stateFlag, stateNameFlag, stateTypeFlag string
	var assigneeFlag, cycleFlag, projectFlag, milestoneFlag, teamFlag, dueDateFlag string
	var addLabels, removeLabels, replaceLabels []string
	var priorityFlag, estimateFlag int
	cmd := &cobra.Command{
		Use:   "batch-update --ids <a,b,c>",
		Short: "Apply one change to up to 50 issues in a single transaction",
		Long: `Apply one IssueUpdateInput to many issues in a single issueBatchUpdate call.

--ids takes issue identifiers (ENG-123) or UUIDs, comma-separated or
repeated. Linear's mutation types the list as [UUID!]!, so identifiers are
resolved before the call. Duplicates collapse, order is kept.

The change itself comes from the flags, which mirror 'issues edit' where the
field exists on IssueUpdateInput, and from --set, which takes a raw JSON
IssueUpdateInput object. --set is the base and explicit flags override it,
so a stored template can be reused with one field varied. Keys in --set are
validated against IssueUpdateInput, and an unknown key is refused by name.

--state-name and --state-type resolve to a stateId, and a workflow state
UUID is per-team, so both require --team. --state takes an already-resolved
UUID and does not.

Linear caps the batch at 50. A larger list is refused rather than split.
Confirmation is required unless --yes, because one call moves up to 50
tickets. --trust-mode strict checks every resolved target against the local
pp_created ledger and refuses the whole batch if any one of them is not a
fixture this CLI created.`,
		Example: `  linear-pp-cli issues batch-update --ids ENG-1,ENG-2,ENG-3 --state-type completed --team ENG --yes --agent
  linear-pp-cli issues batch-update --ids ENG-1,ENG-2 --assignee <user-uuid> --priority 2 --yes --agent
  linear-pp-cli issues batch-update --ids ENG-1,ENG-2 --add-label <label-uuid> --yes --agent
  linear-pp-cli issues batch-update --ids ENG-1,ENG-2 --set '{"estimate":3}' --priority 1 --dry-run --agent`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, err := parseBatchIssueRefs(ids)
			if err != nil {
				return err
			}

			input := map[string]any{}
			if strings.TrimSpace(setJSON) != "" {
				parsed, err := parseIssueUpdateSet(setJSON)
				if err != nil {
					return err
				}
				input = parsed
			}

			if stateFlag != "" {
				if err := requireUUIDFlag("--state", "workflow state", stateFlag); err != nil {
					return err
				}
				input["stateId"] = stateFlag
			}
			if stateNameFlag != "" && stateTypeFlag != "" {
				return usageErr(fmt.Errorf("pass either --state-name or --state-type, not both"))
			}
			if (stateNameFlag != "" || stateTypeFlag != "") && stateFlag != "" {
				return usageErr(fmt.Errorf("pass either --state, --state-name, or --state-type, not several"))
			}
			if stateTypeFlag != "" {
				normalized, err := normalizeWorkflowStateType(stateTypeFlag)
				if err != nil {
					return err
				}
				stateTypeFlag = normalized
			}
			if (stateNameFlag != "" || stateTypeFlag != "") && teamFlag == "" {
				return usageErr(fmt.Errorf("--state-name and --state-type need --team: a workflow state UUID belongs to one team, and a batch can span teams. Pass --team <key> to name the team to resolve against, or pass --state <uuid> directly"))
			}
			if assigneeFlag != "" {
				if err := requireUUIDFlag("--assignee", "user", assigneeFlag); err != nil {
					return err
				}
				input["assigneeId"] = assigneeFlag
			}
			if cycleFlag != "" {
				if err := requireUUIDFlag("--cycle", "cycle", cycleFlag); err != nil {
					return err
				}
				input["cycleId"] = cycleFlag
			}
			if projectFlag != "" {
				if err := requireUUIDFlag("--project", "project", projectFlag); err != nil {
					return err
				}
				input["projectId"] = projectFlag
			}
			if milestoneFlag != "" {
				if err := requireUUIDFlag("--milestone", "project milestone", milestoneFlag); err != nil {
					return err
				}
				input["projectMilestoneId"] = milestoneFlag
			}
			if len(replaceLabels) > 0 && (len(addLabels) > 0 || len(removeLabels) > 0) {
				return usageErr(fmt.Errorf("--label replaces the whole label set, so it cannot be combined with --add-label or --remove-label"))
			}
			if len(replaceLabels) > 0 {
				input["labelIds"] = replaceLabels
			}
			if len(addLabels) > 0 {
				input["addedLabelIds"] = addLabels
			}
			if len(removeLabels) > 0 {
				input["removedLabelIds"] = removeLabels
			}
			if cmd.Flags().Changed("priority") {
				input["priority"] = priorityFlag
			}
			if cmd.Flags().Changed("estimate") {
				if estimateFlag < 0 {
					return usageErr(fmt.Errorf("--estimate expects a non-negative point value (got %d)", estimateFlag))
				}
				input["estimate"] = estimateFlag
			}
			if cmd.Flags().Changed("due-date") {
				due, err := parseTimelessDate("--due-date", dueDateFlag)
				if err != nil {
					return err
				}
				input["dueDate"] = due
			}

			if len(input) == 0 && stateNameFlag == "" && stateTypeFlag == "" {
				return usageErr(fmt.Errorf("nothing to update: pass at least one field flag or --set '<json>'"))
			}

			dbPath := inheritedDBPath(cmd)

			if flags.dryRun && (flags == nil || flags.trustMode != "strict") {
				// Offline when we are not proving fixture ownership.
				// Strict mode must resolve each identifier first so a
				// TEAM-NUMBER that the ledger still names cannot pass a dry
				// run after Linear has reused that identifier for a different
				// issue UUID.
				fields := map[string]any{
					"ids":   targets,
					"input": input,
					"count": len(targets),
				}
				if stateNameFlag != "" {
					fields["state_name"] = stateNameFlag
				}
				if stateTypeFlag != "" {
					fields["state_type"] = stateTypeFlag
				}
				if teamFlag != "" {
					fields["team"] = teamFlag
				}
				return renderMutationDryRun(cmd, flags, "would_batch_update_issues", "issueBatchUpdate", fields)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if stateNameFlag != "" || stateTypeFlag != "" {
				teamID, err := resolveWriteTeamID(c, dbPath, teamFlag)
				if err != nil {
					return err
				}
				stateID, err := resolveWorkflowState(c, issueTeamInfo{ID: teamID}, stateNameFlag, stateTypeFlag)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["stateId"] = stateID
			}
			if len(input) == 0 {
				return usageErr(fmt.Errorf("nothing to update: pass at least one field flag or --set '<json>'"))
			}

			// Resolve every identifier and run the trust gate over the whole
			// batch before mutating any of it, so strict mode cannot let a
			// partial batch through.
			uuids := make([]string, 0, len(targets))
			for _, target := range targets {
				issueID, err := resolveIssueTarget(c, flags, dbPath, target)
				if err != nil {
					return err
				}
				uuids = append(uuids, issueID)
			}

			if flags.dryRun {
				fields := map[string]any{
					"ids":   uuids,
					"input": input,
					"count": len(uuids),
				}
				if stateNameFlag != "" {
					fields["state_name"] = stateNameFlag
				}
				if stateTypeFlag != "" {
					fields["state_type"] = stateTypeFlag
				}
				if teamFlag != "" {
					fields["team"] = teamFlag
				}
				return renderMutationDryRun(cmd, flags, "would_batch_update_issues", "issueBatchUpdate", fields)
			}

			if err := confirmMutation(cmd, flags, fmt.Sprintf("Apply this change to %d issues in one transaction?", len(uuids))); err != nil {
				return err
			}

			resp, err := c.Mutate(client.IssueBatchUpdateMutation, map[string]any{
				"input": input,
				"ids":   uuids,
			})
			if err != nil {
				return classifyGraphQLMutationError("issueBatchUpdate", err, flags)
			}
			issues, err := decodeIssueBatchPayload(resp, "issueBatchUpdate")
			if err != nil {
				return err
			}
			out, err := json.Marshal(map[string]any{
				"issues":  issues,
				"updated": len(issues),
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "issues", true)
		},
	}
	cmd.Flags().StringSliceVar(&ids, "ids", nil, "Issue identifiers or UUIDs, comma-separated or repeated (required, max 50)")
	cmd.Flags().StringVar(&setJSON, "set", "", "Raw JSON IssueUpdateInput used as the base. Explicit flags override it")
	cmd.Flags().StringVar(&stateFlag, "state", "", "Workflow state UUID")
	cmd.Flags().StringVar(&stateNameFlag, "state-name", "", "Workflow state name, resolved against --team")
	cmd.Flags().StringVar(&stateTypeFlag, "state-type", "", "Workflow state type (triage, backlog, unstarted, started, completed, canceled, duplicate), resolved against --team")
	cmd.Flags().StringVar(&teamFlag, "team", "", "Team key, name, or UUID that --state-name and --state-type resolve against")
	cmd.Flags().StringVar(&assigneeFlag, "assignee", "", "Assignee user UUID")
	cmd.Flags().StringVar(&cycleFlag, "cycle", "", "Cycle UUID")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project UUID")
	cmd.Flags().StringVar(&milestoneFlag, "milestone", "", "Project milestone UUID")
	cmd.Flags().StringVar(&dueDateFlag, "due-date", "", "Due date as YYYY-MM-DD (TimelessDate). A timestamp is rejected")
	cmd.Flags().StringSliceVar(&addLabels, "add-label", nil, "Label UUIDs to add (repeatable, addedLabelIds)")
	cmd.Flags().StringSliceVar(&removeLabels, "remove-label", nil, "Label UUIDs to remove (repeatable, removedLabelIds)")
	cmd.Flags().StringSliceVar(&replaceLabels, "label", nil, "Label UUIDs replacing the whole set (repeatable, labelIds)")
	cmd.Flags().IntVar(&priorityFlag, "priority", 0, "Priority: 1=Urgent, 2=High, 3=Medium, 4=Low (0=None). Only sent when the flag is passed, so --priority 0 is an explicit \"No priority\"")
	cmd.Flags().IntVar(&estimateFlag, "estimate", 0, "Estimate points. Only sent when the flag is passed, so --estimate 0 is an explicit zero")
	return cmd
}

// parseBatchIssueRefs normalises --ids into an ordered, deduplicated target
// list and enforces Linear's ceiling before anything reaches the network.
func parseBatchIssueRefs(ids []string) ([]string, error) {
	targets := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		for _, part := range strings.Split(raw, ",") {
			ref := strings.TrimSpace(part)
			if ref == "" {
				continue
			}
			key := ref
			if !store.IsUUID(ref) {
				key = strings.ToUpper(ref)
				ref = key
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			targets = append(targets, ref)
		}
	}
	if len(targets) == 0 {
		return nil, usageErr(fmt.Errorf("--ids is required: pass issue identifiers or UUIDs, comma-separated or repeated"))
	}
	if len(targets) > issueBatchMax {
		return nil, usageErr(fmt.Errorf("issueBatchUpdate accepts at most %d ids per call, got %d. Split the list, because splitting it here would silently turn one transaction into several", issueBatchMax, len(targets)))
	}
	return targets, nil
}

// parseIssueUpdateSet validates --set against IssueUpdateInput so a typo is
// named here instead of being accepted and ignored upstream.
func parseIssueUpdateSet(raw string) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, usageErr(fmt.Errorf("--set expects a JSON object of IssueUpdateInput fields: %w", err))
	}
	for _, key := range sortedKeys(parsed) {
		if !issueUpdateInputFields[key] {
			return nil, usageErr(fmt.Errorf("--set: %q is not an IssueUpdateInput field", key))
		}
	}
	return parsed, nil
}
