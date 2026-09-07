package shadow

import (
	"fmt"
	"math"

	"go-stock/backend/models"
)

type actionCtx struct {
	Tag             string
	DaysAgo         int
	IsHistorical    bool
	ChecklistReady  bool
	SellPositionPct *float64
	AddPositionPct  *float64
	RushReducePct   *float64
	SourceTag       string
	Zone            *models.QuantEntryZone
}

// deriveShadowAction：Go shadow 侧 Action 派生（与 js deriveQuantAction 语义对齐的子集）。
// 注意：这是 shadow 候选实现，不替换 js_legacy，也不接入交易链路。
func deriveShadowAction(ctx actionCtx) models.QuantAction {
	tag := ctx.Tag
	if tag == "" {
		return models.QuantAction{Code: "", Label: "", Side: "none"}
	}

	if tag == "止" || tag == "减" {
		label := "先风控"
		code := models.QuantActionReduce
		if tag == "止" {
			code = models.QuantActionExitPartial
		}
		if ctx.IsHistorical {
			label = "历史参考"
		}
		return models.QuantAction{Code: code, Label: label, Side: "sell"}
	}

	if tag == "冲" {
		label := "早减"
		if ctx.IsHistorical {
			label = "历史参考"
		}
		return models.QuantAction{Code: models.QuantActionReduce, Label: label, Side: "sell"}
	}

	if tag == "加" {
		label := "可加仓"
		if ctx.IsHistorical {
			label = "历史参考"
		}
		return models.QuantAction{Code: models.QuantActionScaleIn, Label: label, Side: "buy"}
	}

	if tag == "冰" {
		return models.QuantAction{Code: models.QuantActionWatch, Label: "仅观察", Side: "none"}
	}

	if !isEntryTag(tag) {
		return models.QuantAction{Code: "", Label: "", Side: "none"}
	}

	// entry tags
	if ctx.IsHistorical {
		return models.QuantAction{Code: models.QuantActionWatch, Label: "历史参考", Side: "none"}
	}
	if ctx.ChecklistReady {
		return models.QuantAction{Code: models.QuantActionEnter, Label: "可买", Side: "buy"}
	}
	zone := ctx.Zone
	mode := models.QuantZoneModeUnknown
	deferMode := models.QuantDeferSame
	if zone != nil {
		mode = zone.Mode
		deferMode = zone.DeferMode
	}
	if mode == models.QuantZoneModeAbove || deferMode == models.QuantDeferWait {
		return models.QuantAction{Code: models.QuantActionWaitPullback, Label: "等回踩", Side: "none"}
	}
	if mode == models.QuantZoneModeInZone || mode == models.QuantZoneModeNear {
		if tag == "买" && ctx.DaysAgo == 0 {
			return models.QuantAction{Code: models.QuantActionWatch, Label: "仅观察", Side: "none"}
		}
		return models.QuantAction{Code: models.QuantActionEnter, Label: "可买", Side: "buy"}
	}
	if mode == models.QuantZoneModeBelow {
		return models.QuantAction{Code: models.QuantActionEnter, Label: "可买", Side: "buy"}
	}
	if tag == "买" {
		return models.QuantAction{Code: models.QuantActionWatch, Label: "仅观察", Side: "none"}
	}
	if tag == "强" || tag == "突" || tag == "弹" || tag == "转" {
		return models.QuantAction{Code: models.QuantActionWaitPullback, Label: "等回踩", Side: "none"}
	}
	return models.QuantAction{Code: models.QuantActionWatch, Label: "仅观察", Side: "none"}
}

func isEntryTag(tag string) bool {
	switch tag {
	case "强", "趋", "转", "突", "买", "弹":
		return true
	default:
		return false
	}
}

// FormatPct 辅助测试展示。
func FormatPct(p float64) string {
	return fmt.Sprintf("%d%%", int(math.Round(p*100)))
}
