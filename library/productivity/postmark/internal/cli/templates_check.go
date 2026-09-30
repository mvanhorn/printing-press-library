// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

const (
	checkStatusPass = "pass"
	checkStatusFail = "fail"

	checkKindRenderError     = "render_error"
	checkKindMissingKeys     = "missing_model_keys"
	checkKindDanglingLayout  = "dangling_layout"
	checkKindMissingAlias    = "missing_alias"
	checkKindMissingSubject  = "missing_subject"
	checkKindValidateFailure = "validate_rejected"
	checkKindInvalidMeta     = "invalid_meta"

	checkModelMeta = "meta.json"
	checkModelNone = "none"

	// templatesCheckFailedExit is the typed exit code for "at least one
	// template failed"; declared in pp:typed-exit-codes.
	templatesCheckFailedExit = 3
)

type templateCheckProblem struct {
	Kind     string `json:"kind"`
	Message  string `json:"message"`
	Part     string `json:"part,omitempty"`
	Line     *int   `json:"line,omitempty"`
	Position *int   `json:"position,omitempty"`
}

type templateCheckRow struct {
	Alias            string                 `json:"alias"`
	Name             string                 `json:"name"`
	TemplateType     string                 `json:"template_type"`
	TemplateID       int64                  `json:"template_id,omitempty"`
	LayoutTemplate   string                 `json:"layout_template,omitempty"`
	Path             string                 `json:"path,omitempty"`
	Status           string                 `json:"status"`
	TestModel        string                 `json:"test_model"`
	Problems         []templateCheckProblem `json:"problems"`
	MissingModelKeys []string               `json:"missing_model_keys"`
	Warnings         []string               `json:"warnings"`
}

type templateCheckView struct {
	Mode          string                 `json:"mode"`
	Dir           string                 `json:"dir,omitempty"`
	Total         int                    `json:"total"`
	Passed        int                    `json:"passed"`
	Failed        int                    `json:"failed"`
	Templates     []templateCheckRow     `json:"templates"`
	FetchFailures []postmarkFetchFailure `json:"fetch_failures,omitempty"`
	Note          string                 `json:"note,omitempty"`
}

func newTemplateCheckRow(c templateContent) templateCheckRow {
	row := templateCheckRow{
		Alias:            c.Alias,
		Name:             c.Name,
		TemplateType:     c.TemplateType,
		Status:           checkStatusPass,
		TestModel:        checkModelNone,
		Problems:         make([]templateCheckProblem, 0),
		MissingModelKeys: make([]string, 0),
		Warnings:         make([]string, 0),
	}
	if c.TemplateType == templateTypeStandard {
		row.LayoutTemplate = c.LayoutTemplate
	}
	return row
}

func (r *templateCheckRow) fail(kind, message string) {
	r.Problems = append(r.Problems, templateCheckProblem{Kind: kind, Message: message})
	r.Status = checkStatusFail
}

// staticTemplateProblems runs the checks that need no API call: alias,
// subject, and whether the referenced layout exists anywhere.
func staticTemplateProblems(row *templateCheckRow, c templateContent, layoutExists func(string) bool) (dangling bool) {
	if strings.TrimSpace(c.Alias) == "" {
		row.fail(checkKindMissingAlias, "template has no alias; pull and push match templates by alias, so this one is skipped by both")
	}
	if c.TemplateType != templateTypeStandard {
		return false
	}
	if strings.TrimSpace(c.Subject) == "" {
		row.fail(checkKindMissingSubject, "Standard template has no Subject")
	}
	if c.LayoutTemplate != "" && !layoutExists(c.LayoutTemplate) {
		row.fail(checkKindDanglingLayout, fmt.Sprintf("LayoutTemplate %q does not match any layout", c.LayoutTemplate))
		return true
	}
	return false
}

// applyValidateResult folds a validate response into the row: render errors
// and, when a test model was supplied, the variables it does not cover.
func applyValidateResult(row *templateCheckRow, resp templateValidateResponse, model json.RawMessage) {
	for _, issue := range resp.issues() {
		row.Problems = append(row.Problems, templateCheckProblem{
			Kind: checkKindRenderError, Message: issue.Message, Part: issue.Part, Line: issue.Line, Position: issue.Position,
		})
		row.Status = checkStatusFail
	}
	if !hasTemplateModel(model) {
		return
	}
	missing := missingTemplateModelKeys(resp.SuggestedTemplateModel, model)
	if len(missing) > 0 {
		row.MissingModelKeys = missing
		row.fail(checkKindMissingKeys, "TestRenderModel lacks variables the template uses: "+strings.Join(missing, ", "))
	}
}

