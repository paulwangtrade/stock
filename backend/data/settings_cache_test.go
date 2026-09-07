package data

import (
	"fmt"
	"sync"
	"testing"

	"go-stock/backend/db"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSettingsCacheTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:settings_cache_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, testDB.AutoMigrate(&Settings{}, &AIConfig{}))
	db.Dao = testDB
	ResetSettingCacheForTest()
	t.Cleanup(func() {
		ResetSettingCacheForTest()
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestGetSettingConfig_UsesCache(t *testing.T) {
	setupSettingsCacheTestDB(t)
	require.NoError(t, db.Dao.Create(&Settings{
		RefreshInterval: 30,
		DarkTheme:       true,
		BrowserPath:     "C:\\fake\\chrome.exe",
		BrowserPoolSize: 2,
	}).Error)

	cfg1 := GetSettingConfig()
	require.NotNil(t, cfg1)
	require.NotNil(t, cfg1.Settings)
	require.Equal(t, int64(30), cfg1.RefreshInterval)
	require.True(t, cfg1.DarkTheme)

	// 直接改库，不刷新缓存时读到的仍是旧值
	require.NoError(t, db.Dao.Model(&Settings{}).Where("id = ?", cfg1.ID).Update("refresh_interval", 99).Error)
	cfg2 := GetSettingConfig()
	require.Equal(t, cfg1, cfg2) // 同一缓存指针
	require.Equal(t, int64(30), cfg2.RefreshInterval)

	RefreshSettingCache()
	cfg3 := GetSettingConfig()
	require.Equal(t, int64(99), cfg3.RefreshInterval)
}

func TestUpdateConfig_RefreshesCache(t *testing.T) {
	setupSettingsCacheTestDB(t)
	s := &Settings{
		RefreshInterval: 10,
		BrowserPath:     "C:\\fake\\chrome.exe",
		BrowserPoolSize: 1,
	}
	require.NoError(t, db.Dao.Create(s).Error)

	cfg := GetSettingConfig()
	require.Equal(t, int64(10), cfg.RefreshInterval)

	cfg.RefreshInterval = 42
	cfg.DarkTheme = true
	msg := UpdateConfig(cfg)
	require.Contains(t, msg, "成功")

	fresh := GetSettingConfig()
	require.Equal(t, int64(42), fresh.RefreshInterval)
	require.True(t, fresh.DarkTheme)
}

func TestInvalidateSettingCache_ForcesReload(t *testing.T) {
	setupSettingsCacheTestDB(t)
	require.NoError(t, db.Dao.Create(&Settings{
		RefreshInterval: 1,
		BrowserPath:     "C:\\fake\\chrome.exe",
		BrowserPoolSize: 1,
	}).Error)

	_ = GetSettingConfig()
	require.NoError(t, db.Dao.Model(&Settings{}).Where("1=1").Update("refresh_interval", 7).Error)

	InvalidateSettingCache()
	cfg := GetSettingConfig()
	require.Equal(t, int64(7), cfg.RefreshInterval)
}

func TestGetSettingConfig_Concurrent(t *testing.T) {
	setupSettingsCacheTestDB(t)
	require.NoError(t, db.Dao.Create(&Settings{
		RefreshInterval: 5,
		BrowserPath:     "C:\\fake\\chrome.exe",
		BrowserPoolSize: 1,
	}).Error)

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := GetSettingConfig()
			require.NotNil(t, cfg)
			require.Equal(t, int64(5), cfg.RefreshInterval)
		}()
	}
	wg.Wait()
}

func TestGetSettingConfig_LoadsAiConfigs(t *testing.T) {
	setupSettingsCacheTestDB(t)
	require.NoError(t, db.Dao.Create(&Settings{
		OpenAiEnable:    true,
		CrawlTimeOut:    60,
		KDays:           60,
		BrowserPath:     "C:\\fake\\chrome.exe",
		BrowserPoolSize: 1,
	}).Error)
	require.NoError(t, db.Dao.Create(&AIConfig{
		Name:      "test",
		BaseUrl:   "http://localhost",
		ApiKey:    "k",
		ModelName: "m",
		TimeOut:   0, // 应被默认成 300
	}).Error)

	cfg := GetSettingConfig()
	require.Len(t, cfg.AiConfigs, 1)
	require.Equal(t, 60*5, cfg.AiConfigs[0].TimeOut)
}
