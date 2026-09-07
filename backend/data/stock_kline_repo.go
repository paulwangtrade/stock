package data

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KLineBar 统一写入/查询 DTO（日线与分钟共用）。
type KLineBar struct {
	Market         string
	TSCode         string
	Symbol         string
	Period         string
	AdjustType     string
	BarTime        time.Time
	Open           float64
	High           float64
	Low            float64
	Close          float64
	Volume         float64
	Amount         *float64
	ChangePct      *float64
	ChangeVal      *float64
	Amplitude      *float64
	TurnoverRate   *float64
	Source         string
	SourceSecID    string
	SourceSinaCode string
	FetchedAt      *time.Time
}

// KLineQuery 区间查询条件。
type KLineQuery struct {
	Market     string
	TSCode     string
	Period     string
	AdjustType string // 空则默认 none
	Start      *time.Time
	End        *time.Time
	Limit      int // 0 表示不限制（仍建议调用方设上限）
	OrderAsc   bool
}

// StockKLineRepo 正式 K 线仓储。
type StockKLineRepo struct {
	db *gorm.DB
}

func NewStockKLineRepo() *StockKLineRepo {
	return &StockKLineRepo{db: db.Dao}
}

func NewStockKLineRepoWithDB(database *gorm.DB) *StockKLineRepo {
	return &StockKLineRepo{db: database}
}

func normalizeAdjustType(adjust string) string {
	a := strings.ToLower(strings.TrimSpace(adjust))
	switch a {
	case "", KLineAdjustNone:
		return KLineAdjustNone
	case KLineAdjustQFQ, "1", "forward":
		return KLineAdjustQFQ
	case KLineAdjustHFQ, "2", "backward":
		return KLineAdjustHFQ
	default:
		return a
	}
}

