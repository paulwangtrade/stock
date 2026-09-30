package decisiontimeline

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/papertrading"

	"gorm.io/gorm"
)

const (
	defaultSnapshotLimit = 60
	maxSnapshotLimit     = 120
	maxEvents            = 200
)

var shanghai = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

// Build assembles a chronological evidence trail. It only reads.
func Build(stockCode string, opt Options) (*Timeline, error) {
	norm, err := data.NormalizeStockCode(stockCode)
	if err != nil || strings.TrimSpace(norm.Symbol) == "" {
		return nil, ErrInvalidStockCode
	}
	now := time.Now().In(shanghai)
	tl := &Timeline{
		StockCode:       opportunity.NormalizeReadStockCode(stockCode),
		Disclaimer:      Disclaimer,
		ObservationOnly: true,
		Events:          []Event{},
		GeneratedAt:     now.Format(time.RFC3339),
	}
	if tl.StockCode == "" {
		tl.StockCode = strings.ToLower(strings.TrimSpace(norm.SinaCode))
	}
	if db.Dao == nil {
		tl.Sources = []SourceStatus{
			unavailable(SourceSignal, "本地库不可用，无法读取信号快照"),
			unavailable(SourceWatchFollow, "本地库不可用，无法读取关注或自选记录"),
			unavailable(SourcePaperSim, "本地库不可用，无法读取模拟盘记录"),
			unavailable(SourceExternalMirror, "本地库不可用，无法读取 external_mirror"),
			unavailable(SourceOutcome, "本地库不可用，无法读取结果备注"),
		}
		return tl, nil
	}

	keys := lookupKeys(stockCode, norm)
	limit := opt.SnapshotLimit
	if limit <= 0 {
		limit = defaultSnapshotLimit
	}
	if limit > maxSnapshotLimit {
		limit = maxSnapshotLimit
	}

	var events []Event
	var name string

	sigEvents, sigStatus, sigName := collectSignals(db.Dao, norm, limit)
	events = append(events, sigEvents...)
	name = firstName(name, sigName)

	watchEvents, watchStatus, watchName := collectWatchFollow(db.Dao, keys)
	events = append(events, watchEvents...)
	name = firstName(name, watchName)

	paperEvents, paperStatus, paperName := collectPaperSim(db.Dao, keys)
	events = append(events, paperEvents...)
	name = firstName(name, paperName)

	mirrorEvents, mirrorStatus := collectExternalMirror(db.Dao, keys)
	events = append(events, mirrorEvents...)

	noteEvents, noteStatus, noteName := collectOutcomeNotes(db.Dao, stockCode, keys)
	events = append(events, noteEvents...)
	name = firstName(name, noteName)

	sortEvents(events)
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}
	tl.StockName = name
	tl.Events = events
	tl.Sources = []SourceStatus{sigStatus, watchStatus, paperStatus, mirrorStatus, noteStatus}
	return tl, nil
}

