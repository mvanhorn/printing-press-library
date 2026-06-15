package classroomwrite

import (
	"fmt"
	"strings"
)

const (
	AllowedCommunitySlug = "ai-for-real-estate-pros-2367"
	AllowedCommunityName = "AI for Real Estate Pros"
	AllowedGroupID       = "daa1f816d1ff4d4a83e8874c0b2fc3c2"

	MaxCoursesPerImport = 5
	MaxLessonsPerImport = 40
)

var (
	allowedImportCourseTitles = map[string]struct{}{
		"CapCut & Video for Real Estate":            {},
		"Social Media & Facebook for Realtors":      {},
		"Canva AI & Content Creation":               {},
		"Gemini & Creative AI Tools":                {},
		"Real Estate Prompt Library":                {},
		"AI Productivity & Daily Realtor Workflows":   {},
		"Local SEO & Google Business Profile":         {},
		"YouTube, Reels & Video Distribution":         {},
		"Verified Market Updates & Data Content":      {},
		"Gamma & AI Presentations":                    {},
		"AI Lead Follow-Up & Client Conversations":  {},
		"Open House Marketing & Lead Conversion":    {},
		"Listing Marketing System":                    {},
		"Buyer & Seller Consultations With AI":      {},
		"Database, Past Clients & Referral Marketing": {},
		"The AI Sandbox": {},
	}

	protectedCourseTitles = map[string]struct{}{
		"Start Here":                      {},
		"ChatGPT for Real Estate":         {},
		"AI Images & Personal Branding":   {},
	}
)

type CommunityIdentity struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	GroupID     string `json:"group_id"`
}

func NormalizeTitle(title string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
}

func ValidateCommunityIdentity(id CommunityIdentity) error {
	if id.Slug != AllowedCommunitySlug {
		return fmt.Errorf("community slug mismatch: got %q want %q", id.Slug, AllowedCommunitySlug)
	}
	if id.GroupID != AllowedGroupID {
		return fmt.Errorf("community group id mismatch: got %q want %q", id.GroupID, AllowedGroupID)
	}
	if id.DisplayName != AllowedCommunityName {
		return fmt.Errorf("community display name mismatch: got %q want %q", id.DisplayName, AllowedCommunityName)
	}
	return nil
}

func RequireCommunityConfirmation(confirmed bool) error {
	if !confirmed {
		return fmt.Errorf("refusing write: pass --confirm-community %q to acknowledge target community", AllowedCommunitySlug)
	}
	return nil
}

func AssertImportCourseTitle(title string) error {
	n := NormalizeTitle(title)
	if _, ok := protectedCourseTitles[n]; ok {
		return fmt.Errorf("refusing to modify protected course %q", n)
	}
	if _, ok := allowedImportCourseTitles[n]; !ok {
		return fmt.Errorf("course title %q is not in the approved import allowlist", n)
	}
	return nil
}

func AssertDraftPrivate(state int, privacy int) error {
	if state != 1 {
		return fmt.Errorf("refusing write: expected draft state=1, got state=%d", state)
	}
	if privacy != 1 {
		return fmt.Errorf("refusing write: expected private privacy=1, got privacy=%d", privacy)
	}
	return nil
}

func AssertLessonDraft(state int) error {
	if state != 1 {
		return fmt.Errorf("refusing write: expected draft lesson state=1, got state=%d", state)
	}
	return nil
}

func AssertDraftCourse(state int) error {
	if state != 1 {
		return fmt.Errorf("refusing write: expected draft course state=1, got state=%d", state)
	}
	return nil
}

func AssertPrivacyUnchanged(before, after int) error {
	if before != after {
		return fmt.Errorf("refusing write: parent course privacy changed from %d to %d", before, after)
	}
	return nil
}
