// Exit Policy — read-only re-assessment thresholds (Phase10-D.2.6).
//
// Configures Exit Evaluation observation only. Never produces sell actions.
// Not wired into Broker / Gateway / Execution / Order / Fill / Position writes.

package papertrading

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExitPolicy is the product-facing threshold catalog entry for Exit Evaluation.
type ExitPolicy struct {
	PolicyID             string  `json:"policy_id"`
	Version              int     `json:"version"`
	MaxHoldingDays       int     `json:"max_holding_days"`
	LossWatchThreshold   float64 `json:"loss_watch_threshold"`  // ratio; default -0.05
	LossReviewThreshold  float64 `json:"loss_review_threshold"` // ratio; default -0.10
	DisplayName          string  `json:"display_name,omitempty"`
	Notes                string  `json:"notes,omitempty"`
}

// DefaultExitPolicy is the built-in default_v1 (matches D.2.1 MVP behaviour).
var DefaultExitPolicy = ExitPolicy{
	PolicyID:            "default_v1",
	Version:             1,
	MaxHoldingDays:      20,
	LossWatchThreshold:  -0.05,
	LossReviewThreshold: -0.10,
	DisplayName:         "Default mid-swing",
	Notes:               "Exit Evaluation observation defaults; not a sell policy",
}

// Normalized fills missing / invalid fields from DefaultExitPolicy.
// Empty / zero policy must not panic and must fallback safely.
func (p ExitPolicy) Normalized() ExitPolicy {
	out := p
	if strings.TrimSpace(out.PolicyID) == "" {
		out.PolicyID = DefaultExitPolicy.PolicyID
	}
	if out.Version <= 0 {
		out.Version = DefaultExitPolicy.Version
	}
	if out.MaxHoldingDays <= 0 {
		out.MaxHoldingDays = DefaultExitPolicy.MaxHoldingDays
	}
	if out.LossWatchThreshold == 0 && out.LossReviewThreshold == 0 {
		out.LossWatchThreshold = DefaultExitPolicy.LossWatchThreshold
		out.LossReviewThreshold = DefaultExitPolicy.LossReviewThreshold
	} else {
		if out.LossWatchThreshold == 0 {
			out.LossWatchThreshold = DefaultExitPolicy.LossWatchThreshold
		}
		if out.LossReviewThreshold == 0 {
			out.LossReviewThreshold = DefaultExitPolicy.LossReviewThreshold
		}
	}
	// Keep review at least as severe as watch (more negative or equal).
	if out.LossReviewThreshold > out.LossWatchThreshold {
		out.LossReviewThreshold = out.LossWatchThreshold
	}
	return out
}

// ToEvaluationPolicy maps product ExitPolicy → legacy ExitEvaluationPolicy (compat).
func (p ExitPolicy) ToEvaluationPolicy() ExitEvaluationPolicy {
	n := p.Normalized()
	return ExitEvaluationPolicy{
		TimeReviewDays:       n.MaxHoldingDays,
		LossReviewReturn:     n.LossWatchThreshold,
		ReviewRequiredReturn: n.LossReviewThreshold,
	}
}

// ExitPolicyFromEvaluation maps legacy ExitEvaluationPolicy → ExitPolicy.
func ExitPolicyFromEvaluation(p ExitEvaluationPolicy) ExitPolicy {
	return ExitPolicy{
		PolicyID:            DefaultExitPolicy.PolicyID,
		Version:             DefaultExitPolicy.Version,
		MaxHoldingDays:      p.TimeReviewDays,
		LossWatchThreshold:  p.LossReviewReturn,
		LossReviewThreshold: p.ReviewRequiredReturn,
		DisplayName:         DefaultExitPolicy.DisplayName,
	}.Normalized()
}

// ExitPolicyRef is an audit-friendly snapshot attached to ExitEvaluationView.
type ExitPolicyRef struct {
	PolicyID            string  `json:"policy_id"`
	Version             int     `json:"version"`
	MaxHoldingDays      int     `json:"max_holding_days"`
	LossWatchThreshold  float64 `json:"loss_watch_threshold"`
	LossReviewThreshold float64 `json:"loss_review_threshold"`
}

// Ref returns the observation snapshot for a policy.
func (p ExitPolicy) Ref() ExitPolicyRef {
	n := p.Normalized()
	return ExitPolicyRef{
		PolicyID:            n.PolicyID,
		Version:             n.Version,
		MaxHoldingDays:      n.MaxHoldingDays,
		LossWatchThreshold:  n.LossWatchThreshold,
		LossReviewThreshold: n.LossReviewThreshold,
	}
}

func (p ExitPolicy) policyNote() string {
	n := p.Normalized()
	return fmt.Sprintf(
		"policy=%s@v%d; days>%d TIME_REVIEW; return<=%.0f%% LOSS_REVIEW+WATCH; return<=%.0f%% REVIEW_REQUIRED; plan superseded|failed|invalidated → PLAN_REVIEW (re-check only)",
		n.PolicyID, n.Version, n.MaxHoldingDays,
		n.LossWatchThreshold*100, n.LossReviewThreshold*100,
	)
}

// --- Provider ---

// ExitPolicyResolveContext is the optional resolve key (MVP mostly unused beyond default).
type ExitPolicyResolveContext struct {
	AccountID    uint
	StockCode    string
	PlanID       uint
	StrategyName string
	PolicyID     string // explicit catalog id when present
}

