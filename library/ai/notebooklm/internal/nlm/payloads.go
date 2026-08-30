// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

// TemplateBlock is the nested request-options wrapper required by migrated NotebookLM backends.
func TemplateBlock() []any {
	return []any{2, nil, nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}}
}

// NestSourceIDs wraps each source ID with depth inner lists (notebooklm-py nest_source_ids).
func NestSourceIDs(ids []string, depth int) []any {
	if depth < 1 || len(ids) == 0 {
		return []any{}
	}
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	for d := 0; d < depth; d++ {
		wrapped := make([]any, len(out))
		for i, item := range out {
			wrapped[i] = []any{item}
		}
		out = wrapped
	}
	return out
}

func artifactClientOptions() []any {
	return []any{
		2, nil, nil,
		[]any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}},
		[]any{[]any{1, 4, 8, 2, 3, 6}},
	}
}

// BuildCreateNotebookParams returns CREATE_NOTEBOOK (CCqFvf) params.
func BuildCreateNotebookParams(title string) []any {
	return []any{title, nil, nil, TemplateBlock()}
}

// BuildGetNotebookParams returns GET_NOTEBOOK (rLM1Ne) params.
func BuildGetNotebookParams(notebookID string) []any {
	return []any{notebookID, nil, TemplateBlock(), nil, 0}
}

// BuildDeleteNotebookParams returns DELETE_NOTEBOOK (WWINqb) params.
func BuildDeleteNotebookParams(notebookID string) []any {
	return []any{[]any{notebookID}, []int{2}}
}

// BuildRenameNotebookParams returns RENAME_NOTEBOOK (s0tc2d) params.
func BuildRenameNotebookParams(notebookID, title string) []any {
	return []any{notebookID, []any{[]any{nil, nil, nil, []any{nil, title}}}}
}

// BuildAddURLSourceParams returns ADD_SOURCE (izAoDd) params for a web URL.
func BuildAddURLSourceParams(notebookID, url string) []any {
	return []any{
		[]any{[]any{nil, nil, []any{url}, nil, nil, nil, nil, nil, nil, nil, 1}},
		notebookID,
		TemplateBlock(),
	}
}

// BuildDeleteSourceParams returns DELETE_SOURCE (tGMBJ) params.
func BuildDeleteSourceParams(sourceID string) []any {
	return []any{[]any{[]any{sourceID}}}
}

// BuildListArtifactsParams returns LIST_ARTIFACTS (gArtLc) params.
func BuildListArtifactsParams(notebookID string) []any {
	return []any{[]int{2}, notebookID, `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`}
}

// BuildQuizArtifactParams returns CREATE_ARTIFACT (R7cb6c) params for quiz generation.
func BuildQuizArtifactParams(notebookID string, sourceIDs []string, instructions string) []any {
	var instr any
	if instructions != "" {
		instr = instructions
	}
	return []any{
		artifactClientOptions(),
		notebookID,
		[]any{
			nil, nil, ArtifactTypeQuiz,
			NestSourceIDs(sourceIDs, 2),
			nil, nil, nil, nil, nil,
			[]any{
				nil,
				[]any{2, nil, instr, nil, nil, nil, nil, []any{1, 2}},
			},
		},
	}
}

// BuildShareStatusParams returns GET_SHARE_STATUS (JFMDGd) params.
func BuildShareStatusParams(notebookID string) []any {
	return []any{notebookID, []int{2}}
}

// BuildUserSettingsParams returns GET_USER_SETTINGS (ZwVcOc) params.
func BuildUserSettingsParams() []any {
	return []any{nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}}
}

// BuildConversationTurnsParams returns GET_CONVERSATION_TURNS (khqZz) params.
func BuildConversationTurnsParams(notebookID, conversationID string) []any {
	return []any{notebookID, conversationID, []int{2}}
}

// BuildLastConversationParams returns GET_LAST_CONVERSATION_ID (hPTbtc) params.
func BuildLastConversationParams(notebookID string) []any {
	return []any{notebookID, []int{2}}
}

const (
	ArtifactTypeQuiz       = 4
	ArtifactTypeFlashcards = 4
	ArtifactTypeAudio      = 1
	ArtifactTypeVideo      = 3
	ArtifactTypeReport     = 2
)

const QueryEndpointPath = "/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed"
