package papertrading_test

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestDevAccountTools_DisabledByDefault(t *testing.T) {
	off := false
	papertrading.SetDevAccountToolsForTest(&off)
	t.Cleanup(func() { papertrading.SetDevAccountToolsForTest(nil) })

	_, err := papertrading.SetPaperSimCash(500_000)
	require.Error(t, err)
	require.Contains(t, err.Error(), papertrading.DevAccountEnv)

	_, err = papertrading.ResetPaperSimAccount(500_000)
	require.Error(t, err)
}

func TestSetPaperSimCash_OnlyPaperSimAccounts(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	on := true
	papertrading.SetDevAccountToolsForTest(&on)
	t.Cleanup(func() { papertrading.SetDevAccountToolsForTest(nil) })

	require.NoError(t, papertrading.EnsureSchema(nil))

	// Seed a legacy paper_accounts row — must remain untouched.
	type legacyAcc struct {
		ID   uint `gorm:"primaryKey"`
		Name string
		Cash float64
	}
	require.NoError(t, db.Dao.Table("paper_accounts").AutoMigrate(&legacyAcc{}))
	require.NoError(t, db.Dao.Table("paper_accounts").Create(&legacyAcc{Name: "默认模拟账户", Cash: 42}).Error)

	res, err := papertrading.SetPaperSimCash(250_000)
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Equal(t, 250_000.0, res.Cash)
	require.Equal(t, "paper_sim_default", res.Name)

	var sim papertrading.PaperSimAccount
	require.NoError(t, db.Dao.Where("name = ?", "paper_sim_default").First(&sim).Error)
	require.Equal(t, 250_000.0, sim.Cash)

	var leg legacyAcc
	require.NoError(t, db.Dao.Table("paper_accounts").Where("name = ?", "默认模拟账户").First(&leg).Error)
	require.Equal(t, 42.0, leg.Cash, "legacy paper_accounts must be unchanged")
}

func TestResetPaperSimAccount_SetsInitialAndCash(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	on := true
	papertrading.SetDevAccountToolsForTest(&on)
	t.Cleanup(func() { papertrading.SetDevAccountToolsForTest(nil) })
	require.NoError(t, papertrading.EnsureSchema(nil))

	_, err := papertrading.SetPaperSimCash(10_000)
	require.NoError(t, err)

	res, err := papertrading.ResetPaperSimAccount(2_000_000)
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Equal(t, 2_000_000.0, res.Cash)
	require.Equal(t, 2_000_000.0, res.InitialCash)

	acc, err := papertrading.GetPaperSimDefaultAccount()
	require.NoError(t, err)
	require.Equal(t, 2_000_000.0, acc.Cash)
	require.Equal(t, 2_000_000.0, acc.InitialCash)
	require.InDelta(t, acc.Cash+acc.MarketValue, acc.Equity, 0.01)
}
