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

// ErrManualTradeIntentNotExecutable Intent 非 confirmed，禁止进入 Execution。
var ErrManualTradeIntentNotExecutable = errors.New("manual trade intent not executable")

// ErrUnsupportedManualTradeOrderKind 首期仅支持 order_kind=normal。
var ErrUnsupportedManualTradeOrderKind = errors.New("unsupported manual trade order kind")

// ManualTradeExecutor 模拟盘人工交易 façade（Phase3-PR4-B）。
// 唯一下单出口：ExecutionService.ExecutePlanItem；不直连纸面会计层。
type ManualTradeExecutor interface {
	ConfirmAndExecuteManualTrade(ctx context.Context, intentID uint) error
}

type manualIntentStore interface {
	GetManualTradeIntent(id uint) (*models.ManualTradeIntent, error)
	UpdateManualTradeIntentStatus(id uint, toStatus string, patch *data.ManualTradeIntentStatusPatch) error
}

// ManualTradeFacade 将 ManualTradeIntent 映射为临时 TradePlanItem 后走 ExecutionService。
type ManualTradeFacade struct {
	svc  *ExecutionService
	repo manualIntentStore
}

// NewManualTradeFacade 构造 façade；repo 为 nil 时使用默认 ManualTradeIntentRepo。
func NewManualTradeFacade(svc *ExecutionService, repo manualIntentStore) *ManualTradeFacade {
	if repo == nil {
		repo = data.NewManualTradeIntentRepo()
	}
	return &ManualTradeFacade{svc: svc, repo: repo}
}

var _ ManualTradeExecutor = (*ManualTradeFacade)(nil)

// ConfirmAndExecuteManualTrade 仅当 status=confirmed 且 order_kind=normal 时执行。
// 成功→submitted，失败→rejected；不自动 draft→confirmed。
func (f *ManualTradeFacade) ConfirmAndExecuteManualTrade(ctx context.Context, intentID uint) error {
	if f == nil || f.svc == nil {
		return fmt.Errorf("manual trade facade: nil ExecutionService")
	}
	if f.repo == nil {
		return fmt.Errorf("manual trade facade: nil intent repo")
	}
	if intentID == 0 {
		return fmt.Errorf("manual trade facade: intentID is required")
	}

	intent, err := f.repo.GetManualTradeIntent(intentID)
	if err != nil {
		return err
	}
	if intent.Status != models.ManualTradeIntentStatusConfirmed {
		return fmt.Errorf("%w: status=%s (only confirmed may execute)", ErrManualTradeIntentNotExecutable, intent.Status)
	}

	kind := strings.TrimSpace(intent.OrderKind)
	if kind == "" {
		kind = models.ManualTradeOrderKindNormal
	}
	if kind != models.ManualTradeOrderKindNormal {
		return fmt.Errorf("%w: %s", ErrUnsupportedManualTradeOrderKind, kind)
	}

	side := strings.ToLower(strings.TrimSpace(intent.Side))
	if side == "" {
		side = "buy"
	}
	if side != "buy" && side != "sell" {
		return fmt.Errorf("%w: side=%s", ErrManualTradeIntentNotExecutable, side)
	}

	item := mapManualIntentToPlanItem(intent)
	opts := mapManualIntentToExecOpts(intent)

	order, execErr := f.svc.ExecutePlanItem(ctx, item, opts)
	if execErr != nil {
		code := PreTradeRejectCode(execErr)
		if code == "" {
			code = data.PaperOrderRejectInternal
		}
		_ = f.repo.UpdateManualTradeIntentStatus(intentID, models.ManualTradeIntentStatusRejected, &data.ManualTradeIntentStatusPatch{
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
	if err := f.repo.UpdateManualTradeIntentStatus(intentID, models.ManualTradeIntentStatusSubmitted, &data.ManualTradeIntentStatusPatch{
		ClientOrderID: clientOID,
		OrderID:       orderID,
	}); err != nil {
		return fmt.Errorf("execution ok but intent status update failed: %w", err)
	}
	return nil
}

func mapManualIntentToPlanItem(intent *models.ManualTradeIntent) models.TradePlanItem {
	side := strings.TrimSpace(intent.Side)
	if side == "" {
		side = "buy"
	}
	return models.TradePlanItem{
		StockCode:    intent.Symbol,
		StockName:    intent.StockName,
		Side:         side,
		LimitPrice:   intent.Price,
		TargetVolume: intent.Volume,
		Reason:       buildManualExecReason(intent),
		StrategyName: models.ManualTradeStrategyTag,
		Status:       models.TradePlanItemPending,
	}
}

func mapManualIntentToExecOpts(intent *models.ManualTradeIntent) ExecutePlanItemOpts {
	return ExecutePlanItemOpts{
		AccountID:        intent.AccountID,
		StockName:        intent.StockName,
		Price:            intent.Price,
		Volume:           intent.Volume,
		Reason:           buildManualExecReason(intent),
		StrategyTag:      models.ManualTradeStrategyTag,
		AutoFill:         true,
		SkipSafetyFrozen: true, // Manual path is not a Frozen TradePlan
	}
}

func buildManualExecReason(intent *models.ManualTradeIntent) string {
	src := intent.ManualSource
	if src == "" {
		src = models.ManualSourcePaperTradingPanel
	}
	kind := intent.OrderKind
	if kind == "" {
		kind = models.ManualTradeOrderKindNormal
	}
	parts := []string{
		fmt.Sprintf("manual_source=%s", src),
		fmt.Sprintf("order_kind=%s", kind),
	}
	if user := strings.TrimSpace(intent.Reason); user != "" {
		parts = append(parts, "reason="+user)
	}
	return strings.Join(parts, ";")
}
