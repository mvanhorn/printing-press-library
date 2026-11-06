// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

type courseRef struct {
	ID            int64  `yaml:"id"`
	MediaChecksum string `yaml:"media_checksum,omitempty"`
	Status        string `yaml:"status,omitempty"`
	BodySHA256    string `yaml:"body_sha256,omitempty"`
}
type courseManifest struct {
	SpaceID  int64                `yaml:"space_id"`
	Sections map[string]courseRef `yaml:"sections"`
	Lessons  map[string]courseRef `yaml:"lessons"`
	Order    []courseOrder        `yaml:"order"`
}

func emptyCourseManifest() courseManifest {
	return courseManifest{Sections: make(map[string]courseRef), Lessons: make(map[string]courseRef)}
}

type courseFrontmatter struct {
	ID     int64  `yaml:"id"`
	Name   string `yaml:"name"`
	Status string `yaml:"status,omitempty"`
}
type courseLocalLesson struct {
	Path, Name, Status, BodyHTML, MediaPath, Checksum string
	ID, Size                                          int64
}
type courseLocalSection struct {
	Path, Name string
	ID         int64
	Lessons    []courseLocalLesson
}
type courseFolder struct {
	Manifest courseManifest
	Sections []courseLocalSection
}
type courseSection struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type courseLesson struct {
	ID        int64   `json:"id"`
	SectionID int64   `json:"section_id"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	BodyHTML  *string `json:"body_html"`
}
type courseLive struct {
	Sections []courseSection
	Lessons  []courseLesson
}
type courseOrder struct {
	SectionID int64   `json:"section_id" yaml:"section_id"`
	LessonIDs []int64 `json:"lesson_ids" yaml:"lesson_ids"`
}
type courseStep struct {
	Action              string         `json:"action"`
	Kind                string         `json:"kind"`
	Path                string         `json:"path"`
	Name                string         `json:"name"`
	ID                  int64          `json:"id"`
	Changes             []courseChange `json:"changes,omitempty"`
	LiveChanged         bool           `json:"live_changed,omitempty"`
	LocalChanged        bool           `json:"local_changed,omitempty"`
	Synced              courseRef      `json:"-"`
	Body                map[string]any `json:"-"`
	MediaPath, Checksum string         `json:"-"`
	Size                int64          `json:"-"`
}
type courseChange struct {
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}
type coursePlan struct {
	Applied  bool           `json:"applied"`
	SpaceID  int64          `json:"space_id"`
	Plan     []courseStep   `json:"plan"`
	Summary  map[string]int `json:"summary"`
	Order    []courseOrder  `json:"order"`
	Force    bool           `json:"force"`
	Manifest courseManifest `json:"-"`
}

func (step courseStep) shouldApply(force bool) bool {
	return step.Action != "live-edited" && (force || (step.Action != "conflict" && (step.Action != "upload" || !step.LiveChanged)))
}

func courseSpaceID(value string, fallback int64) (int64, error) {
	id := fallback
	if value != "" {
		var err error
		id, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("--space must be a positive integer")
		}
	}
	if id <= 0 {
		return 0, fmt.Errorf("provide --space or a positive space_id in course.yaml")
	}
	return id, nil
}

func parseCourseMarkdown(data []byte) (courseFrontmatter, string, error) {
	var front courseFrontmatter
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[0] == "---" {
		end := 1
		for end < len(lines) && lines[end] != "---" {
			end++
		}
		if end == len(lines) {
			return front, "", fmt.Errorf("unterminated YAML frontmatter")
		}
		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &front); err != nil {
			return front, "", err
		}
		text = strings.Join(lines[end+1:], "\n")
	}
	if front.ID < 0 {
		return front, "", fmt.Errorf("frontmatter id must be positive")
	}
	if front.Status != "" && front.Status != "draft" && front.Status != "published" {
		return front, "", fmt.Errorf("status must be draft or published")
	}
	return front, text, nil
}

func coursePrefix(name string) (int, string) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) == 2 {
		if n, err := strconv.Atoi(parts[0]); err == nil && n >= 0 {
			return n, parts[1]
		}
	}
	return int(^uint(0) >> 1), name
}
func courseName(name string) string {
	_, name = coursePrefix(name)
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	for i, word := range words {
		r := []rune(word)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
func sortCourseEntries(entries []os.DirEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, _ := coursePrefix(entries[i].Name())
		b, _ := coursePrefix(entries[j].Name())
		if a != b {
			return a < b
		}
		return entries[i].Name() < entries[j].Name()
	})
}
func courseMediaChecksum(file string) (string, int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	h := md5.New()
	size, err := io.Copy(h, f)
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), size, err
}
func readCourseFolder(folder string) (courseFolder, error) {
	local := courseFolder{Manifest: emptyCourseManifest(), Sections: make([]courseLocalSection, 0)}
	data, err := os.ReadFile(filepath.Join(folder, "course.yaml"))
	if err == nil {
		err = yaml.Unmarshal(data, &local.Manifest)
	}
	if err != nil && !os.IsNotExist(err) {
		return local, err
	}
	if local.Manifest.Sections == nil {
		local.Manifest.Sections = make(map[string]courseRef)
	}
	if local.Manifest.Lessons == nil {
		local.Manifest.Lessons = make(map[string]courseRef)
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return local, err
	}
	sortCourseEntries(entries)
	md := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			sectionPath := filepath.Join(folder, entry.Name())
			info, err := os.Stat(sectionPath)
			if err != nil {
				return local, fmt.Errorf("%s: %w", sectionPath, err)
			}
			if info.IsDir() {
				return local, fmt.Errorf("section directory cannot be a symlink: %s", sectionPath)
			}
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		section := courseLocalSection{Path: entry.Name(), Name: courseName(entry.Name()), ID: local.Manifest.Sections[entry.Name()].ID, Lessons: make([]courseLocalLesson, 0)}
		sectionDir := filepath.Join(folder, entry.Name())
		data, err := os.ReadFile(filepath.Join(sectionDir, "_section.md"))
		if err == nil {
			front, _, err := parseCourseMarkdown(data)
			if err != nil {
				return local, fmt.Errorf("%s/_section.md: %w", entry.Name(), err)
			}
			if front.Name != "" {
				section.Name = front.Name
			}
			if section.ID == 0 {
				section.ID = front.ID
			}
		} else if !os.IsNotExist(err) {
			return local, err
		}
		lessons, err := os.ReadDir(sectionDir)
		if err != nil {
			return local, err
		}
		sortCourseEntries(lessons)
		for _, file := range lessons {
			if file.IsDir() || file.Name() == "_section.md" || strings.HasPrefix(file.Name(), ".") || filepath.Ext(file.Name()) != ".md" {
				continue
			}
			if file.Type()&os.ModeSymlink != 0 {
				return local, fmt.Errorf("lesson cannot be a symlink: %s", file.Name())
			}
			lessonPath := filepath.ToSlash(filepath.Join(section.Path, file.Name()))
			data, err := os.ReadFile(filepath.Join(sectionDir, file.Name()))
			if err != nil {
				return local, err
			}
			front, body, err := parseCourseMarkdown(data)
			if err != nil {
				return local, fmt.Errorf("%s: %w", lessonPath, err)
			}
			if !strings.HasPrefix(strings.TrimSpace(body), "<") {
				var rendered bytes.Buffer
				if err := md.Convert([]byte(body), &rendered); err != nil {
					return local, err
				}
				body = rendered.String()
			}
			lesson := courseLocalLesson{Path: lessonPath, Name: front.Name, Status: front.Status, ID: local.Manifest.Lessons[lessonPath].ID, BodyHTML: body}
			if lesson.ID == 0 {
				lesson.ID = front.ID
			}
			if lesson.Name == "" {
				lesson.Name = courseName(strings.TrimSuffix(file.Name(), ".md"))
			}
			for _, ext := range []string{".mp4", ".mov", ".m4v", ".mp3", ".png", ".jpg", ".jpeg", ".pdf"} {
				media := filepath.Join(sectionDir, strings.TrimSuffix(file.Name(), ".md")+ext)
				info, err := os.Lstat(media)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return local, err
				}
				if !info.Mode().IsRegular() {
					return local, fmt.Errorf("media must be a regular file: %s", media)
				}
				if lesson.MediaPath != "" {
					return local, fmt.Errorf("multiple media files for %s", lessonPath)
				}
				lesson.MediaPath = media
				lesson.Checksum, lesson.Size, err = courseMediaChecksum(media)
				if err != nil {
					return local, err
				}
			}
			section.Lessons = append(section.Lessons, lesson)
		}
		local.Sections = append(local.Sections, section)
	}
	return local, nil
}

func courseResponse(data json.RawMessage, out any) error {
	var failure struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &failure); err != nil {
		return err
	}
	if failure.Success != nil && !*failure.Success {
		return fmt.Errorf("circle: %s", failure.Message)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
func fetchCoursePages[T any](ctx context.Context, c *client.Client, endpoint string, spaceID int64) ([]T, error) {
	all := make([]T, 0)
	for page := 1; ; page++ {
		data, err := c.Get(ctx, endpoint, map[string]string{"space_id": strconv.FormatInt(spaceID, 10), "page": strconv.Itoa(page), "per_page": "100"})
		if err != nil {
			return nil, err
		}
		var result struct {
			Records []T  `json:"records"`
			Next    bool `json:"has_next_page"`
		}
		if err := courseResponse(data, &result); err != nil {
			return nil, err
		}
		if result.Records == nil {
			return nil, fmt.Errorf("%s: response missing records", endpoint)
		}
		all = append(all, result.Records...)
		if !result.Next {
			return all, nil
		}
		if len(result.Records) == 0 {
			return nil, fmt.Errorf("%s: empty page with has_next_page=true", endpoint)
		}
	}
}
func fetchCourse(ctx context.Context, c *client.Client, spaceID int64) (courseLive, error) {
	c.NoCache = true
	var live courseLive
	var err error
	live.Sections, err = fetchCoursePages[courseSection](ctx, c, "/course_sections", spaceID)
	if err != nil {
		return live, err
	}
	live.Lessons, err = fetchCoursePages[courseLesson](ctx, c, "/course_lessons", spaceID)
	if err != nil {
		return live, err
	}
	for i := range live.Lessons {
		lesson := &live.Lessons[i]
		if lesson.BodyHTML == nil {
			data, err := c.Get(ctx, fmt.Sprintf("/course_lessons/%d", lesson.ID), nil)
			if err != nil {
				return live, err
			}
			var detail courseLesson
			if err := courseResponse(data, &detail); err != nil {
				return live, err
			}
			if detail.ID != lesson.ID {
				return live, fmt.Errorf("lesson detail ID mismatch for %d", lesson.ID)
			}
			lesson.BodyHTML = detail.BodyHTML
			if lesson.BodyHTML == nil {
				empty := ""
				lesson.BodyHTML = &empty
			}
		}
	}
	return live, validateCourseLive(live)
}
func validateCourseLive(live courseLive) error {
	sections := make(map[int64]bool)
	lessons := make(map[int64]bool)
	for _, s := range live.Sections {
		if s.ID <= 0 || sections[s.ID] {
			return fmt.Errorf("invalid or duplicate section ID %d", s.ID)
		}
		sections[s.ID] = true
	}
	for _, l := range live.Lessons {
		if l.ID <= 0 || lessons[l.ID] || !sections[l.SectionID] {
			return fmt.Errorf("invalid, duplicate, or orphan lesson %d", l.ID)
		}
		lessons[l.ID] = true
	}
	return nil
}

var courseTagWhitespace = regexp.MustCompile(`>\s+<`)
var courseHeadingOne = regexp.MustCompile(`(?i)<(/?)h1(\s[^>]*)?>`)
var courseVerbatim = regexp.MustCompile(`(?is)<pre\b[^>]*>.*?</pre\s*>|<code\b[^>]*>.*?</code\s*>`)

func normalizeCourseBody(body string) string {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "<div>") && strings.HasSuffix(body, "</div>") {
		body = body[len("<div>") : len(body)-len("</div>")]
	}
	var normalized strings.Builder
	ordinary := func(text string, before, after bool) {
		// Include adjacent tag boundaries when collapsing ordinary whitespace.
		if before {
			text = ">" + text
		}
		if after {
			text += "<"
		}
		text = courseHeadingOne.ReplaceAllString(text, "<${1}h2${2}>")
		text = courseTagWhitespace.ReplaceAllString(text, "><")
		text = strings.Join(strings.Fields(text), " ")
		if before {
			text = strings.TrimPrefix(text, ">")
		}
		if after {
			text = strings.TrimSuffix(text, "<")
		}
		normalized.WriteString(text)
	}
	end := 0
	for _, span := range courseVerbatim.FindAllStringIndex(body, -1) {
		ordinary(body[end:span[0]], end > 0, true)
		normalized.WriteString(body[span[0]:span[1]])
		end = span[1]
	}
	ordinary(body[end:], end > 0, false)
	return strings.TrimSpace(normalized.String())
}

func courseBodySHA256(body string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(normalizeCourseBody(body))))
}

func courseLessonRef(manifest courseManifest, path string, id int64) courseRef {
	ref := manifest.Lessons[path]
	if ref.ID == id && id != 0 {
		return ref
	}
	for _, candidate := range manifest.Lessons {
		if candidate.ID == id && id != 0 {
			return candidate
		}
	}
	return courseRef{ID: id}
}

func recordCourseLessonSync(manifest courseManifest, id int64, synced courseRef) {
	for path, ref := range manifest.Lessons {
		if ref.ID == id {
			ref.Status, ref.BodySHA256 = synced.Status, synced.BodySHA256
			manifest.Lessons[path] = ref
		}
	}
}

func buildCoursePlan(local courseFolder, live courseLive, spaceID int64, publish bool) (coursePlan, error) {
	p := coursePlan{SpaceID: spaceID, Plan: make([]courseStep, 0), Summary: make(map[string]int), Order: make([]courseOrder, 0), Manifest: emptyCourseManifest()}
	p.Manifest.SpaceID = spaceID
	for k, v := range local.Manifest.Sections {
		p.Manifest.Sections[k] = v
	}
	for k, v := range local.Manifest.Lessons {
		p.Manifest.Lessons[k] = v
	}
	if err := validateCourseLive(live); err != nil {
		return p, err
	}
	usedSections := make(map[int64]bool)
	usedLessons := make(map[int64]bool)
	nextID := int64(-1)
	add := func(step courseStep) { p.Plan = append(p.Plan, step); p.Summary[step.Action]++ }
	for _, s := range local.Sections {
		var matched *courseSection
		for i := range live.Sections {
			candidate := &live.Sections[i]
			if (s.ID != 0 && candidate.ID == s.ID) || (s.ID == 0 && candidate.Name == s.Name) {
				if matched != nil {
					return p, fmt.Errorf("ambiguous section name %q; specify an id", s.Name)
				}
				matched = candidate
			}
		}
		id := s.ID
		if matched == nil {
			if id != 0 {
				return p, fmt.Errorf("section id %d from %s is absent from this course", id, s.Path)
			}
			id = nextID
			nextID--
			add(courseStep{Action: "create", Kind: "section", Path: s.Path, Name: s.Name, ID: id, Body: map[string]any{"name": s.Name, "space_id": spaceID}})
		} else {
			id = matched.ID
			if matched.Name != s.Name {
				add(courseStep{Action: "rename", Kind: "section", Path: s.Path, Name: s.Name, ID: id, Body: map[string]any{"name": s.Name}})
			}
		}
		if usedSections[id] {
			return p, fmt.Errorf("multiple folders match section %d", id)
		}
		usedSections[id] = true
		p.Manifest.Sections[s.Path] = courseRef{ID: id}
		order := courseOrder{SectionID: id, LessonIDs: make([]int64, 0)}
		for _, l := range s.Lessons {
			var matched *courseLesson
			for i := range live.Lessons {
				candidate := &live.Lessons[i]
				if (l.ID != 0 && candidate.ID == l.ID) || (l.ID == 0 && candidate.SectionID == id && candidate.Name == l.Name) {
					if matched != nil {
						return p, fmt.Errorf("ambiguous lesson name %q; specify an id", l.Name)
					}
					matched = candidate
				}
			}
			lessonID := l.ID
			refID := lessonID
			if matched != nil {
				refID = matched.ID
			}
			ref := courseLessonRef(local.Manifest, l.Path, refID)
			blocked := false
			liveOnly := false
			if matched == nil {
				if lessonID != 0 {
					return p, fmt.Errorf("lesson id %d from %s is absent from this course", lessonID, l.Path)
				}
				lessonID = nextID
				nextID--
				status := "draft"
				if l.Status == "published" || publish {
					status = "published"
				}
				add(courseStep{Action: "create", Kind: "lesson", Path: l.Path, Name: l.Name, ID: lessonID, Body: map[string]any{"section_id": id, "name": l.Name, "status": status, "body_html": l.BodyHTML}, Synced: courseRef{Status: status, BodySHA256: courseBodySHA256(l.BodyHTML)}})
			} else {
				lessonID = matched.ID
				body := make(map[string]any)
				changes := make([]courseChange, 0)
				if l.Name != matched.Name {
					body["name"] = l.Name
					changes = append(changes, courseChange{Field: "name", From: matched.Name, To: l.Name})
				}
				if l.Status != "" && l.Status != matched.Status {
					body["status"] = l.Status
					changes = append(changes, courseChange{Field: "status", From: matched.Status, To: l.Status})
				}
				liveBody := ""
				if matched.BodyHTML != nil {
					liveBody = *matched.BodyHTML
				}
				if normalizeCourseBody(l.BodyHTML) != normalizeCourseBody(liveBody) {
					body["body_html"] = l.BodyHTML
					changes = append(changes, courseChange{Field: "body_html"})
				}
				status := l.Status
				if status == "" {
					status = matched.Status
				}
				synced := courseRef{Status: status, BodySHA256: courseBodySHA256(l.BodyHTML)}
				liveChanged := ref.BodySHA256 != "" && (courseBodySHA256(liveBody) != ref.BodySHA256 || matched.Status != ref.Status)
				localChanged := ref.BodySHA256 != "" && (synced.BodySHA256 != ref.BodySHA256 || (l.Status != "" && l.Status != ref.Status))
				action := "update"
				if liveChanged && (len(body) > 0 || !localChanged) {
					action = "live-edited"
					if localChanged {
						action = "conflict"
					}
					blocked = true
					liveOnly = action == "live-edited"
				}
				if len(body) > 0 || liveOnly {
					add(courseStep{Action: action, Kind: "lesson", Path: l.Path, Name: l.Name, ID: lessonID, Body: body, Changes: changes, LiveChanged: liveChanged, LocalChanged: localChanged, Synced: synced})
				} else {
					p.Summary["noop"]++
					ref.Status, ref.BodySHA256 = synced.Status, synced.BodySHA256
					recordCourseLessonSync(p.Manifest, lessonID, synced)
				}
			}
			if usedLessons[lessonID] {
				return p, fmt.Errorf("multiple files match lesson %d", lessonID)
			}
			usedLessons[lessonID] = true
			ref.ID = lessonID
			for path, previous := range p.Manifest.Lessons {
				if previous.ID == lessonID && path != l.Path {
					delete(p.Manifest.Lessons, path)
				}
			}
			p.Manifest.Lessons[l.Path] = ref
			if !liveOnly && l.MediaPath != "" && l.Checksum != ref.MediaChecksum {
				add(courseStep{Action: "upload", Kind: "media", Path: l.Path, Name: l.Name, ID: lessonID, MediaPath: l.MediaPath, Checksum: l.Checksum, Size: l.Size, LiveChanged: blocked})
			}
			order.LessonIDs = append(order.LessonIDs, lessonID)
		}
		p.Order = append(p.Order, order)
	}
	p.Order = mergeCourseOrder(local.Manifest.Order, live, p.Order)
	// Circle lists creation order, so only the manifest can remember a successful reorder.
	if local.Manifest.Order == nil || p.Summary["create"] > 0 || !reflect.DeepEqual(local.Manifest.Order, p.Order) {
		add(courseStep{Action: "reorder", Kind: "course", Path: "", Name: "", ID: spaceID})
	}
	return p, nil
}

// Replace only managed slots; unmanaged items retain their positions and relative order.
func mergeCourseIDs(base, desired []int64) []int64 {
	managed := make(map[int64]bool)
	for _, id := range desired {
		managed[id] = true
	}
	result := make([]int64, 0, len(base)+len(desired))
	next := 0
	for _, id := range base {
		if managed[id] {
			result = append(result, desired[next])
			next++
		} else {
			result = append(result, id)
		}
	}
	return append(result, desired[next:]...)
}

func mergeCourseOrder(recorded []courseOrder, live courseLive, desired []courseOrder) []courseOrder {
	sections := make([]int64, 0)
	lessons := make(map[int64][]int64)
	for _, s := range live.Sections {
		sections = append(sections, s.ID)
		lessons[s.ID] = make([]int64, 0)
	}
	for _, l := range live.Lessons {
		lessons[l.SectionID] = append(lessons[l.SectionID], l.ID)
	}
	// Retain recorded order only for IDs still present in the live course.
	validRecorded := func(base, saved []int64) []int64 {
		present := make(map[int64]bool)
		for _, id := range base {
			present[id] = true
		}
		valid := make([]int64, 0)
		for _, id := range saved {
			if present[id] {
				valid = append(valid, id)
				delete(present, id)
			}
		}
		return mergeCourseIDs(base, valid)
	}
	savedSections := make([]int64, 0)
	for _, s := range recorded {
		savedSections = append(savedSections, s.SectionID)
		if base, ok := lessons[s.SectionID]; ok {
			lessons[s.SectionID] = validRecorded(base, s.LessonIDs)
		}
	}
	sections = validRecorded(sections, savedSections)
	managedSections := make([]int64, 0)
	targets := make(map[int64]int64)
	for _, s := range desired {
		managedSections = append(managedSections, s.SectionID)
		for _, id := range s.LessonIDs {
			targets[id] = s.SectionID
		}
	}
	for section, base := range lessons {
		kept := make([]int64, 0, len(base))
		for _, id := range base {
			if target, moved := targets[id]; !moved || target == section {
				kept = append(kept, id)
			}
		}
		lessons[section] = kept
	}
	for _, s := range desired {
		lessons[s.SectionID] = mergeCourseIDs(lessons[s.SectionID], s.LessonIDs)
	}
	result := make([]courseOrder, 0)
	for _, id := range mergeCourseIDs(sections, managedSections) {
		result = append(result, courseOrder{SectionID: id, LessonIDs: lessons[id]})
	}
	return result
}

func applyCoursePlan(ctx context.Context, c *client.Client, folder string, original courseManifest, p *coursePlan) error {
	for _, step := range p.Plan {
		if step.Action == "conflict" && !p.Force {
			return fmt.Errorf("conflicting local and live edits in %s; inspect the preview and use --force --yes to overwrite", step.Path)
		}
	}
	ids := make(map[int64]int64)
	resolve := func(id int64) int64 {
		if actual, ok := ids[id]; ok {
			return actual
		}
		return id
	}
	manifest := original
	if manifest.Sections == nil {
		manifest.Sections = make(map[string]courseRef)
	}
	if manifest.Lessons == nil {
		manifest.Lessons = make(map[string]courseRef)
	}
	manifest.SpaceID = p.SpaceID
	// Record each confirmed ID immediately so a failed later upload can be retried safely.
	persist := func() error {
		for key, ref := range p.Manifest.Sections {
			ref.ID = resolve(ref.ID)
			if ref.ID > 0 {
				manifest.Sections[key] = ref
			}
		}
		for key, ref := range p.Manifest.Lessons {
			ref.ID = resolve(ref.ID)
			if ref.ID > 0 {
				for oldPath, previous := range manifest.Lessons {
					if previous.ID == ref.ID && oldPath != key {
						delete(manifest.Lessons, oldPath)
					}
				}
				manifest.Lessons[key] = ref
			}
		}
		return saveCourseManifest(folder, manifest)
	}
	for i := range p.Plan {
		step := &p.Plan[i]
		if !step.shouldApply(p.Force) {
			continue
		}
		var raw json.RawMessage
		var err error
		switch step.Action {
		case "create":
			endpoint := "/course_sections"
			if step.Kind == "lesson" {
				endpoint = "/course_lessons"
				step.Body["section_id"] = resolve(step.Body["section_id"].(int64))
			}
			raw, _, err = c.Post(ctx, endpoint, step.Body)
			if err == nil {
				var result struct {
					ID int64 `json:"id"`
				}
				err = courseResponse(raw, &result)
				if err == nil && result.ID <= 0 {
					err = fmt.Errorf("create %s returned no id", step.Kind)
				}
				if err == nil {
					ids[step.ID] = result.ID
					step.ID = result.ID
					err = persist()
				}
			}
		case "rename":
			raw, _, err = c.Put(ctx, fmt.Sprintf("/course_sections/%d", resolve(step.ID)), step.Body)
		case "update", "conflict":
			raw, _, err = c.Patch(ctx, fmt.Sprintf("/course_lessons/%d", resolve(step.ID)), step.Body)
		case "upload":
			step.ID = resolve(step.ID)
			var signedID string
			signedID, err = uploadCourseMedia(ctx, c, *step)
			if err == nil {
				raw, _, err = c.Patch(ctx, fmt.Sprintf("/course_lessons/%d", step.ID), map[string]any{"featured_media": signedID})
			}
			if err == nil {
				err = courseResponse(raw, nil)
			}
			if err == nil {
				ref := p.Manifest.Lessons[step.Path]
				ref.MediaChecksum = step.Checksum
				p.Manifest.Lessons[step.Path] = ref
				err = persist()
			}
		case "reorder":
			for i := range p.Order {
				p.Order[i].SectionID = resolve(p.Order[i].SectionID)
				for j := range p.Order[i].LessonIDs {
					p.Order[i].LessonIDs[j] = resolve(p.Order[i].LessonIDs[j])
				}
			}
			raw, _, err = c.Put(ctx, "/course_lessons/reorder", map[string]any{"space_id": p.SpaceID, "new_order": p.Order})
		}
		if err == nil && len(raw) > 0 {
			err = courseResponse(raw, nil)
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w (completed writes are not rolled back)", step.Action, step.Path, err)
		}
		if step.Action == "reorder" {
			manifest.Order = p.Order
		}
		if step.Kind == "lesson" && step.Synced.BodySHA256 != "" {
			recordCourseLessonSync(p.Manifest, p.Manifest.Lessons[step.Path].ID, step.Synced)
			if err := persist(); err != nil {
				return err
			}
		}
	}
	return persist()
}

func uploadCourseMedia(ctx context.Context, c *client.Client, step courseStep) (string, error) {
	f, err := os.Open(step.MediaPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := md5.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", err
	}
	if size != step.Size || base64.StdEncoding.EncodeToString(h.Sum(nil)) != step.Checksum {
		return "", fmt.Errorf("media changed since preview: %s", step.MediaPath)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(step.MediaPath)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	raw, _, err := c.Post(ctx, "/direct_uploads", map[string]any{"blob": map[string]any{"filename": filepath.Base(step.MediaPath), "content_type": contentType, "byte_size": size, "checksum": step.Checksum}})
	if err != nil {
		return "", err
	}
	var upload struct {
		SignedID string `json:"signed_id"`
		Direct   struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"direct_upload"`
	}
	if err := courseResponse(raw, &upload); err != nil {
		return "", err
	}
	u, err := url.Parse(upload.Direct.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || upload.SignedID == "" {
		return "", fmt.Errorf("invalid direct upload response")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.Direct.URL, f)
	if err != nil {
		return "", fmt.Errorf("invalid presigned upload request")
	}
	req.ContentLength = size
	for key, value := range upload.Direct.Headers {
		if strings.EqualFold(key, "Authorization") {
			return "", fmt.Errorf("direct upload unexpectedly requests Authorization")
		}
		req.Header.Set(key, value)
	}
	uploadClient := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := uploadClient.Do(req)
	if err != nil {
		return "", courseUploadError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("presigned upload returned HTTP %d", resp.StatusCode)
	}
	return upload.SignedID, nil
}

