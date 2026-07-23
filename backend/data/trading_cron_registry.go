package data

import (
	"strings"
	"sync"
)

// Trading cron keys observed for Production Readiness (readonly).
// Spec / AddFunc bodies are owned by App Init*; this registry only mirrors setCronEntry.
const (
	TradingCronKeyAfterClose = "after_close_plan_workflow"
	TradingCronKeyMorning    = "paper_daily_plan"
)

var (
	tradingCronMu         sync.RWMutex
	tradingCronRegistered = map[string]bool{}
)

// ReportTradingCronRegistered records that a cron key was registered in-process.
// Safe to call from App.setCronEntry; does not start jobs or change Spec.
func ReportTradingCronRegistered(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	tradingCronMu.Lock()
	tradingCronRegistered[key] = true
	tradingCronMu.Unlock()
}

// IsTradingCronRegistered reports whether key was observed via ReportTradingCronRegistered.
func IsTradingCronRegistered(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	tradingCronMu.RLock()
	defer tradingCronMu.RUnlock()
	return tradingCronRegistered[key]
}

// GetTradingDayCronStatus returns after_close / morning registration flags (readonly).
func GetTradingDayCronStatus() TradingDayCronView {
	return TradingDayCronView{
		AfterCloseRegistered: IsTradingCronRegistered(TradingCronKeyAfterClose),
		MorningRegistered:    IsTradingCronRegistered(TradingCronKeyMorning),
	}
}

// ResetTradingCronRegistryForTest clears the in-memory map (tests only).
func ResetTradingCronRegistryForTest() {
	tradingCronMu.Lock()
	tradingCronRegistered = map[string]bool{}
	tradingCronMu.Unlock()
}
