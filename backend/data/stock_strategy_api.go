package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"reflect"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
)

type StockStrategyApi struct{}

func NewStockStrategyApi() *StockStrategyApi {
	return &StockStrategyApi{}
}

func (a *StockStrategyApi) Create(s *models.StockStrategy) error {
	if s.PageSize <= 0 {
		s.PageSize = 50
	}
	return db.Dao.Create(s).Error
}

func (a *StockStrategyApi) Update(s *models.StockStrategy) error {
	if s.PageSize <= 0 {
		s.PageSize = 50
	}
	return db.Dao.Save(s).Error
}

func (a *StockStrategyApi) Delete(id uint) error {
	_ = db.Dao.Where("strategy_id = ?", id).Delete(&models.StockStrategyRun{})
	return db.Dao.Delete(&models.StockStrategy{}, id).Error
}

func (a *StockStrategyApi) GetByID(id uint) (*models.StockStrategy, error) {
	var s models.StockStrategy
	err := db.Dao.First(&s, id).Error
	return &s, err
}

func (a *StockStrategyApi) List(q *models.StockStrategyQuery) *models.StockStrategyPageResp {
	var list []models.StockStrategy
	var total int64
	query := db.Dao.Model(&models.StockStrategy{})
	if q.Name != "" {
		query = query.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.QueryType != "" {
		query = query.Where("query_type = ?", q.QueryType)
	}
	query.Count(&total)
	page, pageSize := q.Page, q.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	return &models.StockStrategyPageResp{Total: int(total), Data: list}
}

func (a *StockStrategyApi) GetAllEnabled() []models.StockStrategy {
	var list []models.StockStrategy
	db.Dao.Where("enable = ? AND cron_expr <> ''", true).Find(&list)
	return list
}

func (a *StockStrategyApi) ListRuns(q *models.StockStrategyRunQuery) *models.StockStrategyRunPageResp {
	var list []models.StockStrategyRun
	var total int64
	query := db.Dao.Model(&models.StockStrategyRun{}).Where("strategy_id = ?", q.StrategyID)
	query.Count(&total)
	page, pageSize := q.Page, q.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	return &models.StockStrategyRunPageResp{Total: int(total), Data: list}
}

func (a *StockStrategyApi) GetRunByID(id uint) (*models.StockStrategyRun, error) {
	var run models.StockStrategyRun
	err := db.Dao.First(&run, id).Error
	return &run, err
}

func (a *StockStrategyApi) SummaryText(s *models.StockStrategy) string {
	switch s.QueryType {
	case "eastmoney_nl":
		t := strings.TrimSpace(s.QueryText)
		inds := parseNlIndustryList(s.Industry)
		var parts []string
		if len(inds) > 0 {
			parts = append(parts, "行业:"+strings.Join(inds, "、"))
		}
		if t != "" {
			if len(t) > 80 {
				t = t[:80] + "…"
			}
			parts = append(parts, t)
		}
		if len(parts) == 0 {
			return "自然语言条件（未填写）"
		}
		return strings.Join(parts, "；")
	case "technical":
		var ind models.TechnicalIndicators
		if s.QueryJSON != "" {
			_ = json.Unmarshal([]byte(s.QueryJSON), &ind)
		}
		parts := technicalIndicatorsSummary(&ind)
		if s.Keyword != "" {
			parts = append(parts, "关键词:"+s.Keyword)
		}
		if s.Industry != "" {
			parts = append(parts, "行业:"+s.Industry)
		}
		if len(parts) == 0 {
			return "技术面条件（未勾选）"
		}
		return strings.Join(parts, "；")
	default:
		return s.QueryType
	}
}

func hasActiveTechnicalIndicator(ind models.TechnicalIndicators) bool {
	v := reflect.ValueOf(ind)
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		switch t.Field(i).Type.Kind() {
		case reflect.Bool:
			if v.Field(i).Bool() {
				return true
			}
		case reflect.Int, reflect.Int64:
			if v.Field(i).Int() > 0 {
				return true
			}
		}
	}
	return false
}

/** 自然语言策略：Industry 存 JSON 数组或旧版单行业文本 */
func parseNlIndustryList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, s := range arr {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return []string{raw}
}

func mergeEastMoneyNlQuery(s *models.StockStrategy) string {
	words := strings.TrimSpace(s.QueryText)
	inds := parseNlIndustryList(s.Industry)
	if len(inds) == 0 {
		return words
	}
	prefix := strings.Join(inds, "；")
	if words == "" {
		return prefix
	}
	return prefix + "；" + words
}

