package research

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"
)

// AssembleFromSnapshot projects ResearchSnapshotCandidateList → Research Candidate DTOs.
// Does not touch Trade Candidate (models.CandidatePool) or execution paths.
func AssembleFromSnapshot(src *models.ResearchSnapshotCandidateList) ListResult {
	out := ListResult{
		Items: []Candidate{},
	}
	if src == nil {
		out.Message = "无快照数据"
		return out
	}
	out.TradeDate = strings.TrimSpace(src.TradeDate)
	out.AsOf = strings.TrimSpace(src.SnapshotTime)
	if out.AsOf == "" {
		out.AsOf = time.Now().Format(time.RFC3339)
	}
	out.Message = src.Message
	out.Threshold = src.MinScore
	out.SnapshotID = src.SnapshotID

	items := make([]Candidate, 0, len(src.Items))
	for i, it := range src.Items {
		items = append(items, assembleOne(src, it, i+1))
	}
	out.Items = items
	return out
}

func assembleOne(src *models.ResearchSnapshotCandidateList, it models.ResearchSnapshotCandidate, rank int) Candidate {
	td := strings.TrimSpace(src.TradeDate)
	code := strings.TrimSpace(it.StockCode)
	id := MakeCandidateID(td, code)

	score := it.SignalScore
	var snapID *uint
	if src.SnapshotID > 0 {
		sid := src.SnapshotID
		snapID = &sid
	}
	r := rank
	tags := []string{}
	if ind := strings.TrimSpace(it.Industry); ind != "" {
		tags = append(tags, ind)
	}

	return Candidate{
		ID:               id,
		TradeDate:        td,
		StockCode:        code,
		StockName:        strings.TrimSpace(it.StockName),
		Status:           StatusNew,
		Source:           SourceSignalSnapshot,
		SourceRef:        fmt.Sprintf("snapshot_id=%d", src.SnapshotID),
		SignalTag:        strings.TrimSpace(it.SignalTag),
		SignalScore:      &score,
		SignalSnapshotID: snapID,
		Score:            nil,
		Rank:             &r,
		Tags:             tags,
		Note:             "",
		PromotedPoolID:   nil,
		PromotedAt:       nil,
		Direction:        strings.TrimSpace(it.Direction),
		Price:            strings.TrimSpace(it.Price),
		Reason:           strings.TrimSpace(it.StatusText),
	}
}
