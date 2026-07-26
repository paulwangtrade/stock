// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A2-1：向 data 包注入 KlineService，避免 data→adapter→data 循环依赖

package data

import (
	"sync"

	"go-stock/backend/marketdata"
)

var (
	klineServiceMu      sync.RWMutex
	klineService        marketdata.KlineService
	klineServiceFactory func() marketdata.KlineService
)

// SetKlineService 注入已构造的只读 K 线服务。
func SetKlineService(svc marketdata.KlineService) {
	klineServiceMu.Lock()
	defer klineServiceMu.Unlock()
	klineService = svc
}

// SetKlineServiceFactory 注入懒加载工厂（推荐：避免 init 阶段访问未就绪的 DB/配置）。
func SetKlineServiceFactory(factory func() marketdata.KlineService) {
	klineServiceMu.Lock()
	defer klineServiceMu.Unlock()
	klineServiceFactory = factory
}

// GetKlineService 返回已注入的 KlineService；若仅有工厂则懒创建并缓存。
func GetKlineService() marketdata.KlineService {
	klineServiceMu.RLock()
	svc := klineService
	factory := klineServiceFactory
	klineServiceMu.RUnlock()
	if svc != nil {
		return svc
	}
	if factory == nil {
		return nil
	}
	klineServiceMu.Lock()
	defer klineServiceMu.Unlock()
	if klineService != nil {
		return klineService
	}
	klineService = factory()
	return klineService
}
