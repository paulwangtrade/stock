// Dev-only PaperSim account cash tools (Phase10-C Observation Test Preparation).
// Touches paper_sim_accounts only. Never paper_* legacy, Execution, or Gateway.
package papertrading

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/tradingconfig"

	"gorm.io/gorm"
)

const (
	// DevAccountEnv enables SetCash / ResetPaperSimAccount outside unit tests.
	DevAccountEnv = "GO_STOCK_PAPER_SIM_DEV"
	defaultSimName = "paper_sim_default"
)

var (
	devToolsMu      sync.RWMutex
	devToolsForced  *bool // tests only
)

// SetDevAccountToolsForTest overrides the env gate (nil = use env).
func SetDevAccountToolsForTest(enabled *bool) {
	devToolsMu.Lock()
	devToolsForced = enabled
	devToolsMu.Unlock()
}

// DevAccountToolsEnabled reports whether cash adjustment APIs may run.
func DevAccountToolsEnabled() bool {
	devToolsMu.RLock()
	forced := devToolsForced
	devToolsMu.RUnlock()
	if forced != nil {
		return *forced
	}
	v := strings.TrimSpace(os.Getenv(DevAccountEnv))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func requireDevAccountTools() error {
	if DevAccountToolsEnabled() {
		return nil
	}
	return fmt.Errorf("paper_sim dev tools disabled: set %s=1 (dev/observation prep only)", DevAccountEnv)
}

// DevAccountResult is the post-adjustment snapshot (paper_sim_accounts only).
type DevAccountResult struct {
	OK           bool    `json:"ok"`
	Operation    string  `json:"operation"`
	AccountID    uint    `json:"accountId"`
	Name         string  `json:"name"`
	Cash         float64 `json:"cash"`
	InitialCash  float64 `json:"initialCash"`
	Equity       float64 `json:"equity"`
	MarketValue  float64 `json:"marketValue"`
	Message      string  `json:"message"`
	Error        string  `json:"error,omitempty"`
}

// SetPaperSimCash sets cash on paper_sim_default (creates row if missing).
// Equity := cash + market_value. Does not touch orders/fills/positions rows,
// paper_* legacy, Execution, or Gateway.
func SetPaperSimCash(cash float64) (*DevAccountResult, error) {
	if err := requireDevAccountTools(); err != nil {
		return failDev("SetPaperSimCash", err), err
	}
	if cash < 0 {
		err := fmt.Errorf("cash must be >= 0")
		return failDev("SetPaperSimCash", err), err
	}
	if err := EnsureSchema(nil); err != nil {
		return failDev("SetPaperSimCash", err), err
	}
	if db.Dao == nil {
		err := fmt.Errorf("database not initialized")
		return failDev("SetPaperSimCash", err), err
	}

	before, acc, err := loadOrCreateDefaultSimAccount()
	if err != nil {
		return failDev("SetPaperSimCash", err), err
	}
	prevCash := acc.Cash
	acc.Cash = cash
	acc.Equity = cash + acc.MarketValue
	acc.UpdatedAt = time.Now()
	if err := db.Dao.Model(&PaperSimAccount{}).Where("id = ?", acc.ID).Updates(map[string]any{
		"cash":       acc.Cash,
		"equity":     acc.Equity,
		"updated_at": acc.UpdatedAt,
	}).Error; err != nil {
		logger.SugaredLogger.Errorf(
			"PaperSimDev SetPaperSimCash FAILED account_id=%d before_cash=%.2f want=%.2f err=%v",
			acc.ID, prevCash, cash, err,
		)
		return failDev("SetPaperSimCash", err), err
	}

	logger.SugaredLogger.Infof(
		"PaperSimDev SetPaperSimCash OK account_id=%d name=%s before_cash=%.2f after_cash=%.2f equity=%.2f market_value=%.2f created=%v",
		acc.ID, acc.Name, before.Cash, acc.Cash, acc.Equity, acc.MarketValue, before.ID == 0,
	)
	return okDev("SetPaperSimCash", acc, fmt.Sprintf("cash %.2f → %.2f", prevCash, cash)), nil
}

// ResetPaperSimAccount sets initial_cash and cash to the given seed and
// recomputes equity = cash + market_value. Positions/orders/fills are kept.
// paper_* legacy / Execution / Gateway are untouched.
func ResetPaperSimAccount(initialCash float64) (*DevAccountResult, error) {
	if err := requireDevAccountTools(); err != nil {
		return failDev("ResetPaperSimAccount", err), err
	}
	if initialCash <= 0 {
		initialCash = tradingconfig.DefaultPaperInitialCash
		if initialCash <= 0 {
			initialCash = DefaultInitialCash
		}
	}
	if err := EnsureSchema(nil); err != nil {
		return failDev("ResetPaperSimAccount", err), err
	}
	if db.Dao == nil {
		err := fmt.Errorf("database not initialized")
		return failDev("ResetPaperSimAccount", err), err
	}

	before, acc, err := loadOrCreateDefaultSimAccount()
	if err != nil {
		return failDev("ResetPaperSimAccount", err), err
	}
	prevCash, prevInit := acc.Cash, acc.InitialCash
	acc.InitialCash = initialCash
	acc.Cash = initialCash
	acc.Equity = initialCash + acc.MarketValue
	acc.UpdatedAt = time.Now()
	if err := db.Dao.Model(&PaperSimAccount{}).Where("id = ?", acc.ID).Updates(map[string]any{
		"initial_cash": acc.InitialCash,
		"cash":         acc.Cash,
		"equity":       acc.Equity,
		"updated_at":   acc.UpdatedAt,
	}).Error; err != nil {
		logger.SugaredLogger.Errorf(
			"PaperSimDev ResetPaperSimAccount FAILED account_id=%d before_cash=%.2f want=%.2f err=%v",
			acc.ID, prevCash, initialCash, err,
		)
		return failDev("ResetPaperSimAccount", err), err
	}

	logger.SugaredLogger.Infof(
		"PaperSimDev ResetPaperSimAccount OK account_id=%d name=%s before_cash=%.2f before_initial=%.2f after_cash=%.2f after_initial=%.2f equity=%.2f market_value=%.2f",
		acc.ID, acc.Name, prevCash, prevInit, acc.Cash, acc.InitialCash, acc.Equity, acc.MarketValue,
	)
	_ = before
	return okDev("ResetPaperSimAccount", acc, fmt.Sprintf("reset cash/initial to %.2f", initialCash)), nil
}

// GetPaperSimDefaultAccount returns the Track-B default account (read-only).
func GetPaperSimDefaultAccount() (*PaperSimAccount, error) {
	if err := EnsureSchema(nil); err != nil {
		return nil, err
	}
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var acc PaperSimAccount
	err := db.Dao.Where("name = ?", defaultSimName).First(&acc).Error
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

type accountSnapshot struct {
	ID   uint
	Cash float64
}

func loadOrCreateDefaultSimAccount() (accountSnapshot, *PaperSimAccount, error) {
	var acc PaperSimAccount
	err := db.Dao.Where("name = ?", defaultSimName).First(&acc).Error
	if err == nil {
		return accountSnapshot{ID: acc.ID, Cash: acc.Cash}, &acc, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return accountSnapshot{}, nil, err
	}
	initial := tradingconfig.Default().PaperInitialCash()
	if initial <= 0 {
		initial = DefaultInitialCash
	}
	now := time.Now()
	acc = PaperSimAccount{
		Name:        defaultSimName,
		InitialCash: initial,
		Cash:        initial,
		Equity:      initial,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Dao.Create(&acc).Error; err != nil {
		return accountSnapshot{}, nil, err
	}
	logger.SugaredLogger.Infof(
		"PaperSimDev created paper_sim_default account_id=%d initial_cash=%.2f",
		acc.ID, initial,
	)
	return accountSnapshot{ID: acc.ID, Cash: acc.Cash}, &acc, nil
}

func okDev(op string, acc *PaperSimAccount, msg string) *DevAccountResult {
	return &DevAccountResult{
		OK:          true,
		Operation:   op,
		AccountID:   acc.ID,
		Name:        acc.Name,
		Cash:        acc.Cash,
		InitialCash: acc.InitialCash,
		Equity:      acc.Equity,
		MarketValue: acc.MarketValue,
		Message:     msg,
	}
}

func failDev(op string, err error) *DevAccountResult {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &DevAccountResult{OK: false, Operation: op, Error: msg, Message: msg}
}
