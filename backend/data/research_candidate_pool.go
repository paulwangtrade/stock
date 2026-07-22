package data

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

const DefaultResearchCandidateMinScore = 60.0

// CalcResearchSignalScore 与前端 calcSignalScore 对齐（0–100）。
func CalcResearchSignalScore(tag string, daysAgo *int, rsi float64) float64 {
	baseByTag := map[string]float64{
		"强": 92, "趋": 84, "加": 86, "转": 80, "突": 78, "弹": 72, "买": 68,
		"冰": 52, "减": 16, "止": 28, "冲": 22, "卖": 22,
	}
	base, ok := baseByTag[strings.TrimSpace(tag)]
	if !ok {
		return 0
	}
	score := base
	if daysAgo != nil && *daysAgo > 0 {
		score -= math.Min(12, float64(*daysAgo)*3)
	}
	sellLike := tag == "减" || tag == "止" || tag == "冲" || tag == "卖"
	if !sellLike && rsi > 0 {
		if rsi < 25 {
			score += 8
		} else if rsi < 30 {
			score += 5
		} else if rsi < 35 {
			score += 2
		}
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return math.Round(score)
}

// PredictDirectionFromTag 由信号标签推断预测方向。
func PredictDirectionFromTag(tag string) string {
	switch strings.TrimSpace(tag) {
	case "强", "趋", "加", "转", "突", "弹", "买", "冰":
		return "看多"
	case "减", "止", "冲", "卖":
		return "看空"
	default:
		return "中性"
	}
}

func normalizeFollowableCode(secucode, securityCode string) string {
	code := strings.TrimSpace(strings.ToLower(secucode))
	if code == "" {
		code = strings.TrimSpace(strings.ToLower(securityCode))
	}
	// 600000.SH → sh600000
	if i := strings.IndexByte(code, '.'); i > 0 {
		sym := code[:i]
		mkt := code[i+1:]
		switch mkt {
		case "sh", "sz", "bj":
			return mkt + sym
		}
	}
	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
		return code
	}
	if len(code) == 6 {
		switch code[0] {
		case '6':
			return "sh" + code
		case '0', '3':
			return "sz" + code
		case '4', '8':
			return "bj" + code
		}
	}
	return code
}

// ListResearchCandidatesFromLatestSnapshot 从最新 status=done 的信号快照提取候选。
// 不落库、不改 BuildCandidatePool / 扫描逻辑。
// minScore<=0 时使用配置 candidate_pool_score_threshold（默认 60）。
func ListResearchCandidatesFromLatestSnapshot(minScore float64) *models.ResearchSnapshotCandidateList {
	out := &models.ResearchSnapshotCandidateList{
		MinScore: minScore,
		Items:    []models.ResearchSnapshotCandidate{},
	}
	if minScore <= 0 {
		minScore = GetCandidatePoolScoreThreshold()
		out.MinScore = minScore
	}
	if db.Dao == nil {
		out.Message = "数据库未初始化"
		return out
	}
	if !db.Dao.Migrator().HasTable(&models.SignalScanSnapshot{}) {
		out.Message = "尚无信号快照表，请先生成盘后快照"
		return out
	}

	var snap models.SignalScanSnapshot
	err := db.Dao.Where("status = ?", "done").
		Order("created_at DESC").
		First(&snap).Error
	if err != nil || snap.ID == 0 {
		out.Message = "暂无可用信号快照（status=done）"
		return out
	}

	out.SnapshotID = snap.ID
	out.TradeDate = snap.TradeDate
	out.Session = snap.Session
	out.StrategyName = snap.StrategyName
	out.HitTotal = snap.HitTotal
	if !snap.CreatedAt.IsZero() {
		out.SnapshotTime = snap.CreatedAt.UTC().Format(time.RFC3339)
	}

	items := buildCandidatesFromResultJSON(snap.ResultJSON, minScore)
	out.Items = items
	out.ItemCount = len(items)
	if out.ItemCount == 0 {
		out.Message = "最新快照中无信号分大于阈值的股票"
	}
	return out
}

func buildCandidatesFromResultJSON(resultJSON string, minScore float64) []models.ResearchSnapshotCandidate {
	hits := NewSignalSnapshotRepo().ParseSnapshotHits(&models.SignalScanSnapshot{ResultJSON: resultJSON})
	items := make([]models.ResearchSnapshotCandidate, 0)
	seen := map[string]bool{}

	if len(hits) > 0 {
		for _, h := range hits {
			cand, ok := candidateFromHit(h, minScore)
			if !ok || seen[cand.StockCode] {
				continue
			}
			seen[cand.StockCode] = true
			items = append(items, cand)
		}
	} else {
		for _, raw := range parseFlexibleSnapshotRows(resultJSON) {
			cand, ok := candidateFromFlexible(raw, minScore)
			if !ok || seen[cand.StockCode] {
				continue
			}
			seen[cand.StockCode] = true
			items = append(items, cand)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].SignalScore != items[j].SignalScore {
			return items[i].SignalScore > items[j].SignalScore
		}
		return items[i].StockCode < items[j].StockCode
	})
	return items
}

func candidateFromHit(h models.SignalScanHit, minScore float64) (models.ResearchSnapshotCandidate, bool) {
	tag := strings.TrimSpace(h.Tag)
	score := CalcResearchSignalScore(tag, h.DaysAgo, h.RSI)
	if score <= minScore {
		return models.ResearchSnapshotCandidate{}, false
	}
	code := normalizeFollowableCode(h.SECUCODE, h.SECURITY_CODE)
	if code == "" {
		return models.ResearchSnapshotCandidate{}, false
	}
	return models.ResearchSnapshotCandidate{
		StockCode:   code,
		StockName:   strings.TrimSpace(h.SECURITY_NAME_ABBR),
		SignalScore: score,
		SignalTag:   tag,
		Direction:   PredictDirectionFromTag(tag),
		StatusText:  h.StatusText,
		Price:       h.NEW_PRICE,
		Industry:    h.INDUSTRY,
	}, true
}

