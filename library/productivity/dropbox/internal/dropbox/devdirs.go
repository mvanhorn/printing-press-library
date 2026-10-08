package dropbox

import "strings"

// DevDirNames are directory segments that contain generated development files.
var DevDirNames = []string{"node_modules", ".git", ".venv", "venv", "site-packages", "__pycache__", "bower_components", ".pnpm-store", ".yarn", ".gradle", ".terraform", "Pods", "DerivedData"}

var devDirKinds = func() map[string]string {
	kinds := make(map[string]string, len(DevDirNames))
	for _, name := range DevDirNames {
		kinds[strings.ToLower(name)] = name
	}
	return kinds
}()

// DevDirKind returns the first exact development directory segment in a path.
func DevDirKind(pathLower string) (kind string, ok bool) {
	for _, segment := range strings.Split(pathLower, "/") {
		if kind, ok := devDirKinds[strings.ToLower(segment)]; ok {
			return kind, true
		}
	}
	return "", false
}
