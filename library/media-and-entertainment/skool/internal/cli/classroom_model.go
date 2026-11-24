// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Skool's classroom is a three-level tree: a community holds courses, a course
// holds folders (the UI calls them modules or sets), and a folder holds pages
// (lessons). These are the normalized kinds; the wire payloads use different
// vocabulary at each level and across page versions, which is why the
// extractors below are tolerant rather than struct-shaped.
const (
	classroomKindCourse = "course"
	classroomKindFolder = "folder"
	classroomKindPage   = "page"
)

// classroomNode is one node of the normalized classroom tree.
type classroomNode struct {
	ID       string          `json:"id"`
	Slug     string          `json:"slug,omitempty"`
	Title    string          `json:"title"`
	Kind     string          `json:"kind"`
	Order    int             `json:"order"`
	Access   string          `json:"access,omitempty"`
	Children []classroomNode `json:"children,omitempty"`
}

// titleKey normalizes a title for matching. Skool titles are author-entered, so
// a local markdown bundle and the live course rarely agree on casing or inner
// whitespace even when they mean the same lesson. Matching on this key is what
// makes `classroom push` idempotent — the same bundle pushed twice updates the
// same pages instead of creating duplicates.
func titleKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// courseTreeFromPayload normalizes a /_next/data/.../classroom/<slug>.json
// response into the three-level tree.
//
// The field path (pageProps.course.modules[].lessons[]) is the one shape proven
// by the shipped `classroom export` command, so it is read with a struct. A
// payload that does not carry it is reported as an error rather than returned
// as an empty tree — an empty tree would make `push` plan a create for every
// lesson that already exists.
func courseTreeFromPayload(raw json.RawMessage) (classroomNode, error) {
	var env struct {
		PageProps struct {
			Course struct {
				ID       string         `json:"id"`
				Name     string         `json:"name"`
				Metadata map[string]any `json:"metadata"`
				Modules  []struct {
					ID       string         `json:"id"`
					Name     string         `json:"name"`
					Metadata map[string]any `json:"metadata"`
					Lessons  []struct {
						ID       string         `json:"id"`
						Name     string         `json:"name"`
						Metadata map[string]any `json:"metadata"`
					} `json:"lessons"`
				} `json:"modules"`
			} `json:"course"`
		} `json:"pageProps"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return classroomNode{}, fmt.Errorf("parsing course response: %w", err)
	}
	src := env.PageProps.Course
	if src.ID == "" && src.Name == "" {
		return classroomNode{}, fmt.Errorf("no course found at pageProps.course — the community or course slug may be wrong, or Skool's page shape changed (re-run with --json to inspect the raw payload)")
	}

	root := classroomNode{
		ID:     src.ID,
		Title:  src.Name,
		Kind:   classroomKindCourse,
		Slug:   metaString(src.Metadata, "slug"),
		Order:  metaOrder(src.Metadata, 0),
		Access: accessFromMetadata(src.Metadata),
	}
	if root.Slug == "" {
		root.Slug = slugify(src.Name)
	}
	for mi, m := range src.Modules {
		folder := classroomNode{
			ID:     m.ID,
			Title:  m.Name,
			Kind:   classroomKindFolder,
			Order:  metaOrder(m.Metadata, mi+1),
			Access: accessFromMetadata(m.Metadata),
		}
		for li, l := range m.Lessons {
			folder.Children = append(folder.Children, classroomNode{
				ID:     l.ID,
				Title:  l.Name,
				Kind:   classroomKindPage,
				Order:  metaOrder(l.Metadata, li+1),
				Access: accessFromMetadata(l.Metadata),
			})
		}
		root.Children = append(root.Children, folder)
	}
	return root, nil
}

// courseSummariesFromPayload pulls the course list out of a
// /_next/data/.../classroom.json response.
//
// Unlike the single-course payload there is no field path proven by shipped
// code here, and Skool has moved this list between keys across page versions.
// So this walks the payload for the first array of objects that all carry a
// string id and a string name and that hangs off a key naming courses. A
// regen must re-confirm the real path rather than trusting this heuristic.
func courseSummariesFromPayload(raw json.RawMessage) ([]classroomNode, error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parsing classroom response: %w", err)
	}
	found := findCourseArray(root, "")
	if found == nil {
		// No populated course array anywhere. A community whose admin has not
		// created a course yet answers with an empty list, and that is a valid
		// classroom — exactly the state someone reaches for `push` in — so it
		// is accepted, but only at the one place a course list can sit.
		if hasEmptyCourseList(root) {
			return []classroomNode{}, nil
		}
		return nil, fmt.Errorf("could not locate the course list in the classroom payload — Skool's page shape may have changed (re-run with --json to inspect the raw payload)")
	}
	out := make([]classroomNode, 0, len(found))
	for i, item := range found {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		node := classroomNode{
			ID:     stringField(obj, "id"),
			Title:  stringField(obj, "name"),
			Kind:   classroomKindCourse,
			Access: accessFromObject(obj),
		}
		node.Slug = stringField(obj, "slug")
		if node.Slug == "" {
			if meta, ok := obj["metadata"].(map[string]any); ok {
				node.Slug = stringField(meta, "slug")
			}
		}
		if node.Slug == "" {
			node.Slug = slugify(node.Title)
		}
		node.Order = objectOrder(obj, i+1)
		out = append(out, node)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("course list in the classroom payload held no readable entries")
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out, nil
}

// findCourseArray walks v looking for an array of course-shaped objects. key is
// the map key v was reached through, so the search can prefer a "courses" array
// over an unrelated array of id/name objects elsewhere in the page props.
func findCourseArray(v any, key string) []any {
	switch t := v.(type) {
	case []any:
		if strings.Contains(strings.ToLower(key), "course") && allCourseShaped(t) {
			return t
		}
		for _, item := range t {
			if got := findCourseArray(item, key); got != nil {
				return got
			}
		}
	case map[string]any:
		// Deterministic order: Go randomizes map iteration, and without
		// sorting the same payload can resolve to different arrays run to run.
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Prefer a course-named key at this level before descending.
		for _, k := range keys {
			if arr, ok := t[k].([]any); ok && strings.Contains(strings.ToLower(k), "course") && allCourseShaped(arr) {
				return arr
			}
		}
		for _, k := range keys {
			if got := findCourseArray(t[k], k); got != nil {
				return got
			}
		}
	}
	return nil
}

// emptyCourseListKeys are the keys under pageProps whose empty array is read as
// "this community has no courses yet". The populated list is found by shape
// under any course-named key; an empty array has no shape to check, so it is
// trusted only under these exact names.
var emptyCourseListKeys = []string{"courses", "allCourses"}

// hasEmptyCourseList reports whether the payload carries an empty course list
// directly under pageProps. It is deliberately narrower than findCourseArray:
// an empty array under some other course-flavoured key (courseTags,
// courseProgress), or anywhere outside pageProps, says nothing about whether
// the community has courses, and neither does the client's dry-run placeholder.
func hasEmptyCourseList(root any) bool {
	top, ok := root.(map[string]any)
	if !ok {
		return false
	}
	props, ok := top["pageProps"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range emptyCourseListKeys {
		if arr, ok := props[key].([]any); ok && len(arr) == 0 {
			return true
		}
	}
	return false
}

// allCourseShaped reports whether every element is an object with a non-empty
// string id and name. An empty array is not course-shaped: accepting one here
// would let `push` conclude the community has no courses when it simply read
// the wrong key. The legitimately empty list is handled by hasEmptyCourseList.
func allCourseShaped(arr []any) bool {
	if len(arr) == 0 {
		return false
	}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			return false
		}
		if stringField(obj, "id") == "" || stringField(obj, "name") == "" {
			return false
		}
	}
	return true
}

func stringField(obj map[string]any, key string) string {
	if v, ok := obj[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func metaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	return stringField(meta, key)
}

// accessFromMetadata reads whichever access field the payload carries. Skool
// spells this differently per level and per page version, so every known
// spelling is tried before giving up.
func accessFromMetadata(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	return accessFromObject(meta)
}

func accessFromObject(obj map[string]any) string {
	for _, key := range []string{"access", "accessLevel", "access_level", "visibility"} {
		if v := stringField(obj, key); v != "" {
			return v
		}
	}
	if meta, ok := obj["metadata"].(map[string]any); ok {
		for _, key := range []string{"access", "accessLevel", "access_level", "visibility"} {
			if v := stringField(meta, key); v != "" {
				return v
			}
		}
	}
	return ""
}

func metaOrder(meta map[string]any, fallback int) int {
	if meta == nil {
		return fallback
	}
	return objectOrder(meta, fallback)
}

func objectOrder(obj map[string]any, fallback int) int {
	for _, key := range []string{"order", "position", "sortOrder", "sort_order"} {
		switch v := obj[key].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	if meta, ok := obj["metadata"].(map[string]any); ok {
		for _, key := range []string{"order", "position", "sortOrder", "sort_order"} {
			switch v := meta[key].(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
		}
	}
	return fallback
}

// flattenPages returns every page in the tree keyed by titleKey, so a push plan
// can decide create-vs-update without caring which folder holds the page.
// Skool lets an author move a lesson between folders without renaming it, and
// treating a moved lesson as a new one is how duplicates get created.
//
// A key maps to every live page carrying that title, in tree order. Skool does
// not stop two pages in one course sharing a title, and keeping only the first
// would let the plan pick an update target by accident and drop the other page
// from the orphan report.
func flattenPages(root classroomNode) map[string][]classroomNode {
	out := map[string][]classroomNode{}
	for _, folder := range root.Children {
		for _, page := range folder.Children {
			if page.Title == "" {
				continue
			}
			key := titleKey(page.Title)
			out[key] = append(out[key], page)
		}
	}
	return out
}

// indexCoursesByTitle groups the live courses by titleKey, keeping every course
// that shares a title for the same reason flattenPages does.
func indexCoursesByTitle(courses []classroomNode) map[string][]classroomNode {
	out := map[string][]classroomNode{}
	for _, course := range courses {
		key := titleKey(course.Title)
		out[key] = append(out[key], course)
	}
	return out
}

// nodeIDs lists node ids for an error message, so the user can tell the
// same-titled items apart in the classroom.
func nodeIDs(nodes []classroomNode) string {
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return strings.Join(ids, ", ")
}

// assertClassroomScope fails closed on any request path outside the classroom
// surface. The CLI can reach member moderation, billing-adjacent group
// settings, and DM endpoints with the same cookie, and a classroom bulk tool
// has no business touching them — so every classroom write path is checked
// against this allow-list before it is sent, rather than relying on the
// caller to have built the right URL.
func assertClassroomScope(path string) error {
	trimmed := path
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(trimmed, prefix) {
			if idx := strings.Index(trimmed[len(prefix):], "/"); idx >= 0 {
				trimmed = trimmed[len(prefix)+idx:]
			}
		}
	}
	allowed := []string{"/courses", "/classroom"}
	for _, prefix := range allowed {
		if trimmed == prefix || strings.HasPrefix(trimmed, prefix+"/") || strings.HasPrefix(trimmed, prefix+"?") {
			return nil
		}
	}
	return fmt.Errorf("refusing to send %s: classroom commands may only touch %s (members, billing, DMs, and group settings are out of scope)",
		path, strings.Join(allowed, " or "))
}

// resolveCommunity picks the community slug for a classroom command: the
// explicit flag wins, then the configured template var (SKOOL_COMMUNITY or
// config.toml), so no community slug is ever compiled into the CLI.
func resolveCommunity(flagValue string, templateVars map[string]string) (string, error) {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(templateVars["community"]); v != "" {
		return v, nil
	}
	return "", usageErr(fmt.Errorf("--community is required (or set SKOOL_COMMUNITY, or community under [template_vars] in the config file)"))
}
