package data

import (
	"go-stock/backend/db"
	"go-stock/backend/models"
	"strings"
)

// TelegraphExists 判断同来源下是否已有相同或极相近的快讯（避免多通道重复入库/推送）。
func TelegraphExists(t *models.Telegraph) bool {
	if t == nil {
		return false
	}
	src := strings.TrimSpace(t.Source)
	content := strings.TrimSpace(t.Content)
	title := strings.TrimSpace(t.Title)
	var cnt int64

	if content != "" {
		q := db.Dao.Model(&models.Telegraph{})
		if src != "" {
			q = q.Where("source = ?", src)
		}
		q.Where("content = ?", content).Count(&cnt)
		if cnt > 0 {
			return true
		}
	}
	if title != "" {
		cnt = 0
		q := db.Dao.Model(&models.Telegraph{})
		if src != "" {
			q = q.Where("source = ?", src)
		}
		q.Where("title = ?", title).Count(&cnt)
		if cnt > 0 {
			return true
		}
	}
	if content != "" {
		runes := []rune(content)
		if len(runes) >= 30 {
			prefix := string(runes)
			if len(runes) > 80 {
				prefix = string(runes[:80])
			}
			cnt = 0
			q := db.Dao.Model(&models.Telegraph{})
			if src != "" {
				q = q.Where("source = ?", src)
			}
			q.Where("content LIKE ?", prefix+"%").Count(&cnt)
			if cnt > 0 {
				return true
			}
		}
	}
	return false
}
