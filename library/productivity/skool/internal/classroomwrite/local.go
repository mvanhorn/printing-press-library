package classroomwrite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadLocalCoursePlan(dir string) (*LocalCoursePlan, error) {
	metaPath := filepath.Join(dir, "course.meta.json")
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", metaPath, err)
	}
	var meta struct {
		CourseNumber string `json:"course_number"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Status       string `json:"status"`
		Visibility   string `json:"visibility"`
		Lessons      []struct {
			Order int    `json:"order"`
			Title string `json:"title"`
			File  string `json:"file"`
		} `json:"lessons"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", metaPath, err)
	}
	if meta.Status != "draft" {
		return nil, fmt.Errorf("%s: status must be draft", metaPath)
	}
	if meta.Visibility != "private" {
		return nil, fmt.Errorf("%s: visibility must be private", metaPath)
	}
	plan := &LocalCoursePlan{
		Dir:         dir,
		CourseNum:   meta.CourseNumber,
		Title:       meta.Title,
		Description: meta.Description,
	}
	for _, item := range meta.Lessons {
		path := filepath.Join(dir, "lessons", item.File)
		bodyRaw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading lesson %s: %w", path, err)
		}
		body, err := StripFrontmatter(string(bodyRaw))
		if err != nil {
			return nil, err
		}
		plan.Lessons = append(plan.Lessons, LocalLessonPlan{
			Order: item.Order,
			Title: item.Title,
			Body:  body,
			File:  item.File,
		})
	}
	if len(plan.Lessons) != 8 {
		return nil, fmt.Errorf("%s: expected 8 lessons, got %d", dir, len(plan.Lessons))
	}
	return plan, nil
}

func StripFrontmatter(raw string) (string, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return strings.TrimSpace(raw), nil
	}
	end := strings.Index(raw[4:], "\n---\n")
	if end == -1 {
		return "", fmt.Errorf("malformed YAML frontmatter")
	}
	body := raw[4+end+5:]
	return strings.TrimSpace(body), nil
}

func Preset0304CourseDirs() []string {
	return []string{
		`C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate`,
		`C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors`,
	}
}

func LoadPreset0304Plans() ([]*LocalCoursePlan, error) {
	dirs := Preset0304CourseDirs()
	out := make([]*LocalCoursePlan, 0, len(dirs))
	for _, dir := range dirs {
		plan, err := LoadLocalCoursePlan(dir)
		if err != nil {
			return nil, err
		}
		if err := AssertImportCourseTitle(plan.Title); err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func Preset0507CourseDirs() []string {
	return []string{
		`C:\Users\Graha\skool-content\courses\05-canva-ai-content-creation`,
		`C:\Users\Graha\skool-content\courses\06-gemini-creative-ai-tools`,
		`C:\Users\Graha\skool-content\courses\07-real-estate-prompt-library`,
	}
}

func LoadPreset0507Plans() ([]*LocalCoursePlan, error) {
	dirs := Preset0507CourseDirs()
	out := make([]*LocalCoursePlan, 0, len(dirs))
	for _, dir := range dirs {
		plan, err := LoadLocalCoursePlan(dir)
		if err != nil {
			return nil, err
		}
		if err := AssertImportCourseTitle(plan.Title); err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func Preset0812CourseDirs() []string {
	return []string{
		`C:\Users\Graha\skool-content\courses\08-gamma-ai-presentations`,
		`C:\Users\Graha\skool-content\courses\09-verified-market-updates-data-content`,
		`C:\Users\Graha\skool-content\courses\10-youtube-reels-video-distribution`,
		`C:\Users\Graha\skool-content\courses\11-local-seo-google-business-profile`,
		`C:\Users\Graha\skool-content\courses\12-ai-productivity-daily-realtor-workflows`,
	}
}

func LoadPreset0812Plans() ([]*LocalCoursePlan, error) {
	dirs := Preset0812CourseDirs()
	out := make([]*LocalCoursePlan, 0, len(dirs))
	for _, dir := range dirs {
		plan, err := LoadLocalCoursePlan(dir)
		if err != nil {
			return nil, err
		}
		if err := AssertImportCourseTitle(plan.Title); err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func Preset1317CourseDirs() []string {
	return []string{
		`C:\Users\Graha\skool-content\courses\13-ai-lead-follow-up-client-conversations`,
		`C:\Users\Graha\skool-content\courses\14-open-house-marketing-lead-conversion`,
		`C:\Users\Graha\skool-content\courses\15-listing-marketing-system`,
		`C:\Users\Graha\skool-content\courses\16-buyer-seller-consultations-with-ai`,
		`C:\Users\Graha\skool-content\courses\17-database-past-clients-referral-marketing`,
	}
}

func LoadPreset1317Plans() ([]*LocalCoursePlan, error) {
	dirs := Preset1317CourseDirs()
	out := make([]*LocalCoursePlan, 0, len(dirs))
	for _, dir := range dirs {
		plan, err := LoadLocalCoursePlan(dir)
		if err != nil {
			return nil, err
		}
		if err := AssertImportCourseTitle(plan.Title); err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

