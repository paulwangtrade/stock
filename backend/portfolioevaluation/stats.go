package portfolioevaluation

import (
	"math"
	"sort"
)

func scalarStats(vals []float64) ScalarStats {
	out := ScalarStats{Count: len(vals)}
	if len(vals) == 0 {
		return out
	}
	out.Min = vals[0]
	out.Max = vals[0]
	sum := 0.0
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		sum += v
		if v < out.Min {
			out.Min = v
		}
		if v > out.Max {
			out.Max = v
		}
	}
	out.Sum = sum
	out.Mean = sum / float64(len(vals))
	return out
}

func meanInts(vals []int) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := 0
	for _, v := range vals {
		s += v
	}
	return float64(s) / float64(len(vals))
}

func meanFloats(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := 0.0
	n := 0
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		s += v
		n++
	}
	if n == 0 {
		return 0
	}
	return s / float64(n)
}

func binsFromInt(hist map[int]int) []CountBin {
	keys := make([]int, 0, len(hist))
	for k := range hist {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]CountBin, 0, len(keys))
	for _, k := range keys {
		out = append(out, CountBin{Value: k, Count: hist[k]})
	}
	return out
}

func bump(m map[string]int, key string, n int) {
	if m == nil || key == "" || n == 0 {
		return
	}
	m[key] += n
}

func mergeCountMaps(dst map[string]int, src map[string]int) {
	if dst == nil || src == nil {
		return
	}
	for k, v := range src {
		dst[k] += v
	}
}

func cloneCountMap(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