func collectSignals(gdb *gorm.DB, norm data.NormalizedStockCode, limit int) ([]Event, SourceStatus, string) {
	if gdb == nil || !gdb.Migrator().HasTable(&models.SignalScanSnapshot{}) {
		return nil, unavailable(SourceSignal, "没有信号快照表，无法回顾冰/趋等标签"), ""
	}
	patterns := signalLikePatterns(norm)
	if len(patterns) == 0 {
		return nil, emptyStatus(SourceSignal, "无法为该代码构造快照检索"), ""
	}
	q := gdb.Model(&models.SignalScanSnapshot{}).
		Select("id", "trade_date", "session", "strategy_id", "strategy_name", "scope", "result_json", "status").
		Where("status = ?", "done")
	var parts []string
	var args []any
	for _, p := range patterns {
		parts = append(parts, "result_json LIKE ?")
		args = append(args, "%"+p+"%")
	}
	q = q.Where(strings.Join(parts, " OR "), args...).
		Order("trade_date DESC, id DESC").
		Limit(limit)

	var snaps []models.SignalScanSnapshot
	if err := q.Find(&snaps).Error; err != nil {
		return nil, unavailable(SourceSignal, "读取信号快照失败"), ""
	}
	repo := data.NewSignalSnapshotRepo()
	var events []Event
	name := ""
	for i := range snaps {
		snap := snaps[i]
		hits := repo.ParseSnapshotHits(&snap)
		for _, hit := range hits {
			if !hitMatches(hit, norm) {
				continue
			}
			tag := strings.TrimSpace(hit.Tag)
			if tag == "" {
				tag = strings.TrimSpace(hit.StatusText)
			}
			if tag == "" {
				continue
			}
			if name == "" {
				name = strings.TrimSpace(hit.SECURITY_NAME_ABBR)
			}
			day := strings.TrimSpace(snap.TradeDate)
			if len(day) >= 10 {
				day = day[:10]
			}
			strategy := strings.TrimSpace(snap.StrategyName)
			if strategy == "" {
				strategy = strings.TrimSpace(snap.StrategyID)
			}
			if strategy == "" {
				strategy = "未命名策略"
			}
			detail := fmt.Sprintf("%s · %s快照命中「%s」。这是当时的观察标签，不是买入指令。",
				strategy, sessionLabel(snap.Session), tag)
			if hit.SignalPrice > 0 {
				detail += fmt.Sprintf(" 信号价 %.4g。", hit.SignalPrice)
			}
			at := strings.TrimSpace(hit.SignalTime)
			if at == "" && day != "" {
				at = day
			}
			events = append(events, Event{
				ID:         fmt.Sprintf("signal:%d:%s", snap.ID, tag),
				Kind:       KindSignal,
				Lane:       LaneObserve,
				OccurredOn: day,
				OccurredAt: at,
				Title:      "信号 · " + tag,
				Detail:     detail,
				Tag:        tag,
				Source:     fmt.Sprintf("signal_scan_snapshots#%d", snap.ID),
				KlineDate:  day,
			})
		}
	}
	if len(events) == 0 {
		return nil, emptyStatus(SourceSignal, "已查近期完成的信号快照，没有这只股票的冰/趋或其他标签"), name
	}
	st := okStatus(SourceSignal, "来自信号扫描快照")
	st.Count = len(events)
	return events, st, name
}

func collectWatchFollow(gdb *gorm.DB, keys []string) ([]Event, SourceStatus, string) {
	hasActions := gdb.Migrator().HasTable(&opportunity.UserOpportunityAction{})
	hasFollow := gdb.Migrator().HasTable(&data.FollowedStock{})
	if !hasActions && !hasFollow {
		return nil, unavailable(SourceWatchFollow, "没有关注动作表，也没有自选表"), ""
	}
	var events []Event
	name := ""
	var notes []string
	if hasActions {
		var rows []opportunity.UserOpportunityAction
		err := gdb.Where("stock_code IN ?", keys).Order("created_at ASC").Limit(100).Find(&rows).Error
		if err != nil {
			notes = append(notes, "关注动作读取失败")
		} else if len(rows) == 0 {
			notes = append(notes, "没有记录到的关注/忽略/查看动作")
		}
		for _, row := range rows {
			if !keyMatch(row.StockCode, keys) {
				continue
			}
			events = append(events, actionEvent(row))
		}
	} else {
		notes = append(notes, "没有关注动作表")
	}
	if hasFollow {
		var rows []data.FollowedStock
		err := gdb.Where("stock_code IN ?", keys).Find(&rows).Error
		if err != nil {
			notes = append(notes, "自选读取失败")
		} else if len(rows) == 0 {
			notes = append(notes, "自选里没有这只股票")
		}
		for _, row := range rows {
			if !keyMatch(row.StockCode, keys) {
				continue
			}
			if name == "" {
				name = strings.TrimSpace(row.Name)
			}
			events = append(events, followEvent(row))
		}
	} else {
		notes = append(notes, "没有自选表")
	}
	if len(events) == 0 {
		msg := "没有可回顾的关注或自选记录"
		if len(notes) > 0 {
			msg = strings.Join(notes, "；")
		}
		if !hasActions && !hasFollow {
			return nil, unavailable(SourceWatchFollow, msg), name
		}
		return nil, emptyStatus(SourceWatchFollow, msg), name
	}
	st := okStatus(SourceWatchFollow, "来自关注动作与自选加入时间")
	st.Count = len(events)
	return events, st, name
}