func courseUploadError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("presigned upload timed out; raise --timeout: %w", context.DeadlineExceeded)
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return fmt.Errorf("presigned upload failed")
}

func courseSlug(name string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	if b.Len() == 0 {
		return "untitled"
	}
	return b.String()
}

type courseDiskFile struct {
	Path string
	Data []byte
}

func coursePullFiles(live courseLive, spaceID int64) ([]courseDiskFile, courseManifest, error) {
	files := make([]courseDiskFile, 0)
	manifest := emptyCourseManifest()
	manifest.SpaceID = spaceID
	manifest.Order = make([]courseOrder, 0)
	if err := validateCourseLive(live); err != nil {
		return files, manifest, err
	}
	for i, s := range live.Sections {
		order := courseOrder{SectionID: s.ID, LessonIDs: make([]int64, 0)}
		dir := fmt.Sprintf("%02d-%s", i+1, courseSlug(s.Name))
		manifest.Sections[dir] = courseRef{ID: s.ID}
		front, err := yaml.Marshal(courseFrontmatter{ID: s.ID, Name: s.Name})
		if err != nil {
			return files, manifest, err
		}
		files = append(files, courseDiskFile{Path: dir + "/_section.md", Data: []byte("---\n" + string(front) + "---\n")})
		j := 0
		for _, l := range live.Lessons {
			if l.SectionID != s.ID {
				continue
			}
			j++
			order.LessonIDs = append(order.LessonIDs, l.ID)
			path := fmt.Sprintf("%s/%02d-%s.md", dir, j, courseSlug(l.Name))
			front, err := yaml.Marshal(courseFrontmatter{ID: l.ID, Name: l.Name, Status: l.Status})
			if err != nil {
				return files, manifest, err
			}
			body := ""
			if l.BodyHTML != nil {
				body = *l.BodyHTML
			}
			manifest.Lessons[path] = courseRef{ID: l.ID, Status: l.Status, BodySHA256: courseBodySHA256(body)}
			files = append(files, courseDiskFile{Path: path, Data: []byte("---\n" + string(front) + "---\n" + body)})
		}
		manifest.Order = append(manifest.Order, order)
	}
	return files, manifest, nil
}
func checkCourseDestination(folder string, force bool, spaceID int64) error {
	if err := rejectCourseSymlinks(folder, folder); err != nil {
		return err
	}
	entries, err := os.ReadDir(folder)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 && !force {
		return fmt.Errorf("folder is not empty; use --force to overwrite")
	}
	manifestPath := filepath.Join(folder, "course.yaml")
	if err := rejectCourseSymlinks(manifestPath, folder); err != nil {
		return err
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		var manifest courseManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("course.yaml: %w", err)
		}
		if manifest.SpaceID != 0 && manifest.SpaceID != spaceID {
			return fmt.Errorf("course.yaml space_id %d differs from requested space %d; use a separate folder", manifest.SpaceID, spaceID)
		}
	}
	return nil
}

