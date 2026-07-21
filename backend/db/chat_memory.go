package db

import (
	"go-stock/backend/models"
	"time"
)

type ChatMemory struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	SessionID string    `gorm:"index;size:64" json:"sessionId"`
	Role      string    `gorm:"size:20" json:"role"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ChatMemory) TableName() string {
	return "chat_memory"
}

func (c *ChatMemory) Save() error {
	return Dao.Create(c).Error
}

func GetChatMemoryList(sessionID string, limit int) ([]ChatMemory, error) {
	var memories []ChatMemory
	err := Dao.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}
	return memories, nil
}

func GetRecentChatMemory(sessionID string, limit int) ([]ChatMemory, error) {
	var memories []ChatMemory
	var err error
	if sessionID == "" {
		err = Dao.Order("created_at DESC").
			Limit(limit).
			Find(&memories).Error
	} else {
		err = Dao.Where("session_id = ?", sessionID).
			Order("created_at DESC").
			Limit(limit).
			Find(&memories).Error
	}
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}
	return memories, err
}

func ClearChatMemory(sessionID string) error {
	return Dao.Where("session_id = ?", sessionID).Delete(&ChatMemory{}).Error
}

// AutoMigrate 迁移 chat_memory / stock_change_history（兼容显式调用；主路径由 main 版本门控统一迁移）。
func AutoMigrate() {
	if Dao == nil {
		return
	}
	_ = Dao.AutoMigrate(&ChatMemory{}, &models.StockChangeHistory{})
}
