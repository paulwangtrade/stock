package provenance

import "go-stock/backend/models"

// SignalSnapshotSource reads close signal snapshots (no scan side effects).
type SignalSnapshotSource interface {
	GetLatestCloseSignalSnapshot(asOfDate string) (*models.SignalScanSnapshot, error)
	GetCloseSnapshotByTradeDate(tradeDate string) (*models.SignalScanSnapshot, error)
	GetSnapshotByID(id uint) (*models.SignalScanSnapshot, error)
	GetUniverseSnapshotByRun(tradeDate, session, strategyKey, universeID string) (*models.SignalScanSnapshot, error)
	GetLatestCloseSnapshotByStrategy(asOfDate, session, strategyKey, scope string) (*models.SignalScanSnapshot, error)
	ParseSnapshotHits(snap *models.SignalScanSnapshot) []models.SignalScanHit
}

// ItemProvenanceUpdater persists signal provenance on pool items (score/rank untouched).
type ItemProvenanceUpdater interface {
	UpdateItemProvenance(itemID uint, snapshotID uint, tag string) (updated bool, err error)
}