func staleCourseFiles(folder string, files []courseDiskFile) ([]string, error) {
	written := make(map[string]bool)
	for _, file := range files {
		written[file.Path] = true
	}
	stale := make([]string, 0)
	entries, err := os.ReadDir(folder)
	if os.IsNotExist(err) {
		return stale, nil
	}
	if err != nil {
		return nil, err
	}
	for _, section := range entries {
		n, _ := coursePrefix(section.Name())
		if !section.IsDir() || n == int(^uint(0)>>1) {
			continue
		}
		children, err := os.ReadDir(filepath.Join(folder, section.Name()))
		if err != nil {
			return nil, err
		}
		for _, lesson := range children {
			n, _ := coursePrefix(lesson.Name())
			if lesson.IsDir() || (lesson.Name() != "_section.md" && (n == int(^uint(0)>>1) || filepath.Ext(lesson.Name()) != ".md")) {
				continue
			}
			name := section.Name() + "/" + lesson.Name()
			if !written[name] {
				stale = append(stale, name)
			}
		}
	}
	return stale, nil
}
func rejectCourseSymlinks(target, folder string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, abs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("course output escapes folder")
	}
	for current := abs; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlink %s", current)
		}
		if current == root {
			break
		}
	}
	return nil
}
func writeCourseFile(folder, name string, data []byte) error {
	target := filepath.Join(folder, filepath.FromSlash(name))
	if err := rejectCourseSymlinks(target, folder); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".course-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), target)
}
func saveCourseManifest(folder string, manifest courseManifest) error {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	return writeCourseFile(folder, "course.yaml", data)
}
