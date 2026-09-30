package data

import (
	"math"
	"sort"
	"strings"

	"github.com/duke-git/lancet/v2/convertor"
	"go-stock/backend/models"
)

const (
	nextDaySetupDisclaimer  = "若触及可能形成，不保证，非买卖指令"
	nextDaySetupUnavailable = "无法给出唯一价"
	nextDaySetupMaxRows     = 400
)

func appendScanBatch(out *signalScanBatchOutput, items, setups *[]map[string]any) {
	if out == nil {
		return
	}
	if len(out.Items) > 0 {
		*items = append(*items, out.Items...)
	}
	if len(out.Setups) > 0 {
		*setups = append(*setups, out.Setups...)
	}
}

func finalizeNextDaySetups(raw []map[string]any) []models.NextDaySetupWatch {
	capped := capNextDaySetups(raw, nextDaySetupMaxRows)
	return mapSliceToSetups(capped)
}

func capNextDaySetups(in []map[string]any, limit int) []map[string]any {
	if len(in) == 0 {
		return []map[string]any{}
	}
	sort.SliceStable(in, func(i, j int) bool {
		di, oki := absDistance(in[i]["distancePct"])
		dj, okj := absDistance(in[j]["distancePct"])
		if oki != okj {
			return oki
		}
		if oki && di != dj {
			return di < dj
		}
		ni := convertor.ToString(in[i]["SECURITY_NAME_ABBR"])
		nj := convertor.ToString(in[j]["SECURITY_NAME_ABBR"])
		return ni < nj
	})
	if limit > 0 && len(in) > limit {
		return in[:limit]
	}
	return in
}

func absDistance(v any) (float64, bool) {
	f, ok := finiteFloat(v)
	if !ok {
		return 0, false
	}
	return math.Abs(f), true
}

func finiteFloat(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	f, err := convertor.ToFloat(v)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

func stringSliceField(v any) []string {
	switch t := v.(type) {
	case []string:
		return compactStrings(t)
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, convertor.ToString(item))
		}
		return compactStrings(out)
	default:
		return nil
	}
}

func compactStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// mapSliceToSetups 把观察行收成只读结构。
// 强制 confirmed/orderIntent 为 false；非 price 模式丢掉触发价。
func mapSliceToSetups(items []map[string]any) []models.NextDaySetupWatch {
	out := make([]models.NextDaySetupWatch, 0, len(items))
	for _, m := range items {
		if m == nil {
			continue
		}
		code := strings.TrimSpace(convertor.ToString(m["SECUCODE"]))
		tag := strings.TrimSpace(convertor.ToString(m["tag"]))
		engine := strings.TrimSpace(convertor.ToString(m["engine"]))
		if code == "" || tag == "" || engine == "" {
			continue
		}
		mode := strings.TrimSpace(convertor.ToString(m["priceMode"]))
		if mode != "price" && mode != "gap" {
			mode = "unavailable"
		}
		row := models.NextDaySetupWatch{
			SECUCODE:           code,
			SECURITY_CODE:      strings.TrimSpace(convertor.ToString(m["SECURITY_CODE"])),
			SECURITY_NAME_ABBR: strings.TrimSpace(convertor.ToString(m["SECURITY_NAME_ABBR"])),
			Engine:             engine,
			Tag:                tag,
			PriceMode:          mode,
			GapText:            strings.TrimSpace(convertor.ToString(m["gapText"])),
			Summary:            strings.TrimSpace(convertor.ToString(m["summary"])),
			StatusText:         strings.TrimSpace(convertor.ToString(m["statusText"])),
			Disclaimer:         nextDaySetupDisclaimer,
			AsOfDate:           strings.TrimSpace(convertor.ToString(m["asOfDate"])),
			ConditionGaps:      stringSliceField(m["conditionGaps"]),
			Confirmed:          false,
			OrderIntent:        false,
			ObservationOnly:    true,
		}
		if row.StatusText == "" {
			row.StatusText = "未确认 · 次日观察"
		}
		if closeT, ok := finiteFloat(m["closeT"]); ok && closeT > 0 {
			row.CloseT = closeT
		}
		if mode == "price" {
			px, ok := finiteFloat(m["triggerPrice"])
			if !ok || px <= 0 {
				row.PriceMode = "unavailable"
				row.TriggerPrice = nil
				if row.GapText == "" {
					row.GapText = nextDaySetupUnavailable
				}
			} else {
				row.TriggerPrice = &px
				if d, dok := finiteFloat(m["distancePct"]); dok {
					row.DistancePct = &d
				}
			}
		} else {
			row.TriggerPrice = nil
			row.DistancePct = nil
			if row.GapText == "" {
				row.GapText = nextDaySetupUnavailable
			}
		}
		out = append(out, row)
	}
	return out
}
