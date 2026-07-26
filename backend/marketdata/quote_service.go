// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A1 QuoteService 参数规范化与结果查找（不含任何取数实现）

package marketdata

import "strings"

// NormalizeCode 仅做空白裁剪，不改写市场前缀：代码格式转换仍由旧实现负责，
// 避免 Phase7-A1 引入任何行为差异。
func NormalizeCode(code string) (string, error) {
	c := strings.TrimSpace(code)
	if c == "" {
		return "", ErrEmptyCode
	}
	return c, nil
}

// NormalizeCodes 去空、去重并保持原始顺序；全部无效时返回 ErrEmptyCodes。
func NormalizeCodes(codes []string) ([]string, error) {
	out := make([]string, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, raw := range codes {
		c := strings.TrimSpace(raw)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, ErrEmptyCodes
	}
	return out, nil
}

// FindQuote 在批量结果中按代码查找，忽略大小写与市场前缀差异（如 sh600000 与 600000）。
func FindQuote(quotes []Quote, code string) *Quote {
	target := strings.ToUpper(strings.TrimSpace(code))
	if target == "" {
		return nil
	}
	for i := range quotes {
		got := strings.ToUpper(strings.TrimSpace(quotes[i].Code))
		if got == "" {
			continue
		}
		if got == target || strings.HasSuffix(got, target) || strings.HasSuffix(target, got) {
			return &quotes[i]
		}
	}
	return nil
}
