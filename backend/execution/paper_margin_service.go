package execution

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/risk"

	"gorm.io/gorm"
)

const (
	defaultInitialCash      = 1_000_000
	defaultWarningRatio     = 1.5
	defaultCloseoutRatio    = 1.3
	defaultFinanceRate      = 0.08
	defaultSecuritiesRate   = 0.10
	defaultCollateralRate   = 0.70
	defaultMarginRatio      = 0.50
	defaultFinanceCredit    = 500_000
	defaultSecuritiesCredit = 500_000
)

type PaperMarginService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewPaperMarginService(db *gorm.DB) *PaperMarginService {
	return &PaperMarginService{db: db, now: time.Now}
}

func MigratePaperMargin(db *gorm.DB) error {
	if db == nil {
		return errors.New("paper margin database is nil")
	}
	models := []any{&data.PaperAccount{}, &data.PaperPosition{}}
	models = append(models, data.PaperMarginModels()...)
	return db.AutoMigrate(models...)
}

func (s *PaperMarginService) ConfigureAccount(ctx context.Context, config AccountConfig) (*data.PaperMarginAccount, error) {
	if config.Mode != data.PaperAccountModeCash && config.Mode != data.PaperAccountModeMargin {
		return nil, errors.New("account mode must be cash or margin")
	}
	if config.FinanceCreditLimit < 0 || config.SecuritiesCreditLimit < 0 ||
		config.FinanceAnnualRate < 0 || config.SecuritiesAnnualRate < 0 {
		return nil, errors.New("credit limits and annual rates cannot be negative")
	}
	if config.WarningRatio <= 0 || config.CloseoutRatio <= 0 || config.WarningRatio <= config.CloseoutRatio {
		return nil, errors.New("simulated warning ratio must be greater than closeout ratio")
	}
	tx := s.db.WithContext(ctx)
	_, margin, err := s.ensureAccount(tx, config.AccountID)
	if err != nil {
		return nil, err
	}
	margin.Mode = config.Mode
	margin.FinanceCreditLimit = config.FinanceCreditLimit
	margin.SecuritiesCreditLimit = config.SecuritiesCreditLimit
	margin.WarningRatio = config.WarningRatio
	margin.CloseoutRatio = config.CloseoutRatio
	margin.FinanceAnnualRate = config.FinanceAnnualRate
	margin.SecuritiesAnnualRate = config.SecuritiesAnnualRate
	if err = tx.Save(margin).Error; err != nil {
		return nil, err
	}
	return margin, nil
}

func (s *PaperMarginService) UpsertBorrowPool(ctx context.Context, pool data.PaperBorrowPool) (*data.PaperBorrowPool, error) {
	if pool.StockCode == "" || pool.AvailableQuantity < 0 {
		return nil, errors.New("stock code is required and available quantity cannot be negative")
	}
	if pool.CollateralRate < 0 || pool.CollateralRate > 1 ||
		pool.FinanceMarginRatio < 0 || pool.SecuritiesMarginRatio < 0 {
		return nil, errors.New("invalid simulated collateral or margin ratio")
	}
	var current data.PaperBorrowPool
	tx := s.db.WithContext(ctx)
	err := tx.Where("stock_code = ?", pool.StockCode).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		current = pool
		current.ID = 0
		err = tx.Create(&current).Error
	} else if err == nil {
		pool.ID = current.ID
		err = tx.Save(&pool).Error
		current = pool
	}
	if err != nil {
		return nil, err
	}
	return &current, nil
}

