package message

import "time"

// Draft is a local-only LinkedIn message draft. It is never sent unless the
// user explicitly runs messages send with confirmation.
type Draft struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	To          string    `json:"to"`
	Body        string    `json:"body"`
	Status      string    `json:"status"`
	SourceNote  string    `json:"source_note"`
	SentAt      time.Time `json:"sent_at,omitempty"`
	SendMode    string    `json:"send_mode,omitempty"`
	ConfirmText string    `json:"confirm_text,omitempty"`
}