// checkTemplateContent validates one template. A 422 from Postmark is a
// template problem; any other error is returned as a fetch failure.
func checkTemplateContent(ctx context.Context, c *client.Client, row *templateCheckRow, content templateContent, model json.RawMessage, layoutExists, serverHasLayout func(string) bool) error {
	dangling := staticTemplateProblems(row, content, layoutExists)
	req := validateRequestFor(content, model)
	if content.TemplateType == templateTypeStandard && content.LayoutTemplate != "" {
		switch {
		case dangling:
			req.LayoutTemplate = ""
		case !serverHasLayout(content.LayoutTemplate):
			req.LayoutTemplate = ""
			row.Warnings = append(row.Warnings, fmt.Sprintf("layout %q is not on the server yet; rendered without it", content.LayoutTemplate))
		}
	}
	resp, fellBack, err := validateTemplateWithLayoutFallback(ctx, c, req)
	if err != nil {
		if f, ok := postmarkAPIFailure(err); ok && f.Status == 422 {
			row.fail(checkKindValidateFailure, f.String())
			return nil
		}
		return err
	}
	if fellBack {
		row.Warnings = append(row.Warnings, fmt.Sprintf("layout %q is not on the server; rendered without it", content.LayoutTemplate))
	}
	applyValidateResult(row, resp, model)
	return nil
}

// missingTemplateModelKeys lists the dotted paths (arrays as "items[].field")
// present in Postmark's SuggestedTemplateModel but absent from the supplied
// model: variables the template uses that the test model does not provide.
func missingTemplateModelKeys(suggested, model json.RawMessage) []string {
	out := make([]string, 0)
	if !hasTemplateModel(suggested) {
		return out
	}
	var s, m any
	if err := json.Unmarshal(suggested, &s); err != nil {
		return out
	}
	if hasTemplateModel(model) {
		if err := json.Unmarshal(model, &m); err != nil {
			m = nil
		}
	}
	seen := map[string]bool{}
	collectMissingModelKeys("", s, m, seen)
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func joinModelPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func collectMissingModelKeys(prefix string, suggested, model any, out map[string]bool) {
	switch s := suggested.(type) {
	case map[string]any:
		mm, _ := model.(map[string]any)
		for k, sv := range s {
			path := joinModelPath(prefix, k)
			mv, ok := mm[k]
			if !ok {
				collectModelLeafPaths(path, sv, out)
				continue
			}
			collectMissingModelKeys(path, sv, mv, out)
		}
	case []any:
		if len(s) == 0 {
			return
		}
		ma, ok := model.([]any)
		if !ok {
			return
		}
		for _, elem := range ma {
			collectMissingModelKeys(prefix+"[]", s[0], elem, out)
		}
	}
}

func collectModelLeafPaths(path string, v any, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			out[path] = true
			return
		}
		for k, child := range x {
			collectModelLeafPaths(joinModelPath(path, k), child, out)
		}
	case []any:
		if len(x) == 0 {
			out[path] = true
			return
		}
		collectModelLeafPaths(path+"[]", x[0], out)
	default:
		out[path] = true
	}
}

