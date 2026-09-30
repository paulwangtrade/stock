// Package externalmirror stores manual real-broker mirror holdings for observation.
//
// Rows live only in external_mirror_holdings with source=external_mirror.
// They are not paper_sim inventory, not TradePlan input, and not Track-A orders.
package externalmirror

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/stockname"

	"gorm.io/gorm"
)

const (
	// Source is the only persisted book for manual real-broker observation.
	Source = "external_mirror"

	// Disclaimer is the fixed copy shown on the mirror page.
	Disclaimer = "仅观察 · 不进模拟账本 · 不进交易计划 · 不下单"

	tableName = "external_mirror_holdings"

	maxNameRunes = 64
	maxNoteRunes = 500
)

// Holding is one observation row. Unique on stock_code (one manual lot per code).
type Holding struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Source    string    `gorm:"size:32;not null;index;default:external_mirror" json:"source"`
	StockCode string    `gorm:"size:32;not null;uniqueIndex:uidx_external_mirror_code" json:"stockCode"`
	StockName string    `gorm:"size:64" json:"stockName"`
	Quantity  int64     `gorm:"not null" json:"quantity"`
	CostPrice float64   `gorm:"not null" json:"costPrice"`
	EntryDate string    `gorm:"size:10;not null" json:"entryDate"`
	Note      string    `gorm:"size:500" json:"note"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Holding) TableName() string { return tableName }

// View is the API row. FeedsTradePlan and Tradable are always false.
type View struct {
	ID             uint    `json:"id"`
	Source         string  `json:"source"`
	StockCode      string  `json:"stockCode"`
	StockName      string  `json:"stockName"`
	Quantity       int64   `json:"quantity"`
	CostPrice      float64 `json:"costPrice"`
	EntryDate      string  `json:"entryDate"`
	Note           string  `json:"note"`
	FeedsTradePlan bool    `json:"feedsTradePlan"`
	Tradable       bool    `json:"tradable"`
	UpdatedAt      string  `json:"updatedAt"`
}

// Input is a create/update payload. Empty Source is forced to external_mirror.
// A non-empty other source, or FeedsTradePlan/Tradable true, is rejected.
type Input struct {
	StockCode      string
	StockName      string
	Quantity       *float64
	CostPrice      *float64
	EntryDate      string
	Note           string
	Source         string
	FeedsTradePlan *bool
	Tradable       *bool
	// NameSet distinguishes "omit name" (keep on update) from an explicit empty name.
	NameSet bool
	NoteSet bool
}

// ValidationError is a fail-closed field rejection. Nothing is written.
type ValidationError struct {
	Reason  string
	Message string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Reason
}

// EnsureSchema creates external_mirror_holdings only. It does not touch paper_sim_* or trade_plans.
func EnsureSchema(gdb *gorm.DB) error {
	if gdb == nil {
		gdb = db.Dao
	}
	if gdb == nil {
		return fmt.Errorf("externalmirror: db not initialized")
	}
	if err := gdb.AutoMigrate(&Holding{}); err != nil {
		return fmt.Errorf("externalmirror schema: %w", err)
	}
	return nil
}

var nowFn = time.Now

// SetNowForTest overrides the clock used for the default entry date. Pass nil to restore.
func SetNowForTest(fn func() time.Time) {
	if fn == nil {
		nowFn = time.Now
		return
	}
	nowFn = fn
}

var resolveName = func(code string) string {
	r := stockname.Resolve(code)
	n := strings.TrimSpace(r.Name)
	if n == "" || n == stockname.UnknownName {
		return ""
	}
	return n
}

// SetNameResolverForTest injects name lookup. Pass nil to restore stockname.Resolve.
func SetNameResolverForTest(fn func(code string) string) {
	if fn == nil {
		resolveName = func(code string) string {
			r := stockname.Resolve(code)
			n := strings.TrimSpace(r.Name)
			if n == "" || n == stockname.UnknownName {
				return ""
			}
			return n
		}
		return
	}
	resolveName = fn
}

func today() string {
	return nowFn().Format("2006-01-02")
}

func reject(reason, message string) error {
	return &ValidationError{Reason: reason, Message: message}
}

func asValidation(err error) *ValidationError {
	var v *ValidationError
	if errors.As(err, &v) {
		return v
	}
	return nil
}

// normalizeSource fail-closes any book other than external_mirror.
func normalizeSource(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return Source, nil
	}
	if s != Source {
		return "", reject("invalid_source", "来源必须为 external_mirror（实盘镜像观察），不能写入模拟账本")
	}
	return Source, nil
}

func rejectTradeFlags(in Input) error {
	if in.FeedsTradePlan != nil && *in.FeedsTradePlan {
		return reject("observation_only", "镜像持仓仅观察，不进交易计划")
	}
	if in.Tradable != nil && *in.Tradable {
		return reject("observation_only", "镜像持仓仅观察，不能作为可交易持仓")
	}
	return nil
}

func normalizeCode(raw string) (string, error) {
	n, err := data.NormalizeStockCode(raw)
	if err != nil || strings.TrimSpace(n.SinaCode) == "" {
		return "", reject("invalid_code", "代码无效，请填写证券代码")
	}
	return strings.ToLower(strings.TrimSpace(n.SinaCode)), nil
}

func normalizeQty(q *float64) (int64, error) {
	if q == nil || math.IsNaN(*q) || math.IsInf(*q, 0) {
		return 0, reject("invalid_quantity", "数量必填且必须大于 0")
	}
	if *q <= 0 || *q != math.Trunc(*q) {
		return 0, reject("invalid_quantity", "数量必填且必须为大于 0 的整数")
	}
	if *q > float64(math.MaxInt64) {
		return 0, reject("invalid_quantity", "数量过大")
	}
	return int64(*q), nil
}

func normalizeCost(c *float64) (float64, error) {
	if c == nil || math.IsNaN(*c) || math.IsInf(*c, 0) || *c <= 0 {
		return 0, reject("invalid_cost", "成本价必填且必须大于 0")
	}
	return *c, nil
}

func normalizeEntryDate(raw string, emptyMeansToday bool) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		if emptyMeansToday {
			return today(), nil
		}
		return "", nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil || t.Format("2006-01-02") != s {
		return "", reject("invalid_entry_date", "录入日格式应为 YYYY-MM-DD")
	}
	return s, nil
}

func normalizeOptionalText(raw string, maxRunes int, reason, message string) (string, error) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > maxRunes {
		return "", reject(reason, message)
	}
	return s, nil
}

func toView(h Holding) View {
	return View{
		ID:             h.ID,
		Source:         Source,
		StockCode:      h.StockCode,
		StockName:      h.StockName,
		Quantity:       h.Quantity,
		CostPrice:      h.CostPrice,
		EntryDate:      h.EntryDate,
		Note:           h.Note,
		FeedsTradePlan: false,
		Tradable:       false,
		UpdatedAt:      h.UpdatedAt.Format(time.RFC3339),
	}
}

func gdbOrErr() (*gorm.DB, error) {
	if err := EnsureSchema(nil); err != nil {
		return nil, err
	}
	return db.Dao, nil
}

// List returns external_mirror rows only.
func List() ([]View, error) {
	gdb, err := gdbOrErr()
	if err != nil {
		return nil, err
	}
	var rows []Holding
	if err := gdb.Where("source = ?", Source).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, row := range rows {
		if row.Source != Source {
			continue
		}
		out = append(out, toView(row))
	}
	return out, nil
}

// Create inserts one observation holding. It never writes paper_sim_* or trade plans.
func Create(in Input) (View, error) {
	gdb, err := gdbOrErr()
	if err != nil {
		return View{}, err
	}
	if err := rejectTradeFlags(in); err != nil {
		return View{}, err
	}
	if _, err := normalizeSource(in.Source); err != nil {
		return View{}, err
	}
	code, err := normalizeCode(in.StockCode)
	if err != nil {
		return View{}, err
	}
	qty, err := normalizeQty(in.Quantity)
	if err != nil {
		return View{}, err
	}
	cost, err := normalizeCost(in.CostPrice)
	if err != nil {
		return View{}, err
	}
	entry, err := normalizeEntryDate(in.EntryDate, true)
	if err != nil {
		return View{}, err
	}
	name, err := normalizeOptionalText(in.StockName, maxNameRunes, "invalid_name", "名称过长")
	if err != nil {
		return View{}, err
	}
	if name == "" {
		name = resolveName(code)
		name, err = normalizeOptionalText(name, maxNameRunes, "invalid_name", "名称过长")
		if err != nil {
			name = ""
		}
	}
	note, err := normalizeOptionalText(in.Note, maxNoteRunes, "invalid_note", "备注过长")
	if err != nil {
		return View{}, err
	}

	var existing Holding
	err = gdb.Where("stock_code = ?", code).First(&existing).Error
	if err == nil {
		return View{}, reject("duplicate_code", "该代码已在实盘镜像中，请编辑原记录")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return View{}, err
	}

	row := Holding{
		Source:    Source,
		StockCode: code,
		StockName: name,
		Quantity:  qty,
		CostPrice: cost,
		EntryDate: entry,
		Note:      note,
	}
	if err := gdb.Create(&row).Error; err != nil {
		return View{}, err
	}
	return toView(row), nil
}

// Update changes quantity, cost, and optional name/note/entry date. Code is immutable.
func Update(id uint, in Input) (View, error) {
	if id == 0 {
		return View{}, reject("not_found", "记录不存在")
	}
	gdb, err := gdbOrErr()
	if err != nil {
		return View{}, err
	}
	if err := rejectTradeFlags(in); err != nil {
		return View{}, err
	}
	if _, err := normalizeSource(in.Source); err != nil {
		return View{}, err
	}
	qty, err := normalizeQty(in.Quantity)
	if err != nil {
		return View{}, err
	}
	cost, err := normalizeCost(in.CostPrice)
	if err != nil {
		return View{}, err
	}
	entry, err := normalizeEntryDate(in.EntryDate, false)
	if err != nil {
		return View{}, err
	}
	var row Holding
	err = gdb.Where("id = ? AND source = ?", id, Source).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return View{}, reject("not_found", "记录不存在")
	}
	if err != nil {
		return View{}, err
	}
	row.Quantity = qty
	row.CostPrice = cost
	if entry != "" {
		row.EntryDate = entry
	}
	if in.NameSet {
		name, nerr := normalizeOptionalText(in.StockName, maxNameRunes, "invalid_name", "名称过长")
		if nerr != nil {
			return View{}, nerr
		}
		row.StockName = name
	}
	if in.NoteSet {
		note, nerr := normalizeOptionalText(in.Note, maxNoteRunes, "invalid_note", "备注过长")
		if nerr != nil {
			return View{}, nerr
		}
		row.Note = note
	}
	if err := gdb.Save(&row).Error; err != nil {
		return View{}, err
	}
	return toView(row), nil
}

// Delete removes one external_mirror row. Other books are untouched.
func Delete(id uint) error {
	if id == 0 {
		return reject("not_found", "记录不存在")
	}
	gdb, err := gdbOrErr()
	if err != nil {
		return err
	}
	res := gdb.Where("id = ? AND source = ?", id, Source).Delete(&Holding{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return reject("not_found", "记录不存在")
	}
	return nil
}