func actionEvent(row opportunity.UserOpportunityAction) Event {
	label := "查看"
	switch strings.ToUpper(strings.TrimSpace(row.Action)) {
	case opportunity.ActionWatch:
		label = "关注"
	case opportunity.ActionIgnore:
		label = "忽略"
	case opportunity.ActionView:
		label = "查看"
	default:
		if strings.TrimSpace(row.Action) != "" {
			label = row.Action
		}
	}
	day := dateOf(row.CreatedAt)
	return Event{
		ID:         "watch:" + row.ID,
		Kind:       KindWatch,
		Lane:       LaneObserve,
		OccurredOn: day,
		OccurredAt: rfc(row.CreatedAt),
		Title:      "关注动作 · " + label,
		Detail: fmt.Sprintf("记录了%s（机会 %s）。回顾当时是否点过关注，不是买卖指令。",
			label, strings.TrimSpace(row.OpportunityID)),
		Source:    "user_opportunity_actions",
		KlineDate: day,
	}
}

func followEvent(row data.FollowedStock) Event {
	day := dateOf(row.Time)
	detail := "自选列表里有这只股票。"
	if day == "" {
		detail += "加入时间没有记下。"
	} else {
		detail += "加入时间来自自选记录。"
	}
	detail += "这只说明当时放进了自选，不是买入指令。"
	return Event{
		ID:         "follow:" + strings.ToLower(strings.TrimSpace(row.StockCode)),
		Kind:       KindFollow,
		Lane:       LaneObserve,
		OccurredOn: day,
		OccurredAt: rfc(row.Time),
		Title:      "自选 · 加入关注",
		Detail:     detail,
		Source:     "followed_stock",
		KlineDate:  day,
	}
}

func collectPaperSim(gdb *gorm.DB, keys []string) ([]Event, SourceStatus, string) {
	hasOrders := gdb.Migrator().HasTable(&papertrading.PaperSimOrder{})
	hasFills := gdb.Migrator().HasTable(&papertrading.PaperSimFill{})
	hasPos := gdb.Migrator().HasTable(&papertrading.PaperSimPosition{})
	if !hasOrders && !hasFills && !hasPos {
		return nil, unavailable(SourcePaperSim, "没有 paper_sim 表，无法回顾模拟盘触及"), ""
	}
	var events []Event
	name := ""
	filledOrder := map[uint]struct{}{}
	if hasFills {
		var fills []papertrading.PaperSimFill
		if err := gdb.Where("stock_code IN ?", keys).Order("filled_at ASC").Limit(80).Find(&fills).Error; err == nil {
			for _, f := range fills {
				if !keyMatch(f.StockCode, keys) {
					continue
				}
				if f.OrderID > 0 {
					filledOrder[f.OrderID] = struct{}{}
				}
				if name == "" {
					name = strings.TrimSpace(f.StockName)
				}
				events = append(events, fillEvent(f))
			}
		}
	}
	if hasOrders {
		var orders []papertrading.PaperSimOrder
		if err := gdb.Where("stock_code IN ?", keys).Order("order_time ASC").Limit(80).Find(&orders).Error; err == nil {
			for _, o := range orders {
				if !keyMatch(o.StockCode, keys) {
					continue
				}
				if _, ok := filledOrder[o.ID]; ok && strings.EqualFold(o.Status, papertrading.OrderStatusFilled) {
					continue
				}
				if name == "" {
					name = strings.TrimSpace(o.StockName)
				}
				events = append(events, orderEvent(o))
			}
		}
	}
	if hasPos {
		var positions []papertrading.PaperSimPosition
		if err := gdb.Where("stock_code IN ?", keys).Find(&positions).Error; err == nil {
			for _, p := range positions {
				if !keyMatch(p.StockCode, keys) || p.TotalVolume <= 0 {
					continue
				}
				if name == "" {
					name = strings.TrimSpace(p.StockName)
				}
				events = append(events, positionEvent(p))
			}
		}
	}
	if len(events) == 0 {
		return nil, emptyStatus(SourcePaperSim, "模拟盘里没有这只股票的委托、成交或持仓记录"), name
	}
	st := okStatus(SourcePaperSim, "来自 paper_sim 观察记录，不是真实委托")
	st.Count = len(events)
	return events, st, name
}

