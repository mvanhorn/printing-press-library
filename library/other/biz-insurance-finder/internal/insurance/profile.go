package insurance

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DefaultProfileFileName is the conventional on-disk profile name. It is
// gitignored because it may contain PII.
const DefaultProfileFileName = "profile.json"

// DefaultProfile returns a profile pre-filled with the sensible defaults the
// intake interview offers, so a user mostly presses Enter. The effective date
// defaults to the next business working day relative to `now`.
func DefaultProfile(now time.Time) Profile {
	return Profile{
		EntityStructure:            "LLC",
		GLPerOccurrence:            "$1,000,000",
		GLAggregate:                "$2,000,000",
		ProductsCompletedOps:       true,
		TradeShowAdditionalInsured: false,
		CurrentCoverage:            "None - first-time buyer",
		EffectiveDate:              NextBusinessDay(now).Format("2006-01-02"),
	}
}

// NextBusinessDay returns the next Monday-Friday date strictly after `from`
// (weekends roll to Monday). Used as the default policy effective date.
func NextBusinessDay(from time.Time) time.Time {
	d := from.AddDate(0, 0, 1)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// LoadProfile reads and decodes a profile from disk.
func LoadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile %q: %w", path, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile %q: %w", path, err)
	}
	return p, nil
}

// SaveProfile writes the profile as indented JSON with owner-only permissions
// (0600) because it may contain PII.
func SaveProfile(path string, p Profile) error {
	p.SavedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write profile %q: %w", path, err)
	}
	return nil
}

// Validate returns a list of human-readable problems with the profile (missing
// fields that quote forms will require). An empty slice means the profile is
// complete enough to start filling forms. It never errors - it is advisory.
func (p Profile) Validate() []string {
	var problems []string
	req := func(ok bool, msg string) {
		if !ok {
			problems = append(problems, msg)
		}
	}
	req(p.LegalName != "", "legal entity name is missing")
	req(p.EntityStructure != "", "entity structure is missing")
	req(p.FormationState != "", "state of formation is missing")
	req(p.BusinessAddress != "", "business address is missing")
	req(p.ContactName != "", "contact / named insured is missing")
	req(p.ContactEmail != "", "contact email is missing")
	req(p.ContactPhone != "", "contact phone is missing")
	req(p.YearStarted != 0, "year business started is missing")
	req(p.RevenueBand != "", "revenue band is missing")
	req(p.IndustryClass != "", "industry / class is missing")
	req(p.GLPerOccurrence != "", "GL per-occurrence limit is missing")
	req(p.GLAggregate != "", "GL aggregate limit is missing")
	if p.IsImporterClass() {
		req(len(p.CountriesOfOrigin) > 0, "importer/private-label set but no country of origin given")
	}
	return problems
}
