package data

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

//go:embed signal_scan_bundle.js
var signalScanBundleJS string

var (
	signalScanVMOnce sync.Once
	signalScanVM     *goja.Runtime
	signalScanVMErr  error
)

func initSignalScanVM() {
	signalScanVM = goja.New()
	_, signalScanVMErr = signalScanVM.RunString(signalScanBundleJS)
}

type signalScanStockInput struct {
	Code          string         `json:"code"`
	Name          string         `json:"name"`
	Secucode      string         `json:"secucode,omitempty"`
	Closes        []float64      `json:"closes"`
	Opens         []float64      `json:"opens"`
	Highs         []float64      `json:"highs"`
	Lows          []float64      `json:"lows"`
	Volumes       []float64      `json:"volumes"`
	DayKeys       []string       `json:"dayKeys"`
	LastBarIndex  int            `json:"lastBarIndex,omitempty"`
	Row           map[string]any `json:"row,omitempty"`
}

type signalScanBatchInput struct {
	Stocks           []signalScanStockInput `json:"stocks"`
	IndexClose       map[string]float64     `json:"indexClose"`
	SignalParamsJSON string                 `json:"signalParamsJson,omitempty"`
	IncludeSell      bool                   `json:"includeSell"`
}

type signalScanBatchOutput struct {
	Items    []map[string]any `json:"items"`
	HitTotal int              `json:"hitTotal"`
}

// RunSignalScanBatchJS 使用内嵌 JS（与前端 icePointSignals 同源）批量计算信号
func RunSignalScanBatchJS(input signalScanBatchInput) (*signalScanBatchOutput, error) {
	signalScanVMOnce.Do(initSignalScanVM)
	if signalScanVMErr != nil {
		return nil, fmt.Errorf("signal scan vm init: %w", signalScanVMErr)
	}
	fnVal := signalScanVM.Get("SignalScanBatch")
	if fnVal == nil {
		return nil, fmt.Errorf("SignalScanBatch not defined in bundle")
	}
	obj := fnVal.ToObject(signalScanVM)
	if obj == nil {
		return nil, fmt.Errorf("SignalScanBatch is not an object")
	}
	runFn := obj.Get("runSignalScanBatch")
	if runFn == nil {
		return nil, fmt.Errorf("runSignalScanBatch not found")
	}
	inJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var inMap map[string]any
	if err := json.Unmarshal(inJSON, &inMap); err != nil {
		return nil, err
	}
	callable, ok := goja.AssertFunction(runFn)
	if !ok {
		return nil, fmt.Errorf("runSignalScanBatch is not a function")
	}
	res, err := callable(goja.Undefined(), signalScanVM.ToValue(inMap))
	if err != nil {
		return nil, fmt.Errorf("runSignalScanBatch: %w", err)
	}
	outJSON, err := json.Marshal(res.Export())
	if err != nil {
		return nil, err
	}
	var out signalScanBatchOutput
	if err := json.Unmarshal(outJSON, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
