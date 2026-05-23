package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Vault is a rooted Obsidian vault on disk.
type Vault struct {
	Root string // absolute path to the vault root
}

// New returns a Vault, verifying the root exists and is a directory.
func New(root string) (*Vault, error) {
	if root == "" {
		return nil, fmt.Errorf("vault path is empty: set OBSIDIAN_VAULT_PATH or pass --vault")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("vault path %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("vault path %s is not a directory", root)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Vault{Root: abs}, nil
}

// Walk visits every `.md` note under the vault root, parsing frontmatter and
// invoking fn for each. Skips hidden directories (`.obsidian`, `.git`, etc.).
func (v *Vault) Walk(fn func(*Note) error) error {
	return filepath.WalkDir(v.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && path != v.Root {
				return filepath.SkipDir
			}
			// Skip Obsidian's trash and template dirs by convention.
			if name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		note, err := v.LoadNote(path)
		if err != nil {
			// Skip files that can't be read — vanished mid-walk (common with
			// iCloud Drive's on-demand sync), permission errors, etc. The
			// next sync picks them up if they reappear. Malformed-but-readable
			// files come back as a Note with FMError set, not as an error.
			return nil
		}
		return fn(note)
	})
}

// LoadNote reads and parses a single note from disk.
func (v *Vault) LoadNote(absPath string) (*Note, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(v.Root, absPath)
	if err != nil {
		rel = absPath
	}
	note := &Note{Path: rel, AbsPath: absPath}
	fmYAML, body, offset, found := SplitFrontmatter(content)
	if found {
		fm, err := ParseFrontmatter(fmYAML)
		if err != nil {
			// Malformed YAML in a real vault is common (hand-edited quotes,
			// stray tabs). Don't abort the whole walk: degrade to a no-FM
			// read so sync/lint can still index the file and surface the
			// parse error to the caller.
			note.Body = string(body)
			note.BodyOffset = offset
			note.FMError = fmt.Sprintf("frontmatter: %v", err)
			return note, nil
		}
		note.Frontmatter = fm
		note.HasFM = true
		note.Body = string(body)
		note.BodyOffset = offset
	} else {
		note.Body = string(content)
	}
	return note, nil
}

// ResolveAbs resolves a vault-relative or absolute path into an absolute path
// under the vault root. Rejects paths that escape the vault.
func (v *Vault) ResolveAbs(p string) (string, error) {
	if filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(v.Root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path %s escapes vault root %s", p, v.Root)
		}
		return abs, nil
	}
	candidate := filepath.Join(v.Root, p)
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(v.Root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %s escapes vault root %s", p, v.Root)
	}
	return abs, nil
}

// LoadFactsTOML reads a sidecar `_facts/<name>.toml` file when frontmatter
// declares facts_file. Returns nil + nil if the file does not exist (treated as no facts).
func (v *Vault) LoadFactsTOML(notePath string, factsFile string) ([]Fact, error) {
	if factsFile == "" {
		return nil, nil
	}
	noteDir := filepath.Dir(notePath)
	factsPath := filepath.Join(noteDir, factsFile)
	data, err := os.ReadFile(factsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var wrap struct {
		Facts []Fact `toml:"facts"`
	}
	if err := toml.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("parse facts toml %s: %w", factsPath, err)
	}
	return wrap.Facts, nil
}

// AtomicWrite writes data to absPath via a temp file + rename. Creates parent
// dirs if needed. mode is the file mode for the final file.
func AtomicWrite(absPath string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".obsidian-pp-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, absPath)
}

// wikilinkRe matches [[Target]] or [[Target|Alias]] or [[Target#section]] or
// [[Target#section|alias]] — the link target is capture group 1.
var wikilinkRe = regexp.MustCompile(`\[\[([^\[\]\|#]+)(?:#[^\[\]\|]+)?(?:\|[^\[\]]+)?\]\]`)

// ExtractWikilinks returns the unique link targets referenced in body.
func ExtractWikilinks(body string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		t := strings.TrimSpace(m[1])
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// tagInBodyRe matches inline `#tag` patterns (not headings — # at the start of
// a line followed by space is a heading, not a tag).
var tagInBodyRe = regexp.MustCompile(`(?m)(?:^|[^\w#])#([A-Za-z][A-Za-z0-9_\-/]*)`)

// ExtractInlineTags returns unique #tags appearing in body text.
func ExtractInlineTags(body string) []string {
	matches := tagInBodyRe.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		t := m[1]
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
