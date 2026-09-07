package research

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// DeriveExplain builds a deterministic signal_derived Explain from a Candidate.
// Does not call AI, Broker, Intent, or Trade paths.
func DeriveExplain(c Candidate, now time.Time) Explain {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	id := MakeExplainID(c.ID)
	ex := Explain{
		ID:                id,
		SchemaVersion:     ExplainSchemaVersion,
		CandidateID:       c.ID,
		TradeDate:         c.TradeDate,
		StockCode:         c.StockCode,
		ExplainType:       ExplainTypeSignalDerived,
		Available:         true,
		StrategyIntentRef: nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if strings.TrimSpace(c.ID) == "" || !ParseCandidateIDOK(c.ID) {
		ex.Available = false
		ex.ExplainType = ExplainTypeUnavailable
		ex.MissingReason = "invalid_candidate"
		ex.Summary = ""
		ex.ResearchReason = ResearchReason{Kind: ReasonKindUnknown, Text: ""}
		return ex
	}

	steps := []string{}
	if tag := strings.TrimSpace(c.SignalTag); tag != "" {
		steps = append(steps, fmt.Sprintf("信号标签=%s", tag))
	}
	if c.SignalScore != nil {
		steps = append(steps, fmt.Sprintf("信号分=%.0f", *c.SignalScore))
	}
	if dir := strings.TrimSpace(c.Direction); dir != "" {
		steps = append(steps, fmt.Sprintf("方向=%s", dir))
	}
	if thr := strings.TrimSpace(c.Reason); thr == "" {
		// no-op
	} else {
		steps = append(steps, "含快照状态文案")
	}

	var scoreCopy *float64
	if c.SignalScore != nil {
		v := *c.SignalScore
		scoreCopy = &v
	}
	var snapCopy *uint
	if c.SignalSnapshotID != nil {
		v := *c.SignalSnapshotID
		snapCopy = &v
	}

	asOf := now.Format(time.RFC3339)
	ev := ExplainEvidence{
		AsOf:             asOf,
		SignalSnapshotID: snapCopy,
		Source:           c.Source,
		SourceRef:        c.SourceRef,
		SignalTag:        c.SignalTag,
		SignalScore:      scoreCopy,
		Direction:        c.Direction,
		Price:            c.Price,
		StatusText:       c.Reason,
		ScoreSteps:       steps,
	}
	ev.EvidenceHash = hashEvidence(ev)
	ex.Evidence = ev

	reasonText := strings.TrimSpace(c.Reason)
	kind := ReasonKindSignalText
	if reasonText == "" {
		kind = ReasonKindRule
		reasonText = fmt.Sprintf("研究宇宙投影：来源=%s，标签=%s",
			nonEmpty(c.Source, SourceSignalSnapshot),
			nonEmpty(c.SignalTag, "—"))
	}
	ex.ResearchReason = ResearchReason{Kind: kind, Text: reasonText}
	ex.Summary = buildExplainSummary(c)
	ex.RiskNote = deriveRiskNote(c)
	return ex
}

func ParseCandidateIDOK(id string) bool {
	_, _, ok := ParseCandidateID(id)
	return ok
}

func buildExplainSummary(c Candidate) string {
	parts := []string{}
	if c.StockName != "" || c.StockCode != "" {
		parts = append(parts, strings.TrimSpace(c.StockName+" "+c.StockCode))
	}
	if c.SignalTag != "" {
		parts = append(parts, "信号「"+c.SignalTag+"」")
	}
	if c.SignalScore != nil {
		parts = append(parts, fmt.Sprintf("分%.0f", *c.SignalScore))
	}
	if c.Direction != "" {
		parts = append(parts, c.Direction)
	}
	if len(parts) == 0 {
		return "研究候选信号派生解释"
	}
	s := strings.Join(parts, " · ")
	return TruncateExplainSummary(s, 240)
}

func deriveRiskNote(c Candidate) *RiskNote {
	tag := strings.TrimSpace(c.SignalTag)
	sellLike := tag == "减" || tag == "止" || tag == "冲" || tag == "卖"
	if sellLike {
		return &RiskNote{
			Severity: RiskSeverityWarn,
			Text:     "信号偏卖出/减仓类，研究解释不构成交易指令",
		}
	}
	if c.SignalScore != nil && *c.SignalScore >= 90 {
		return &RiskNote{
			Severity: RiskSeverityInfo,
			Text:     "信号分很高，仍需人工复核流动性与交易规则（非执行门控）",
		}
	}
	return &RiskNote{
		Severity: RiskSeverityInfo,
		Text:     "本解释为研究旁路，不含下单/晋级意图",
	}
}

func hashEvidence(ev ExplainEvidence) string {
	h := sha1.New()
	snap := ""
	if ev.SignalSnapshotID != nil {
		snap = fmt.Sprintf("%d", *ev.SignalSnapshotID)
	}
	score := ""
	if ev.SignalScore != nil {
		score = fmt.Sprintf("%.4f", *ev.SignalScore)
	}
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s",
		ev.SourceRef,
		snap,
		ev.SignalTag,
		score,
		ev.Direction,
		ev.Price,
		ev.StatusText,
		strings.Join(ev.ScoreSteps, ";"),
	)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
