package agent

import (
	"fmt"
	"strings"
	"time"
)

const (
	LogActivityTypeCall    = "call"
	LogActivityTypeEmail   = "email"
	LogActivityTypeMeeting = "meeting"
)

type LogActivityOptions struct {
	Type            string
	WhatID          string
	WhoID           string
	Subject         string
	Description     string
	DurationSeconds int
	Start           time.Time
	End             time.Time
	IdempotencyKey  string
	Now             func() time.Time
}

func NewLogActivityWriteOptions(opts LogActivityOptions) (WriteOptions, error) {
	opts.Type = strings.ToLower(strings.TrimSpace(opts.Type))
	opts.WhatID = strings.TrimSpace(opts.WhatID)
	opts.WhoID = strings.TrimSpace(opts.WhoID)
	opts.Subject = strings.TrimSpace(opts.Subject)
	opts.IdempotencyKey = strings.TrimSpace(opts.IdempotencyKey)
	if opts.Now == nil {
		opts.Now = time.Now
	}

	if opts.Type == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --type is required")
	}
	if opts.Subject == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --subject is required")
	}
	if opts.IdempotencyKey == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --idempotency-key is required")
	}
	if opts.WhatID == "" && opts.WhoID == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_RELATED_RECORD: at least one of --what or --who is required")
	}

	switch opts.Type {
	case LogActivityTypeCall, LogActivityTypeEmail:
		fields := map[string]any{
			"Subject":      opts.Subject,
			"ActivityDate": opts.Now().UTC().Format("2006-01-02"),
			"Status":       "Completed",
			"Priority":     "Normal",
			"TaskSubtype":  opts.Type,
		}
		if opts.Description != "" {
			fields["Description"] = opts.Description
		}
		if opts.WhatID != "" {
			fields["WhatId"] = opts.WhatID
		}
		if opts.WhoID != "" {
			fields["WhoId"] = opts.WhoID
		}
		if opts.DurationSeconds > 0 {
			fields["CallDurationInSeconds"] = opts.DurationSeconds
		}
		return NewCreateWriteOptions("Task", opts.IdempotencyKey, fields), nil
	case LogActivityTypeMeeting:
		if opts.Start.IsZero() || opts.End.IsZero() {
			return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --start and --end are required for --type meeting")
		}
		fields := map[string]any{
			"Subject":       opts.Subject,
			"StartDateTime": opts.Start.UTC().Format(time.RFC3339),
			"EndDateTime":   opts.End.UTC().Format(time.RFC3339),
		}
		if opts.Description != "" {
			fields["Description"] = opts.Description
		}
		if opts.WhatID != "" {
			fields["WhatId"] = opts.WhatID
		}
		if opts.WhoID != "" {
			fields["WhoId"] = opts.WhoID
		}
		return NewCreateWriteOptions("Event", opts.IdempotencyKey, fields), nil
	default:
		return WriteOptions{}, fmt.Errorf("INVALID_TYPE: --type must be one of call, email, meeting")
	}
}
