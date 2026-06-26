package packet

import "strings"

var sensitiveSections = map[string]bool{
	"headline":         true,
	"job_title":        true,
	"title":            true,
	"company":          true,
	"experience":       true,
	"role_description": true,
	"open_to_work":     true,
}

func IsSensitiveSection(section string) bool {
	key := strings.ToLower(strings.TrimSpace(section))
	key = strings.ReplaceAll(key, " ", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return sensitiveSections[key]
}

func SensitiveChanges(changes []Change) []Change {
	var out []Change
	for _, ch := range changes {
		if IsSensitiveSection(ch.Section) {
			out = append(out, ch)
		}
	}
	return out
}
