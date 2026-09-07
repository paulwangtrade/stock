package data

import (
	"encoding/json"
	"errors"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/security"
	"strings"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

type Settings struct {
	gorm.Model
	TushareToken           string `json:"tushareToken"`
	LocalPushEnable        bool   `json:"localPushEnable"`
	DingPushEnable         bool   `json:"dingPushEnable"`
	DingRobot              string `json:"dingRobot"`
	UpdateBasicInfoOnStart bool   `json:"updateBasicInfoOnStart"`
	RefreshInterval        int64  `json:"refreshInterval"`
	OpenAiEnable           bool   `json:"openAiEnable"`
	Prompt                 string `json:"prompt"`
	CheckUpdate            bool   `json:"checkUpdate"`
	QuestionTemplate       string `json:"questionTemplate"`
	CrawlTimeOut           int64  `json:"crawlTimeOut"`
	KDays                  int64  `json:"kDays"`
	EnableDanmu            bool   `json:"enableDanmu"`
	BrowserPath            string `json:"browserPath"`
	EnableNews             bool   `json:"enableNews"`
	DarkTheme              bool   `json:"darkTheme"`
	BrowserPoolSize        int    `json:"browserPoolSize"`
	EnableFund             bool   `json:"enableFund"`
	EnablePushNews         bool   `json:"enablePushNews"`
	EnableOnlyPushRedNews  bool   `json:"enableOnlyPushRedNews"`
	SponsorCode            string `json:"sponsorCode"`
	HttpProxy              string `json:"httpProxy"`
	HttpProxyEnabled       bool   `json:"httpProxyEnabled"`
	EnableAgent            bool   `json:"enableAgent"`
	QgqpBId                string `json:"qgqpBId" gorm:"column:qgqp_b_id"`
	// 记录上一次窗口大小（用户拖动调整后保存），为 0 表示未设置，使用自适应默认值
	WindowWidth  int `json:"windowWidth"`
	WindowHeight int `json:"windowHeight"`
	/** JSON：K 线买卖点信号参数，见前端 signalSettings.js */
	SignalParams string `json:"signalParams"`
	/** 研究页候选池：信号分过滤阈值（默认 60） */
	CandidatePoolScoreThreshold float64 `json:"candidate_pool_score_threshold" gorm:"column:candidate_pool_score_threshold;default:60"`
}

func (receiver Settings) TableName() string {
	return "settings"
}

type AIConfig struct {
	ID               uint `gorm:"primarykey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Name             string  `json:"name"`
	BaseUrl          string  `json:"baseUrl"`
	ApiKey           string  `json:"apiKey" `
	ModelName        string  `json:"modelName"`
	MaxTokens        int     `json:"maxTokens"`
	Temperature      float64 `json:"temperature"`
	TimeOut          int     `json:"timeOut"`
	HttpProxy        string  `json:"httpProxy"`
	HttpProxyEnabled bool    `json:"httpProxyEnabled"`
	SessionId        string  `json:"sessionId" gorm:"index;size:64"`
	Thinking         bool    `json:"thinking"`
}

func (AIConfig) TableName() string {
	return "ai_config"
}

type SettingConfig struct {
	*Settings
	AiConfigs []*AIConfig `json:"aiConfigs"`
}

type SettingsApi struct {
	Config *SettingConfig
}

func NewSettingsApi() *SettingsApi {
	return &SettingsApi{
		Config: GetSettingConfig(),
	}
}

func (s *SettingsApi) Export() string {
	// Phase13-V.5: default export is redacted (no API Key / Token plaintext).
	return ExportConfigJSON(s.Config, true)
}

// ExportConfigJSON serializes settings. When redact=true (default export path),
// secrets become security.RedactedPlaceholder without mutating live config.
func ExportConfigJSON(cfg *SettingConfig, redact bool) string {
	if cfg == nil {
		return "{}"
	}
	if !redact {
		d, _ := json.MarshalIndent(cfg, "", "    ")
		return string(d)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	var copy SettingConfig
	if err := json.Unmarshal(raw, &copy); err != nil {
		return "{}"
	}
	redactSettingConfig(&copy)
	d, _ := json.MarshalIndent(&copy, "", "    ")
	return string(d)
}

func redactSettingConfig(cfg *SettingConfig) {
	if cfg == nil {
		return
	}
	if cfg.Settings != nil {
		if strings.TrimSpace(cfg.TushareToken) != "" {
			cfg.TushareToken = security.RedactedPlaceholder
		}
		if strings.TrimSpace(cfg.SponsorCode) != "" {
			cfg.SponsorCode = security.RedactedPlaceholder
		}
		if strings.TrimSpace(cfg.DingRobot) != "" {
			cfg.DingRobot = security.RedactedPlaceholder
		}
	}
	for _, ai := range cfg.AiConfigs {
		if ai == nil {
			continue
		}
		if strings.TrimSpace(ai.ApiKey) != "" {
			ai.ApiKey = security.RedactedPlaceholder
		}
		if strings.TrimSpace(ai.SessionId) != "" {
			ai.SessionId = security.RedactedPlaceholder
		}
	}
}

func UpdateConfig(s *SettingConfig) string {
	count := int64(0)
	db.Dao.Model(&Settings{}).Count(&count)
	if count > 0 {
		updates := map[string]any{
			"local_push_enable":                s.LocalPushEnable,
			"ding_push_enable":                 s.DingPushEnable,
			"update_basic_info_on_start":       s.UpdateBasicInfoOnStart,
			"refresh_interval":                 s.RefreshInterval,
			"open_ai_enable":                   s.OpenAiEnable,
			"prompt":                           s.Prompt,
			"check_update":                     s.CheckUpdate,
			"question_template":                s.QuestionTemplate,
			"crawl_time_out":                   s.CrawlTimeOut,
			"k_days":                           s.KDays,
			"enable_danmu":                     s.EnableDanmu,
			"browser_path":                     s.BrowserPath,
			"enable_news":                      s.EnableNews,
			"dark_theme":                       s.DarkTheme,
			"enable_fund":                      s.EnableFund,
			"enable_push_news":                 s.EnablePushNews,
			"enable_only_push_red_news":        s.EnableOnlyPushRedNews,
			"http_proxy":                       s.HttpProxy,
			"http_proxy_enabled":               s.HttpProxyEnabled,
			"enable_agent":                     s.EnableAgent,
			"qgqp_b_id":                        strings.TrimSpace(s.QgqpBId),
			"window_width":                     s.WindowWidth,
			"window_height":                    s.WindowHeight,
			"signal_params":                    s.SignalParams,
			"candidate_pool_score_threshold":   normalizeCandidatePoolThreshold(s.CandidatePoolScoreThreshold),
		}
		// Phase13-V.5: do not overwrite secrets when import/export placeholder is present.
		if !security.IsRedactedPlaceholder(s.DingRobot) {
			updates["ding_robot"] = s.DingRobot
		}
		if !security.IsRedactedPlaceholder(s.TushareToken) {
			updates["tushare_token"] = s.TushareToken
		}
		if !security.IsRedactedPlaceholder(s.SponsorCode) {
			updates["sponsor_code"] = s.SponsorCode
		}
		db.Dao.Model(&Settings{}).Where("id=?", s.ID).Updates(updates)

		//更新AiConfig
		err := updateAiConfigs(s.AiConfigs)
		if err != nil {
			logger.SugaredLogger.Errorf("更新AI模型服务配置失败: %v", err)
			RefreshSettingCache()
			return "更新AI模型服务配置失败: " + err.Error()
		}
	} else {
		//logger.SugaredLogger.Infof("未找到配置，创建默认配置")
		// 创建主配置
		result := db.Dao.Model(&Settings{}).Create(&Settings{})
		if result.Error != nil {
			logger.SugaredLogger.Error("创建配置失败:", result.Error)
			return "创建配置失败: " + result.Error.Error()
		}
	}
	RefreshSettingCache()
	return "保存成功！"
}

func updateAiConfigs(aiConfigs []*AIConfig) error {
	if len(aiConfigs) == 0 {
		err := db.Dao.Exec("DELETE FROM ai_config").Error
		if err != nil {
			return err
		}
		return db.Dao.Exec("DELETE FROM sqlite_sequence WHERE name='ai_config'").Error
	}
	// 仅收集大于 0 的 ID，用于识别已存在的配置；
	// ID<=0 视为“新配置”，强制走插入逻辑，避免多个 ID 为 0 的配置互相覆盖。
	var ids []uint
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	})
	var existAiConfigs []*AIConfig
	err := db.Dao.Model(&AIConfig{}).Select("id").Where("id in (?) ", ids).Find(&existAiConfigs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	idMap := make(map[uint]bool)
	lo.ForEach(existAiConfigs, func(item *AIConfig, index int) {
		idMap[item.ID] = true
	})
	var addAiConfigs []*AIConfig
	var notDeleteIds []uint
	var e error
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if e != nil {
			return
		}
		// ID<=0 一律视为新配置，走插入逻辑；否则根据是否已存在决定更新或新增
		if item.ID <= 0 || !idMap[item.ID] {
			if security.IsRedactedPlaceholder(item.ApiKey) {
				item.ApiKey = ""
			}
			if security.IsRedactedPlaceholder(item.SessionId) {
				item.SessionId = ""
			}
			addAiConfigs = append(addAiConfigs, item)
		} else {
			notDeleteIds = append(notDeleteIds, item.ID)
			aiUpdates := map[string]interface{}{
				"name":               item.Name,
				"base_url":           item.BaseUrl,
				"model_name":         item.ModelName,
				"max_tokens":         item.MaxTokens,
				"temperature":        item.Temperature,
				"time_out":           item.TimeOut,
				"http_proxy":         item.HttpProxy,
				"http_proxy_enabled": item.HttpProxyEnabled,
			}
			if !security.IsRedactedPlaceholder(item.ApiKey) {
				aiUpdates["api_key"] = item.ApiKey
			}
			if !security.IsRedactedPlaceholder(item.SessionId) {
				aiUpdates["session_id"] = item.SessionId
			}
			e = db.Dao.Model(&AIConfig{}).Where("id=?", item.ID).Updates(aiUpdates).Error
			if e != nil {
				return
			}
		}
	})
	if e != nil {
		return e
	}
	//删除旧的配置
	if len(notDeleteIds) > 0 {
		err = db.Dao.Exec("DELETE FROM ai_config WHERE id NOT IN ?", notDeleteIds).Error
		if err != nil {
			return err
		}
	}
	//logger.SugaredLogger.Infof("更新aiConfigs +%d", len(addAiConfigs))
	//批量新增的配置
	err = db.Dao.CreateInBatches(addAiConfigs, len(addAiConfigs)).Error
	return err
}

const defaultMaxFollowCount = 100
const maxFollowCountHardCap = 500

// GetMaxFollowCount 自选上限，来自 signalParams.display.maxFollowCount，默认 100。
func GetMaxFollowCount() int {
	cfg := GetSettingConfig()
	if cfg == nil || strings.TrimSpace(cfg.SignalParams) == "" {
		return defaultMaxFollowCount
	}
	var parsed struct {
		Display struct {
			MaxFollowCount int `json:"maxFollowCount"`
		} `json:"display"`
	}
	if err := json.Unmarshal([]byte(cfg.SignalParams), &parsed); err != nil {
		return defaultMaxFollowCount
	}
	n := parsed.Display.MaxFollowCount
	if n <= 0 {
		return defaultMaxFollowCount
	}
	if n > maxFollowCountHardCap {
		return maxFollowCountHardCap
	}
	return n
}

func ensureSettingsSchema() {
	_ = db.Dao.AutoMigrate(&Settings{})
}