func (s *PaperMarginService) ensureAccount(tx *gorm.DB, accountID uint) (*data.PaperAccount, *data.PaperMarginAccount, error) {
	if tx == nil {
		return nil, nil, errors.New("paper margin database is nil")
	}
	var account data.PaperAccount
	query := tx
	if accountID > 0 {
		query = query.Where("id = ?", accountID)
	}
	err := query.Order("id ASC").First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && accountID == 0 {
		account = data.PaperAccount{
			Name:        "默认模拟账户",
			Cash:        defaultInitialCash,
			InitialCash: defaultInitialCash,
			Equity:      defaultInitialCash,
		}
		if err = tx.Create(&account).Error; err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}

	var margin data.PaperMarginAccount
	err = tx.Where("account_id = ?", account.ID).First(&margin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := s.now()
		margin = data.PaperMarginAccount{
			AccountID:             account.ID,
			Mode:                  data.PaperAccountModeMargin,
			FinanceCreditLimit:    defaultFinanceCredit,
			SecuritiesCreditLimit: defaultSecuritiesCredit,
			WarningRatio:          defaultWarningRatio,
			CloseoutRatio:         defaultCloseoutRatio,
			FinanceAnnualRate:     defaultFinanceRate,
			SecuritiesAnnualRate:  defaultSecuritiesRate,
			LastAccruedAt:         &now,
		}
		if err = tx.Create(&margin).Error; err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}
	return &account, &margin, nil
}

func markMap(marks []PositionMark) map[string]float64 {
	result := make(map[string]float64, len(marks))
	for _, mark := range marks {
		if mark.StockCode != "" && mark.Price > 0 {
			result[mark.StockCode] = mark.Price
		}
	}
	return result
}

func markedPrice(code string, avg float64, marks map[string]float64) float64 {
	if price := marks[code]; price > 0 {
		return price
	}
	return avg
}

type accountState struct {
	account       data.PaperAccount
	margin        data.PaperMarginAccount
	positions     []data.PaperPosition
	finance       []data.PaperFinanceLiability
	securities    []data.PaperSecuritiesLiability
	pools         map[string]data.PaperBorrowPool
	longValue     float64
	shortValue    float64
	collateral    float64
	financeTotal  float64
	interestTotal float64
	securityFees  float64
	marginUsed    float64
}

func (s *PaperMarginService) loadState(tx *gorm.DB, accountID uint, marks []PositionMark) (*accountState, error) {
	account, margin, err := s.ensureAccount(tx, accountID)
	if err != nil {
		return nil, err
	}
	state := &accountState{account: *account, margin: *margin, pools: map[string]data.PaperBorrowPool{}}
	if err = tx.Where("account_id = ?", account.ID).Find(&state.positions).Error; err != nil {
		return nil, err
	}
	if err = tx.Where("account_id = ?", account.ID).Find(&state.finance).Error; err != nil {
		return nil, err
	}
	if err = tx.Where("account_id = ?", account.ID).Find(&state.securities).Error; err != nil {
		return nil, err
	}
	var pools []data.PaperBorrowPool
	if err = tx.Find(&pools).Error; err != nil {
		return nil, err
	}
	for _, pool := range pools {
		state.pools[pool.StockCode] = pool
	}
	prices := markMap(marks)
	state.collateral = account.Cash
	for _, pos := range state.positions {
		value := markedPrice(pos.StockCode, pos.AvgCost, prices) * float64(pos.Volume)
		state.longValue += value
		rate := defaultCollateralRate
		if pool, ok := state.pools[pos.StockCode]; ok {
			rate = pool.CollateralRate
		}
		state.collateral += value * rate
	}
	for _, debt := range state.finance {
		state.financeTotal += debt.Principal
		state.interestTotal += debt.AccruedInterest
		ratio := defaultMarginRatio
		if pool, ok := state.pools[debt.StockCode]; ok {
			ratio = pool.FinanceMarginRatio
		}
		state.marginUsed += debt.Principal * ratio
	}
	for _, debt := range state.securities {
		value := markedPrice(debt.StockCode, debt.AvgPrice, prices) * float64(debt.Quantity)
		state.shortValue += value
		state.securityFees += debt.AccruedFee
		ratio := defaultMarginRatio
		if pool, ok := state.pools[debt.StockCode]; ok {
			ratio = pool.SecuritiesMarginRatio
		}
		state.marginUsed += value * ratio
	}
	return state, nil
}

