package cli

// fathomMeeting is the full shape of a Fathom meeting as stored in local SQLite.
type fathomMeeting struct {
	Title                       string                `json:"title"`
	MeetingTitle                *string               `json:"meeting_title"`
	URL                         string                `json:"url"`
	ShareURL                    string                `json:"share_url"`
	CreatedAt                   string                `json:"created_at"`
	ScheduledStartTime          *string               `json:"scheduled_start_time"`
	ScheduledEndTime            *string               `json:"scheduled_end_time"`
	RecordingID                 int64                 `json:"recording_id"`
	RecordingStartTime          *string               `json:"recording_start_time"`
	RecordingEndTime            *string               `json:"recording_end_time"`
	CalendarInviteesDomainsType string                `json:"calendar_invitees_domains_type"`
	TranscriptLanguage          string                `json:"transcript_language"`
	Transcript                  []fathomTranscriptSeg `json:"transcript"`
	DefaultSummary              *fathomSummary        `json:"default_summary"`
	ActionItems                 []fathomActionItem    `json:"action_items"`
	CalendarInvitees            []fathomInvitee       `json:"calendar_invitees"`
	RecordedBy                  *fathomRecordedBy     `json:"recorded_by"`
	CRMMatches                  *fathomCRMMatches     `json:"crm_matches"`
}

type fathomTranscriptSeg struct {
	Speaker   fathomSpeaker `json:"speaker"`
	Text      string        `json:"text"`
	Timestamp string        `json:"timestamp"`
}

type fathomSpeaker struct {
	DisplayName                 string  `json:"display_name"`
	MatchedCalendarInviteeEmail *string `json:"matched_calendar_invitee_email"`
}

type fathomSummary struct {
	TemplateName      *string `json:"template_name"`
	MarkdownFormatted *string `json:"markdown_formatted"`
}

type fathomActionItem struct {
	Description        string         `json:"description"`
	UserGenerated      bool           `json:"user_generated"`
	Completed          bool           `json:"completed"`
	RecordingTimestamp string         `json:"recording_timestamp"`
	RecordingPlayback  string         `json:"recording_playback_url"`
	Assignee           *fathomAssignee `json:"assignee"`
}

type fathomAssignee struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Team  string `json:"team"`
}

type fathomInvitee struct {
	Name                      string  `json:"name"`
	Email                     string  `json:"email"`
	EmailDomain               string  `json:"email_domain"`
	IsExternal                bool    `json:"is_external"`
	MatchedSpeakerDisplayName *string `json:"matched_speaker_display_name"`
}

type fathomRecordedBy struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	EmailDomain string `json:"email_domain"`
	Team        string `json:"team"`
}

type fathomCRMMatches struct {
	Contacts  []fathomCRMContact `json:"contacts"`
	Companies []fathomCRMCompany `json:"companies"`
	Deals     []fathomCRMDeal    `json:"deals"`
	Error     *string            `json:"error"`
}

type fathomCRMContact struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	RecordURL string `json:"record_url"`
}

type fathomCRMCompany struct {
	Name      string `json:"name"`
	RecordURL string `json:"record_url"`
}

type fathomCRMDeal struct {
	Name      string  `json:"name"`
	Amount    float64 `json:"amount"`
	RecordURL string  `json:"record_url"`
}

// meetingTitle returns the best available display title for a meeting.
func (m fathomMeeting) meetingTitle() string {
	if m.MeetingTitle != nil && *m.MeetingTitle != "" {
		return *m.MeetingTitle
	}
	return m.Title
}

// durationMinutes computes the meeting duration in minutes from recording times.
// Falls back to scheduled times if recording times are absent.
func (m fathomMeeting) durationMinutes() float64 {
	start, end := m.RecordingStartTime, m.RecordingEndTime
	if start == nil || end == nil {
		start, end = m.ScheduledStartTime, m.ScheduledEndTime
	}
	if start == nil || end == nil {
		return 0
	}
	s, err1 := parseFlexTime(*start)
	e, err2 := parseFlexTime(*end)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := e.Sub(s).Minutes()
	if d < 0 {
		return 0
	}
	return d
}
