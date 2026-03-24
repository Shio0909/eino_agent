package service

// ChatRequest carries user input and request-scoped runtime options.
type ChatRequest struct {
	Message          string   `json:"message"`
	SessionID        string   `json:"session_id"`
	UseAgent         bool     `json:"use_agent"`
	Mode             string   `json:"mode,omitempty"`
	TenantID         int      `json:"tenant_id,omitempty"`
	UserID           string   `json:"user_id,omitempty"`
	ForceCitation    bool     `json:"force_citation,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	DocumentIDs      []string `json:"document_ids,omitempty"`
	EnableLongTerm   *bool    `json:"enable_long_term,omitempty"`
	EnableSkills     *bool    `json:"enable_skills,omitempty"`
	SelectedSkills   []string `json:"selected_skills,omitempty"`
	ResolvedMode     string   `json:"resolved_mode,omitempty"`
}

// ChatResponse is the non-streaming chat payload returned to callers.
type ChatResponse struct {
	Answer    string   `json:"answer"`
	Sources   []Source `json:"sources,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

// StreamEvent is the SSE payload sent during streaming chat responses.
type StreamEvent struct {
	Type         string   `json:"type"`
	Content      string   `json:"content,omitempty"`
	DocID        string   `json:"doc_id,omitempty"`
	Error        string   `json:"error,omitempty"`
	SessionID    string   `json:"session_id,omitempty"`
	ResolvedMode string   `json:"resolved_mode,omitempty"`
	ToolName     string   `json:"tool_name,omitempty"`
	ToolInput    string   `json:"tool_input,omitempty"`
	Sources      []Source `json:"sources,omitempty"`
}
