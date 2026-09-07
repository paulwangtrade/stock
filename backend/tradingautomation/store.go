package tradingautomation

import "sync"

var (
	lastStepMu sync.RWMutex
	lastMat    StepResult
	lastAppr   StepResult
	lastFreeze StepResult
)

// RecordStep stores the latest step result for UI observation (in-memory; not persisted).
func RecordStep(res StepResult) {
	lastStepMu.Lock()
	defer lastStepMu.Unlock()
	switch res.Step {
	case "materialize":
		lastMat = res
	case "approve":
		lastAppr = res
	case "freeze":
		lastFreeze = res
	}
}

// LastSteps returns copies of the latest automation step results.
func LastSteps() (mat, appr, freeze StepResult) {
	lastStepMu.RLock()
	defer lastStepMu.RUnlock()
	return lastMat, lastAppr, lastFreeze
}

// ResetLastSteps clears stored step results (tests).
func ResetLastSteps() {
	lastStepMu.Lock()
	defer lastStepMu.Unlock()
	lastMat = StepResult{}
	lastAppr = StepResult{}
	lastFreeze = StepResult{}
}
