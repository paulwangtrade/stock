// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A2-1：注册 EastMoneyKlineAdapter 为 data 包默认 KlineService（懒加载）

package tools

import (
	"go-stock/backend/data"
	"go-stock/backend/marketdata"
	"go-stock/backend/marketdata/adapter"
)

func init() {
	// 懒加载：避免 tools 包 init 时 DB/Setting 尚未就绪。
	// app.go 已 import 本包；桌面 Agent / OpenAI tool handler 取数走 KlineService。
	data.SetKlineServiceFactory(func() marketdata.KlineService {
		return adapter.NewEastMoneyKlineAdapter()
	})
}
