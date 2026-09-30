package main

import (
	"go-stock/backend/data"
	"go-stock/backend/signalattribution"
)

// GetSignalScanAttribution is a research-only table of snapshot hits versus
// later local daily closes. It does not create trade plans or broker orders.
func (a *App) GetSignalScanAttribution(query *signalattribution.Query) *signalattribution.View {
	return data.BuildSignalScanAttribution(query)
}
