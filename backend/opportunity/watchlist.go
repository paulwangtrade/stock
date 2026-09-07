package opportunity

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
)

// WatchlistItem is a read-only row for GET /api/watchlist (current WATCHING only).
type WatchlistItem struct {
	OpportunityID    string    `json:"opportunity_id,omitempty"`
	StockCode        string    `json:"stock_code"`
	StockName        string    `json:"stock_name,omitempty"`
	ScanBatchKey     string    `json:"scan_batch_key"`
	Source           string    `json:"source,omitempty"`
	WatchTime        time.Time `json:"watch_time"`
	LatestAction     string    `json:"latest_action"`
	InTradePlan      bool      `json:"in_trade_plan"`
	TradePlanStatus  string    `json:"trade_plan_status"`
	// TradePlanID is the representative plan id for this stock_code (0 when none).
	TradePlanID uint `json:"trade_plan_id,omitempty"`
}

// WatchlistView is GET /api/watchlist response body.
type WatchlistView struct {
	AccountID      uint            `json:"account_id"`
	WatchingCount  int             `json:"watching_count"`
	Items          []WatchlistItem `json:"items"`
	DataSourceNote string          `json:"data_source_note,omitempty"`
}

// ListWatchingQuery parameters for account-level watching list.
type ListWatchingQuery struct {
	AccountID uint
	Limit     int
}

// ListWatching returns opportunities whose latest user action is WATCH (append-only latest wins).
// IGNORE (and any non-WATCH latest) are excluded. No schema changes.
func ListWatching(q ListWatchingQuery) (*WatchlistView, error) {
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}
	accountID := q.AccountID
	if accountID == 0 {
		acc, err := papertrading.GetDefaultAccount()
		if err != nil {
			return nil, err
		}
		accountID = acc.ID
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var rows []UserOpportunityAction
	err := db.Dao.Where("account_id = ?", accountID).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	items := make([]WatchlistItem, 0)
	for i := range rows {
		oid := strings.TrimSpace(rows[i].OpportunityID)
		if oid == "" {
			continue
		}
		if _, ok := seen[oid]; ok {
			continue
		}
		seen[oid] = struct{}{}
		if rows[i].Action != ActionWatch {
			continue
		}
		name, source := enrichWatchlistMeta(rows[i].ScanBatchKey, rows[i].StockCode)
		items = append(items, WatchlistItem{
			OpportunityID:   oid,
			StockCode:       rows[i].StockCode,
			StockName:       name,
			ScanBatchKey:    rows[i].ScanBatchKey,
			Source:          source,
			WatchTime:       rows[i].CreatedAt,
			LatestAction:    ActionWatch,
			InTradePlan:     false,
			TradePlanStatus: WatchlistTradePlanNone,
		})
	}

	watchingCount := len(items)
	if len(items) > limit {
		items = items[:limit]
	}
	enrichWatchlistTradePlan(items)

	return &WatchlistView{
		AccountID:      accountID,
		WatchingCount:  watchingCount,
		Items:          items,
		DataSourceNote: "user_opportunity_actions · latest per opportunity_id · WATCH only · trade_plan by stock_code",
	}, nil
}

// enrichWatchlistMeta derives display name/source from existing snapshot data (no new columns).
func enrichWatchlistMeta(scanBatchKey, stockCode string) (stockName, source string) {
	batch := strings.TrimSpace(scanBatchKey)
	code := NormalizeStockCode(stockCode)
	source = formatBatchSource(batch)

	if strings.HasPrefix(batch, "snap:") {
		idRaw := strings.TrimPrefix(batch, "snap:")
		id, err := strconv.ParseUint(idRaw, 10, 64)
		if err != nil || id == 0 {
			return "", source
		}
		var snap models.SignalScanSnapshot
		if err := db.Dao.First(&snap, uint(id)).Error; err != nil {
			return "", source
		}
		source = formatSnapshotSource(&snap)
		repo := data.NewSignalSnapshotRepo()
		for _, hit := range repo.ParseSnapshotHits(&snap) {
			if NormalizeStockCode(SecucodeToStockCode(hit.SECUCODE)) == code {
				return strings.TrimSpace(hit.SECURITY_NAME_ABBR), source
			}
		}
		return "", source
	}

	parts := strings.Split(batch, "|")
	if len(parts) == 3 {
		source = formatBatchSource(batch)
	}
	return "", source
}

func formatBatchSource(batch string) string {
	batch = strings.TrimSpace(batch)
	if batch == "" {
		return ""
	}
	if strings.HasPrefix(batch, "snap:") {
		return batch
	}
	parts := strings.Split(batch, "|")
	if len(parts) == 3 {
		return strings.TrimSpace(parts[0]) + " · " + strings.TrimSpace(parts[1]) + " · " + strings.TrimSpace(parts[2])
	}
	return batch
}

func formatSnapshotSource(snap *models.SignalScanSnapshot) string {
	if snap == nil {
		return ""
	}
	parts := []string{}
	if s := strings.TrimSpace(snap.TradeDate); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(snap.Session); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(snap.StrategyID); s != "" {
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("snap:%d", snap.ID)
	}
	return strings.Join(parts, " · ")
}
