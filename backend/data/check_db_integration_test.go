//go:build integration

package data

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestCheckHKName(t *testing.T) {

	db.Init("../../data/stock.db")

	hks := []models.StockInfoHK{}

	db.Dao.
		Model(&models.StockInfoHK{}).
		Limit(10).
		Find(&hks)

	for _, hk := range hks {
		t.Logf(
			"code=%s name=%s",
			hk.Code,
			hk.Name,
		)
	}
}