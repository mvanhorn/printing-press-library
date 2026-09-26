package tbprofile

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Folder is one mail folder of a server directory.
type Folder struct {
	Account   string    `json:"account"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	MboxPath  string    `json:"mbox_path"`
	Offline   bool      `json:"offline"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mtime"`
}

func skipFolderFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(name, ".") || lower == "popstate.dat" || lower == "msgfilterrules.dat" {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".msf", ".dat", ".json", ".sbd", ".html", ".sqlite", ".mozlz4", ".bak", ".tmp":
		return true
	}
	return false
}

// UnreadableSubtree is an X.sbd directory that could not be listed; Prefix is its folder path.
type UnreadableSubtree struct {
	Prefix string
	Err    error
}

// DirReader lists a directory; nil means os.ReadDir.
type DirReader func(name string) ([]os.DirEntry, error)

// DiscoverFolders walks a server directory: extension-less files are mbox
// folders, X.sbd holds subfolders, X.msf without mbox is a folder that is
// not stored for offline use.
func DiscoverFolders(serverDir, account string, readDir DirReader) ([]Folder, []UnreadableSubtree, error) {
	if readDir == nil {
		readDir = os.ReadDir
	}
	out := make([]Folder, 0)
	var skipped []UnreadableSubtree
	if serverDir == "" {
		return out, nil, nil
	}
	if _, err := os.Stat(serverDir); err != nil {
		if os.IsNotExist(err) {
			return out, nil, nil
		}
		return nil, nil, err
	}
	if err := walkFolderDir(readDir, serverDir, "", account, &out, &skipped); err != nil {
		return nil, nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, skipped, nil
}

func walkFolderDir(readDir DirReader, dir, prefix, account string, out *[]Folder, skipped *[]UnreadableSubtree) error {
	entries, err := readDir(dir)
	if err != nil {
		return err
	}
	files := map[string]os.DirEntry{}
	for _, e := range entries {
		files[e.Name()] = e
	}
	seen := map[string]bool{}
	add := func(name string, f Folder) {
		if seen[name] {
			return
		}
		seen[name] = true
		f.Account = account
		f.Name = name
		f.Path = joinFolder(prefix, name)
		*out = append(*out, f)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || skipFolderFile(name) || filepath.Ext(name) != "" && !knownMboxWithDot(name, files) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		add(name, Folder{MboxPath: filepath.Join(dir, name), Offline: true, SizeBytes: info.Size(), ModTime: info.ModTime()})
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.EqualFold(filepath.Ext(name), ".msf") {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if _, hasMbox := files[base]; hasMbox {
			continue
		}
		add(base, Folder{})
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !strings.EqualFold(filepath.Ext(name), ".sbd") {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		sub := joinFolder(prefix, base)
		if err := walkFolderDir(readDir, filepath.Join(dir, name), sub, account, out, skipped); err != nil {
			*skipped = append(*skipped, UnreadableSubtree{Prefix: sub, Err: err})
		}
	}
	return nil
}

// knownMboxWithDot keeps folder names containing a dot ("Re.Work") when a
// sibling .msf confirms the file is a mailbox.
func knownMboxWithDot(name string, files map[string]os.DirEntry) bool {
	_, ok := files[name+".msf"]
	return ok
}

func joinFolder(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}
