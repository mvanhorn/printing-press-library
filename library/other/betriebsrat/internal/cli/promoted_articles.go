// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type legalArticle struct {
	Paragraph  string `json:"paragraph"`
	Law        string `json:"law"`
	Title      string `json:"title"`
	SourceURL  string `json:"source_url"`
}

var topicArticles = map[string][]legalArticle{
	"kuendigung": {
		{Paragraph: "§ 102", Law: "BetrVG", Title: "Anhörung bei Kündigung", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__102.html"},
		{Paragraph: "§ 103", Law: "BetrVG", Title: "Außerordentliche Kündigung und Versetzung von Betriebsratsmitgliedern", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__103.html"},
	},
	"betriebsvereinbarungen": {
		{Paragraph: "§ 77", Law: "BetrVG", Title: "Durchführung gemeinsamer Beschlüsse, Betriebsvereinbarungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__77.html"},
		{Paragraph: "§ 88", Law: "BetrVG", Title: "Freiwillige Betriebsvereinbarungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__88.html"},
	},
	"bv": {
		{Paragraph: "§ 77", Law: "BetrVG", Title: "Durchführung gemeinsamer Beschlüsse, Betriebsvereinbarungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__77.html"},
		{Paragraph: "§ 88", Law: "BetrVG", Title: "Freiwillige Betriebsvereinbarungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__88.html"},
	},
	"homeoffice": {
		{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte – Nr. 2 Beginn/Ende Arbeitszeit, Nr. 14 mobiles Arbeiten", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	},
	"mobile-arbeit": {
		{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte – Nr. 2 Beginn/Ende Arbeitszeit, Nr. 14 mobiles Arbeiten", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	},
	"ki": {
		{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte – Nr. 6 Überwachung durch technische Einrichtungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	},
	"software": {
		{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte – Nr. 6 Überwachung durch technische Einrichtungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	},
	"digitalisierung": {
		{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte – Nr. 6 Überwachung durch technische Einrichtungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	},
	"betriebsaenderung": {
		{Paragraph: "§ 111", Law: "BetrVG", Title: "Betriebsänderungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__111.html"},
		{Paragraph: "§ 112", Law: "BetrVG", Title: "Interessenausgleich und Sozialplan", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__112.html"},
		{Paragraph: "§ 113", Law: "BetrVG", Title: "Nachteilsausgleich", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__113.html"},
	},
	"sozialplan": {
		{Paragraph: "§ 111", Law: "BetrVG", Title: "Betriebsänderungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__111.html"},
		{Paragraph: "§ 112", Law: "BetrVG", Title: "Interessenausgleich und Sozialplan", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__112.html"},
		{Paragraph: "§ 113", Law: "BetrVG", Title: "Nachteilsausgleich", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__113.html"},
	},
	"einstellung": {
		{Paragraph: "§ 99", Law: "BetrVG", Title: "Mitbestimmung bei personellen Einzelmaßnahmen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__99.html"},
		{Paragraph: "§ 100", Law: "BetrVG", Title: "Vorläufige personelle Maßnahmen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__100.html"},
		{Paragraph: "§ 101", Law: "BetrVG", Title: "Zwangsgeld", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__101.html"},
	},
	"versetzung": {
		{Paragraph: "§ 99", Law: "BetrVG", Title: "Mitbestimmung bei personellen Einzelmaßnahmen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__99.html"},
		{Paragraph: "§ 100", Law: "BetrVG", Title: "Vorläufige personelle Maßnahmen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__100.html"},
		{Paragraph: "§ 101", Law: "BetrVG", Title: "Zwangsgeld", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__101.html"},
	},
	"arbeitszeit": {
		{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte – Nr. 3 vorübergehende Verkürzung/Verlängerung der Arbeitszeit", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	},
	"ueberstunden": {
		{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte – Nr. 3 vorübergehende Verkürzung/Verlängerung der Arbeitszeit", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	},
	"betriebsratswahl": {
		{Paragraph: "§ 1", Law: "BetrVG", Title: "Errichtung von Betriebsräten", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__1.html"},
		{Paragraph: "§ 9", Law: "BetrVG", Title: "Größe des Betriebsrats", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__9.html"},
		{Paragraph: "§ 14", Law: "BetrVG", Title: "Wahlvorstand und Wahl", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__14.html"},
		{Paragraph: "§ 17", Law: "BetrVG", Title: "Wahl des Betriebsrats in besonderen Fällen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__17.html"},
	},
	"massenentlassung": {
		{Paragraph: "§ 17", Law: "KSchG", Title: "Anzeigepflichtige Entlassungen (Massenentlassung)", SourceURL: "https://www.gesetze-im-internet.de/kschg/__17.html"},
	},
	"betriebsubergang": {
		{Paragraph: "§ 613a", Law: "BGB", Title: "Rechte und Pflichten bei Betriebsübergang", SourceURL: "https://www.gesetze-im-internet.de/bgb/__613a.html"},
	},
}

var defaultArticles = []legalArticle{
	{Paragraph: "§ 1", Law: "BetrVG", Title: "Errichtung von Betriebsräten", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__1.html"},
	{Paragraph: "§ 77", Law: "BetrVG", Title: "Betriebsvereinbarungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__77.html"},
	{Paragraph: "§ 87", Law: "BetrVG", Title: "Mitbestimmungsrechte (Soziale Angelegenheiten)", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__87.html"},
	{Paragraph: "§ 99", Law: "BetrVG", Title: "Mitbestimmung bei personellen Einzelmaßnahmen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__99.html"},
	{Paragraph: "§ 102", Law: "BetrVG", Title: "Mitbestimmung bei Kündigungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__102.html"},
	{Paragraph: "§ 111", Law: "BetrVG", Title: "Betriebsänderungen", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__111.html"},
	{Paragraph: "§ 112", Law: "BetrVG", Title: "Interessenausgleich und Sozialplan", SourceURL: "https://www.gesetze-im-internet.de/betrvg/__112.html"},
}

func newArticlesPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagTopic string

	cmd := &cobra.Command{
		Use:         "articles",
		Short:       "Search for articles within a topic area",
		Long:        "Returns relevant BetrVG paragraphs and official legal sources for a topic area.",
		Example:     "  betriebsrat articles --topic kuendigung",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("topic") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "topic")
			}
			if dryRunOK(flags) {
				return nil
			}

			slug := strings.ToLower(strings.TrimSpace(flagTopic))
			articles, ok := topicArticles[slug]
			if !ok {
				articles = defaultArticles
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(articles)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Thema: %s\n\n", flagTopic)
			for _, a := range articles {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s – %s\n  %s\n\n", a.Paragraph, a.Law, a.Title, a.SourceURL)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Quelle: gesetze-im-internet.de (Bundesministerium der Justiz)\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTopic, "topic", "", "Topic slug to search within")

	return cmd
}
