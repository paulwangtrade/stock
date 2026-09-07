package outcome

import (
	"sort"
	"strings"

	"go-stock/backend/papertrading"
)

// matchedLeg is one FIFO-matched quantity between a buy and optional sell fill.
type matchedLeg struct {
	BuyFill  papertrading.PaperSimFill
	SellFill *papertrading.PaperSimFill
	Qty      int64
	Status   string // OPEN or CLOSED
}

type buyQueueEntry struct {
	fill      papertrading.PaperSimFill
	remaining int64
}

func isBuyFill(f papertrading.PaperSimFill) bool {
	side := strings.ToLower(strings.TrimSpace(f.Side))
	return side == "" || side == "buy"
}

func isSellFill(f papertrading.PaperSimFill) bool {
	return strings.ToLower(strings.TrimSpace(f.Side)) == "sell"
}

// matchFIFOLegs pairs buy and sell fills for one stock using FIFO by filled_at, id.
func matchFIFOLegs(fills []papertrading.PaperSimFill) []matchedLeg {
	if len(fills) == 0 {
		return nil
	}
	sorted := append([]papertrading.PaperSimFill(nil), fills...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].FilledAt.Equal(sorted[j].FilledAt) {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].FilledAt.Before(sorted[j].FilledAt)
	})

	var (
		queue []buyQueueEntry
		out   []matchedLeg
	)

	for _, f := range sorted {
		vol := f.Volume
		if vol <= 0 {
			continue
		}
		if isBuyFill(f) {
			queue = append(queue, buyQueueEntry{fill: f, remaining: vol})
			continue
		}
		if !isSellFill(f) {
			continue
		}
		remainingSell := vol
		sellCopy := f
		for remainingSell > 0 && len(queue) > 0 {
			if queue[0].remaining <= 0 {
				queue = queue[1:]
				continue
			}
			matchQty := remainingSell
			if queue[0].remaining < matchQty {
				matchQty = queue[0].remaining
			}
			out = append(out, matchedLeg{
				BuyFill:  queue[0].fill,
				SellFill: &sellCopy,
				Qty:      matchQty,
				Status:   OutcomeStatusClosed,
			})
			queue[0].remaining -= matchQty
			remainingSell -= matchQty
			if queue[0].remaining == 0 {
				queue = queue[1:]
			}
		}
	}

	for _, q := range queue {
		if q.remaining <= 0 {
			continue
		}
		out = append(out, matchedLeg{
			BuyFill: q.fill,
			Qty:     q.remaining,
			Status:  OutcomeStatusOpen,
		})
	}
	return out
}