func newNovelTemplatesCheckCmd(flags *rootFlags) *cobra.Command {
	var flagDir string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate every template and layout on a server or in a pulled folder and fail the build on render errors",
		Long: strings.Trim(`
Use this command to gate a whole server or pulled template directory before a push: it validates every template and layout and exits non-zero on any failure. Do NOT use this command to see the rendered output of one template; use 'templates render' instead.

Each template is rendered through Postmark's validate endpoint (no email is
sent) with its LayoutTemplate and, with --dir, the TestRenderModel from its
meta.json. A template fails on render errors, test-model variables the
template uses but the model lacks, a LayoutTemplate that matches no layout, a
missing alias, or a Standard template without a Subject. Postmark does not
store test models, so the missing-variable check needs --dir.

Exit codes:
  0  every template and layout passed
  3  at least one template failed a check`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli templates check --json
  postmark-pp-cli templates check --dir ./templates --server "Main App"
  postmark-pp-cli templates check --server "Main App" --json`, "\n"),
		Annotations: map[string]string{
			"pp:data-source":      "live",
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "templates check")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			var local []localTemplate
			var problems []localTemplateProblem
			if flagDir != "" {
				var err error
				local, problems, err = readLocalTemplates(flagDir)
				if err != nil {
					return usageErr(fmt.Errorf("reading %s: %w", flagDir, err))
				}
			}
			c, err := newUncachedClient(flags)
			if err != nil {
				return err
			}
			var view templateCheckView
			if flagDir != "" {
				view, err = runTemplatesCheckDir(ctx, c, flagDir, local, problems)
			} else {
				view, err = runTemplatesCheckServer(ctx, c)
			}
			if err != nil {
				if isRateLimited(err) {
					return rateLimitErr(err)
				}
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			failed := len(view.FetchFailures)
			warnPartialFailures(cmd.ErrOrStderr(), failed, failed+view.Total, "templates could not be checked", remainingNote(view.Total))
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "problems", "missing_model_keys", "warnings", "path", "layout_template"); err != nil {
					return err
				}
			} else {
				printTemplateCheckHuman(cmd.OutOrStdout(), view)
			}
			if view.Failed > 0 {
				return &cliError{code: templatesCheckFailedExit, err: fmt.Errorf("%d of %d templates failed the check", view.Failed, view.Total)}
			}
			if len(view.FetchFailures) > 0 {
				return apiErr(fmt.Errorf("%d template(s) could not be checked", len(view.FetchFailures)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDir, "dir", "", "Check a directory pulled with 'templates pull' instead of the server's templates")
	return cmd
}

func finalizeTemplateCheckView(view *templateCheckView) {
	view.Total = len(view.Templates)
	for _, r := range view.Templates {
		if r.Status == checkStatusFail {
			view.Failed++
		} else {
			view.Passed++
		}
	}
}

// runTemplatesCheckServer checks every template stored on the server.
func runTemplatesCheckServer(ctx context.Context, c *client.Client) (templateCheckView, error) {
	view := templateCheckView{Mode: "server", Templates: make([]templateCheckRow, 0),
		Note: "Postmark stores no test models, so missing-variable checks need --dir"}
	maxPages := 0
	if cliutil.IsDogfoodEnv() {
		maxPages = 1
	}
	summaries, total, err := listPostmarkTemplates(ctx, c, "All", maxPages)
	if err != nil {
		return view, err
	}
	layouts := map[string]bool{}
	for _, s := range summaries {
		if normalizeTemplateType(s.TemplateType) == templateTypeLayout && s.Alias != "" {
			layouts[s.Alias] = true
		}
	}
	if len(summaries) < total {
		view.Note = fmt.Sprintf("checked %d of %d templates (listing was curtailed)", len(summaries), total)
	}
	layoutExists := func(a string) bool { return layouts[a] }
	limit, limitNote := dogfoodTemplateLimit("checked")
	for i, s := range summaries {
		if limit > 0 && i >= limit {
			view.Note = limitNote
			break
		}
		idOrAlias := s.Alias
		if idOrAlias == "" {
			idOrAlias = strconv.FormatInt(s.TemplateId, 10)
		}
		d, err := getPostmarkTemplate(ctx, c, idOrAlias)
		if err != nil {
			if isRateLimited(err) {
				return view, err
			}
			view.FetchFailures = append(view.FetchFailures, itemFailure(idOrAlias, err))
			continue
		}
		content := d.content()
		row := newTemplateCheckRow(content)
		row.TemplateID = d.TemplateId
		if err := checkTemplateContent(ctx, c, &row, content, nil, layoutExists, layoutExists); err != nil {
			if isRateLimited(err) {
				return view, err
			}
			view.FetchFailures = append(view.FetchFailures, itemFailure(idOrAlias, err))
			continue
		}
		view.Templates = append(view.Templates, row)
	}
	finalizeTemplateCheckView(&view)
	return view, nil
}

// runTemplatesCheckDir checks a pulled directory. Layout references resolve
// against local layouts and the server's layouts, since a push can rely on
// either.
func runTemplatesCheckDir(ctx context.Context, c *client.Client, dir string, local []localTemplate, problems []localTemplateProblem) (templateCheckView, error) {
	view := templateCheckView{Mode: "dir", Dir: dir, Templates: make([]templateCheckRow, 0)}
	serverLayoutList, _, err := listPostmarkTemplates(ctx, c, templateTypeLayout, 0)
	if err != nil {
		return view, err
	}
	serverLayouts := map[string]bool{}
	for _, s := range serverLayoutList {
		if s.Alias != "" {
			serverLayouts[s.Alias] = true
		}
	}
	localLayouts := map[string]bool{}
	aliasPaths := map[string][]string{}
	for _, t := range local {
		if t.Meta.TemplateType == templateTypeLayout && t.Meta.Alias != "" {
			localLayouts[t.Meta.Alias] = true
		}
		if t.Meta.Alias != "" {
			aliasPaths[t.Meta.Alias] = append(aliasPaths[t.Meta.Alias], t.Dir)
		}
	}
	layoutExists := func(a string) bool { return localLayouts[a] || serverLayouts[a] }
	serverHasLayout := func(a string) bool { return serverLayouts[a] }
	for _, p := range problems {
		row := newTemplateCheckRow(templateContent{Alias: p.Alias})
		row.Path = p.Path
		row.fail(checkKindInvalidMeta, p.Reason)
		view.Templates = append(view.Templates, row)
	}
	limit, limitNote := dogfoodTemplateLimit("checked")
	for i, t := range local {
		if limit > 0 && i >= limit {
			view.Note = limitNote
			break
		}
		content := t.content()
		row := newTemplateCheckRow(content)
		row.Path = t.Dir
		if paths := aliasPaths[t.Meta.Alias]; len(paths) > 1 {
			row.fail(checkKindInvalidMeta, "alias is used by more than one folder: "+strings.Join(paths, ", "))
		}
		model := t.Meta.TestRenderModel
		if hasTemplateModel(model) {
			row.TestModel = checkModelMeta
		} else {
			model = nil
			row.Warnings = append(row.Warnings, "meta.json has no TestRenderModel; rendered with Postmark's suggested model")
		}
		merged, ok, layoutErr := inlineLocalLayout(content, local)
		if layoutErr != nil {
			row.fail(checkKindValidateFailure, layoutErr.Error())
			view.Templates = append(view.Templates, row)
			continue
		}
		if ok {
			// Validate against the layout in this folder, not the server's
			// copy, so the check matches what a push would deploy.
			content = merged
		}
		if err := checkTemplateContent(ctx, c, &row, content, model, layoutExists, serverHasLayout); err != nil {
			if isRateLimited(err) {
				return view, err
			}
			view.FetchFailures = append(view.FetchFailures, itemFailure(t.Dir, err))
			continue
		}
		view.Templates = append(view.Templates, row)
	}
	finalizeTemplateCheckView(&view)
	return view, nil
}

func printTemplateCheckHuman(w io.Writer, v templateCheckView) {
	target := "server"
	if v.Mode == "dir" {
		target = v.Dir
	}
	fmt.Fprintf(w, "Template check (%s): %d passed, %d failed of %d\n", target, v.Passed, v.Failed, v.Total)
	if len(v.Templates) > 0 {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "STATUS\tALIAS\tTYPE\tLAYOUT\tPROBLEMS")
		for _, r := range v.Templates {
			alias := r.Alias
			if alias == "" {
				alias = "(no alias) " + r.Name
			}
			msgs := make([]string, 0, len(r.Problems))
			for _, p := range r.Problems {
				msg := p.Message
				if p.Part != "" {
					msg = p.Part + ": " + msg
				}
				msgs = append(msgs, msg)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Status, alias, r.TemplateType, r.LayoutTemplate, strings.Join(msgs, "; "))
		}
		_ = tw.Flush()
	}
	for _, f := range v.FetchFailures {
		fmt.Fprintf(w, "could not check %s: %s\n", f.ID, f.Error)
	}
	if v.Note != "" {
		fmt.Fprintln(w, v.Note)
	}
}