func (state *accountState) metrics() risk.Metrics {
	return risk.CalculateMetrics(risk.RiskContext{
		Cash:             state.account.Cash,
		LongMarketValue:  state.longValue,
		ShortMarketValue: state.shortValue,
		FinancePrincipal: state.financeTotal,
		FinanceInterest:  state.interestTotal,
		SecuritiesFee:    state.securityFees,
		MarginAvailable:  state.collateral - state.marginUsed,
	})
}

func (s *PaperMarginService) Snapshot(ctx context.Context, accountID uint, marks []PositionMark) (*Snapshot, error) {
	if s.db == nil {
		return nil, errors.New("paper margin database is nil")
	}
	state, err := s.loadState(s.db.WithContext(ctx), accountID, marks)
	if err != nil {
		return nil, err
	}
	snapshot := &Snapshot{
		Account:               state.account,
		MarginAccount:         state.margin,
		Positions:             state.positions,
		FinanceLiabilities:    state.finance,
		SecuritiesLiabilities: state.securities,
		Metrics:               state.metrics(),
	}
	tx := s.db.WithContext(ctx)
	_ = tx.Where("account_id = ?", state.account.ID).Order("id DESC").Limit(100).Find(&snapshot.Orders)
	_ = tx.Where("account_id = ?", state.account.ID).Order("id DESC").Limit(200).Find(&snapshot.Ledger)
	_ = tx.Where("account_id = ?", state.account.ID).Order("id DESC").Limit(100).Find(&snapshot.RiskEvents)
	return snapshot, nil
}

func paperFee(amount float64, sell bool) float64 {
	fee := math.Max(5, amount*0.00025)
	if sell {
		fee += amount * 0.0005
	}
	return fee
}

