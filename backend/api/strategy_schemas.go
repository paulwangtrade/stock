package api

import (
	"errors"
	"net/http"
	"strings"

	"go-stock/backend/strategyschema"
)

// StrategySchemasHandler serves Phase13 Strategy Schema APIs.
// B3-A: GET list/detail/revisions. B3-B: Create/Update Draft, Submit, Activate.
// No AI / Intent / Promote / TradePlan / Execution.
type StrategySchemasHandler struct{}

func NewStrategySchemasHandler() *StrategySchemasHandler {
	return &StrategySchemasHandler{}
}

func (h *StrategySchemasHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewStrategySchemasHandler()
	}
	path := strings.TrimSuffix(r.URL.Path, "/")

	switch {
	case path == "/api/strategy/schemas":
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreateDraft(w, r)
		default:
			writeSchemaMethodNotAllowed(w)
		}
	case strings.HasPrefix(path, "/api/strategy/schemas/"):
		rest := strings.TrimPrefix(path, "/api/strategy/schemas/")
		rest = strings.Trim(rest, "/")
		if rest == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		h.routeNested(w, r, rest)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *StrategySchemasHandler) routeNested(w http.ResponseWriter, r *http.Request, rest string) {
	// {id}/revisions/{version}/submit|activate
	if i := strings.Index(rest, "/revisions/"); i >= 0 {
		id := rest[:i]
		after := strings.TrimPrefix(rest[i:], "/revisions/")
		after = strings.Trim(after, "/")
		parts := strings.Split(after, "/")
		if id == "" || len(parts) == 0 || parts[0] == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		version := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				h.handleRevisionDetail(w, r, id, version)
			case http.MethodPatch:
				h.handleUpdateDraft(w, r, id, version)
			default:
				writeSchemaMethodNotAllowed(w)
			}
			return
		}
		if len(parts) == 2 {
			action := parts[1]
			if r.Method != http.MethodPost {
				writeSchemaMethodNotAllowed(w)
				return
			}
			switch action {
			case "submit":
				h.handleSubmit(w, r, id, version)
			case "activate":
				h.handleActivate(w, r, id, version)
			default:
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			}
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	// {id}/revisions
	if strings.HasSuffix(rest, "/revisions") {
		id := strings.TrimSuffix(rest, "/revisions")
		id = strings.Trim(id, "/")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if r.Method != http.MethodGet {
			writeSchemaMethodNotAllowed(w)
			return
		}
		h.handleRevisionList(w, r, id)
		return
	}

	if strings.Contains(rest, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeSchemaMethodNotAllowed(w)
		return
	}
	h.handleDetail(w, r, rest)
}

func (h *StrategySchemasHandler) handleList(w http.ResponseWriter, r *http.Request) {
	out, err := strategyschema.ListSchemas()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "list_failed", "message": err.Error()})
		return
	}
	if st := strings.TrimSpace(r.URL.Query().Get("status")); st != "" {
		filtered := make([]strategyschema.ListItem, 0, len(out.Items))
		for _, it := range out.Items {
			if it.Status == st {
				filtered = append(filtered, it)
			}
		}
		out.Items = filtered
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategySchemasHandler) handleDetail(w http.ResponseWriter, r *http.Request, id string) {
	out, err := strategyschema.GetSchema(id)
	if err != nil {
		writeSchemaErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategySchemasHandler) handleRevisionList(w http.ResponseWriter, r *http.Request, id string) {
	out, err := strategyschema.ListRevisions(id)
	if err != nil {
		writeSchemaErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategySchemasHandler) handleRevisionDetail(w http.ResponseWriter, r *http.Request, id, version string) {
	out, err := strategyschema.GetRevision(id, version)
	if err != nil {
		writeSchemaErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategySchemasHandler) handleCreateDraft(w http.ResponseWriter, r *http.Request) {
	var req strategyschema.CreateDraftRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json", "message": err.Error()})
		return
	}
	out, err := strategyschema.CreateDraft(req)
	if err != nil {
		writeSchemaErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *StrategySchemasHandler) handleUpdateDraft(w http.ResponseWriter, r *http.Request, id, version string) {
	var req strategyschema.UpdateDraftRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json", "message": err.Error()})
		return
	}
	out, err := strategyschema.UpdateDraft(id, version, req)
	if err != nil {
		writeSchemaErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategySchemasHandler) handleSubmit(w http.ResponseWriter, r *http.Request, id, version string) {
	out, err := strategyschema.SubmitReview(id, version)
	if err != nil {
		writeSchemaErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategySchemasHandler) handleActivate(w http.ResponseWriter, r *http.Request, id, version string) {
	out, err := strategyschema.Activate(id, version)
	if err != nil {
		writeSchemaErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeSchemaMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"error":   "method_not_allowed",
		"message": "allowed: GET; POST create/submit/activate; PATCH draft only (B3-B)",
	})
}

func writeSchemaErr(w http.ResponseWriter, err error) {
	var nf strategyschema.ErrNotFound
	if errors.As(err, &nf) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": strategyschema.CodeNotFound, "message": err.Error()})
		return
	}
	var we strategyschema.WriteError
	if errors.As(err, &we) {
		status := http.StatusBadRequest
		switch we.Code {
		case strategyschema.CodeDraftConflict:
			status = http.StatusConflict
		case strategyschema.CodeNotFound:
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": we.Code, "message": we.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal", "message": err.Error()})
}

// RegisterStrategySchemasRoutes mounts schema routes.
func RegisterStrategySchemasRoutes(mux *http.ServeMux) {
	h := NewStrategySchemasHandler()
	mux.Handle("/api/strategy/schemas", h)
	mux.Handle("/api/strategy/schemas/", h)
}

// StrategySchemasAssetMiddleware mounts /api/strategy/schemas* on Wails AssetServer.
func StrategySchemasAssetMiddleware(next http.Handler) http.Handler {
	h := NewStrategySchemasHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		if path == "/api/strategy/schemas" || strings.HasPrefix(path, "/api/strategy/schemas/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
