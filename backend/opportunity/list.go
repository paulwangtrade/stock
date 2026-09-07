package opportunity

import (
	"errors"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"gorm.io/gorm"
)

// PoolEntry is a read-only opportunity projection from a signal scan snapshot hit.
type PoolEntry struct {
	OpportunityID    string                         `json:"opportunity_id"`
	ScanBatchKey     string                         `json:"scan_batch_key"`
	StockCode        string                         `json:"stock_code"`
	Secucode         string                         `json:"secucode"`
	StockName        string                         `json:"stock_name,omitempty"`
	SignalTag        string                         `json:"signal_tag,omitempty"`
	SignalTime       string                         `json:"signal_time,omitempty"`
	SignalPrice      float64                        `json:"signal_price,omitempty"`
	SignalPriceStatus string                        `json:"signal_price_status,omitempty"`
	LatestUserAction *UserOpportunityActionSummary  `json:"latest_user_action,omitempty"`
}

// PoolListView is GET /api/opportunities/list response body.
type PoolListView struct {
	AccountID    uint                    `json:"account_id"`
	SnapshotID   uint                    `json:"snapshot_id,omitempty"`
	TradeDate    string                  `json:"trade_date,omitempty"`
	Session      string                  `json:"session,omitempty"`
	StrategyID   string                  `json:"strategy_id,omitempty"`
	ScanBatchKey string                  `json:"scan_batch_key"`
	Entries      []PoolEntry             `json:"entries"`
	DataSourceNote string                `json:"data_source_note,omitempty"`
}

// ListQuery parameters for opportunity pool read model.
type ListQuery struct {
	TradeDate          string
	Session            string
	StrategyID         string
	AccountID          uint
	IncludeUserAction  bool
}

// ListFromSnapshot builds opportunity entries from an existing signal scan snapshot (read-only).
func ListFromSnapshot(q ListQuery) (*PoolListView, error) {
	api := data.NewSignalScanApi()
	var snap *models.SignalScanSnapshot
	var err error

	tradeDate := strings.TrimSpace(q.TradeDate)
	session := strings.TrimSpace(q.Session)
	strategyID := strings.TrimSpace(q.StrategyID)

	if tradeDate != "" {
		if strategyID != "" {
			snap, err = api.GetLatestSnapshotByStrategy(tradeDate, session, strategyID)
		} else {
			snap, err = api.GetLatestSnapshot(tradeDate, session)
		}
	} else {
		// Latest done snapshot for session/strategy when trade_date omitted.
		if strategyID != "" {
			snap, err = api.GetLatestSnapshotByStrategy("", session, strategyID)
		} else {
			snap, err = api.GetLatestSnapshot("", session)
		}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &PoolListView{
				ScanBatchKey:   BatchKeyFromSnapshot(0, tradeDate, session, strategyID),
				Entries:        []PoolEntry{},
				DataSourceNote: "signal_scan_snapshot · no snapshot found",
			}, nil
		}
		return nil, err
	}
	if snap == nil || snap.ID == 0 {
		return &PoolListView{
			ScanBatchKey:   BatchKeyFromSnapshot(0, tradeDate, session, strategyID),
			Entries:        []PoolEntry{},
			DataSourceNote: "signal_scan_snapshot · no snapshot found",
		}, nil
	}

	repo := data.NewSignalSnapshotRepo()
	hits := repo.ParseSnapshotHits(snap)
	batchKey := BatchKeyFromSnapshot(snap.ID, snap.TradeDate, snap.Session, snap.StrategyID)

	accountID := q.AccountID
	if accountID == 0 {
		acc, accErr := papertrading.GetDefaultAccount()
		if accErr != nil {
			return nil, accErr
		}
		accountID = acc.ID
	}

	var latest map[string]*UserOpportunityActionSummary
	if q.IncludeUserAction {
		latest, err = LoadLatestActionsByBatch(accountID, batchKey)
		if err != nil {
			return nil, err
		}
	}

	entries := make([]PoolEntry, 0, len(hits))
	for _, hit := range hits {
		if strings.TrimSpace(hit.SECUCODE) == "" {
			continue
		}
		tag := strings.TrimSpace(hit.Tag)
		signalTime := strings.TrimSpace(hit.SignalTime)
		oid := BuildOpportunityID(batchKey, hit.SECUCODE, signalTime, tag)
		entry := PoolEntry{
			OpportunityID:     oid,
			ScanBatchKey:      batchKey,
			StockCode:         SecucodeToStockCode(hit.SECUCODE),
			Secucode:          hit.SECUCODE,
			StockName:         hit.SECURITY_NAME_ABBR,
			SignalTag:         tag,
			SignalTime:        signalTime,
			SignalPrice:       hit.SignalPrice,
			SignalPriceStatus: hit.SignalPriceStatus,
		}
		if latest != nil {
			if ua, ok := latest[oid]; ok {
				entry.LatestUserAction = ua
			}
		}
		entries = append(entries, entry)
	}

	return &PoolListView{
		AccountID:      accountID,
		SnapshotID:     snap.ID,
		TradeDate:      snap.TradeDate,
		Session:        snap.Session,
		StrategyID:     snap.StrategyID,
		ScanBatchKey:   batchKey,
		Entries:        entries,
		DataSourceNote: "signal_scan_snapshot · read-only · user actions optional",
	}, nil
}
