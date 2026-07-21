package strategy

import "testing"

func TestStrategyScoreFromRank(t *testing.T) {
	if strategyScoreFromRank(1, 10) != 1.0 {
		t.Fatal("rank1")
	}
	if strategyScoreFromRank(2, 10) != 0.9 {
		t.Fatal("rank2")
	}
	if strategyScoreFromRank(10, 10) != 0.1 {
		t.Fatal("rank10")
	}
}