func (r *StockKLineRepo) prepareBar(bar *KLineBar) error {
	if bar == nil {
		return errors.New("nil bar")
	}
	bar.Market = strings.ToUpper(strings.TrimSpace(bar.Market))
	bar.TSCode = strings.TrimSpace(bar.TSCode)
	bar.Symbol = strings.TrimSpace(bar.Symbol)
	bar.Period = strings.TrimSpace(bar.Period)
	bar.AdjustType = normalizeAdjustType(bar.AdjustType)
	if bar.Market == "" || bar.TSCode == "" || bar.Period == "" {
		return fmt.Errorf("market/ts_code/period required")
	}
	if bar.BarTime.IsZero() {
		return fmt.Errorf("bar_time required")
	}
	if bar.Symbol == "" {
		if n, err := NormalizeStockCode(bar.TSCode); err == nil {
			bar.Symbol = n.Symbol
			if bar.Market == "" {
				bar.Market = n.Market
			}
		}
	}
	if IsDayKLinePeriod(bar.Period) {
		// 日/周/月：归一到当日 00:00:00（保留 Loc，不强制 UTC）
		t := bar.BarTime
		bar.BarTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
	return nil
}

// UpsertBars 按周期路由到 day/minute 表；唯一键冲突时更新行情字段。
func (r *StockKLineRepo) UpsertBars(bars []KLineBar) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("kline repo db is nil")
	}
	if len(bars) == 0 {
		return 0, nil
	}
	dayRows := make([]StockKLineDay, 0)
	minRows := make([]StockKLineMinute, 0)
	now := time.Now()
	for i := range bars {
		bar := bars[i]
		if err := r.prepareBar(&bar); err != nil {
			return 0, err
		}
		fetched := bar.FetchedAt
		if fetched == nil {
			fetched = &now
		}
		switch {
		case IsDayKLinePeriod(bar.Period):
			dayRows = append(dayRows, StockKLineDay{
				Market: bar.Market, TSCode: bar.TSCode, Symbol: bar.Symbol,
				Period: bar.Period, AdjustType: bar.AdjustType, BarTime: bar.BarTime,
				Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close, Volume: bar.Volume,
				Amount: bar.Amount, ChangePct: bar.ChangePct, ChangeVal: bar.ChangeVal,
				Amplitude: bar.Amplitude, TurnoverRate: bar.TurnoverRate,
				Source: bar.Source, SourceSecID: bar.SourceSecID, SourceSinaCode: bar.SourceSinaCode,
				FetchedAt: fetched,
			})
		case IsMinuteKLinePeriod(bar.Period):
			minRows = append(minRows, StockKLineMinute{
				Market: bar.Market, TSCode: bar.TSCode, Symbol: bar.Symbol,
				Period: bar.Period, AdjustType: bar.AdjustType, BarTime: bar.BarTime,
				Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close, Volume: bar.Volume,
				Amount: bar.Amount, ChangePct: bar.ChangePct, ChangeVal: bar.ChangeVal,
				Amplitude: bar.Amplitude, TurnoverRate: bar.TurnoverRate,
				Source: bar.Source, SourceSecID: bar.SourceSecID, SourceSinaCode: bar.SourceSinaCode,
				FetchedAt: fetched,
			})
		default:
			return 0, fmt.Errorf("unsupported kline period: %s", bar.Period)
		}
	}

	n := 0
	if len(dayRows) > 0 {
		err := r.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "market"}, {Name: "ts_code"}, {Name: "period"},
				{Name: "adjust_type"}, {Name: "bar_time"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"symbol", "open", "high", "low", "close", "volume", "amount",
				"change_pct", "change_val", "amplitude", "turnover_rate",
				"source", "source_secid", "source_sina_code", "fetched_at", "updated_at",
			}),
		}).CreateInBatches(&dayRows, 200).Error
		if err != nil {
			return n, err
		}
		n += len(dayRows)
	}
	if len(minRows) > 0 {
		err := r.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "market"}, {Name: "ts_code"}, {Name: "period"},
				{Name: "adjust_type"}, {Name: "bar_time"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"symbol", "open", "high", "low", "close", "volume", "amount",
				"change_pct", "change_val", "amplitude", "turnover_rate",
				"source", "source_secid", "source_sina_code", "fetched_at", "updated_at",
			}),
		}).CreateInBatches(&minRows, 200).Error
		if err != nil {
			return n, err
		}
		n += len(minRows)
	}
	return n, nil
}

// QueryBars 按股票+周期+复权+时间范围查询；路由 day/minute 表。
func (r *StockKLineRepo) QueryBars(q KLineQuery) ([]KLineBar, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("kline repo db is nil")
	}
	q.TSCode = strings.TrimSpace(q.TSCode)
	q.Period = strings.TrimSpace(q.Period)
	q.Market = strings.ToUpper(strings.TrimSpace(q.Market))
	q.AdjustType = normalizeAdjustType(q.AdjustType)
	if q.TSCode == "" || q.Period == "" {
		return nil, fmt.Errorf("ts_code and period required")
	}
	if q.Market == "" {
		if n, err := NormalizeStockCode(q.TSCode); err == nil {
			q.Market = n.Market
		}
	}

	switch {
	case IsDayKLinePeriod(q.Period):
		return r.queryDayBars(q)
	case IsMinuteKLinePeriod(q.Period):
		return r.queryMinuteBars(q)
	default:
		return nil, fmt.Errorf("unsupported kline period: %s", q.Period)
	}
}

