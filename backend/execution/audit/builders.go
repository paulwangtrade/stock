package audit

import "time"

// EmitSubmitAttempt builds SUBMIT_ATTEMPT before BrokerAdapter.Submit.
func BuildSubmitAttempt(
	backend, brokerRequestID, clientOrderID, localOrderID, accountID, specHash string,
	symbol, side string, limitPrice float64, targetVolume int64,
	eventTime time.Time,
) Event {
	ev := NewBase(TypeSubmitAttempt, SourceExecution, backend, brokerRequestID, specHash, eventTime)
	ev.EventID = IDSubmitAttempt(brokerRequestID)
	ev.ClientOrderID = clientOrderID
	ev.LocalOrderID = localOrderID
	ev.AccountID = accountID
	ev.Payload = map[string]any{
		"broker_request_id": brokerRequestID,
		"gate_result":       "allowed",
		"symbol":            symbol,
		"side":              side,
		"limit_price":       limitPrice,
		"target_volume":     targetVolume,
		"submit_price":      limitPrice,
		"submit_volume":     targetVolume,
	}
	return ev
}

// BuildSubmitResult builds SUBMIT_ACCEPTED / SUBMIT_REJECTED / SUBMIT_TIMEOUT.
// submitOutcome must be accepted|rejected|timeout|unknown.
// response is the broker SubmitResponse snapshot (and optional adapter_error) stored in payload.response.
func BuildSubmitResult(
	eventType, backend, brokerRequestID, clientOrderID, localOrderID, accountID, specHash string,
	submitOutcome, omsStatus, brokerStatus string,
	submitTime, responseTime time.Time,
	rejectCode, rejectReason string,
	response any,
) Event {
	ev := NewBase(eventType, SourceExecution, backend, brokerRequestID, specHash, responseTime)
	ev.EventID = IDSubmitResult(brokerRequestID, submitOutcome)
	ev.ClientOrderID = clientOrderID
	ev.LocalOrderID = localOrderID
	ev.AccountID = accountID
	if response == nil {
		response = map[string]any{}
	}
	ev.Payload = map[string]any{
		"broker_request_id": brokerRequestID,
		"submit_outcome":    submitOutcome,
		"response":          response,
		"submit_time":       submitTime.UTC(),
		"response_time":     responseTime.UTC(),
		"oms_status":        omsStatus,
		"broker_status":     brokerStatus,
	}
	if submitOutcome == "timeout" || submitOutcome == "unknown" {
		ev.Payload["reconciliation_required"] = true
	}
	if rejectCode != "" {
		ev.Payload["reject_code"] = rejectCode
	}
	if rejectReason != "" {
		ev.Payload["reject_reason"] = rejectReason
	}
	return ev
}

// BuildReportReceived builds REPORT_RECEIVED.
func BuildReportReceived(
	backend, brokerRequestID, clientOrderID, localOrderID, accountID, specHash string,
	reportID, reportType, execID string,
	dedupResult, applyResult string,
	occurredAt time.Time,
	rawReport any,
	extra map[string]any,
) Event {
	ev := NewBase(TypeReportReceived, SourceReportConsumer, backend, brokerRequestID, specHash, time.Now())
	ev.EventID = IDReportReceived(reportID)
	ev.ClientOrderID = clientOrderID
	ev.LocalOrderID = localOrderID
	ev.AccountID = accountID
	ev.ReportID = reportID
	ev.ExecID = execID
	ev.Payload = map[string]any{
		"report_id":    reportID,
		"exec_id":      execID,
		"order_id":     localOrderID,
		"report_type":  reportType,
		"occurred_at":  occurredAt.UTC(),
		"raw_report":   rawReport,
		"dedup_result": dedupResult,
		"apply_result": applyResult,
	}
	for k, v := range extra {
		ev.Payload[k] = v
	}
	return ev
}

// BuildFillApplied builds FILL_APPLIED.
func BuildFillApplied(
	backend, brokerRequestID, clientOrderID, localOrderID, accountID, specHash string,
	reportID, execID, causationID string,
	fillQty int64, fillPrice float64,
	cumQty int64, avgPrice float64, leaves int64, positionDelta int64,
	omsAfter, brokerAfter string,
	eventTime time.Time,
) Event {
	ev := NewBase(TypeFillApplied, SourceReportConsumer, backend, brokerRequestID, specHash, eventTime)
	ev.EventID = IDFillApplied(execID)
	ev.ClientOrderID = clientOrderID
	ev.LocalOrderID = localOrderID
	ev.AccountID = accountID
	ev.ReportID = reportID
	ev.ExecID = execID
	ev.CausationEventID = causationID
	ev.Payload = map[string]any{
		"exec_id":             execID,
		"fill_qty":            fillQty,
		"fill_price":          fillPrice,
		"spec_hash":           specHash,
		"order_id":            localOrderID,
		"report_id":           reportID,
		"cum_qty":             cumQty,
		"avg_price":           avgPrice,
		"leaves_quantity":     leaves,
		"position_delta":      positionDelta,
		"oms_status_after":    omsAfter,
		"broker_status_after": brokerAfter,
	}
	return ev
}

// BuildOrderTerminal builds ORDER_TERMINAL.
func BuildOrderTerminal(
	backend, brokerRequestID, clientOrderID, localOrderID, accountID, specHash string,
	terminalStatus, brokerStatus string,
	filledVolume, leaves int64, avgFillPrice float64,
	causationID string,
	eventTime time.Time,
	extra map[string]any,
) Event {
	ev := NewBase(TypeOrderTerminal, SourceReportConsumer, backend, brokerRequestID, specHash, eventTime)
	ev.EventID = IDOrderTerminal(localOrderID, terminalStatus, causationID)
	ev.ClientOrderID = clientOrderID
	ev.LocalOrderID = localOrderID
	ev.AccountID = accountID
	ev.CausationEventID = causationID
	ev.Payload = map[string]any{
		"order_id":        localOrderID,
		"terminal_status": terminalStatus,
		"broker_status":   brokerStatus,
		"filled_volume":   filledVolume,
		"leaves_quantity": leaves,
		"avg_fill_price":  avgFillPrice,
	}
	for k, v := range extra {
		ev.Payload[k] = v
	}
	return ev
}
