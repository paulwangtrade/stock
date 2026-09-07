// Exit Review Outcome — user decision persistence (Phase14-D3).
// Append-only records; does not modify PositionState, Exit Evaluation, Execution, or TradePlan lifecycle.

package papertrading

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ExitReviewDecisionHold            = "HOLD"
	ExitReviewDecisionWatch           = "WATCH"
	ExitReviewDecisionCreateSellPlan  = "CREATE_SELL_PLAN"
	exitReviewOutcomeSchemaVersion    = "exit_review_outcome.v1"
	exitReviewOutcomeDefaultCreatedBy = "ui:exit-review"
)

var validExitReviewDecisions = map[string]struct{}{
	ExitReviewDecisionHold:           {},
	ExitReviewDecisionWatch:          {},
	ExitReviewDecisionCreateSellPlan: {},
}

// ExitReviewOutcome is the persisted user decision after exit review.
type ExitReviewOutcome struct {
	ID                          string    `gorm:"primaryKey;size:64" json:"id"`
	AccountID                   uint      `gorm:"index:idx_ero_acct_code_time,priority:1;not null" json:"account_id"`
	StockCode                   string    `gorm:"size:32;index:idx_ero_acct_code_time,priority:2;not null" json:"stock_code"`
	ReviewTime                  time.Time `gorm:"index:idx_ero_acct_code_time,priority:3;not null" json:"review_time"`
	Decision                    string    `gorm:"size:32;not null" json:"decision"`
	Reason                      string    `gorm:"size:500" json:"reason,omitempty"`
	CreatedBy                   string    `gorm:"size:120;not null" json:"created_by"`
	CreatedAt                   time.Time `json:"created_at"`
	ExitStateSnapshot           string    `gorm:"size:32;not null" json:"exit_state_snapshot"`
	ReasonCodesSnapshot         string    `gorm:"type:text" json:"-"`
	EvaluationSummarySnapshot   string    `gorm:"size:500" json:"evaluation_summary_snapshot,omitempty"`
	RelatedTradePlanID          *uint     `json:"related_trade_plan_id,omitempty"`
	SchemaVersion               string    `gorm:"size:32;default:exit_review_outcome.v1" json:"schema_version,omitempty"`
}

func (ExitReviewOutcome) TableName() string { return "exit_review_outcomes" }

// ExitReviewOutcomeSummary is the read model merged into exit-evaluation rows.
type ExitReviewOutcomeSummary struct {
	ID                 string    `json:"id"`
	Decision           string    `json:"decision"`
	ReviewTime         time.Time `json:"review_time"`
	Reason             string    `json:"reason,omitempty"`
	CreatedBy          string    `json:"created_by"`
	RelatedTradePlanID *uint     `json:"related_trade_plan_id,omitempty"`
}

// SaveExitReviewOutcomeInput is the write contract for POST outcome.
type SaveExitReviewOutcomeInput struct {
	AccountID                 uint
	StockCode                 string
	Decision                  string
	Reason                    string
	ReviewTime                time.Time
	CreatedBy                 string
	ExitStateSnapshot         string
	ReasonCodesSnapshot       []string
	EvaluationSummarySnapshot string
	RelatedTradePlanID        *uint
}

func normalizeOutcomeStockCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func validateExitReviewDecision(decision string) error {
	if _, ok := validExitReviewDecisions[strings.TrimSpace(decision)]; !ok {
		return fmt.Errorf("invalid decision %q", decision)
	}
	return nil
}

