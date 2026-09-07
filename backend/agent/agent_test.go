package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/fileutil"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func initAgentTestDB(t *testing.T) {
	t.Helper()
	path := filepath.Join("..", "..", "data", "stock.db")
	if _, err := os.Stat(path); err != nil {
		t.Skip("data/stock.db missing:", err)
	}
	testDB, err := gorm.Open(sqlite.Open(path+"?_busy_timeout=5000"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Skip("data/stock.db unusable:", err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Skip("data/stock.db open:", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Skip("data/stock.db ping:", err)
	}
	orig := db.Dao
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = orig
		_ = sqlDB.Close()
	})
}

func TestGetStockAiAgent(t *testing.T) {

	ctx := context.Background()

	initAgentTestDB(t)

	config := data.GetSettingConfig()

	// 防止配置为空导致测试崩溃
	if config == nil {
		t.Fatal("获取系统配置失败")
	}

	// 没有AI模型配置时跳过
	if len(config.AiConfigs) == 0 {
		t.Skip("没有配置AI模型，跳过AI Agent测试")
	}

	aiAgent := GetStockAiAgent(&ctx, *config.AiConfigs[0])

	opt := []agent.AgentOption{
		//agent.WithComposeOptions(compose.WithCallbacks(&tool_logger.LoggerCallback{})),
		//react.WithChatModelOptions(ark.WithCache(cacheOption)),
	}

	sr, err := aiAgent.Stream(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: config.Settings.Prompt + "",
		},
		{
			Role:    schema.User,
			Content: "结合以上提供的宏观经济数据/市场指数行情/国内外市场资讯/电报/会议/事件/投资者关注的问题，\n结合宏观经济，事件驱动，政策支持，投资者关注的问题，分析当前市场情绪和热点 找出有潜力/优质的板块/行业/概念/标的/主题，\n多因子深度分析计算上涨或下跌的逻辑和概率，\n最后按风险和投资周期给出具体推荐标的操作建议",
		},
	}, opt...)

	if err != nil {
		logger.SugaredLogger.Errorf("stream error: %v", err)
		return
	}

	defer sr.Close()

	md := strings.Builder{}

	for {
		msg, err := sr.Recv()

		if err != nil {

			if errors.Is(err, io.EOF) {
				break
			}

			logger.SugaredLogger.Errorf("failed to recv: %v", err)
			return
		}

		logger.SugaredLogger.Infof("stream recv: %v", msg)

		if msg.ReasoningContent != "" {
			md.WriteString(msg.ReasoningContent)
		}

		if msg.Content != "" {
			md.WriteString(msg.Content)
		}
	}

	logger.SugaredLogger.Info(md.String())
}


func TestAgent(t *testing.T) {

	initAgentTestDB(t)

	config := data.GetSettingConfig()
	if config == nil || len(config.AiConfigs) == 0 {
		t.Skip("没有配置AI模型，跳过AI Agent测试")
	}

	md := strings.Builder{}

	ch := NewStockAiAgentApi().Chat("分析一下立讯精密", 2, nil)

	for message := range ch {
		logger.SugaredLogger.Infof("res:%s", message.String())
		md.WriteString(message.String())
	}

	logger.SugaredLogger.Info(md.String())

	fileutil.WriteStringToFile(
		"../../data/result.md",
		md.String(),
		false,
	)
}