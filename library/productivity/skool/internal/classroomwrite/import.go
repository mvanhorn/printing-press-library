package classroomwrite

import (
	"fmt"
)

type Importer struct {
	API *API
}

func (im *Importer) ImportPlan(plan *LocalCoursePlan, existing []CourseRecord, counters *ImportCounters) (*ImportResult, error) {
	if err := AssertImportCourseTitle(plan.Title); err != nil {
		return nil, err
	}
	result := &ImportResult{CourseTitle: plan.Title}
	if found := FindCourseByTitle(existing, plan.Title); found != nil {
		result.Action = "skip_exists"
		result.CourseID = found.ID
		lessons, err := im.importLessons(plan, found.ID, counters)
		if err != nil {
			return nil, err
		}
		result.Lessons = lessons
		return result, nil
	}
	if counters.CoursesCreated >= MaxCoursesPerImport {
		return nil, fmt.Errorf("refusing import: would exceed max new courses (%d)", MaxCoursesPerImport)
	}
	created, err := im.API.CreateCourse(AllowedGroupID, plan.Title, plan.Description)
	if err != nil {
		return nil, err
	}
	counters.CoursesCreated++
	result.Action = "created"
	result.CourseID = created.ID
	lessons, err := im.importLessons(plan, created.ID, counters)
	if err != nil {
		return nil, err
	}
	result.Lessons = lessons
	return result, nil
}

func (im *Importer) DryRunPlan(plan *LocalCoursePlan, existing []CourseRecord, counters *ImportCounters) (*ImportResult, error) {
	if err := AssertImportCourseTitle(plan.Title); err != nil {
		return nil, err
	}
	result := &ImportResult{CourseTitle: plan.Title}
	if found := FindCourseByTitle(existing, plan.Title); found != nil {
		result.Action = "skip_exists"
		result.CourseID = found.ID
		lessons, err := im.dryRunLessons(plan, found.ID, counters)
		if err != nil {
			return nil, err
		}
		result.Lessons = lessons
		return result, nil
	}
	if counters.CoursesCreated >= MaxCoursesPerImport {
		return nil, fmt.Errorf("refusing import: would exceed max new courses (%d)", MaxCoursesPerImport)
	}
	counters.CoursesCreated++
	result.Action = "would_create"
	result.CourseID = "(dry-run)"
	lessons, err := im.dryRunLessons(plan, "(dry-run)", counters)
	if err != nil {
		return nil, err
	}
	result.Lessons = lessons
	return result, nil
}

func (im *Importer) dryRunLessons(plan *LocalCoursePlan, courseID string, counters *ImportCounters) ([]LessonImportResult, error) {
	var existingLessons []CourseRecord
	if courseID != "(dry-run)" {
		tree, err := im.API.GetCourseTree(courseID)
		if err != nil {
			return nil, err
		}
		existingLessons = flattenChildren(tree)
	}
	out := make([]LessonImportResult, 0, len(plan.Lessons))
	for _, lesson := range plan.Lessons {
		item := LessonImportResult{Order: lesson.Order, Title: lesson.Title}
		if found := FindLessonByTitle(existingLessons, lesson.Title); found != nil {
			item.Action = "skip_exists"
			item.LessonID = found.ID
			out = append(out, item)
			continue
		}
		if counters.LessonsCreated >= MaxLessonsPerImport {
			return nil, fmt.Errorf("refusing import: would exceed max new lessons (%d)", MaxLessonsPerImport)
		}
		if _, err := MarkdownToLessonDesc(lesson.Body); err != nil {
			return nil, fmt.Errorf("lesson %q markdown conversion: %w", lesson.Title, err)
		}
		counters.LessonsCreated++
		item.Action = "would_create"
		item.LessonID = "(dry-run)"
		out = append(out, item)
	}
	return out, nil
}

func (im *Importer) importLessons(plan *LocalCoursePlan, courseID string, counters *ImportCounters) ([]LessonImportResult, error) {
	tree, err := im.API.GetCourseTree(courseID)
	if err != nil {
		return nil, err
	}
	existingLessons := flattenChildren(tree)
	out := make([]LessonImportResult, 0, len(plan.Lessons))
	for _, lesson := range plan.Lessons {
		item := LessonImportResult{Order: lesson.Order, Title: lesson.Title}
		if found := FindLessonByTitle(existingLessons, lesson.Title); found != nil {
			item.Action = "skip_exists"
			item.LessonID = found.ID
			out = append(out, item)
			continue
		}
		if counters.LessonsCreated >= MaxLessonsPerImport {
			return nil, fmt.Errorf("refusing import: would exceed max new lessons (%d)", MaxLessonsPerImport)
		}
		desc, err := MarkdownToLessonDesc(lesson.Body)
		if err != nil {
			return nil, fmt.Errorf("lesson %q markdown conversion: %w", lesson.Title, err)
		}
		created, err := im.API.CreateLesson(AllowedGroupID, courseID, lesson.Title, desc)
		if err != nil {
			return nil, fmt.Errorf("creating lesson %q: %w", lesson.Title, err)
		}
		counters.LessonsCreated++
		item.Action = "created"
		item.LessonID = created.ID
		out = append(out, item)
	}
	return out, nil
}

type ImportCounters struct {
	CoursesCreated int
	LessonsCreated int
}

func VerifyRemoteCourses(api *API, expectedTitles []string) (map[string]bool, error) {
	courses, err := api.ListCourses(AllowedGroupID)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, title := range expectedTitles {
		found[title] = FindCourseByTitle(courses, title) != nil
	}
	return found, nil
}
