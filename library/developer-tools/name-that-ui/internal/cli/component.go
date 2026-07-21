// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
// Component commands intentionally read only the NameThatUI Sync mirror.
func init() {
	registerNovelCommand(registerComponentCmd)
}

func registerComponentCmd(rootCmd *cobra.Command, flags *rootFlags) {
	rootCmd.AddCommand(newComponentCmd(flags))
}

type componentMeta struct {
	DataSource string `json:"data_source"`
	SyncedAt   string `json:"synced_at,omitempty"`
	Stale      bool   `json:"stale"`
}

type componentCandidates struct {
	Ambiguous  bool                   `json:"ambiguous"`
	Candidates []namethatui.Component `json:"candidates"`
	Meta       componentMeta          `json:"meta"`
}

func newComponentCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "component", Short: "Read locally synced NameThatUI components", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.AddCommand(newComponentListCmd(flags, &dbPath))
	cmd.AddCommand(newComponentGetCmd(flags, &dbPath))
	cmd.AddCommand(newComponentAnatomyCmd(flags, &dbPath))
	cmd.AddCommand(newComponentAPICmd(flags, &dbPath))
	cmd.AddCommand(newComponentPromptCmd(flags, &dbPath))
	cmd.AddCommand(newComponentDebugPromptCmd(flags, &dbPath))
	cmd.AddCommand(newComponentRelatedCmd(flags, &dbPath))
	cmd.AddCommand(newComponentGuidanceCmd(flags, &dbPath))
	cmd.AddCommand(newComponentCompareCmd(flags, &dbPath))
	return cmd
}

func componentDBPath(value string) string {
	if value != "" {
		return value
	}
	return defaultDBPath("name-that-ui-pp-cli")
}

func componentDryRun(cmd *cobra.Command, flags *rootFlags, operation string, args []string) error {
	return componentPrint(cmd, flags, map[string]any{"dry_run": true, "operation": operation, "args": args, "data_source": "local", "sqlite_opened": false})
}

func componentStore(cmd *cobra.Command, path string) (*store.Store, componentMeta, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, componentMeta{}, fmt.Errorf("NameThatUI component mirror is unavailable; run 'name-that-ui-pp-cli sync --resources catalog' first: %w", err)
	}
	db, err := store.OpenReadOnlyContext(cmd.Context(), path)
	if err != nil {
		return nil, componentMeta{}, fmt.Errorf("opening NameThatUI component mirror; run 'name-that-ui-pp-cli sync --resources catalog' first: %w", err)
	}
	_, synced, count, err := db.GetSyncState("components")
	if err != nil || count == 0 {
		db.Close()
		return nil, componentMeta{}, fmt.Errorf("NameThatUI component mirror is unavailable; run 'name-that-ui-pp-cli sync --resources catalog' first")
	}
	meta := componentMeta{DataSource: "local"}
	if !synced.IsZero() {
		meta.SyncedAt = synced.UTC().Format(time.RFC3339)
		meta.Stale = time.Since(synced) > flagsMaxAge(cmd)
	}
	return db, meta, nil
}

func flagsMaxAge(cmd *cobra.Command) time.Duration {
	if f := cmd.Root().PersistentFlags().Lookup("max-age"); f != nil {
		if d, err := time.ParseDuration(f.Value.String()); err == nil && d > 0 {
			return d
		}
	}
	return 30 * time.Minute
}

