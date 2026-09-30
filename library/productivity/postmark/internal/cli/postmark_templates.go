// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

// ---------------------------------------------------------------------------
// Template API calls.
// ---------------------------------------------------------------------------

const (
	templateListPageSize = 100
	templateListMaxPages = 200
)

type templateSummary struct {
	TemplateId     int64  `json:"TemplateId"`
	Name           string `json:"Name"`
	Alias          string `json:"Alias"`
	TemplateType   string `json:"TemplateType"`
	LayoutTemplate string `json:"LayoutTemplate"`
	Active         bool   `json:"Active"`
}

type templateDetail struct {
	TemplateId     int64  `json:"TemplateId"`
	Name           string `json:"Name"`
	Alias          string `json:"Alias"`
	Subject        string `json:"Subject"`
	HtmlBody       string `json:"HtmlBody"`
	TextBody       string `json:"TextBody"`
	TemplateType   string `json:"TemplateType"`
	LayoutTemplate string `json:"LayoutTemplate"`
	Active         bool   `json:"Active"`
}

func (d templateDetail) content() templateContent {
	ttype := normalizeTemplateType(d.TemplateType)
	if ttype == "" {
		ttype = templateTypeStandard
	}
	return templateContent{
		Name:           d.Name,
		Alias:          d.Alias,
		Subject:        d.Subject,
		HtmlBody:       d.HtmlBody,
		TextBody:       d.TextBody,
		TemplateType:   ttype,
		LayoutTemplate: d.LayoutTemplate,
	}
}

type templateListPage struct {
	TotalCount int               `json:"TotalCount"`
	Templates  []templateSummary `json:"Templates"`
}

// listPostmarkTemplates pages through GET /templates until TotalCount is
// reached (no 100 or 300 cap). maxPages bounds the walk; the returned total
// lets callers detect a curtailed listing.
func listPostmarkTemplates(ctx context.Context, c *client.Client, templateType string, maxPages int) ([]templateSummary, int, error) {
	if maxPages <= 0 || maxPages > templateListMaxPages {
		maxPages = templateListMaxPages
	}
	all := make([]templateSummary, 0)
	total := 0
	paging := postmarkPaging{pageSize: templateListPageSize, maxPages: maxPages, untilEmpty: true}
	_, err := walkPostmarkPagesWith(paging, func(offset int) ([]templateSummary, int, error) {
		params := map[string]string{
			"Count":  strconv.Itoa(templateListPageSize),
			"Offset": strconv.Itoa(offset),
		}
		if templateType != "" {
			params["TemplateType"] = templateType
		}
		raw, err := c.Get(ctx, "/templates", params)
		if err != nil {
			return nil, 0, err
		}
		var p templateListPage
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, 0, fmt.Errorf("parsing template list: %w", err)
		}
		total = p.TotalCount
		return p.Templates, p.TotalCount, nil
	}, func(t templateSummary) bool {
		all = append(all, t)
		return false
	})
	if err != nil {
		return nil, 0, err
	}
	return all, total, nil
}

func getPostmarkTemplate(ctx context.Context, c *client.Client, idOrAlias string) (templateDetail, error) {
	raw, err := c.Get(ctx, "/templates/"+url.PathEscape(idOrAlias), nil)
	if err != nil {
		return templateDetail{}, err
	}
	var d templateDetail
	if err := json.Unmarshal(raw, &d); err != nil {
		return templateDetail{}, fmt.Errorf("parsing template %s: %w", idOrAlias, err)
	}
	return d, nil
}

type templateValidateRequest struct {
	Subject         string          `json:"Subject,omitempty"`
	HtmlBody        string          `json:"HtmlBody,omitempty"`
	TextBody        string          `json:"TextBody,omitempty"`
	TestRenderModel json.RawMessage `json:"TestRenderModel,omitempty"`
	TemplateType    string          `json:"TemplateType,omitempty"`
	LayoutTemplate  string          `json:"LayoutTemplate,omitempty"`
}

type templateValidationError struct {
	Message           string `json:"Message"`
	Line              *int   `json:"Line"`
	CharacterPosition *int   `json:"CharacterPosition"`
}

type templateValidatePart struct {
	ContentIsValid   bool                      `json:"ContentIsValid"`
	ValidationErrors []templateValidationError `json:"ValidationErrors"`
	RenderedContent  *string                   `json:"RenderedContent"`
}

type templateValidateResponse struct {
	AllContentIsValid      bool                  `json:"AllContentIsValid"`
	HtmlBody               *templateValidatePart `json:"HtmlBody"`
	TextBody               *templateValidatePart `json:"TextBody"`
	Subject                *templateValidatePart `json:"Subject"`
	SuggestedTemplateModel json.RawMessage       `json:"SuggestedTemplateModel"`
}

type templateRenderIssue struct {
	Part     string `json:"part"`
	Message  string `json:"message"`
	Line     *int   `json:"line,omitempty"`
	Position *int   `json:"position,omitempty"`
}

func (r templateValidateResponse) issues() []templateRenderIssue {
	out := make([]templateRenderIssue, 0)
	for _, p := range []struct {
		name string
		part *templateValidatePart
	}{{"Subject", r.Subject}, {"HtmlBody", r.HtmlBody}, {"TextBody", r.TextBody}} {
		if p.part == nil {
			continue
		}
		for _, e := range p.part.ValidationErrors {
			out = append(out, templateRenderIssue{Part: p.name, Message: e.Message, Line: e.Line, Position: e.CharacterPosition})
		}
		if !p.part.ContentIsValid && len(p.part.ValidationErrors) == 0 {
			out = append(out, templateRenderIssue{Part: p.name, Message: "content is not valid"})
		}
	}
	return out
}

