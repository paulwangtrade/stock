package enhancer

import (
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

const (
	StrategyWeight = 0.6
	SignalWeight   = 0.4
)

// SignalSnapshotSource 只读快照源（由 data.SignalSnapshotRepo 实现）。
type SignalSnapshotSource interface {
	GetLatestCloseSignalSnapshot(asOfDate string) (*models.SignalScanSnapshot, error)
	ParseSnapshotHits(snap *models.SignalScanSnapshot) []models.SignalScanHit
}

// SignalSnapshotEnhancer 用 asOf 之前最近 close snapshot 增强排序分。
type SignalSnapshotEnhancer struct {
	Source SignalSnapshotSource
}

func NewSignalSnapshotEnhancer() *SignalSnapshotEnhancer {
	return &SignalSnapshotEnhancer{Source: data.NewSignalSnapshotRepo()}
}

func (e *SignalSnapshotEnhancer) Enhance(items []CandidateItem, ctx EnhanceContext) ([]CandidateItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	if e.Source == nil {
		e.Source = data.NewSignalSnapshotRepo()
	}

	snap, err := e.Source.GetLatestCloseSignalSnapshot(ctx.TradeDate)
	if err != nil || snap == nil {
		logger.SugaredLogger.Infof("signal enhancer skipped: no close snapshot")
		for i := range items {
			items[i].SignalTag = ""
			items[i].SignalSnapshotID = 0
			items[i].ApplyScore(ComposeCandidateScore(items[i].StrategyScore, 0))
		}
		return items, nil
	}

	hits := e.Source.ParseSnapshotHits(snap)
	byCode := buildHitIndex(hits)
	enhanced := 0
	for i := range items {
		code := strings.ToLower(strings.TrimSpace(items[i].StockCode))
		hit, ok := byCode[code]
		if !ok {
			items[i].SignalTag = ""
			items[i].SignalSnapshotID = 0
			items[i].ApplyScore(ComposeCandidateScore(items[i].StrategyScore, 0))
			continue
		}
		tag := strings.TrimSpace(hit.Tag)
		sig := signalScoreFromTag(tag)
		items[i].SignalTag = tag
		items[i].SignalSnapshotID = snap.ID
		items[i].ApplyScore(ComposeCandidateScore(items[i].StrategyScore, sig))
		if tag != "" || sig > 0 {
			enhanced++
		}
	}
	logger.SugaredLogger.Infof("signal enhancer applied snapshot_id=%d hits=%d matched=%d",
		snap.ID, len(hits), enhanced)
	return items, nil
}

func buildHitIndex(hits []models.SignalScanHit) map[string]models.SignalScanHit {
	out := map[string]models.SignalScanHit{}
	for _, h := range hits {
		for _, raw := range []string{h.SECUCODE, h.SECURITY_CODE} {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			n, err := data.NormalizeStockCode(raw)
			if err != nil || n.Market != data.MarketCN {
				continue
			}
			code := strings.ToLower(strings.TrimSpace(n.SinaCode))
			if data.IsAShareSinaCode(code) {
				out[code] = h
			}
		}
	}
	return out
}

func signalScoreFromTag(tag string) float64 {
	switch strings.TrimSpace(tag) {
	case "强":
		return 1.0
	case "买":
		return 0.85
	case "突":
		return 0.75
	case "趋":
		return 0.65
	case "转":
		return 0.55
	case "弹":
		return 0.45
	default:
		return 0
	}
}

// SignalScoreFromTag 导出供单测。
func SignalScoreFromTag(tag string) float64 { return signalScoreFromTag(tag) }
