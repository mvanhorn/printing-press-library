package vault

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Severity classifies a Finding for filtering and exit-code policy.
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

// Finding is one violation detected by the validator.
type Finding struct {
	Path     string   `json:"path"`
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Field    string   `json:"field,omitempty"`
	Message  string   `json:"message"`
	Layer    string   `json:"layer,omitempty"`
}

// AllowedTypes is the closed enum from UCE/PROTOCOL-THREE-LAYER-MEMORY.md
// (also enforced by UCE/src/vault/frontmatter_parser.py).
var AllowedTypes = []string{
	"person", "company", "project",
	"meeting", "journal", "session", "decision",
	"idea", "framework", "research",
}

// AllowedStatuses is the closed enum for the status field.
var AllowedStatuses = []string{
	"active", "paused", "completed", "archived", "superseded",
}

// AllowedFactCategories enumerates the category values protocol facts may carry.
var AllowedFactCategories = []string{
	"relationship", "milestone", "status", "preference", "decision", "role",
}

// Layers groups types into the three protocol layers.
var Layers = map[string]string{
	"person":    "knowledge-graph",
	"company":   "knowledge-graph",
	"project":   "knowledge-graph",
	"meeting":   "events",
	"journal":   "events",
	"session":   "events",
	"decision":  "events",
	"idea":      "patterns",
	"framework": "patterns",
	"research":  "patterns",
}

// cmBlockingRules is the subset of rule IDs whose violations break the
// downstream cm extraction pipeline (per SourceType::ObsidianImport semantics).
// Used by the readiness audit to filter the full lint set.
var cmBlockingRules = map[string]bool{
	"missing-type":         true,
	"missing-date":         true,
	"missing-description":  true,
	"missing-status":       true,
	"invalid-type":         true,
	"invalid-status":       true,
	"bad-date-format":      true,
	"description-too-long": false, // long but parseable — cm extracts fine
}

// IsCMBlocking returns true if a rule blocks downstream cm extraction.
func IsCMBlocking(rule string) bool {
	return cmBlockingRules[rule]
}

// Validate runs the three-layer-memory protocol checks against one note's
// frontmatter. Returns findings sorted by severity (error -> warn -> info)
// then by rule ID for stable output.
func Validate(path string, fm Frontmatter, hasFM bool) []Finding {
	var fnd []Finding
	if !hasFM {
		fnd = append(fnd, Finding{
			Path:     path,
			Rule:     "no-frontmatter",
			Severity: SeverityError,
			Message:  "note has no YAML frontmatter block",
		})
		return fnd
	}
	add := func(rule, field, msg string, sev Severity) {
		layer := Layers[fm.Type]
		fnd = append(fnd, Finding{
			Path: path, Rule: rule, Severity: sev,
			Field: field, Message: msg, Layer: layer,
		})
	}

	// Required fields
	if fm.Type == "" {
		add("missing-type", "type", "required field `type` is missing", SeverityError)
	} else if !contains(AllowedTypes, fm.Type) {
		add("invalid-type", "type",
			fmt.Sprintf("type %q is not in the protocol enum (allowed: %s)", fm.Type, strings.Join(AllowedTypes, ", ")),
			SeverityError)
	}
	if fm.Date == "" {
		add("missing-date", "date", "required field `date` is missing", SeverityError)
	} else if !isISODate(fm.Date) {
		add("bad-date-format", "date",
			fmt.Sprintf("date %q must be ISO format YYYY-MM-DD", fm.Date),
			SeverityError)
	}
	if fm.Description == "" {
		add("missing-description", "description", "required field `description` is missing", SeverityError)
	} else if utf8.RuneCountInString(fm.Description) > 150 {
		add("description-too-long", "description",
			fmt.Sprintf("description is %d chars (protocol max 150)", utf8.RuneCountInString(fm.Description)),
			SeverityWarn)
	}
	if fm.Status == "" {
		add("missing-status", "status", "required field `status` is missing", SeverityError)
	} else if !contains(AllowedStatuses, fm.Status) {
		add("invalid-status", "status",
			fmt.Sprintf("status %q not in protocol enum (allowed: %s)", fm.Status, strings.Join(AllowedStatuses, ", ")),
			SeverityError)
	}

	// Cross-field rules
	if fm.Status == "superseded" && fm.SupersededBy == "" {
		add("superseded-missing-target", "superseded_by",
			"status=superseded requires superseded_by pointing to the replacement note",
			SeverityError)
	}

	// Type-specific rules
	switch fm.Type {
	case "person":
		if _, ok := fm.Extra["email"]; !ok {
			add("person-missing-email", "email",
				"person notes should declare an email (canonical entity ID for cm)",
				SeverityWarn)
		}
	case "meeting":
		if _, ok := fm.Extra["people"]; !ok {
			add("meeting-missing-people", "people",
				"meeting notes should declare a people list",
				SeverityWarn)
		}
	case "company":
		if _, ok := fm.Extra["relationship"]; !ok {
			add("company-missing-relationship", "relationship",
				"company notes should declare relationship (customer|partner|vendor|competitor)",
				SeverityInfo)
		}
	}

	// Fact-level rules
	for i, fact := range fm.Facts {
		fieldPrefix := fmt.Sprintf("facts[%d]", i)
		if fact.ID == "" {
			add("fact-missing-id", fieldPrefix+".id", "fact missing id", SeverityWarn)
		}
		if fact.Fact == "" {
			add("fact-missing-text", fieldPrefix+".fact", "fact missing fact text", SeverityError)
		}
		if fact.Category != "" && !contains(AllowedFactCategories, fact.Category) {
			add("fact-invalid-category", fieldPrefix+".category",
				fmt.Sprintf("category %q not in enum (allowed: %s)", fact.Category, strings.Join(AllowedFactCategories, ", ")),
				SeverityWarn)
		}
		if fact.Status != "" && fact.Status != "active" && fact.Status != "superseded" {
			add("fact-invalid-status", fieldPrefix+".status",
				fmt.Sprintf("fact status %q must be active|superseded", fact.Status),
				SeverityWarn)
		}
	}

	sort.SliceStable(fnd, func(i, j int) bool {
		if fnd[i].Severity != fnd[j].Severity {
			return severityRank(fnd[i].Severity) < severityRank(fnd[j].Severity)
		}
		return fnd[i].Rule < fnd[j].Rule
	})
	return fnd
}

// FilterBySeverity returns only findings >= minSev.
func FilterBySeverity(findings []Finding, minSev Severity) []Finding {
	min := severityRank(minSev)
	var out []Finding
	for _, f := range findings {
		if severityRank(f.Severity) <= min {
			out = append(out, f)
		}
	}
	return out
}

func severityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarn:
		return 1
	case SeverityInfo:
		return 2
	}
	return 99
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func isISODate(s string) bool {
	// Strict YYYY-MM-DD; tolerates an optional time suffix which is common in YAML.
	if len(s) < 10 {
		return false
	}
	candidate := s[:10]
	_, err := time.Parse("2006-01-02", candidate)
	return err == nil
}
