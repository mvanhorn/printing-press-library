package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/monday/internal/store"
)

type mondayGraphQLSyncClient interface {
	GraphQL(string, map[string]any) (json.RawMessage, error)
	RateLimit() float64
}

func isMondayGraphQLSyncResource(resource string) bool {
	switch resource {
	case "workspaces", "boards", "users", "teams", "items", "columns", "groups", "updates", "docs", "activity_logs":
		return true
	default:
		return false
	}
}

func syncMondayGraphQLResource(c mondayGraphQLSyncClient, db *store.Store, resource string, maxPages int, started time.Time) syncResult {
	var (
		items []json.RawMessage
		err   error
	)

	switch resource {
	case "workspaces":
		items, err = fetchMondayPagedList(c, queryWorkspacesList, "workspaces", nil, maxPages)
	case "boards":
		items, err = fetchMondayPagedList(c, queryBoardsList, "boards", nil, maxPages)
	case "users":
		items, err = fetchMondayPagedList(c, queryUsersList, "users", nil, maxPages)
	case "teams":
		items, err = fetchMondaySingleList(c, queryTeamsList, "teams", nil)
	case "updates":
		items, err = fetchMondayPagedList(c, queryUpdatesList, "updates", nil, maxPages)
	case "docs":
		items, err = fetchMondayPagedList(c, queryDocsList, "docs", nil, maxPages)
	case "items":
		items, err = fetchMondayItemsForAllBoards(c, maxPages)
	case "columns":
		items, err = fetchMondayBoardChildList(c, queryColumnsList, "columns", maxPages)
	case "groups":
		items, err = fetchMondayBoardChildList(c, queryGroupsList, "groups", maxPages)
	case "activity_logs":
		items, err = fetchMondayActivityLogsForAllBoards(c, maxPages)
	default:
		err = fmt.Errorf("unsupported monday GraphQL sync resource %q", resource)
	}
	if err != nil {
		if !humanFriendly {
			fmt.Fprintf(os.Stdout, `{"event":"sync_error","resource":"%s","error":"%s"}`+"\n", resource, escapeJSONString(err.Error()))
		}
		return syncResult{Resource: resource, Err: err, Duration: time.Since(started)}
	}

	stored, extractFailures, err := upsertResourceBatch(db, resource, items)
	if err != nil {
		if !humanFriendly {
			fmt.Fprintf(os.Stdout, `{"event":"sync_error","resource":"%s","error":"%s"}`+"\n", resource, escapeJSONString(err.Error()))
		}
		return syncResult{Resource: resource, Err: err, Duration: time.Since(started)}
	}
	if extractFailures > 0 && !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_anomaly","resource":"%s","consumed":%d,"stored":%d,"count":%d,"reason":"primary_key_unresolved"}`+"\n", resource, len(items), stored, extractFailures)
	}
	if err := db.SaveSyncState(resource, "", stored); err != nil {
		return syncResult{Resource: resource, Count: stored, Err: err, Duration: time.Since(started)}
	}
	if !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_complete","resource":"%s","total":%d,"duration_ms":%d}`+"\n", resource, stored, time.Since(started).Milliseconds())
	}
	return syncResult{Resource: resource, Count: stored, Duration: time.Since(started)}
}