// ExitPolicyProvider resolves an ExitPolicy for Exit Evaluation (observation only).
type ExitPolicyProvider interface {
	Resolve(ctx ExitPolicyResolveContext) ExitPolicy
}

// BuiltinExitPolicyProvider always returns DefaultExitPolicy.
type BuiltinExitPolicyProvider struct{}

// Resolve implements ExitPolicyProvider.
func (BuiltinExitPolicyProvider) Resolve(ctx ExitPolicyResolveContext) ExitPolicy {
	_ = ctx
	return DefaultExitPolicy.Normalized()
}

// CatalogExitPolicyProvider resolves from an in-memory / JSON-loaded catalog.
// Priority: explicit PolicyID → strategy_name map → default_policy_id → DefaultExitPolicy.
type CatalogExitPolicyProvider struct {
	DefaultPolicyID string
	Policies        map[string]ExitPolicy
	StrategyToID    map[string]string // optional strategy_name → policy_id
}

// Resolve implements ExitPolicyProvider.
func (c *CatalogExitPolicyProvider) Resolve(ctx ExitPolicyResolveContext) ExitPolicy {
	if c == nil {
		return DefaultExitPolicy.Normalized()
	}
	if id := strings.TrimSpace(ctx.PolicyID); id != "" {
		if p, ok := c.Policies[id]; ok {
			return p.Normalized()
		}
	}
	if c.StrategyToID != nil {
		if sid := strings.TrimSpace(ctx.StrategyName); sid != "" {
			if pid, ok := c.StrategyToID[sid]; ok {
				if p, ok := c.Policies[pid]; ok {
					return p.Normalized()
				}
			}
		}
	}
	defID := strings.TrimSpace(c.DefaultPolicyID)
	if defID != "" {
		if p, ok := c.Policies[defID]; ok {
			return p.Normalized()
		}
	}
	return DefaultExitPolicy.Normalized()
}

// ExitPolicyCatalogJSON is the on-disk / in-memory JSON shape (no DB).
type ExitPolicyCatalogJSON struct {
	DefaultPolicyID string            `json:"default_policy_id"`
	Policies        []ExitPolicy      `json:"policies"`
	StrategyMap     map[string]string `json:"strategy_map,omitempty"`
}

// ParseExitPolicyCatalogJSON builds a CatalogExitPolicyProvider from JSON bytes.
// Invalid / empty catalog falls back to Builtin defaults (no panic).
func ParseExitPolicyCatalogJSON(raw []byte) (*CatalogExitPolicyProvider, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return &CatalogExitPolicyProvider{
			DefaultPolicyID: DefaultExitPolicy.PolicyID,
			Policies: map[string]ExitPolicy{
				DefaultExitPolicy.PolicyID: DefaultExitPolicy,
			},
		}, nil
	}
	var doc ExitPolicyCatalogJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("exit policy catalog: %w", err)
	}
	out := &CatalogExitPolicyProvider{
		DefaultPolicyID: strings.TrimSpace(doc.DefaultPolicyID),
		Policies:        map[string]ExitPolicy{},
		StrategyToID:    doc.StrategyMap,
	}
	for _, p := range doc.Policies {
		id := strings.TrimSpace(p.PolicyID)
		if id == "" {
			continue
		}
		out.Policies[id] = p.Normalized()
	}
	if out.DefaultPolicyID == "" {
		out.DefaultPolicyID = DefaultExitPolicy.PolicyID
	}
	if _, ok := out.Policies[out.DefaultPolicyID]; !ok {
		out.Policies[DefaultExitPolicy.PolicyID] = DefaultExitPolicy.Normalized()
		out.DefaultPolicyID = DefaultExitPolicy.PolicyID
	}
	return out, nil
}

var globalExitPolicyProvider ExitPolicyProvider = BuiltinExitPolicyProvider{}

// GetExitPolicyProvider returns the process-wide provider (default: Builtin).
func GetExitPolicyProvider() ExitPolicyProvider {
	if globalExitPolicyProvider == nil {
		return BuiltinExitPolicyProvider{}
	}
	return globalExitPolicyProvider
}

// SetExitPolicyProviderForTest replaces the process-wide provider (tests only).
func SetExitPolicyProviderForTest(p ExitPolicyProvider) {
	if p == nil {
		globalExitPolicyProvider = BuiltinExitPolicyProvider{}
		return
	}
	globalExitPolicyProvider = p
}

// ResetExitPolicyProviderForTest restores Builtin provider.
func ResetExitPolicyProviderForTest() {
	globalExitPolicyProvider = BuiltinExitPolicyProvider{}
}

// ResolveExitPolicy applies BuildOptions override order then Provider.
// Order: ExitPolicy override → legacy Policy override → Provider.Resolve → Default.
func ResolveExitPolicy(opts ExitEvaluationBuildOptions, resolveCtx ExitPolicyResolveContext) ExitPolicy {
	if opts.ExitPolicy != nil {
		return opts.ExitPolicy.Normalized()
	}
	if opts.Policy != nil {
		return ExitPolicyFromEvaluation(*opts.Policy)
	}
	provider := opts.PolicyProvider
	if provider == nil {
		provider = GetExitPolicyProvider()
	}
	return provider.Resolve(resolveCtx).Normalized()
}