func componentLoad(cmd *cobra.Command, path string) ([]namethatui.Component, componentMeta, error) {
	db, meta, err := componentStore(cmd, path)
	if err != nil {
		return nil, meta, err
	}
	defer db.Close()
	raw, err := db.List("components", 0)
	if err != nil {
		return nil, meta, err
	}
	items := make([]namethatui.Component, 0, len(raw))
	for _, item := range raw {
		var c namethatui.Component
		if json.Unmarshal(item, &c) == nil {
			c.AKA = nonNilStrings(c.AKA)
			c.Fuzzy = nonNilStrings(c.Fuzzy)
			c.API = nonNilAPIs(c.API)
			c.Parts = nonNilParts(c.Parts)
			c.Related = nonNilStrings(c.Related)
			items = append(items, c)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Platform == items[j].Platform {
			return items[i].Name < items[j].Name
		}
		return items[i].Platform < items[j].Platform
	})
	return items, meta, nil
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
func nonNilAPIs(v []namethatui.API) []namethatui.API {
	if v == nil {
		return []namethatui.API{}
	}
	return v
}
func nonNilParts(v []namethatui.Part) []namethatui.Part {
	if v == nil {
		return []namethatui.Part{}
	}
	return v
}

func componentNormalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

func componentResolve(items []namethatui.Component, query string) (*namethatui.Component, []namethatui.Component) {
	for i := range items {
		if items[i].ID == query {
			return &items[i], nil
		}
	}
	q := componentNormalize(query)
	exact := []namethatui.Component{}
	for _, c := range items {
		fields := append([]string{c.Name, c.Slug, c.Platform + "/" + c.Slug}, c.AKA...)
		for _, field := range fields {
			if componentNormalize(field) == q {
				exact = append(exact, c)
				break
			}
		}
	}
	if len(exact) == 1 {
		return &exact[0], nil
	}
	if len(exact) > 1 {
		return nil, exact
	}
	fuzzy := []namethatui.Component{}
	for _, c := range items {
		fields := append([]string{c.Name, c.Slug, c.Platform + "/" + c.Slug}, append(c.AKA, c.Fuzzy...)...)
		for _, field := range fields {
			n := componentNormalize(field)
			if len(q) >= 3 && (strings.Contains(n, q) || strings.Contains(q, n)) {
				fuzzy = append(fuzzy, c)
				break
			}
		}
	}
	if len(fuzzy) == 1 {
		return &fuzzy[0], nil
	}
	return nil, fuzzy
}

func newComponentListCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	var platform string
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List locally synced components", Example: "  name-that-ui-pp-cli component list --platform web --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return componentDryRun(cmd, flags, "component.list", args)
		}
		items, meta, err := componentLoad(cmd, componentDBPath(*dbPath))
		if err != nil {
			return err
		}
		out := make([]namethatui.Component, 0, len(items))
		for _, c := range items {
			if platform == "" || strings.EqualFold(platform, c.Platform) {
				out = append(out, c)
			}
		}
		if limit > 0 && len(out) > limit {
			out = out[:limit]
		}
		return componentPrint(cmd, flags, map[string]any{"results": out, "meta": meta})
	}}
	cmd.Flags().StringVar(&platform, "platform", "", "Filter by platform")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum results (0 for all)")
	return cmd
}

func newComponentGetCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newComponentReadCmd(flags, dbPath, &cobra.Command{Use: "get <component>", Short: "Get a component", Example: "  name-that-ui-pp-cli component get web/combobox --agent"}, "get")
}

func newComponentAnatomyCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newComponentReadCmd(flags, dbPath, &cobra.Command{Use: "anatomy <component>", Short: "Get component anatomy", Example: "  name-that-ui-pp-cli component anatomy web/combobox --agent"}, "anatomy")
}

func newComponentAPICmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newComponentReadCmd(flags, dbPath, &cobra.Command{Use: "api <component>", Short: "Get component API mappings", Example: "  name-that-ui-pp-cli component api web/combobox --framework ARIA --agent"}, "api")
}

func newComponentPromptCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newComponentReadCmd(flags, dbPath, &cobra.Command{Use: "prompt <component>", Short: "Get a component implementation prompt", Example: "  name-that-ui-pp-cli component prompt web/combobox --agent"}, "prompt")
}

func newComponentDebugPromptCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newComponentReadCmd(flags, dbPath, &cobra.Command{Use: "debug-prompt <component>", Short: "Get a component debug prompt", Example: "  name-that-ui-pp-cli component debug-prompt web/combobox --agent"}, "debug-prompt")
}

func newComponentRelatedCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newComponentReadCmd(flags, dbPath, &cobra.Command{Use: "related <component>", Short: "Get related components", Example: "  name-that-ui-pp-cli component related web/combobox --agent"}, "related")
}

func newComponentGuidanceCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newComponentReadCmd(flags, dbPath, &cobra.Command{Use: "guidance <component>", Short: "Assemble source-backed component guidance", Example: "  name-that-ui-pp-cli component guidance web/combobox --agent"}, "guidance")
}

func newComponentReadCmd(flags *rootFlags, dbPath *string, cmd *cobra.Command, operation string) *cobra.Command {
	var framework string
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if isPrintingPressInvalidFixture(args) {
			return usageErr(fmt.Errorf("invalid component fixture"))
		}
		if dryRunOK(flags) {
			return componentDryRun(cmd, flags, "component."+operation, args)
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("%s requires <platform/slug-or-name>", cmd.CommandPath()))
		}
		items, meta, err := componentLoad(cmd, componentDBPath(*dbPath))
		if err != nil {
			return err
		}
		c, candidates := componentResolve(items, args[0])
		if c == nil {
			return componentPrint(cmd, flags, componentCandidates{Ambiguous: len(candidates) > 1, Candidates: nonNilComponents(candidates), Meta: meta})
		}
		var result any
		switch operation {
		case "get":
			result = map[string]any{"result": c, "meta": meta}
		case "anatomy":
			result = map[string]any{"component": c, "parts": nonNilParts(c.Parts), "source_url": c.SourceURL, "meta": meta}
		case "api":
			result = map[string]any{"component": c, "api": componentAPIs(c.API, framework), "source_url": c.SourceURL, "meta": meta}
		case "prompt":
			result = map[string]any{"component": c, "prompt": c.Prompt, "api": componentAPIs(c.API, framework), "source_url": c.SourceURL, "meta": meta}
		case "debug-prompt":
			result = map[string]any{"component": c, "debug_prompt": c.DebugPrompt, "source_url": c.SourceURL, "meta": meta}
		case "related":
			result = map[string]any{"component": c, "related": componentRelated(items, c.Related), "source_url": c.SourceURL, "meta": meta}
		case "guidance":
			result = componentGuidance(items, c, framework, meta)
		}
		return componentPrint(cmd, flags, result)
	}
	if operation == "api" || operation == "prompt" || operation == "guidance" {
		cmd.Flags().StringVar(&framework, "framework", "", "Filter API mappings by framework")
	}
	return cmd
}

