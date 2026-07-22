package data

import (
	"sync"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"strings"

	"github.com/samber/lo"
)

// SettingCache 进程内 Settings 配置缓存，避免重复读库。
type SettingCache struct {
	sync.RWMutex
	config *SettingConfig
	loaded bool
}

var settingCache = &SettingCache{}

// GetSettingConfig 优先返回进程内缓存；未加载时从 DB 读取并缓存。
func GetSettingConfig() *SettingConfig {
	settingCache.RLock()
	if settingCache.loaded && settingCache.config != nil {
		cfg := settingCache.config
		settingCache.RUnlock()
		return cfg
	}
	settingCache.RUnlock()

	settingCache.Lock()
	defer settingCache.Unlock()
	if settingCache.loaded && settingCache.config != nil {
		return settingCache.config
	}
	settingCache.config = loadSettingConfigFromDB()
	settingCache.loaded = true
	return settingCache.config
}

// RefreshSettingCache 从数据库重新加载配置并更新缓存（设置变更后调用）。
func RefreshSettingCache() {
	settingCache.Lock()
	defer settingCache.Unlock()
	settingCache.config = loadSettingConfigFromDB()
	settingCache.loaded = true
}

// InvalidateSettingCache 清空缓存，下次 GetSettingConfig 将重新读库。
func InvalidateSettingCache() {
	settingCache.Lock()
	defer settingCache.Unlock()
	settingCache.config = nil
	settingCache.loaded = false
}

// ResetSettingCacheForTest 仅测试用。
func ResetSettingCacheForTest() {
	InvalidateSettingCache()
}

func loadSettingConfigFromDB() *SettingConfig {
	ensureSettingsSchema()
	settingConfig := &SettingConfig{}
	settings := &Settings{}
	aiConfigs := make([]*AIConfig, 0)
	_ = db.Dao.Model(&Settings{}).First(settings)
	if settings.OpenAiEnable {
		result := db.Dao.Model(&AIConfig{}).Find(&aiConfigs)
		if result.Error != nil {
			logger.SugaredLogger.Error("查询AI配置失败:", result.Error)
		} else if len(aiConfigs) > 0 {
			lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
				if item.TimeOut <= 0 {
					item.TimeOut = 60 * 5
				}
			})
		}
		if settings.CrawlTimeOut <= 0 {
			settings.CrawlTimeOut = 60
		}
		if settings.KDays < 30 {
			settings.KDays = 60
		}
	}
	if settings.BrowserPath == "" {
		settings.BrowserPath, _ = CheckBrowser()
	}
	if settings.BrowserPoolSize <= 0 {
		settings.BrowserPoolSize = 1
	}
	settings.EnableFund = false
	settings.EnableAgent = false
	settings.QgqpBId = strings.TrimSpace(settings.QgqpBId)
	settings.CandidatePoolScoreThreshold = normalizeCandidatePoolThreshold(settings.CandidatePoolScoreThreshold)

	settingConfig.Settings = settings
	settingConfig.AiConfigs = aiConfigs
	return settingConfig
}

// GetCandidatePoolScoreThreshold 读取配置项 candidate_pool_score_threshold（默认 60）。
func GetCandidatePoolScoreThreshold() float64 {
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		return DefaultResearchCandidateMinScore
	}
	return normalizeCandidatePoolThreshold(cfg.CandidatePoolScoreThreshold)
}

func normalizeCandidatePoolThreshold(v float64) float64 {
	if v <= 0 {
		return DefaultResearchCandidateMinScore
	}
	return v
}