func fillEvent(f papertrading.PaperSimFill) Event {
	lane, side := laneForSide(f.Side)
	day := dateOf(f.FilledAt)
	return Event{
		ID:         fmt.Sprintf("paper-fill:%d", f.ID),
		Kind:       KindPaperSim,
		Lane:       lane,
		OccurredOn: day,
		OccurredAt: rfc(f.FilledAt),
		Title:      "模拟盘触及 · " + side + "成交",
		Detail: fmt.Sprintf("模拟成交价 %.4g，数量 %d。这是 paper_sim 观察记录，不是真实下单，也不会在这里生成交易计划。",
			f.Price, f.Volume),
		Source:    "paper_sim_fills",
		KlineDate: day,
	}
}

func orderEvent(o papertrading.PaperSimOrder) Event {
	lane, side := laneForSide(o.Side)
	day := strings.TrimSpace(o.TradeDate)
	if len(day) >= 10 {
		day = day[:10]
	}
	if day == "" {
		day = dateOf(o.OrderTime)
	}
	status := strings.TrimSpace(o.Status)
	if status == "" {
		status = "unknown"
	}
	detail := fmt.Sprintf("模拟委托状态 %s，参考价 %.4g，数量 %d。只是模拟盘留下的触及，不是新的委托。",
		status, o.OrderPrice, o.Quantity)
	if strings.TrimSpace(o.RejectReason) != "" {
		detail += " 拒绝原因：" + strings.TrimSpace(o.RejectReason) + "。"
	}
	return Event{
		ID:         fmt.Sprintf("paper-order:%d", o.ID),
		Kind:       KindPaperSim,
		Lane:       lane,
		OccurredOn: day,
		OccurredAt: rfc(o.OrderTime),
		Title:      "模拟盘触及 · " + side + "委托",
		Detail:     detail,
		Source:     "paper_sim_orders",
		KlineDate:  day,
	}
}

func positionEvent(p papertrading.PaperSimPosition) Event {
	day := dateOf(p.UpdatedAt)
	return Event{
		ID:         fmt.Sprintf("paper-pos:%d", p.ID),
		Kind:       KindPaperSim,
		Lane:       LaneHolding,
		OccurredOn: day,
		OccurredAt: rfc(p.UpdatedAt),
		Title:      "模拟盘触及 · 持仓",
		Detail: fmt.Sprintf("模拟持仓 %d 股，成本 %.4g。这是持有观察，不是加仓指令。",
			p.TotalVolume, p.AvgCost),
		Source:    "paper_sim_positions",
		KlineDate: day,
	}
}

func collectExternalMirror(gdb *gorm.DB, keys []string) ([]Event, SourceStatus) {
	const table = "external_mirror_events"
	if gdb == nil || !gdb.Migrator().HasTable(table) {
		return nil, unavailable(SourceExternalMirror, "未接入 external_mirror，没有外部镜像记录")
	}
	if !gdb.Migrator().HasColumn(table, "stock_code") {
		return nil, unavailable(SourceExternalMirror, "external_mirror 表无法按股票只读解析")
	}
	type row struct {
		ID         uint      `gorm:"column:id"`
		StockCode  string    `gorm:"column:stock_code"`
		Note       string    `gorm:"column:note"`
		OccurredAt time.Time `gorm:"column:occurred_at"`
	}
	var rows []row
	q := gdb.Table(table).Select("id", "stock_code", "note", "occurred_at").
		Where("stock_code IN ?", keys).
		Order("occurred_at ASC").
		Limit(40)
	if err := q.Find(&rows).Error; err != nil {
		return nil, unavailable(SourceExternalMirror, "读取 external_mirror 失败")
	}
	var events []Event
	for _, row := range rows {
		if !keyMatch(row.StockCode, keys) {
			continue
		}
		day := dateOf(row.OccurredAt)
		note := strings.TrimSpace(row.Note)
		if note == "" {
			note = "有一条外部镜像触及，但没有备注正文。"
		}
		events = append(events, Event{
			ID:         fmt.Sprintf("mirror:%d", row.ID),
			Kind:       KindExternalMirror,
			Lane:       LaneObserve,
			OccurredOn: day,
			OccurredAt: rfc(row.OccurredAt),
			Title:      "外部镜像触及",
			Detail:     note + " 只作回顾，不是买卖指令。",
			Source:     table,
			KlineDate:  day,
		})
	}
	if len(events) == 0 {
		return nil, emptyStatus(SourceExternalMirror, "external_mirror 里没有这只股票的记录")
	}
	st := okStatus(SourceExternalMirror, "来自 external_mirror")
	st.Count = len(events)
	return events, st
}

