package cli

import "github.com/googlarz/betriebsrat/internal/betrvg"

// tr returns the German string when lang is "de" (or anything other than "en"),
// and the English string when lang is "en".
// Legal document bodies are always generated in German regardless of lang.
func tr(lang, de, en string) string {
	if lang == "en" {
		return en
	}
	return de
}

// coDetermTypeLabel translates a CoDeterminationType to the display language.
func coDetermTypeLabel(lang string, t betrvg.CoDeterminationType) string {
	switch t {
	case betrvg.MitbestimmungErzwingbar:
		return tr(lang, "Mitbestimmung (erzwingbar)", "Co-determination (enforceable)")
	case betrvg.Zustimmung:
		return tr(lang, "Zustimmungsvorbehalt", "Consent required")
	case betrvg.Mitwirkung:
		return tr(lang, "Mitwirkung", "Participation right")
	case betrvg.Beratung:
		return tr(lang, "Beratung", "Consultation right")
	case betrvg.Unterrichtung:
		return tr(lang, "Unterrichtung", "Information right")
	default:
		return tr(lang, "Keine", "None")
	}
}
