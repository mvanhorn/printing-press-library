package agent

import (
	"encoding/json"
	"fmt"
	"os"
)

type SlackPoster interface {
	PostMessage(channel, text string) error
	PostEphemeral(channel, user, text string) error
	UploadFile(channel, path string) error
}

type InjectOptions struct {
	BundlePath    string
	Channel       string
	Ephemeral     bool
	EphemeralUser string
	Attach        bool
	AllowWaiver   bool
	Members       []AudienceMember
	Slack         SlackPoster
}

type InjectResult struct {
	Channel                   string               `json:"channel"`
	Posted                    bool                 `json:"posted"`
	Ephemeral                 bool                 `json:"ephemeral"`
	Attached                  bool                 `json:"attached"`
	InjectAudienceIntersected bool                 `json:"inject_audience_intersected"`
	InjectAudienceWaived      bool                 `json:"inject_audience_waived"`
	AudienceIntersection      AudienceIntersection `json:"audience_intersection"`
	RenderedMarkdown          string               `json:"rendered_markdown,omitempty"`
}

func InjectBundle(opts InjectOptions) (*InjectResult, error) {
	if opts.BundlePath == "" {
		return nil, fmt.Errorf("bundle path is required")
	}
	if opts.Channel == "" {
		return nil, fmt.Errorf("Slack channel is required")
	}
	if opts.Slack == nil {
		return nil, fmt.Errorf("Slack poster is required")
	}
	if _, err := VerifyBundle(opts.BundlePath, VerifyOptions{}); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(opts.BundlePath)
	if err != nil {
		return nil, fmt.Errorf("read bundle: %w", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parse bundle: %w", err)
	}
	intersection, err := IntersectAudienceFLS(opts.Members, opts.AllowWaiver)
	if err != nil {
		return &InjectResult{Channel: opts.Channel, AudienceIntersection: intersection}, err
	}
	markdown, err := RenderInjectMarkdown(bundle, intersection)
	if err != nil {
		return nil, err
	}
	if opts.Ephemeral {
		if opts.EphemeralUser == "" {
			return nil, fmt.Errorf("ephemeral Slack posts require target user")
		}
		if err := opts.Slack.PostEphemeral(opts.Channel, opts.EphemeralUser, markdown); err != nil {
			return nil, err
		}
	} else if err := opts.Slack.PostMessage(opts.Channel, markdown); err != nil {
		return nil, err
	}
	attached := false
	if opts.Attach {
		if err := opts.Slack.UploadFile(opts.Channel, opts.BundlePath); err != nil {
			return nil, err
		}
		attached = true
	}
	return &InjectResult{
		Channel:                   opts.Channel,
		Posted:                    true,
		Ephemeral:                 opts.Ephemeral,
		Attached:                  attached,
		InjectAudienceIntersected: true,
		InjectAudienceWaived:      intersection.Waived,
		AudienceIntersection:      intersection,
		RenderedMarkdown:          markdown,
	}, nil
}
