// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type bagDecision struct {
	Date      string `json:"date"`
	Court     string `json:"court"`
	Reference string `json:"reference"`
	Topic     string `json:"topic"`
	Summary   string `json:"summary"`
	Source    string `json:"source"`
	SearchURL string `json:"search_url"`
}

var landmarkDecisions = []bagDecision{
	{
		Date:      "22.09.2016",
		Court:     "BAG",
		Reference: "2 AZR 276/16",
		Topic:     "Massenentlassung (§ 17 KSchG)",
		Summary:   "Kündigungen ohne vorherige Massenentlassungsanzeige nach § 17 KSchG sind unwirksam. Die Anzeigepflicht ist zwingendes Recht.",
		Source:    "dejure.org",
		SearchURL: "https://dejure.org/dienste/vernetzung/rechtsprechung?Gericht=BAG&Datum=22.09.2016&Aktenzeichen=2+AZR+276%2F16",
	},
	{
		Date:      "22.04.2010",
		Court:     "BAG",
		Reference: "2 AZR 991/08",
		Topic:     "Anhörung bei Kündigung (§ 102 BetrVG)",
		Summary:   "Eine unvollständige Anhörung des Betriebsrats macht die Kündigung unwirksam. Der Arbeitgeber muss alle für die Entscheidung maßgeblichen Umstände mitteilen.",
		Source:    "dejure.org",
		SearchURL: "https://dejure.org/dienste/vernetzung/rechtsprechung?Gericht=BAG&Datum=22.04.2010&Aktenzeichen=2+AZR+991%2F08",
	},
	{
		Date:      "26.01.2016",
		Court:     "BAG",
		Reference: "1 ABR 68/13",
		Topic:     "Mitbestimmung bei technischer Überwachung (§ 87 Nr. 6 BetrVG)",
		Summary:   "Technische Einrichtungen zur Verhaltens- oder Leistungsüberwachung unterliegen der erzwingbaren Mitbestimmung — auch GPS-basierte Systeme und digitale Tracking-Tools.",
		Source:    "dejure.org",
		SearchURL: "https://dejure.org/dienste/vernetzung/rechtsprechung?Gericht=BAG&Datum=26.01.2016&Aktenzeichen=1+ABR+68%2F13",
	},
	{
		Date:      "22.07.2004",
		Court:     "BAG",
		Reference: "8 AZR 394/03",
		Topic:     "Betriebsübergang (§ 613a BGB)",
		Summary:   "Beim Betriebsübergang gehen Arbeitsverhältnisse automatisch auf den neuen Inhaber über. Eine Kündigung wegen des Betriebsübergangs ist unwirksam.",
		Source:    "dejure.org",
		SearchURL: "https://dejure.org/dienste/vernetzung/rechtsprechung?Gericht=BAG&Datum=22.07.2004&Aktenzeichen=8+AZR+394%2F03",
	},
	{
		Date:      "11.11.2008",
		Court:     "BAG",
		Reference: "1 AZR 475/07",
		Topic:     "Sozialplan und Nachteilsausgleich (§§ 112, 113 BetrVG)",
		Summary:   "Der Betriebsrat kann einen Sozialplan über die Einigungsstelle erzwingen. Weicht der Arbeitgeber ohne zwingenden Grund vom Interessenausgleich ab, entsteht ein Nachteilsausgleichsanspruch.",
		Source:    "dejure.org",
		SearchURL: "https://dejure.org/dienste/vernetzung/rechtsprechung?Gericht=BAG&Datum=11.11.2008&Aktenzeichen=1+AZR+475%2F07",
	},
	{
		Date:      "25.01.2005",
		Court:     "BAG",
		Reference: "1 ABR 59/03",
		Topic:     "Zustimmungsverweigerung bei Einstellung (§ 99 BetrVG)",
		Summary:   "Der Betriebsrat kann die Zustimmung zu einer Einstellung nur aus den in § 99 Abs. 2 BetrVG abschließend geregelten Gründen verweigern. Die Verweigerung muss innerhalb einer Woche schriftlich erfolgen.",
		Source:    "dejure.org",
		SearchURL: "https://dejure.org/dienste/vernetzung/rechtsprechung?Gericht=BAG&Datum=25.01.2005&Aktenzeichen=1+ABR+59%2F03",
	},
}

func newCasesPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cases",
		Short:       "Fetch landmark court decisions relevant to works councils",
		Long:        "Returns an embedded list of landmark BAG decisions relevant to Betriebsrat practice.",
		Example:     "  betriebsrat cases",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(landmarkDecisions)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Wichtige BAG-Entscheidungen für Betriebsräte\n")
			for _, d := range landmarkDecisions {
				fmt.Fprintf(cmd.OutOrStdout(), "BAG %s – %s\nThema: %s\n%s\n  %s\n\n", d.Date, d.Reference, d.Topic, d.Summary, d.SearchURL)
			}
			return nil
		},
	}

	return cmd
}