func isPrintingPressInvalidFixture(args []string) bool {
	for _, arg := range args {
		if arg == "__printing_press_invalid__" {
			return true
		}
	}
	return false
}

func nonNilComponents(v []namethatui.Component) []namethatui.Component {
	if v == nil {
		return []namethatui.Component{}
	}
	return v
}
func componentAPIs(apis []namethatui.API, framework string) []namethatui.API {
	out := make([]namethatui.API, 0, len(apis))
	for _, a := range apis {
		if framework == "" || strings.EqualFold(framework, a.Framework) || (strings.EqualFold(framework, "web") && (strings.EqualFold(a.Framework, "HTML") || strings.EqualFold(a.Framework, "ARIA"))) {
			out = append(out, a)
		}
	}
	return out
}
func componentRelated(items []namethatui.Component, refs []string) []any {
	out := make([]any, 0, len(refs))
	for _, ref := range refs {
		c, candidates := componentResolve(items, ref)
		if c != nil {
			out = append(out, c)
		} else if len(candidates) == 1 {
			out = append(out, candidates[0])
		} else {
			out = append(out, map[string]any{"reference": ref})
		}
	}
	return out
}
func componentGuidance(items []namethatui.Component, c *namethatui.Component, framework string, meta componentMeta) map[string]any {
	return map[string]any{"name": c.Name, "platform": c.Platform, "slug": c.Slug, "description": c.Description, "tagline": c.Tagline, "parts": nonNilParts(c.Parts), "api": componentAPIs(c.API, framework), "prompt": c.Prompt, "debug_prompt": c.DebugPrompt, "related": componentRelated(items, c.Related), "source_url": c.SourceURL, "meta": meta}
}

func newComponentCompareCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return &cobra.Command{Use: "compare <left> <right>", Short: "Compare two components", Example: "  name-that-ui-pp-cli component compare web/combobox web/select --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return componentDryRun(cmd, flags, "component.compare", args)
		}
		if len(args) != 2 {
			return usageErr(fmt.Errorf("%s requires <left> <right>", cmd.CommandPath()))
		}
		items, meta, err := componentLoad(cmd, componentDBPath(*dbPath))
		if err != nil {
			return err
		}
		left, leftCandidates := componentResolve(items, args[0])
		right, rightCandidates := componentResolve(items, args[1])
		if left == nil || right == nil {
			return componentPrint(cmd, flags, map[string]any{"left_candidates": nonNilComponents(leftCandidates), "right_candidates": nonNilComponents(rightCandidates), "meta": meta})
		}
		return componentPrint(cmd, flags, map[string]any{"left": left, "right": right, "differences": componentDifferences(left, right), "meta": meta})
	}}
}

func componentDifferences(left, right *namethatui.Component) map[string]any {
	d := map[string]any{}
	if left.Platform != right.Platform {
		d["platform"] = []string{left.Platform, right.Platform}
	}
	if left.Tagline != right.Tagline {
		d["tagline"] = []string{left.Tagline, right.Tagline}
	}
	if left.Description != right.Description {
		d["description"] = []string{left.Description, right.Description}
	}
	if left.Prompt != right.Prompt {
		d["prompt"] = true
	}
	if len(left.Parts) != len(right.Parts) {
		d["parts_count"] = []int{len(left.Parts), len(right.Parts)}
	}
	return d
}

func componentPrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	return componentPrintWithAgentSource(cmd, flags, value, "local")
}

// componentPrintWithAgentSource keeps local mirror commands' existing
// envelope while allowing identify's live semantic response to report its
// actual source to agent callers.
func componentPrintWithAgentSource(cmd *cobra.Command, flags *rootFlags, value any, source string) error {
	if wantsMachineOutput(flags) {
		if flags.agent {
			return printJSONFilteredWithAgentMeta(cmd.OutOrStdout(), value, flags, map[string]any{"source": source})
		}
		return printJSONFiltered(cmd.OutOrStdout(), value, flags)
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return err
}
