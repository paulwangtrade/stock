package assistant

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/riskreport"
	"go-stock/backend/strategysnapshot"
	"go-stock/backend/usagemetrics"
)

// Service exposes gated Context building for Shell / future Analyze.
type Service interface {
	BuildContext(ctx context.Context, req ContextRequest) (*AssistantContext, error)
}

// ContextRequest is the Shell-facing build request.
type ContextRequest struct {
	User       *featuregate.User
	UserID     string
	Tier       string
	Scene      AssistantScene
	StockCode  string
	StockName  string
	Plan       *models.TradePlan
	Snapshot   *strategysnapshot.StrategySnapshot
	RiskReport *riskreport.RiskReport
	Execution  *papertrading.ExecutionSummaryView
	Now        time.Time
}

type service struct {
	builder ContextBuilder
}

// NewService returns an Assistant service with ContextBuilder.
func NewService(builder ContextBuilder) Service {
	if builder == nil {
		builder = NewContextBuilder()
	}
	return &service{builder: builder}
}

func (s *service) BuildContext(ctx context.Context, req ContextRequest) (*AssistantContext, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	user := resolveUser(req)
	gate := featuregate.CanAccess(user, featuregate.FeatureAIAnalysis)
	if !gate.Allowed {
		return &AssistantContext{
			SchemaVersion:  ContextSchemaVersion,
			Scene:          req.Scene,
			BuiltAt:        now,
			Status:         "gated",
			GateReason:     string(gate.Reason),
			Facts:          FactBundle{Disclaimers: defaultDisclaimers()},
			PromptSkeleton: "SYSTEM:\nAccess denied for AIAnalysis. Do not call external AI.\n",
		}, nil
	}

	out, err := s.builder.Build(BuildInput{
		Scene:      req.Scene,
		StockCode:  req.StockCode,
		StockName:  req.StockName,
		Now:        now,
		Plan:       req.Plan,
		Snapshot:   req.Snapshot,
		RiskReport: req.RiskReport,
		Execution:  req.Execution,
	})
	if err != nil {
		return &AssistantContext{
			SchemaVersion: ContextSchemaVersion,
			Scene:         req.Scene,
			BuiltAt:       now,
			Status:        "failed",
			Facts:         FactBundle{Disclaimers: defaultDisclaimers()},
		}, err
	}

	_, _ = usagemetrics.RecordFeatureEvent(ctx, user, featuregate.FeatureAIAnalysis, usagemetrics.EventContextGenerated, map[string]string{
		"scene":    string(out.Scene),
		"status":   out.Status,
		"degraded": fmt.Sprintf("%v", out.Degraded),
	})
	return out, nil
}

func resolveUser(req ContextRequest) *featuregate.User {
	if req.User != nil {
		return req.User
	}
	id := strings.TrimSpace(req.UserID)
	if id == "" {
		id = "shell:assistant"
	}
	user := &featuregate.User{ID: id, Tier: featuregate.NormalizeTier(featuregate.Tier(req.Tier))}
	_ = entitlement.Default().EnsureTierDefaults(user)
	return user
}

var (
	defaultMu  sync.RWMutex
	defaultSvc Service = NewService(nil)
)

// Default returns the process-wide assistant service.
func Default() Service {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultSvc
}

// SetDefault replaces the process service (tests).
func SetDefault(svc Service) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if svc == nil {
		defaultSvc = NewService(nil)
		return
	}
	defaultSvc = svc
}
