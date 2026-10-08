package dropbox

import (
	"fmt"
	"strings"
)

// PathKey is the case-insensitive index key for a validated Dropbox path.
func PathKey(p string) string { return strings.ToLower(p) }

func ValidateCanonicalPath(p string) error {
	if p == "" || !strings.HasPrefix(p, "/") || p == "/" || strings.HasSuffix(p, "/") {
		return fmt.Errorf("path %q is not canonical: use a leading slash and no trailing slash", p)
	}
	for _, segment := range strings.Split(p[1:], "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("path %q is not canonical: empty, . and .. segments are forbidden", p)
		}
	}
	return nil
}

// PathWithin compares Dropbox paths at segment boundaries.
func PathWithin(p, parent string) bool {
	p = strings.ToLower(strings.TrimRight(p, "/"))
	parent = strings.ToLower(strings.TrimRight(parent, "/"))
	return parent == "" || p == parent || strings.HasPrefix(p, parent+"/")
}

func ParentBase(p string) (parent, base string) {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "", p
	}
	if i == 0 {
		return "", p[1:]
	}
	return p[:i], p[i+1:]
}

// IndexRoot is the top-level folder containing a path, or empty at the root.
func IndexRoot(p string) string {
	p = strings.ToLower(strings.TrimRight(p, "/"))
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	if i := strings.IndexByte(p[1:], '/'); i >= 0 {
		return p[:i+1]
	}
	return ""
}

func EscapeLike(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `%`, `\%`)
	return strings.ReplaceAll(v, `_`, `\_`)
}
