package execution

import (
	"errors"
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
)

// preTradeRejectError 执行前拒单（与 paper reject_code 对齐）。
type preTradeRejectError struct {
	Code string
	Msg  string
}

func (e *preTradeRejectError) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func preTradeReject(code, format string, args ...any) error {
	return &preTradeRejectError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// PreTradeRejectCode 从错误链提取拒单码。
func PreTradeRejectCode(err error) string {
	if err == nil {
		return ""
	}
	var pe *preTradeRejectError
	if errors.As(err, &pe) && pe.Code != "" {
		return pe.Code
	}
	return data.PaperOrderRejectInternal
}

// preTradeAccountSnapshot 轻量账户视图（仅现金与持仓）。
type preTradeAccountSnapshot struct {
	Cash      float64
	Positions map[string]preTradePosition
}

type preTradePosition struct {
	Volume   int64
	Sellable int64
}

// preTradeSnapshotLoader 可注入，便于单测不走真实 DB。
type preTradeSnapshotLoader func(accountID uint) (*preTradeAccountSnapshot, error)

func defaultPreTradeSnapshotLoader(accountID uint) (*preTradeAccountSnapshot, error) {
	api := data.NewPaperTradingApi()
	if accountID == 0 {
		acc, err := api.GetOrCreateDefaultAccount(0)
		if err != nil {
			return nil, preTradeReject(data.PaperOrderRejectInternal, "加载默认账户失败: %v", err)
		}
		accountID = acc.ID
	}
	snap, err := api.GetSnapshot(accountID)
	if err != nil {
		// GetSnapshot 在账户不存在时可能失败；尝试 First
		if db.Dao == nil {
			return nil, preTradeReject(data.PaperOrderRejectInternal, "数据库未初始化")
		}
		var acc data.PaperAccount
		if ferr := db.Dao.First(&acc, accountID).Error; ferr != nil {
			return nil, preTradeReject(data.PaperOrderRejectInternal, "加载账户失败: %v", ferr)
		}
		var positions []data.PaperPosition
		_ = db.Dao.Where("account_id = ?", accountID).Find(&positions)
		out := &preTradeAccountSnapshot{Cash: acc.Cash, Positions: map[string]preTradePosition{}}
		for _, p := range positions {
			out.Positions[strings.ToLower(p.StockCode)] = preTradePosition{Volume: p.Volume, Sellable: p.Sellable}
		}
		return out, nil
	}
	out := &preTradeAccountSnapshot{Cash: snap.Account.Cash, Positions: map[string]preTradePosition{}}
	for _, p := range snap.Positions {
		out.Positions[strings.ToLower(p.StockCode)] = preTradePosition{Volume: p.Volume, Sellable: p.Sellable}
	}
	return out, nil
}

// PreTradeCheck 下单瞬间轻量检查：合法性 + 现金/可卖持仓。不改仓、不写单。
// 与 PlanFilter（计划态）和 FillPaperOrder（最终事务）职责分离。
func PreTradeCheck(intent SubmitIntent, load preTradeSnapshotLoader) error {
	if load == nil {
		load = defaultPreTradeSnapshotLoader
	}
	symbol := strings.TrimSpace(intent.StockCode)
	if symbol == "" || intent.Price <= 0 || intent.Volume <= 0 {
		return preTradeReject(data.PaperOrderRejectInvalidOrder, "证券、价格或数量无效")
	}
	side := strings.ToLower(strings.TrimSpace(intent.Side))
	if side == "" {
		side = "buy"
	}
	if side != "buy" && side != "sell" {
		return preTradeReject(data.PaperOrderRejectInvalidOrder, "side 须为 buy 或 sell")
	}

	snap, err := load(intent.AccountID)
	if err != nil {
		var pe *preTradeRejectError
		if errors.As(err, &pe) {
			return err
		}
		return preTradeReject(data.PaperOrderRejectInternal, "%v", err)
	}
	if snap == nil {
		return preTradeReject(data.PaperOrderRejectInternal, "账户快照为空")
	}

	amount := intent.Price * float64(intent.Volume)
	switch side {
	case "buy":
		if snap.Cash < amount {
			return preTradeReject(data.PaperOrderRejectCashInsufficient,
				"PreTradeCheck 现金不足：需要 %.2f，可用 %.2f", amount, snap.Cash)
		}
	case "sell":
		pos, ok := snap.Positions[strings.ToLower(symbol)]
		if !ok || pos.Sellable < intent.Volume {
			sellable := int64(0)
			if ok {
				sellable = pos.Sellable
			}
			return preTradeReject(data.PaperOrderRejectPositionInsufficient,
				"PreTradeCheck 可卖持仓不足：可卖 %d，委托 %d", sellable, intent.Volume)
		}
	}
	return nil
}
