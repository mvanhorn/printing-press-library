package classroomwrite

import (
	"fmt"
	"os"
	"strings"
)

type BatchLessonTarget struct {
	CourseID   string
	CourseTitle string
	LessonID   string
	Order      int
	Title      string
	ContentFile string
	Skip       bool
	SkipReason string
}

type BatchLessonResult struct {
	CourseTitle     string `json:"course_title"`
	Order           int    `json:"order"`
	Title           string `json:"title"`
	LessonID        string `json:"lesson_id"`
	Skipped         bool   `json:"skipped,omitempty"`
	SkipReason      string `json:"skip_reason,omitempty"`
	UpdatedAtBefore string `json:"updated_at_before,omitempty"`
	UpdatedAtAfter  string `json:"updated_at_after,omitempty"`
	IdempotentNoOp  bool   `json:"idempotent_no_op,omitempty"`
	ParentPrivacy   int    `json:"parent_privacy,omitempty"`
	ParentState     int    `json:"parent_state,omitempty"`
}

type BatchNormalizeReport struct {
	Results         []BatchLessonResult `json:"results"`
	Stopped         bool                `json:"stopped"`
	StopReason      string              `json:"stop_reason,omitempty"`
	LastSuccess     *BatchLessonResult  `json:"last_success,omitempty"`
}

