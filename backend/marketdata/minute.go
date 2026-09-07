package marketdata

// MinutePoint is one intraday minute sample (market fields only).
type MinutePoint struct {
	Time   string  `json:"time"`
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

// MinuteService reads intraday minute series (read-only).
type MinuteService interface {
	GetMinute(code string) (points []MinutePoint, asOfDate string, err error)
}
