// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A2-2-1：向 data 包注入 QuoteService，避免 data→adapter→data 循环依赖

package data

import (
	"sync"

	"go-stock/backend/marketdata"
)

var (
	quoteServiceMu      sync.RWMutex
	quoteService        marketdata.QuoteService
	quoteServiceFactory func() marketdata.QuoteService
)

// SetQuoteService 注入已构造的只读行情服务。
func SetQuoteService(svc marketdata.QuoteService) {
	quoteServiceMu.Lock()
	defer quoteServiceMu.Unlock()
	quoteService = svc
}

// SetQuoteServiceFactory 注入懒加载工厂（推荐：避免 init 阶段访问未就绪的 DB/配置）。
func SetQuoteServiceFactory(factory func() marketdata.QuoteService) {
	quoteServiceMu.Lock()
	defer quoteServiceMu.Unlock()
	quoteServiceFactory = factory
}

// GetQuoteService 返回已注入的 QuoteService；若仅有工厂则懒创建并缓存。
func GetQuoteService() marketdata.QuoteService {
	quoteServiceMu.RLock()
	svc := quoteService
	factory := quoteServiceFactory
	quoteServiceMu.RUnlock()
	if svc != nil {
		return svc
	}
	if factory == nil {
		return nil
	}
	quoteServiceMu.Lock()
	defer quoteServiceMu.Unlock()
	if quoteService != nil {
		return quoteService
	}
	quoteService = factory()
	return quoteService
}