// validateRequestFor builds the /templates/validate body for one template.
// Layouts carry no Subject or LayoutTemplate.
func validateRequestFor(c templateContent, model json.RawMessage) templateValidateRequest {
	req := templateValidateRequest{
		HtmlBody:     c.HtmlBody,
		TextBody:     c.TextBody,
		TemplateType: c.TemplateType,
	}
	if c.TemplateType != templateTypeLayout {
		req.Subject = c.Subject
		req.LayoutTemplate = c.LayoutTemplate
	}
	if hasTemplateModel(model) {
		req.TestRenderModel = model
	}
	return req
}

func validatePostmarkTemplate(ctx context.Context, c *client.Client, req templateValidateRequest) (templateValidateResponse, error) {
	raw, _, err := c.PostQueryWithParams(ctx, "/templates/validate", nil, req)
	if err != nil {
		return templateValidateResponse{}, err
	}
	var resp templateValidateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return templateValidateResponse{}, fmt.Errorf("parsing validate response: %w", err)
	}
	return resp, nil
}

// isMissingLayoutFailure reports Postmark's "LayoutTemplate ... was not found"
// rejection.
func isMissingLayoutFailure(err error) bool {
	f, ok := postmarkAPIFailure(err)
	return ok && f.ErrorCode == postmarkErrTemplateNotFound && strings.Contains(f.Message, "LayoutTemplate")
}

// validateTemplateWithLayoutFallback validates content against its layout and,
// when the server does not have that layout yet, validates the content alone
// so render errors still surface. The bool reports the fallback.
func validateTemplateWithLayoutFallback(ctx context.Context, c *client.Client, req templateValidateRequest) (templateValidateResponse, bool, error) {
	resp, err := validatePostmarkTemplate(ctx, c, req)
	if err == nil || req.LayoutTemplate == "" || !isMissingLayoutFailure(err) {
		return resp, false, err
	}
	req.LayoutTemplate = ""
	resp, err = validatePostmarkTemplate(ctx, c, req)
	return resp, true, err
}

// ---------------------------------------------------------------------------
// templates pull
// ---------------------------------------------------------------------------

const (
	pullStatusCreated   = "created"
	pullStatusUpdated   = "updated"
	pullStatusUnchanged = "unchanged"

	pullModelSuggested = "suggested"
	pullModelKept      = "kept"
	pullModelNone      = "none"

	// Under the live-dogfood harness a pull or check reads at most this many
	// template bodies so it fits the per-command timeout.
	dogfoodTemplateCap = 10
)

// dogfoodTemplateLimit returns how many template bodies a command may read,
// 0 meaning no limit outside the live-dogfood harness, and the note that
// reports the cut. verb is the past tense of the command's action.
func dogfoodTemplateLimit(verb string) (limit int, note string) {
	if !cliutil.IsDogfoodEnv() {
		return 0, ""
	}
	return dogfoodTemplateCap, fmt.Sprintf("live-dogfood harness: %s the first %d templates only", verb, dogfoodTemplateCap)
}

type templatePullItem struct {
	Alias        string `json:"alias"`
	Name         string `json:"name"`
	TemplateType string `json:"template_type"`
	TemplateID   int64  `json:"template_id,omitempty"`
	Path         string `json:"path"`
	Status       string `json:"status"`
	TestModel    string `json:"test_model"`
	Warning      string `json:"warning,omitempty"`
}

type templatePullSkip struct {
	Alias      string `json:"alias,omitempty"`
	Name       string `json:"name,omitempty"`
	TemplateID int64  `json:"template_id,omitempty"`
	Reason     string `json:"reason"`
}

type templatePullView struct {
	Dir           string                 `json:"dir"`
	TemplateType  string                 `json:"template_type"`
	ServerTotal   int                    `json:"server_total"`
	Listed        int                    `json:"listed"`
	Pulled        []templatePullItem     `json:"pulled"`
	Skipped       []templatePullSkip     `json:"skipped"`
	FetchFailures []postmarkFetchFailure `json:"fetch_failures,omitempty"`
	HarnessNoop   bool                   `json:"harness_noop,omitempty"`
	Note          string                 `json:"note,omitempty"`
}

// parseTemplateTypeFilter maps --type to the GET /templates TemplateType value.
func parseTemplateTypeFilter(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "all":
		return "All", nil
	case "standard":
		return templateTypeStandard, nil
	case "layout", "layouts":
		return templateTypeLayout, nil
	}
	return "", fmt.Errorf("invalid --type %q: use all, standard, or layout", v)
}

