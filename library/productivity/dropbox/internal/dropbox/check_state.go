package dropbox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// planState keeps the index immutable. Only paths changed by the plan enter the overlay.
type planState struct {
	base         map[string]SnapshotEntry
	paths        []string
	changes      map[string]*SnapshotEntry
	changedUnder map[string]map[string]struct{}
}

func newPlanState(base map[string]SnapshotEntry) *planState {
	s := &planState{base: base, paths: make([]string, 0, len(base)), changes: map[string]*SnapshotEntry{}, changedUnder: map[string]map[string]struct{}{}}
	for p := range base {
		s.paths = append(s.paths, checkPath(p))
	}
	sort.Strings(s.paths)
	return s
}
func (s *planState) get(p string) (SnapshotEntry, bool) {
	p = checkPath(p)
	if change, ok := s.changes[p]; ok {
		if change == nil {
			return SnapshotEntry{}, false
		}
		return *change, true
	}
	e, ok := s.base[p]
	return e, ok
}
func (s *planState) put(p string, e *SnapshotEntry) {
	p = checkPath(p)
	if _, exists := s.changes[p]; !exists {
		for parent := checkParent(p); parent != ""; parent = checkParent(parent) {
			if s.changedUnder[parent] == nil {
				s.changedUnder[parent] = map[string]struct{}{}
			}
			s.changedUnder[parent][p] = struct{}{}
		}
	}
	s.changes[p] = e
}
func (s *planState) eachDescendant(p string, visit func(string, SnapshotEntry)) {
	p = checkPath(p)
	lower, upper := p+"/", p+"0"
	for i := sort.SearchStrings(s.paths, lower); i < len(s.paths) && s.paths[i] < upper; i++ {
		q := s.paths[i]
		if e, ok := s.get(q); ok {
			visit(q, e)
		}
	}
	for q := range s.changedUnder[p] {
		if _, exists := s.base[q]; exists {
			continue
		}
		if e, ok := s.get(q); ok {
			visit(q, e)
		}
	}
}
func (s *planState) eachWithin(p string, visit func(string, SnapshotEntry)) {
	if e, ok := s.get(p); ok {
		visit(checkPath(p), e)
	}
	s.eachDescendant(p, visit)
}
func (s *planState) move(from, to string) {
	type pair struct {
		path  string
		entry SnapshotEntry
	}
	moved := make([]pair, 0)
	s.eachWithin(from, func(p string, e SnapshotEntry) { moved = append(moved, pair{p, e}) })
	for _, item := range moved {
		s.put(item.path, nil)
	}
	for _, item := range moved {
		e := item.entry
		s.put(checkPath(to)+strings.TrimPrefix(item.path, checkPath(from)), &e)
	}
}
func (s *planState) delete(p string) {
	paths := make([]string, 0)
	s.eachWithin(p, func(path string, _ SnapshotEntry) { paths = append(paths, path) })
	for _, path := range paths {
		s.put(path, nil)
	}
}
func (s *planState) treeHash(p string) string {
	type file struct {
		relative, hash string
		size           int64
	}
	files := make([]file, 0)
	s.eachDescendant(p, func(q string, e SnapshotEntry) {
		if e.Tag == "file" {
			files = append(files, file{strings.TrimPrefix(q, checkPath(p)+"/"), e.ContentHash, e.Size})
		}
	})
	sort.Slice(files, func(i, j int) bool { return files[i].relative < files[j].relative })
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s|%s|%d\n", f.relative, f.hash, f.size)
	}
	return hex.EncodeToString(h.Sum(nil))
}
