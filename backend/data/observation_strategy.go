package data

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

const (
	ObservationQueryType = "observation"
	// ObservationCronExpr is registered only when the user enables cron.
	// Enabling cron does not set feedsTradePlan.
	ObservationCronExpr = "0 20 15 * * 1-5"
)

const (
	StrategyIDMaPullback  = "ext_ma_pullback"
	StrategyIDVolBreakout = "ext_vol_breakout"
	StrategyIDDdBounce    = "ext_dd_bounce"
)

// ObservationMeta is stored in stock_strategies.query_json.
// feedsTradePlan defaults to false. trackA is always false (no auto orders).
type ObservationMeta struct {
	StrategyID      string `json:"strategyId"`
	FeedsTradePlan  bool   `json:"feedsTradePlan"`
	ObservationOnly bool   `json:"observationOnly"`
	TrackA          bool   `json:"trackA"`
}

// ObservationStrategyDef is a built-in observation screen (not an ice-point preset).
type ObservationStrategyDef struct {
	StrategyID     string
	Name           string
	Blurb          string
	FeedsTradePlan bool
	EnableCron     bool
	TrackA         bool
}

// ObservationStrategies are the three buy-oriented day-bar screens.
// ext_xsmom_v1 is not in this repo, so it is not cloned here.
func ObservationStrategies() []ObservationStrategyDef {
	return []ObservationStrategyDef{
		{
			StrategyID:     StrategyIDMaPullback,
			Name:           "均线趋势回踩",
			Blurb:          "日线收盘站在上升的 20 日均线之上，且近 3 日内低点曾靠近该均线后收回阳线。只作观察名单，不是买卖指令。K 线不足、价格无效或均线未上升时跳过。",
			FeedsTradePlan: false,
			EnableCron:     false,
			TrackA:         false,
		},
		{
			StrategyID:     StrategyIDVolBreakout,
			Name:           "放量突破确认",
			Blurb:          "收盘价突破此前 20 日最高价，且当日成交量高于此前 20 日均量的 1.5 倍。只作观察名单，不是买卖指令。成交量缺失、为零或未放大时跳过。",
			FeedsTradePlan: false,
			EnableCron:     false,
			TrackA:         false,
		},
		{
			StrategyID:     StrategyIDDdBounce,
			Name:           "受控回撤反弹",
			Blurb:          "自近 20 日高点回撤约 8%–18% 后出现阳线反弹，且不是 60 日新低、RSI 不低于 30。只作观察名单，不是买卖指令。回撤过深、过浅，或与冰点新低/超卖重叠时跳过。",
			FeedsTradePlan: false,
			EnableCron:     false,
			TrackA:         false,
		},
	}
}

func IsObservationStrategyID(strategyID string) bool {
	switch strings.TrimSpace(strategyID) {
	case StrategyIDMaPullback, StrategyIDVolBreakout, StrategyIDDdBounce:
		return true
	default:
		return false
	}
}

func ObservationByID(strategyID string) *ObservationStrategyDef {
	strategyID = strings.TrimSpace(strategyID)
	for _, def := range ObservationStrategies() {
		if def.StrategyID == strategyID {
			cp := def
			return &cp
		}
	}
	return nil
}

func ParseObservationMeta(raw string) (ObservationMeta, bool) {
	meta := ObservationMeta{ObservationOnly: true}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return meta, false
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return ObservationMeta{ObservationOnly: true}, false
	}
	meta.StrategyID = strings.TrimSpace(meta.StrategyID)
	meta.TrackA = false
	if !meta.ObservationOnly {
		meta.ObservationOnly = true
	}
	return meta, true
}

func observationIDFromStrategy(s *models.StockStrategy) string {
	if s == nil {
		return ""
	}
	meta, ok := ParseObservationMeta(s.QueryJSON)
	if !ok {
		return ""
	}
	return meta.StrategyID
}

// StrategyFeedsTradePlan reports whether an enabled strategy may become the
// paper TradePlan universe source.
// Observation screens fail closed unless feedsTradePlan is explicitly true.
// Legacy strategies keep the previous behavior: enabled means eligible.
func StrategyFeedsTradePlan(s *models.StockStrategy) bool {
	if s == nil || !s.Enable {
		return false
	}
	id := observationIDFromStrategy(s)
	if s.QueryType == ObservationQueryType || IsObservationStrategyID(id) {
		meta, ok := ParseObservationMeta(s.QueryJSON)
		if !ok || !meta.FeedsTradePlan {
			return false
		}
		if s.QueryType == ObservationQueryType && !IsObservationStrategyID(meta.StrategyID) {
			return false
		}
		return true
	}
	return true
}

func normalizeObservationStrategy(s *models.StockStrategy) error {
	if s == nil || strings.TrimSpace(s.QueryType) != ObservationQueryType {
		return nil
	}
	meta, ok := ParseObservationMeta(s.QueryJSON)
	if !ok || !IsObservationStrategyID(meta.StrategyID) {
		return fmt.Errorf("未知观察策略")
	}
	def := ObservationByID(meta.StrategyID)
	raw, err := json.Marshal(ObservationMeta{
		StrategyID:      meta.StrategyID,
		FeedsTradePlan:  meta.FeedsTradePlan,
		ObservationOnly: true,
		TrackA:          false,
	})
	if err != nil {
		return err
	}
	s.QueryJSON = string(raw)
	s.QueryText = ""
	if strings.TrimSpace(s.Name) == "" && def != nil {
		s.Name = def.Name
	}
	if strings.TrimSpace(s.Description) == "" && def != nil {
		s.Description = def.Blurb
	}
	// Cron enable fills an expression when empty. It does not flip feedsTradePlan.
	if s.Enable && strings.TrimSpace(s.CronExpr) == "" {
		s.CronExpr = ObservationCronExpr
	}
	if s.PageSize <= 0 {
		s.PageSize = 50
	}
	return nil
}

func (a *StockStrategyApi) EnsureObservationStrategies() error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	for _, def := range ObservationStrategies() {
		var n int64
		pattern := `%"strategyId":"` + def.StrategyID + `"%`
		if err := db.Dao.Model(&models.StockStrategy{}).
			Where("query_type = ? AND query_json LIKE ?", ObservationQueryType, pattern).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		raw, err := json.Marshal(ObservationMeta{
			StrategyID:      def.StrategyID,
			FeedsTradePlan:  false,
			ObservationOnly: true,
			TrackA:          false,
		})
		if err != nil {
			return err
		}
		row := &models.StockStrategy{
			Name:        def.Name,
			QueryType:   ObservationQueryType,
			QueryJSON:   string(raw),
			Description: def.Blurb,
			Enable:      false,
			CronExpr:    "",
			PageSize:    50,
		}
		if err := db.Dao.Create(row).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetFirstTradePlanSource returns the first enabled strategy that is allowed
// to feed the paper TradePlan universe. Observation rows with feedsTradePlan
// false are skipped even when cron is enabled.
func (a *StockStrategyApi) GetFirstTradePlanSource() (*models.StockStrategy, error) {
	var list []models.StockStrategy
	if err := db.Dao.Where("enable = ?", true).Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		if StrategyFeedsTradePlan(&list[i]) {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("no trade-plan strategy source")
}
