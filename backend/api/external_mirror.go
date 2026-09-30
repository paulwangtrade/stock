package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/externalmirror"
)

const externalMirrorHoldingsPath = "/api/external-mirror/holdings"

// ExternalMirrorHandler serves manual external_mirror holding CRUD.
// It does not read or write paper_sim_* / TradePlan / Track-A.
type ExternalMirrorHandler struct{}

func NewExternalMirrorHandler() *ExternalMirrorHandler {
	return &ExternalMirrorHandler{}
}

// RegisterExternalMirrorRoutes mounts observation-only mirror holding routes.
func RegisterExternalMirrorRoutes(mux *http.ServeMux) {
	h := NewExternalMirrorHandler()
	mux.Handle(externalMirrorHoldingsPath, h)
	mux.Handle(externalMirrorHoldingsPath+"/", h)
}

// ExternalMirrorAssetMiddleware mounts /api/external-mirror/* on the Wails asset server.
func ExternalMirrorAssetMiddleware(next http.Handler) http.Handler {
	h := NewExternalMirrorHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == externalMirrorHoldingsPath || strings.HasPrefix(r.URL.Path, externalMirrorHoldingsPath+"/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type mirrorUpsertBody struct {
	StockCode      string   `json:"stockCode"`
	StockCodeSnake string   `json:"stock_code"`
	StockName      string   `json:"stockName"`
	StockNameSnake string   `json:"stock_name"`
	Quantity       *float64 `json:"quantity"`
	CostPrice      *float64 `json:"costPrice"`
	CostPriceSnake *float64 `json:"cost_price"`
	EntryDate      string   `json:"entryDate"`
	EntryDateSnake string   `json:"entry_date"`
	Note           string   `json:"note"`
	Source         string   `json:"source"`
	FeedsTradePlan *bool    `json:"feedsTradePlan"`
	Tradable       *bool    `json:"tradable"`
}

func (b mirrorUpsertBody) toInput(nameSet, noteSet bool) externalmirror.Input {
	code := strings.TrimSpace(b.StockCode)
	if code == "" {
		code = strings.TrimSpace(b.StockCodeSnake)
	}
	name := b.StockName
	if strings.TrimSpace(name) == "" && strings.TrimSpace(b.StockNameSnake) != "" {
		name = b.StockNameSnake
	}
	cost := b.CostPrice
	if cost == nil {
		cost = b.CostPriceSnake
	}
	entry := b.EntryDate
	if strings.TrimSpace(entry) == "" {
		entry = b.EntryDateSnake
	}
	return externalmirror.Input{
		StockCode:      code,
		StockName:      name,
		Quantity:       b.Quantity,
		CostPrice:      cost,
		EntryDate:      entry,
		Note:           b.Note,
		Source:         b.Source,
		FeedsTradePlan: b.FeedsTradePlan,
		Tradable:       b.Tradable,
		NameSet:        nameSet,
		NoteSet:        noteSet,
	}
}

func (h *ExternalMirrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == externalMirrorHoldingsPath {
		switch r.Method {
		case http.MethodGet:
			h.list(w, r)
		case http.MethodPost:
			h.create(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"ok": false, "code": 405, "message": "GET or POST required",
			})
		}
		return
	}
	if !strings.HasPrefix(r.URL.Path, externalMirrorHoldingsPath+"/") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.Trim(strings.TrimPrefix(r.URL.Path, externalMirrorHoldingsPath+"/"), "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		http.NotFound(w, r)
		return
	}
	id64, err := strconv.ParseUint(idPart, 10, 64)
	if err != nil || id64 == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": 400, "reason": "not_found", "message": "记录不存在",
		})
		return
	}
	switch r.Method {
	case http.MethodPut:
		h.update(w, r, uint(id64))
	case http.MethodDelete:
		h.delete(w, uint(id64))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok": false, "code": 405, "message": "PUT or DELETE required",
		})
	}
}

func mirrorMeta() map[string]any {
	return map[string]any{
		"source":         externalmirror.Source,
		"disclaimer":     externalmirror.Disclaimer,
		"feedsTradePlan": false,
		"tradable":       false,
	}
}

func (h *ExternalMirrorHandler) list(w http.ResponseWriter, _ *http.Request) {
	rows, err := externalmirror.List()
	if err != nil {
		writeMirrorErr(w, err)
		return
	}
	body := mirrorMeta()
	body["ok"] = true
	body["code"] = 0
	body["holdings"] = rows
	writeJSON(w, http.StatusOK, body)
}

func (h *ExternalMirrorHandler) create(w http.ResponseWriter, r *http.Request) {
	in, nameSet, noteSet, err := decodeMirrorBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": 400, "reason": "invalid_body", "message": "请求体无效",
		})
		return
	}
	view, err := externalmirror.Create(in.toInput(nameSet, noteSet))
	if err != nil {
		writeMirrorErr(w, err)
		return
	}
	body := mirrorMeta()
	body["ok"] = true
	body["code"] = 0
	body["holding"] = view
	writeJSON(w, http.StatusOK, body)
}

func (h *ExternalMirrorHandler) update(w http.ResponseWriter, r *http.Request, id uint) {
	in, nameSet, noteSet, err := decodeMirrorBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": 400, "reason": "invalid_body", "message": "请求体无效",
		})
		return
	}
	view, err := externalmirror.Update(id, in.toInput(nameSet, noteSet))
	if err != nil {
		writeMirrorErr(w, err)
		return
	}
	body := mirrorMeta()
	body["ok"] = true
	body["code"] = 0
	body["holding"] = view
	writeJSON(w, http.StatusOK, body)
}

func (h *ExternalMirrorHandler) delete(w http.ResponseWriter, id uint) {
	if err := externalmirror.Delete(id); err != nil {
		writeMirrorErr(w, err)
		return
	}
	body := mirrorMeta()
	body["ok"] = true
	body["code"] = 0
	writeJSON(w, http.StatusOK, body)
}

func decodeMirrorBody(r *http.Request) (mirrorUpsertBody, bool, bool, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return mirrorUpsertBody{}, false, false, err
	}
	if len(bytes.TrimSpace(raw)) == 0 || !json.Valid(raw) {
		return mirrorUpsertBody{}, false, false, errors.New("invalid json")
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return mirrorUpsertBody{}, false, false, err
	}
	var body mirrorUpsertBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return mirrorUpsertBody{}, false, false, err
	}
	_, nameSet := keys["stockName"]
	if !nameSet {
		_, nameSet = keys["stock_name"]
	}
	_, noteSet := keys["note"]
	return body, nameSet, noteSet, nil
}

func writeMirrorErr(w http.ResponseWriter, err error) {
	var v *externalmirror.ValidationError
	if errors.As(err, &v) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": 400, "reason": v.Reason, "message": v.Message,
			"source": externalmirror.Source, "feedsTradePlan": false, "tradable": false,
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"ok": false, "code": 500, "message": err.Error(),
	})
}