func RemainingBatchTargets0304() []BatchLessonTarget {
	return []BatchLessonTarget{
		{CourseTitle: "CapCut & Video for Real Estate", Order: 1, Title: "Video Content for Real Estate Pros", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\01-video-content-for-real-estate-pros.md`, Skip: true, SkipReason: "pilot already normalized"},
		{CourseTitle: "CapCut & Video for Real Estate", Order: 2, Title: "Plan Your Video Before Editing", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\02-plan-your-video-before-editing.md`},
		{CourseTitle: "CapCut & Video for Real Estate", Order: 3, Title: "Create an Open House Video", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\03-create-an-open-house-video.md`},
		{CourseTitle: "CapCut & Video for Real Estate", Order: 4, Title: "Customize Text and Captions", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\04-customize-text-and-captions.md`},
		{CourseTitle: "CapCut & Video for Real Estate", Order: 5, Title: "Write Better Reel Hooks", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\05-write-better-reel-hooks.md`},
		{CourseTitle: "CapCut & Video for Real Estate", Order: 6, Title: "Add Music, Timing, and Transitions", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\06-add-music-timing-and-transitions.md`},
		{CourseTitle: "CapCut & Video for Real Estate", Order: 7, Title: "Export for Social Media", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\07-export-for-social-media.md`},
		{CourseTitle: "CapCut & Video for Real Estate", Order: 8, Title: "Publish-One-Video Challenge", ContentFile: `C:\Users\Graha\skool-content\courses\03-capcut-video-for-real-estate\lessons\08-publish-one-video-challenge.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 1, Title: "Why Social Media Matters for Real Estate", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\01-why-social-media-matters-for-real-estate.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 2, Title: "Facebook Professional Mode", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\02-facebook-professional-mode.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 3, Title: "Optimize Your Profile for Real Estate", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\03-optimize-your-profile-for-real-estate.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 4, Title: "Useful vs. Risky Post Types", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\04-useful-vs-risky-post-types.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 5, Title: "Turn One Idea Into Three Posts", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\05-turn-one-idea-into-three-posts.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 6, Title: "Create a Simple Reel Caption", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\06-create-a-simple-reel-caption.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 7, Title: "Housing Ads and Special Ad Categories", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\07-housing-ads-and-special-ad-categories.md`},
		{CourseTitle: "Social Media & Facebook for Realtors", Order: 8, Title: "Social Visibility Challenge", ContentFile: `C:\Users\Graha\skool-content\courses\04-social-media-facebook-for-realtors\lessons\08-social-visibility-challenge.md`},
	}
}

func ResolveBatchTargets(api *API, targets []BatchLessonTarget) ([]BatchLessonTarget, error) {
	courseIDs := map[string]string{}
	for _, t := range targets {
		if _, ok := courseIDs[t.CourseTitle]; ok {
			continue
		}
		courses, err := api.ListCourses(AllowedGroupID)
		if err != nil {
			return nil, err
		}
		rec := FindCourseByTitle(courses, t.CourseTitle)
		if rec == nil {
			return nil, fmt.Errorf("course %q not found", t.CourseTitle)
		}
		courseIDs[t.CourseTitle] = rec.ID
	}
	out := make([]BatchLessonTarget, 0, len(targets))
	for _, t := range targets {
		t.CourseID = courseIDs[t.CourseTitle]
		if t.Skip {
			tree, err := api.GetCourseTree(t.CourseID)
			if err != nil {
				return nil, err
			}
			lesson := FindLessonByTitle(flattenChildren(tree), t.Title)
			if lesson == nil {
				return nil, fmt.Errorf("lesson %q not found under %q", t.Title, t.CourseTitle)
			}
			t.LessonID = lesson.ID
			out = append(out, t)
			continue
		}
		tree, err := api.GetCourseTree(t.CourseID)
		if err != nil {
			return nil, err
		}
		lesson := FindLessonByTitle(flattenChildren(tree), t.Title)
		if lesson == nil {
			return nil, fmt.Errorf("lesson %q not found under %q", t.Title, t.CourseTitle)
		}
		t.LessonID = lesson.ID
		out = append(out, t)
	}
	return out, nil
}

func (a *API) BatchNormalizeLessons(targets []BatchLessonTarget) (*BatchNormalizeReport, error) {
	report := &BatchNormalizeReport{}
	var lastSuccess *BatchLessonResult
	for _, target := range targets {
		if target.Skip {
			item := BatchLessonResult{
				CourseTitle: target.CourseTitle,
				Order:       target.Order,
				Title:       target.Title,
				LessonID:    target.LessonID,
				Skipped:     true,
				SkipReason:  target.SkipReason,
			}
			report.Results = append(report.Results, item)
			continue
		}
		desc, err := descFromContentFile(target.ContentFile)
		if err != nil {
			report.Stopped = true
			report.StopReason = err.Error()
			report.LastSuccess = lastSuccess
			return report, err
		}
		res, err := a.UpdateLesson(target.CourseID, target.LessonID, UpdateLessonOptions{Desc: desc})
		if err != nil {
			report.Stopped = true
			report.StopReason = err.Error()
			report.LastSuccess = lastSuccess
			return report, err
		}
		parent, err := a.GetCourseTree(target.CourseID)
		if err != nil {
			report.Stopped = true
			report.StopReason = err.Error()
			report.LastSuccess = lastSuccess
			return report, err
		}
		item := BatchLessonResult{
			CourseTitle:     target.CourseTitle,
			Order:           target.Order,
			Title:           target.Title,
			LessonID:        target.LessonID,
			UpdatedAtBefore: res.UpdatedAtBefore,
			UpdatedAtAfter:  res.UpdatedAtAfter,
			IdempotentNoOp:  res.IdempotentNoOp,
			ParentPrivacy:   parent.Course.Metadata.Privacy,
			ParentState:     parent.Course.State,
		}
		report.Results = append(report.Results, item)
		lastSuccess = &item
	}
	report.LastSuccess = lastSuccess
	return report, nil
}

func descFromContentFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text, err := StripFrontmatter(string(raw))
	if err != nil {
		return "", err
	}
	return MarkdownToLessonDesc(text)
}

func HasLiteralFormattingArtifacts(desc string) bool {
	if strings.Contains(desc, "**") || strings.Contains(desc, "```") {
		return true
	}
	if strings.Contains(desc, "contentReference") || strings.HasPrefix(strings.TrimSpace(desc), "---") {
		return true
	}
	return false
}