func collectOutcomeNotes(gdb *gorm.DB, raw string, keys []string) ([]Event, SourceStatus, string) {
	var events []Event
	name := ""
	var problems []string
	readable := false

	if gdb.Migrator().HasTable(&papertrading.ExitReviewOutcome{}) {
		readable = true
		var rows []papertrading.ExitReviewOutcome
		if err := gdb.Where("stock_code IN ?", keys).Order("review_time ASC").Limit(40).Find(&rows).Error; err != nil {
			problems = append(problems, "离场复核备注读取失败")
		} else {
			for _, row := range rows {
				if !keyMatch(row.StockCode, keys) {
					continue
				}
				events = append(events, exitReviewEvent(row))
			}
		}
	}
	if gdb.Migrator().HasTable(&data.TradingRecord{}) {
		readable = true
		var rows []data.TradingRecord
		if err := gdb.Where("stock_code IN ?", keys).Order("trading_time ASC").Limit(40).Find(&rows).Error; err != nil {
			problems = append(problems, "交易日志备注读取失败")
		} else {
			for _, row := range rows {
				if !keyMatch(row.StockCode, keys) {
					continue
				}
				if ev, ok := tradingNoteEvent(row); ok {
					events = append(events, ev)
				}
			}
		}
	}
	if gdb.Migrator().HasTable(&papertrading.PaperSimFill{}) {
		readable = true
		proj, err := projectStockSafe(raw)
		if err != nil && !errors.Is(err, outcome.ErrNotFound) && !errors.Is(err, outcome.ErrInvalidStockCode) {
			problems = append(problems, "结果投影不可用")
		}
		for _, item := range proj {
			evs, n := projectionEvents(item)
			events = append(events, evs...)
			name = firstName(name, n)
		}
	}
	if !readable && len(events) == 0 {
		msg := "没有结果备注来源"
		if len(problems) > 0 {
			msg = strings.Join(problems, "；")
		}
		return nil, unavailable(SourceOutcome, msg), name
	}
	if len(events) == 0 {
		msg := "没有这只股票的结果备注"
		if len(problems) > 0 {
			msg = strings.Join(problems, "；")
		}
		return nil, emptyStatus(SourceOutcome, msg), name
	}
	msg := "来自结果投影、离场复核或交易日志里写下的备注"
	if len(problems) > 0 {
		msg += "（" + strings.Join(problems, "；") + "）"
	}
	st := okStatus(SourceOutcome, msg)
	st.Count = len(events)
	return events, st, name
}

func exitReviewEvent(row papertrading.ExitReviewOutcome) Event {
	lane := LaneExit
	label := "离场复核"
	switch strings.ToUpper(strings.TrimSpace(row.Decision)) {
	case papertrading.ExitReviewDecisionHold:
		lane = LaneHolding
		label = "复核后继续持有"
	case papertrading.ExitReviewDecisionWatch:
		lane = LaneObserve
		label = "复核后继续观察"
	case papertrading.ExitReviewDecisionCreateSellPlan:
		lane = LaneExit
		label = "曾记下卖出意向"
	}
	day := dateOf(row.ReviewTime)
	detail := "离场复核备注：" + label + "。"
	if strings.TrimSpace(row.Reason) != "" {
		detail += strings.TrimSpace(row.Reason) + "。"
	}
	if strings.TrimSpace(row.EvaluationSummarySnapshot) != "" {
		detail += "摘要：" + strings.TrimSpace(row.EvaluationSummarySnapshot) + "。"
	}
	detail += "这是当时留下的备注，本时间轴不会据此生成卖出计划。"
	return Event{
		ID:         "outcome-review:" + row.ID,
		Kind:       KindOutcome,
		Lane:       lane,
		OccurredOn: day,
		OccurredAt: rfc(row.ReviewTime),
		Title:      "结果备注 · " + label,
		Detail:     detail,
		Source:     "exit_review_outcomes",
		KlineDate:  day,
	}
}