func parseAliasList(v string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

// pullDecision decides what a pull does with one template given the local
// copy (nil when absent). It returns the test model to write (nil means ask
// the server for a suggested one) or a skip reason.
func pullDecision(remote templateContent, local *localTemplate, localErr string, overwrite bool) (keepModel json.RawMessage, skipReason string) {
	if localErr != "" {
		if !overwrite {
			return nil, "existing meta.json could not be read (" + localErr + "); pass --overwrite to replace it"
		}
		return nil, ""
	}
	if local == nil {
		return nil, ""
	}
	diff := templateContentDiff(remote, local.content())
	if local.Meta.TemplateType != remote.TemplateType {
		diff = append(diff, "template_type")
	}
	if len(diff) > 0 && !overwrite {
		return nil, fmt.Sprintf("local copy differs from the server (%s); pass --overwrite to replace it", strings.Join(diff, ", "))
	}
	if !overwrite && local.hasTestModel() {
		return local.Meta.TestRenderModel, ""
	}
	return nil, ""
}

func newTemplatesPullCmd(flags *rootFlags) *cobra.Command {
	var aliasFlag, typeFlag string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "pull <dir>",
		Short: "Pull templates and layouts into a directory in the official postmark-cli layout",
		Long: strings.Trim(`
Pull every template on the server into <dir> using the same layout as the
official postmark-cli, so either tool can read the result:

  <dir>/<alias>/content.html, content.txt, meta.json
  <dir>/_layouts/<alias>/content.html, content.txt, meta.json

meta.json carries Name, Alias, Subject, TemplateType, LayoutTemplate, and a
TestRenderModel filled from Postmark's suggested model. Listing pages through
every template (no 100 or 300 cap). Templates without an alias cannot be
written and are reported under "skipped" instead of being dropped silently.

An existing local copy is never clobbered: when it differs from the server the
template is skipped until you pass --overwrite, and an existing TestRenderModel
is kept unless --overwrite is set.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli templates pull ./templates --server "Main App"
  postmark-pp-cli templates pull ./templates --alias password-reset,welcome --server "Main App" --json
  postmark-pp-cli templates pull ./templates --type layout --overwrite --server "Main App"`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			"pp:happy-args":  "dir=pp-dogfood-templates",
			// <dir> is a write destination chosen by the caller; MCP tools
			// cannot restrict positionals on write commands, so pull stays CLI-only.
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "templates pull")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("<dir> is required: the directory to pull templates into"))
			}
			typeParam, err := parseTemplateTypeFilter(typeFlag)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			root := args[0]
			wanted := parseAliasList(aliasFlag)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newUncachedClient(flags)
			if err != nil {
				return err
			}
			maxPages := 0
			if cliutil.IsDogfoodEnv() {
				maxPages = 1
			}
			summaries, total, err := listPostmarkTemplates(ctx, c, typeParam, maxPages)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			view := templatePullView{
				Dir:          root,
				TemplateType: typeParam,
				ServerTotal:  total,
				Listed:       len(summaries),
				Pulled:       make([]templatePullItem, 0),
				Skipped:      make([]templatePullSkip, 0),
				HarnessNoop:  cliutil.IsAnyHarness(),
			}
			wantSet := map[string]bool{}
			for _, a := range wanted {
				wantSet[a] = true
			}
			found := map[string]bool{}
			fetched := 0
			limit, limitNote := dogfoodTemplateLimit("pulled")
			for _, s := range summaries {
				if len(wantSet) > 0 {
					if !wantSet[s.Alias] {
						continue
					}
					found[s.Alias] = true
				}
				ttype := normalizeTemplateType(s.TemplateType)
				if s.Alias == "" {
					view.Skipped = append(view.Skipped, templatePullSkip{Name: s.Name, TemplateID: s.TemplateId,
						Reason: "missing alias: a directory needs an alias (the official postmark-cli drops these silently); set an alias in Postmark to pull it"})
					continue
				}
				if !safeTemplateAlias(s.Alias, ttype) {
					view.Skipped = append(view.Skipped, templatePullSkip{Alias: s.Alias, Name: s.Name, TemplateID: s.TemplateId,
						Reason: "alias is not usable as a directory name"})
					continue
				}
				if limit > 0 && fetched >= limit {
					view.Note = limitNote
					break
				}
				fetched++
				detail, err := getPostmarkTemplate(ctx, c, s.Alias)
				if err != nil {
					if isRateLimited(err) {
						return rateLimitErr(err)
					}
					view.FetchFailures = append(view.FetchFailures, itemFailure(s.Alias, err))
					continue
				}
				item, skip, err := pullOneTemplate(ctx, c, root, detail, overwrite, view.HarnessNoop)
				if err != nil {
					if isRateLimited(err) {
						return rateLimitErr(err)
					}
					return fmt.Errorf("writing template %s: %w", s.Alias, err)
				}
				if skip != nil {
					view.Skipped = append(view.Skipped, *skip)
					continue
				}
				view.Pulled = append(view.Pulled, item)
			}
			for _, a := range wanted {
				if !found[a] {
					view.Skipped = append(view.Skipped, templatePullSkip{Alias: a, Reason: "no template with this alias on the server"})
				}
			}
			if view.HarnessNoop {
				view.Note = strings.TrimSpace(view.Note + " harness run: files were not written")
			}
			if total > len(summaries) && !cliutil.IsDogfoodEnv() {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: server reported %d templates but listing returned %d\n", total, len(summaries))
			}
			warnPartialFailures(cmd.ErrOrStderr(), len(view.FetchFailures), fetched, "template bodies could not be fetched", "they were not pulled")
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "warning", "reason"); err != nil {
					return err
				}
			} else {
				printTemplatePullHuman(cmd.OutOrStdout(), view)
			}
			if len(view.FetchFailures) > 0 {
				return apiErr(fmt.Errorf("%d template(s) could not be fetched", len(view.FetchFailures)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&aliasFlag, "alias", "", "Comma-separated template aliases to pull (default: every template)")
	cmd.Flags().StringVar(&typeFlag, "type", "all", "Template type to pull: all, standard, or layout")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace local copies that differ from the server and regenerate TestRenderModel")
	return cmd
}

// pullOneTemplate writes one template (unless noWrite) and returns the row
// or a skip reason.
func pullOneTemplate(ctx context.Context, c *client.Client, root string, detail templateDetail, overwrite, noWrite bool) (templatePullItem, *templatePullSkip, error) {
	remote := detail.content()
	if !safeTemplateAlias(remote.Alias, remote.TemplateType) {
		return templatePullItem{}, &templatePullSkip{Alias: remote.Alias, Reason: "alias is not safe to use as a folder name"}, nil
	}
	dir := templateLocalDir(root, remote.Alias, remote.TemplateType)
	item := templatePullItem{
		Alias:        remote.Alias,
		Name:         remote.Name,
		TemplateType: remote.TemplateType,
		TemplateID:   detail.TemplateId,
		Path:         dir,
	}
	metaPath := filepath.Join(dir, templateMetaFileName)
	var local *localTemplate
	localErr := ""
	localExists := false
	if _, statErr := os.Stat(metaPath); statErr == nil {
		localExists = true
		tpl, problem := loadLocalTemplate(root, metaPath)
		if problem != nil {
			localErr = problem.Reason
		} else {
			local = &tpl
		}
	}
	keepModel, skipReason := pullDecision(remote, local, localErr, overwrite)
	if skipReason != "" {
		return item, &templatePullSkip{Alias: remote.Alias, Name: remote.Name, TemplateID: detail.TemplateId, Reason: skipReason}, nil
	}
	testModel := keepModel
	item.TestModel = pullModelKept
	if testModel == nil {
		item.TestModel = pullModelNone
		resp, err := validatePostmarkTemplate(ctx, c, validateRequestFor(remote, nil))
		switch {
		case err != nil && isRateLimited(err):
			return item, nil, err
		case err != nil:
			item.Warning = "could not fetch a suggested test model: " + postmarkErrorText(err)
		case hasTemplateModel(resp.SuggestedTemplateModel):
			testModel = resp.SuggestedTemplateModel
			item.TestModel = pullModelSuggested
		}
	}
	if noWrite {
		item.Status = pullStatusCreated
		if localExists {
			item.Status = pullStatusUnchanged
		}
		return item, nil, nil
	}
	res, err := writeTemplateFiles(root, remote, testModel)
	if err != nil {
		return item, nil, err
	}
	switch {
	case !localExists:
		item.Status = pullStatusCreated
	case res.Changed:
		item.Status = pullStatusUpdated
	default:
		item.Status = pullStatusUnchanged
	}
	return item, nil, nil
}

func printTemplatePullHuman(w io.Writer, v templatePullView) {
	counts := map[string]int{}
	for _, p := range v.Pulled {
		counts[p.Status]++
	}
	fmt.Fprintf(w, "Pulled %d template(s) into %s (created %d, updated %d, unchanged %d)\n",
		len(v.Pulled), v.Dir, counts[pullStatusCreated], counts[pullStatusUpdated], counts[pullStatusUnchanged])
	if len(v.Pulled) > 0 {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "STATUS\tALIAS\tTYPE\tTEST MODEL\tPATH")
		for _, p := range v.Pulled {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Status, p.Alias, p.TemplateType, p.TestModel, p.Path)
		}
		_ = tw.Flush()
	}
	for _, p := range v.Pulled {
		if p.Warning != "" {
			fmt.Fprintf(w, "warning: %s: %s\n", p.Alias, p.Warning)
		}
	}
	if len(v.Skipped) > 0 {
		fmt.Fprintf(w, "\nSkipped %d:\n", len(v.Skipped))
		for _, s := range v.Skipped {
			label := s.Alias
			if label == "" {
				label = fmt.Sprintf("%q (id %d)", s.Name, s.TemplateID)
			}
			fmt.Fprintf(w, "  %s: %s\n", label, s.Reason)
		}
	}
	for _, f := range v.FetchFailures {
		fmt.Fprintf(w, "fetch failed: %s: %s\n", f.ID, f.Error)
	}
	if v.Note != "" {
		fmt.Fprintln(w, v.Note)
	}
}

