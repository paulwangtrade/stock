package assistant

import "time"

// ResponseSchemaVersion is the AssistantResponse contract (Phase13-F).
const ResponseSchemaVersion = "assist-resp-1"

const (
	ResponseStatusOK       = "ok"
	ResponseStatusFailed   = "failed"
	ResponseStatusDegraded = "degraded"
	ResponseStatusEmpty    = "empty"
	ResponseStatusGated    = "gated"
)

// AssistantResponse is the Analysis Result from an AIProvider.
// It may only explain / summarize / aid understanding — never trade instructions.
type AssistantResponse struct {
	SchemaVersion string         `json:"schema_version"`
	RequestID     string         `json:"request_id,omitempty"`
	Scene         AssistantScene `json:"scene"`
	Status        string         `json:"status"`
	Title         string         `json:"title"`
	Summary       string         `json:"summary"`
	Body          string         `json:"body"`
	Bullets       []string       `json:"bullets,omitempty"`
	Citations     []Citation     `json:"citations,omitempty"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Provider      string         `json:"provider"`
	Confidence    string         `json:"confidence,omitempty"` // low|medium|high|unknown
	Disclaimers   []string       `json:"disclaimers"`
	ErrorMessage  string         `json:"error_message,omitempty"`
}

// Citation references a fact source used in the explanation (read-only).
type Citation struct {
	Kind  string `json:"kind"`
	Ref   string `json:"ref,omitempty"`
	Label string `json:"label,omitempty"`
}

// DefaultResponseDisclaimers are always attached to AssistantResponse.
func DefaultResponseDisclaimers() []string {
	return []string{
		"本内容仅供解释、总结与归纳，不构成投资建议或买卖指令。",
		"AI 不可替代交易决策；不会修改 TradePlan / Strategy / Execution，也不会生成或提交任何订单。",
	}
}

