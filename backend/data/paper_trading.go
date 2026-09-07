package data

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/db"
	"go-stock/backend/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PaperAccount 模拟账户
type PaperAccount struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Name              string    `gorm:"size:64;default:默认模拟账户" json:"name"`
	Cash              float64   `json:"cash"`
	InitialCash       float64   `json:"initialCash"`
	Equity            float64   `json:"equity"`
	ValuationStatus   string    `gorm:"size:16;default:complete" json:"valuationStatus"`
	UnpricedPositions int       `gorm:"default:0" json:"unpricedPositions"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (PaperAccount) TableName() string { return "paper_accounts" }

// PaperPosition 模拟持仓
type PaperPosition struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	AccountID uint       `gorm:"index;uniqueIndex:idx_paper_pos" json:"accountId"`
	StockCode string     `gorm:"size:16;uniqueIndex:idx_paper_pos" json:"stockCode"`
	StockName string     `gorm:"size:64" json:"stockName"`
	Volume    int64      `json:"volume"`
	Sellable  int64      `json:"sellable"`
	AvgCost   float64    `json:"avgCost"`
	MarkPrice float64    `json:"markPrice"`
	MarkedAt  *time.Time `json:"markedAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func (PaperPosition) TableName() string { return "paper_positions" }

// PaperOrder 模拟委托/成交
type PaperOrder struct {
	ID               uint    `gorm:"primaryKey" json:"id"`
	AccountID        uint    `gorm:"index" json:"accountId"`
	StockCode        string  `gorm:"size:16;index" json:"stockCode"`
	StockName        string  `gorm:"size:64" json:"stockName"`
	Side             string  `gorm:"size:8;index" json:"side"`
	Status           string  `gorm:"size:16;index" json:"status"`
	Price            float64 `json:"price"`
	Volume           int64   `json:"volume"`
	FilledPrice      float64 `json:"filledPrice"` // 成交均价（avg）；全成时即成交价
	FilledVol        int64   `json:"filledVol"`   // 累计成交量（cum qty）
	Fee              float64 `json:"fee"`
	Reason           string  `gorm:"type:text" json:"reason"`
	StrategyTag      string  `gorm:"size:32" json:"strategyTag"`
	RejectCode       string  `gorm:"size:64" json:"rejectCode"`
	RejectReason     string  `gorm:"size:500" json:"rejectReason"`
	FillAttemptCount int     `gorm:"default:0" json:"fillAttemptCount"`
	ExecMode         string  `gorm:"size:32" json:"execMode"` // ioc_autofill | resting
	// Phase2-C 身份冻结：见 paper_order_lifecycle.go 头部约定。
	ClientOrderID   string     `gorm:"size:64;uniqueIndex:uidx_paper_order_client_order_id" json:"clientOrderId"`
	ExecBackend     string     `gorm:"size:32;index" json:"execBackend"`     // paper | real_xxx
	BrokerOrderID   string     `gorm:"size:64;index" json:"brokerOrderId"`   // 券商委托号；Paper 必须空
	ExternalOrderID string     `gorm:"size:64;index" json:"externalOrderId"` // 外部单号；Paper 必须空
	BrokerStatus    string     `gorm:"size:32" json:"brokerStatus"`          // Paper=paper；Real=细态预留
	FilledAt        *time.Time `json:"filledAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (PaperOrder) TableName() string { return "paper_orders" }

// PaperFill 模拟成交明细
type PaperFill struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AccountID   uint      `gorm:"index" json:"accountId"`
	OrderID     uint      `gorm:"index" json:"orderId"`
	StockCode   string    `gorm:"size:16;index" json:"stockCode"`
	StockName   string    `gorm:"size:64" json:"stockName"`
	Side        string    `gorm:"size:8;index" json:"side"`
	Price       float64   `json:"price"`
	Volume      int64     `json:"volume"`
	Fee         float64   `json:"fee"`
	StrategyTag string    `gorm:"size:32" json:"strategyTag"`
	FilledAt    time.Time `gorm:"index" json:"filledAt"`
}

func (PaperFill) TableName() string { return "paper_fills" }

// PaperEquityPoint 权益曲线点
type PaperEquityPoint struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AccountID uint      `gorm:"index" json:"accountId"`
	DayKey    string    `gorm:"size:16;index" json:"dayKey"`
	Equity    float64   `json:"equity"`
	Cash      float64   `json:"cash"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (PaperEquityPoint) TableName() string { return "paper_equity_points" }

var paperInitOnce sync.Once

// MigratePaperTrading 统一迁移模拟交易表，并在建立幂等唯一约束前清理旧版重复数据。
func MigratePaperTrading(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if err := database.AutoMigrate(
		&PaperAccount{},
		&PaperPosition{},
		&PaperOrder{},
		&PaperFill{},
		&PaperEquityPoint{},
		&PaperOrderEvent{},
		&RealStubOrder{},
		&RealStubReportLedger{},
		&RealStubFill{},
	); err != nil {
		return err
	}
	return database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			DELETE FROM paper_fills
			WHERE order_id > 0
			  AND id NOT IN (SELECT MIN(id) FROM paper_fills WHERE order_id > 0 GROUP BY order_id)
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_paper_fill_order ON paper_fills(order_id)").Error; err != nil {
			return err
		}
		if err := backfillPaperOrderIdentity(tx); err != nil {
			return err
		}
		if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_paper_order_client_order_id ON paper_orders(client_order_id)").Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			DELETE FROM paper_equity_points
			WHERE id NOT IN (
				SELECT MAX(id) FROM paper_equity_points GROUP BY account_id, day_key
			)
		`).Error; err != nil {
			return err
		}
		return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_paper_equity_day ON paper_equity_points(account_id, day_key)").Error
	})
}

func EnsurePaperTradingTables() {
	paperInitOnce.Do(func() {
		if db.Dao == nil {
			return
		}
		if err := MigratePaperTrading(db.Dao); err != nil {
			logger.SugaredLogger.Warnf("paper trading migrate: %v", err)
		}
	})
}

// newPaperClientOrderID 生成本地唯一 ClientOrderID（PaperClientOrderIDPrefix + 32hex）。
func newPaperClientOrderID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s%d", PaperClientOrderIDPrefix, time.Now().UnixNano())
	}
	return PaperClientOrderIDPrefix + hex.EncodeToString(b[:])
}

// backfillPaperOrderIdentity 为旧行补齐 client_order_id / exec_backend / broker_status，再挂唯一索引。
func backfillPaperOrderIdentity(tx *gorm.DB) error {
	if tx == nil {
		return nil
	}
	var orders []PaperOrder
	if err := tx.Where("client_order_id = ? OR client_order_id IS NULL", "").Find(&orders).Error; err != nil {
		return err
	}
	for i := range orders {
		updates := map[string]any{}
		if orders[i].ClientOrderID == "" {
			updates["client_order_id"] = newPaperClientOrderID()
		}
		if orders[i].ExecBackend == "" {
			updates["exec_backend"] = PaperExecBackendPaper
		}
		if orders[i].BrokerStatus == "" {
			updates["broker_status"] = PaperBrokerStatusPaper
		}
		if len(updates) == 0 {
			continue
		}
		if err := tx.Model(&PaperOrder{}).Where("id = ?", orders[i].ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&PaperOrder{}).
		Where("exec_backend = ? OR exec_backend IS NULL", "").
		Update("exec_backend", PaperExecBackendPaper).Error; err != nil {
		return err
	}
	// Paper 不伪造券商 ID；仅补齐通道状态占位。
	return tx.Model(&PaperOrder{}).
		Where("broker_status = ? OR broker_status IS NULL", "").
		Update("broker_status", PaperBrokerStatusPaper).Error
}

type PaperTradingApi struct{}

func NewPaperTradingApi() *PaperTradingApi { return &PaperTradingApi{} }

func (p *PaperTradingApi) GetOrCreateDefaultAccount(initialCash float64) (*PaperAccount, error) {
	EnsurePaperTradingTables()
	if initialCash <= 0 {
		initialCash = 1_000_000
	}
	var acc PaperAccount
	err := db.Dao.Order("id ASC").First(&acc).Error
	if err == nil {
		return &acc, nil
	}
	acc = PaperAccount{
		Name:            "默认模拟账户",
		Cash:            initialCash,
		InitialCash:     initialCash,
		Equity:          initialCash,
		ValuationStatus: "complete",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if cerr := db.Dao.Create(&acc).Error; cerr != nil {
		return nil, cerr
	}
	return &acc, nil
}

func (p *PaperTradingApi) ResetAccount(initialCash float64) (*PaperAccount, error) {
	started := time.Now()
	EnsurePaperTradingTables()
	if initialCash <= 0 {
		initialCash = 1_000_000
	}
	var account PaperAccount
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		for _, model := range []any{&PaperFill{}, &PaperOrder{}, &PaperPosition{}, &PaperEquityPoint{}, &PaperAccount{}} {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error; err != nil {
				return err
			}
		}
		now := time.Now()
		account = PaperAccount{
			Name:            "默认模拟账户",
			Cash:            initialCash,
			InitialCash:     initialCash,
			Equity:          initialCash,
			ValuationStatus: "complete",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		return tx.Create(&account).Error
	})
	logger.SugaredLogger.Infow("paper trading operation", "operation", "reset_account", "duration", time.Since(started), "ok", err == nil, "error", err)
	if err != nil {
		return nil, err
	}
	paperTradingEvents.AccountChanged(toTradeAccount(account))
	return &account, nil
}

type PaperSubmitOrderReq struct {
	AccountID   uint    `json:"accountId"`
	StockCode   string  `json:"stockCode"`
	StockName   string  `json:"stockName"`
	Side        string  `json:"side"`
	Price       float64 `json:"price"`
	Volume      int64   `json:"volume"`
	Reason      string  `json:"reason"`
	StrategyTag string  `json:"strategyTag"`
	AutoFill    bool    `json:"autoFill"`
}

func roundLotPaper(vol, lot int64) int64 {
	if lot <= 0 {
		lot = 100
	}
	return (vol / lot) * lot
}

// Quantity-policy hooks for Submit (C2.1) + Fill (C2.3).
// Registered from tradingrule to avoid data↔tradingrule import cycles.
var (
	quantityPolicyEnabledFn   func() bool
	quantityPolicyBuyVolumeOK func(stockCode string, volume int64) bool
)

// RegisterQuantityPolicySubmitHooks wires EnableQuantityPolicy + ValidateBuyQuantity.
// Shared by Submit and Fill. nil clears hooks (legacy-only path).
func RegisterQuantityPolicySubmitHooks(enabled func() bool, buyVolumeOK func(stockCode string, volume int64) bool) {
	quantityPolicyEnabledFn = enabled
	quantityPolicyBuyVolumeOK = buyVolumeOK
}

// applySubmitVolumePolicy resolves Submit volume.
//
//	Flag OFF (or sell / hooks unset): legacy roundLotPaper(...,100) + min 100
//	Flag ON + buy: Validate-only via hooks — no Normalize
func applySubmitVolumePolicy(stockCode, side string, volume int64) (int64, error) {
	policyOn := quantityPolicyEnabledFn != nil && quantityPolicyEnabledFn()
	if policyOn && !strings.EqualFold(strings.TrimSpace(side), "sell") {
		if quantityPolicyBuyVolumeOK == nil || !quantityPolicyBuyVolumeOK(stockCode, volume) {
			return 0, fmt.Errorf("数量不符合 QuantityPolicy（非法手数）")
		}
		return volume, nil
	}
	v := roundLotPaper(volume, 100)
	if v < 100 {
		return 0, fmt.Errorf("数量须至少一手(100股)")
	}
	return v, nil
}

// fillQuantityLotOK validates Fill qty without Normalize / rewrite.
//
//	Flag OFF (or sell / hooks unset): legacy qty>=100 && qty%100==0
//	Flag ON + buy: ValidateBuyQuantity via hooks only
func fillQuantityLotOK(stockCode, side string, qty int64) bool {
	policyOn := quantityPolicyEnabledFn != nil && quantityPolicyEnabledFn()
	if policyOn && !strings.EqualFold(strings.TrimSpace(side), "sell") {
		return quantityPolicyBuyVolumeOK != nil && quantityPolicyBuyVolumeOK(stockCode, qty)
	}
	return qty >= 100 && qty%100 == 0
}

func calcPaperBuyFee(amount float64) float64 {
	fee := amount * 0.00025
	if fee < 5 {
		fee = 5
	}
	return fee
}

func calcPaperSellFee(amount float64) float64 {
	return calcPaperBuyFee(amount) + amount*0.0005
}

func (p *PaperTradingApi) SubmitPaperOrder(req PaperSubmitOrderReq) (*PaperOrder, error) {
	// Paper accounting primitive. UI/API/Façade must not call directly;
	// only PaperBroker (ExecutionPort) and test doubles may invoke this.
	EnsurePaperTradingTables()
	acc, err := p.GetOrCreateDefaultAccount(0)
	if err != nil {
		return nil, err
	}
	if req.AccountID == 0 {
		req.AccountID = acc.ID
	}
	vol, verr := applySubmitVolumePolicy(req.StockCode, req.Side, req.Volume)
	if verr != nil {
		return nil, verr
	}
	req.Volume = vol
	if req.Price <= 0 {
		return nil, fmt.Errorf("价格无效")
	}
	if req.Side != "buy" && req.Side != "sell" {
		return nil, fmt.Errorf("side 须为 buy 或 sell")
	}

	order := PaperOrder{
		AccountID:     req.AccountID,
		StockCode:     req.StockCode,
		StockName:     req.StockName,
		Side:          req.Side,
		Status:        PaperOrderStatusPending,
		Price:         req.Price,
		Volume:        req.Volume,
		Reason:        req.Reason,
		StrategyTag:   req.StrategyTag,
		ClientOrderID: newPaperClientOrderID(),
		ExecBackend:   PaperExecBackendPaper,
		// Paper 不生成券商委托号；仅占位 broker_status。
		BrokerOrderID:   "",
		ExternalOrderID: "",
		BrokerStatus:    PaperBrokerStatusPaper,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if req.AutoFill {
		order.ExecMode = PaperOrderExecModeIOCAutofill
	}
	if err := db.Dao.Create(&order).Error; err != nil {
		return nil, err
	}
	if evErr := recordPaperOrderSubmitted(&order); evErr != nil {
		logger.SugaredLogger.Warnf("record order_submitted event failed order_id=%d: %v", order.ID, evErr)
	}
	paperTradingEvents.OrderSubmitted(toTradeOrder(order))
	if req.AutoFill {
		if ferr := p.FillPaperOrder(order.ID, req.Price); ferr != nil {
			// Fill 事务已回滚；独立事务 pending→rejected + order_rejected 审计
			if markErr := markPaperOrderRejectedAfterAutofillFail(order.ID, ferr); markErr != nil {
				logger.SugaredLogger.Warnf("mark paper order rejected failed order_id=%d: %v (fill_err=%v)", order.ID, markErr, ferr)
			}
			_ = db.Dao.First(&order, order.ID)
			return &order, ferr
		}
		_ = db.Dao.First(&order, order.ID)
		if evErr := recordPaperOrderFilled(&order); evErr != nil {
			logger.SugaredLogger.Warnf("record order_filled event failed order_id=%d: %v", order.ID, evErr)
		}
	}
	return &order, nil
}

func (p *PaperTradingApi) FillPaperOrder(orderID uint, fillPrice float64) error {
	// fillQty=0 → fill all remaining (legacy full-fill behavior).
	return p.FillPaperOrderQty(orderID, fillPrice, 0)
}

// FillPaperOrderQty fills up to fillQty shares at fillPrice.
// fillQty<=0 means fill all remaining (order.Volume - FilledVol).
// Partial: OMS stays pending with updated FilledVol; full: OMS→filled.
// Cancelled/rejected orders cannot be filled (pending claim fails).
func (p *PaperTradingApi) FillPaperOrderQty(orderID uint, fillPrice float64, fillQty int64) (err error) {
	started := time.Now()
	defer func() {
		logger.SugaredLogger.Infow("paper trading operation", "operation", "fill_order", "order_id", orderID, "fill_qty", fillQty, "duration", time.Since(started), "ok", err == nil, "error", err)
	}()
	EnsurePaperTradingTables()

	var order PaperOrder
	var fill PaperFill
	var account PaperAccount
	var changedPosition PaperPosition
	committed := false
	fullyFilled := false
	err = db.Dao.Transaction(func(tx *gorm.DB) error {
		claim := tx.Model(&PaperOrder{}).
			Where("id = ? AND status = ?", orderID, PaperOrderStatusPending).
			Updates(map[string]any{"status": PaperOrderStatusProcessing, "updated_at": time.Now()})
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			if err := tx.First(&order, orderID).Error; err != nil {
				return err
			}
			if order.Status == PaperOrderStatusFilled {
				// Idempotent: already fully filled — no rollback.
				return nil
			}
			return paperFillReject(PaperOrderRejectInvalidOrder, "订单状态不是 pending: %s", order.Status)
		}
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		if fillPrice <= 0 {
			fillPrice = order.Price
		}
		if fillPrice <= 0 {
			return paperFillReject(PaperOrderRejectInvalidOrder, "成交价格无效")
		}

		remaining := order.Volume - order.FilledVol
		if remaining <= 0 {
			return paperFillReject(PaperOrderRejectInvalidOrder, "no remaining quantity")
		}
		qty := fillQty
		if qty <= 0 {
			qty = remaining
		}
		if qty > remaining {
			return paperFillReject(PaperOrderRejectInvalidOrder, "fill_qty %d > remaining %d", qty, remaining)
		}
		// Phase12-M2.5-C2.3: Flag OFF keep %100; Flag ON buy → QuantityPolicy Validate-only.
		// Fill never Normalizes or rewrites qty; slice/partial-fill path above unchanged.
		if !fillQuantityLotOK(order.StockCode, order.Side, qty) {
			if quantityPolicyEnabledFn != nil && quantityPolicyEnabledFn() && !strings.EqualFold(strings.TrimSpace(order.Side), "sell") {
				return paperFillReject(PaperOrderRejectInvalidOrder, "fill_qty 不符合 QuantityPolicy (got %d)", qty)
			}
			return paperFillReject(PaperOrderRejectInvalidOrder, "fill_qty must be lot multiple >= 100 (got %d)", qty)
		}

		if err := tx.First(&account, order.AccountID).Error; err != nil {
			return err
		}

		now := time.Now()
		amount := fillPrice * float64(qty)
		var position PaperPosition
		positionQuery := tx.Where("account_id = ? AND stock_code = ?", order.AccountID, order.StockCode).First(&position)
		var legFee float64
		if order.Side == "buy" {
			legFee = calcPaperBuyFee(amount)
			total := amount + legFee
			if account.Cash < total {
				return paperFillReject(PaperOrderRejectCashInsufficient, "现金不足：需要 %.2f，可用 %.2f", total, account.Cash)
			}
			account.Cash -= total
			if errors.Is(positionQuery.Error, gorm.ErrRecordNotFound) {
				position = PaperPosition{
					AccountID: order.AccountID,
					StockCode: order.StockCode,
					StockName: order.StockName,
					Volume:    qty,
					AvgCost:   fillPrice,
					MarkPrice: fillPrice,
					MarkedAt:  &now,
					UpdatedAt: now,
				}
				if err := tx.Create(&position).Error; err != nil {
					return err
				}
			} else if positionQuery.Error != nil {
				return positionQuery.Error
			} else {
				newVolume := position.Volume + qty
				position.AvgCost = (position.AvgCost*float64(position.Volume) + amount) / float64(newVolume)
				position.Volume = newVolume
				position.MarkPrice = fillPrice
				position.MarkedAt = &now
				position.UpdatedAt = now
				if err := tx.Save(&position).Error; err != nil {
					return err
				}
			}
		} else {
			if errors.Is(positionQuery.Error, gorm.ErrRecordNotFound) {
				return paperFillReject(PaperOrderRejectPositionInsufficient, "无持仓")
			}
			if positionQuery.Error != nil {
				return positionQuery.Error
			}
			if position.Sellable < qty {
				return paperFillReject(PaperOrderRejectPositionInsufficient, "可卖数量不足（T+1）：可卖 %d，委托 %d", position.Sellable, qty)
			}
			legFee = calcPaperSellFee(amount)
			account.Cash += amount - legFee
			position.Volume -= qty
			position.Sellable -= qty
			position.MarkPrice = fillPrice
			position.MarkedAt = &now
			position.UpdatedAt = now
			if position.Volume == 0 {
				if err := tx.Delete(&position).Error; err != nil {
					return err
				}
			} else if err := tx.Save(&position).Error; err != nil {
				return err
			}
		}
		changedPosition = position

		prevVol := order.FilledVol
		prevAvg := order.FilledPrice
		prevFee := order.Fee
		newVol := prevVol + qty
		var newAvg float64
		if prevVol <= 0 {
			newAvg = fillPrice
		} else {
			newAvg = (prevAvg*float64(prevVol) + amount) / float64(newVol)
		}
		newFee := prevFee + legFee
		fullyFilled = newVol >= order.Volume
		finalStatus := PaperOrderStatusPending
		updates := map[string]any{
			"filled_price": newAvg,
			"filled_vol":   newVol,
			"fee":          newFee,
			"updated_at":   now,
		}
		if fullyFilled {
			finalStatus = PaperOrderStatusFilled
			updates["status"] = PaperOrderStatusFilled
			updates["filled_at"] = now
		} else {
			updates["status"] = PaperOrderStatusPending
		}

		fill = PaperFill{
			AccountID:   order.AccountID,
			OrderID:     order.ID,
			StockCode:   order.StockCode,
			StockName:   order.StockName,
			Side:        order.Side,
			Price:       fillPrice,
			Volume:      qty,
			Fee:         legFee,
			StrategyTag: order.StrategyTag,
			FilledAt:    now,
		}
		// Phase2 unique index uidx_paper_fill_order: one fill row per order (no migration).
		// Partial legs accumulate into the same row (Volume/Price/Fee = cum).
		var existingFill PaperFill
		ferr := tx.Where("order_id = ?", order.ID).First(&existingFill).Error
		if errors.Is(ferr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&fill).Error; err != nil {
				return err
			}
		} else if ferr != nil {
			return ferr
		} else {
			existingFill.Price = newAvg
			existingFill.Volume = newVol
			existingFill.Fee = newFee
			existingFill.FilledAt = now
			if err := tx.Save(&existingFill).Error; err != nil {
				return err
			}
			fill = existingFill
		}
		finalize := tx.Model(&PaperOrder{}).
			Where("id = ? AND status = ?", order.ID, PaperOrderStatusProcessing).
			Updates(updates)
		if finalize.Error != nil {
			return finalize.Error
		}
		if finalize.RowsAffected != 1 {
			return paperFillReject(PaperOrderRejectInvalidOrder, "订单状态 CAS 失败")
		}
		order.Status = finalStatus
		order.FilledPrice = newAvg
		order.FilledVol = newVol
		order.Fee = newFee
		if fullyFilled {
			order.FilledAt = &now
		}
		order.UpdatedAt = now

		if err := revaluePaperAccount(tx, &account, now); err != nil {
			return err
		}
		if err := upsertPaperEquityPoint(tx, account, now); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil || !committed {
		return err
	}
	paperTradingEvents.Filled(toTradeFill(fill))
	if fullyFilled {
		paperTradingEvents.OrderFilled(toTradeOrder(order))
	}
	paperTradingEvents.PositionChanged(toTradePosition(changedPosition))
	paperTradingEvents.AccountChanged(toTradeAccount(account))
	return nil
}

// RejectPaperOrderSim marks a zero-fill pending order as rejected (scenario simulation).
// Does not touch Position/Cash. reason must be price|liquidity|broker.
func (p *PaperTradingApi) RejectPaperOrderSim(orderID uint, reason, message string) error {
	EnsurePaperTradingTables()
	if db.Dao == nil || orderID == 0 {
		return paperFillReject(PaperOrderRejectInvalidOrder, "invalid order id")
	}
	reason = strings.TrimSpace(strings.ToLower(reason))
	switch reason {
	case PaperSimRejectReasonPrice, PaperSimRejectReasonLiquidity, PaperSimRejectReasonBroker:
	default:
		return paperFillReject(PaperOrderRejectInvalidOrder, "reject reason must be price|liquidity|broker")
	}
	if strings.TrimSpace(message) == "" {
		message = "simulated reject: " + reason
	}
	if len(message) > 500 {
		message = message[:500]
	}
	now := time.Now()
	var rejected *PaperOrder
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		var order PaperOrder
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		if order.Status != PaperOrderStatusPending {
			return paperFillReject(PaperOrderRejectInvalidOrder, "订单状态不是 pending: %s", order.Status)
		}
		if order.FilledVol > 0 {
			return paperFillReject(PaperOrderRejectInvalidOrder, "cannot reject after partial fills (use cancel)")
		}
		res := tx.Model(&PaperOrder{}).
			Where("id = ? AND status = ? AND filled_vol = 0", orderID, PaperOrderStatusPending).
			Updates(map[string]any{
				"status":             PaperOrderStatusRejected,
				"reject_code":        reason,
				"reject_reason":      message,
				"fill_attempt_count": gorm.Expr("fill_attempt_count + 1"),
				"updated_at":         now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return paperFillReject(PaperOrderRejectInvalidOrder, "reject CAS miss")
		}
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		if err := writePaperOrderEvent(tx, &order, PaperOrderEventRejected, reason, message, now); err != nil {
			return err
		}
		rejected = &order
		return nil
	})
	if err != nil {
		return err
	}
	if rejected != nil {
		paperTradingEvents.OrderRejected(toTradeOrder(*rejected))
	}
	return nil
}

func revaluePaperAccount(tx *gorm.DB, account *PaperAccount, now time.Time) error {
	var positions []PaperPosition
	if err := tx.Where("account_id = ?", account.ID).Find(&positions).Error; err != nil {
		return err
	}
	value := account.Cash
	unpriced := 0
	for _, position := range positions {
		if position.MarkPrice <= 0 || position.MarkedAt == nil {
			unpriced++
			continue
		}
		value += position.MarkPrice * float64(position.Volume)
	}
	account.Equity = math.Round(value*100) / 100
	account.UnpricedPositions = unpriced
	account.ValuationStatus = "complete"
	if unpriced > 0 {
		account.ValuationStatus = "partial"
	}
	account.UpdatedAt = now
	return tx.Save(account).Error
}

func upsertPaperEquityPoint(tx *gorm.DB, account PaperAccount, now time.Time) error {
	point := PaperEquityPoint{
		AccountID: account.ID,
		DayKey:    now.Format("2006-01-02"),
		Equity:    account.Equity,
		Cash:      account.Cash,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}, {Name: "day_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"equity", "cash", "updated_at"}),
	}).Create(&point).Error
}

func (p *PaperTradingApi) SettleAllSellable(accountID uint) (err error) {
	started := time.Now()
	defer func() {
		logger.SugaredLogger.Infow("paper trading operation", "operation", "settle_sellable", "account_id", accountID, "duration", time.Since(started), "ok", err == nil, "error", err)
	}()
	EnsurePaperTradingTables()
	if accountID == 0 {
		account, err := p.GetOrCreateDefaultAccount(0)
		if err != nil {
			return err
		}
		accountID = account.ID
	}
	var positions []PaperPosition
	var account PaperAccount
	err = db.Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", accountID).Find(&positions).Error; err != nil {
			return err
		}
		now := time.Now()
		for i := range positions {
			positions[i].Sellable = positions[i].Volume
			positions[i].UpdatedAt = now
			if err := tx.Save(&positions[i]).Error; err != nil {
				return err
			}
		}
		if err := tx.First(&account, accountID).Error; err != nil {
			return err
		}
		account.UpdatedAt = now
		return tx.Save(&account).Error
	})
	if err != nil {
		return err
	}
	for _, position := range positions {
		paperTradingEvents.PositionChanged(toTradePosition(position))
	}
	paperTradingEvents.AccountChanged(toTradeAccount(account))
	return nil
}

// SetMarkPrice 更新持仓市价并重新估值。未设置市价的持仓不会按成本价冒充市值，
// 账户会以 valuationStatus=partial 和 unpricedPositions 明确标识估值不完整。
func (p *PaperTradingApi) SetMarkPrice(accountID uint, stockCode string, markPrice float64) (err error) {
	started := time.Now()
	defer func() {
		logger.SugaredLogger.Infow("paper trading operation", "operation", "set_mark_price", "account_id", accountID, "stock_code", stockCode, "duration", time.Since(started), "ok", err == nil, "error", err)
	}()
	if accountID == 0 {
		account, getErr := p.GetOrCreateDefaultAccount(0)
		if getErr != nil {
			return getErr
		}
		accountID = account.ID
	}
	if markPrice <= 0 {
		return fmt.Errorf("市价必须大于 0")
	}

	var position PaperPosition
	var account PaperAccount
	err = db.Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ? AND stock_code = ?", accountID, stockCode).First(&position).Error; err != nil {
			return err
		}
		now := time.Now()
		position.MarkPrice = markPrice
		position.MarkedAt = &now
		position.UpdatedAt = now
		if err := tx.Save(&position).Error; err != nil {
			return err
		}
		if err := tx.First(&account, accountID).Error; err != nil {
			return err
		}
		if err := revaluePaperAccount(tx, &account, now); err != nil {
			return err
		}
		return upsertPaperEquityPoint(tx, account, now)
	})
	if err != nil {
		return err
	}
	paperTradingEvents.PositionChanged(toTradePosition(position))
	paperTradingEvents.AccountChanged(toTradeAccount(account))
	return nil
}

type PaperAccountSnapshot struct {
	Account   PaperAccount       `json:"account"`
	Positions []PaperPosition    `json:"positions"`
	Orders    []PaperOrder       `json:"orders"`
	Fills     []PaperFill        `json:"fills"`
	Equity    []PaperEquityPoint `json:"equity"`
}

func (p *PaperTradingApi) GetSnapshot(accountID uint) (*PaperAccountSnapshot, error) {
	EnsurePaperTradingTables()
	acc, err := p.GetOrCreateDefaultAccount(0)
	if err != nil {
		return nil, err
	}
	if accountID == 0 {
		accountID = acc.ID
	}
	var positions []PaperPosition
	var orders []PaperOrder
	var fills []PaperFill
	var equity []PaperEquityPoint
	_ = db.Dao.Where("account_id = ?", accountID).Find(&positions)
	_ = db.Dao.Where("account_id = ?", accountID).Order("id DESC").Limit(50).Find(&orders)
	_ = db.Dao.Where("account_id = ?", accountID).Order("id DESC").Limit(100).Find(&fills)
	_ = db.Dao.Where("account_id = ?", accountID).Order("id ASC").Limit(365).Find(&equity)
	_ = db.Dao.First(acc, accountID)
	return &PaperAccountSnapshot{Account: *acc, Positions: positions, Orders: orders, Fills: fills, Equity: equity}, nil
}

var paperTradingEvents = broker.NewPaperEventBridge(broker.DefaultHub)

func paperTradingID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func toTradeOrder(order PaperOrder) broker.TradeOrder {
	tradeOrder := broker.TradeOrder{
		ID:               paperTradingID(order.ID),
		AccountID:        paperTradingID(order.AccountID),
		StockCode:        order.StockCode,
		Symbol:           order.StockCode,
		StockName:        order.StockName,
		Side:             order.Side,
		Status:           order.Status,
		Price:            order.Price,
		Volume:           order.Volume,
		FilledPrice:      order.FilledPrice,
		FilledVolume:     order.FilledVol,
		Fee:              order.Fee,
		Reason:           order.Reason,
		StrategyTag:      order.StrategyTag,
		RejectCode:       order.RejectCode,
		RejectReason:     order.RejectReason,
		FillAttemptCount: order.FillAttemptCount,
		ClientOrderID:    order.ClientOrderID,
		ExecBackend:      order.ExecBackend,
		BrokerOrderID:    order.BrokerOrderID,
		ExternalOrderID:  order.ExternalOrderID,
		BrokerStatus:     order.BrokerStatus,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        order.UpdatedAt,
	}
	if order.FilledAt != nil {
		tradeOrder.FilledAt = *order.FilledAt
	}
	return tradeOrder
}

func toTradeFill(fill PaperFill) broker.TradeFill {
	return broker.TradeFill{
		ID:          paperTradingID(fill.ID),
		OrderID:     paperTradingID(fill.OrderID),
		AccountID:   paperTradingID(fill.AccountID),
		StockCode:   fill.StockCode,
		StockName:   fill.StockName,
		Side:        fill.Side,
		Price:       fill.Price,
		Volume:      fill.Volume,
		Fee:         fill.Fee,
		StrategyTag: fill.StrategyTag,
		FilledAt:    fill.FilledAt,
	}
}

func toTradePosition(position PaperPosition) broker.TradePosition {
	tradePosition := broker.TradePosition{
		AccountID: paperTradingID(position.AccountID),
		StockCode: position.StockCode,
		StockName: position.StockName,
		Volume:    position.Volume,
		Sellable:  position.Sellable,
		AvgCost:   position.AvgCost,
		MarkPrice: position.MarkPrice,
		UpdatedAt: position.UpdatedAt,
	}
	if position.MarkedAt != nil {
		tradePosition.MarkedAt = *position.MarkedAt
	}
	return tradePosition
}

func toTradeAccount(account PaperAccount) broker.TradeAccount {
	return broker.TradeAccount{
		ID:                paperTradingID(account.ID),
		Name:              account.Name,
		Cash:              account.Cash,
		InitialCash:       account.InitialCash,
		Equity:            account.Equity,
		ValuationStatus:   account.ValuationStatus,
		UnpricedPositions: account.UnpricedPositions,
		UpdatedAt:         account.UpdatedAt,
	}
}
