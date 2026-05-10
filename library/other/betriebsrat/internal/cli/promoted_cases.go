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
		Topic:     "Massenentlassung",
		Summary:   "Kündigungen ohne vorherige Massenentlassungsanzeige nach § 17 KSchG sind unwirksam. Die Anzeigepflicht ist zwingendes Recht.",
		Source:    "rechtsprechung-im-internet.de",
		SearchURL: "https://www.bundesarbeitsgericht.de/entscheidungen/",
	},
	{
		Date:      "22.04.2010",
		Court:     "BAG",
		Reference: "2 AZR 991/08",
		Topic:     "Anhörung bei Kündigung (§ 102 BetrVG)",
		Summary:   "Eine unvollständige Anhörung des Betriebsrats macht die Kündigung unwirksam. Der Arbeitgeber muss alle für die Entscheidung maßgeblichen Umstände mitteilen.",
		Source:    "rechtsprechung-im-internet.de",
		SearchURL: "https://www.bundesarbeitsgericht.de/entscheidungen/",
	},
	{
		Date:      "08.06.2004",
		Court:     "BAG",
		Reference: "1 ABR 4/03",
		Topic:     "Mitbestimmung bei technischer Überwachung (§ 87 Nr. 6 BetrVG)",
		Summary:   "Technische Einrichtungen, die zur Überwachung des Verhaltens oder der Leistung geeignet sind, unterliegen der erzwingbaren Mitbestimmung des Betriebsrats.",
		Source:    "rechtsprechung-im-internet.de",
		SearchURL: "https://www.bundesarbeitsgericht.de/entscheidungen/",
	},
	{
		Date:      "26.10.2021",
		Court:     "BAG",
		Reference: "1 AZR 253/20",
		Topic:     "Sozialplan (§ 112 BetrVG)",
		Summary:   "Der Sozialplan ist erzwingbar. Die Einigungsstelle kann bei fehlender Einigung einen verbindlichen Sozialplan aufstellen.",
		Source:    "rechtsprechung-im-internet.de",
		SearchURL: "https://www.bundesarbeitsgericht.de/entscheidungen/",
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
				fmt.Fprintf(cmd.OutOrStdout(), "BAG %s – %s\nThema: %s\n%s\n\n", d.Date, d.Reference, d.Topic, d.Summary)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Volltext-Suche:\n  %s\n  https://www.rechtsprechung-im-internet.de\n",
				landmarkDecisions[0].SearchURL)
			return nil
		},
	}

	return cmd
}