func tradingNoteEvent(row data.TradingRecord) (Event, bool) {
	reason := strings.TrimSpace(row.Reason)
	mind := strings.TrimSpace(row.Mindset)
	if reason == "" && mind == "" {
		return Event{}, false
	}
	lane := LaneObserve
	dir := strings.TrimSpace(row.Direction)
	switch dir {
	case "买入":
		lane = LaneEntry
	case "卖出":
		lane = LaneExit
	}
	day := dateOf(row.TradingTime)
	detail := "交易日志里的备注。"
	if dir != "" {
		detail += "当时方向记为「" + dir + "」。"
	}
	if reason != "" {
		detail += reason + "。"
	}
	if mind != "" {
		detail += mind + "。"
	}
	detail += "只回看写下的话，不是新的买卖指令。"
	return Event{
		ID:         fmt.Sprintf("outcome-log:%d", row.ID),
		Kind:       KindOutcome,
		Lane:       lane,
		OccurredOn: day,
		OccurredAt: rfc(row.TradingTime),
		Title:      "结果备注 · 日志",
		Detail:     detail,
		Source:     "trading_records",
		KlineDate:  day,
	}, true
}

// projectStockSafe reads outcome projection and fails closed if that reader panics
// on incomplete rows. Other timeline sources still return.
func projectStockSafe(raw string) (items []outcome.OutcomeProjection, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			items = nil
			err = fmt.Errorf("outcome projection unavailable")
		}
	}()
	return outcome.NewService().ProjectStock(raw, outcome.ProjectOptions{})
}

func projectionEvents(item outcome.OutcomeProjection) ([]Event, string) {
	var events []Event
	name := strings.TrimSpace(item.StockName)
	if item.Entry.Present {
		day := strings.TrimSpace(item.Entry.EntryDate)
		if len(day) >= 10 {
			day = day[:10]
		}
		events = append(events, Event{
			ID:         item.OutcomeID + ":entry",
			Kind:       KindOutcome,
			Lane:       LaneEntry,
			OccurredOn: day,
			OccurredAt: day,
			Title:      "结果备注 · 入场",
			Detail: fmt.Sprintf("结果投影记下入场价 %.4g、数量 %d。这是事后对照，不是买入指令。",
				item.Entry.EntryPrice, item.Entry.EntryQty),
			Source:    "opportunity_outcome",
			KlineDate: day,
		})
	}
	if item.OutcomeStatus == outcome.OutcomeStatusOpen && item.Entry.Present {
		day := strings.TrimSpace(item.Entry.EntryDate)
		if len(day) >= 10 {
			day = day[:10]
		}
		hold := item.Performance.HoldingDays
		events = append(events, Event{
			ID:         item.OutcomeID + ":holding",
			Kind:       KindOutcome,
			Lane:       LaneHolding,
			OccurredOn: day,
			OccurredAt: day,
			Title:      "结果备注 · 持有",
			Detail:     fmt.Sprintf("结果仍未了结，投影持有 %d 天。持有记录与入场、离场分开，不是加仓指令。", hold),
			Source:     "opportunity_outcome",
			KlineDate:  day,
		})
	}
	if item.Exit.Present {
		day := strings.TrimSpace(item.Exit.ExitDate)
		if len(day) >= 10 {
			day = day[:10]
		}
		detail := fmt.Sprintf("结果投影记下离场价 %.4g、数量 %d。", item.Exit.ExitPrice, item.Exit.ExitQty)
		if item.Performance.RealizedReturnPct != nil {
			detail += fmt.Sprintf(" 已实现收益 %.2f%%。", *item.Performance.RealizedReturnPct)
		}
		if strings.TrimSpace(item.Exit.ExitReasonText) != "" {
			detail += " 备注：" + strings.TrimSpace(item.Exit.ExitReasonText) + "。"
		}
		detail += "这是离场记录，不是卖出指令。"
		events = append(events, Event{
			ID:         item.OutcomeID + ":exit",
			Kind:       KindOutcome,
			Lane:       LaneExit,
			OccurredOn: day,
			OccurredAt: day,
			Title:      "结果备注 · 离场",
			Detail:     detail,
			Source:     "opportunity_outcome",
			KlineDate:  day,
		})
	}
	return events, name
}

