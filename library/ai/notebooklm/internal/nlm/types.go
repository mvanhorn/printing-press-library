package nlm

import "time"

type Notebook struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	SourceCount int       `json:"source_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type NotebookDetail struct {
	NotebookID  string `json:"notebook_id"`
	Title       string `json:"title"`
	SourceCount int    `json:"source_count"`
	URL         string `json:"url"`
	Sources     []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"sources"`
}

type NotebookGetResponse struct {
	Value NotebookDetail `json:"value"`
}

type NotebookDescribeResponse struct {
	Value struct {
		Summary         string   `json:"summary"`
		SuggestedTopics []string `json:"suggested_topics"`
	} `json:"value"`
}

type Source struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Type  string  `json:"type"`
	URL   *string `json:"url"`
}

type SourceContent struct {
	Value struct {
		Content    string  `json:"content"`
		Title      string  `json:"title"`
		SourceType string  `json:"source_type"`
		URL        string  `json:"url"`
		CharCount  int     `json:"char_count"`
	} `json:"value"`
}

type SourceDescribeResponse struct {
	Value struct {
		Summary  string   `json:"summary"`
		Keywords []string `json:"keywords"`
	} `json:"value"`
}

type QueryResponse struct {
	Value struct {
		Answer         string            `json:"answer"`
		ConversationID string            `json:"conversation_id"`
		SourcesUsed    []string          `json:"sources_used"`
		Citations      map[string]string `json:"citations"`
	} `json:"value"`
}

type CrossQueryResponse struct {
	Value struct {
		Answer         string            `json:"answer"`
		SourcesUsed    []string          `json:"sources_used"`
		Citations      map[string]string `json:"citations"`
		NotebookIDs    []string          `json:"notebook_ids"`
	} `json:"value"`
}

type Artifact struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type ResearchTask struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type Note struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}