type flexibleSnapshotRow struct {
	Code      string
	Name      string
	Score     float64
	HasScore  bool
	Direction string
	Reason    string
	Tag       string
	Price     string
	RSI       float64
	DaysAgo   *int
}

func candidateFromFlexible(raw flexibleSnapshotRow, minScore float64) (models.ResearchSnapshotCandidate, bool) {
	code := normalizeFollowableCode(raw.Code, raw.Code)
	if code == "" {
		return models.ResearchSnapshotCandidate{}, false
	}
	tag := strings.TrimSpace(raw.Tag)
	score := raw.Score
	if !raw.HasScore {
		if tag == "" {
			return models.ResearchSnapshotCandidate{}, false
		}
		score = CalcResearchSignalScore(tag, raw.DaysAgo, raw.RSI)
	}
	if score <= minScore {
		return models.ResearchSnapshotCandidate{}, false
	}
	dir := strings.TrimSpace(raw.Direction)
	if dir == "" {
		dir = PredictDirectionFromTag(tag)
	}
	return models.ResearchSnapshotCandidate{
		StockCode:   code,
		StockName:   strings.TrimSpace(raw.Name),
		SignalScore: score,
		SignalTag:   tag,
		Direction:   dir,
		StatusText:  strings.TrimSpace(raw.Reason),
		Price:       raw.Price,
	}, true
}

// parseFlexibleSnapshotRows 解析 results[] 或 code→对象 映射（已由 ParseSnapshotHits 覆盖的 items 会在上层去重）。
func parseFlexibleSnapshotRows(raw string) []flexibleSnapshotRow {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []flexibleSnapshotRow

	var wrapResults struct {
		Results []map[string]any `json:"results"`
	}
	if json.Unmarshal([]byte(raw), &wrapResults) == nil && len(wrapResults.Results) > 0 {
		for _, m := range wrapResults.Results {
			if row, ok := mapToFlexibleRow("", m); ok {
				out = append(out, row)
			}
		}
		return out
	}

	var asMap map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &asMap) == nil && len(asMap) > 0 {
		// 排除已知 payload 包装键，避免把 items/meta 当股票代码
		skip := map[string]bool{
			"items": true, "results": true, "scannedTotal": true, "hitTotal": true,
			"tradeDate": true, "session": true, "strategyId": true, "strategyName": true,
			"completedAt": true,
		}
		stockLike := 0
		for k := range asMap {
			if skip[k] {
				continue
			}
			if looksLikeStockCodeKey(k) {
				stockLike++
			}
		}
		if stockLike > 0 {
			for k, v := range asMap {
				if skip[k] || !looksLikeStockCodeKey(k) {
					continue
				}
				var m map[string]any
				if json.Unmarshal(v, &m) != nil {
					continue
				}
				if row, ok := mapToFlexibleRow(k, m); ok {
					out = append(out, row)
				}
			}
		}
	}
	return out
}

func looksLikeStockCodeKey(k string) bool {
	k = strings.TrimSpace(strings.ToLower(k))
	if len(k) == 6 && isDigits(k) {
		return true
	}
	if (strings.HasPrefix(k, "sh") || strings.HasPrefix(k, "sz") || strings.HasPrefix(k, "bj")) && len(k) >= 8 {
		return true
	}
	if i := strings.IndexByte(k, '.'); i > 0 {
		sym := k[:i]
		return len(sym) == 6 && isDigits(sym)
	}
	return false
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func mapToFlexibleRow(codeHint string, m map[string]any) (flexibleSnapshotRow, bool) {
	if m == nil {
		return flexibleSnapshotRow{}, false
	}
	row := flexibleSnapshotRow{
		Code: firstString(m, "code", "stock_code", "stockCode", "SECURITY_CODE", "SECUCODE"),
		Name: firstString(m, "name", "stock_name", "stockName", "SECURITY_NAME_ABBR"),
		Direction: firstString(m, "direction", "predict_direction", "predictDirection"),
		Reason:    firstString(m, "reason", "statusText", "status_text"),
		Tag:       firstString(m, "tag", "signal_tag", "signalTag"),
		Price:     firstString(m, "price", "NEW_PRICE", "new_price"),
	}
	if row.Code == "" {
		row.Code = codeHint
	}
	if row.Code == "" {
		return flexibleSnapshotRow{}, false
	}
	if score, ok := firstFloat(m, "score", "signal_score", "signalScore"); ok {
		row.Score = score
		row.HasScore = true
	}
	if rsi, ok := firstFloat(m, "rsi", "RSI"); ok {
		row.RSI = rsi
	}
	if days, ok := firstInt(m, "recentSignalDaysAgo", "daysAgo", "days_ago"); ok {
		row.DaysAgo = &days
	}
	return row, true
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			case float64:
				return strings.TrimSpace(strconv.FormatFloat(t, 'f', -1, 64))
			case json.Number:
				return t.String()
			default:
				s := strings.TrimSpace(fmt.Sprint(t))
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}

func firstFloat(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case float64:
				return t, true
			case float32:
				return float64(t), true
			case int:
				return float64(t), true
			case int64:
				return float64(t), true
			case json.Number:
				f, err := t.Float64()
				return f, err == nil
			case string:
				f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
				return f, err == nil
			}
		}
	}
	return 0, false
}

func firstInt(m map[string]any, keys ...string) (int, bool) {
	f, ok := firstFloat(m, keys...)
	if !ok {
		return 0, false
	}
	return int(math.Round(f)), true
}
