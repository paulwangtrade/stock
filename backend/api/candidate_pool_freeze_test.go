package api_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// Candidate Pool 只读接入防回退：不得侵入 Paper Execution / Phase2-C / 策略生成链路。

func TestCandidatePool_NoExecutionArchitectureCoupling(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	files := []string{
		filepath.Join(dir, "candidate_pool.go"),
		filepath.Join(dir, "..", "data", "research_candidate_pool.go"),
	}
	forbidden := []string{
		"go-stock/backend/execution",
		"go-stock/backend/strategy",
		"SignalEnhancer",
		"BuildCandidatePool(",
		"BuildTradePlan(",
		"NewExecutionService(",
		"PaperBroker",
		"RealBroker",
		"FakeExecutionReportHandler",
		"SubmitPaperOrder(",
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		require.NoError(t, err, f)
		body := string(b)
		for _, needle := range forbidden {
			require.NotContains(t, body, needle,
				"%s 不得耦合 %q（Candidate Pool 只读解析快照，不改交易架构）", filepath.Base(f), needle)
		}
	}

	researchBody, err := os.ReadFile(filepath.Join(dir, "..", "data", "research_candidate_pool.go"))
	require.NoError(t, err)
	require.Contains(t, string(researchBody), "CalcResearchSignalScore",
		"无内嵌 score 时应复用既有 CalcResearchSignalScore，不新增评分算法")

	apiBody, err := os.ReadFile(filepath.Join(dir, "candidate_pool.go"))
	require.NoError(t, err)
	require.Contains(t, string(apiBody), "ListResearchCandidatesFromLatestSnapshot")
	require.Contains(t, string(apiBody), "GetCandidatePoolScoreThreshold")
}

func TestCandidatePool_FrontendSourceContracts(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	apiTS, err := os.ReadFile(filepath.Join(root, "frontend", "src", "api", "candidatePool.ts"))
	require.NoError(t, err)
	require.Contains(t, string(apiTS), "/api/candidate_pool/list")
	require.Contains(t, string(apiTS), "snapshot_time")
	require.Contains(t, string(apiTS), "threshold")

	vue, err := os.ReadFile(filepath.Join(root, "frontend", "src", "components", "CandidatePool.vue"))
	require.NoError(t, err)
	body := string(vue)
	require.Contains(t, body, "getCandidatePool")
	require.Contains(t, body, "NEmpty")
	require.Contains(t, body, "NSkeleton")
	require.Contains(t, body, "snapshotTime")
	require.Contains(t, body, "threshold")
	require.Contains(t, body, "stock_code")
	require.Contains(t, body, "signal_score")
	// Phase3-PR3-B：模拟买入经 ResearchTradeIntent，禁止直连 SubmitPaperOrder
	require.Contains(t, body, "createResearchTradeIntent")
	require.Contains(t, body, "confirmResearchTradeIntent")
	require.Contains(t, body, "executeResearchTradeIntent")
	require.NotContains(t, body, "SubmitPaperOrder")
	require.NotContains(t, body, "BuildCandidatePool")
	require.NotContains(t, body, "RunPaperOpenBuyOnce")
}
