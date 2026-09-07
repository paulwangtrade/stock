package execution

import (
	"context"
	"fmt"
	"strings"
)

// Phase6.5.7.5.1 Execution Report Consumer MVP。
//
// 统一异步回报消费面：ConsumeExecutionReport(report) 为唯一推荐入口。
// 复用现有 FakeExecutionReportHandler 引擎（ACK/TRADE/REJECT/CANCEL + report_id/exec_id 幂等 +
// 状态保护），并按需驱动 PositionAccountant（持仓会计端口）。
//
// 写界：Fill / Order runtime / Position accounting。
// 禁止：TradePlan / Intent / Frozen Spec / limit_price / target_volume / migration / OMS enum 扩展。

// ExecutionReportConsumer 通道无关的执行回报消费者。
type ExecutionReportConsumer struct {
	broker  *RealBroker
	handler *FakeExecutionReportHandler
}

// NewExecutionReportConsumer 绑定 RealBroker（OMS/Fill 宿主）与可选持仓会计端口。
// accountant 为 nil 时不做持仓会计（与 RealStub 现网行为一致）。
func NewExecutionReportConsumer(rb *RealBroker, accountant PositionAccountant) *ExecutionReportConsumer {
	if rb == nil {
		return nil
	}
	if accountant != nil {
		rb.SetPositionAccountant(accountant)
	}
	return &ExecutionReportConsumer{
		broker:  rb,
		handler: NewFakeExecutionReportHandler(rb),
	}
}

// ConsumeExecutionReport 统一入口：校验类型后委托引擎处理。
// 幂等与状态保护由底层引擎保证：
//   - 同 report_id 重复 → no-op 成功
//   - 同 exec_id 重复 TRADE → 不二次入账
//   - filled 后 REJECT / cancelled 后 TRADE → ErrReportInvalidTransition
func (c *ExecutionReportConsumer) ConsumeExecutionReport(ctx context.Context, report ExecutionReport) error {
	if c == nil || c.handler == nil {
		return fmt.Errorf("execution: nil ExecutionReportConsumer")
	}
	switch strings.ToUpper(strings.TrimSpace(report.ReportType)) {
	case ExecReportTypeACK, ExecReportTypeREJECT, ExecReportTypeTRADE, ExecReportTypeCANCEL:
		return c.handler.OnExecutionReport(ctx, report)
	default:
		return fmt.Errorf("%w: unknown ReportType %q", ErrReportInvalidPayload, report.ReportType)
	}
}