func technicalIndicatorsSummary(ind *models.TechnicalIndicators) []string {
	var parts []string
	if ind.MACDGOLDENFORK {
		parts = append(parts, "MACD金叉")
	}
	if ind.KDJGOLDENFORK {
		parts = append(parts, "KDJ金叉")
	}
	if ind.DOWN7DAYS {
		parts = append(parts, "七连阴")
	}
	if ind.REVERSINGHAMMER {
		parts = append(parts, "倒转锤头")
	}
	if ind.MORNINGSTAR {
		parts = append(parts, "早晨之星")
	}
	if ind.SHORTAVGARRAY {
		parts = append(parts, "均线空头")
	}
	if ind.DOWNNDAY >= 3 {
		parts = append(parts, fmt.Sprintf("连跌≥%d天", ind.DOWNNDAY))
	}
	if ind.UPNDAY >= 3 {
		parts = append(parts, fmt.Sprintf("连涨≥%d天", ind.UPNDAY))
	}
	if ind.BREAKTHROUGH {
		parts = append(parts, "放量突破")
	}
	if ind.LOWFUNDSINFLOW {
		parts = append(parts, "低位资金流入")
	}
	return parts
}

func (a *StockStrategyApi) RunStrategy(s *models.StockStrategy) *models.StockStrategyRunView {
	view := &models.StockStrategyRunView{
		StrategyID: s.ID,
		QueryType:  s.QueryType,
		Code:       -1,
		Message:    "未知错误",
	}
	pageSize := s.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	switch s.QueryType {
	case "eastmoney_nl":
		words := mergeEastMoneyNlQuery(s)
		if words == "" {
			view.Message = "自然语言条件不能为空"
			a.saveRun(s, view)
			return view
		}
		res := NewSearchStockApi(words).SearchStock(pageSize)
		view = mapSearchStockResult(s.ID, res)
	case "technical":
		var ind models.TechnicalIndicators
		if s.QueryJSON != "" {
			if err := json.Unmarshal([]byte(s.QueryJSON), &ind); err != nil {
				view.Message = "技术面参数 JSON 无效: " + err.Error()
				a.saveRun(s, view)
				return view
			}
		}
		if !hasActiveTechnicalIndicator(ind) {
			view.Message = "请至少勾选一项技术面条件"
			a.saveRun(s, view)
			return view
		}
		resp := NewStockDataApi().GetAllStocks(1, pageSize, strings.TrimSpace(s.Keyword), strings.TrimSpace(s.Industry), "", "", ind)
		view = mapTechnicalResult(s.ID, resp)
	default:
		view.Message = "不支持的策略类型: " + s.QueryType
	}
	a.saveRun(s, view)
	return view
}

func mapSearchStockResult(strategyID uint, res map[string]any) *models.StockStrategyRunView {
	view := &models.StockStrategyRunView{
		StrategyID: strategyID,
		QueryType:  "eastmoney_nl",
		Code:       -1,
		Message:    "选股失败",
	}
	if res == nil {
		return view
	}
	if msg, ok := res["message"].(string); ok && msg != "" {
		view.Message = msg
	}
	if msg, ok := res["msg"].(string); ok && msg != "" {
		view.Message = msg
	}
	code, _ := convertor.ToInt(res["code"])
	if code == 100 || code == 1 {
		view.Code = 0
		view.Message = "success"
		if data, ok := res["data"].(map[string]any); ok {
			if result, ok := data["result"].(map[string]any); ok {
				if trace, ok := data["traceInfo"].(map[string]any); ok {
					if show, ok := trace["showText"].(string); ok {
						view.TraceInfo = show
					}
				}
				view.Columns = result["columns"]
				view.DataList = result["dataList"]
				if list, ok := result["dataList"].([]any); ok {
					view.StockCount = len(list)
				}
			}
		}
		return view
	}
	if code == 0 {
		view.Code = 0
	}
	return view
}

func mapTechnicalResult(strategyID uint, resp *models.AllStocksResp) *models.StockStrategyRunView {
	view := &models.StockStrategyRunView{
		StrategyID: strategyID,
		QueryType:  "technical",
		Code:       -1,
		Message:    "获取股票数据失败",
	}
	if resp == nil {
		return view
	}
	if !resp.Success && resp.Message != "" {
		view.Message = resp.Message
		return view
	}
	view.Code = 0
	view.Message = "success"
	view.DataList = resp.Result.Data
	view.StockCount = len(resp.Result.Data)
	return view
}

func (a *StockStrategyApi) saveRun(s *models.StockStrategy, view *models.StockStrategyRunView) {
	now := time.Now()
	errMsg := ""
	if view.Code != 0 {
		errMsg = view.Message
	}
	_ = db.Dao.Model(&models.StockStrategy{}).Where("id = ?", s.ID).Updates(map[string]any{
		"last_run_at":     now,
		"last_run_count":  view.StockCount,
		"last_run_error":  errMsg,
		"updated_at":      now,
	}).Error
	raw, err := json.Marshal(view)
	if err != nil {
		logger.SugaredLogger.Errorf("marshal strategy run: %v", err)
		return
	}
	run := &models.StockStrategyRun{
		StrategyID: s.ID,
		StockCount: view.StockCount,
		Message:    view.Message,
		ResultJSON: string(raw),
	}
	_ = db.Dao.Create(run).Error
	view.RunID = run.ID
}
