// Trello cross-board analytics helpers. Hand-authored (not generated).
// Shared by the novel commands: overdue, workload, velocity, cycletime,
// bottleneck, blocked, churn, checklist-progress.
//
// These read the local SQLite mirror (`resources` table, keyed by
// (resource_type, id), `data` column = raw Trello JSON) and compute joins
// and time-windowed aggregations that no single Trello API call provides.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
)

// trelloCard is the subset of Trello's card JSON the analytics commands read.
type trelloCard struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Desc             string   `json:"desc"`
	Closed           bool     `json:"closed"`
	Due              string   `json:"due"`
	DueComplete      bool     `json:"dueComplete"`
	DateLastActivity string   `json:"dateLastActivity"`
	IDList           string   `json:"idList"`
	IDBoard          string   `json:"idBoard"`
	IDMembers        []string `json:"idMembers"`
	IDLabels         []string `json:"idLabels"`
	Labels           []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"labels"`
}

// trelloAction is the subset of Trello's action (activity log) JSON used by the
// time-series commands (velocity, cycletime, bottleneck, churn).
type trelloAction struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Date   string `json:"date"`
	IDCard string `json:"-"`
	Data   struct {
		Card struct {
			ID string `json:"id"`
		} `json:"card"`
		Board struct {
			ID string `json:"id"`
		} `json:"board"`
		ListBefore struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"listBefore"`
		ListAfter struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"listAfter"`
		List struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	} `json:"data"`
}

var cardResourceTypes = []string{"cards", "boards_cards", "lists_cards", "members_cards"}
var actionResourceTypes = []string{"actions", "boards_actions", "cards_actions", "members_actions"}

// loadCards reads every card-shaped row from the local store. Tolerant: any row
// whose JSON has idBoard+idList+id is treated as a card even under an
// unexpected resource_type.
func loadCards(db *store.Store) ([]trelloCard, error) {
	rows, err := db.Query(`SELECT resource_type, data FROM resources`)
	if err != nil {
		return nil, fmt.Errorf("querying cards: %w", err)
	}
	defer rows.Close()

	known := map[string]bool{}
	for _, t := range cardResourceTypes {
		known[t] = true
	}
	seen := map[string]bool{}
	var cards []trelloCard
	for rows.Next() {
		var resType string
		var data []byte
		if rows.Scan(&resType, &data) != nil {
			continue
		}
		var c trelloCard
		if json.Unmarshal(data, &c) != nil {
			continue
		}
		isCard := known[resType] || (c.ID != "" && c.IDBoard != "" && c.IDList != "")
		if !isCard || c.ID == "" || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		cards = append(cards, c)
	}
	return cards, nil
}

// loadActions reads action-shaped rows from the store, normalizing the card id.
func loadActions(db *store.Store) ([]trelloAction, error) {
	rows, err := db.Query(`SELECT resource_type, data FROM resources`)
	if err != nil {
		return nil, fmt.Errorf("querying actions: %w", err)
	}
	defer rows.Close()

	known := map[string]bool{}
	for _, t := range actionResourceTypes {
		known[t] = true
	}
	seen := map[string]bool{}
	var actions []trelloAction
	for rows.Next() {
		var resType string
		var data []byte
		if rows.Scan(&resType, &data) != nil {
			continue
		}
		var a trelloAction
		if json.Unmarshal(data, &a) != nil {
			continue
		}
		isAction := known[resType] || (a.ID != "" && a.Type != "" && a.Date != "")
		if !isAction || a.ID == "" || seen[a.ID] {
			continue
		}
		a.IDCard = a.Data.Card.ID
		seen[a.ID] = true
		actions = append(actions, a)
	}
	return actions, nil
}

// nameLookup resolves display names from the local store for the given resource
// types, falling back to the raw id when the parent was not synced.
func nameLookup(db *store.Store, resourceTypes []string) map[string]string {
	out := map[string]string{}
	for _, rt := range resourceTypes {
		rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = ?`, rt)
		if err != nil {
			continue
		}
		for rows.Next() {
			var data []byte
			if rows.Scan(&data) != nil {
				continue
			}
			var m map[string]any
			if json.Unmarshal(data, &m) != nil {
				continue
			}
			id, _ := m["id"].(string)
			if id == "" {
				continue
			}
			for _, k := range []string{"name", "fullName", "displayName", "username"} {
				if v, ok := m[k]; ok && v != nil {
					if s := fmt.Sprintf("%v", v); s != "" {
						out[id] = s
						break
					}
				}
			}
		}
		rows.Close()
	}
	return out
}

func resolve(lookup map[string]string, id string) string {
	if id == "" {
		return ""
	}
	if n, ok := lookup[id]; ok && n != "" {
		return n
	}
	return id
}

// parseTrelloTime parses Trello RFC3339 timestamps; zero+false when empty/bad.
func parseTrelloTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// nowUTC is overridable in tests.
var nowUTC = func() time.Time { return time.Now().UTC() }

func sortByCountDesc[T any](items []T, count func(T) int) {
	sort.SliceStable(items, func(i, j int) bool { return count(items[i]) > count(items[j]) })
}
