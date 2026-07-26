// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A2：注册 marketdata Adapter 为 data 包默认服务（懒加载）
// A2-1：EastMoneyKlineAdapter → KlineService
// A2-2-1：LegacyQuoteAdapter → QuoteService

package tools

import (
	"go-stock/backend/data"
	"go-stock/backend/marketdata"
	"go-stock/backend/marketdata/adapter"
)

func init() {
	// 懒加载：避免 tools 包 init 时 DB/Setting 尚未就绪。
	// app.go 已 import 本包；Agent 工具经 interface 取数，不在业务闭包中 import adapter。
	data.SetKlineServiceFactory(func() marketdata.KlineService {
		return adapter.NewEastMoneyKlineAdapter()
	})
	data.SetQuoteServiceFactory(func() marketdata.QuoteService {
		return adapter.NewLegacyQuoteAdapter()
	})
}
