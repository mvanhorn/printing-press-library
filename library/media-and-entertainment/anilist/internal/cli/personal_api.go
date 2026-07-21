// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/anilist/internal/client"
)

// personalNow is replaceable in focused workflow tests so their boundary
// assertions describe the same instant the command uses.
var personalNow = time.Now

func anilistGraphQL(ctx context.Context, c *client.Client, query string, variables map[string]any, into any) error {
	raw, _, err := c.Post(ctx, "/", map[string]any{"query": query, "variables": variables})
	if err != nil {
		return apiErr(err)
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return apiErr(err)
	}
	if len(envelope.Errors) > 0 {
		return apiErr(fmt.Errorf("AniList GraphQL: %s", envelope.Errors[0].Message))
	}
	return json.Unmarshal(envelope.Data, into)
}

type personalMedia struct {
	ID    int `json:"id"`
	Title struct {
		UserPreferred string `json:"userPreferred"`
	} `json:"title"`
	Episodes int    `json:"episodes"`
	Duration int    `json:"duration"`
	Status   string `json:"status"`
}
type personalEntry struct {
	ID       int           `json:"id"`
	Progress int           `json:"progress"`
	Priority int           `json:"priority"`
	Score    float64       `json:"score"`
	Status   string        `json:"status"`
	Media    personalMedia `json:"media"`
}

func viewerID(ctx context.Context, c *client.Client) (int, error) {
	var r struct {
		Viewer struct {
			ID int `json:"id"`
		} `json:"Viewer"`
	}
	err := anilistGraphQL(ctx, c, `query { Viewer { id } }`, nil, &r)
	return r.Viewer.ID, err
}

func allListEntries(ctx context.Context, c *client.Client, userID int, status string) ([]personalEntry, error) {
	const q = `query($user:Int!,$status:MediaListStatus!,$page:Int!){Page(page:$page,perPage:50){pageInfo{hasNextPage} mediaList(userId:$user,type:ANIME,status:$status){id progress priority score status media{id title{userPreferred} episodes duration status}}}}`
	var all []personalEntry
	for page := 1; ; page++ {
		var r struct {
			Page struct {
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"pageInfo"`
				MediaList []personalEntry `json:"mediaList"`
			} `json:"Page"`
		}
		if err := anilistGraphQL(ctx, c, q, map[string]any{"user": userID, "status": status, "page": page}, &r); err != nil {
			return nil, err
		}
		all = append(all, r.Page.MediaList...)
		if !r.Page.PageInfo.HasNextPage {
			return all, nil
		}
	}
}
