package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

// @Author spark
// @Date 2025/8/4 17:58
// @Desc Phase7-A2-2-1：Agent 查价经 QuoteService（LegacyQuoteAdapter），仅市场字段
//-----------------------------------------------------------------------------------

func GetQueryStockPriceInfoTool() tool.InvokableTool {
	return &ToolQueryStockPriceInfo{}
}

type ToolQueryStockPriceInfo struct{}

func (t ToolQueryStockPriceInfo) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "QueryStockPriceInfo",
		Desc: "批量获取实时股价数据",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"stockCodes": {
				Type:     "string",
				Desc:     "股票代码,多个,隔开,股票代码必须转化为sh或者sz或者hk开头的形式，例如：sz399001,sh600859",
				Required: true,
			},
		}),
	}, nil
}

func (t ToolQueryStockPriceInfo) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	parms := map[string]any{}
	err := json.Unmarshal([]byte(argumentsInJSON), &parms)
	if err != nil {
		return "", err
	}
	stockCodes := strings.Split(parms["stockCodes"].(string), ",")
	var codes []string
	for _, code := range stockCodes {
		codes = append(codes, GetStockCode(code))
	}
	return QueryStockPriceInfoJSON(data.GetQuoteService(), codes)
}

// agentQuoteJSON 与旧 StockInfo 市场字段 JSON 标签对齐，仅含行情域（无 cost/position/profit/follow/alarm/order）。
type agentQuoteJSON struct {
	Date     string `json:"日期"`
	Time     string `json:"时间"`
	Code     string `json:"股票代码"`
	Name     string `json:"股票名称"`
	Price    string `json:"当前价格"`
	Volume   string `json:"成交的股票数"`
	Amount   string `json:"成交金额"`
	Open     string `json:"今日开盘价"`
	PreClose string `json:"昨日收盘价"`
	High     string `json:"今日最高价"`
	Low      string `json:"今日最低价"`
}

func quoteToAgentJSON(q marketdata.Quote) agentQuoteJSON {
	return agentQuoteJSON{
		Date:     q.Date,
		Time:     q.Time,
		Code:     q.Code,
		Name:     q.Name,
		Price:    formatQuoteNumber(q.Price),
		Volume:   formatQuoteNumber(q.Volume),
		Amount:   formatQuoteNumber(q.Amount),
		Open:     formatQuoteNumber(q.Open),
		PreClose: formatQuoteNumber(q.PreClose),
		High:     formatQuoteNumber(q.High),
		Low:      formatQuoteNumber(q.Low),
	}
}

func formatQuoteNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// QueryStockPriceInfoJSON 经 QuoteService 批量取行情并序列化为 Agent JSON（可注入 fake，无 DB 依赖）。
func QueryStockPriceInfoJSON(svc marketdata.QuoteService, codes []string) (string, error) {
	if svc == nil {
		return "", errors.New("QuoteService 未初始化")
	}
	quotes, err := svc.GetQuotes(codes)
	if err != nil {
		return "", err
	}
	out := make([]agentQuoteJSON, 0, len(quotes))
	for _, q := range quotes {
		out = append(out, quoteToAgentJSON(q))
	}
	marshal, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(marshal), nil
}

// RenderStockInfoMarkdown 将单只 Quote 渲染为 GetStockInfo markdown（仅市场字段）。
func RenderStockInfoMarkdown(q marketdata.Quote) string {
	price := q.Price
	preClose := q.PreClose
	change := q.ChangeValue
	pChange := q.ChangePercent
	if change == 0 && preClose > 0 {
		change = price - preClose
	}
	if pChange == 0 && preClose > 0 {
		pChange = (price - preClose) / preClose * 100
	}
	ts := strings.TrimSpace(q.Date + " " + q.Time)
	if ts == "" && !q.FetchedAt.IsZero() {
		ts = q.FetchedAt.Format(time.DateTime)
	}
	return fmt.Sprintf("### %s %s\n\n| 项目 | 值 |\n| --- | --- |\n| 股票代码 | %s |\n| 股票名称 | %s |\n| 当前价格 | %.2f |\n| 涨跌额 | %.2f |\n| 涨跌幅 | %.2f%% |\n| 成交量 | %s手 |\n| 成交额 | %s元 |\n| 今开 | %s |\n| 昨收 | %s |\n| 最高 | %s |\n| 最低 | %s |\n| 时间 | %s |",
		q.Name, q.Code,
		q.Code, q.Name, price, change, pChange,
		formatQuoteNumber(q.Volume), formatQuoteNumber(q.Amount),
		formatQuoteNumber(q.Open), formatQuoteNumber(q.PreClose),
		formatQuoteNumber(q.High), formatQuoteNumber(q.Low),
		ts)
}

// RenderGetStockInfo 经 QuoteService 批量取数并按入参顺序渲染（可注入 fake，无 DB 依赖）。
func RenderGetStockInfo(svc marketdata.QuoteService, codes []string) string {
	if svc == nil {
		return "QuoteService 未初始化"
	}
	var results []string
	quotes, err := svc.GetQuotes(codes)
	if err != nil {
		for _, code := range codes {
			if code == "" {
				continue
			}
			results = append(results, code+"：未找到股票信息")
		}
		return strings.Join(results, "\n\n")
	}
	for _, code := range codes {
		if code == "" {
			continue
		}
		if q := marketdata.FindQuote(quotes, code); q != nil {
			results = append(results, RenderStockInfoMarkdown(*q))
			continue
		}
		results = append(results, code+"：未找到股票信息")
	}
	return strings.Join(results, "\n\n")
}
