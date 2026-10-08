package dropbox

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// ExpandFolderTemplate replaces organizer tokens with file metadata.
func ExpandFolderTemplate(tmpl, name string, modified time.Time) (string, error) {
	if !strings.HasPrefix(tmpl, "/") || tmpl == "/" {
		return "", fmt.Errorf("destination template must be an absolute folder path")
	}
	ext := strings.TrimPrefix(path.Ext(name), ".")
	if ext == "" {
		ext = "none"
	}
	values := map[string]string{
		"year": modified.Format("2006"), "month": modified.Format("01"),
		"day": modified.Format("02"), "ext": strings.ToLower(ext),
	}
	result := tmpl
	for token, value := range values {
		result = strings.ReplaceAll(result, "{"+token+"}", value)
	}
	if strings.ContainsAny(result, "{}") || path.Clean(result) != result || strings.Contains(result, "//") {
		return "", fmt.Errorf("invalid destination template %q", tmpl)
	}
	return result, nil
}
