// Package provider defines the AI Provider adapter for Assistant analysis.
//
// Phase13-F: AIProvider abstraction + MockAIProvider only.
// Allowed: explain / summarize / induce understanding from AssistantContext.
// Forbidden: real AI HTTP APIs, uploading user data, generating trade orders,
// mutating TradePlan / Strategy / Execution.
package provider

import (
	"context"

	"go-stock/backend/assistant"
)

// AIProvider turns a read-only AssistantContext into an explanation response.
// Implementations must only explain / summarize / induce — never trade.
// Commercial Feature remains featuregate.FeatureAIAnalysis (gated at Service/UI).
type AIProvider interface {
	// Name identifies the provider implementation (e.g. "mock").
	Name() string
	// Analyze produces an AssistantResponse from AssistantContext.
	// Must not call external services, upload user data, or emit buy/sell/order instructions.
	Analyze(ctx context.Context, ac *assistant.AssistantContext) (*assistant.AssistantResponse, error)
}
