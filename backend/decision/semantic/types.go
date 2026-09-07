// Package semantic provides QuantDecision semantic normalization (Phase2-B0),
// batch Shadow Stability Report (Phase2-C), and Historical Shadow Replay (Phase2-D).
//
// Boundaries:
//   - Does not modify js_legacy producer / UI / TradePlan / Execution
//   - Action.Code is the highest-priority semantic field
//   - Action.Label is display-only (does not affect SemanticEqual)
//   - Phase2-D harness only: fixed Signal/Gate/Zone/Size fixtures → daily reports + Action transition matrix
package semantic

// NormalizeSlices semantic core slices.
var NormalizeSlices = []string{"signal", "entryZone", "gate", "risk", "size", "action"}

// SemanticAction Action 语义核（无 Label）。
type SemanticAction struct {
	Code       string `json:"code"`
	AllowDraft bool   `json:"allowDraft"`
	Side       string `json:"side"`
}

// SemanticGateItem gate item without display label.
type SemanticGateItem struct {
	ID       string `json:"id"`
	Passed   bool   `json:"passed"`
	Required bool   `json:"required"`
}

// SemanticDecision 归一化后的语义快照。
type SemanticDecision struct {
	Signal    map[string]any `json:"signal"`
	EntryZone map[string]any `json:"entryZone"`
	Gate      map[string]any `json:"gate"`
	Risk      map[string]any `json:"risk"`
	Size      map[string]any `json:"size"`
	Action    SemanticAction `json:"action"`
}

// DisplayDiff 展示层差异（Label 等）。
type DisplayDiff struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Left  any    `json:"left"`
	Right any    `json:"right"`
}

// Diff 语义字段差异。
type Diff struct {
	Path     string `json:"path"`
	Slice    string `json:"slice,omitempty"`
	Kind     string `json:"kind"`
	Priority string `json:"priority,omitempty"`
	Left     any    `json:"left"`
	Right    any    `json:"right"`
}

// Report 语义比较报告。
type Report struct {
	SemanticEqual   bool              `json:"semanticEqual"`
	ActionCodeEqual bool              `json:"actionCodeEqual"`
	DisplayDiffs    []DisplayDiff     `json:"displayDiffs"`
	SemanticDiffs   []Diff            `json:"semanticDiffs"`
	BySlice         map[string]SliceR `json:"bySlice"`
	LeftActionCode  string            `json:"leftActionCode"`
	RightActionCode string            `json:"rightActionCode"`
	LeftProducer    string            `json:"leftProducer"`
	RightProducer   string            `json:"rightProducer"`
	Summary         string            `json:"summary"`
}

// SliceR per-slice result.
type SliceR struct {
	Equal bool   `json:"equal"`
	Diffs []Diff `json:"diffs"`
}
