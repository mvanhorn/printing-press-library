// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/internal/tiptap"
)

// lessonFile is one parsed markdown lesson from the push bundle.
type lessonFile struct {
	Path        string `json:"file"`
	Course      string `json:"course"`
	CourseOrder int    `json:"course_order"`
	Lesson      string `json:"lesson"`
	Order       int    `json:"order"`
	Access      string `json:"access,omitempty"`
	Body        string `json:"-"`
}

// parseLessonDir walks dir for *.md lesson files and returns them in push
// order: course_order then course title, then order then lesson title within a
// course. Deterministic ordering matters because the plan is the artifact the
// user reviews, and an unstable plan is not reviewable.
func parseLessonDir(dir string) ([]lessonFile, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("reading lesson directory: %w", err)
	}
	if !info.IsDir() {
		return nil, usageErr(fmt.Errorf("%s is not a directory", dir))
	}

	var lessons []lessonFile
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}
		lesson, parseErr := parseLessonFile(path, string(raw))
		if parseErr != nil {
			return parseErr
		}
		lessons = append(lessons, lesson)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	// Two files claiming the same lesson in the same course would make the
	// push order ambiguous and silently overwrite one body with the other.
	seen := map[string]string{}
	for _, l := range lessons {
		key := titleKey(l.Course) + "\x00" + titleKey(l.Lesson)
		if prev, dup := seen[key]; dup {
			return nil, usageErr(fmt.Errorf("duplicate lesson %q in course %q: %s and %s", l.Lesson, l.Course, prev, l.Path))
		}
		seen[key] = l.Path
	}

	sort.SliceStable(lessons, func(i, j int) bool {
		a, b := lessons[i], lessons[j]
		if a.CourseOrder != b.CourseOrder {
			return a.CourseOrder < b.CourseOrder
		}
		if ak, bk := titleKey(a.Course), titleKey(b.Course); ak != bk {
			return ak < bk
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return titleKey(a.Lesson) < titleKey(b.Lesson)
	})
	return lessons, nil
}

// parseLessonFile reads the `---` delimited frontmatter block and the markdown
// body. The frontmatter is a flat key: value list, not general YAML — the
// format is fixed by this command, so a nested or list-valued key is a mistake
// worth reporting rather than a shape to support.
func parseLessonFile(path, content string) (lessonFile, error) {
	lesson := lessonFile{Path: path, CourseOrder: 1 << 30, Order: 1 << 30}

	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(normalized, "\n")

	idx := 0
	for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
		idx++
	}
	if idx >= len(lines) || strings.TrimSpace(lines[idx]) != "---" {
		return lesson, usageErr(fmt.Errorf("%s: missing frontmatter — the file must open with a '---' line", path))
	}
	idx++

	closed := false
	for idx < len(lines) {
		line := lines[idx]
		if strings.TrimSpace(line) == "---" {
			closed = true
			idx++
			break
		}
		idx++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return lesson, usageErr(fmt.Errorf("%s: frontmatter line is not 'key: value': %q", path, trimmed))
		}
		key = strings.TrimSpace(key)
		value = unquote(strings.TrimSpace(value))

		switch key {
		case "course":
			lesson.Course = value
		case "lesson":
			lesson.Lesson = value
		case "access":
			if err := validateAccess(value); err != nil {
				return lesson, usageErr(fmt.Errorf("%s: %w", path, err))
			}
			lesson.Access = value
		case "course_order", "order":
			n, err := strconv.Atoi(value)
			if err != nil {
				return lesson, usageErr(fmt.Errorf("%s: %s must be a whole number, got %q", path, key, value))
			}
			if key == "course_order" {
				lesson.CourseOrder = n
			} else {
				lesson.Order = n
			}
		default:
			return lesson, usageErr(fmt.Errorf("%s: unknown frontmatter key %q (want course, course_order, lesson, order, access)", path, key))
		}
	}
	if !closed {
		return lesson, usageErr(fmt.Errorf("%s: frontmatter is not closed with a '---' line", path))
	}
	if lesson.Course == "" {
		return lesson, usageErr(fmt.Errorf("%s: frontmatter is missing 'course'", path))
	}
	if lesson.Lesson == "" {
		return lesson, usageErr(fmt.Errorf("%s: frontmatter is missing 'lesson'", path))
	}

	lesson.Body = strings.TrimSpace(strings.Join(lines[idx:], "\n"))
	if lesson.Body == "" {
		return lesson, usageErr(fmt.Errorf("%s: lesson body is empty", path))
	}
	// Convert now so a malformed body fails during planning rather than
	// halfway through an apply.
	if _, err := tiptap.JSONFromMarkdown(lesson.Body); err != nil {
		return lesson, fmt.Errorf("%s: converting body to TipTap: %w", path, err)
	}
	return lesson, nil
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// validateAccess accepts the three access shapes Skool's course settings offer:
// open to every member, gated behind a level, or private.
func validateAccess(value string) error {
	switch {
	case value == "open", value == "private":
		return nil
	case strings.HasPrefix(value, "level:"):
		n, err := strconv.Atoi(strings.TrimPrefix(value, "level:"))
		if err != nil || n < 1 {
			return fmt.Errorf("access %q must be level:<n> with n >= 1", value)
		}
		return nil
	default:
		return fmt.Errorf("access %q must be one of open, level:<n>, private", value)
	}
}
