package execution

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase3-PR4-E：人工/研究执行入口架构冻结。

func TestPhase3_ExecutionEntryFacadesHaveNoPaperAccountingBypass(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	files := []string{
		filepath.Join(dir, "manual_trade_facade.go"),
		filepath.Join(dir, "research_trade_facade.go"),
	}
	forbidden := []string{
		"SubmitPaperOrder",
		"FillPaperOrder",
		"NewPaperTradingApi",
		"PaperTradingApi",
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		require.NoError(t, err, f)
		body := string(b)
		base := filepath.Base(f)
		for _, needle := range forbidden {
			require.NotContains(t, body, needle,
				"%s 不得出现 %q；须经 ExecutionService.ExecutePlanItem", base, needle)
		}
		require.Contains(t, body, "ExecutePlanItem(",
			"%s 必须调用 ExecutionService.ExecutePlanItem", base)
	}
}

func TestPhase3_ExecutionServiceIsSoleManualAndResearchExecEntry(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	portBody, err := os.ReadFile(filepath.Join(dir, "port.go"))
	require.NoError(t, err)
	require.Contains(t, string(portBody), "func (s *ExecutionService) ExecutePlanItem")
	require.Contains(t, string(portBody), "PreTradeCheck")

	manualBody, err := os.ReadFile(filepath.Join(dir, "manual_trade_facade.go"))
	require.NoError(t, err)
	require.Contains(t, string(manualBody), "f.svc.ExecutePlanItem")

	researchBody, err := os.ReadFile(filepath.Join(dir, "research_trade_facade.go"))
	require.NoError(t, err)
	require.Contains(t, string(researchBody), "f.svc.ExecutePlanItem")
}

func TestPhase3_SubmitPaperOrderProductionCallersArePaperBrokerOnly(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	allowedSubstr := []string{
		string(filepath.Join("execution", "paper_broker.go")),
		string(filepath.Join("data", "paper_trading.go")), // 方法定义
		"_test.go",
		string(filepath.Join("execution", "port_test.go")),
		"paper_open_buy_antifallback_test.go",
	}
	// 过渡：Wails 仍导出 App.SubmitPaperOrder，但 UI/API/Façade 不得调用。
	legacyApp := filepath.Join("app.go")

	var offenders []string
	scanRoots := []string{
		filepath.Join(root, "backend", "api"),
		filepath.Join(root, "backend", "execution"),
		filepath.Join(root, "frontend", "src"),
	}
	for _, scanRoot := range scanRoots {
		_ = filepath.Walk(scanRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".go" && ext != ".vue" && ext != ".ts" && ext != ".js" {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			body := string(b)
			if !strings.Contains(body, "SubmitPaperOrder(") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			// paper_broker 允许
			if strings.Contains(rel, "execution/paper_broker.go") {
				return nil
			}
			// 定义处允许出现在签名中；api/facade/vue 禁止调用
			if strings.HasPrefix(rel, "backend/api/") ||
				strings.Contains(rel, "_facade.go") ||
				strings.HasPrefix(rel, "frontend/src/") {
				offenders = append(offenders, rel)
			}
			return nil
		})
	}
	require.Empty(t, offenders, "SubmitPaperOrder( 不得出现在 api/facade/frontend，违规: %v", offenders)

	// App Wails 绑定仍存在（遗留导出），但不计入 Facade/UI 旁路
	appPath := filepath.Join(root, legacyApp)
	if b, err := os.ReadFile(appPath); err == nil {
		require.Contains(t, string(b), "SubmitPaperOrder",
			"遗留 Wails 导出仍可存在；UI 不得调用（见 phase3_trade_ui_freeze_test）")
	}
	_ = allowedSubstr
}
