package anthropic

// Tool represents the structure of a tool in the Anthropic API.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Request represents a request to the Anthropic API.
type Request struct {
	Model       string         `json:"model"`
	MaxTokens   int            `json:"max_tokens"`
	Messages    []RequestMessage `json:"messages"`
	System      []SystemMessage  `json:"system,omitempty"`
	Tools       []Tool           `json:"tools,omitempty"`
	Stream      bool           `json:"stream"`
	Temperature *float64       `json:"temperature,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// StreamResponse represents a streaming response from the Anthropic API.
type StreamResponse struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentDelta struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"delta,omitempty"`
	Content []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

// RequestMessage represents a message in an Anthropic API request.
type RequestMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // Can be string or []ContentBlock
}

// SystemMessage represents a system message in an Anthropic API request.
type SystemMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ContentBlock represents a block of content in a message.
type ContentBlock struct {
	Type      string  `json:"type"`
	Text      *string `json:"text,omitempty"`
	ToolUseId *string `json:"tool_use_id,omitempty"`
	Content   *string `json:"content,omitempty"`
	Name      *string `json:"name,omitempty"`
	Input     *any    `json:"input,omitempty"`
}
