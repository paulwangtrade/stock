package strategy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Phase3-A Freeze：BuildCandidatePool 选股/Rank 路径不得读取或依赖 DecisionID。
func TestBuildCandidatePool_DecisionIDNotInScoreOrRankPath(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "build_candidate_pool.go")
	b, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)

	// Rank / Score 流水线文件不得引用 Decision 绑定
	forbidden := []string{
		"DecisionID",
		"decisionId",
		"decision_id",
		"EnrichCandidatePoolDecisionIDs",
		"go_engine",
		"BuildShadowDecision",
		"QuantProducerGoEngine",
	}
	for _, needle := range forbidden {
		if strings.Contains(body, needle) {
			t.Fatalf("build_candidate_pool.go must not reference %q (Phase3-A: Decision binding is post-Rank bypass only)", needle)
		}
	}

	// 选股与 Rank 步骤仍须存在
	for _, need := range []string{
		"Enhance(",
		"sort.SliceStable",
		"Rank = i + 1",
		"CreatePoolWithItems",
	} {
		if !strings.Contains(body, need) {
			t.Fatalf("build_candidate_pool.go missing expected pipeline step %q", need)
		}
	}
}

func TestEnrichCandidatePoolDecisionIDs_IsBypassOnly(t *testing.T) {
	// 旁路 enrich 属于 models，strategy 生成路径不自动调用 —— 用源码契约锁定。
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "build_candidate_pool.go")
	b, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "EnrichCandidatePoolDecisionIDs") {
		t.Fatal("BuildCandidatePool must not call EnrichCandidatePoolDecisionIDs (keep Rank/Score path clean)")
	}
}