func (s *PaperMarginService) riskContext(state *accountState, req SubmitRequest) risk.RiskContext {
	var positionSellable int64
	for _, position := range state.positions {
		if position.StockCode == req.StockCode {
			positionSellable = position.Sellable
			break
		}
	}
	var financeDebt float64
	for _, debt := range state.finance {
		if debt.StockCode == req.StockCode {
			financeDebt = debt.Principal + debt.AccruedInterest
			break
		}
	}
	var securitiesDebt int64
	var repaymentFee float64
	for _, debt := range state.securities {
		if debt.StockCode == req.StockCode {
			securitiesDebt = debt.Quantity
			if debt.Quantity > 0 {
				repaymentFee = debt.AccruedFee * float64(req.Volume) / float64(debt.Quantity)
			}
			break
		}
	}
	availableCash := state.account.Cash
	if req.Kind == data.PaperMarginOrderNormalBuy || req.Kind == data.PaperMarginOrderBuyReturn {
		availableCash -= paperFee(req.Price*float64(req.Volume), false)
	}
	if req.Kind == data.PaperMarginOrderBuyReturn {
		availableCash -= repaymentFee
	}
	pool, poolFound := state.pools[req.StockCode]
	financeRatio := defaultMarginRatio
	securitiesRatio := defaultMarginRatio
	borrowAvailable := int64(0)
	if poolFound && pool.Enabled {
		financeRatio = pool.FinanceMarginRatio
		securitiesRatio = pool.SecuritiesMarginRatio
		borrowAvailable = pool.AvailableQuantity
	}
	orderAmount := req.Price * float64(req.Volume)
	equityBase := availableCash + state.longValue
	if equityBase <= 0 {
		equityBase = state.account.Cash + state.longValue
	}
	currentName := 0.0
	for _, position := range state.positions {
		if position.StockCode == req.StockCode {
			mark := req.Price
			if mark <= 0 {
				mark = position.AvgCost
			}
			currentName = mark * float64(position.Volume)
			break
		}
	}
	postGross := state.longValue + state.shortValue
	postName := currentName
	switch req.Kind {
	case data.PaperMarginOrderNormalBuy, data.PaperMarginOrderMarginBuy:
		postGross += orderAmount
		postName += orderAmount
	case data.PaperMarginOrderShortSell:
		postGross += orderAmount
		postName += orderAmount
	}
	return risk.RiskContext{
		AccountID:                 state.account.ID,
		AccountMode:               state.margin.Mode,
		Cash:                      availableCash,
		CollateralValue:           state.collateral,
		LongMarketValue:           state.longValue,
		ShortMarketValue:          state.shortValue,
		FinancePrincipal:          state.financeTotal,
		FinanceInterest:           state.interestTotal,
		SecuritiesFee:             state.securityFees,
		FinanceCreditAvailable:    math.Max(0, state.margin.FinanceCreditLimit-state.financeTotal),
		SecuritiesCreditAvailable: math.Max(0, state.margin.SecuritiesCreditLimit-state.shortValue),
		MarginAvailable:           state.collateral - state.marginUsed,
		PositionSellable:          positionSellable,
		FinanceDebt:               financeDebt,
		SecuritiesDebtQuantity:    securitiesDebt,
		BorrowAvailable:           borrowAvailable,
		FinanceMarginRatio:        financeRatio,
		SecuritiesMarginRatio:     securitiesRatio,
		WarningRatio:              state.margin.WarningRatio,
		CloseoutRatio:             state.margin.CloseoutRatio,
		MarketLevel:               req.MarketLevel,
		BlockNewEntries:           req.BlockNewEntries,
		MaxExposurePct:            req.MaxExposurePct,
		EquityBase:                equityBase,
		CurrentNameValue:          currentName,
		PostGrossExposure:         postGross,
		PostNameExposure:          postName,
		MaxSingleNamePct:          req.MaxSingleNamePct,
		MaxGrossExposurePct:       req.MaxGrossExposurePct,
		MaxDailyLossPct:           req.MaxDailyLossPct,
		CurrentDailyPnlPct:        req.CurrentDailyPnlPct,
		Order: risk.RiskOrder{
			Kind:      req.Kind,
			StockCode: req.StockCode,
			Price:     req.Price,
			Volume:    req.Volume,
		},
	}
}

func (s *PaperMarginService) Submit(ctx context.Context, req SubmitRequest) (*data.PaperMarginOrder, risk.RiskDecision, error) {
	var order data.PaperMarginOrder
	var decision risk.RiskDecision
	if s.db == nil {
		return nil, decision, errors.New("paper margin database is nil")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := s.loadState(tx, req.AccountID, []PositionMark{{StockCode: req.StockCode, Price: req.Price}})
		if err != nil {
			return err
		}
		req.AccountID = state.account.ID
		decision = risk.PreTradeCheck(s.riskContext(state, req))
		now := s.now()
		order = data.PaperMarginOrder{
			AccountID:  req.AccountID,
			Kind:       req.Kind,
			StockCode:  req.StockCode,
			StockName:  req.StockName,
			Status:     "rejected",
			Price:      req.Price,
			Volume:     req.Volume,
			Reason:     req.Reason,
			ReasonCode: string(decision.Code),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if !decision.Allowed {
			return tx.Create(&order).Error
		}
		order.Status = "filled"
		order.FilledAt = &now
		if err = tx.Create(&order).Error; err != nil {
			return err
		}
		if err = s.applyOrder(tx, state, &order); err != nil {
			return err
		}
		return s.refreshEquity(tx, state.account.ID)
	})
	return &order, decision, err
}

