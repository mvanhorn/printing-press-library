// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

// Source is a notebook source summary.
type Source struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// NotebookDetail includes sources for a single notebook.
type NotebookDetail struct {
	Notebook
	Sources []Source `json:"sources"`
}

// ChatCitation references a source in a chat answer.
type ChatCitation struct {
	SourceID  string  `json:"source_id"`
	CitedText string  `json:"cited_text,omitempty"`
	Number    int     `json:"number,omitempty"`
	Score     float64 `json:"score,omitempty"`
}

// ChatResult is a completed chat query response.
type ChatResult struct {
	Answer     string         `json:"answer"`
	Citations  []ChatCitation `json:"citations,omitempty"`
	NotebookID string         `json:"notebook_id"`
}

// Artifact is a Studio artifact summary.
type Artifact struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Status string `json:"status,omitempty"`
}

// ShareStatus is notebook sharing configuration.
type ShareStatus struct {
	NotebookID string `json:"notebook_id"`
	Public     bool   `json:"public"`
	Users      int    `json:"users,omitempty"`
}

// AccountInfo from user settings / session.
type AccountInfo struct {
	Tier           string `json:"tier,omitempty"`
	OutputLanguage string `json:"output_language,omitempty"`
}

// ConversationTurn is one Q&A pair from history.
type ConversationTurn struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}
