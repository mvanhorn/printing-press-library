package classroomwrite

type CourseRecord struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	UnitType  string         `json:"unit_type"`
	State     int            `json:"state"`
	GroupID   string         `json:"group_id"`
	RootID    string         `json:"root_id"`
	ParentID  string         `json:"parent_id"`
	Metadata  CourseMetadata `json:"metadata"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type CourseMetadata struct {
	Title      string `json:"title"`
	Desc       string `json:"desc"`
	Privacy    int    `json:"privacy"`
	CoverImage string `json:"cover_image"`
	HasAccess  int    `json:"has_access"`
	Resources  string `json:"resources"`
}

type CourseTree struct {
	Course   CourseRecord `json:"course"`
	Children []struct {
		Course CourseRecord `json:"course"`
	} `json:"children"`
}

type LocalCoursePlan struct {
	Dir         string
	CourseNum   string
	Title       string
	Description string
	Lessons     []LocalLessonPlan
}

type LocalLessonPlan struct {
	Order int
	Title string
	Body  string
	File  string
}

type ImportResult struct {
	CourseTitle string `json:"course_title"`
	Action      string `json:"action"`
	CourseID    string `json:"course_id,omitempty"`
	Lessons     []LessonImportResult `json:"lessons,omitempty"`
}

type LessonImportResult struct {
	Order    int    `json:"order"`
	Title    string `json:"title"`
	Action   string `json:"action"`
	LessonID string `json:"lesson_id,omitempty"`
}
