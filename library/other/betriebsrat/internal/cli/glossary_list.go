// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type glossaryTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

var embeddedGlossary = []glossaryTerm{
	{Term: "Betriebsrat", Definition: "Die gewählte Interessenvertretung der Arbeitnehmer im Betrieb, geregelt im BetrVG. Wählbar ab 5 wahlberechtigten Arbeitnehmern (§ 1 BetrVG)."},
	{Term: "Betriebsvereinbarung", Definition: "Schriftliche Vereinbarung zwischen Arbeitgeber und Betriebsrat, die unmittelbar und zwingend für alle Arbeitnehmer gilt (§ 77 BetrVG)."},
	{Term: "Mitbestimmung", Definition: "Recht des Betriebsrats, Entscheidungen des Arbeitgebers zu beeinflussen oder zu blockieren. Erzwingbare Mitbestimmung: AG darf ohne BR-Zustimmung oder Einigungsstellenspruch nicht handeln (§ 87 BetrVG)."},
	{Term: "Einigungsstelle", Definition: "Innerbetriebliche Schlichtungsstelle aus Beisitzern beider Seiten und einem unparteiischen Vorsitzenden. Ihr Spruch ersetzt die fehlende Einigung bei erzwingbarer Mitbestimmung (§ 76 BetrVG)."},
	{Term: "Sozialplan", Definition: "Vereinbarung zwischen Arbeitgeber und BR über Ausgleich oder Milderung von wirtschaftlichen Nachteilen bei einer Betriebsänderung. Erzwingbar über die Einigungsstelle (§ 112 BetrVG)."},
	{Term: "Interessenausgleich", Definition: "Vereinbarung zwischen Arbeitgeber und BR über das Ob, Wann und Wie einer Betriebsänderung. Nicht erzwingbar, aber Grundlage für Nachteilsausgleich (§ 112 BetrVG)."},
	{Term: "Anhörung", Definition: "Pflicht des Arbeitgebers, den Betriebsrat vor jeder Kündigung zu hören. Ohne ordnungsgemäße Anhörung ist die Kündigung unwirksam (§ 102 BetrVG). Frist: 1 Woche (ordentlich), 3 Tage (außerordentlich)."},
	{Term: "Widerspruch", Definition: "Recht des Betriebsrats, einer Kündigung zu widersprechen (§ 102 Abs. 3 BetrVG). Widerspruchsgründe: fehlerhafte Sozialauswahl, Weiterbeschäftigungsmöglichkeit u.a. Wirksamer Widerspruch gibt AN Anspruch auf Weiterbeschäftigung."},
	{Term: "Zustimmungsvorbehalt", Definition: "Recht des Betriebsrats, einer Maßnahme (z.B. Einstellung, Versetzung) die Zustimmung zu verweigern. Ohne Zustimmung ist die Maßnahme unzulässig (§ 99 BetrVG). Frist: 1 Woche."},
	{Term: "Nachteilsausgleich", Definition: "Abfindungsanspruch der betroffenen Arbeitnehmer, wenn der Arbeitgeber ohne zwingenden Grund vom Interessenausgleich abweicht (§ 113 BetrVG)."},
	{Term: "Betriebsänderung", Definition: "Wesentliche Veränderung des Betriebs (z.B. Stilllegung, Verlagerung, Fusion, wesentliche Änderung der Betriebsorganisation). Auslöser für Unterrichtungs- und Beratungspflichten sowie Interessenausgleich/Sozialplan (§ 111 BetrVG)."},
	{Term: "Betriebsübergang", Definition: "Übergang eines Betriebs oder Betriebsteils auf einen anderen Inhaber. Arbeitsverhältnisse gehen automatisch über, Kündigung wegen Betriebsübergangs ist unzulässig (§ 613a BGB)."},
	{Term: "Sozialauswahl", Definition: "Beim betriebsbedingten Kündigung muss der Arbeitgeber soziale Kriterien beachten: Betriebszugehörigkeit, Lebensalter, Unterhaltspflichten, Schwerbehinderung (§ 1 Abs. 3 KSchG)."},
	{Term: "Tarifvorbehalt", Definition: "Betriebsvereinbarungen dürfen Regelungen, die üblicherweise durch Tarifvertrag getroffen werden, nicht ersetzen, wenn kein Tarifvertrag vorliegt (§ 77 Abs. 3 BetrVG)."},
	{Term: "Kündigungsschutz", Definition: "Schutz von Arbeitnehmern vor willkürlicher Kündigung. Allgemeiner Kündigungsschutz nach KSchG (ab 6 Monate Beschäftigung, Betrieb mit > 10 AN). Besonderer Schutz für BR-Mitglieder, Schwangere, Schwerbehinderte."},
}

func newGlossaryListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Browse legal terms glossary",
		Example:     "  betriebsrat glossary list",
		Annotations: map[string]string{"pp:endpoint": "glossary.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(embeddedGlossary)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "BetrVG-Glossar\n")
			for _, t := range embeddedGlossary {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n\n", t.Term, t.Definition)
			}
			return nil
		},
	}

	return cmd
}