func encodeReasonCodesSnapshot(codes []string) (string, error) {
	if len(codes) == 0 {
		return "[]", nil
	}
	out := make([]string, 0, len(codes))
	seen := map[string]bool{}
	for _, c := range codes {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeReasonCodesSnapshot(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

// SaveExitReviewOutcome inserts a new outcome row (append-only).
func SaveExitReviewOutcome(in SaveExitReviewOutcomeInput) (*ExitReviewOutcome, error) {
	if db.Dao == nil {
		return nil, errors.New("papertrading: db not initialized")
	}
	if err := EnsureExitReviewOutcomeTable(db.Dao); err != nil {
		return nil, err
	}
	code := normalizeOutcomeStockCode(in.StockCode)
	if code == "" {
		return nil, errors.New("stock_code is required")
	}
	if err := validateExitReviewDecision(in.Decision); err != nil {
		return nil, err
	}
	state := strings.TrimSpace(in.ExitStateSnapshot)
	if state == "" {
		return nil, errors.New("exit_state_snapshot is required")
	}

	accountID := in.AccountID
	if accountID == 0 {
		acc, err := GetDefaultAccount()
		if err != nil {
			return nil, err
		}
		accountID = acc.ID
	}

	reviewTime := in.ReviewTime
	if reviewTime.IsZero() {
		reviewTime = time.Now()
	}
	createdBy := strings.TrimSpace(in.CreatedBy)
	if createdBy == "" {
		createdBy = exitReviewOutcomeDefaultCreatedBy
	}

	codesJSON, err := encodeReasonCodesSnapshot(in.ReasonCodesSnapshot)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	row := &ExitReviewOutcome{
		ID:                        "ero_" + uuid.NewString(),
		AccountID:                 accountID,
		StockCode:                 code,
		ReviewTime:                reviewTime,
		Decision:                  strings.TrimSpace(in.Decision),
		Reason:                    strings.TrimSpace(in.Reason),
		CreatedBy:                 createdBy,
		CreatedAt:                 now,
		ExitStateSnapshot:         state,
		ReasonCodesSnapshot:       codesJSON,
		EvaluationSummarySnapshot: strings.TrimSpace(in.EvaluationSummarySnapshot),
		RelatedTradePlanID:        in.RelatedTradePlanID,
		SchemaVersion:             exitReviewOutcomeSchemaVersion,
	}
	if err := db.Dao.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// LoadLatestOutcomesByAccount returns the newest outcome per stock_code for the account.
func LoadLatestOutcomesByAccount(accountID uint, stockCodes []string) (map[string]*ExitReviewOutcomeSummary, error) {
	out := map[string]*ExitReviewOutcomeSummary{}
	if db.Dao == nil || accountID == 0 {
		return out, nil
	}
	if err := EnsureExitReviewOutcomeTable(db.Dao); err != nil {
		return nil, err
	}

	want := map[string]bool{}
	for _, c := range stockCodes {
		if n := normalizeOutcomeStockCode(c); n != "" {
			want[n] = true
		}
	}

	q := db.Dao.Where("account_id = ?", accountID).Order("review_time desc, created_at desc")
	if len(want) > 0 {
		codes := make([]string, 0, len(want))
		for c := range want {
			codes = append(codes, c)
		}
		q = q.Where("stock_code IN ?", codes)
	}

	var rows []ExitReviewOutcome
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		code := rows[i].StockCode
		if _, exists := out[code]; exists {
			continue
		}
		out[code] = outcomeToSummary(&rows[i])
	}
	return out, nil
}

func outcomeToSummary(row *ExitReviewOutcome) *ExitReviewOutcomeSummary {
	if row == nil {
		return nil
	}
	return &ExitReviewOutcomeSummary{
		ID:                 row.ID,
		Decision:           row.Decision,
		ReviewTime:         row.ReviewTime,
		Reason:             row.Reason,
		CreatedBy:          row.CreatedBy,
		RelatedTradePlanID: row.RelatedTradePlanID,
	}
}

// AttachLatestOutcomes merges latest_outcome into exit evaluation holdings (read-only join).
func AttachLatestOutcomes(view *ExitEvaluationView) error {
	if view == nil || len(view.Holdings) == 0 {
		return nil
	}
	codes := make([]string, 0, len(view.Holdings))
	for _, h := range view.Holdings {
		codes = append(codes, h.StockCode)
	}
	accountID := view.AccountID
	if accountID == 0 {
		acc, err := GetDefaultAccount()
		if err != nil {
			return err
		}
		accountID = acc.ID
		view.AccountID = accountID
	}
	latest, err := LoadLatestOutcomesByAccount(accountID, codes)
	if err != nil {
		return err
	}
	for i := range view.Holdings {
		code := normalizeOutcomeStockCode(view.Holdings[i].StockCode)
		if o, ok := latest[code]; ok {
			view.Holdings[i].LatestOutcome = o
		}
	}
	return nil
}

// LoadLatestOutcomeForStock returns the newest outcome for one stock (tests / optional API).
func LoadLatestOutcomeForStock(accountID uint, stockCode string) (*ExitReviewOutcomeSummary, error) {
	m, err := LoadLatestOutcomesByAccount(accountID, []string{stockCode})
	if err != nil {
		return nil, err
	}
	return m[normalizeOutcomeStockCode(stockCode)], nil
}

// EnsureExitReviewOutcomeTable migrates the outcome table (idempotent).
func EnsureExitReviewOutcomeTable(gdb *gorm.DB) error {
	if gdb == nil {
		gdb = db.Dao
	}
	if gdb == nil {
		return fmt.Errorf("papertrading: db not initialized")
	}
	return gdb.AutoMigrate(&ExitReviewOutcome{})
}
