package data

import (
	"errors"
	"fmt"
)

// paperFillRejectError Fill 失败拒单错误（带稳定 reject_code，供 PR2-A 分类）。
type paperFillRejectError struct {
	Code string
	Msg  string
}

func (e *paperFillRejectError) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func paperFillReject(code, format string, args ...any) error {
	return &paperFillRejectError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// classifyFillRejectCode 从 FillPaperOrder 错误链映射拒单码。
func classifyFillRejectCode(err error) string {
	if err == nil {
		return ""
	}
	var pe *paperFillRejectError
	if errors.As(err, &pe) && pe.Code != "" {
		return pe.Code
	}
	return PaperOrderRejectInternal
}