func (r *StockKLineRepo) queryDayBars(q KLineQuery) ([]KLineBar, error) {
	tx := r.db.Model(&StockKLineDay{}).
		Where("ts_code = ? AND period = ? AND adjust_type = ?", q.TSCode, q.Period, q.AdjustType)
	if q.Market != "" {
		tx = tx.Where("market = ?", q.Market)
	}
	if q.Start != nil {
		tx = tx.Where("bar_time >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("bar_time <= ?", *q.End)
	}
	if q.OrderAsc {
		tx = tx.Order("bar_time asc")
	} else {
		tx = tx.Order("bar_time desc")
	}
	if q.Limit > 0 {
		tx = tx.Limit(q.Limit)
	}
	var rows []StockKLineDay
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]KLineBar, 0, len(rows))
	for _, row := range rows {
		out = append(out, KLineBar{
			Market: row.Market, TSCode: row.TSCode, Symbol: row.Symbol,
			Period: row.Period, AdjustType: row.AdjustType, BarTime: row.BarTime,
			Open: row.Open, High: row.High, Low: row.Low, Close: row.Close, Volume: row.Volume,
			Amount: row.Amount, ChangePct: row.ChangePct, ChangeVal: row.ChangeVal,
			Amplitude: row.Amplitude, TurnoverRate: row.TurnoverRate,
			Source: row.Source, SourceSecID: row.SourceSecID, SourceSinaCode: row.SourceSinaCode,
			FetchedAt: row.FetchedAt,
		})
	}
	return out, nil
}

func (r *StockKLineRepo) queryMinuteBars(q KLineQuery) ([]KLineBar, error) {
	tx := r.db.Model(&StockKLineMinute{}).
		Where("ts_code = ? AND period = ? AND adjust_type = ?", q.TSCode, q.Period, q.AdjustType)
	if q.Market != "" {
		tx = tx.Where("market = ?", q.Market)
	}
	if q.Start != nil {
		tx = tx.Where("bar_time >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("bar_time <= ?", *q.End)
	}
	if q.OrderAsc {
		tx = tx.Order("bar_time asc")
	} else {
		tx = tx.Order("bar_time desc")
	}
	if q.Limit > 0 {
		tx = tx.Limit(q.Limit)
	}
	var rows []StockKLineMinute
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]KLineBar, 0, len(rows))
	for _, row := range rows {
		out = append(out, KLineBar{
			Market: row.Market, TSCode: row.TSCode, Symbol: row.Symbol,
			Period: row.Period, AdjustType: row.AdjustType, BarTime: row.BarTime,
			Open: row.Open, High: row.High, Low: row.Low, Close: row.Close, Volume: row.Volume,
			Amount: row.Amount, ChangePct: row.ChangePct, ChangeVal: row.ChangeVal,
			Amplitude: row.Amplitude, TurnoverRate: row.TurnoverRate,
			Source: row.Source, SourceSecID: row.SourceSecID, SourceSinaCode: row.SourceSinaCode,
			FetchedAt: row.FetchedAt,
		})
	}
	return out, nil
}

// UpsertSyncState 更新增量水位。
func (r *StockKLineRepo) UpsertSyncState(state StockKLineSyncState) error {
	if r == nil || r.db == nil {
		return errors.New("kline repo db is nil")
	}
	state.AdjustType = normalizeAdjustType(state.AdjustType)
	state.Market = strings.ToUpper(strings.TrimSpace(state.Market))
	if state.Market == "" || state.TSCode == "" || state.Period == "" || state.LastBarTime.IsZero() {
		return fmt.Errorf("market/ts_code/period/last_bar_time required")
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "market"}, {Name: "ts_code"}, {Name: "period"}, {Name: "adjust_type"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"last_bar_time", "last_success_at", "last_error",
			"source", "source_secid", "source_sina_code", "updated_at",
		}),
	}).Create(&state).Error
}

// GetSyncState 读取水位；不存在返回 nil, nil。
func (r *StockKLineRepo) GetSyncState(market, tsCode, period, adjustType string) (*StockKLineSyncState, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("kline repo db is nil")
	}
	var row StockKLineSyncState
	err := r.db.Where(
		"market = ? AND ts_code = ? AND period = ? AND adjust_type = ?",
		strings.ToUpper(strings.TrimSpace(market)),
		strings.TrimSpace(tsCode),
		strings.TrimSpace(period),
		normalizeAdjustType(adjustType),
	).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