// ---------------------------------------------------------------------------
// templates push
// ---------------------------------------------------------------------------

const (
	pushActionCreate    = "create"
	pushActionUpdate    = "update"
	pushActionUnchanged = "unchanged"
	pushActionDelete    = "delete"
	pushActionConflict  = "conflict"
	pushActionBlocked   = "blocked"
)

type templatePushItem struct {
	Alias          string     `json:"alias"`
	Name           string     `json:"name"`
	TemplateType   string     `json:"template_type"`
	Action         string     `json:"action"`
	Changes        []string   `json:"changes,omitempty"`
	LayoutTemplate string     `json:"layout_template,omitempty"`
	Reason         string     `json:"reason,omitempty"`
	Status         itemStatus `json:"status"`
	Error          string     `json:"error,omitempty"`
	Path           string     `json:"path,omitempty"`
	local          *localTemplate
}

type templateRemoteRef struct {
	Name       string `json:"name"`
	TemplateID int64  `json:"template_id"`
}

type templatePushView struct {
	Mode            runMode                `json:"mode"`
	Dir             string                 `json:"dir"`
	Prune           bool                   `json:"prune"`
	Summary         map[string]int         `json:"summary"`
	Writes          int                    `json:"writes"`
	Failed          int                    `json:"failed"`
	Items           []templatePushItem     `json:"items"`
	InvalidLocal    []localTemplateProblem `json:"invalid_local"`
	UnaliasedRemote []templateRemoteRef    `json:"unaliased_remote,omitempty"`
	Note            string                 `json:"note,omitempty"`
}

func isPushWrite(action string) bool {
	return action == pushActionCreate || action == pushActionUpdate || action == pushActionDelete
}

// validateLocalAliases splits local templates into pushable ones and problems
// (missing alias, unsafe alias, or an alias used by two folders).
func validateLocalAliases(local []localTemplate) ([]localTemplate, []localTemplateProblem) {
	byAlias := map[string][]localTemplate{}
	problems := make([]localTemplateProblem, 0)
	for _, t := range local {
		if strings.TrimSpace(t.Meta.Alias) == "" {
			problems = append(problems, localTemplateProblem{Path: t.Dir, Reason: "meta.json has no Alias; Postmark templates are matched by alias"})
			continue
		}
		byAlias[t.Meta.Alias] = append(byAlias[t.Meta.Alias], t)
	}
	valid := make([]localTemplate, 0, len(local))
	for alias, group := range byAlias {
		if len(group) > 1 {
			paths := make([]string, 0, len(group))
			for _, t := range group {
				paths = append(paths, t.Dir)
			}
			sort.Strings(paths)
			for _, t := range group {
				problems = append(problems, localTemplateProblem{Path: t.Dir, Alias: alias, Reason: "alias is used by more than one folder: " + strings.Join(paths, ", ")})
			}
			continue
		}
		valid = append(valid, group[0])
	}
	sortLocalTemplates(valid)
	sort.SliceStable(problems, func(i, j int) bool { return problems[i].Path < problems[j].Path })
	return valid, problems
}