func fetchMondayPagedList(c mondayGraphQLSyncClient, query, root string, extra map[string]any, maxPages int) ([]json.RawMessage, error) {
	if maxPages <= 0 {
		maxPages = 1000000
	}
	var all []json.RawMessage
	for page := 1; page <= maxPages; page++ {
		vars := map[string]any{"limit": 100, "page": page}
		for k, v := range extra {
			vars[k] = v
		}
		data, err := c.GraphQL(query, vars)
		if err != nil {
			return all, err
		}
		batch, err := pluckJSONArray(data, root)
		if err != nil {
			return all, err
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func fetchMondaySingleList(c mondayGraphQLSyncClient, query, root string, vars map[string]any) ([]json.RawMessage, error) {
	data, err := c.GraphQL(query, vars)
	if err != nil {
		return nil, err
	}
	return pluckJSONArray(data, root)
}

func fetchMondayBoardIDs(c mondayGraphQLSyncClient, maxPages int) ([]string, error) {
	boards, err := fetchMondayPagedList(c, queryBoardsList, "boards", nil, maxPages)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(boards))
	for _, raw := range boards {
		var obj struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &obj) == nil && obj.ID != "" {
			ids = append(ids, obj.ID)
		}
	}
	return ids, nil
}

func fetchMondayItemsForAllBoards(c mondayGraphQLSyncClient, maxPages int) ([]json.RawMessage, error) {
	boardIDs, err := fetchMondayBoardIDs(c, maxPages)
	if err != nil {
		return nil, err
	}
	var all []json.RawMessage
	for _, boardID := range boardIDs {
		cursor := ""
		pages := 0
		for {
			pages++
			if maxPages > 0 && pages > maxPages {
				break
			}
			vars := map[string]any{"board_id": boardID, "limit": 100}
			if cursor != "" {
				vars["cursor"] = cursor
			}
			data, err := c.GraphQL(queryItemsList, vars)
			if err != nil {
				return all, err
			}
			page, err := pluckFirstBoardObjectField(data, "items_page")
			if err != nil {
				return all, err
			}
			var pageObj struct {
				Cursor string            `json:"cursor"`
				Items  []json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(page, &pageObj); err != nil {
				return all, fmt.Errorf("parsing items_page for board %s: %w", boardID, err)
			}
			for _, item := range pageObj.Items {
				all = append(all, addBoardID(item, boardID))
			}
			if pageObj.Cursor == "" || len(pageObj.Items) == 0 {
				break
			}
			cursor = pageObj.Cursor
		}
	}
	return all, nil
}

func fetchMondayBoardChildList(c mondayGraphQLSyncClient, query, childKey string, maxPages int) ([]json.RawMessage, error) {
	boardIDs, err := fetchMondayBoardIDs(c, maxPages)
	if err != nil {
		return nil, err
	}
	var all []json.RawMessage
	for _, boardID := range boardIDs {
		data, err := c.GraphQL(query, map[string]any{"board_id": []string{boardID}})
		if err != nil {
			return all, err
		}
		children, err := pluckFirstBoardArrayField(data, childKey)
		if err != nil {
			return all, err
		}
		for _, child := range children {
			all = append(all, addBoardScopedID(child, boardID))
		}
	}
	return all, nil
}

func fetchMondayActivityLogsForAllBoards(c mondayGraphQLSyncClient, maxPages int) ([]json.RawMessage, error) {
	boardIDs, err := fetchMondayBoardIDs(c, maxPages)
	if err != nil {
		return nil, err
	}
	var all []json.RawMessage
	for _, boardID := range boardIDs {
		pages := maxPages
		if pages <= 0 {
			pages = 1000000
		}
		for page := 1; page <= pages; page++ {
			vars := map[string]any{"id": []string{boardID}, "limit": 100, "page": page}
			data, err := c.GraphQL(queryBoardActivity, vars)
			if err != nil {
				return all, err
			}
			logs, err := pluckFirstBoardArrayField(data, "activity_logs")
			if err != nil {
				return all, err
			}
			for _, log := range logs {
				all = append(all, addBoardID(log, boardID))
			}
			if len(logs) < 100 {
				break
			}
		}
	}
	return all, nil
}

func pluckJSONArray(data json.RawMessage, key string) ([]json.RawMessage, error) {
	raw, err := pluckJSONField(data, key)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" || len(raw) == 0 {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing %s array: %w", key, err)
	}
	return items, nil
}

func pluckFirstBoardObjectField(data json.RawMessage, key string) (json.RawMessage, error) {
	boards, err := pluckJSONField(data, "boards")
	if err != nil {
		return nil, err
	}
	return pluckFirstFromArrayObject(boards, key)
}

func pluckFirstBoardArrayField(data json.RawMessage, key string) ([]json.RawMessage, error) {
	raw, err := pluckFirstBoardObjectField(data, key)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" || len(raw) == 0 {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing %s array: %w", key, err)
	}
	return items, nil
}

func addBoardID(raw json.RawMessage, boardID string) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}
	obj["board_id"] = boardID
	data, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return data
}

func addBoardScopedID(raw json.RawMessage, boardID string) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}
	if id, ok := obj["id"]; ok {
		obj["original_id"] = id
		obj["id"] = fmt.Sprintf("%s:%v", boardID, id)
	}
	obj["board_id"] = boardID
	data, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return data
}

func escapeJSONString(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
