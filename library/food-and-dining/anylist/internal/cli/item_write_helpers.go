package cli

import (
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func normalizedUPC(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// normalizedName provides deterministic exact matching for item names.
func normalizedName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func normalizedPackageSize(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func packageSizeText(size *pb.PBItemPackageSize) string {
	if size == nil {
		return ""
	}
	if raw := strings.TrimSpace(size.GetRawPackageSize()); raw != "" {
		return raw
	}
	return strings.TrimSpace(strings.Join([]string{size.GetSize(), size.GetUnit(), size.GetPackageType()}, " "))
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
