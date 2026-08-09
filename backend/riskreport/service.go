package riskreport

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/usagemetrics"
)

// Service builds read-only Advanced Risk Reports.
type Service interface {
	Build(ctx context.Context, req BuildRequest) (*RiskReport, error)
}

type service struct{}

// NewService returns the Risk Report service.
func NewService() Service { return &service{} }

func (s *service) Build(ctx context.Context, req BuildRequest) (*RiskReport, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	user := resolveUser(req)
	gate := featuregate.CanAccess(user, featuregate.FeatureAdvancedRisk)
	if !gate.Allowed {
		return &RiskReport{
			ReportID:      fmt.Sprintf("riskrpt:gated:%d", now.UnixNano()),
			SchemaVersion: SchemaVersion,
			UserID:        user.ID,
			TradeDate:     req.TradeDate,
			GeneratedAt:   now,
			Status:        StatusGated,
			GateReason:    string(gate.Reason),
			Score:         RiskScore{Band: BandUnknown, ByDimension: map[string]int{}},
			Disclaimers:   defaultDisclaimers(),
			DataQuality:   "UNKNOWN",
		}, nil
	}

	var (
		src   ReportSources
		refs  []SourceRef
		scope string
		err   error
	)
	if req.Sources != nil {
		src = *req.Sources
		refs = []SourceRef{{Kind: "injected", Label: "test_or_preloaded_sources"}}
		scope = "injected"
	} else {
		src, refs, scope, err = FetchLiveSources(req.TradeDate)
		if err != nil {
			return &RiskReport{
				ReportID:      fmt.Sprintf("riskrpt:failed:%d", now.UnixNano()),
				SchemaVersion: SchemaVersion,
				UserID:        user.ID,
				TradeDate:     req.TradeDate,
				GeneratedAt:   now,
				Status:        StatusFailed,
				Score:         RiskScore{Band: BandUnknown, ByDimension: map[string]int{}},
				Disclaimers:   defaultDisclaimers(),
				DataQuality:   "UNKNOWN",
			}, err
		}
	}

	score, factors, warns, sugs, dims, quality := aggregate(src)
	status := StatusOK
	if quality == "PARTIAL" || quality == "UNKNOWN" {
		status = StatusDegraded
	}
	if quality == "UNKNOWN" && len(factors) == 0 {
		status = StatusDegraded
	}

	out := &RiskReport{
		ReportID:      fmt.Sprintf("riskrpt:%s:%d", strings.TrimSpace(req.TradeDate), now.UnixNano()),
		SchemaVersion: SchemaVersion,
		UserID:        user.ID,
		TradeDate:     req.TradeDate,
		AccountScope:  scope,
		GeneratedAt:   now,
		Status:        status,
		Score:         score,
		Factors:       factors,
		Warnings:      warns,
		Suggestions:   sugs,
		Dimensions:    dims,
		Sources:       refs,
		DataQuality:   quality,
		Disclaimers:   defaultDisclaimers(),
	}

	_, _ = usagemetrics.RecordFeatureEvent(ctx, user, featuregate.FeatureAdvancedRisk, usagemetrics.EventExecuted, map[string]string{
		"scene":  "advanced_risk_report",
		"status": status,
		"band":   score.Band,
	})
	return out, nil
}

func resolveUser(req BuildRequest) *featuregate.User {
	id := strings.TrimSpace(req.UserID)
	if id == "" {
		id = "shell:riskreport"
	}
	tier := featuregate.NormalizeTier(featuregate.Tier(req.Tier))
	user := &featuregate.User{ID: id, Tier: tier}
	_ = entitlement.Default().EnsureTierDefaults(user)
	return user
}

var (
	defaultMu  sync.RWMutex
	defaultSvc Service = NewService()
)

// Default returns the process-wide Risk Report service.
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
		defaultSvc = NewService()
		return
	}
	defaultSvc = svc
}
