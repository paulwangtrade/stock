package portfolio

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"gorm.io/gorm"
)

// SnapshotOptions selects which paper_sim account to read. Zero value → paper_sim_default.
type SnapshotOptions struct {
	AccountID   uint
	AccountName string
	AsOf        time.Time
}

// Service loads a read-only Portfolio Snapshot. Implementations must not write
// paper_sim_* / TradePlan / orders, and must not create missing accounts.
type Service interface {
	Snapshot(opts SnapshotOptions) (*Snapshot, error)
}

type dbService struct{}

// NewService returns the default DB-backed reader.
func NewService() Service {
	return &dbService{}
}

// Snapshot reads paper_sim_accounts + paper_sim_positions. Missing account → Found=false, no INSERT.
func (s *dbService) Snapshot(opts SnapshotOptions) (*Snapshot, error) {
	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	name := strings.TrimSpace(opts.AccountName)
	if name == "" {
		name = defaultAccountName
	}

	if db.Dao == nil {
		return emptySnapshot(asOf, name), fmt.Errorf("portfolio: db not initialized")
	}

	acc, err := loadAccount(opts.AccountID, name)
	if err != nil {
		return emptySnapshot(asOf, name), err
	}
	if acc == nil {
		return emptySnapshot(asOf, name), nil
	}

	positions, err := papertrading.GetPositions(acc.ID)
	if err != nil {
		return emptySnapshot(asOf, acc.Name), err
	}
	return ProjectSnapshot(asOf, acc, positions), nil
}

func loadAccount(id uint, name string) (*papertrading.PaperSimAccount, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("portfolio: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&papertrading.PaperSimAccount{}) {
		return nil, nil
	}
	var acc papertrading.PaperSimAccount
	q := db.Dao
	if id > 0 {
		q = q.Where("id = ?", id)
	} else {
		q = q.Where("name = ?", name)
	}
	err := q.First(&acc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &acc, nil
}