// buildTemplatePushPlan compares local templates with the server copies by
// alias. Items come back in execution order: local layouts, local standard
// templates, then (with prune) remote-only standard templates and layouts.
func buildTemplatePushPlan(local []localTemplate, remote []templateDetail, prune bool) []templatePushItem {
	remoteByAlias := map[string]templateDetail{}
	for _, r := range remote {
		if r.Alias != "" {
			remoteByAlias[r.Alias] = r
		}
	}
	localAliases := map[string]bool{}
	layoutsAvailable := map[string]bool{}
	for _, t := range local {
		localAliases[t.Meta.Alias] = true
		if t.Meta.TemplateType == templateTypeLayout {
			layoutsAvailable[t.Meta.Alias] = true
		}
	}
	for _, r := range remote {
		if r.Alias != "" && normalizeTemplateType(r.TemplateType) == templateTypeLayout {
			layoutsAvailable[r.Alias] = true
		}
	}

	items := make([]templatePushItem, 0, len(local))
	for i := range local {
		t := local[i]
		lc := t.content()
		item := templatePushItem{
			Alias:          lc.Alias,
			Name:           lc.Name,
			TemplateType:   lc.TemplateType,
			LayoutTemplate: lc.LayoutTemplate,
			Path:           t.Dir,
			local:          &local[i],
		}
		r, onServer := remoteByAlias[lc.Alias]
		switch {
		case !onServer:
			item.Action = pushActionCreate
		case normalizeTemplateType(r.TemplateType) != lc.TemplateType:
			item.Action = pushActionConflict
			item.Reason = fmt.Sprintf("the server has %q as a %s template; TemplateType cannot change after create", lc.Alias, normalizeTemplateType(r.TemplateType))
		default:
			item.Changes = templateContentDiff(r.content(), lc)
			item.Action = pushActionUnchanged
			if len(item.Changes) > 0 {
				item.Action = pushActionUpdate
			}
		}
		if item.Action == pushActionCreate || item.Action == pushActionUpdate {
			switch {
			case lc.HtmlBody == "" && lc.TextBody == "":
				item.Action = pushActionConflict
				item.Reason = "neither content.html nor content.txt has content"
			case lc.TemplateType == templateTypeStandard && strings.TrimSpace(lc.Subject) == "":
				item.Action = pushActionConflict
				item.Reason = "Standard templates need a Subject in meta.json"
			case lc.TemplateType == templateTypeStandard && lc.LayoutTemplate != "" && !layoutsAvailable[lc.LayoutTemplate]:
				item.Action = pushActionBlocked
				item.Reason = fmt.Sprintf("layout %q is not in this directory or on the server; Postmark would reject it (ErrorCode %d)", lc.LayoutTemplate, postmarkErrTemplateNotFound)
			}
		}
		items = append(items, item)
	}
	if !prune {
		return items
	}

	deleteSet := map[string]templateDetail{}
	for alias, r := range remoteByAlias {
		if !localAliases[alias] {
			deleteSet[alias] = r
		}
	}
	// Layout users after the push: local standard templates plus remote
	// standard templates that stay on the server.
	layoutUsers := map[string][]string{}
	for _, t := range local {
		if t.Meta.TemplateType == templateTypeStandard && t.Meta.LayoutTemplate != "" {
			layoutUsers[t.Meta.LayoutTemplate] = append(layoutUsers[t.Meta.LayoutTemplate], t.Meta.Alias)
		}
	}
	for _, r := range remote {
		ttype := normalizeTemplateType(r.TemplateType)
		if ttype == templateTypeLayout || r.LayoutTemplate == "" || localAliases[r.Alias] {
			continue
		}
		if _, deleting := deleteSet[r.Alias]; deleting {
			continue
		}
		label := r.Alias
		if label == "" {
			label = fmt.Sprintf("%q (id %d)", r.Name, r.TemplateId)
		}
		layoutUsers[r.LayoutTemplate] = append(layoutUsers[r.LayoutTemplate], label)
	}
	deletes := make([]templateDetail, 0, len(deleteSet))
	for _, r := range deleteSet {
		deletes = append(deletes, r)
	}
	sort.SliceStable(deletes, func(i, j int) bool {
		li := normalizeTemplateType(deletes[i].TemplateType) == templateTypeLayout
		lj := normalizeTemplateType(deletes[j].TemplateType) == templateTypeLayout
		if li != lj {
			return !li
		}
		return deletes[i].Alias < deletes[j].Alias
	})
	for _, r := range deletes {
		rc := r.content()
		item := templatePushItem{
			Alias:          rc.Alias,
			Name:           rc.Name,
			TemplateType:   rc.TemplateType,
			LayoutTemplate: rc.LayoutTemplate,
			Action:         pushActionDelete,
			Reason:         "on the server but not in this directory",
		}
		if rc.TemplateType == templateTypeLayout {
			if users := layoutUsers[rc.Alias]; len(users) > 0 {
				sort.Strings(users)
				item.Action = pushActionBlocked
				item.Reason = fmt.Sprintf("layout is still used by %s; Postmark refuses to delete it (ErrorCode %d)", strings.Join(users, ", "), postmarkErrLayoutInUse)
			}
		}
		items = append(items, item)
	}
	return items
}

func summarizePushPlan(items []templatePushItem) map[string]int {
	summary := map[string]int{
		pushActionCreate: 0, pushActionUpdate: 0, pushActionUnchanged: 0,
		pushActionDelete: 0, pushActionConflict: 0, pushActionBlocked: 0,
	}
	for _, it := range items {
		summary[it.Action]++
	}
	return summary
}

func templateCreateBody(t *localTemplate) map[string]any {
	body := map[string]any{
		"Name":         t.Meta.Name,
		"Alias":        t.Meta.Alias,
		"HtmlBody":     t.HtmlBody,
		"TextBody":     t.TextBody,
		"TemplateType": t.Meta.TemplateType,
	}
	if t.Meta.TemplateType == templateTypeStandard {
		body["Subject"] = t.Meta.Subject
		if t.Meta.LayoutTemplate != "" {
			body["LayoutTemplate"] = t.Meta.LayoutTemplate
		}
	}
	return body
}