func laneForSide(side string) (lane, label string) {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "buy", "买入":
		return LaneEntry, "买入"
	case "sell", "卖出":
		return LaneExit, "卖出"
	default:
		return LaneObserve, "记录"
	}
}

func sessionLabel(session string) string {
	switch strings.TrimSpace(session) {
	case models.SignalScanSessionMid:
		return "午盘"
	case models.SignalScanSessionClose, "":
		return "收盘"
	default:
		return session
	}
}

func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		di, dj := events[i].OccurredOn, events[j].OccurredOn
		if di == "" && dj != "" {
			return false
		}
		if dj == "" && di != "" {
			return true
		}
		if di != dj {
			return di > dj
		}
		if events[i].OccurredAt != events[j].OccurredAt {
			return events[i].OccurredAt < events[j].OccurredAt
		}
		return kindRank(events[i].Kind) < kindRank(events[j].Kind)
	})
}

func kindRank(kind string) int {
	switch kind {
	case KindSignal:
		return 1
	case KindWatch:
		return 2
	case KindFollow:
		return 3
	case KindExternalMirror:
		return 4
	case KindPaperSim:
		return 5
	case KindOutcome:
		return 6
	default:
		return 9
	}
}

func lookupKeys(raw string, norm data.NormalizedStockCode) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		for _, v := range []string{s, strings.ToLower(s)} {
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	add(raw)
	add(opportunity.NormalizeReadStockCode(raw))
	add(norm.SinaCode)
	add(norm.TSCode)
	add(norm.SecuCode)
	add(norm.Symbol)
	return out
}

func signalLikePatterns(norm data.NormalizedStockCode) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if len(s) < 4 {
			return
		}
		for _, v := range out {
			if strings.EqualFold(v, s) {
				return
			}
		}
		out = append(out, s)
	}
	add(norm.TSCode)
	add(norm.SecuCode)
	add(norm.SinaCode)
	return out
}

func hitMatches(hit models.SignalScanHit, norm data.NormalizedStockCode) bool {
	for _, raw := range []string{hit.SECUCODE, hit.SECURITY_CODE} {
		n, err := data.NormalizeStockCode(raw)
		if err != nil {
			continue
		}
		if norm.Symbol != "" && n.Symbol == norm.Symbol && norm.Market == n.Market {
			return true
		}
	}
	return false
}

func keyMatch(code string, keys []string) bool {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" {
		return false
	}
	for _, k := range keys {
		if strings.ToLower(strings.TrimSpace(k)) == c {
			return true
		}
	}
	n, err := data.NormalizeStockCode(code)
	if err != nil {
		return false
	}
	for _, k := range keys {
		kn, kerr := data.NormalizeStockCode(k)
		if kerr == nil && kn.Symbol == n.Symbol && kn.Market == n.Market && n.Symbol != "" {
			return true
		}
	}
	return false
}

func dateOf(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(shanghai).Format("2006-01-02")
}

func rfc(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(shanghai).Format(time.RFC3339)
}

func firstName(current, next string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return strings.TrimSpace(next)
}

func okStatus(source, message string) SourceStatus {
	return SourceStatus{Source: source, Status: StatusOK, Message: message}
}

func emptyStatus(source, message string) SourceStatus {
	return SourceStatus{Source: source, Status: StatusEmpty, Message: message}
}

func unavailable(source, message string) SourceStatus {
	return SourceStatus{Source: source, Status: StatusUnavailable, Message: message}
}
