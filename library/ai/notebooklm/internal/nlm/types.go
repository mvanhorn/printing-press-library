// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

// RPC method IDs reverse-engineered from notebooklm-py (teng-lin/notebooklm-py).
const (
	RPCListNotebooks         = "wXbhsf"
	RPCCreateNotebook        = "CCqFvf"
	RPCGetNotebook           = "rLM1Ne"
	RPCRenameNotebook        = "s0tc2d"
	RPCDeleteNotebook        = "WWINqb"
	RPCAddSource             = "izAoDd"
	RPCAddSourceFile         = "o4cbdc"
	RPCDeleteSource          = "tGMBJ"
	RPCGetSource             = "hizoJc"
	RPCRefreshSource         = "FLmJqe"
	RPCCheckSourceFreshness  = "yR9Yof"
	RPCUpdateSource          = "b7Wfje"
	RPCCreateLabel           = "agX4Bc"
	RPCListLabels            = "I3xc3c"
	RPCUpdateLabel           = "le8sX"
	RPCDeleteLabel           = "GyzE7e"
	RPCSummarize             = "VfAZjd"
	RPCGetSourceGuide        = "tr032e"
	RPCGetSuggestedReports   = "ciyUvf"
	RPCCreateArtifact        = "R7cb6c"
	RPCListArtifacts         = "gArtLc"
	RPCDeleteArtifact        = "V5N4be"
	RPCRenameArtifact        = "rc3d8d"
	RPCExportArtifact        = "Krh3pd"
	RPCShareArtifact         = "RGP97b"
	RPCGetInteractiveHTML    = "v9rmvd"
	RPCReviseSlide           = "KmcKPe"
	RPCRetryArtifact         = "Rytqqe"
	RPCStartFastResearch     = "Ljjv0c"
	RPCStartDeepResearch     = "QA9ei"
	RPCPollResearch          = "e3bVqc"
	RPCImportResearch        = "LBwxtb"
	RPCCancelResearch        = "Zbrupe"
	RPCGenerateMindMap       = "yyryJe"
	RPCCreateNote            = "CYK0Xb"
	RPCGetNotesAndMindMaps   = "cFji9"
	RPCUpdateNote            = "cYAfTb"
	RPCDeleteNote            = "AH0mwd"
	RPCGetLastConversationID = "hPTbtc"
	RPCGetConversationTurns  = "khqZz"
	RPCDeleteConversation    = "J7Gthc"
	RPCSuggestPrompts        = "otmP3b"
	RPCShareNotebook         = "QDyure"
	RPCGetShareStatus        = "JFMDGd"
	RPCRemoveRecentlyViewed  = "fejl7e"
	RPCGetUserSettings       = "ZwVcOc"
	RPCSetUserSettings       = "hT54vc"
)

const (
	BatchExecutePath = "/_/LabsTailwindUi/data/batchexecute"
	BaseURL          = "https://notebooklm.google.com"
)

// Notebook is a lightweight notebook summary.
type Notebook struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Emoji       string `json:"emoji,omitempty"`
	SourceCount int    `json:"source_count"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}