func (s *PaperMarginService) applyOrder(tx *gorm.DB, state *accountState, order *data.PaperMarginOrder) error {
	amount := order.Price * float64(order.Volume)
	cashBefore := state.account.Cash
	debtDelta := 0.0
	sell := order.Kind == data.PaperMarginOrderNormalSell || order.Kind == data.PaperMarginOrderSellRepay || order.Kind == data.PaperMarginOrderShortSell
	order.Fee = paperFee(amount, sell)
	if err := tx.Model(order).Update("fee", order.Fee).Error; err != nil {
		return err
	}
	switch order.Kind {
	case data.PaperMarginOrderNormalBuy:
		state.account.Cash -= amount + order.Fee
		if err := s.increasePosition(tx, order, false); err != nil {
			return err
		}
	case data.PaperMarginOrderNormalSell:
		state.account.Cash += amount - order.Fee
		if err := s.decreasePosition(tx, order); err != nil {
			return err
		}
	case data.PaperMarginOrderMarginBuy:
		state.account.Cash -= order.Fee
		debtDelta = amount
		if err := s.increasePosition(tx, order, false); err != nil {
			return err
		}
		if err := s.increaseFinanceDebt(tx, order, amount); err != nil {
			return err
		}
	case data.PaperMarginOrderSellRepay:
		if err := s.decreasePosition(tx, order); err != nil {
			return err
		}
		remainder, err := s.repayFinanceDebt(tx, order.AccountID, order.StockCode, amount-order.Fee)
		if err != nil {
			return err
		}
		state.account.Cash += remainder
		debtDelta = -(amount - order.Fee - remainder)
	case data.PaperMarginOrderShortSell:
		state.account.Cash += amount - order.Fee
		debtDelta = amount
		if err := s.increaseSecuritiesDebt(tx, order); err != nil {
			return err
		}
		if err := tx.Model(&data.PaperBorrowPool{}).Where("stock_code = ?", order.StockCode).
			Update("available_quantity", gorm.Expr("available_quantity - ?", order.Volume)).Error; err != nil {
			return err
		}
	case data.PaperMarginOrderBuyReturn:
		repaymentFee, err := s.returnSecuritiesDebt(tx, order)
		if err != nil {
			return err
		}
		state.account.Cash -= amount + order.Fee + repaymentFee
		debtDelta = -(amount + repaymentFee)
		if err = tx.Model(&data.PaperBorrowPool{}).Where("stock_code = ?", order.StockCode).
			Update("available_quantity", gorm.Expr("available_quantity + ?", order.Volume)).Error; err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported paper margin order kind %q", order.Kind)
	}
	if err := tx.Save(&state.account).Error; err != nil {
		return err
	}
	return tx.Create(&data.PaperMarginLedger{
		AccountID:   order.AccountID,
		OrderID:     order.ID,
		Type:        order.Kind,
		StockCode:   order.StockCode,
		CashDelta:   state.account.Cash - cashBefore,
		DebtDelta:   debtDelta,
		Quantity:    order.Volume,
		Description: "模拟两融成交（不连接真实券商）",
		OccurredAt:  s.now(),
	}).Error
}

func (s *PaperMarginService) increasePosition(tx *gorm.DB, order *data.PaperMarginOrder, sellable bool) error {
	var position data.PaperPosition
	err := tx.Where("account_id = ? AND stock_code = ?", order.AccountID, order.StockCode).First(&position).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		position = data.PaperPosition{AccountID: order.AccountID, StockCode: order.StockCode, StockName: order.StockName}
	} else if err != nil {
		return err
	}
	newVolume := position.Volume + order.Volume
	position.AvgCost = (position.AvgCost*float64(position.Volume) + order.Price*float64(order.Volume)) / float64(newVolume)
	position.Volume = newVolume
	if sellable {
		position.Sellable += order.Volume
	}
	return tx.Save(&position).Error
}

