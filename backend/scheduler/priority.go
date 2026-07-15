package scheduler

// 后台计算分层优先级（专业量化架构约定）
//
// 执行顺序：P0 → P1 → P2 → P3 → P4。
// TaskScheduler 提供有界优先级队列与单 worker；cron 仍可负责触发时刻，
// 回调应经 Submit 入队。P3/P4 不得抢占或阻塞 P0/P1：可取消任务在更高优入队时被软取消。
//
//	P0 执行安全：订单、成交、账户写、风险事件 — 即时、不可被扫描/AI 抢占
//	P1 持仓行情：持仓与活动委托标的批量报价（1–3s），增量权益/维保
//	P2 市场状态：指数与衍生指标（bar close 或 30–60s），写共享快照
//	P3 自选信号：单一后台扫描器（5–20min 或手动），消除双扫
//	P4 研究任务：全市场扫描、回测、AI — 可取消，仅空闲或用户触发
type Priority int

const (
	P0ExecSafety    Priority = 0
	P1PositionQuote Priority = 1
	P2MarketStatus  Priority = 2
	P3WatchlistScan Priority = 3
	P4Research      Priority = 4
)

func (p Priority) String() string {
	switch p {
	case P0ExecSafety:
		return "P0-exec-safety"
	case P1PositionQuote:
		return "P1-position-quote"
	case P2MarketStatus:
		return "P2-market-status"
	case P3WatchlistScan:
		return "P3-watchlist-scan"
	case P4Research:
		return "P4-research"
	default:
		return "P?-unknown"
	}
}

// TaskMeta 描述一个可调度后台任务的元数据。
type TaskMeta struct {
	Name     string
	Priority Priority
	// Cancelable 为 true 时，更高优任务可请求取消（P4/部分 P3）。
	Cancelable bool
}

// ShouldYieldToHigher 约定：当前优先级是否应让路给更高优（数值更小）任务。
func ShouldYieldToHigher(current, incoming Priority) bool {
	return incoming < current
}
