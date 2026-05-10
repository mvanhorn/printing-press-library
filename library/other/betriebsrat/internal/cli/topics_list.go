// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type topicEntry struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Paragraphs string `json:"paragraphs"`
	SourceURL  string `json:"source_url"`
}

var embeddedTopics = []topicEntry{
	{Name: "Anhörung bei Kündigung", Slug: "kuendigung", Paragraphs: "§§ 102, 103 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__102.html"},
	{Name: "Betriebsvereinbarungen", Slug: "betriebsvereinbarungen", Paragraphs: "§§ 77, 88 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__77.html"},
	{Name: "Homeoffice / Mobiles Arbeiten", Slug: "homeoffice", Paragraphs: "§ 87 Nr. 2, 14 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	{Name: "KI und Digitalisierung", Slug: "ki", Paragraphs: "§ 87 Nr. 6 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	{Name: "Betriebsänderung und Sozialplan", Slug: "betriebsaenderung", Paragraphs: "§§ 111, 112, 113 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__111.html"},
	{Name: "Einstellung und Versetzung", Slug: "einstellung", Paragraphs: "§§ 99, 100, 101 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__99.html"},
	{Name: "Arbeitszeit und Überstunden", Slug: "arbeitszeit", Paragraphs: "§ 87 Nr. 3 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	{Name: "Betriebsratswahl", Slug: "betriebsratswahl", Paragraphs: "§§ 1, 9, 14, 17 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__1.html"},
	{Name: "Massenentlassung", Slug: "massenentlassung", Paragraphs: "§ 17 KSchG", SourceURL: "https://www.gesetze-im-internet.de/kschg/__17.html"},
	{Name: "Betriebsübergang", Slug: "betriebsubergang", Paragraphs: "§ 613a BGB", SourceURL: "https://www.gesetze-im-internet.de/bgb/__613a.html"},
	{Name: "Allgemeine Aufgaben des Betriebsrats", Slug: "aufgaben", Paragraphs: "§§ 74, 80 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__80.html"},
	{Name: "Einigungsstelle", Slug: "einigungsstelle", Paragraphs: "§ 76 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__76.html"},
	{Name: "Freistellung von Betriebsratsmitgliedern", Slug: "freistellung", Paragraphs: "§§ 37, 38 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__37.html"},
	{Name: "Kosten des Betriebsrats", Slug: "kosten", Paragraphs: "§ 40 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__40.html"},
	{Name: "Personalplanung", Slug: "personalplanung", Paragraphs: "§§ 92, 93 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__92.html"},
	{Name: "Berufsbildung", Slug: "berufsbildung", Paragraphs: "§§ 96, 97, 98 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__96.html"},
	{Name: "Wirtschaftsausschuss", Slug: "wirtschaftsausschuss", Paragraphs: "§ 106 BetrVG", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__106.html"},
}

func newTopicsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all topic areas with relevant BetrVG paragraphs",
		Example:     "  betriebsrat topics list",
		Annotations: map[string]string{"pp:endpoint": "topics.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(embeddedTopics)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "BetrVG-Themenübersicht (Quelle: gesetze-im-internet.de)\n")
			for _, t := range embeddedTopics {
				fmt.Fprintf(cmd.OutOrStdout(), "%-35s %s\n  %s\n", t.Name, t.Paragraphs, t.SourceURL)
			}
			return nil
		},
	}

	return cmd
}
