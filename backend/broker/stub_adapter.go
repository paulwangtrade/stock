package broker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

// Stub 侧状态常量（券商通道语义，非 OMS）。
const (
	StubStatusPending   = "pending"
	StubStatusWorking   = "working"
	StubStatusCancelled = "cancelled"
	StubStatusRejected  = "rejected"

	// 与 data 包前缀对齐，但本包不 import data（避免循环依赖）。
	stubClientOrderIDPrefix = "real_"
	stubBrokerOrderIDPrefix = "real_stub_"
)

// StubAdapter Phase4：从 RealStub 抽离的模拟券商通道（不接真 API、不发 EventHub、不写 Paper）。
type StubAdapter struct {
	mu        sync.Mutex
	connected bool
	orders    map[string]*stubOrder // client_order_id → stub
}

type stubOrder struct {
	req           SubmitRequest
	brokerOrderID string
	status        string
	filledVolume  int64
	leaves        int64
	avgPrice      float64
}

// NewStubAdapter 创建未连接的 Stub 适配器。
func NewStubAdapter() *StubAdapter {
	return &StubAdapter{
		orders: make(map[string]*stubOrder),
	}
}

// NewConnectedStubAdapter 创建并 Connect，供 RealBroker 默认注入。
func NewConnectedStubAdapter() *StubAdapter {
	a := NewStubAdapter()
	_ = a.Connect(context.Background())
	return a
}

func (a *StubAdapter) Connect(ctx context.Context) error {
	_ = ctx
	if a == nil {
		return fmt.Errorf("broker: nil StubAdapter")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.connected = true
	return nil
}

func (a *StubAdapter) Disconnect(ctx context.Context) error {
	_ = ctx
	if a == nil {
		return fmt.Errorf("broker: nil StubAdapter")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.connected = false
	return nil
}

func (a *StubAdapter) ensureConnectedLocked() error {
	if !a.connected {
		return fmt.Errorf("broker: stub adapter not connected")
	}
	return nil
}

// Submit 受理模拟报单：登记通道侧 pending；broker_order_id 留空（由 ACK 回报生成）。
func (a *StubAdapter) Submit(ctx context.Context, req *SubmitRequest) (*SubmitResponse, error) {
	_ = ctx
	if a == nil {
		return nil, fmt.Errorf("broker: nil StubAdapter")
	}
	if req == nil {
		return nil, fmt.Errorf("broker: nil submit request")
	}
	cid := strings.TrimSpace(req.ClientOrderID)
	if cid == "" || strings.TrimSpace(req.StockCode) == "" || req.Price <= 0 || req.Volume <= 0 {
		return nil, fmt.Errorf("broker: invalid stub submit request")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.ensureConnectedLocked(); err != nil {
		return nil, err
	}
	if _, exists := a.orders[cid]; exists {
		return nil, fmt.Errorf("broker: duplicate client_order_id %q", cid)
	}
	cp := *req
	cp.ClientOrderID = cid
	a.orders[cid] = &stubOrder{
		req:    cp,
		status: StubStatusPending,
		leaves: req.Volume,
	}
	return &SubmitResponse{
		ClientOrderID: cid,
		BrokerOrderID: "",
		Status:        StubStatusPending,
		Message:       "stub accepted (pending ack)",
	}, nil
}

// Cancel 通道侧标记撤单（OMS 终态仍由 RealBroker / 回报施加）。
func (a *StubAdapter) Cancel(ctx context.Context, clientOrderID string) error {
	_ = ctx
	if a == nil {
		return fmt.Errorf("broker: nil StubAdapter")
	}
	cid := strings.TrimSpace(clientOrderID)
	if cid == "" {
		return fmt.Errorf("broker: empty client_order_id")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.ensureConnectedLocked(); err != nil {
		return err
	}
	o, ok := a.orders[cid]
	if !ok || o == nil {
		return fmt.Errorf("broker: stub order %q not found", cid)
	}
	switch o.status {
	case StubStatusCancelled, StubStatusRejected:
		return nil
	}
	o.status = StubStatusCancelled
	o.leaves = 0
	return nil
}

// Query 返回通道侧状态快照。
func (a *StubAdapter) Query(ctx context.Context, clientOrderID string) (*OrderStatus, error) {
	_ = ctx
	if a == nil {
		return nil, fmt.Errorf("broker: nil StubAdapter")
	}
	cid := strings.TrimSpace(clientOrderID)
	if cid == "" {
		return nil, fmt.Errorf("broker: empty client_order_id")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	o, ok := a.orders[cid]
	if !ok || o == nil {
		return nil, fmt.Errorf("broker: stub order %q not found", cid)
	}
	return &OrderStatus{
		ClientOrderID:  cid,
		BrokerOrderID:  o.brokerOrderID,
		Status:         o.status,
		FilledVolume:   o.filledVolume,
		LeavesQuantity: o.leaves,
		AvgPrice:       o.avgPrice,
	}, nil
}

// Restore 启动 hydrate 时把 OMS 快照回填到通道侧（不校验 connected）。
func (a *StubAdapter) Restore(req SubmitRequest, status, brokerOrderID string, filled, leaves int64, avg float64) {
	if a == nil {
		return
	}
	cid := strings.TrimSpace(req.ClientOrderID)
	if cid == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if status == "" {
		status = StubStatusPending
	}
	cp := req
	cp.ClientOrderID = cid
	a.orders[cid] = &stubOrder{
		req:           cp,
		brokerOrderID: strings.TrimSpace(brokerOrderID),
		status:        status,
		filledVolume:  filled,
		leaves:        leaves,
		avgPrice:      avg,
	}
}

// BindBrokerOrderID ACK 后回写模拟券商委托号（供 FakeHandler 调用）。
func (a *StubAdapter) BindBrokerOrderID(clientOrderID, brokerOrderID string) {
	if a == nil {
		return
	}
	cid := strings.TrimSpace(clientOrderID)
	bid := strings.TrimSpace(brokerOrderID)
	if cid == "" || bid == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if o, ok := a.orders[cid]; ok && o != nil {
		o.brokerOrderID = bid
		// accepted 为同步受理细态；通道侧 ACK 前亦视为可升 working。
		if o.status == StubStatusPending || o.status == "accepted" {
			o.status = StubStatusWorking
		}
	}
}

// NewStubBrokerOrderID 与 client_order_id 稳定 1:1（real_stub_ + client 后缀）。
func NewStubBrokerOrderID(clientOrderID string) string {
	suf := strings.TrimPrefix(strings.TrimSpace(clientOrderID), stubClientOrderIDPrefix)
	if suf == "" {
		var buf [8]byte
		_, _ = rand.Read(buf[:])
		suf = hex.EncodeToString(buf[:])
	}
	return stubBrokerOrderIDPrefix + suf
}

var _ BrokerAdapter = (*StubAdapter)(nil)
