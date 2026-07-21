// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/spf13/cobra"
)

// pp:data-source local
// Context packs only assemble records already present in the local mirrors.
func newNovelContextPackCmd(flags *rootFlags) *cobra.Command {
	var flagComponent string
	var flagStyle string
	var flagFramework string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "context-pack",
		Short:       "Assemble one bounded, source-backed implementation packet for a component, optional style, and target framework.",
		Example:     "name-that-ui-pp-cli context-pack --component web/combobox --style glassmorphism --framework web --agent --select component.name,parts,apis,style_signals,cautions,source_urls",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return contextPackPrint(cmd, flags, map[string]any{
					"dry_run":       true,
					"operation":     "context-pack",
					"component":     flagComponent,
					"style":         flagStyle,
					"framework":     flagFramework,
					"data_source":   "local",
					"db_path":       componentDBPath(dbPath),
					"sqlite_opened": false,
				})
			}
			if strings.TrimSpace(flagComponent) == "" {
				return usageErr(fmt.Errorf("%s requires --component", cmd.CommandPath()))
			}

			components, componentMeta, err := componentLoad(cmd, componentDBPath(dbPath))
			if err != nil {
				return err
			}
			component, componentCandidates := componentResolve(components, flagComponent)
			if component == nil {
				return contextPackPrint(cmd, flags, map[string]any{
					"found":       false,
					"ambiguous":   len(componentCandidates) > 1,
					"candidates":  nonNilComponents(componentCandidates),
					"source_urls": []string{},
					"provenance":  map[string]any{"data_source": "local", "component": componentMeta},
				})
			}

			packet := contextPackPacket(components, component, flagFramework, componentMeta)
			if strings.TrimSpace(flagStyle) == "" {
				return contextPackPrint(cmd, flags, packet)
			}

			styles, styleMeta, err := styleLoad(cmd, styleDBPath(dbPath))
			if err != nil {
				return err
			}
			style, styleCandidates := styleResolve(styles, flagStyle)
			if style == nil {
				packet["style_found"] = false
				packet["style_ambiguous"] = len(styleCandidates) > 1
				packet["style_candidates"] = nonNilStyles(styleCandidates)
				return contextPackPrint(cmd, flags, packet)
			}

			packet["style_found"] = true
			packet["style"] = contextPackStyle(style)
			packet["style_signals"] = nonNilSignals(style.Signals)
			packet["code_sections"] = contextPackSections(style.Sections, styleCodeSection)
			packet["cautions"] = contextPackSections(style.Sections, styleCautionSection)
			packet["source_urls"] = contextPackSourceURLs(packet["source_urls"].([]string), style)
			packet["provenance"] = map[string]any{"data_source": "local", "component": componentMeta, "style": styleMeta}
			return contextPackPrint(cmd, flags, packet)
		},
	}
	cmd.Flags().StringVar(&flagComponent, "component", "", "Component ID, platform/slug, name, or known alias (required)")
	cmd.Flags().StringVar(&flagStyle, "style", "", "Optional locally synced visual-style slug or name")
	cmd.Flags().StringVar(&flagFramework, "framework", "", "Optional API framework filter")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func contextPackPacket(items []namethatui.Component, component *namethatui.Component, framework string, meta componentMeta) map[string]any {
	return map[string]any{
		"found":         true,
		"ambiguous":     false,
		"component":     contextPackComponent(component),
		"parts":         nonNilParts(component.Parts),
		"apis":          componentAPIs(component.API, framework),
		"prompt":        component.Prompt,
		"debug_prompt":  component.DebugPrompt,
		"related":       componentRelated(items, component.Related),
		"style_signals": []namethatui.Signal{},
		"code_sections": []namethatui.Section{},
		"cautions":      []namethatui.Section{},
		"source_urls":   contextPackSourceURLs(nil, component),
		"provenance":    map[string]any{"data_source": "local", "component": meta},
	}
}

func contextPackComponent(component *namethatui.Component) map[string]any {
	return map[string]any{
		"id":          component.ID,
		"platform":    component.Platform,
		"slug":        component.Slug,
		"name":        component.Name,
		"tagline":     component.Tagline,
		"description": component.Description,
		"aliases":     nonNilStrings(component.AKA),
		"source_url":  component.SourceURL,
	}
}

func contextPackStyle(style *namethatui.Style) map[string]string {
	return map[string]string{"id": style.ID, "slug": style.Slug, "name": style.Name, "source_url": style.SourceURL}
}

func contextPackSections(sections []namethatui.Section, matches func(string) bool) []namethatui.Section {
	out := make([]namethatui.Section, 0)
	for _, section := range sections {
		if matches(section.Heading) {
			out = append(out, section)
		}
	}
	return out
}

func contextPackSourceURLs(existing []string, record any) []string {
	urls := append([]string{}, existing...)
	switch value := record.(type) {
	case *namethatui.Component:
		urls = append(urls, value.SourceURL)
	case *namethatui.Style:
		urls = append(urls, value.SourceURL)
		for _, section := range value.Sections {
			urls = append(urls, section.SourceURL)
		}
	}
	seen := make(map[string]struct{}, len(urls))
	result := make([]string, 0, len(urls))
	for _, url := range urls {
		if url = strings.TrimSpace(url); url != "" {
			if _, exists := seen[url]; !exists {
				seen[url] = struct{}{}
				result = append(result, url)
			}
		}
	}
	sort.Strings(result)
	return result
}

func contextPackPrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	return componentPrint(cmd, flags, value)
}
