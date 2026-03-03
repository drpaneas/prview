package ai

import "context"

type Caller interface {
	Call(ctx context.Context, system, prompt string) (string, error)
}

const (
	DefaultAnthropicModel = "claude-sonnet-4-20250514"
	DefaultGeminiModel    = "gemini-2.5-flash"
)
