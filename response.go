package main

type ChatCompletionChunk struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	SystemFingerprint string                 `json:"system_fingerprint"`
}

type ChatCompletionChoice struct {
	Index        int              `json:"index"`
	Delta        ChatMessageDelta `json:"delta,omitempty"`
	FinishReason string           `json:"finish_reason,omitempty"`
}

type ChatMessageDelta struct {
	Role      string             `json:"role,omitempty"`
	Content   string             `json:"content,omitempty"`
	ToolCalls []ToolCallResponse `json:"tool_calls,omitempty"`
}

type ToolCallResponse struct {
	Index    int              `json:"index"`
	Id       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}
