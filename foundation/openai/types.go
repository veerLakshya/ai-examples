// Package openai defines the OpenAI-compatible HTTP API schema shared by
// Ollama, vLLM, OpenAI, and other compatible servers. Each struct
// models only the fields our examples actually use; will extend as later
// examples need more.
package openai

// Message is one turn in a chat conversation.
// Role is "user", "assistant" or "system"
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatChunk is one chunk of a streaming chat response (one SSE "data: {...}" payload)
// Delta carries only what changed since the previous chunk -
// accumulate Delta.Content across chunks to reconstruct the full reply.
type ChatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ChatRequest is a chat completions request body.
// optional fields use omitempty so callers leave them at zero values when
// unused (eg. Stream: false wont be serialized)
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

// ChatResponse is a non-streaming chat completion response.
// We only model choices[0].message.content; finish_reason, usage, etc.
// are dropped siletly by Decode.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
