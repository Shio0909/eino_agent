package service

import "time"

// RetrieveRequest carries pure retrieval input for external context providers.
type RetrieveRequest struct {
	Query            string   `json:"query"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	DocumentIDs      []string `json:"document_ids,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
}

// RetrieveResponse contains retrieved context without query-time LLM generation.
type RetrieveResponse struct {
	Query   string           `json:"query"`
	Total   int              `json:"total"`
	Results []RetrieveResult `json:"results"`
}

// RetrieveResult is one retrieved document or wiki page for MCP/tool consumers.
type RetrieveResult struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type,omitempty"`
	Title           string                 `json:"title,omitempty"`
	Content         string                 `json:"content"`
	Snippet         string                 `json:"snippet,omitempty"`
	KnowledgeBaseID string                 `json:"knowledge_base_id,omitempty"`
	KnowledgeID     string                 `json:"knowledge_id,omitempty"`
	WikiPath        string                 `json:"wiki_path,omitempty"`
	WikiPageType    string                 `json:"wiki_page_type,omitempty"`
	MatchType       string                 `json:"match_type,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ChatRequest carries user input and request-scoped runtime options.
type ChatRequest struct {
	Message           string   `json:"message"`
	SessionID         string   `json:"session_id"`
	UseAgent          bool     `json:"use_agent"`
	Mode              string   `json:"mode,omitempty"`
	TenantID          int      `json:"tenant_id,omitempty"`
	UserID            string   `json:"user_id,omitempty"`
	ForceCitation     bool     `json:"force_citation,omitempty"`
	KnowledgeBaseIDs  []string `json:"knowledge_base_ids,omitempty"`
	DocumentIDs       []string `json:"document_ids,omitempty"`
	RestrictRetrieval bool     `json:"restrict_retrieval,omitempty"`
	EnableLongTerm    *bool    `json:"enable_long_term,omitempty"`
	EnableSkills      *bool    `json:"enable_skills,omitempty"`
	SelectedSkills    []string `json:"selected_skills,omitempty"`
	ResolvedMode      string   `json:"resolved_mode,omitempty"`
}

// ChatResponse is the non-streaming chat payload returned to callers.
type ChatResponse struct {
	Answer    string      `json:"answer"`
	Sources   []Source    `json:"sources,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	TraceID   string      `json:"trace_id,omitempty"`
	Trace     []TraceStep `json:"trace,omitempty"`
}

// TraceStep captures one observable stage in a chat request.
type TraceStep struct {
	TraceID    string         `json:"trace_id,omitempty"`
	Seq        int            `json:"seq,omitempty"`
	Type       string         `json:"type"`
	Stage      string         `json:"stage,omitempty"`
	Level      string         `json:"level,omitempty"`
	Summary    string         `json:"summary,omitempty"`
	Content    string         `json:"content,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolInput  string         `json:"tool_input,omitempty"`
	DocID      string         `json:"doc_id,omitempty"`
	LatencyMs  int64          `json:"latency_ms,omitempty"`
	TokenCount int            `json:"token_count,omitempty"`
	Error      string         `json:"error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// StreamEvent is the SSE payload sent during streaming chat responses.
type StreamEvent struct {
	Type           string         `json:"type"`
	Content        string         `json:"content,omitempty"`
	DocID          string         `json:"doc_id,omitempty"`
	Error          string         `json:"error,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	TraceID        string         `json:"trace_id,omitempty"`
	ResolvedMode   string         `json:"resolved_mode,omitempty"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolInput      string         `json:"tool_input,omitempty"`
	ApprovalID     string         `json:"approval_id,omitempty"`
	ApprovalStatus string         `json:"approval_status,omitempty"`
	Action         string         `json:"action,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Sources        []Source       `json:"sources,omitempty"`
	LatencyMs      int64          `json:"latency_ms,omitempty"`
	SourceCount    int            `json:"source_count,omitempty"`
	RetryCount     int            `json:"retry_count,omitempty"`
	TraceStep      *TraceStep     `json:"trace_step,omitempty"`
	TraceSnapshot  []TraceStep    `json:"trace_snapshot,omitempty"`
}