func (s *PaperMarginService) decreasePosition(tx *gorm.DB, order *data.PaperMarginOrder) error {
	var position data.PaperPosition
	if err := tx.Where("account_id = ? AND stock_code = ?", order.AccountID, order.StockCode).First(&position).Error; err != nil {
		return err
	}
	position.Volume -= order.Volume
	position.Sellable -= order.Volume
	if position.Volume == 0 {
		return tx.Delete(&position).Error
	}
	return tx.Save(&position).Error
}

func (s *PaperMarginService) increaseFinanceDebt(tx *gorm.DB, order *data.PaperMarginOrder, amount float64) error {
	var debt data.PaperFinanceLiability
	err := tx.Where("account_id = ? AND stock_code = ?", order.AccountID, order.StockCode).First(&debt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		debt = data.PaperFinanceLiability{AccountID: order.AccountID, StockCode: order.StockCode}
	} else if err != nil {
		return err
	}
	debt.Principal += amount
	debt.Quantity += order.Volume
	return tx.Save(&debt).Error
}

func (s *PaperMarginService) repayFinanceDebt(tx *gorm.DB, accountID uint, stockCode string, payment float64) (float64, error) {
	var debt data.PaperFinanceLiability
	if err := tx.Where("account_id = ? AND stock_code = ?", accountID, stockCode).First(&debt).Error; err != nil {
		return 0, err
	}
	applied := math.Min(payment, debt.AccruedInterest)
	debt.AccruedInterest -= applied
	payment -= applied
	applied = math.Min(payment, debt.Principal)
	debt.Principal -= applied
	payment -= applied
	if debt.Principal <= 0 && debt.AccruedInterest <= 0 {
		return payment, tx.Delete(&debt).Error
	}
	return payment, tx.Save(&debt).Error
}

func (s *PaperMarginService) increaseSecuritiesDebt(tx *gorm.DB, order *data.PaperMarginOrder) error {
	var debt data.PaperSecuritiesLiability
	err := tx.Where("account_id = ? AND stock_code = ?", order.AccountID, order.StockCode).First(&debt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		debt = data.PaperSecuritiesLiability{AccountID: order.AccountID, StockCode: order.StockCode, StockName: order.StockName}
	} else if err != nil {
		return err
	}
	newQuantity := debt.Quantity + order.Volume
	debt.AvgPrice = (debt.AvgPrice*float64(debt.Quantity) + order.Price*float64(order.Volume)) / float64(newQuantity)
	debt.Quantity = newQuantity
	return tx.Save(&debt).Error
}

func (s *PaperMarginService) returnSecuritiesDebt(tx *gorm.DB, order *data.PaperMarginOrder) (float64, error) {
	var debt data.PaperSecuritiesLiability
	if err := tx.Where("account_id = ? AND stock_code = ?", order.AccountID, order.StockCode).First(&debt).Error; err != nil {
		return 0, err
	}
	fee := debt.AccruedFee * float64(order.Volume) / float64(debt.Quantity)
	debt.AccruedFee -= fee
	debt.Quantity -= order.Volume
	if debt.Quantity == 0 {
		return fee, tx.Delete(&debt).Error
	}
	return fee, tx.Save(&debt).Error
}

func (s *PaperMarginService) refreshEquity(tx *gorm.DB, accountID uint) error {
	state, err := s.loadState(tx, accountID, nil)
	if err != nil {
		return err
	}
	state.account.Equity = math.Round((state.account.Cash+state.longValue-state.shortValue-state.financeTotal-state.interestTotal-state.securityFees)*100) / 100
	return tx.Save(&state.account).Error
}

