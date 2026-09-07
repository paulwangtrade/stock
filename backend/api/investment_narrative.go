package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/investmentnarrative"
)

// InvestmentNarrativeHandler serves GET /api/investment/narrative/{stock_code} (read-only).
type InvestmentNarrativeHandler struct{}

func NewInvestmentNarrativeHandler() *InvestmentNarrativeHandler {
	return &InvestmentNarrativeHandler{}
}

func (h *InvestmentNarrativeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/investment/narrative/") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}

	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/investment/narrative/"), "/")
	if code == "" || strings.Contains(code, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": 400, "ok": false, "message": "stock_code required",
		})
		return
	}

	var accountID uint
	if raw := strings.TrimSpace(r.URL.Query().Get("account_id")); raw != "" {
		if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
			accountID = uint(v)
		}
	}

	view, err := investmentnarrative.BuildInvestmentNarrative(investmentnarrative.BuildOptions{
		StockCode: code,
		AccountID: accountID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":       0,
		"ok":         true,
		"narrative":  view,
	})
}

// RegisterInvestmentNarrativeRoutes mounts narrative route (httptest).
func RegisterInvestmentNarrativeRoutes(mux *http.ServeMux) {
	h := NewInvestmentNarrativeHandler()
	mux.Handle("/api/investment/narrative/", h)
}
