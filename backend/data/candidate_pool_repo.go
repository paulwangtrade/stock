package data

import (
	"fmt"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

// CandidatePoolRepo 候选池持久化（strategy 不得直接碰 DB）。
type CandidatePoolRepo struct{}

func NewCandidatePoolRepo() *CandidatePoolRepo { return &CandidatePoolRepo{} }

func (r *CandidatePoolRepo) CreatePoolWithItems(pool *models.CandidatePool, items []models.CandidatePoolItem) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if pool == nil {
		return fmt.Errorf("pool is nil")
	}
	now := time.Now()
	if pool.GeneratedAt.IsZero() {
		pool.GeneratedAt = now
	}
	if pool.Status == "" {
		pool.Status = models.CandidatePoolStatusReady
	}
	pool.ItemCount = len(items)
	pool.CreatedAt = now
	pool.UpdatedAt = now

	return db.Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(pool).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].PoolID = pool.ID
			items[i].TradeDate = pool.TradeDate
			items[i].CreatedAt = now
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		pool.Items = items
		return nil
	})
}

func (r *CandidatePoolRepo) GetLatestByTradeDate(tradeDate string) (*models.CandidatePool, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var pool models.CandidatePool
	err := db.Dao.Where("trade_date = ?", tradeDate).Order("id DESC").First(&pool).Error
	if err != nil {
		return nil, err
	}
	var items []models.CandidatePoolItem
	if err := db.Dao.Where("pool_id = ?", pool.ID).Order("rank ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	pool.Items = items
	return &pool, nil
}

func (r *CandidatePoolRepo) GetByID(id uint) (*models.CandidatePool, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var pool models.CandidatePool
	if err := db.Dao.First(&pool, id).Error; err != nil {
		return nil, err
	}
	var items []models.CandidatePoolItem
	if err := db.Dao.Where("pool_id = ?", pool.ID).Order("rank ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	pool.Items = items
	return &pool, nil
}

func (r *CandidatePoolRepo) ListByTradeDate(tradeDate string, limit int) ([]models.CandidatePool, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	if limit <= 0 {
		limit = 20
	}
	var list []models.CandidatePool
	err := db.Dao.Where("trade_date = ?", tradeDate).Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}