// templateUpdateBody always sends LayoutTemplate for Standard templates:
// an empty string clears a layout that was removed locally.
func templateUpdateBody(t *localTemplate) map[string]any {
	body := map[string]any{
		"Name":     t.Meta.Name,
		"Alias":    t.Meta.Alias,
		"HtmlBody": t.HtmlBody,
		"TextBody": t.TextBody,
	}
	if t.Meta.TemplateType == templateTypeStandard {
		body["Subject"] = t.Meta.Subject
		body["LayoutTemplate"] = t.Meta.LayoutTemplate
	}
	return body
}

// executeTemplatePushPlan issues creates and updates in plan order, then
// deletes. It stops at the first rate-limit error, marking the rest not
// attempted, and returns it. Deletes run only when every local template was
// pushed: after a failed, conflicted, or blocked upsert, deleting the server
// copy of a renamed template would leave neither version.
func executeTemplatePushPlan(ctx context.Context, c *client.Client, items []templatePushItem) (writes, failed int, rateErr error) {
	run := func(it *templatePushItem) {
		if rateErr != nil {
			it.Status = pushStatusNotAttempted
			return
		}
		path := "/templates/" + url.PathEscape(it.Alias)
		var err error
		switch it.Action {
		case pushActionCreate:
			_, _, err = c.Post(ctx, "/templates", templateCreateBody(it.local))
		case pushActionUpdate:
			_, _, err = c.Put(ctx, path, templateUpdateBody(it.local))
		case pushActionDelete:
			_, _, err = c.Delete(ctx, path)
		}
		if err != nil {
			it.Status = statusFailed
			it.Error = postmarkErrorText(err)
			failed++
			if isRateLimited(err) {
				rateErr = err
			}
			return
		}
		it.Status = pushStatusDone
		writes++
	}
	for i := range items {
		if it := &items[i]; it.Action == pushActionCreate || it.Action == pushActionUpdate {
			run(it)
		}
	}
	held := failed + unpushableLocalTemplates(items)
	for i := range items {
		it := &items[i]
		if it.Action != pushActionDelete {
			continue
		}
		if held > 0 {
			it.Status = pushStatusNotAttempted
			it.Error = fmt.Sprintf("not deleted: %d local template(s) failed or could not be pushed", held)
			continue
		}
		run(it)
	}
	return writes, failed, rateErr
}

// unpushableLocalTemplates counts local templates the plan could not push
// (conflicts and missing layouts). A server layout kept during --prune is
// blocked too, but it has no local template and is not counted.
func unpushableLocalTemplates(items []templatePushItem) int {
	n := 0
	for _, it := range items {
		if it.local != nil && (it.Action == pushActionConflict || it.Action == pushActionBlocked) {
			n++
		}
	}
	return n
}

func newTemplatesPushDirCmd(flags *rootFlags) *cobra.Command {
	var prune bool
	cmd := &cobra.Command{
		Use:   "push <dir>",
		Short: "Diff a pulled template directory against the server and push the changes; plans by default and writes with --yes",
		Long: strings.Trim(`
Compare every template in <dir> (official postmark-cli layout) with the server
by alias on Name, Subject, HtmlBody, TextBody, and LayoutTemplate, and print a
plan of creates, updates, and unchanged templates. Nothing is written without
--yes.

With --yes, layouts are pushed first (POST /templates for new ones, PUT
/templates/{alias} for changed ones), then standard templates. --prune adds
deletes for templates on the server that are missing locally; a layout that
templates still use is never deleted, and templates without an alias are never
touched. Deletes run only after every local template was pushed. Global
--dry-run prints the request without contacting Postmark.

Exit codes: 0 success, 5 a write failed, 6 --yes skipped a local template
because of a conflict or missing layout.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli templates push ./templates --server "Main App"
  postmark-pp-cli templates push ./templates --server "Main App" --yes
  postmark-pp-cli templates push ./templates --prune --server "Main App" --json`, "\n"),
		Annotations: map[string]string{
			"pp:data-source":      "live",
			"pp:happy-args":       "dir=.",
			"pp:typed-exit-codes": "0,6",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "templates push")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("<dir> is required: a directory pulled with 'templates pull'"))
			}
			apply := flags.yes
			if apply && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "push templates to Postmark")
			}
			root := args[0]
			localAll, problems, err := readLocalTemplates(root)
			if err != nil {
				return usageErr(fmt.Errorf("reading %s: %w", root, err))
			}
			local, aliasProblems := validateLocalAliases(localAll)
			problems = append(problems, aliasProblems...)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newUncachedClient(flags)
			if err != nil {
				return err
			}
			summaries, total, err := listPostmarkTemplates(ctx, c, "All", 0)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if len(summaries) < total {
				return apiErr(fmt.Errorf("server reported %d templates but listing returned %d; refusing to diff an incomplete list", total, len(summaries)))
			}
			remote := make([]templateDetail, 0, len(summaries))
			unaliased := make([]templateRemoteRef, 0)
			for _, s := range summaries {
				if s.Alias == "" {
					unaliased = append(unaliased, templateRemoteRef{Name: s.Name, TemplateID: s.TemplateId})
					remote = append(remote, templateDetail{TemplateId: s.TemplateId, Name: s.Name, TemplateType: s.TemplateType, LayoutTemplate: s.LayoutTemplate})
					continue
				}
				d, err := getPostmarkTemplate(ctx, c, s.Alias)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching server copy of %s: %w", s.Alias, err), flags)
				}
				remote = append(remote, d)
			}
			items := buildTemplatePushPlan(local, remote, prune)
			view := templatePushView{
				Mode:            modePlan,
				Dir:             root,
				Prune:           prune,
				Items:           items,
				InvalidLocal:    problems,
				UnaliasedRemote: unaliased,
			}
			for i := range view.Items {
				switch {
				case isPushWrite(view.Items[i].Action):
					view.Items[i].Status = statusPlanned
				case view.Items[i].Action == pushActionUnchanged:
					view.Items[i].Status = pushActionUnchanged
				default:
					view.Items[i].Status = statusSkipped
				}
			}
			view.Summary = summarizePushPlan(view.Items)
			if len(localAll) == 0 && len(problems) == 0 {
				view.Note = "no meta.json files found under " + root
				if apply && prune {
					return usageErr(fmt.Errorf("refusing --prune --yes: no templates found under %s, which would delete every template on the server", root))
				}
			}
			var rateErr error
			if apply {
				if len(problems) > 0 {
					if err := printTemplatePushView(cmd, flags, view); err != nil {
						return err
					}
					return usageErr(fmt.Errorf("%d local template folder(s) are invalid; fix or remove them before pushing with --yes", len(problems)))
				}
				view.Mode = modeApply
				view.Writes, view.Failed, rateErr = executeTemplatePushPlan(ctx, c, view.Items)
			} else if view.Summary[pushActionCreate]+view.Summary[pushActionUpdate]+view.Summary[pushActionDelete] > 0 {
				view.Note = strings.TrimSpace(view.Note + " plan only: re-run with --yes to push")
			}
			if err := printTemplatePushView(cmd, flags, view); err != nil {
				return err
			}
			if rateErr != nil {
				return rateLimitErr(rateErr)
			}
			if view.Failed > 0 {
				return apiErr(fmt.Errorf("%d of %d template writes failed", view.Failed, view.Failed+view.Writes))
			}
			if n := unpushableLocalTemplates(view.Items); apply && n > 0 {
				return partialFailureErr(fmt.Errorf("%d local template(s) were not pushed because of a conflict or missing layout; see the plan", n))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&prune, "prune", false, "Also delete server templates that are missing locally (layouts still in use are kept)")
	return cmd
}

