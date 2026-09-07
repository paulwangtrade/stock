package outcome

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"
)

func loadDistinctFillStockCodes(accountID uint) ([]string, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("outcome: db not initialized")
	}
	if accountID == 0 {
		return []string{}, nil
	}
	var codes []string
	err := db.Dao.Model(&papertrading.PaperSimFill{}).
		Where("account_id = ?", accountID).
		Distinct().
		Order("stock_code asc").
		Pluck("stock_code", &codes).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(codes))
	seen := map[string]bool{}
	for _, raw := range codes {
		code := normalizeCode(raw)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out, nil
}

func loadPoolStockCodes(tradeDate string) ([]string, error) {
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		tradeDate = normalizeTradeDate("")
	}
	pool, err := data.NewCandidatePoolRepo().GetLatestByTradeDate(tradeDate)
	if err != nil || pool == nil {
		return []string{}, nil
	}
	out := make([]string, 0, len(pool.Items))
	seen := map[string]bool{}
	for _, it := range pool.Items {
		code := normalizeCode(it.StockCode)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out, nil
}

func mergeStockCodes(primary, secondary []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(primary)+len(secondary))
	for _, list := range [][]string{primary, secondary} {
		for _, code := range list {
			c := normalizeCode(code)
			if c == "" || seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}
