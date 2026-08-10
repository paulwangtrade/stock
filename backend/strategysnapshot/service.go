package strategysnapshot

import (
	"fmt"
	"sync"

	"go-stock/backend/models"
)

// Service is the read-only Strategy Snapshot application port.
// Implementations must not call Broker / Gateway / Execution / Fill / Settlement.
type Service interface {
	// CaptureAfterTradePlanCreate records plan+item snapshots and a PlanReference.
	CaptureAfterTradePlanCreate(in CaptureInput) (*CaptureResult, error)
	// CaptureAfterStrategyEvaluation is an alternate allowed trigger (same write path).
	CaptureAfterStrategyEvaluation(in CaptureInput) (*CaptureResult, error)

	Get(snapshotID string) (*StrategySnapshot, error)
	ListByPlan(planID uint) ([]StrategySnapshot, error)
	GetPlanReference(planID uint) (*PlanReference, error)
}

type service struct {
	store Store
}

// NewService returns a Strategy Snapshot service backed by store (default MemoryStore).
func NewService(store Store) Service {
	if store == nil {
		store = NewMemoryStore()
	}
	return &service{store: store}
}

func (s *service) CaptureAfterTradePlanCreate(in CaptureInput) (*CaptureResult, error) {
	in.Trigger = TriggerTradePlanCreate
	return s.capture(in)
}

func (s *service) CaptureAfterStrategyEvaluation(in CaptureInput) (*CaptureResult, error) {
	in.Trigger = TriggerStrategyEvaluation
	return s.capture(in)
}

func (s *service) capture(in CaptureInput) (*CaptureResult, error) {
	res, err := BuildSnapshots(in)
	if err != nil {
		return nil, err
	}
	if err := s.store.Save(res.PlanSnap); err != nil {
		return nil, err
	}
	for i := range res.ItemSnaps {
		if err := s.store.Save(&res.ItemSnaps[i]); err != nil {
			return nil, err
		}
	}
	if err := s.store.SavePlanRef(res.PlanRef); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) Get(snapshotID string) (*StrategySnapshot, error) {
	if snapshotID == "" {
		return nil, fmt.Errorf("strategysnapshot: snapshot id required")
	}
	return s.store.Get(snapshotID)
}

func (s *service) ListByPlan(planID uint) ([]StrategySnapshot, error) {
	if planID == 0 {
		return nil, fmt.Errorf("strategysnapshot: plan id required")
	}
	return s.store.ListByPlan(planID)
}

func (s *service) GetPlanReference(planID uint) (*PlanReference, error) {
	if planID == 0 {
		return nil, fmt.Errorf("strategysnapshot: plan id required")
	}
	return s.store.GetPlanRef(planID)
}

// --- process-wide default for best-effort hooks (tests may SetDefault) ---

var (
	defaultMu  sync.RWMutex
	defaultSvc Service = NewService(NewMemoryStore())
)

// Default returns the process-wide Strategy Snapshot service.
func Default() Service {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultSvc
}

// SetDefault replaces the process-wide service (tests). Pass nil to reset to a fresh MemoryStore.
func SetDefault(svc Service) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if svc == nil {
		defaultSvc = NewService(NewMemoryStore())
		return
	}
	defaultSvc = svc
}

// RecordAfterTradePlanCreate is a best-effort bypass hook.
// Callers must log failures and must NOT fail TradePlan persistence because of snapshot errors.
func RecordAfterTradePlanCreate(plan *models.TradePlan, pool *models.CandidatePool) (*CaptureResult, error) {
	return Default().CaptureAfterTradePlanCreate(CaptureInput{Plan: plan, Pool: pool})
}

// RecordAfterStrategyEvaluation records snapshots after strategy evaluation (allowed integration point).
func RecordAfterStrategyEvaluation(plan *models.TradePlan, pool *models.CandidatePool) (*CaptureResult, error) {
	return Default().CaptureAfterStrategyEvaluation(CaptureInput{Plan: plan, Pool: pool})
}
