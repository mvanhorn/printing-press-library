package datacloud

const (
	UnifiedAccountDMO    = "UnifiedAccount__dlm"
	UnifiedIndividualDMO = "UnifiedIndividual__dlm"
)

type DMOMap map[string]string

func CandidateDMOs(entity string, m DMOMap) []string {
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	add(UnifiedAccountDMO)
	add(UnifiedIndividualDMO)
	if m != nil {
		add(m[entity])
	}
	return out
}
