package strategyexplain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/strategysnapshot"
	"go-stock/backend/usagemetrics"
)

// SnapshotReader loads StrategySnapshot records (usually strategysnapshot.Service).
type SnapshotReader interface {
	Get(snapshotID string) (*strategysnapshot.StrategySnapshot, error)
	GetPlanReference(planID uint) (*strategysnapshot.PlanReference, error)
}

// Service renders read-only Explanations for TradePlan / Observation UI.
type Service interface {
	Explain(ctx context.Context, req ExplainRequest) (*Explanation, error)
}

type service struct {
	snaps SnapshotReader
}

// NewService wires a SnapshotReader (defaults to strategysnapshot.Default()).
func NewService(snaps SnapshotReader) Service {
	if snaps == nil {
		snaps = strategysnapshot.Default()
	}
	return &service{snaps: snaps}
}

func (s *service) Explain(ctx context.Context, req ExplainRequest) (*Explanation, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	user := resolveUser(req)
	gate := featuregate.CanAccess(user, featuregate.FeatureAdvancedObservation)
	if !gate.Allowed {
		return &Explanation{
			ExplanationID: fmt.Sprintf("expl:gated:%d", now.UnixNano()),
			SchemaVersion: SchemaVersion,
			GeneratedAt:   now,
			Status:        StatusGated,
			GateReason:    string(gate.Reason),
			Headline:      "高级策略解释未授权",
			Disclaimers:   defaultDisclaimers(),
		}, nil
	}

	snap, err := s.resolveSnapshot(req)
	if err != nil {
		if errors.Is(err, strategysnapshot.ErrNotFound) || isMissing(err) {
			return &Explanation{
				ExplanationID: fmt.Sprintf("expl:missing:%d", now.UnixNano()),
				SchemaVersion: SchemaVersion,
				SnapshotID:    strings.TrimSpace(req.SnapshotID),
				PlanID:        req.PlanID,
				PlanItemID:    req.PlanItemID,
				GeneratedAt:   now,
				Status:        StatusMissing,
				Headline:      "该计划创建时未保存策略快照，无法恢复完整解释",
				Disclaimers:   defaultDisclaimers(),
			}, nil
		}
		return &Explanation{
			ExplanationID: fmt.Sprintf("expl:failed:%d", now.UnixNano()),
			SchemaVersion: SchemaVersion,
			GeneratedAt:   now,
			Status:        StatusFailed,
			Headline:      "解释生成失败",
			Disclaimers:   defaultDisclaimers(),
		}, err
	}

	signal := renderSignal(snap.SignalResult)
	risk := renderRisk(snap.RiskDecision)
	entry := renderEntry(snap, req.EntryReasonHint, req.EntryRuleHint)
	exit := renderExit(req.ExitOverlay, req.IncludeExit)

	status := StatusOK
	if !signal.Available || !risk.Available || snap.MarketDataRef.Unavailable {
		status = StatusDegraded
	}

	citations := []Citation{{
		Kind:  "snapshot",
		Ref:   snap.SnapshotID,
		Label: "StrategySnapshot",
	}}
	if signal.Available && signal.SnapshotRef > 0 {
		citations = append(citations, Citation{
			Kind:  "signal_snapshot",
			Ref:   fmt.Sprintf("%d", signal.SnapshotRef),
			Label: "SignalScanSnapshot",
		})
	}
	if exit != nil {
		for _, c := range exit.ReasonCodes {
			citations = append(citations, Citation{Kind: "exit_code", Ref: c})
		}
	}

	out := &Explanation{
		ExplanationID: fmt.Sprintf("expl:%s:%d", snap.SnapshotID, now.UnixNano()),
		SchemaVersion: SchemaVersion,
		SnapshotID:    snap.SnapshotID,
		PlanID:        snap.PlanID,
		PlanItemID:    snap.PlanItemID,
		TradeDate:     snap.TradeDate,
		GeneratedAt:   now,
		Status:        status,
		Headline:      buildHeadline(signal, risk, entry),
		Sections: ExplanationSections{
			Signal: signal,
			Risk:   risk,
			Entry:  entry,
			Exit:   exit,
		},
		Citations:   citations,
		Disclaimers: defaultDisclaimers(),
	}

	// Usage: Shell/Observation may also record opened; here record successful generation.
	_, _ = usagemetrics.RecordFeatureEvent(ctx, user, featuregate.FeatureAdvancedObservation, usagemetrics.EventExecuted, map[string]string{
		"scene":       "strategy_explanation",
		"snapshot_id": snap.SnapshotID,
		"status":      status,
	})

	return out, nil
}

func resolveUser(req ExplainRequest) *featuregate.User {
	if req.User != nil {
		return req.User
	}
	id := strings.TrimSpace(req.UserID)
	if id == "" {
		id = "shell:explain"
	}
	tier := featuregate.NormalizeTier(featuregate.Tier(req.Tier))
	user := &featuregate.User{ID: id, Tier: tier}
	_ = entitlement.Default().EnsureTierDefaults(user)
	return user
}

func (s *service) resolveSnapshot(req ExplainRequest) (*strategysnapshot.StrategySnapshot, error) {
	if id := strings.TrimSpace(req.SnapshotID); id != "" {
		return s.snaps.Get(id)
	}
	if req.PlanID == 0 {
		return nil, strategysnapshot.ErrNotFound
	}
	ref, err := s.snaps.GetPlanReference(req.PlanID)
	if err != nil {
		return nil, err
	}
	if req.PlanItemID != 0 {
		if sid, ok := ref.ItemSnapshotIDs[req.PlanItemID]; ok && sid != "" {
			return s.snaps.Get(sid)
		}
		return nil, strategysnapshot.ErrNotFound
	}
	if ref.PlanSnapshotID != "" {
		return s.snaps.Get(ref.PlanSnapshotID)
	}
	return nil, strategysnapshot.ErrNotFound
}

func isMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}

// --- process default for UI hooks ---

var (
	defaultMu  sync.RWMutex
	defaultSvc Service = NewService(nil)
)

// Default returns the process-wide explanation service.
func Default() Service {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultSvc
}

// SetDefault replaces the process service (tests). Pass nil to reset.
func SetDefault(svc Service) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if svc == nil {
		defaultSvc = NewService(nil)
		return
	}
	defaultSvc = svc
}
