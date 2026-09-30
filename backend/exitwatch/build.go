package exitwatch

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/externalmirror"
	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/positionstate"
)

// Options selects books and the shared clock. QuoteService is optional;
// when nil, the process quote reader is used for mirror rows only.
type Options struct {
	AsOf            time.Time
	Bar             string
	Sources         []string
	Policy          Policy
	QuoteService    marketdata.QuoteService
	ManualDraftRefs map[string]string
}

func (o Options) normalized() Options {
	if o.AsOf.IsZero() {
		o.AsOf = time.Now()
	}
	o.Bar = strings.TrimSpace(o.Bar)
	if o.Bar == "" {
		o.Bar = o.AsOf.In(time.Local).Format("2006-01-02")
	}
	if o.Policy.ID == "" && o.Policy.Version == 0 {
		o.Policy = DefaultPolicy
	}
	return o
}

func (o Options) wants(src string) bool {
	if len(o.Sources) == 0 {
		return true
	}
	for _, s := range o.Sources {
		switch strings.TrimSpace(strings.ToLower(s)) {
		case "all", src:
			return true
		}
	}
	return false
}

// ParseSource accepts paper_sim, external_mirror, all, or empty.
func ParseSource(raw string) (sources []string, ok bool) {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case "", "all":
		return nil, true
	case SourcePaperSim, SourceExternalMirror:
		return []string{s}, true
	default:
		return nil, false
	}
}

// Build reads the requested books and projects alerts. It does not persist alerts.
func Build(opt Options) *View {
	opt = opt.normalized()
	pol := DefaultPolicy.normalized()
	view := &View{
		Items:          []Item{},
		Sources:        []SourceStatus{},
		AsOf:           opt.AsOf,
		PolicyID:       pol.ID,
		PolicyVersion:  pol.Version,
		PolicyRef:      pol.Ref(),
		Disclaimer:     Disclaimer,
		DataSourceNote: dataSourceNote,
	}
	if opt.wants(SourcePaperSim) {
		items, err := loadPaper(opt)
		if err != nil {
			view.Sources = append(view.Sources, SourceStatus{
				Source:  SourcePaperSim,
				OK:      false,
				Message: "模拟持仓退出观察读取失败，已失败关闭",
			})
		} else {
			view.Sources = append(view.Sources, SourceStatus{Source: SourcePaperSim, OK: true})
			view.Items = append(view.Items, items...)
		}
	}
	if opt.wants(SourceExternalMirror) {
		items, err := loadMirror(opt)
		if err != nil {
			view.Sources = append(view.Sources, SourceStatus{
				Source:  SourceExternalMirror,
				OK:      false,
				Message: "实盘镜像退出观察读取失败，已失败关闭",
			})
		} else {
			view.Sources = append(view.Sources, SourceStatus{Source: SourceExternalMirror, OK: true})
			view.Items = append(view.Items, items...)
		}
	}
	view.Items = dedupe(sortItems(view.Items))
	return view
}

func loadPaper(opt Options) ([]Item, error) {
	view, err := papertrading.BuildExitEvaluation(papertrading.ExitEvaluationBuildOptions{AsOf: opt.AsOf})
	if err != nil {
		return nil, err
	}
	states := positionstate.NewService(nil).Evaluate(positionstate.Query{
		TradeDate: opt.Bar,
		AsOf:      opt.AsOf,
	})
	var rows []positionstate.PositionStateView
	if states != nil {
		rows = states.Positions
	}
	return EvaluatePaper(view, rows, opt), nil
}

func loadMirror(opt Options) ([]Item, error) {
	rows, err := externalmirror.List()
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		code := normCode(row.StockCode)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		codes = append(codes, row.StockCode)
	}
	svc := opt.QuoteService
	if svc == nil {
		svc = data.GetQuoteService()
	}
	return EvaluateMirror(rows, quoteSnaps(svc, codes), opt), nil
}

func quoteSnaps(svc marketdata.QuoteService, codes []string) []QuoteSnap {
	if svc == nil || len(codes) == 0 {
		return nil
	}
	quotes, err := svc.GetQuotes(codes)
	if err != nil || len(quotes) == 0 {
		return nil
	}
	out := make([]QuoteSnap, 0, len(quotes))
	for _, q := range quotes {
		out = append(out, QuoteSnap{Code: q.Code, Price: q.Price, FetchedAt: q.FetchedAt})
	}
	return out
}

func sortItems(items []Item) []Item {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		if items[i].StockCode != items[j].StockCode {
			return items[i].StockCode < items[j].StockCode
		}
		return items[i].PositionID < items[j].PositionID
	})
	return items
}

func dedupe(items []Item) []Item {
	if len(items) == 0 {
		return []Item{}
	}
	seen := map[string]bool{}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.ReasonCodes == nil {
			it.ReasonCodes = []string{}
		}
		if it.DedupKey != "" && seen[it.DedupKey] {
			continue
		}
		if it.DedupKey != "" {
			seen[it.DedupKey] = true
		}
		out = append(out, it)
	}
	return out
}