func (s *PaperMarginService) AccrueInterest(ctx context.Context, accountID uint, asOf time.Time) (*AccrualResult, error) {
	result := &AccrualResult{AccountID: accountID, AccruedAt: asOf}
	if asOf.IsZero() {
		asOf = s.now()
		result.AccruedAt = asOf
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, margin, err := s.ensureAccount(tx, accountID)
		if err != nil {
			return err
		}
		result.AccountID = margin.AccountID
		from := asOf
		if margin.LastAccruedAt != nil {
			from = *margin.LastAccruedAt
		}
		if !asOf.After(from) {
			return nil
		}
		result.Days = asOf.Sub(from).Hours() / 24
		var finance []data.PaperFinanceLiability
		if err = tx.Where("account_id = ?", margin.AccountID).Find(&finance).Error; err != nil {
			return err
		}
		for i := range finance {
			accrual := finance[i].Principal * margin.FinanceAnnualRate * result.Days / 365
			finance[i].AccruedInterest += accrual
			result.FinanceInterest += accrual
			if err = tx.Save(&finance[i]).Error; err != nil {
				return err
			}
		}
		var securities []data.PaperSecuritiesLiability
		if err = tx.Where("account_id = ?", margin.AccountID).Find(&securities).Error; err != nil {
			return err
		}
		for i := range securities {
			accrual := securities[i].AvgPrice * float64(securities[i].Quantity) * margin.SecuritiesAnnualRate * result.Days / 365
			securities[i].AccruedFee += accrual
			result.SecuritiesFee += accrual
			if err = tx.Save(&securities[i]).Error; err != nil {
				return err
			}
		}
		margin.LastAccruedAt = &asOf
		if err = tx.Save(margin).Error; err != nil {
			return err
		}
		if result.FinanceInterest != 0 || result.SecuritiesFee != 0 {
			if err = tx.Create(&data.PaperMarginLedger{
				AccountID:   margin.AccountID,
				Type:        "interest_accrual",
				DebtDelta:   result.FinanceInterest + result.SecuritiesFee,
				Description: "模拟日终融资利息及融券费用计提（ACT/365）",
				OccurredAt:  asOf,
			}).Error; err != nil {
				return err
			}
		}
		return s.refreshEquity(tx, margin.AccountID)
	})
	return result, err
}

func (s *PaperMarginService) ScanRisk(ctx context.Context, accountID uint, marks []PositionMark, asOf time.Time) ([]data.PaperMarginRiskEvent, error) {
	if asOf.IsZero() {
		asOf = s.now()
	}
	state, err := s.loadState(s.db.WithContext(ctx), accountID, marks)
	if err != nil {
		return nil, err
	}
	metrics := state.metrics()
	var level string
	var code risk.ReasonCode
	var message string
	// 无负债时维持担保比例为 0（不适用），不触发警戒/平仓巡检。
	if metrics.TotalLiabilities <= 0 {
		return []data.PaperMarginRiskEvent{}, nil
	}
	if state.margin.CloseoutRatio > 0 && metrics.MaintenanceRatio < state.margin.CloseoutRatio {
		level, code, message = "closeout", risk.ReasonCloseoutTriggered, "低于模拟平仓线；仅为本地风险演练，不代表真实券商处置"
	} else if state.margin.WarningRatio > 0 && metrics.MaintenanceRatio < state.margin.WarningRatio {
		level, code, message = "warning", risk.ReasonWarningTriggered, "低于模拟警戒线；阈值可配置且非监管参数"
	} else {
		return []data.PaperMarginRiskEvent{}, nil
	}
	dayStart := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, asOf.Location())
	var count int64
	if err = s.db.WithContext(ctx).Model(&data.PaperMarginRiskEvent{}).
		Where("account_id = ? AND reason_code = ? AND occurred_at >= ?", state.account.ID, code, dayStart).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return []data.PaperMarginRiskEvent{}, nil
	}
	event := data.PaperMarginRiskEvent{
		AccountID:        state.account.ID,
		Level:            level,
		ReasonCode:       string(code),
		Message:          message,
		MaintenanceRatio: metrics.MaintenanceRatio,
		OccurredAt:       asOf,
	}
	if err = s.db.WithContext(ctx).Create(&event).Error; err != nil {
		return nil, err
	}
	return []data.PaperMarginRiskEvent{event}, nil
}
