package execution

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

// ErrResearchTradeIntentNotExecutable Intent 非 confirmed，禁止进入 Execution。
var ErrResearchTradeIntentNotExecutable = errors.New("research trade intent not executable")

// ResearchTradeExecutor 研究人工买入 façade（Phase3-PR2）。
// 唯一下单出口：ExecutionService.ExecutePlanItem；不直连 PaperTrading。
type ResearchTradeExecutor interface {
	ConfirmAndExecuteResearchBuy(ctx context.Context, intentID uint) error
}

type researchIntentStore interface {
	GetResearchTradeIntent(id uint) (*models.ResearchTradeIntent, error)
	UpdateResearchTradeIntentStatus(id uint, toStatus string, patch *data.ResearchTradeIntentStatusPatch) error
}

// ResearchTradeFacade 将 ResearchTradeIntent 映射为临时 TradePlanItem 后走 ExecutionService。
type ResearchTradeFacade struct {
	svc  *ExecutionService
	repo researchIntentStore
}

// NewResearchTradeFacade 构造 façade；repo 为 nil 时使用默认 ResearchTradeIntentRepo。
func NewResearchTradeFacade(svc *ExecutionService, repo researchIntentStore) *ResearchTradeFacade {
	if repo == nil {
		repo = data.NewResearchTradeIntentRepo()
	}
	return &ResearchTradeFacade{svc: svc, repo: repo}
}

var _ ResearchTradeExecutor = (*ResearchTradeFacade)(nil)

// ConfirmAndExecuteResearchBuy 仅当 status=confirmed 时执行；成功→submitted，失败→rejected。
// 名称表示「已确认的研究买入执行」，不自动将 draft 升为 confirmed。
func (f *ResearchTradeFacade) ConfirmAndExecuteResearchBuy(ctx context.Context, intentID uint) error {
	if f == nil || f.svc == nil {
		return fmt.Errorf("research trade facade: nil ExecutionService")
	}
	if f.repo == nil {
		return fmt.Errorf("research trade facade: nil intent repo")
	}
	if intentID == 0 {
		return fmt.Errorf("research trade facade: intentID is required")
	}

	intent, err := f.repo.GetResearchTradeIntent(intentID)
	if err != nil {
		return err
	}
	if intent.Status != models.ResearchTradeIntentStatusConfirmed {
		return fmt.Errorf("%w: status=%s (only confirmed may execute)", ErrResearchTradeIntentNotExecutable, intent.Status)
	}

	item := mapResearchIntentToPlanItem(intent)
	opts := mapResearchIntentToExecOpts(intent)

	order, execErr := f.svc.ExecutePlanItem(ctx, item, opts)
	if execErr != nil {
		code := PreTradeRejectCode(execErr)
		if code == "" {
			code = data.PaperOrderRejectInternal
		}
		_ = f.repo.UpdateResearchTradeIntentStatus(intentID, models.ResearchTradeIntentStatusRejected, &data.ResearchTradeIntentStatusPatch{
			ErrorCode:    code,
			ErrorMessage: truncateErrMsg(execErr.Error(), 500),
		})
		return execErr
	}

	orderID := uint(0)
	clientOID := ""
	if order != nil {
		clientOID = order.ClientOrderID
		if id, perr := strconv.ParseUint(order.ID, 10, 64); perr == nil {
			orderID = uint(id)
		}
	}
	if err := f.repo.UpdateResearchTradeIntentStatus(intentID, models.ResearchTradeIntentStatusSubmitted, &data.ResearchTradeIntentStatusPatch{
		ClientOrderID: clientOID,
		OrderID:       orderID,
	}); err != nil {
		return fmt.Errorf("execution ok but intent status update failed: %w", err)
	}
	return nil
}

func mapResearchIntentToPlanItem(intent *models.ResearchTradeIntent) models.TradePlanItem {
	side := strings.TrimSpace(intent.Side)
	if side == "" {
		side = "buy"
	}
	reason := strings.TrimSpace(intent.Reason)
	if reason == "" {
		reason = buildResearchExecReason(intent)
	}
	return models.TradePlanItem{
		StockCode:    intent.Symbol,
		StockName:    intent.StockName,
		Side:         side,
		LimitPrice:   intent.Price,
		TargetVolume: intent.Volume,
		Reason:       reason,
		StrategyName: models.ResearchTradeStrategyTag,
		Score:        intent.SignalScore,
		Status:       models.TradePlanItemPending,
	}
}

func mapResearchIntentToExecOpts(intent *models.ResearchTradeIntent) ExecutePlanItemOpts {
	return ExecutePlanItemOpts{
		AccountID:        intent.AccountID,
		StockName:        intent.StockName,
		Price:            intent.Price,
		Volume:           intent.Volume,
		Reason:           buildResearchExecReason(intent),
		StrategyTag:      models.ResearchTradeStrategyTag,
		AutoFill:         true,
		SkipSafetyFrozen: true, // Research path is not a Frozen TradePlan
	}
}

func buildResearchExecReason(intent *models.ResearchTradeIntent) string {
	src := intent.ResearchSource
	if src == "" {
		src = models.ResearchSourceSignalScanSnapshot
	}
	parts := []string{
		fmt.Sprintf("research_source=%s", src),
		fmt.Sprintf("snapshot_id=%d", intent.CandidateSnapshotID),
		fmt.Sprintf("signal_tag=%s", intent.SignalTag),
	}
	if user := strings.TrimSpace(intent.Reason); user != "" {
		parts = append(parts, "note="+user)
	}
	return strings.Join(parts, ";")
}

func truncateErrMsg(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
