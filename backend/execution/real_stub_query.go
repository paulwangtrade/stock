package execution

import (
	"fmt"
	"strings"

	"go-stock/backend/broker"
	"go-stock/backend/data"
)

// RealStubOrderDetail 订单 + fills（只读查询，供 API/UI）。
type RealStubOrderDetail struct {
	Order broker.TradeOrder    `json:"order"`
	Fills []data.RealStubFill  `json:"fills"`
}

// ListRealStubOrders 列出全部 RealStub 订单（不经 RealBroker 内存，直接读库）。
func ListRealStubOrders() ([]broker.TradeOrder, error) {
	return newGormRealStubStore().ListOrders()
}

// GetRealStubOrderDetail 按 local / client / broker order id 查询订单及 fills。
func GetRealStubOrderDetail(orderKey string) (*RealStubOrderDetail, error) {
	key := strings.TrimSpace(orderKey)
	if key == "" {
		return nil, fmt.Errorf("execution: empty order key")
	}
	store := newGormRealStubStore()
	orders, err := store.ListOrders()
	if err != nil {
		return nil, err
	}
	var found *broker.TradeOrder
	for i := range orders {
		o := &orders[i]
		if o.ID == key || o.ClientOrderID == key || o.BrokerOrderID == key {
			cp := *o
			found = &cp
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("execution: real stub order %q not found", key)
	}
	allFills, err := store.ListFills()
	if err != nil {
		return nil, err
	}
	fills := make([]data.RealStubFill, 0)
	for _, f := range allFills {
		if f.LocalOrderID == found.ID {
			fills = append(fills, f)
		}
	}
	return &RealStubOrderDetail{Order: *found, Fills: fills}, nil
}
