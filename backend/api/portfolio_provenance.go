package api

import (
	"errors"
	"net/http"
	"strings"

	"go-stock/backend/portfolio/provenance"
)

const provenancePathPrefix = "/api/portfolio/positions/"

// PortfolioProvenanceHandler serves GET /api/portfolio/positions/{stock_code}/provenance.
type PortfolioProvenanceHandler struct {
	svc *provenance.Service
}

// NewPortfolioProvenanceHandler returns a handler. nil svc → provenance.NewService().
func NewPortfolioProvenanceHandler(svc *provenance.Service) *PortfolioProvenanceHandler {
	if svc == nil {
		svc = provenance.NewService()
	}
	return &PortfolioProvenanceHandler{svc: svc}
}

func parseProvenancePath(path string) (stockCode string, ok bool) {
	path = strings.TrimSuffix(path, "/")
	if !strings.HasPrefix(path, provenancePathPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, provenancePathPrefix)
	if rest == "" {
		return "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[1] != "provenance" {
		return "", false
	}
	code := strings.TrimSpace(parts[0])
	if code == "" {
		return "", false
	}
	return code, true
}

func (h *PortfolioProvenanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code, ok := parseProvenancePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}

	view, err := h.svc.Evaluate(code)
	if err != nil {
		if errors.Is(err, provenance.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"code": 404, "ok": false, "message": "position not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":       0,
		"ok":         true,
		"provenance": view,
	})
}

// RegisterPortfolioProvenanceRoutes mounts the provenance route pattern.
func RegisterPortfolioProvenanceRoutes(mux *http.ServeMux) {
	mux.Handle(provenancePathPrefix, NewPortfolioProvenanceHandler(nil))
}
