package positionstate

import (
	"strings"
	"time"
)

// State codes (S0–S4).
const (
	S0NoPosition       = "S0_NO_POSITION"
	S1NewLocked        = "S1_NEW_LOCKED"
	S2Available        = "S2_AVAILABLE"
	S3PartialLocked    = "S3_PARTIAL_LOCKED"
	S4ReducedAvailable = "S4_REDUCED_AVAILABLE"
)

// Risk tags (observation only).
const (
	RiskTagNone        = ""
	RiskTagNewLocked   = "NEW_LOCKED_T1"
	RiskTagPartialLock = "PARTIAL_T1_LOCK"
	RiskTagStaleLock   = "STALE_LOCK_NOT_NEW"
	RiskTagReduced     = "REDUCED_AFTER_SELL"
	RiskTagNoPosition  = "NO_POSITION"
)

// LotRecord is one buy or sell lot used for age / unlock semantics.
// Quantity is canonical qty; when loaded from DB it MUST come from paper_sim_fills.volume.
type LotRecord struct {
	TradeDate string `json:"trade_date"`
	Quantity  int64  `json:"quantity"` // canonical qty
	Side      string `json:"side,omitempty"` // BUY|SELL optional
}

// SnapshotInput is the calculator input (qty + lots + dates).
type SnapshotInput struct {
	Symbol       string      `json:"symbol"`
	TotalQty     int64       `json:"total_qty"`
	AvailableQty int64       `json:"available_qty"`
	LockedQty    *int64      `json:"locked_qty,omitempty"`
	BuyRecords   []LotRecord `json:"buy_records"`
	SellRecords  []LotRecord `json:"sell_records"`
	TradeDate    string      `json:"trade_date"`
	CurrentDate  string      `json:"current_date"`
}

// PositionStateView is the unified output consumed by Home / Monitor / Risk labels.
type PositionStateView struct {
	Symbol        string `json:"symbol"`
	TotalQty      int64  `json:"total_qty"`
	AvailableQty  int64  `json:"available_qty"`
	LockedQty     int64  `json:"locked_qty"`
	State         string `json:"state"`
	HoldingDays   int    `json:"holding_days"`
	IsNewPosition bool   `json:"is_new_position"`
	CanSell       bool   `json:"can_sell"`
	RiskTag       string `json:"risk_tag,omitempty"`
	FirstBuyDate  string `json:"first_buy_date,omitempty"`
	Explanation   string `json:"explanation,omitempty"`
}

// Bundle is a multi-position envelope for API / Home / Monitor.
type Bundle struct {
	TradeDate      string              `json:"trade_date"`
	AsOf           time.Time           `json:"as_of"`
	Positions      []PositionStateView `json:"positions"`
	DataSourceNote string              `json:"data_source_note"`
	Disclaimer     string              `json:"disclaimer"`
}

const (
	dataSourceNote = "PositionState K-Beta · unified calculate_position_state; T+1 via buy_records / available+locked; no trade execution"
	disclaimer     = "持仓状态机（只读计算）。不生成买卖指令，不修改 Gateway / TradingEvent emit。"
)

func earliestBuyDate(buys []LotRecord) string {
	best := ""
	for _, b := range buys {
		d := strings.TrimSpace(b.TradeDate)
		if d == "" || b.Quantity <= 0 {
			continue
		}
		side := strings.ToUpper(strings.TrimSpace(b.Side))
		if side != "" && side != "BUY" {
			continue
		}
		if best == "" || d < best {
			best = d
		}
	}
	return best
}

func hasPositiveLots(lots []LotRecord) bool {
	for _, l := range lots {
		if l.Quantity > 0 && strings.TrimSpace(l.TradeDate) != "" {
			side := strings.ToUpper(strings.TrimSpace(l.Side))
			if side == "" || side == "SELL" {
				return true
			}
		}
	}
	return false
}

func computeHoldingDays(firstBuy, current string) int {
	if firstBuy == "" || current == "" {
		return 0
	}
	a, err1 := time.Parse("2006-01-02", firstBuy)
	b, err2 := time.Parse("2006-01-02", current)
	if err1 != nil || err2 != nil {
		return 0
	}
	if b.Before(a) {
		return 0
	}
	return int(b.Sub(a).Hours() / 24)
}

// UnlockQtyForSymbol sums buy lots on previousTradeDay (release candidates).
func UnlockQtyForSymbol(buys []LotRecord, previousTradeDay string) int64 {
	prev := strings.TrimSpace(previousTradeDay)
	var sum int64
	for _, b := range buys {
		if strings.TrimSpace(b.TradeDate) != prev || b.Quantity <= 0 {
			continue
		}
		side := strings.ToUpper(strings.TrimSpace(b.Side))
		if side != "" && side != "BUY" {
			continue
		}
		sum += b.Quantity
	}
	return sum
}

// ApplyUnlockToQty is a pure T+1 release helper for Settlement Engine planning/tests.
// Moves min(locked, unlockQty) from locked → available.
func ApplyUnlockToQty(total, available, locked, unlockQty int64) (newAvail, newLocked int64) {
	if unlockQty < 0 {
		unlockQty = 0
	}
	if locked < 0 {
		locked = 0
	}
	if available < 0 {
		available = 0
	}
	u := unlockQty
	if u > locked {
		u = locked
	}
	return available + u, locked - u
}
