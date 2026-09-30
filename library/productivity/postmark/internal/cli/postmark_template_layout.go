// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// On-disk template layout shared by `templates pull`, `templates push`,
// `templates render`, and `templates check`. The layout matches the official
// postmark-cli so a directory pulled by either tool works with the other:
//
//	<dir>/<alias>/content.html
//	<dir>/<alias>/content.txt
//	<dir>/<alias>/meta.json
//	<dir>/_layouts/<alias>/{content.html,content.txt,meta.json}

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	templateTypeStandard = "Standard"
	templateTypeLayout   = "Layout"
	templateLayoutsDir   = "_layouts"
	templateMetaFileName = "meta.json"
	templateHTMLFileName = "content.html"
	templateTextFileName = "content.txt"
)

// Field names and order follow the official postmark-cli meta.json so files
// round-trip byte-for-byte between the two tools.
type templateMeta struct {
	Name            string          `json:"Name"`
	Alias           string          `json:"Alias"`
	Subject         string          `json:"Subject,omitempty"`
	TemplateType    string          `json:"TemplateType"`
	LayoutTemplate  string          `json:"LayoutTemplate,omitempty"`
	TestRenderModel json.RawMessage `json:"TestRenderModel,omitempty"`
}

// templateContent is the comparable shape of one template, whether it came
// from the server or from a pulled directory.
type templateContent struct {
	Name           string
	Alias          string
	Subject        string
	HtmlBody       string
	TextBody       string
	TemplateType   string
	LayoutTemplate string
}

type localTemplate struct {
	Meta     templateMeta
	HtmlBody string
	TextBody string
	Dir      string
}

type localTemplateProblem struct {
	Path   string `json:"path"`
	Alias  string `json:"alias,omitempty"`
	Reason string `json:"reason"`
}

func (t localTemplate) content() templateContent {
	return templateContent{
		Name:           t.Meta.Name,
		Alias:          t.Meta.Alias,
		Subject:        t.Meta.Subject,
		HtmlBody:       t.HtmlBody,
		TextBody:       t.TextBody,
		TemplateType:   t.Meta.TemplateType,
		LayoutTemplate: t.Meta.LayoutTemplate,
	}
}

func (t localTemplate) hasTestModel() bool {
	return hasTemplateModel(t.Meta.TestRenderModel)
}

func hasTemplateModel(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

// normalizeTemplateType maps any casing of standard/layout to the API value;
// unknown values pass through so callers can report them.
func normalizeTemplateType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "standard":
		return templateTypeStandard
	case "layout":
		return templateTypeLayout
	}
	return strings.TrimSpace(t)
}

