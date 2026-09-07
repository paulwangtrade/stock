package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/data"
)

// CandidatePoolListItem GET /api/candidate_pool/list 单条。
type CandidatePoolListItem struct {
	StockCode   string  `json:"stock_code"`
	StockName   string  `json:"stock_name"`
	SignalScore float64 `json:"signal_score"`
	Direction   string  `json:"direction"` // 买入 / 卖出 / 中性
	Reason      string  `json:"reason,omitempty"`
	SignalTag   string  `json:"signal_tag,omitempty"`
	Price       string  `json:"price,omitempty"`
}

// CandidatePoolListResponse 统一包装响应。
type CandidatePoolListResponse struct {
	Code         int                     `json:"code"`
	Data         []CandidatePoolListItem `json:"data"`
	Total        int                     `json:"total"`
	SnapshotTime string                  `json:"snapshot_time"`
	SnapshotID   uint                    `json:"snapshot_id"`
	Threshold    float64                 `json:"threshold"`
	Message      string                  `json:"message,omitempty"`
}

// CandidatePoolHandler 候选池只读 HTTP API（不改自选/快照扫描逻辑）。
// Deprecated (Phase13-A5): 本路径名实为研究快照投影；新契约请用
// GET /api/research/candidates（Research Candidate Pool）。落库 Trade Candidate 仍为 models.CandidatePool。
type CandidatePoolHandler struct{}

func NewCandidatePoolHandler() *CandidatePoolHandler {
	return &CandidatePoolHandler{}
}

// ServeHTTP GET /api/candidate_pool/list?min_score=60
func (h *CandidatePoolHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewCandidatePoolHandler()
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path != "/api/candidate_pool/list" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	h.handleList(w, r)
}

func (h *CandidatePoolHandler) handleList(w http.ResponseWriter, r *http.Request) {
	minScore := data.GetCandidatePoolScoreThreshold()
	if raw := strings.TrimSpace(r.URL.Query().Get("min_score")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			minScore = v
		}
	}

	src := data.ListResearchCandidatesFromLatestSnapshot(minScore)
	items := make([]CandidatePoolListItem, 0, len(src.Items))
	for _, it := range src.Items {
		items = append(items, CandidatePoolListItem{
			StockCode:   it.StockCode,
			StockName:   it.StockName,
			SignalScore: it.SignalScore,
			Direction:   mapDirectionForAPI(it.Direction),
			Reason:      strings.TrimSpace(it.StatusText),
			SignalTag:   it.SignalTag,
			Price:       it.Price,
		})
	}
	writeJSON(w, http.StatusOK, CandidatePoolListResponse{
		Code:         0,
		Data:         items,
		Total:        len(items),
		SnapshotTime: src.SnapshotTime,
		SnapshotID:   src.SnapshotID,
		Threshold:    src.MinScore,
		Message:      src.Message,
	})
}

func mapDirectionForAPI(dir string) string {
	switch strings.TrimSpace(dir) {
	case "看多":
		return "买入"
	case "看空":
		return "卖出"
	case "买入", "卖出", "中性":
		return dir
	default:
		if dir == "" {
			return "中性"
		}
		return dir
	}
}

// RegisterCandidatePoolRoutes 挂载候选池路由。
func RegisterCandidatePoolRoutes(mux *http.ServeMux) {
	mux.Handle("/api/candidate_pool/list", NewCandidatePoolHandler())
}

// CandidatePoolAssetMiddleware 供 Wails AssetServer 挂载 /api/candidate_pool/*。
func CandidatePoolAssetMiddleware(next http.Handler) http.Handler {
	h := NewCandidatePoolHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/candidate_pool/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ChainAssetMiddleware 串联多个 AssetServer 中间件（先匹配的优先）。
func ChainAssetMiddleware(mw ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		h := next
		for i := len(mw) - 1; i >= 0; i-- {
			h = mw[i](h)
		}
		return h
	}
}
