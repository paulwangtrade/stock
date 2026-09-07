package execution

import (
	"math"
	"strings"
	"sync/atomic"
	"time"
)

// RealBrokerMetrics RealBroker 基础监控快照（进程内累计；不接入 Prometheus）。
type RealBrokerMetrics struct {
	// 订单提交相关（生命周期事件计数，非互斥快照）
	OrdersPending  int64 // Submit 成功次数
	OrdersAccepted int64 // 首次 ACK 次数
	OrdersRejected int64 // 首次 Reject 次数

	// 成交
	FillsBuy     int64   // 买入成交笔数
	FillsSell    int64   // 卖出成交笔数
	FillNotional float64 // 成交总金额 Σ(qty * price)

	// 撤单
	Cancels int64

	// Submit → 首次 ACK 平均耗时
	AvgSubmitToAck time.Duration
	AckSamples     int64 // 参与均时统计的 ACK 样本数
}

// realBrokerMetricStore atomic 累计器（嵌入 RealBroker，零值可用）。
type realBrokerMetricStore struct {
	ordersPending   atomic.Int64
	ordersAccepted  atomic.Int64
	ordersRejected  atomic.Int64
	fillsBuy        atomic.Int64
	fillsSell       atomic.Int64
	fillNotional    atomic.Uint64 // float64 bits
	cancels         atomic.Int64
	ackLatencySumNs atomic.Int64
	ackSamples      atomic.Int64
}

func (m *realBrokerMetricStore) snapshot() RealBrokerMetrics {
	if m == nil {
		return RealBrokerMetrics{}
	}
	out := RealBrokerMetrics{
		OrdersPending:  m.ordersPending.Load(),
		OrdersAccepted: m.ordersAccepted.Load(),
		OrdersRejected: m.ordersRejected.Load(),
		FillsBuy:       m.fillsBuy.Load(),
		FillsSell:      m.fillsSell.Load(),
		FillNotional:   math.Float64frombits(m.fillNotional.Load()),
		Cancels:        m.cancels.Load(),
		AckSamples:     m.ackSamples.Load(),
	}
	if out.AckSamples > 0 {
		out.AvgSubmitToAck = time.Duration(m.ackLatencySumNs.Load() / out.AckSamples)
	}
	return out
}

func (m *realBrokerMetricStore) recordSubmitPending() {
	if m == nil {
		return
	}
	m.ordersPending.Add(1)
}

func (m *realBrokerMetricStore) recordAccepted(latency time.Duration) {
	if m == nil {
		return
	}
	m.ordersAccepted.Add(1)
	if latency < 0 {
		latency = 0
	}
	m.ackLatencySumNs.Add(latency.Nanoseconds())
	m.ackSamples.Add(1)
}

func (m *realBrokerMetricStore) recordRejected() {
	if m == nil {
		return
	}
	m.ordersRejected.Add(1)
}

func (m *realBrokerMetricStore) recordCancel() {
	if m == nil {
		return
	}
	m.cancels.Add(1)
}

func (m *realBrokerMetricStore) recordFill(side string, qty int64, price float64) {
	if m == nil || qty <= 0 || price <= 0 {
		return
	}
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "sell":
		m.fillsSell.Add(1)
	default:
		m.fillsBuy.Add(1)
	}
	addAtomicFloat64(&m.fillNotional, float64(qty)*price)
}

func addAtomicFloat64(addr *atomic.Uint64, delta float64) {
	for {
		old := addr.Load()
		next := math.Float64bits(math.Float64frombits(old) + delta)
		if addr.CompareAndSwap(old, next) {
			return
		}
	}
}