// safeTemplateAlias reports whether alias can be used as a single directory
// name without escaping the pull root or colliding with the layouts folder.
func safeTemplateAlias(alias, templateType string) bool {
	if alias == "" || alias == "." || alias == ".." {
		return false
	}
	if strings.ContainsAny(alias, `/\`+"\x00") {
		return false
	}
	if filepath.Base(alias) != alias {
		return false
	}
	if templateType != templateTypeLayout && strings.EqualFold(alias, templateLayoutsDir) {
		return false
	}
	return true
}

func templateLocalDir(root, alias, templateType string) string {
	if templateType == templateTypeLayout {
		return filepath.Join(root, templateLayoutsDir, alias)
	}
	return filepath.Join(root, alias)
}

// templateContentDiff lists the fields that differ between the server copy
// and the local copy. Subject and layout only count for Standard templates,
// because layouts cannot have either.
func templateContentDiff(remote, local templateContent) []string {
	changes := make([]string, 0, 5)
	if remote.Name != local.Name {
		changes = append(changes, "name")
	}
	standard := local.TemplateType != templateTypeLayout
	if standard && remote.Subject != local.Subject {
		changes = append(changes, "subject")
	}
	if remote.HtmlBody != local.HtmlBody {
		changes = append(changes, "html")
	}
	if remote.TextBody != local.TextBody {
		changes = append(changes, "text")
	}
	if standard && remote.LayoutTemplate != local.LayoutTemplate {
		changes = append(changes, "layout")
	}
	return changes
}

// readLocalTemplates walks root for meta.json files the same way the official
// postmark-cli does and loads each template's content. Hidden directories are
// skipped so a checked-out .git folder is never scanned.
func readLocalTemplates(root string) ([]localTemplate, []localTemplateProblem, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil, nil, usageErr(fmt.Errorf("template folder %s does not exist; run 'postmark-pp-cli templates pull %s' first", root, root))
	}
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("%s is not a directory", root)
	}
	templates := make([]localTemplate, 0)
	problems := make([]localTemplateProblem, 0)
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			problems = append(problems, localTemplateProblem{Path: path, Reason: err.Error()})
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != templateMetaFileName {
			return nil
		}
		tpl, problem := loadLocalTemplate(path)
		if problem != nil {
			problems = append(problems, *problem)
			return nil
		}
		templates = append(templates, tpl)
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}
	sortLocalTemplates(templates)
	return templates, problems, nil
}

func loadLocalTemplate(metaPath string) (localTemplate, *localTemplateProblem) {
	dir := filepath.Dir(metaPath)
	raw, err := os.ReadFile(metaPath) // #nosec G304 -- meta.json inside the template folder the user selected.
	if err != nil {
		return localTemplate{}, &localTemplateProblem{Path: metaPath, Reason: err.Error()}
	}
	var meta templateMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return localTemplate{}, &localTemplateProblem{Path: metaPath, Reason: "invalid meta.json: " + err.Error()}
	}
	meta.TemplateType = normalizeTemplateType(meta.TemplateType)
	if meta.TemplateType == "" {
		meta.TemplateType = templateTypeStandard
		if filepath.Base(filepath.Dir(dir)) == templateLayoutsDir {
			meta.TemplateType = templateTypeLayout
		}
	}
	if !hasTemplateModel(meta.TestRenderModel) {
		meta.TestRenderModel = nil
	}
	html, err := readOptionalFile(filepath.Join(dir, templateHTMLFileName))
	if err != nil {
		return localTemplate{}, &localTemplateProblem{Path: metaPath, Alias: meta.Alias, Reason: err.Error()}
	}
	text, err := readOptionalFile(filepath.Join(dir, templateTextFileName))
	if err != nil {
		return localTemplate{}, &localTemplateProblem{Path: metaPath, Alias: meta.Alias, Reason: err.Error()}
	}
	return localTemplate{Meta: meta, HtmlBody: html, TextBody: text, Dir: dir}, nil
}

func readOptionalFile(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- fixed file name inside the user-selected template folder.
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// sortLocalTemplates orders layouts first (they must be pushed before the
// templates that use them), then by alias.
func sortLocalTemplates(templates []localTemplate) {
	sort.SliceStable(templates, func(i, j int) bool {
		li := templates[i].Meta.TemplateType == templateTypeLayout
		lj := templates[j].Meta.TemplateType == templateTypeLayout
		if li != lj {
			return li
		}
		return templates[i].Meta.Alias < templates[j].Meta.Alias
	})
}

// marshalTemplateMeta renders meta.json the way the official CLI does:
// two-space indent, no HTML escaping, no trailing newline.
func marshalTemplateMeta(meta templateMeta) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(meta); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// metaForContent builds the meta.json for a template. Subject is written only
// when set and LayoutTemplate only for Standard templates, as the official
// CLI does.
func metaForContent(c templateContent, testModel json.RawMessage) templateMeta {
	meta := templateMeta{
		Name:         c.Name,
		Alias:        c.Alias,
		Subject:      c.Subject,
		TemplateType: c.TemplateType,
	}
	if c.TemplateType == templateTypeStandard {
		meta.LayoutTemplate = c.LayoutTemplate
	}
	if hasTemplateModel(testModel) {
		meta.TestRenderModel = testModel
	}
	return meta
}

// templateWriteResult reports what writeTemplateFiles changed on disk.
type templateWriteResult struct {
	Dir     string
	Changed bool
}

// writeTemplateFiles writes one template into root using the official layout.
// A body that is empty on the server removes a stale local file so a later
// push cannot resurrect content that no longer exists. Files whose bytes
// already match are left untouched.
func writeTemplateFiles(root string, c templateContent, testModel json.RawMessage) (templateWriteResult, error) {
	dir := templateLocalDir(root, c.Alias, c.TemplateType)
	res := templateWriteResult{Dir: dir}
	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- template folders are project files meant for version control.
		return res, err
	}
	metaBytes, err := marshalTemplateMeta(metaForContent(c, testModel))
	if err != nil {
		return res, err
	}
	files := []struct {
		name    string
		content string
		keep    bool
	}{
		{templateHTMLFileName, c.HtmlBody, c.HtmlBody != ""},
		{templateTextFileName, c.TextBody, c.TextBody != ""},
		{templateMetaFileName, string(metaBytes), true},
	}
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		existing, readErr := os.ReadFile(path) // #nosec G304 -- file inside a template folder whose alias passed safeTemplateAlias.
		exists := readErr == nil
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return res, readErr
		}
		if !f.keep {
			if exists {
				if err := os.Remove(path); err != nil {
					return res, err
				}
				res.Changed = true
			}
			continue
		}
		if exists && string(existing) == f.content {
			continue
		}
		if err := os.WriteFile(path, []byte(f.content), 0o644); err != nil { // #nosec G306 -- template sources are shared project files meant for version control.
			return res, err
		}
		res.Changed = true
	}
	return res, nil
}

// findLocalTemplate returns the template with the given alias from a pulled
// directory, searching layouts too.
func findLocalTemplate(templates []localTemplate, alias string) (localTemplate, bool) {
	for _, t := range templates {
		if t.Meta.Alias == alias {
			return t, true
		}
	}
	for _, t := range templates {
		if strings.EqualFold(t.Meta.Alias, alias) {
			return t, true
		}
	}
	return localTemplate{}, false
}