func printTemplatePushView(cmd *cobra.Command, flags *rootFlags, view templatePushView) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFilteredKeep(w, view, flags, "changes", "reason", "error", "layout_template")
	}
	fmt.Fprintf(w, "Template push %s for %s\n", view.Mode, view.Dir)
	if len(view.Items) > 0 {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "ACTION\tALIAS\tTYPE\tSTATUS\tDETAIL")
		for _, it := range view.Items {
			detail := strings.Join(it.Changes, ",")
			if it.Reason != "" && it.Action != pushActionUpdate {
				detail = it.Reason
			}
			if it.Error != "" {
				detail = it.Error
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", it.Action, it.Alias, it.TemplateType, it.Status, detail)
		}
		_ = tw.Flush()
	}
	s := view.Summary
	fmt.Fprintf(w, "create %d, update %d, unchanged %d, delete %d, blocked %d, conflict %d; writes %d, failed %d\n",
		s[pushActionCreate], s[pushActionUpdate], s[pushActionUnchanged], s[pushActionDelete], s[pushActionBlocked], s[pushActionConflict], view.Writes, view.Failed)
	for _, p := range view.InvalidLocal {
		fmt.Fprintf(w, "invalid: %s: %s\n", p.Path, p.Reason)
	}
	if len(view.UnaliasedRemote) > 0 {
		fmt.Fprintf(w, "%d server template(s) have no alias and are left alone\n", len(view.UnaliasedRemote))
	}
	if view.Note != "" {
		fmt.Fprintln(w, view.Note)
	}
	return nil
}

// ---------------------------------------------------------------------------
// templates render
// ---------------------------------------------------------------------------

const (
	renderModelFlag      = "flag"
	renderModelMeta      = "meta.json"
	renderModelSuggested = "suggested"
)

type templateRenderView struct {
	Alias            string                `json:"alias"`
	Name             string                `json:"name"`
	TemplateType     string                `json:"template_type"`
	LayoutTemplate   string                `json:"layout_template,omitempty"`
	Source           string                `json:"source"`
	ModelSource      string                `json:"model_source"`
	Valid            bool                  `json:"all_content_is_valid"`
	Subject          *string               `json:"subject"`
	HtmlBody         *string               `json:"html_body"`
	TextBody         *string               `json:"text_body"`
	ValidationErrors []templateRenderIssue `json:"validation_errors"`
	MissingModelKeys []string              `json:"missing_model_keys"`
	Model            json.RawMessage       `json:"model,omitempty"`
	SuggestedModel   json.RawMessage       `json:"suggested_model,omitempty"`
	Warnings         []string              `json:"warnings"`
}

// postmarkWithinWorkingDir resolves symlinks and rejects a path that lands
// outside the current directory.
func postmarkWithinWorkingDir(path string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	realCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return err
	}
	realPath, err := filepath.EvalSymlinks(filepath.Join(cwd, path))
	if err != nil {
		return fmt.Errorf("reading --model file: %w", err)
	}
	rel, err := filepath.Rel(realCwd, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("--model @file must stay inside the current directory")
	}
	return nil
}

// parseModelFlag reads --model as inline JSON or @path and requires a JSON
// object, the only shape Postmark accepts as a TestRenderModel.
func parseModelFlag(v string) (json.RawMessage, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	data := []byte(v)
	if strings.HasPrefix(v, "@") {
		path := filepath.Clean(strings.TrimPrefix(v, "@"))
		if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("--model @file must be a relative path inside the current directory")
		}
		if err := postmarkWithinWorkingDir(path); err != nil {
			return nil, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading --model file: %w", err)
		}
		data = b
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("--model must be a JSON object (inline or @file): %w", err)
	}
	return json.RawMessage(data), nil
}

func newTemplatesRenderCmd(flags *rootFlags) *cobra.Command {
	var dir, modelFlag string
	cmd := &cobra.Command{
		Use:   "render [alias]",
		Short: "Render one template with its layout and a test model",
		Long: strings.Trim(`
Use this command to see the rendered output of one template. Do NOT use it to validate every template before a push; use 'templates check' instead.

The template content comes from the server, or from a pulled directory with
--dir. Rendering runs through Postmark's validate endpoint (no email is sent),
applies the template's LayoutTemplate, and uses the first model found in this
order: --model, the TestRenderModel in meta.json, Postmark's suggested model.
Output includes the rendered Subject, HtmlBody, and TextBody plus validation
errors and any template variables the model does not supply.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli templates render password-reset --server "Main App"
  postmark-pp-cli templates render password-reset --dir ./templates --server "Main App" --json
  postmark-pp-cli templates render welcome --model '{"user":{"name":"Jane"}}' --server "Main App"`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "templates render")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("[alias] is required: the template alias to render"))
			}
			alias := strings.TrimSpace(args[0])
			flagModel, err := parseModelFlag(modelFlag)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			view := templateRenderView{
				ValidationErrors: make([]templateRenderIssue, 0),
				MissingModelKeys: make([]string, 0),
				Warnings:         make([]string, 0),
			}
			var content templateContent
			var metaModel json.RawMessage
			if dir != "" {
				local, _, err := readLocalTemplates(dir)
				if err != nil {
					return usageErr(fmt.Errorf("reading %s: %w", dir, err))
				}
				t, ok := findLocalTemplate(local, alias)
				if !ok {
					return notFoundErr(fmt.Errorf("no template with alias %q under %s", alias, dir))
				}
				content = t.content()
				merged, ok, layoutErr := inlineLocalLayout(content, local)
				if layoutErr != nil {
					return usageErr(layoutErr)
				}
				if ok {
					content = merged
					view.Warnings = append(view.Warnings, fmt.Sprintf("rendered with the local layout %q from %s, not the server's copy", t.Meta.LayoutTemplate, dir))
				}
				metaModel = t.Meta.TestRenderModel
				view.Source = t.Dir
			}
			c, err := newUncachedClient(flags)
			if err != nil {
				return err
			}
			if dir == "" {
				d, err := getPostmarkTemplate(ctx, c, alias)
				if err != nil {
					if f, ok := postmarkAPIFailure(err); ok && (f.ErrorCode == postmarkErrTemplateNotFound || f.Status == 404) {
						return notFoundErr(fmt.Errorf("no template with alias %q on this server: %s", alias, f.String()))
					}
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				content = d.content()
				view.Source = "server"
			}
			model := flagModel
			view.ModelSource = renderModelFlag
			if !hasTemplateModel(model) {
				model = metaModel
				view.ModelSource = renderModelMeta
			}
			if !hasTemplateModel(model) {
				model = nil
				view.ModelSource = renderModelSuggested
			}
			view.Alias, view.Name, view.TemplateType = content.Alias, content.Name, content.TemplateType
			if content.TemplateType == templateTypeStandard {
				view.LayoutTemplate = content.LayoutTemplate
			}
			resp, fellBack, err := validateTemplateWithLayoutFallback(ctx, c, validateRequestFor(content, model))
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if fellBack {
				view.Warnings = append(view.Warnings, fmt.Sprintf("layout %q is not on the server; rendered without it", content.LayoutTemplate))
			}
			view.Valid = resp.AllContentIsValid
			view.ValidationErrors = resp.issues()
			view.Subject = renderedPart(resp.Subject)
			view.HtmlBody = renderedPart(resp.HtmlBody)
			view.TextBody = renderedPart(resp.TextBody)
			view.SuggestedModel = resp.SuggestedTemplateModel
			if model != nil {
				view.Model = model
				view.MissingModelKeys = missingTemplateModelKeys(resp.SuggestedTemplateModel, model)
			} else if hasTemplateModel(resp.SuggestedTemplateModel) {
				view.Model = resp.SuggestedTemplateModel
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "message", "part")
			}
			printTemplateRenderHuman(cmd.OutOrStdout(), view)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Render from a pulled template directory instead of the server copy")
	cmd.Flags().StringVar(&modelFlag, "model", "", "Test model as inline JSON or @file (overrides meta.json TestRenderModel)")
	return cmd
}

func renderedPart(p *templateValidatePart) *string {
	if p == nil {
		return nil
	}
	return p.RenderedContent
}

func printTemplateRenderHuman(w io.Writer, v templateRenderView) {
	layout := v.LayoutTemplate
	if layout == "" {
		layout = "none"
	}
	fmt.Fprintf(w, "Template %s (%s, layout: %s) from %s, model: %s\n", v.Alias, v.TemplateType, layout, v.Source, v.ModelSource)
	if v.Subject != nil {
		fmt.Fprintf(w, "Subject: %s\n", *v.Subject)
	}
	if v.TextBody != nil {
		fmt.Fprintf(w, "\n--- Text ---\n%s\n", *v.TextBody)
	}
	if v.HtmlBody != nil {
		fmt.Fprintf(w, "\n--- HTML ---\n%s\n", *v.HtmlBody)
	}
	if len(v.ValidationErrors) == 0 {
		fmt.Fprintln(w, "\nNo validation errors.")
	} else {
		fmt.Fprintln(w, "\nValidation errors:")
		for _, e := range v.ValidationErrors {
			loc := ""
			if e.Line != nil {
				loc = fmt.Sprintf(" (line %d", *e.Line)
				if e.Position != nil {
					loc += fmt.Sprintf(", char %d", *e.Position)
				}
				loc += ")"
			}
			fmt.Fprintf(w, "  %s%s: %s\n", e.Part, loc, e.Message)
		}
	}
	if len(v.MissingModelKeys) > 0 {
		fmt.Fprintf(w, "Model is missing: %s\n", strings.Join(v.MissingModelKeys, ", "))
	}
	for _, warn := range v.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, err := root.Find([]string{"templates"})
		if err == nil && parent != root {
			addNovelCommandIfAbsent(parent, newTemplatesPullCmd(flags))
			addNovelCommandIfAbsent(parent, newTemplatesPushDirCmd(flags))
			addNovelCommandIfAbsent(parent, newTemplatesRenderCmd(flags))
		}
	})
}
